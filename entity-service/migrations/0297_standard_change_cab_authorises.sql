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

-- Standard change proposals: the CAB authorises them (2026-11-10). The CAB
-- (the "CAB Approval" group, the one that approves normal change requests)
-- replaces the "Standard Change Authority" group (0287): a proposal (new
-- template, revision, retirement) is submitted to one stage, labelled "CAB
-- Approval", of which two different CAB members, never the proposer, must
-- approve; one rejection ends it. This is still the single stage 0295 left: no
-- CAB stage is put back in front of another, and CAB_REVIEW stays gone.
--
-- A proposal still waiting on the authority (AUTHORITY_REVIEW, with no "CAB
-- Approval" stage) goes back to DRAFT, for its proposer to submit again (it then
-- goes to the CAB), as 0295 did for the proposals in CAB review: submitted_on is
-- cleared, since a draft has not been submitted, customer_locked stays set, as
-- at submit, and the authority's open stage is deleted (its approver rows go with
-- it, ON DELETE CASCADE, an approval already given included), so the returned
-- draft is a clean draft again (it can be deleted). The stored state keeps its
-- name: AUTHORITY_REVIEW now means "awaiting CAB approval".
--
-- Which databases this can touch: only one that ran an earlier build of this
-- change, whose code submitted proposals to the authority. 0287 adds
-- AUTHORITY_REVIEW and the code before it writes CAB_REVIEW (which 0295 returns
-- to draft), so where 0287 to 0297 run together, before the new build, no
-- proposal is in AUTHORITY_REVIEW and this returns none. Where it does return
-- one, every proposal is reported as a WARNING in the migration's output: its
-- number and id, and each decision recorded on the deleted stage (who, what,
-- when, the comment), so its proposer can be told and an approval given there
-- can be recovered from the output. The stages are not kept on the draft
-- instead: the code reads every stage of a proposal (its page lists them, a
-- draft with one cannot be deleted, and a decision goes to the latest), so the
-- authority's stage would show, still asking its members, on a draft. Afterwards,
-- while nobody has edited them since, the returned drafts are found with
--   SELECT w.number, p.id FROM standard_change_proposal p JOIN work_item w ON w.id = p.id
--    WHERE w.updated_by = 'migration:0297_standard_change_cab_authorises';
--
-- Decided proposals (APPROVED, REJECTED, WITHDRAWN, CANCELLED) keep their
-- "Standard Change Authority" stages: that is the record of who approved the
-- template in force. So the group row is kept (those stages point at it); only
-- its description is reworded to say it is no longer used, and only while it
-- still reads as 0295 left it, or as 0287 wrote it (a database that applied an
-- earlier draft of 0295, which did not reword it); an operator's own wording is
-- left alone.
--
-- Rollout: run 0297 IMMEDIATELY BEFORE the entity-service build that asks the
-- CAB is rolled out. A proposal an old pod submits after 0297 has run asks the
-- authority's members; it still works (a decision is taken on the proposal's
-- latest stage, by the people that stage asked), it is just not returned to
-- draft.
--
-- One transaction. Locks are taken in the application's order (lockProposal:
-- the work_item row, then the standard_change_proposal row), so a concurrent
-- proposal write waits for this migration instead of deadlocking with it.
-- lock_timeout bounds the wait for a write that holds one of them.
--
-- standard_change_proposal (100026), approval_stage_approver (0145) and
-- work_item (100008) are row-level-secured with FORCE: a migrating role without
-- BYPASSRLS that is not flagged internal sees no row, and the statements would
-- write nothing without an error. So the session is flagged internal, as the
-- application's own writes are, and cleared afterwards, as 0295 does
-- (session-level: a LOCAL setting would not outlive a statement in autocommit).
--
-- Safe to re-run, also once the new build has submitted and decided proposals
-- (a database that recorded this file under an earlier number runs it again): a
-- proposal submitted to the CAB has a "CAB Approval" stage and is never
-- selected, nor is a draft or a decided one, so a second run finds nothing to
-- return and leaves the reworded description alone. (The one thing a re-run
-- can find is a proposal an old pod sent to the authority after the first run,
-- during the rollout; it is returned to draft as the first run would have, and
-- reported.)

