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

-- Standard change suggestions are computed on a schedule, not on every read
-- (POST /standard-change-suggestions/refresh, driven twice a day by
-- operations/csm-scheduled-tasks' standard_change_suggestions_refresh): reading
-- every completed change request with its plan text and comparing the plans is
-- far too much to do whenever the page opens. GET /standard-change-suggestions
-- reads the last published snapshot and never computes.
--
-- standard_change_suggestion_run: one refresh. It fills its snapshot rows a few
-- customers per call (each call must finish inside the request timeout) and is
-- published by its last call, which sets completed_on and deletes every other
-- run (their rows go with them, ON DELETE CASCADE) in one transaction. Readers
-- read the newest run with completed_on set, so they see the previous snapshot
-- until that commit and the whole new one after it, never a half-written one.
-- A run that never finishes (the job died) is never read, and is deleted by the
-- next publish (or by the next run started a day later).
--
-- standard_change_suggestion_run_customer: each customer a run has computed, with
-- how many candidate change requests it had. Saving a customer again (a call
-- sent twice) replaces its row and its suggestions; the run's totals are summed
-- from these at publish.
--
-- standard_change_suggestion_snapshot: one suggestion of one customer in a run:
-- the computed suggestion (JSON, as the API returns it, without the parts read
-- live: the open proposals and the dismissal), whether it met the criteria, and
-- every member's id (for the open-proposal check at read time). Not unique on
-- customer and key: one subject can split into two clusters whose plans differ.

CREATE TABLE IF NOT EXISTS standard_change_suggestion_run (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    started_on      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_on    TIMESTAMPTZ,
    criteria        JSONB NOT NULL,
    -- Totals, set when the run is published.
    customers       INTEGER NOT NULL DEFAULT 0,
    change_requests INTEGER NOT NULL DEFAULT 0,
    clusters        INTEGER NOT NULL DEFAULT 0,
    eligible        INTEGER NOT NULL DEFAULT 0
);

-- The reader's lookup: the newest published run.
CREATE INDEX IF NOT EXISTS idx_standard_change_suggestion_run_published
    ON standard_change_suggestion_run (completed_on DESC)
    WHERE completed_on IS NOT NULL;

CREATE TABLE IF NOT EXISTS standard_change_suggestion_run_customer (
    run_id          UUID NOT NULL REFERENCES standard_change_suggestion_run(id) ON DELETE CASCADE,
    account_id      UUID NOT NULL REFERENCES account(id) ON DELETE CASCADE,
    change_requests INTEGER NOT NULL,
    PRIMARY KEY (run_id, account_id)
);

CREATE TABLE IF NOT EXISTS standard_change_suggestion_snapshot (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    run_id         UUID NOT NULL REFERENCES standard_change_suggestion_run(id) ON DELETE CASCADE,
    account_id     UUID NOT NULL REFERENCES account(id) ON DELETE CASCADE,
    suggestion_key VARCHAR(512) NOT NULL,
    eligible       BOOLEAN NOT NULL,
    member_ids     UUID[] NOT NULL,
    suggestion     JSONB NOT NULL,
    computed_on    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_standard_change_suggestion_snapshot_run_account
    ON standard_change_suggestion_snapshot (run_id, account_id);
