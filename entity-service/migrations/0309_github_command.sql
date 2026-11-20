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


-- Product Issues, phase 3 (write-back): the outbound command queue.
--
--   github_command   One portal action on a product issue that must reach GitHub
--                    through the engineering entity: a comment, a board move
--                    (WOC -> WOW) or closing the issue. The portal writes the row;
--                    a worker delivers it and records the outcome here. The stored
--                    github_issue_link state is never changed by a command: it moves
--                    only when GitHub confirms through the webhook / reconciliation.
--
-- Vocabularies are CHECK constraints, as in 0304. Additive and idempotent: safe to
-- re-run. Not registered for audit (the table churns on every delivery attempt).
-- Row-level security is in 100031_github_command_rls.sql. Like every table here the
-- migration grants nothing: the deployment grants the application role.
CREATE TABLE IF NOT EXISTS github_command (
    id               UUID        PRIMARY KEY DEFAULT uuidv7(),
    work_item_id     UUID        NOT NULL REFERENCES work_item(id) ON DELETE CASCADE,
    kind             TEXT        NOT NULL CHECK (kind IN ('ADD_COMMENT', 'SET_BOARD_STATUS', 'SET_STATE')),
    -- ADD_COMMENT {"body": "<exact text sent to GitHub>"};
    -- SET_BOARD_STATUS {"status": "WAITING_ON_PRODUCT_TEAM", "rawStatus": "WOW"};
    -- SET_STATE {"state": "closed", "stateReason": "completed"}.
    payload          JSONB       NOT NULL,
    -- ADD_COMMENT only: the portal comment this command delivers.
    comment_id       UUID        REFERENCES comment(id) ON DELETE SET NULL,
    status           TEXT        NOT NULL DEFAULT 'PENDING'
                     CHECK (status IN ('PENDING', 'DELIVERED', 'FAILED', 'CANCELLED')),
    attempts         INTEGER     NOT NULL DEFAULT 0,
    -- Set by the worker before its first call to the engineering entity and never
    -- reset (not by the retry route): from then on the action may already have
    -- happened on GitHub, so a comment is looked up before it is posted again.
    delivery_attempted BOOLEAN   NOT NULL DEFAULT FALSE,
    next_attempt_on  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- A classification of the failure, never a GitHub response body.
    last_error       TEXT        CHECK (char_length(last_error) <= 300),
    -- Last X-GitHub-Status, else the engineering entity's own status.
    github_status    INTEGER,
    idempotency_key  TEXT,
    -- The caller's email, as the other audit columns.
    requested_by     TEXT        NOT NULL,
    created_on       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_on       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    delivered_on     TIMESTAMPTZ,
    -- Start of the 24 h retry window; the retry route resets it.
    window_started_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT github_command_comment_only_for_add_comment
        CHECK (comment_id IS NULL OR kind = 'ADD_COMMENT')
);

-- One open move / close per issue; comments may queue up freely.
CREATE UNIQUE INDEX IF NOT EXISTS github_command_one_pending_per_kind_key
    ON github_command (work_item_id, kind)
    WHERE status = 'PENDING' AND kind <> 'ADD_COMMENT';

-- The same Idempotency-Key on the same item is the same command.
CREATE UNIQUE INDEX IF NOT EXISTS github_command_idempotency_key
    ON github_command (work_item_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

-- What the worker polls.
CREATE INDEX IF NOT EXISTS idx_github_command_due
    ON github_command (next_attempt_on)
    WHERE status = 'PENDING';

-- The detail view lists an item's commands.
CREATE INDEX IF NOT EXISTS idx_github_command_work_item
    ON github_command (work_item_id, created_on DESC);
