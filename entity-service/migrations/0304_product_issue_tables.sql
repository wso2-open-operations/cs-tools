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

-- Product Issues: internal work items, each backed by one GitHub issue in a
-- product team's repository, linked to the support case they were raised from,
-- and synced from GitHub.
--
--   product_issue                Extension of work_item (shared primary key, like
--                                change_request). The work item carries number
--                                (PI...), subject, project_id and the activity.
--   github_issue_link            The GitHub issue behind a work item, and the
--                                state last read from it. Deliberately NOT
--                                work_item.github_issue_number / github_owner /
--                                github_repository: setting those arms
--                                trg_github_outbound_comment (0206), which would
--                                forward every comment to that repository.
--   work_item_link               A typed edge between two work items. Only
--                                'AFFECTS' for now; extend the CHECK to add a kind.
--   work_item_status_event       One row per status change, for the history and
--                                for who-is-waiting-on-whom timings. dedupe_key
--                                makes a replayed webhook or reconcile a no-op.
--   comment_github_ref           Which GitHub comment a portal comment mirrors.
--   board_status_map             One global map from a project board column to
--                                the normalised status.
--   work_item_creation_request   Idempotency for the create endpoint.
--
-- Vocabularies are CHECK constraints, not Postgres enums, so adding a value is an
-- ordinary migration. Additive and idempotent: safe to re-run.

-- Number series. next_work_item_number('PRODUCT_ISSUE') -> 'PI0000001'.
CREATE SEQUENCE IF NOT EXISTS pi_number_seq START 1;

INSERT INTO number_series (series, digits, sequence_name) VALUES
    ('PI', 7, 'pi_number_seq')
ON CONFLICT (series) DO NOTHING;

-- Redefined from 0180 (the latest definition) with one added line; every other
-- type maps to the series it did before.
CREATE OR REPLACE FUNCTION work_item_number_series(p_type work_item_type_enum)
RETURNS TEXT AS $$
    SELECT CASE p_type
        WHEN 'CASE'                     THEN 'CS'
        WHEN 'SERVICE_REQUEST'          THEN 'CS'
        WHEN 'SECURITY_REPORT_ANALYSIS' THEN 'CS'
        WHEN 'ANNOUNCEMENT'             THEN 'CS'
        WHEN 'ENGAGEMENT'               THEN 'CS'
        WHEN 'INCIDENT'                 THEN 'INC'
        WHEN 'PROBLEM'                  THEN 'PRB'
        WHEN 'CHANGE_REQUEST'           THEN 'CHG'
        WHEN 'INCIDENT_TASK'            THEN 'TASK'
        WHEN 'CHANGE_TASK'              THEN 'CTASK'
        WHEN 'PROBLEM_TASK'             THEN 'PTASK'
        WHEN 'INCIDENT_ALERT_TASK'      THEN 'ICT'
        WHEN 'CONVERSATION'             THEN 'CHAT'
        WHEN 'PRODUCT_ISSUE'            THEN 'PI'
    END;
$$ LANGUAGE sql IMMUTABLE PARALLEL SAFE;

CREATE TABLE IF NOT EXISTS product_issue (
    id                UUID         PRIMARY KEY REFERENCES work_item(id) ON DELETE CASCADE,
    priority          VARCHAR(50),
    category          VARCHAR(100),
    opened_by_user_id UUID         REFERENCES "user"(id) ON DELETE SET NULL,
    opened_by_email   VARCHAR(255),
    abt_team          VARCHAR(255),
    public_issue_url  TEXT
);

