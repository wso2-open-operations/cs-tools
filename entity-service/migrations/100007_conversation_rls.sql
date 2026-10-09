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

-- Enforces project-membership visibility for conversation at the database
-- layer. Sixth table after case_escalation (0141), sla (0142),
-- customer_call (0143), time_card (0144), change_request (0145).
--
-- conversation shares its primary key with work_item (conversation.id IS
-- work_item.id), same shape as change_request -- membership is derived
-- through that shared id, and (learned the hard way in migration 100006)
-- every correlated subquery below writes conversation.id explicitly, never
-- a bare id, since work_item also has a column literally named id and an
-- unqualified reference inside a "FROM work_item wi" subquery resolves to
-- wi.id itself rather than correlating back to the outer row.
--
-- One transaction, and every CREATE POLICY preceded by its own DROP POLICY
-- IF EXISTS -- see migration 100002's identical note.
BEGIN;

ALTER TABLE conversation ENABLE ROW LEVEL SECURITY;
ALTER TABLE conversation FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS conversation_visibility ON conversation;
CREATE POLICY conversation_visibility ON conversation
  FOR SELECT
  USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = conversation.id))
  );

-- UpdateConversation (the only write conversation.go performs) is a plain
-- customer-facing PATCH -- closing/reopening a conversation -- with no
-- internal-only gate anywhere in the service or handler layer, same
-- "thin passthrough" shape as change_request's PatchChangeRequest. USING and
-- WITH CHECK share the same condition: this UPDATE only ever touches
-- conversation's own `state` column, never work_item.project_id, so there is
-- no "moved to another project" case to additionally guard against.
DROP POLICY IF EXISTS conversation_update ON conversation;
CREATE POLICY conversation_update ON conversation
  FOR UPDATE USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = conversation.id))
  )
  WITH CHECK (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = conversation.id))
  );

-- INSERT is internal-only: CreateConversation has no Postgres implementation
-- at all today (work_item.number has no generation strategy -- see
-- ConversationRepository's own package doc comment), so nothing currently
-- writes a new conversation row. Still given an explicit internal-only
-- policy rather than left at zero, matching case_escalation's own
-- precedent (migration 100002): FORCE plus zero INSERT policies would also
-- block a future internal/admin tooling need, not just an external one.
DROP POLICY IF EXISTS conversation_write_internal_only ON conversation;
CREATE POLICY conversation_write_internal_only ON conversation
  FOR INSERT WITH CHECK (current_setting('app.is_internal', true) = 'true');
-- No DELETE policy: nothing in this codebase deletes a conversation row.

COMMIT;