BEGIN;
SET LOCAL lock_timeout = '5s';

SELECT set_config('app.is_internal', 'true', false);

SELECT w.id
  FROM work_item w
 WHERE w.id IN (SELECT p.id FROM standard_change_proposal p
                 WHERE p.state = 'AUTHORITY_REVIEW'
                   AND NOT EXISTS (SELECT 1 FROM approval_stage s
                                    WHERE s.work_item_id = p.id AND s.checkpoint_label = 'CAB Approval'))
 ORDER BY w.id
   FOR NO KEY UPDATE OF w;

SELECT p.id
  FROM standard_change_proposal p
 WHERE p.state = 'AUTHORITY_REVIEW'
   AND NOT EXISTS (SELECT 1 FROM approval_stage s
                    WHERE s.work_item_id = p.id AND s.checkpoint_label = 'CAB Approval')
 ORDER BY p.id
   FOR UPDATE OF p;

DO $$
DECLARE
    r record;
    n integer := 0;
BEGIN
    FOR r IN
        SELECT w.number, p.id,
               (SELECT string_agg(format('%s %s on %s%s', COALESCE(u.email, ap.approver_user_id::text), ap.state, ap.updated_on,
                                         COALESCE(': ' || ap.comments, '')),
                                  '; ' ORDER BY ap.updated_on)
                  FROM approval_stage s
                  JOIN approval_stage_approver ap ON ap.stage_id = s.id
                  LEFT JOIN "user" u ON u.id = ap.approver_user_id
                 WHERE s.work_item_id = p.id
                   AND ap.state IN ('APPROVED', 'REJECTED')) AS decisions
          FROM standard_change_proposal p
          JOIN work_item w ON w.id = p.id
         WHERE p.state = 'AUTHORITY_REVIEW'
           AND NOT EXISTS (SELECT 1 FROM approval_stage s
                            WHERE s.work_item_id = p.id AND s.checkpoint_label = 'CAB Approval')
         ORDER BY w.number
    LOOP
        n := n + 1;
        RAISE WARNING '0297: standard change proposal % (%): returned from the authority to draft; decisions on its deleted stages: %',
            r.number, r.id, COALESCE(r.decisions, 'none');
    END LOOP;
    IF n > 0 THEN
        RAISE WARNING '0297 returned % standard change proposal(s) to draft, listed above; tell their proposers. Find them (until edited again) with: SELECT w.number, p.id FROM standard_change_proposal p JOIN work_item w ON w.id = p.id WHERE w.updated_by = ''migration:0297_standard_change_cab_authorises'';', n;
    END IF;
END $$;

WITH returned AS (
    UPDATE standard_change_proposal p
       SET state = 'DRAFT', submitted_on = NULL
     WHERE p.state = 'AUTHORITY_REVIEW'
       AND NOT EXISTS (SELECT 1 FROM approval_stage s
                        WHERE s.work_item_id = p.id AND s.checkpoint_label = 'CAB Approval')
    RETURNING p.id
), dropped AS (
    DELETE FROM approval_stage s
     USING returned r
     WHERE s.work_item_id = r.id
    RETURNING s.work_item_id
)
UPDATE work_item w
   SET updated_on = NOW(), updated_by = 'migration:0297_standard_change_cab_authorises'
 WHERE w.id IN (SELECT id FROM returned)
    OR w.id IN (SELECT work_item_id FROM dropped);

UPDATE "group"
   SET description = 'No longer used: until 2026-11-10 this group authorised standard change templates. The CAB (the "CAB Approval" group) does now: two of its members approve every new template, revision and retirement. Kept because the proposals it decided name it.',
       updated_on = NOW(), updated_by = 'migration'
 WHERE id = '00000000-0000-4000-8000-0000000005ca'::uuid
   AND description IN ('Authorises standard change templates: two members must approve every new template, revision and retirement, and only a member may resume a suspended template or clear a review flag.',
                       'Authorises standard change templates after the CAB: two members must approve every new template, revision and retirement, and only a member may resume a suspended template or clear a review flag.');

SELECT set_config('app.is_internal', '', false);

COMMIT;
