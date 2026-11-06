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

-- Standard change templates (ITIL 4 "standard change models").
--
-- A standard change is pre-authorised: the CAB assesses its risk once, when the
-- template is created or changed, and never again for each use. So:
--
--   standard_change_template          STD number. Name, state, owner, current
--                                     version and the "review required" flag a
--                                     failed use raises.
--   standard_change_template_version  The authorised content: immutable once
--                                     written, one row per CAB-approved version.
--   standard_change_proposal          SCP number. A work item (migration 0251)
--                                     proposing a NEW template, a MODIFY of one
--                                     or a RETIRE. The CAB decides it through an
--                                     approval_stage. A rejected or withdrawn
--                                     proposal is never deleted: it is the log.
--   standard_change_proposal_evidence The completed change requests a proposal
--                                     cites as evidence that the change is
--                                     repeatable.
--
-- The template content is the same column set on the proposal (a draft, editable
-- while the proposal is a DRAFT) and on the version (frozen). Placeholders
-- written {{key}} in the text fields are declared in "parameters", a JSON array
-- of {key, label, type, required, helpText, options}; a change request raised
-- from a version stores the values it was given (change_request columns below).
--
-- category / impact / risk hold the domain spelling (lower case, as the API
-- reads them), not change_request's upper-case enum labels: they are copied
-- into a change request through the same create path a client uses.
--
-- Additive and idempotent: safe to re-run.

CREATE SEQUENCE IF NOT EXISTS std_number_seq START 1;
CREATE SEQUENCE IF NOT EXISTS scp_number_seq START 1;

INSERT INTO number_series (series, digits, sequence_name) VALUES
    ('STD', 7, 'std_number_seq'),
    ('SCP', 7, 'scp_number_seq')
ON CONFLICT (series) DO NOTHING;

