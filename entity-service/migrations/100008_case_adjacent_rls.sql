-- Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
--
-- WSO2 LLC. licenses this file to you under the Apache License,
-- Version 2.0 (the "License"); you may not use this file except
-- in compliance with the License.
-- You may obtain a copy of the License at
--
-- http://www.apache.org/licenses/LICENSE-2.0
--
-- Unless required by applicable law or agreed to in writing,
-- software distributed under the License is distributed on an
-- "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
-- KIND, either express or implied.  See the License for the
-- specific language governing permissions and limitations
-- under the License.

-- Enforces project-membership visibility for work_item and "case" (the
-- shared-PK case extension table) plus the case-adjacent satellite tables
-- keyed off a work_item id: comment, comment_edit_history, case_attachment,
-- work_item_tag, work_item_watcher, work_item_activity. This is the biggest
-- single migration in the series so far, and the one that finally converts
-- case_repo.go and comment_repo.go, which is why it lands as one migration
-- rather than several: work_item's own RLS affects every one of these
-- tables simultaneously (they all reach a project only through it), so a
-- half-converted state -- work_item protected but comment not, say -- would
-- be actively broken, not just incomplete.
--
-- work_item.project_id is a real column on work_item itself (unlike
-- change_request/conversation, which needed a subquery through a shared-PK
-- join to find it) -- so is_project_member is called directly on it, no
-- subquery needed, in every policy below that touches work_item or "case".
--
-- service_request/engagement/security_report_analysis (the three other
-- case-like work_item extension tables, alongside "case" and announcement)
-- got NO RLS of their own in this migration -- CORRECTED by migration 100012.
-- The reasoning originally written here borrowed project_contact's own
-- exclusion justification (migration 100002: protecting it would recurse,
-- since is_project_member() itself queries project_contact) and wrongly
-- applied it to these three tables, which is_project_member() never
-- queries at all -- they are structurally identical to "case"/comment/
-- case_attachment below, which already use the exact same
-- is_project_member-via-work_item-subquery shape with no recursion issue.
-- Safe today only because nothing in this codebase reads or writes them
-- without also going through a work_item-gated statement (confirmed by
-- grep, not assumed) -- exactly the "safe by Go-code accident, not by
-- database guarantee" gap this whole migration series exists to close.
-- See 0151 for the real fix and the full reasoning.
--
-- One transaction, and every CREATE POLICY preceded by its own DROP POLICY
-- IF EXISTS -- see migration 100002's identical note.
BEGIN;

ALTER TABLE work_item ENABLE ROW LEVEL SECURITY;
ALTER TABLE work_item FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS work_item_visibility ON work_item;
CREATE POLICY work_item_visibility ON work_item
  FOR SELECT
  USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member(project_id)
  );

-- WITH CHECK repeats is_project_member(project_id) rather than copying
-- USING's row wholesale: in an UPDATE policy, USING's unqualified
-- project_id resolves to the OLD row and WITH CHECK's to the NEW one, so
-- writing the same expression in both is what stops a caller moving a case
-- OUT of a project they belong to as much as it stops moving one IN --
-- PatchChangeRequest's own project_id write (already live, migration
-- 100006's own deferred-gap note) is exactly the write path this closes.
DROP POLICY IF EXISTS work_item_update ON work_item;
CREATE POLICY work_item_update ON work_item
  FOR UPDATE USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member(project_id)
  )
  WITH CHECK (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member(project_id)
  );

-- INSERT is the same is_internal OR is_project_member shape as UPDATE, not
-- internal-only like change_request's/conversation's: CreateCase is
-- genuinely customer-facing (it just never succeeds today, blocked on
-- work_item.number having no generation strategy -- see CreateCase's own
-- doc comment), and CreateCaseFromServiceNow's five branches (case/
-- announcement/service_request/engagement/security_report_analysis) all
-- explicitly stamp WithSystemIdentity in Go before this INSERT runs (see
-- CreateCaseFromServiceNow's own doc comment) since none of them run on a
-- request context with a real customer viewer to forward -- they're
-- ServiceNow-sync writes, same reasoning already established for
-- change_request's own INSERT policy, just satisfied by is_internal here
-- rather than needing its own internal-only policy, since a real customer
-- create (once it exists) must also be able to succeed through this same
-- policy.
DROP POLICY IF EXISTS work_item_write ON work_item;
CREATE POLICY work_item_write ON work_item
  FOR INSERT WITH CHECK (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member(project_id)
  );
-- Internal-only, not omitted entirely: nothing in PRODUCTION code deletes a
-- work_item row, but FORCE plus zero policies would also block legitimate
-- internal/admin/test cleanup, the same sla_delete lesson from migration
-- 0142.
DROP POLICY IF EXISTS work_item_delete_internal_only ON work_item;
CREATE POLICY work_item_delete_internal_only ON work_item
  FOR DELETE USING (current_setting('app.is_internal', true) = 'true');

ALTER TABLE "case" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "case" FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS case_visibility ON "case";
CREATE POLICY case_visibility ON "case"
  FOR SELECT
  USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = "case".id))
  );

