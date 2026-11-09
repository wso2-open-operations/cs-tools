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

-- A standard change template (migration 0252) belongs to one customer: an
-- account. What the CAB pre-authorised for one customer's deployments is never
-- used for another's:
--
--   standard_change_template.account_id  the customer the template is for. A
--                                        change request is raised from it only
--                                        for a project of this account.
--   standard_change_proposal.account_id  the customer a proposal is for: NEW
--                                        names it (a draft may not have one
--                                        yet; it cannot be submitted without),
--                                        MODIFY / RETIRE carry their template's.
--
-- A template's name is unique among the live templates OF ITS ACCOUNT (two
-- customers may each have "Rotate the TLS certificate"), so the live-name index
-- of 0252 is replaced by one on (account_id, name).
--
--   standard_change_proposal.copied_from_proposal_id  the proposal a NEW
--                                        draft was copied from ("Copy into a new
--                                        proposal"). A copy is for the copied
--                                        proposal's customer, so its customer is
--                                        fixed like one started from a change
--                                        request; only a NEW draft started from
--                                        nothing may name or change its customer.
--
-- Every template must have an account. A row written before this migration is
-- given its account below when it can be told without guessing. Then:
--
--   - no template is left without one (every real deployment: 0252 and this
--     migration ship together, so no template exists before templates belong to
--     a customer): account_id becomes NOT NULL;
--   - some are (a local development database that ran 0252 alone and created
--     templates through it): a CHECK (account_id IS NOT NULL) NOT VALID instead,
--     so every write from now on needs an account while those rows stay. A NOT
--     VALID check still applies to any UPDATE of such a row, including the ON
--     DELETE SET NULL cascades of its owner, review change request and current
--     version, so those deletes fail until the row has an account. Rebuild the
--     catalogue with scripts/csm-compose/seed-standard-changes.sh --reset (or set
--     the rows' account_id) and run this migration again: it then sets NOT NULL
--     and drops the check.
--
-- Additive and idempotent: safe to re-run.

-- The four standard change tables, work_item and change_request are FORCE row-level
-- secured (100026, 100008, 100006). A migrating role that owns them without
-- BYPASSRLS would see none of their rows: the backfills below would update
-- nothing, and the "no template left without a customer" test would read true and
-- SET NOT NULL would then fail on the rows it could not see. So the session is
-- flagged internal, as 0193 does, and cleared again at the end. (Session-level,
-- not LOCAL: a migration is applied with plain autocommit.)
SELECT set_config('app.is_internal', 'true', false);

ALTER TABLE standard_change_template
    ADD COLUMN IF NOT EXISTS account_id UUID REFERENCES account(id) ON DELETE RESTRICT;
ALTER TABLE standard_change_proposal
    ADD COLUMN IF NOT EXISTS account_id UUID REFERENCES account(id) ON DELETE RESTRICT;
ALTER TABLE standard_change_proposal
    ADD COLUMN IF NOT EXISTS copied_from_proposal_id UUID REFERENCES standard_change_proposal(id) ON DELETE SET NULL;

-- customer_locked: the proposal's customer can no longer be changed. Set when the
-- customer comes from where the proposal started (a change request, a copy, the
-- template of a modification or retirement), once a copy names its customer, and
-- at submit. A flag of its own, not derived from source_change_request_id or
-- copied_from_proposal_id: those are cleared (ON DELETE SET NULL) when the record
-- they point at is deleted, and the customer must stay fixed regardless.
ALTER TABLE standard_change_proposal
    ADD COLUMN IF NOT EXISTS customer_locked BOOLEAN NOT NULL DEFAULT false;
UPDATE standard_change_proposal
   SET customer_locked = true
 WHERE NOT customer_locked
   AND (kind <> 'NEW' OR state <> 'DRAFT' OR source_change_request_id IS NOT NULL
        OR (copied_from_proposal_id IS NOT NULL AND account_id IS NOT NULL));

-- Backfill, only where the answer is unambiguous: the accounts of the projects
-- of the change requests the template is known by (the evidence and source of
-- the proposals that approved its versions, and the change requests raised from
-- it). Exactly one distinct account, or nothing is written.
WITH known AS (
    SELECT v.template_id, p.account_id
      FROM standard_change_template_version v
      JOIN standard_change_proposal_evidence e ON e.proposal_id = v.proposal_id
      JOIN work_item wi ON wi.id = e.change_request_id
      JOIN project p ON p.id = wi.project_id
    UNION ALL
    SELECT v.template_id, p.account_id
      FROM standard_change_template_version v
      JOIN standard_change_proposal sp ON sp.id = v.proposal_id
      JOIN work_item wi ON wi.id = sp.source_change_request_id
      JOIN project p ON p.id = wi.project_id
    UNION ALL
    SELECT v.template_id, p.account_id
      FROM change_request cr
      JOIN standard_change_template_version v ON v.id = cr.standard_change_template_version_id
      JOIN work_item wi ON wi.id = cr.id
      JOIN project p ON p.id = wi.project_id
), one AS (
    SELECT template_id, MIN(account_id::text)::uuid AS account_id
      FROM known
     WHERE account_id IS NOT NULL
     GROUP BY template_id
    HAVING COUNT(DISTINCT account_id) = 1
)
UPDATE standard_change_template t
   SET account_id = one.account_id
  FROM one
 WHERE t.id = one.template_id AND t.account_id IS NULL;

-- A proposal: its template's account (MODIFY / RETIRE, and an approved NEW),
-- else the one account of its source change request and evidence.
UPDATE standard_change_proposal sp
   SET account_id = t.account_id
  FROM standard_change_template t
 WHERE sp.template_id = t.id AND sp.account_id IS NULL AND t.account_id IS NOT NULL;

WITH known AS (
    SELECT e.proposal_id, p.account_id
      FROM standard_change_proposal_evidence e
      JOIN work_item wi ON wi.id = e.change_request_id
      JOIN project p ON p.id = wi.project_id
    UNION ALL
    SELECT sp.id, p.account_id
      FROM standard_change_proposal sp
      JOIN work_item wi ON wi.id = sp.source_change_request_id
      JOIN project p ON p.id = wi.project_id
), one AS (
    SELECT proposal_id, MIN(account_id::text)::uuid AS account_id
      FROM known
     WHERE account_id IS NOT NULL
     GROUP BY proposal_id
    HAVING COUNT(DISTINCT account_id) = 1
)
UPDATE standard_change_proposal sp
   SET account_id = one.account_id
  FROM one
 WHERE sp.id = one.proposal_id AND sp.account_id IS NULL;

-- One live template per name per customer, ignoring case and surrounding spaces.
DROP INDEX IF EXISTS standard_change_template_live_name_key;
CREATE UNIQUE INDEX IF NOT EXISTS standard_change_template_account_live_name_key
    ON standard_change_template (account_id, lower(btrim(name))) WHERE state <> 'RETIRED';

-- NOT NULL when every template has its customer; otherwise (a local development
-- database only, see above) the NOT VALID check, until a re-run finds none left.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM standard_change_template WHERE account_id IS NULL) THEN
        ALTER TABLE standard_change_template ALTER COLUMN account_id SET NOT NULL;
        ALTER TABLE standard_change_template DROP CONSTRAINT IF EXISTS standard_change_template_account_required;
    ELSE
        BEGIN
            ALTER TABLE standard_change_template
                ADD CONSTRAINT standard_change_template_account_required CHECK (account_id IS NOT NULL) NOT VALID;
        EXCEPTION WHEN duplicate_object THEN NULL;
        END;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_standard_change_template_account_id ON standard_change_template (account_id);
CREATE INDEX IF NOT EXISTS idx_standard_change_proposal_account_id ON standard_change_proposal (account_id);
CREATE INDEX IF NOT EXISTS idx_standard_change_proposal_copied_from ON standard_change_proposal (copied_from_proposal_id);

SELECT set_config('app.is_internal', '', false);
