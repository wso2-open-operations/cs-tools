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
-- again (it then goes to the authority); submitted_on is cleared, since a draft
-- has not been submitted, and customer_locked stays set, as at submit.
--
-- No proposal that is still open keeps a CAB stage. The old flow's CAB stage
-- (any stage not labelled "Standard Change Authority") is deleted, its approver
-- rows with it (ON DELETE CASCADE), from a proposal in DRAFT (the returned ones,
-- so each is a clean draft again and can be deleted) and from a proposal in
-- AUTHORITY_REVIEW that the CAB approved under the old flow, which shows only
-- the authority's stage afterwards and carries on there. The old flow is told
-- by the order of the stages, not by the label alone: it opened the authority's
-- stage when the CAB approved, so its CAB stage is older than an authority
-- stage on the same proposal. Since 0297 a submitted proposal's one stage is
-- labelled "CAB Approval" too, with no authority stage after it, and is never
-- touched (nor has a draft any stage under that code: only submit provisions
-- one, and nothing but these migrations returns a proposal to draft). Decided
-- proposals (APPROVED, REJECTED, WITHDRAWN, CANCELLED) keep their stages: that
-- is the record of who approved the template in force. A decided proposal is
-- never CAB_REVIEW, so once the UPDATE has run no row holds it, and
-- standard_change_proposal_state_check is recreated without it. The
-- one-open-per-template index (0287) still names CAB_REVIEW in its predicate;
-- with no row able to hold it, it is left as it is.
--
-- Every proposal this changes is reported as a WARNING in the migration's
-- output: its number and id, whether it went back to draft or only lost its CAB
-- stage, and each decision recorded on the deleted stage (who, what, when, the
-- comment), so its proposer can be told and no decision goes unseen. (Under the
-- code before the authority, a stage in CAB review has no decision: one
-- decision settled it.) Afterwards, while nobody has edited them since, they
-- are found with
--   SELECT w.number, p.id, p.state FROM standard_change_proposal p JOIN work_item w ON w.id = p.id
--    WHERE w.updated_by = 'migration:0295_standard_change_no_cab_stage';
--
-- Rollout: run 0295 IMMEDIATELY BEFORE the entity-service build without the CAB
-- stage is rolled out. A new pod running before 0295 no longer knows CAB_REVIEW,
-- so nobody can act on a proposal still in it: those proposals would be
-- stranded until 0295 runs. An old pod still running after 0295 works except for
-- one thing: its submit, which writes CAB_REVIEW, fails on the new check (an
-- error, nothing written) until that pod is replaced.
--
-- One transaction. Locks are taken in the application's order (lockProposal:
-- the work_item row, then the standard_change_proposal row), so a concurrent
-- proposal write waits for this migration instead of deadlocking with it: first
-- the work items of the proposals it changes, then the table's ACCESS
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
-- application's own writes are, and cleared afterwards, as 0193 and 0288 do
-- (session-level: a LOCAL setting would not outlive a statement in autocommit).
--
-- The authority group's description (0287) said it authorises "after the CAB";
-- it is reworded, only while it still reads as 0287 wrote it.
--
-- Safe to re-run, also once the code that follows 0297 has submitted and
-- decided proposals (a database that recorded this file under an earlier
-- number runs it again): no row is CAB_REVIEW any more, today's "CAB Approval"
-- stage has no authority stage after it and keeps its approvals, the same check
-- is recreated, and the description is left alone.

BEGIN;
SET LOCAL lock_timeout = '5s';

SELECT set_config('app.is_internal', 'true', false);

-- The work items of every proposal changed below: those in CAB review, and
-- those with an old-flow CAB stage.
SELECT w.id
  FROM work_item w
 WHERE w.id IN (SELECT p.id FROM standard_change_proposal p
                 WHERE p.state = 'CAB_REVIEW'
                    OR (p.state IN ('DRAFT', 'AUTHORITY_REVIEW')
                        AND EXISTS (SELECT 1 FROM approval_stage s
                                     WHERE s.work_item_id = p.id
                                       AND s.checkpoint_label IS DISTINCT FROM 'Standard Change Authority'
                                       AND (p.state = 'DRAFT'
                                            OR EXISTS (SELECT 1 FROM approval_stage a
                                                        WHERE a.work_item_id = p.id
                                                          AND a.checkpoint_label = 'Standard Change Authority'
                                                          AND a.created_on > s.created_on)))))
 ORDER BY w.id
   FOR NO KEY UPDATE OF w;

ALTER TABLE standard_change_proposal DROP CONSTRAINT IF EXISTS standard_change_proposal_state_check;