DROP POLICY IF EXISTS case_update ON "case";
CREATE POLICY case_update ON "case"
  FOR UPDATE USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = "case".id))
  )
  WITH CHECK (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = "case".id))
  );

DROP POLICY IF EXISTS case_write ON "case";
CREATE POLICY case_write ON "case"
  FOR INSERT WITH CHECK (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = "case".id))
  );
-- Internal-only, not omitted entirely -- same sla_delete-style reasoning as
-- work_item_delete_internal_only above.
DROP POLICY IF EXISTS case_delete_internal_only ON "case";
CREATE POLICY case_delete_internal_only ON "case"
  FOR DELETE USING (current_setting('app.is_internal', true) = 'true');

-- comment backs FOUR reference types through one generic table
-- (comment_repo.go's ReferenceTypeToWorkItemType: case-like, CONVERSATION,
-- CHANGE_REQUEST, INCIDENT), plus case_repo.go's own case-specific
-- CreateCaseComment/SearchCaseComments and incident_repo.go's own
-- CreateIncidentComment/SearchIncidentActivities -- all keyed by the same
-- work_item_id with no reference-type branch of its own needed here: an
-- INCIDENT-linked comment's work_item.project_id is always NULL (incidents
-- have no project concept at all, confirmed live earlier in this session),
-- so is_project_member(NULL) is simply false for it -- every external
-- caller is correctly denied and only is_internal sees it, which is
-- exactly the deny-all behaviour this branch's plan calls for on
-- incident/incident_task/problem, arrived at for free rather than as a
-- special case.
--
-- Both comment_repo.go's generic path AND case_repo.go's/incident_repo.go's
-- own case/incident-specific comment methods had ZERO project-based
-- authorization at the SQL level before this migration -- confirmed by
-- reading every one of them: SearchCaseComments/SearchComments/
-- CreateCaseComment/CreateComment all queried or wrote comment directly,
-- with no join or scope check against work_item.project_id anywhere. This
-- was a real, live gap: any authenticated caller could read or write any
-- case's comments by knowing (or guessing) its UUID, regardless of project
-- membership.
ALTER TABLE comment ENABLE ROW LEVEL SECURITY;
ALTER TABLE comment FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS comment_visibility ON comment;
CREATE POLICY comment_visibility ON comment
  FOR SELECT
  USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = comment.work_item_id))
  );

DROP POLICY IF EXISTS comment_write ON comment;
CREATE POLICY comment_write ON comment
  FOR INSERT WITH CHECK (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = work_item_id))
  );
DROP POLICY IF EXISTS comment_update ON comment;
CREATE POLICY comment_update ON comment
  FOR UPDATE USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = comment.work_item_id))
  )
  WITH CHECK (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = work_item_id))
  );
-- Internal-only, not omitted entirely: comments are soft-deleted
-- (comment.deleted_at) by every PRODUCTION code path, never hard-deleted,
-- but the same sla_delete-style test/admin cleanup need applies.
DROP POLICY IF EXISTS comment_delete_internal_only ON comment;
CREATE POLICY comment_delete_internal_only ON comment
  FOR DELETE USING (current_setting('app.is_internal', true) = 'true');

-- comment_edit_history has no project_id or work_item_id of its own -- it
-- reaches one via comment.work_item_id, so its policy re-derives the same
-- membership check through that join, mirroring case_escalation_
-- notification_list's own reasoning (migration 100002). Append-only from
-- this codebase's perspective (UpdateComment inserts a row every time it
-- runs, nothing ever updates or deletes one), so only SELECT/INSERT
-- policies exist.
ALTER TABLE comment_edit_history ENABLE ROW LEVEL SECURITY;
ALTER TABLE comment_edit_history FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS comment_edit_history_visibility ON comment_edit_history;
CREATE POLICY comment_edit_history_visibility ON comment_edit_history
  FOR SELECT
  USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((
      SELECT wi.project_id
      FROM comment cm
      JOIN work_item wi ON wi.id = cm.work_item_id
      WHERE cm.id = comment_edit_history.comment_id
    ))
  );

DROP POLICY IF EXISTS comment_edit_history_write ON comment_edit_history;
CREATE POLICY comment_edit_history_write ON comment_edit_history
  FOR INSERT WITH CHECK (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((
      SELECT wi.project_id
      FROM comment cm
      JOIN work_item wi ON wi.id = cm.work_item_id
      WHERE cm.id = comment_id
    ))
  );
-- Internal-only, not omitted entirely -- same sla_delete-style test/admin
-- cleanup reasoning as every other "nothing in production deletes this"
-- table in this migration.
DROP POLICY IF EXISTS comment_edit_history_delete_internal_only ON comment_edit_history;
CREATE POLICY comment_edit_history_delete_internal_only ON comment_edit_history
  FOR DELETE USING (current_setting('app.is_internal', true) = 'true');

