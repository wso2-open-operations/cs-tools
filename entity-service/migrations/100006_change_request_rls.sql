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

-- Enforces project-membership visibility for change_request (plus the
-- generic approval_stage/approval_stage_approver tables it uses for its
-- approval workflow) at the database layer. Fifth table after
-- case_escalation (0141), sla (0142), customer_call (0143), time_card
-- (0144). This is another previously-ACKNOWLEDGED gap, specifically named
-- in the team review that started this whole migration series
-- ("GetChangeRequestApprovals" and the patch flow) -- change_request_repo.go
-- never did any caller-scoped authorization at all before this.
--
-- change_request shares its primary key with work_item (change_request.id =
-- work_item.id), so, unlike every earlier table in this series, there is no
-- separate join column -- the membership check goes straight through that
-- shared id.
--
-- IMPORTANT, deliberately deferred (same posture already accepted for
-- case_repo.go/incident_repo.go's SLA-filter regression -- see migration
-- 100005's own history): work_item itself has NO RLS yet (that lands in the
-- case-adjacent phase, which finally converts case_repo.go). PatchChangeRequest
-- updates BOTH work_item and change_request in one transaction; only the
-- change_request half is protected by this migration. Concretely: title/
-- description/projectId/caseId/deploymentId/deployedProductId/
-- assignedEngineerId (all work_item columns) remain unscoped until then,
-- while impact/state/justification/serviceId/and the rest of
-- change_request's own columns become correctly scoped by this migration.
-- This is a strict improvement over today (where NONE of it was scoped),
-- not a new gap.
--
-- One transaction, and every CREATE POLICY preceded by its own DROP POLICY
-- IF EXISTS -- see migration 100002's identical note.
BEGIN;

ALTER TABLE change_request ENABLE ROW LEVEL SECURITY;
ALTER TABLE change_request FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS change_request_visibility ON change_request;
CREATE POLICY change_request_visibility ON change_request
  FOR SELECT
  USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = change_request.id))
  );

-- The WITH CHECK subquery below deliberately writes change_request.id, not a
-- bare id: work_item ALSO has a column literally named id (change_request
-- shares work_item's primary key), so an unqualified "id" inside a subquery
-- whose own FROM clause is "work_item wi" resolves to wi.id itself (an
-- always-true self-comparison, silently returning every work_item row)
-- rather than correlating back to the row being updated -- caught live via
-- "more than one row returned by a subquery used as an expression" the first
-- time this policy was exercised against real data. sla/customer_call/
-- time_card's own WITH CHECK subqueries (migrations 100003-100005) never hit
-- this because they correlate on work_item_id, a name work_item has no
-- column called, so there was no ambiguity to resolve incorrectly.
DROP POLICY IF EXISTS change_request_update ON change_request;
CREATE POLICY change_request_update ON change_request
  FOR UPDATE USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = change_request.id))
  )
  WITH CHECK (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = change_request.id))
  );