CREATE TABLE IF NOT EXISTS github_issue_link (
    work_item_id         UUID         PRIMARY KEY REFERENCES work_item(id) ON DELETE CASCADE,
    owner                VARCHAR(255) NOT NULL,
    repository           VARCHAR(255) NOT NULL,
    issue_number         INTEGER      NOT NULL CHECK (issue_number > 0),
    node_id              TEXT,
    html_url             TEXT,
    github_state         VARCHAR(10)  NOT NULL DEFAULT 'OPEN' CHECK (github_state IN ('OPEN', 'CLOSED')),
    state_reason         VARCHAR(50),
    -- The project board column as GitHub spells it, and the normalised status
    -- it maps to (board_status_map). NULL: the issue is not on the board.
    board_status_raw     TEXT,
    board_status         VARCHAR(30)  CHECK (board_status IN
        ('OPEN', 'IN_PROGRESS', 'WAITING_ON_PRODUCT_TEAM', 'WAITING_ON_CS', 'REOPENED', 'RESOLVED', 'CLOSED', 'OTHER')),
    board_item_id        TEXT,
    board_project_number INTEGER,
    labels               JSONB        NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(labels) = 'array'),
    github_created_on    TIMESTAMPTZ,
    github_updated_on    TIMESTAMPTZ,
    github_closed_on     TIMESTAMPTZ,
    last_synced_on       TIMESTAMPTZ,
    -- ACTIVE: readable. INACCESSIBLE: the app can no longer read it (deleted,
    -- transferred out, permission lost). Extend the CHECK to add a state.
    link_state           VARCHAR(20)  NOT NULL DEFAULT 'ACTIVE'
                         CHECK (link_state IN ('ACTIVE', 'INACCESSIBLE', 'DELETED', 'TRANSFERRED')),
    created_on           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_on           TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- One work item per GitHub issue; GitHub treats owner and repository names as
-- case-insensitive.
CREATE UNIQUE INDEX IF NOT EXISTS github_issue_link_issue_key
    ON github_issue_link (lower(owner), lower(repository), issue_number);
CREATE UNIQUE INDEX IF NOT EXISTS github_issue_link_node_id_key
    ON github_issue_link (node_id);

CREATE TABLE IF NOT EXISTS work_item_link (
    id         UUID        PRIMARY KEY DEFAULT uuidv7(),
    from_id    UUID        NOT NULL REFERENCES work_item(id) ON DELETE CASCADE,
    to_id      UUID        NOT NULL REFERENCES work_item(id) ON DELETE CASCADE,
    kind       VARCHAR(30) NOT NULL CHECK (kind IN ('AFFECTS')),
    source     VARCHAR(30) NOT NULL CHECK (source IN ('CREATION', 'MANUAL', 'BACKFILL')),
    is_active  BOOLEAN     NOT NULL DEFAULT true,
    created_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by TEXT        NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by TEXT        NOT NULL,
    CONSTRAINT work_item_link_not_self CHECK (from_id <> to_id)
);

-- A work item AFFECTS at most one other, while the link is live. Replacing it
-- deactivates the old row (history stays) and adds a new one.
CREATE UNIQUE INDEX IF NOT EXISTS work_item_link_one_active_affects_key
    ON work_item_link (from_id) WHERE kind = 'AFFECTS' AND is_active;
CREATE INDEX IF NOT EXISTS idx_work_item_link_to_id ON work_item_link (to_id);

CREATE TABLE IF NOT EXISTS work_item_status_event (
    id           UUID         PRIMARY KEY DEFAULT uuidv7(),
    work_item_id UUID         NOT NULL REFERENCES work_item(id) ON DELETE CASCADE,
    from_status  VARCHAR(30)  CHECK (from_status IN
        ('OPEN', 'IN_PROGRESS', 'WAITING_ON_PRODUCT_TEAM', 'WAITING_ON_CS', 'REOPENED', 'RESOLVED', 'CLOSED', 'OTHER')),
    to_status    VARCHAR(30)  NOT NULL CHECK (to_status IN
        ('OPEN', 'IN_PROGRESS', 'WAITING_ON_PRODUCT_TEAM', 'WAITING_ON_CS', 'REOPENED', 'RESOLVED', 'CLOSED', 'OTHER')),
    waits_on     VARCHAR(20)  NOT NULL CHECK (waits_on IN ('CS', 'PRODUCT_TEAM', 'CUSTOMER', 'NOBODY')),
    occurred_at  TIMESTAMPTZ  NOT NULL,
    actor        TEXT,
    source       VARCHAR(20)  NOT NULL CHECK (source IN ('WEBHOOK', 'RECONCILE', 'BACKFILL', 'PORTAL')),
    -- Same key, same event: a replayed delivery or a reconcile pass inserts nothing.
    dedupe_key   TEXT         NOT NULL,
    raw_status   TEXT,
    created_on   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT work_item_status_event_dedupe_key UNIQUE (dedupe_key)
);

CREATE INDEX IF NOT EXISTS idx_work_item_status_event_work_item_occurred
    ON work_item_status_event (work_item_id, occurred_at);