CREATE TABLE IF NOT EXISTS standard_change_template (
    id                       UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    number                   TEXT        NOT NULL DEFAULT next_series_number('STD'),
    name                     TEXT        NOT NULL CHECK (btrim(name) <> '' AND char_length(name) <= 255),
    -- ACTIVE: can be used. SUSPENDED: kept, not usable until resumed (a CAB
    -- member's call, e.g. after a failed use). RETIRED: final, reached only
    -- through an approved RETIRE proposal.
    state                    VARCHAR(20) NOT NULL DEFAULT 'ACTIVE' CHECK (state IN ('ACTIVE', 'SUSPENDED', 'RETIRED')),
    current_version_id       UUID,
    owner_user_id            UUID        REFERENCES "user"(id) ON DELETE SET NULL,
    -- Raised when a change request raised from this template is rolled back or
    -- records a deviation from the implementation plan. It does not stop the
    -- template being used: the owner and the CAB decide what to do (suspend,
    -- propose a modification, or clear it).
    review_required          BOOLEAN     NOT NULL DEFAULT false,
    review_required_reason   TEXT,
    review_required_on       TIMESTAMPTZ,
    review_required_change_request_id UUID REFERENCES change_request(id) ON DELETE SET NULL,
    created_on               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by               TEXT        NOT NULL,
    updated_on               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by               TEXT        NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS standard_change_template_number_key ON standard_change_template (number);
-- One live template per name, ignoring case; a retired name may be reused.
CREATE UNIQUE INDEX IF NOT EXISTS standard_change_template_live_name_key
    ON standard_change_template (lower(btrim(name))) WHERE state <> 'RETIRED';

CREATE TABLE IF NOT EXISTS standard_change_template_version (
    id                               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id                      UUID        NOT NULL REFERENCES standard_change_template(id) ON DELETE CASCADE,
    version                          INTEGER     NOT NULL CHECK (version >= 1),
    -- The proposal this version was approved from (NULL only if that work item
    -- is ever removed by hand).
    proposal_id                      UUID        REFERENCES work_item(id) ON DELETE SET NULL,
    subject                          TEXT        NOT NULL CHECK (btrim(subject) <> ''),
    description                      TEXT,
    justification                    TEXT,
    implementation_plan              TEXT        NOT NULL,
    risk_impact_analysis             TEXT,
    backout_plan                     TEXT        NOT NULL,
    test_plan                        TEXT        NOT NULL,
    category                         VARCHAR(50),
    impact                           VARCHAR(20),
    risk                             VARCHAR(20),
    rollback_duration_text           TEXT,
    is_planning_visible_to_customers BOOLEAN     NOT NULL DEFAULT false,
    customer_approval_required       BOOLEAN     NOT NULL DEFAULT false,
    customer_review_required         BOOLEAN     NOT NULL DEFAULT false,
    parameters                       JSONB       NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(parameters) = 'array'),
    approved_on                      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    approved_by                      TEXT        NOT NULL,
    created_on                       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT standard_change_template_version_number_key UNIQUE (template_id, version)
);

CREATE INDEX IF NOT EXISTS idx_standard_change_template_version_proposal_id ON standard_change_template_version (proposal_id);

DO $$ BEGIN
    ALTER TABLE standard_change_template
        ADD CONSTRAINT standard_change_template_current_version_fk
        FOREIGN KEY (current_version_id) REFERENCES standard_change_template_version(id) ON DELETE SET NULL;
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- Shared-primary-key extension of work_item, like change_request: the work
-- item carries number (SCP...), subject (the template name), created_by and
-- the activity; this row carries the proposal itself.
CREATE TABLE IF NOT EXISTS standard_change_proposal (
    id                               UUID        PRIMARY KEY REFERENCES work_item(id) ON DELETE CASCADE,
    kind                             VARCHAR(10) NOT NULL CHECK (kind IN ('NEW', 'MODIFY', 'RETIRE')),
    -- DRAFT -> CAB_REVIEW -> APPROVED | REJECTED; DRAFT or CAB_REVIEW ->
    -- WITHDRAWN (by the proposer). CAB_REVIEW -> DRAFT when the CAB sends it
    -- back for rework is not a state: a rejection is final, the proposer starts
    -- a new proposal (and can copy this one).
    state                            VARCHAR(20) NOT NULL DEFAULT 'DRAFT'
                                     CHECK (state IN ('DRAFT', 'CAB_REVIEW', 'APPROVED', 'REJECTED', 'WITHDRAWN')),
    -- MODIFY / RETIRE: the template acted on. NEW: set when it is approved.
    template_id                      UUID        REFERENCES standard_change_template(id) ON DELETE SET NULL,
    -- MODIFY: the version the draft was started from. Approval is refused when
    -- the template has moved on since (someone else's modification landed).
    base_version_id                  UUID        REFERENCES standard_change_template_version(id) ON DELETE SET NULL,
    -- The completed change request the draft was started from, if any.
    source_change_request_id         UUID        REFERENCES change_request(id) ON DELETE SET NULL,
    -- Why this change is low risk and repeatable enough to be pre-authorised.
    rationale                        TEXT,
    -- Draft content (see the header). Nullable here: a draft may be incomplete;
    -- submission checks it is whole.
    subject                          TEXT,
    description                      TEXT,
    justification                    TEXT,
    implementation_plan              TEXT,
    risk_impact_analysis             TEXT,
    backout_plan                     TEXT,
    test_plan                        TEXT,
    category                         VARCHAR(50),
    impact                           VARCHAR(20),
    risk                             VARCHAR(20),
    rollback_duration_text           TEXT,
    is_planning_visible_to_customers BOOLEAN     NOT NULL DEFAULT false,
    customer_approval_required       BOOLEAN     NOT NULL DEFAULT false,
    customer_review_required         BOOLEAN     NOT NULL DEFAULT false,
    parameters                       JSONB       NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(parameters) = 'array'),
    submitted_on                     TIMESTAMPTZ,
    decided_on                       TIMESTAMPTZ,
    decided_by_user_id               UUID        REFERENCES "user"(id) ON DELETE SET NULL,
    decision_comment                 TEXT,
    -- APPROVED NEW / MODIFY: the version the approval wrote.
    resulting_version_id             UUID        REFERENCES standard_change_template_version(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_standard_change_proposal_template_id ON standard_change_proposal (template_id);
CREATE INDEX IF NOT EXISTS idx_standard_change_proposal_source_cr ON standard_change_proposal (source_change_request_id);
CREATE INDEX IF NOT EXISTS idx_standard_change_proposal_state ON standard_change_proposal (state);
-- At most one open (DRAFT or CAB_REVIEW) MODIFY or RETIRE per template, so two
-- people cannot race their edits of the same template through the CAB.
CREATE UNIQUE INDEX IF NOT EXISTS standard_change_proposal_one_open_per_template
    ON standard_change_proposal (template_id)
    WHERE template_id IS NOT NULL AND kind IN ('MODIFY', 'RETIRE') AND state IN ('DRAFT', 'CAB_REVIEW');

CREATE TABLE IF NOT EXISTS standard_change_proposal_evidence (
    proposal_id       UUID        NOT NULL REFERENCES standard_change_proposal(id) ON DELETE CASCADE,
    change_request_id UUID        NOT NULL REFERENCES change_request(id) ON DELETE CASCADE,
    created_on        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by        TEXT        NOT NULL,
    PRIMARY KEY (proposal_id, change_request_id)
);

CREATE INDEX IF NOT EXISTS idx_standard_change_proposal_evidence_cr ON standard_change_proposal_evidence (change_request_id);

-- A change request raised from a template: the version it follows (its frozen
-- fields were rendered from it on create and can never be edited) and the
-- values of that version's parameters, {"key": "value", ...}. Both NULL on
-- every other change request, legacy Standard ones included.
ALTER TABLE change_request ADD COLUMN IF NOT EXISTS standard_change_template_version_id UUID
    REFERENCES standard_change_template_version(id) ON DELETE RESTRICT;
ALTER TABLE change_request ADD COLUMN IF NOT EXISTS standard_change_parameter_values JSONB;

CREATE INDEX IF NOT EXISTS idx_change_request_standard_change_template_version_id
    ON change_request (standard_change_template_version_id) WHERE standard_change_template_version_id IS NOT NULL;
