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

-- Standard changes, the Standard Change Authority and the rest of the template
-- gate.
--
-- A standard change skips the CAB on every use, so making something one is the
-- risky step. A proposal (new template, revision, retirement) is approved by the
-- "Standard Change Authority" group (the heads of SaaS SRE, of each SaaS ABT and
-- of SRE), of whom two different people must approve; the proposer may not.
-- (This migration was written for two stages, the CAB and then the authority,
-- and so keeps CAB_REVIEW in the state check below; 0295 removed the CAB stage,
-- returned the proposals still in CAB review to draft and recreated the check
-- without it.)
--
--   "group" "Standard Change Authority"    Created empty, like the CAB group in
--                                           0188: who sits on it is an operator's
--                                           decision (team_member.group_id or
--                                           group_member rows). A proposal cannot
--                                           be submitted until it has at least two
--                                           eligible members besides the proposer.
--   approval_stage.required_approvals      How many different approvers a stage
--                                           needs. 1 (every existing stage, and
--                                           every change request stage) keeps the
--                                           "first decision settles it" rule.
--   standard_change_proposal.state         + AUTHORITY_REVIEW (waiting on the
--                                           authority), + CANCELLED (an
--                                           authority member ended a proposal its
--                                           proposer abandoned). The one-open-per-
--                                           template index counts AUTHORITY_REVIEW.
--   summary, priority, deployment_types    Template content, on the proposal and
--                                           the version (0252's shared column set).
--                                           summary: what the template is for, when
--                                           to use it, what it does not cover.
--                                           priority: the change request's priority,
--                                           NULL = chosen per change. deployment_types:
--                                           the deployment types (domain spelling, as
--                                           category is) a change request raised from
--                                           it must be for, NULL = any deployment.
--   standard_change_template_event         Append-only history of what happens to a
--                                           template outside a proposal: suspended,
--                                           resumed, flagged for review, review
--                                           cleared. The template row keeps only the
--                                           current values.
--
-- Additive and idempotent: safe to re-run.

INSERT INTO "group" (id, created_on, updated_on, created_by, updated_by, name, description, is_active)
SELECT '00000000-0000-4000-8000-0000000005ca'::uuid, now(), now(), 'migration', 'migration',
       'Standard Change Authority',
       'Authorises standard change templates after the CAB: two members must approve every new template, revision and retirement, and only a member may resume a suspended template or clear a review flag.',
       true
WHERE NOT EXISTS (SELECT 1 FROM "group" WHERE name = 'Standard Change Authority')
ON CONFLICT (id) DO NOTHING;

-- Every ALTER below waits at most 5s for a lock; a busy table fails the run
-- instead of queueing every change request approval behind it. Reset at the end.
SET lock_timeout = '5s';

ALTER TABLE approval_stage
    ADD COLUMN IF NOT EXISTS required_approvals SMALLINT NOT NULL DEFAULT 1 CHECK (required_approvals >= 1);

ALTER TABLE standard_change_proposal DROP CONSTRAINT IF EXISTS standard_change_proposal_state_check;
ALTER TABLE standard_change_proposal ADD CONSTRAINT standard_change_proposal_state_check
    CHECK (state IN ('DRAFT', 'CAB_REVIEW', 'AUTHORITY_REVIEW', 'APPROVED', 'REJECTED', 'WITHDRAWN', 'CANCELLED'));

DROP INDEX IF EXISTS standard_change_proposal_one_open_per_template;
CREATE UNIQUE INDEX IF NOT EXISTS standard_change_proposal_one_open_per_template
    ON standard_change_proposal (template_id)
    WHERE template_id IS NOT NULL AND kind IN ('MODIFY', 'RETIRE') AND state IN ('DRAFT', 'CAB_REVIEW', 'AUTHORITY_REVIEW');

ALTER TABLE standard_change_proposal ADD COLUMN IF NOT EXISTS summary TEXT;
ALTER TABLE standard_change_proposal ADD COLUMN IF NOT EXISTS priority VARCHAR(20);
ALTER TABLE standard_change_proposal ADD COLUMN IF NOT EXISTS deployment_types TEXT[];
ALTER TABLE standard_change_template_version ADD COLUMN IF NOT EXISTS summary TEXT;
ALTER TABLE standard_change_template_version ADD COLUMN IF NOT EXISTS priority VARCHAR(20);
ALTER TABLE standard_change_template_version ADD COLUMN IF NOT EXISTS deployment_types TEXT[];

CREATE TABLE IF NOT EXISTS standard_change_template_event (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id       UUID        NOT NULL REFERENCES standard_change_template(id) ON DELETE CASCADE,
    action            VARCHAR(20) NOT NULL CHECK (action IN ('SUSPENDED', 'RESUMED', 'REVIEW_FLAGGED', 'REVIEW_CLEARED')),
    -- Why: the suspension reason, the resume or clear note, the flag's reason.
    note              TEXT,
    -- REVIEW_FLAGGED: the change request that raised the flag.
    change_request_id UUID        REFERENCES change_request(id) ON DELETE SET NULL,
    actor             TEXT        NOT NULL,
    created_on        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_standard_change_template_event_template
    ON standard_change_template_event (template_id, created_on DESC);

RESET lock_timeout;