CREATE TABLE IF NOT EXISTS comment_github_ref (
    comment_id        UUID        PRIMARY KEY REFERENCES comment(id) ON DELETE CASCADE,
    github_comment_id BIGINT,
    origin            VARCHAR(10) NOT NULL CHECK (origin IN ('PORTAL', 'GITHUB')),
    author_login      TEXT,
    github_updated_on TIMESTAMPTZ,
    deleted_on        TIMESTAMPTZ,
    CONSTRAINT comment_github_ref_github_comment_id_key UNIQUE (github_comment_id)
);

CREATE TABLE IF NOT EXISTS board_status_map (
    id                UUID        PRIMARY KEY DEFAULT uuidv7(),
    raw_status        TEXT        NOT NULL CHECK (btrim(raw_status) <> ''),
    normalized_status VARCHAR(30) NOT NULL CHECK (normalized_status IN
        ('OPEN', 'IN_PROGRESS', 'WAITING_ON_PRODUCT_TEAM', 'WAITING_ON_CS', 'REOPENED', 'RESOLVED', 'CLOSED', 'OTHER')),
    waits_on          VARCHAR(20) NOT NULL CHECK (waits_on IN ('CS', 'PRODUCT_TEAM', 'CUSTOMER', 'NOBODY')),
    -- false: the clock stops while the issue sits in this column.
    accrues_sla       BOOLEAN     NOT NULL,
    is_terminal       BOOLEAN     NOT NULL DEFAULT false,
    created_on        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_on        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Looked up by lower(btrim(raw_status)); a column name unknown to the map is
-- OTHER, and accrues, in code.
CREATE UNIQUE INDEX IF NOT EXISTS board_status_map_raw_status_key
    ON board_status_map (lower(btrim(raw_status)));

INSERT INTO board_status_map (raw_status, normalized_status, waits_on, accrues_sla, is_terminal) VALUES
    ('Open',        'OPEN',                    'PRODUCT_TEAM', true,  false),
    ('In Progress', 'IN_PROGRESS',             'PRODUCT_TEAM', true,  false),
    ('WOW',         'WAITING_ON_PRODUCT_TEAM', 'PRODUCT_TEAM', true,  false),
    ('WOC',         'WAITING_ON_CS',           'CS',           false, false),
    ('Resolved',    'RESOLVED',                'NOBODY',       false, true),
    ('Reopened',    'REOPENED',                'PRODUCT_TEAM', true,  false)
ON CONFLICT (lower(btrim(raw_status))) DO NOTHING;

-- Idempotency for creating a product issue. A repeated request with the same
-- (case_id, idempotency_key) returns the first result; if GitHub succeeded but
-- the database write did not, the retry finishes registration from the
-- github_* columns instead of creating a second issue. kind UNTRACKED_PATCH is
-- the option that opens a GitHub issue and creates no work item.
CREATE TABLE IF NOT EXISTS work_item_creation_request (
    id                 UUID         PRIMARY KEY DEFAULT uuidv7(),
    idempotency_key    TEXT         NOT NULL CHECK (btrim(idempotency_key) <> ''),
    case_id            UUID         NOT NULL REFERENCES work_item(id),
    request_hash       TEXT         NOT NULL,
    kind               VARCHAR(30)  NOT NULL CHECK (kind IN ('PRODUCT_ISSUE', 'UNTRACKED_PATCH')),
    status             VARCHAR(20)  NOT NULL DEFAULT 'PENDING'
                       CHECK (status IN ('PENDING', 'GITHUB_CREATED', 'COMPLETED', 'FAILED')),
    github_owner       VARCHAR(255),
    github_repository  VARCHAR(255),
    github_issue_number INTEGER,
    github_node_id     TEXT,
    github_html_url    TEXT,
    work_item_id       UUID         REFERENCES work_item(id) ON DELETE SET NULL,
    response           JSONB,
    error              TEXT,
    created_on         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_by         TEXT         NOT NULL,
    updated_on         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_by         TEXT         NOT NULL,
    CONSTRAINT work_item_creation_request_case_key UNIQUE (case_id, idempotency_key)
);

-- product_repo_mapping (0119): which project board an issue for the product is
-- tracked on, and whether the repository is internal (private to WSO2).
ALTER TABLE product_repo_mapping ADD COLUMN IF NOT EXISTS board_project_number INTEGER;
ALTER TABLE product_repo_mapping ADD COLUMN IF NOT EXISTS is_internal BOOLEAN NOT NULL DEFAULT false;
