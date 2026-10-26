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

-- An SLA definition (SLA SPEC.md section 6.1/6.1.1/6.1.2, R1-R21, R25-R27,
-- R39, R20's cascade): the one entity admins create, edit, draft and
-- publish. No row is seeded here -- unlike sla_calendar's two placeholder
-- rows, a published definition actually starts tracking real cases/
-- incidents, so fabricating one without a confirmed real policy would be
-- actively wrong, not merely a low-risk placeholder. Definitions are
-- created entirely through the admin UI once it exists.
--
-- Four condition columns (start/pause/stop/cancel) each hold a predicate
-- tree (R4): {"all"|"any": [...]} | {"not": ...} | {"field","op","value"}.
-- Only start_condition is required (R18); the rest are plain JSONB NULL
-- when "not set never fires" (section 6.1.2's own framing). Validating a
-- predicate's fields/operators against the enabled catalog (R5, R7) is
-- application-layer work (internal/service/sla_definition_service.go), not
-- a DB constraint -- the same "parse, validate against an allow-list, then
-- store" discipline this codebase already uses for case_filters.go's own
-- predicate parsing, just for a recursive tree instead of a flat list.
--
-- threshold_percentages and also_completes_definition_ids are plain array
-- columns, not child tables -- both are short, bounded, non-relational
-- lists with no independent query need (same reasoning as sla_calendar's
-- working_week), unlike this file's earlier draft which gave each its own
-- table. "Always includes 100" (R39) and "no self-reference" (a definition
-- cannot cascade to itself) are both application-layer validation, enforced
-- at publish time, not DB constraints -- the same posture R39's own text
-- takes ("Admins set them in the UI... Values must be greater than 0 and
-- unique within the definition").
BEGIN;
SET LOCAL lock_timeout = '5s';

CREATE TABLE IF NOT EXISTS sla_definition (
    id                            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                          VARCHAR(255) NOT NULL,
    description                   TEXT,
    work_item_type                work_item_type_enum NOT NULL
        REFERENCES sla_work_item_type(work_item_type),
    status                        VARCHAR(20) NOT NULL DEFAULT 'draft',
    calendar_id                   UUID NOT NULL REFERENCES sla_calendar(id),
    timezone_source               VARCHAR(20) NOT NULL,
    -- IANA zone; required iff timezone_source = 'definition' (R26).
    definition_timezone           VARCHAR(100),
    -- IANA zone; required iff timezone_source = 'account' (R26's "must also
    -- store a fallback... used when the account has no time zone set").
    account_timezone_fallback     VARCHAR(100),
    start_mode                    VARCHAR(20) NOT NULL DEFAULT 'now',
    -- Free-text metadata only ('response'/'workaround'/'resolution' or
    -- anything an admin names) -- the engine has no hardcoded notion of
    -- this value (R1); it exists for display and for the admin picking
    -- also_completes_definition_ids targets, nothing else reads it.
    target_label                  VARCHAR(100) NOT NULL,
    -- The target, measured in working time (R25, R40) -- not wall-clock.
    duration                      INTERVAL NOT NULL,
    -- Recorded against a paused instance verbatim (R19) -- the pause
    -- condition itself decides *when*, this decides what a pause is called.
    pause_reason                  VARCHAR(255),
    start_condition               JSONB NOT NULL,
    pause_condition               JSONB,
    stop_condition                JSONB,
    cancel_condition              JSONB,
    threshold_percentages         INTEGER[] NOT NULL DEFAULT '{100}',
    also_completes_definition_ids UUID[] NOT NULL DEFAULT '{}',
    -- Bumped on every edit -- a lightweight audit trail pending a real
    -- answer to open question 5 ("is there versioning, audit and approval
    -- of changes?").
    version                       INTEGER NOT NULL DEFAULT 1,
    created_on                    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_on                    TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by                    VARCHAR(255) NOT NULL,
    updated_by                    VARCHAR(255) NOT NULL,
    CONSTRAINT sla_definition_status_check
        CHECK (status IN ('draft', 'published', 'retired')),
    CONSTRAINT sla_definition_timezone_source_check
        CHECK (timezone_source IN ('definition', 'account')),
    CONSTRAINT sla_definition_start_mode_check
        CHECK (start_mode IN ('now', 'retroactive', 'carry_over')),
    CONSTRAINT sla_definition_timezone_source_fields_check
        CHECK (
            (timezone_source = 'definition' AND definition_timezone IS NOT NULL)
            OR
            (timezone_source = 'account' AND account_timezone_fallback IS NOT NULL)
        )
);

COMMENT ON TABLE sla_definition IS
    'Admin-authored SLA policy (R1-R21, R25-R27, R39): which work items it matches, its four conditions, its calendar/timezone, its thresholds and its "also completes" cascade. No SLA behaviour lives anywhere else (R2).';

CREATE INDEX IF NOT EXISTS sla_definition_work_item_type_status_idx
    ON sla_definition (work_item_type, status);

CREATE INDEX IF NOT EXISTS sla_definition_calendar_id_idx
    ON sla_definition (calendar_id);

COMMIT;
