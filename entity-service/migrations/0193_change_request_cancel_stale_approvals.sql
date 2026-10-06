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

-- DATA FIX: cancels approval requests that are still `requested` but can no
-- longer be acted on, so nobody is offered Approve / Reject for an approval
-- whose change request has moved on (reported: an internal reviewer could still
-- approve the Review stage of a change that was already Closed, and during
-- Customer Review, when the customer should be the one answering).
--
-- Why they exist: deciding the internal Review stage records the answer but
-- changes no state -- the change is moved on by hand (Review -> Customer Review /
-- Closed / Rollback / Canceled) -- and nothing cancelled the Review stage's other
-- approvers when it left Review, so their rows stayed `requested` for ever.
-- Cancelling a change did not cancel its internal stages' pending approvers
-- either. From this release on the application keeps this true by itself
-- (reconcileStaleApprovers runs on every state change, and a decision on a stage
-- the change has left is refused with a 409); this migration repairs the rows
-- written before that.
--
-- WHAT IT CHANGES (approval_stage_approver only; stages stay as a record, and the
-- change request itself is never touched). Every row whose state is `requested`:
--
--   (a) on a change request that is CLOSED, CANCELED or ROLLBACK -- the change is
--       final, nothing on it can be approved any more (whatever the stage);
--   (b) on a stage with an EXPLICIT approval_stage.checkpoint_label whose state
--       differs from the change request's current state (NULL / unknown state
--       excepted), by the stage -> decidable state map:
--
--           Peer Approval, Assess ........ ASSESS
--           CAB Approval, Authorize,
--           ECAB Approval ................ AUTHORIZE
--           Review ....................... REVIEW
--           Customer Approval ............ CUSTOMER_APPROVAL
--           Customer Review .............. CUSTOMER_REVIEW
--
--       (the same map as approvalStageDecidableState; "Assess" / "Authorize" are
--       the labels stages carried before the CAB flow, still recognised).
--
-- WHAT IT NEVER TOUCHES: rows that are not `requested` (approved, rejected,
-- cancelled, ... keep their history); rows of a stage with a NULL or unrecognised
-- checkpoint_label (a ServiceNow-synced stage) unless rule (a) applies; rows of a
-- stage whose label matches the change's current state (live approvals); rows of
-- a change with a NULL state.
--
-- Each cancelled row gets state = 'CANCELLED', updated_on = NOW() and
-- updated_by = 'migration:0193_change_request_cancel_stale_approvals', like the
-- application's own cancel helpers stamp the acting user. The approvals read
-- model then shows them as Cancelled and canDecide is false.
--
-- This is a DATA change, and it cancels approval requests that are currently
-- actionable (if wrong). It is safe to re-run: a second run finds no `requested`
-- row left that matches and changes nothing.
--
-- approval_stage_approver is row-level-secured with FORCE (migration 0145): a
-- migrating role that is a table owner without BYPASSRLS would see none of the
-- rows, so the session is flagged internal for the update, as the application's
-- own approval writes are, and cleared again afterwards. (Session-level, not
-- LOCAL: a migration is applied with plain autocommit, where a LOCAL setting
-- would be gone before the UPDATE runs.)

SELECT set_config('app.is_internal', 'true', false);

UPDATE approval_stage_approver asa
SET state = 'CANCELLED',
    updated_on = NOW(),
    updated_by = 'migration:0193_change_request_cancel_stale_approvals'
WHERE asa.state = 'REQUESTED'
  AND (
    -- (a) the change request is final
    EXISTS (
      SELECT 1 FROM change_request cr
      WHERE cr.id = asa.work_item_id
        AND cr.state IN ('CLOSED'::change_request_state_enum,
                         'CANCELED'::change_request_state_enum,
                         'ROLLBACK'::change_request_state_enum)
    )
    -- (b) a labelled stage whose state the change request is not in
    OR EXISTS (
      SELECT 1
      FROM approval_stage ast
      JOIN change_request cr ON cr.id = ast.work_item_id
      WHERE ast.id = asa.stage_id
        AND cr.state IS NOT NULL
        AND (CASE ast.checkpoint_label
               WHEN 'Peer Approval'      THEN 'ASSESS'
               WHEN 'Assess'             THEN 'ASSESS'
               WHEN 'CAB Approval'       THEN 'AUTHORIZE'
               WHEN 'Authorize'          THEN 'AUTHORIZE'
               WHEN 'ECAB Approval'      THEN 'AUTHORIZE'
               WHEN 'Review'             THEN 'REVIEW'
               WHEN 'Customer Approval'  THEN 'CUSTOMER_APPROVAL'
               WHEN 'Customer Review'    THEN 'CUSTOMER_REVIEW'
             END) <> cr.state::text
    )
  );

SELECT set_config('app.is_internal', '', false);
