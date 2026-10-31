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

-- Case auto-closure: reminders and automatic closure of tickets that sit in
-- Awaiting Info / Solution Proposed with no activity.
--
-- The workflow position already lives in autoclosure_step / autoclosure_state_on
-- (migrations 0023/0024), and a customer-requested hold is already stored there
-- as step ON_HOLD with the hold-until day in autoclosure_state_on (see
-- setAutocloseHoldTx). This migration adds only what the workflow was missing:
--
--   autoclose_enabled        per-ticket opt-out. Defaults to TRUE, so every
--                            existing ticket stays eligible; the workflow only
--                            reads it on the two tables it applies to.
--   case_autoclosure_event   append-only audit of every automated action, which
--                            is also the evidence behind "no duplicate comment".
--   autoclosure_reset trigger  restarts the workflow (step DEFAULT, time now)
--                            whenever a parent ticket ENTERS AWAITING_INFO,
--                            SOLUTION_PROPOSED, WAITING_ON_WSO2 or REOPENED
--                            from a different state. Entering a waiting state
--                            is a fresh request to the customer, so it must not
--                            inherit the clock of an earlier one (Awaiting Info
--                            -> Work in Progress -> Awaiting Info with a new
--                            question). One exception: a customer hold
--                            (step ON_HOLD) survives entering Awaiting Info or
--                            Solution Proposed, because the customer asked for
--                            the ticket not to be closed; moving to Waiting on
--                            WSO2 or Reopened still ends it, as a customer
--                            reply does. A trigger rather than service code so
--                            every writer (portal, API, sync) restarts it
--                            identically.

ALTER TABLE "case" ADD COLUMN IF NOT EXISTS autoclose_enabled BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE security_report_analysis ADD COLUMN IF NOT EXISTS autoclose_enabled BOOLEAN NOT NULL DEFAULT TRUE;

-- Candidate scans only ever look at the two waiting states.
CREATE INDEX IF NOT EXISTS idx_case_autoclosure_candidates
    ON "case" (state, autoclosure_step)
    WHERE state IN ('AWAITING_INFO', 'SOLUTION_PROPOSED');
CREATE INDEX IF NOT EXISTS idx_sra_autoclosure_candidates
    ON security_report_analysis (state, autoclosure_step)
    WHERE state IN ('AWAITING_INFO', 'SOLUTION_PROPOSED');

CREATE TABLE IF NOT EXISTS case_autoclosure_event (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    work_item_id UUID NOT NULL REFERENCES work_item(id) ON DELETE CASCADE,
    -- The stage, spelled as the code writes it and as CountAdvancedSince
    -- counts it: release_hold, first_reminder, second_reminder or close.
    -- (Not the step names FIRST_COMMENT / SECOND_COMMENT, which are the
    -- values of autoclosure_step.)
    stage        VARCHAR(30) NOT NULL,
    from_step    VARCHAR(50),
    to_step      VARCHAR(50),
    -- ADVANCED (done; the only outcome the daily limit counts), FAILED (the
    -- action was claimed and then rolled back for a reason that will repeat:
    -- a refusal; counts toward the failure cap), FAILED_TRANSIENT (the same,
    -- for a cancelled request, timeout or outage; ignored by the cap), or
    -- DRY_RUN (a dry run found the ticket due and changed nothing).
    outcome      VARCHAR(20) NOT NULL,
    detail       TEXT,
    created_on   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_case_autoclosure_event_work_item
    ON case_autoclosure_event (work_item_id, created_on DESC);

-- Restart the workflow when a ticket enters a state that starts (or restarts)
-- a request to the customer. Parent-level tickets only: a child
-- (work_item.parent_id set) never has its own workflow.
CREATE OR REPLACE FUNCTION autoclosure_reset() RETURNS trigger AS $$
BEGIN
    IF NEW.state IS DISTINCT FROM OLD.state
       AND (
            NEW.state::TEXT IN ('WAITING_ON_WSO2', 'REOPENED')
            OR (NEW.state::TEXT IN ('AWAITING_INFO', 'SOLUTION_PROPOSED')
                AND COALESCE(OLD.autoclosure_step, '') <> 'ON_HOLD')
       )
       AND NOT EXISTS (SELECT 1 FROM work_item w WHERE w.id = NEW.id AND w.parent_id IS NOT NULL)
    THEN
        NEW.autoclosure_step := 'DEFAULT';
        NEW.autoclosure_state_on := NOW();
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_case_autoclosure_reset ON "case";
CREATE TRIGGER trg_case_autoclosure_reset
    BEFORE UPDATE OF state ON "case"
    FOR EACH ROW EXECUTE FUNCTION autoclosure_reset();

DROP TRIGGER IF EXISTS trg_sra_autoclosure_reset ON security_report_analysis;
CREATE TRIGGER trg_sra_autoclosure_reset
    BEFORE UPDATE OF state ON security_report_analysis
    FOR EACH ROW EXECUTE FUNCTION autoclosure_reset();