-- INSERT is internal-only, unlike sla/customer_call/time_card's
-- customer-facing writes: CreateChangeRequestFromServiceNow's own INSERT
-- (see change_request_repo.go's package doc comment) never sets a project_id
-- at all -- CreateChangeRequestRequest has no project field whatsoever, so
-- there is nothing to check membership against here regardless of who
-- issued the original HTTP request. This mirrors what ServiceNow's own
-- workflow has already authorized (the insert only runs after ServiceNow
-- itself accepted the create), not a raw, uncontrolled customer write --
-- the repository method explicitly stamps an internal identity for this one
-- statement (see CreateChangeRequestFromServiceNow's own code comment)
-- rather than trusting whatever identity happened to be on the original
-- request context.
DROP POLICY IF EXISTS change_request_write_internal_only ON change_request;
CREATE POLICY change_request_write_internal_only ON change_request
  FOR INSERT WITH CHECK (current_setting('app.is_internal', true) = 'true');
-- No DELETE policy: nothing in this codebase deletes a change_request row.

-- approval_stage/approval_stage_approver (migration 000087) are generic,
-- reused by any approvable work_item type, but the only Go code touching
-- them today is change_request_repo.go (GetChangeRequestApprovals/
-- DecideChangeRequestApproval) -- see that migration's own doc comment.
--
-- Real writer of approval_stage / most approval_stage_approver rows is
-- csm-sync-service, a wholly separate service with its own DB role -- NOT
-- entity-service's migration-owning role. FORCE ROW LEVEL SECURITY here is
-- being applied for entity-service's own connection only; whether
-- csm-sync-service's role needs an explicit exemption (or already has
-- BYPASSRLS) in staging/production is the SAME open prerequisite already
-- called out in this branch's plan for every table's ownership assumption --
-- it must be confirmed before this migration runs anywhere but local Docker.
ALTER TABLE approval_stage ENABLE ROW LEVEL SECURITY;
ALTER TABLE approval_stage FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS approval_stage_visibility ON approval_stage;
CREATE POLICY approval_stage_visibility ON approval_stage
  FOR SELECT
  USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = approval_stage.work_item_id))
  );

-- entity-service never writes approval_stage itself (read-only from this
-- codebase's perspective) -- internal-only is purely a safe default for
-- internal tooling/admin use, not a path any current Go code exercises.
DROP POLICY IF EXISTS approval_stage_write_internal_only ON approval_stage;
CREATE POLICY approval_stage_write_internal_only ON approval_stage
  FOR INSERT WITH CHECK (current_setting('app.is_internal', true) = 'true');
DROP POLICY IF EXISTS approval_stage_update_internal_only ON approval_stage;
CREATE POLICY approval_stage_update_internal_only ON approval_stage
  FOR UPDATE USING (current_setting('app.is_internal', true) = 'true')
  WITH CHECK (current_setting('app.is_internal', true) = 'true');
DROP POLICY IF EXISTS approval_stage_delete_internal_only ON approval_stage;
CREATE POLICY approval_stage_delete_internal_only ON approval_stage
  FOR DELETE USING (current_setting('app.is_internal', true) = 'true');

ALTER TABLE approval_stage_approver ENABLE ROW LEVEL SECURITY;
ALTER TABLE approval_stage_approver FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS approval_stage_approver_visibility ON approval_stage_approver;
CREATE POLICY approval_stage_approver_visibility ON approval_stage_approver
  FOR SELECT
  USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = approval_stage_approver.work_item_id))
  );

-- UPDATE, unlike approval_stage's own (internal-only): DecideChangeRequestApproval
-- is the one write entity-service genuinely performs here, and its caller is
-- routinely an external customer approver (a "Customer Approval" stage --
-- see changeRequestApprovalStagePosition's own doc comment) deciding their
-- own pending approval. Must mirror change_request_update's condition so
-- that real flow keeps working.
DROP POLICY IF EXISTS approval_stage_approver_update ON approval_stage_approver;
CREATE POLICY approval_stage_approver_update ON approval_stage_approver
  FOR UPDATE USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = approval_stage_approver.work_item_id))
  )
  WITH CHECK (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = work_item_id))
  );
-- INSERT/DELETE are internal-only: entity-service never performs either
-- (rows are sync-service-inserted and FK-cascade-deleted only).
DROP POLICY IF EXISTS approval_stage_approver_write_internal_only ON approval_stage_approver;
CREATE POLICY approval_stage_approver_write_internal_only ON approval_stage_approver
  FOR INSERT WITH CHECK (current_setting('app.is_internal', true) = 'true');
DROP POLICY IF EXISTS approval_stage_approver_delete_internal_only ON approval_stage_approver;
CREATE POLICY approval_stage_approver_delete_internal_only ON approval_stage_approver
  FOR DELETE USING (current_setting('app.is_internal', true) = 'true');

COMMIT;
