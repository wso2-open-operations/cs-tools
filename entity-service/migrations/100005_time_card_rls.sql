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

-- Enforces project-membership visibility for time_card (and its child
-- time_card_approver) at the database layer. Fourth table after
-- case_escalation (0141), sla (0142), customer_call (0143).
--
-- time_card.case_id is NOT NULL (unlike every nullable join column in the
-- earlier three migrations), so is_project_member sees a real project_id
-- for every row whose case is a genuine case-like work item.
--
-- Every write policy uses the same is_internal OR is_project_member
-- condition as SELECT, not internal-only: today's Customer Portal BFF
-- persona matrix (PR #1668) already blocks external personas from
-- create/update on Time Cards entirely, but that is a BFF-side rule this
-- database has no notion of (see PR #1668's own "Enforcement boundary"
-- section) -- RLS's job here is the structural backstop for exactly the
-- scenario that PR's own description names as its acknowledged residual
-- risk: an external caller reaching entity-service directly, bypassing the
-- BFF's persona gate. If that happens, this policy still confines them to
-- their own project's time cards; it does not re-implement "customers
-- can't create time cards at all", which is a persona rule, not a
-- project-membership one, and stays the BFF's job.
--
-- One transaction, and every CREATE POLICY preceded by its own DROP POLICY
-- IF EXISTS -- see migration 100002's identical note.
BEGIN;

ALTER TABLE time_card ENABLE ROW LEVEL SECURITY;
ALTER TABLE time_card FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS time_card_visibility ON time_card;
CREATE POLICY time_card_visibility ON time_card
  FOR SELECT
  USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = time_card.case_id))
  );

DROP POLICY IF EXISTS time_card_write ON time_card;
CREATE POLICY time_card_write ON time_card
  FOR INSERT WITH CHECK (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = case_id))
  );
DROP POLICY IF EXISTS time_card_update ON time_card;
CREATE POLICY time_card_update ON time_card
  FOR UPDATE USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = time_card.case_id))
  )
  WITH CHECK (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = case_id))
  );
DROP POLICY IF EXISTS time_card_delete ON time_card;
CREATE POLICY time_card_delete ON time_card
  FOR DELETE USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = time_card.case_id))
  );

-- time_card_approver has no project_id of its own -- it reaches one via
-- time_card.case_id, so its policy re-derives the same membership check
-- through that join, mirroring case_escalation_notification_list's own
-- reasoning (migration 100002).
ALTER TABLE time_card_approver ENABLE ROW LEVEL SECURITY;
ALTER TABLE time_card_approver FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS time_card_approver_visibility ON time_card_approver;
CREATE POLICY time_card_approver_visibility ON time_card_approver
  FOR SELECT
  USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((
      SELECT wi.project_id
      FROM time_card tc
      JOIN work_item wi ON wi.id = tc.case_id
      WHERE tc.id = time_card_approver.time_card_id
    ))
  );

DROP POLICY IF EXISTS time_card_approver_write ON time_card_approver;
CREATE POLICY time_card_approver_write ON time_card_approver
  FOR INSERT WITH CHECK (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((
      SELECT wi.project_id
      FROM time_card tc
      JOIN work_item wi ON wi.id = tc.case_id
      WHERE tc.id = time_card_id
    ))
  );
DROP POLICY IF EXISTS time_card_approver_delete ON time_card_approver;
CREATE POLICY time_card_approver_delete ON time_card_approver
  FOR DELETE USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((
      SELECT wi.project_id
      FROM time_card tc
      JOIN work_item wi ON wi.id = tc.case_id
      WHERE tc.id = time_card_approver.time_card_id
    ))
  );
-- No UPDATE policy on time_card_approver: nothing in this codebase updates
-- an existing approver row (UpdateTimeCardFields deletes and re-inserts the
-- whole list instead), so FORCE + zero UPDATE policy correctly blocks a
-- command that should never run.

COMMIT;