DO $$
DECLARE
    r record;
    n integer := 0;
BEGIN
    FOR r IN
        SELECT w.number, p.id, p.state,
               (SELECT string_agg(format('%s %s on %s%s', COALESCE(u.email, ap.approver_user_id::text), ap.state, ap.updated_on,
                                         COALESCE(': ' || ap.comments, '')),
                                  '; ' ORDER BY ap.updated_on)
                  FROM approval_stage s
                  JOIN approval_stage_approver ap ON ap.stage_id = s.id
                  LEFT JOIN "user" u ON u.id = ap.approver_user_id
                 WHERE s.work_item_id = p.id
                   AND s.checkpoint_label IS DISTINCT FROM 'Standard Change Authority'
                   AND (p.state IN ('CAB_REVIEW', 'DRAFT')
                        OR EXISTS (SELECT 1 FROM approval_stage a
                                    WHERE a.work_item_id = p.id
                                      AND a.checkpoint_label = 'Standard Change Authority'
                                      AND a.created_on > s.created_on))
                   AND ap.state IN ('APPROVED', 'REJECTED')) AS decisions
          FROM standard_change_proposal p
          JOIN work_item w ON w.id = p.id
         WHERE p.state = 'CAB_REVIEW'
            OR (p.state IN ('DRAFT', 'AUTHORITY_REVIEW')
                AND EXISTS (SELECT 1 FROM approval_stage s
                             WHERE s.work_item_id = p.id
                               AND s.checkpoint_label IS DISTINCT FROM 'Standard Change Authority'
                               AND (p.state = 'DRAFT'
                                    OR EXISTS (SELECT 1 FROM approval_stage a
                                                WHERE a.work_item_id = p.id
                                                  AND a.checkpoint_label = 'Standard Change Authority'
                                                  AND a.created_on > s.created_on))))
         ORDER BY w.number
    LOOP
        n := n + 1;
        RAISE WARNING '0295: standard change proposal % (%): %; decisions on its deleted CAB stage: %',
            r.number, r.id,
            CASE WHEN r.state = 'CAB_REVIEW' THEN 'returned from CAB review to draft'
                 ELSE 'stays ' || r.state || ', its CAB stage deleted' END,
            COALESCE(r.decisions, 'none');
    END LOOP;
    IF n > 0 THEN
        RAISE WARNING '0295 changed % standard change proposal(s), listed above; tell their proposers. Find them (until edited again) with: SELECT w.number, p.id, p.state FROM standard_change_proposal p JOIN work_item w ON w.id = p.id WHERE w.updated_by = ''migration:0295_standard_change_no_cab_stage'';', n;
    END IF;
END $$;

WITH returned AS (
    UPDATE standard_change_proposal
       SET state = 'DRAFT', submitted_on = NULL
     WHERE state = 'CAB_REVIEW'
    RETURNING id
)
UPDATE work_item w
   SET updated_on = NOW(), updated_by = 'migration:0295_standard_change_no_cab_stage'
 WHERE w.id IN (SELECT id FROM returned);

WITH dropped AS (
    DELETE FROM approval_stage s
     USING standard_change_proposal p
     WHERE s.work_item_id = p.id
       AND s.checkpoint_label IS DISTINCT FROM 'Standard Change Authority'
       AND (p.state = 'DRAFT'
            OR (p.state = 'AUTHORITY_REVIEW'
                AND EXISTS (SELECT 1 FROM approval_stage a
                             WHERE a.work_item_id = p.id
                               AND a.checkpoint_label = 'Standard Change Authority'
                               AND a.created_on > s.created_on)))
    RETURNING s.work_item_id
)
UPDATE work_item w
   SET updated_on = NOW(), updated_by = 'migration:0295_standard_change_no_cab_stage'
 WHERE w.id IN (SELECT work_item_id FROM dropped);

ALTER TABLE standard_change_proposal ADD CONSTRAINT standard_change_proposal_state_check
    CHECK (state IN ('DRAFT', 'AUTHORITY_REVIEW', 'APPROVED', 'REJECTED', 'WITHDRAWN', 'CANCELLED'));

UPDATE "group"
   SET description = 'Authorises standard change templates: two members must approve every new template, revision and retirement, and only a member may resume a suspended template or clear a review flag.',
       updated_on = NOW(), updated_by = 'migration'
 WHERE id = '00000000-0000-4000-8000-0000000005ca'::uuid
   AND description = 'Authorises standard change templates after the CAB: two members must approve every new template, revision and retirement, and only a member may resume a suspended template or clear a review flag.';

SELECT set_config('app.is_internal', '', false);

COMMIT;
