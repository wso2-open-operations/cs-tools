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

-- Standard change proposals: no CAB stage. The two approvals of the Standard
-- Change Authority are enough to make something a standard change, so a
-- proposal is now submitted straight to the authority (DRAFT ->
-- AUTHORITY_REVIEW) and CAB_REVIEW is no longer a state.
--
-- A proposal still in CAB_REVIEW goes back to DRAFT, for its proposer to submit
-- again (it then goes to the authority):
--   - the REQUESTED rows of its stage are CANCELLED, as a withdrawal cancels
--     them; the stage itself stays, with what was decided on it, and the
--     proposal keeps showing it;
--   - submitted_on is cleared: a draft has not been submitted. The stage it
--     keeps is what stops it being deleted (a proposal that was once before
--     anyone stays as the record); customer_locked stays set, as at submit.
-- Proposals in AUTHORITY_REVIEW (the CAB approved them under the old flow) and
-- decided ones are left as they are, with their CAB Approval stage. A decided
-- proposal is APPROVED, REJECTED, WITHDRAWN or CANCELLED, never CAB_REVIEW, so
-- once the UPDATE has run no row holds it, and standard_change_proposal_state_check
-- is recreated without it. The one-open-per-template index (0277) still names
-- CAB_REVIEW in its predicate; with no row able to hold it, it is left as it is.
--
-- Rollout: run 0287 IMMEDIATELY BEFORE the entity-service build without the CAB
-- stage is rolled out. A new pod running before 0287 no longer knows CAB_REVIEW,
-- so nobody can act on a proposal still in it: those proposals would be
-- stranded until 0287 runs. An old pod still running after 0287 works except for
-- one thing: its submit, which writes CAB_REVIEW, fails on the new check (an
-- error, nothing written) until that pod is replaced.
--
-- One transaction. Locks are taken in the application's order (lockProposal:
-- the work_item row, then the standard_change_proposal row), so a concurrent
-- proposal write waits for this migration instead of deadlocking with it: first
-- the work items of the proposals in CAB_REVIEW, then the table's ACCESS
-- EXCLUSIVE lock (dropping the check), so no proposal can be submitted to the CAB
-- by an old pod between the UPDATE and the new check. (A proposal an old pod
-- submits to the CAB in the moment between the two, and that another write then
-- holds, can still meet a deadlock; PostgreSQL then aborts one side, and either
-- the request or this migration is simply run again.)
--
-- standard_change_proposal (100026), approval_stage_approver (0145) and
-- work_item (100008) are row-level-secured with FORCE: a migrating role without
-- BYPASSRLS that is not flagged internal sees no row, and the UPDATEs would
-- write nothing without an error. So the session is flagged internal, as the
-- application's own writes are, and cleared afterwards, as 0193 and 0279 do
-- (session-level: a LOCAL setting would not outlive a statement in autocommit).
--
-- The authority group's description (0277) said it authorises "after the CAB";
-- it is reworded, only while it still reads as 0277 wrote it.
--
-- Safe to re-run: a second run finds no CAB_REVIEW proposal, recreates the same
-- check and leaves the description alone.

BEGIN;

SELECT set_config('app.is_internal', 'true', false);

SELECT w.id
  FROM work_item w
 WHERE w.id IN (SELECT p.id FROM standard_change_proposal p WHERE p.state = 'CAB_REVIEW')
 ORDER BY w.id
   FOR NO KEY UPDATE OF w;

ALTER TABLE standard_change_proposal DROP CONSTRAINT IF EXISTS standard_change_proposal_state_check;

WITH returned AS (
    UPDATE standard_change_proposal
       SET state = 'DRAFT', submitted_on = NULL
     WHERE state = 'CAB_REVIEW'
    RETURNING id
), cancelled AS (
    UPDATE approval_stage_approver a
       SET state = 'CANCELLED', updated_on = NOW(), updated_by = 'migration:0287_standard_change_no_cab_stage'
      FROM returned r
     WHERE a.work_item_id = r.id AND a.state = 'REQUESTED'
    RETURNING a.id
)
UPDATE work_item w
   SET updated_on = NOW(), updated_by = 'migration:0287_standard_change_no_cab_stage'
  FROM returned r
 WHERE w.id = r.id;

ALTER TABLE standard_change_proposal ADD CONSTRAINT standard_change_proposal_state_check
    CHECK (state IN ('DRAFT', 'AUTHORITY_REVIEW', 'APPROVED', 'REJECTED', 'WITHDRAWN', 'CANCELLED'));

UPDATE "group"
   SET description = 'Authorises standard change templates: two members must approve every new template, revision and retirement, and only a member may resume a suspended template or clear a review flag.',
       updated_on = NOW(), updated_by = 'migration'
 WHERE id = '00000000-0000-4000-8000-0000000005ca'::uuid
   AND description = 'Authorises standard change templates after the CAB: two members must approve every new template, revision and retirement, and only a member may resume a suspended template or clear a review flag.';

SELECT set_config('app.is_internal', '', false);

COMMIT;