-- case_attachment has no project_id of its own -- it reaches one via
-- case_id (== work_item.id, the same shared-PK pattern as "case" itself).
-- Same "zero authorization today" gap as comment: CreateCaseAttachment/
-- SearchCaseAttachments/GetCaseAttachmentByID/DeleteCaseAttachment/
-- UpdateCaseAttachmentName/ConfirmCaseAttachment all queried or wrote
-- case_attachment directly, with no project scope check anywhere.
ALTER TABLE case_attachment ENABLE ROW LEVEL SECURITY;
ALTER TABLE case_attachment FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS case_attachment_visibility ON case_attachment;
CREATE POLICY case_attachment_visibility ON case_attachment
  FOR SELECT
  USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = case_attachment.case_id))
  );

DROP POLICY IF EXISTS case_attachment_write ON case_attachment;
CREATE POLICY case_attachment_write ON case_attachment
  FOR INSERT WITH CHECK (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = case_id))
  );
DROP POLICY IF EXISTS case_attachment_update ON case_attachment;
CREATE POLICY case_attachment_update ON case_attachment
  FOR UPDATE USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = case_attachment.case_id))
  )
  WITH CHECK (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = case_id))
  );
DROP POLICY IF EXISTS case_attachment_delete ON case_attachment;
CREATE POLICY case_attachment_delete ON case_attachment
  FOR DELETE USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = case_attachment.case_id))
  );

-- work_item_tag has no project_id of its own -- reaches one via
-- work_item_id, same shape as case_escalation_notification_list.
ALTER TABLE work_item_tag ENABLE ROW LEVEL SECURITY;
ALTER TABLE work_item_tag FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS work_item_tag_visibility ON work_item_tag;
CREATE POLICY work_item_tag_visibility ON work_item_tag
  FOR SELECT
  USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = work_item_tag.work_item_id))
  );

DROP POLICY IF EXISTS work_item_tag_write ON work_item_tag;
CREATE POLICY work_item_tag_write ON work_item_tag
  FOR INSERT WITH CHECK (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = work_item_id))
  );
DROP POLICY IF EXISTS work_item_tag_update ON work_item_tag;
CREATE POLICY work_item_tag_update ON work_item_tag
  FOR UPDATE USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = work_item_tag.work_item_id))
  )
  WITH CHECK (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = work_item_id))
  );
DROP POLICY IF EXISTS work_item_tag_delete ON work_item_tag;
CREATE POLICY work_item_tag_delete ON work_item_tag
  FOR DELETE USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = work_item_tag.work_item_id))
  );

-- work_item_watcher has no project_id of its own -- reaches one via
-- work_item_id. No UPDATE policy: SetCaseWatchList always deletes and
-- re-inserts the whole list, mirroring time_card_approver's identical
-- reasoning (migration 100005) for why that table also has no UPDATE
-- policy.
ALTER TABLE work_item_watcher ENABLE ROW LEVEL SECURITY;
ALTER TABLE work_item_watcher FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS work_item_watcher_visibility ON work_item_watcher;
CREATE POLICY work_item_watcher_visibility ON work_item_watcher
  FOR SELECT
  USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = work_item_watcher.work_item_id))
  );

DROP POLICY IF EXISTS work_item_watcher_write ON work_item_watcher;
CREATE POLICY work_item_watcher_write ON work_item_watcher
  FOR INSERT WITH CHECK (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = work_item_id))
  );
DROP POLICY IF EXISTS work_item_watcher_delete ON work_item_watcher;
CREATE POLICY work_item_watcher_delete ON work_item_watcher
  FOR DELETE USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = work_item_watcher.work_item_id))
  );

-- work_item_activity has no project_id of its own -- reaches one via
-- work_item_id. Written only as a best-effort side effect of another write
-- (RecordCaseFieldChangeActivity), on the SAME ctx/identity as whatever
-- triggered it, so is_internal OR is_project_member matches every other
-- write policy in this migration rather than being internal-only: a
-- genuine customer-triggered UpdateCase must still be able to record its
-- own field-change activity entry.
ALTER TABLE work_item_activity ENABLE ROW LEVEL SECURITY;
ALTER TABLE work_item_activity FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS work_item_activity_visibility ON work_item_activity;
CREATE POLICY work_item_activity_visibility ON work_item_activity
  FOR SELECT
  USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = work_item_activity.work_item_id))
  );

DROP POLICY IF EXISTS work_item_activity_write ON work_item_activity;
CREATE POLICY work_item_activity_write ON work_item_activity
  FOR INSERT WITH CHECK (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = work_item_id))
  );
-- No UPDATE policy: work_item_activity rows are an append-only audit
-- trail, nothing in this codebase updates one. DELETE is internal-only,
-- not omitted, for the same test/admin cleanup reasoning as elsewhere in
-- this migration.
DROP POLICY IF EXISTS work_item_activity_delete_internal_only ON work_item_activity;
CREATE POLICY work_item_activity_delete_internal_only ON work_item_activity
  FOR DELETE USING (current_setting('app.is_internal', true) = 'true');

COMMIT;
