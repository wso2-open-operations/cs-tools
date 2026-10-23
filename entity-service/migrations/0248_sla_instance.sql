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

-- sla_instance is the SLA module's own live engine state (SLA SPEC.md
-- R8-R13, R28-R38, R40-R45) -- one row per (definition, work item) clock,
-- created when a published definition's start condition matches (R9, R18)
-- and never recreated for the same definition/work item pair while a
-- cancelled-or-completed row already exists in a final state (R11, R34).
--
-- calendar_snapshot/timezone_resolved/duration are frozen at creation (R8,
-- R27) from the definition's own calendar/timezone/duration at that moment
-- -- a later edit to the definition's calendar, holidays or duration never
-- moves an already-running instance's target or due dates.
--
-- elapsed_working_seconds is a boundary snapshot, not a continuously
-- updated counter: it only changes when a segment ends (pause, resume,
-- complete, cancel). For a currently-running instance, the *live* value for
-- display is computed at read time as elapsed_working_seconds +
-- WorkingSecondsBetween(calendar_snapshot, timezone_resolved,
-- segment_started_on, now) -- entity-service's own internal/slacalendar
-- package, never a periodic background write. This is a deliberate
-- continuation of the lesson already learned once in this codebase (the
-- old CSM-native engine's SLAEngineRecomputeWorker, a 45s full-table sweep,
-- was deleted in favour of exactly this "snapshot at the boundary, project
-- forward at read time" shape -- see entity-service/CLAUDE.md's own
-- "CSM-native SLA clock engine" section) -- not a reintroduction of that
-- sweep.
--
-- next_due_percentage/next_due_on back the one bounded background process
-- this module runs (R41, R42, R45): recomputed only when an instance starts
-- or resumes, read by a ticker that only ever touches instances whose due
-- time has actually arrived -- cost bounded by real threshold crossings,
-- never by the number of open instances.
--
-- The partial unique index below is R10's actual enforcement ("one active
-- instance per definition and work item") -- not a service-layer check
-- alone, so a race between two near-simultaneous triggering changes can
-- never create two running/paused instances of the same definition for the
-- same work item.
BEGIN;
SET LOCAL lock_timeout = '5s';

CREATE TABLE IF NOT EXISTS sla_instance (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    definition_id           UUID NOT NULL REFERENCES sla_definition(id),
    work_item_id            UUID NOT NULL REFERENCES work_item(id),
    state                   VARCHAR(20) NOT NULL,
    -- Set once, only on completion (R31) -- never meaningful before then.
    outcome                 VARCHAR(20),
    calendar_snapshot       JSONB NOT NULL,
    timezone_resolved       VARCHAR(100) NOT NULL,
    duration                INTERVAL NOT NULL,
    elapsed_working_seconds BIGINT NOT NULL DEFAULT 0,
    started_on              TIMESTAMPTZ NOT NULL,
    segment_started_on      TIMESTAMPTZ NOT NULL,
    pause_reason            VARCHAR(255),
    completed_on            TIMESTAMPTZ,
    cancelled_on            TIMESTAMPTZ,
    next_due_percentage     INTEGER,
    next_due_on             TIMESTAMPTZ,
    created_on              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_on              TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT sla_instance_state_check
        CHECK (state IN ('running', 'paused', 'completed', 'cancelled')),
    CONSTRAINT sla_instance_outcome_check
        CHECK (outcome IS NULL OR outcome IN ('achieved', 'breached'))
);

COMMENT ON TABLE sla_instance IS
    'A running SLA clock (R8-R13, R28-R38). Elapsed time is a boundary snapshot, projected forward at read time, not a periodic write (see entity-service/internal/slacalendar).';

-- R10: at most one running/paused instance per (definition, work item) --
-- a cancelled or completed row never blocks a fresh one (R11).
CREATE UNIQUE INDEX IF NOT EXISTS sla_instance_one_active_per_definition_work_item
    ON sla_instance (definition_id, work_item_id)
    WHERE state IN ('running', 'paused');

CREATE INDEX IF NOT EXISTS sla_instance_work_item_id_idx
    ON sla_instance (work_item_id);

-- Feeds the bounded threshold-crossing worker: only rows with a due
-- threshold in the past are ever selected.
CREATE INDEX IF NOT EXISTS sla_instance_next_due_on_idx
    ON sla_instance (next_due_on)
    WHERE state = 'running' AND next_due_on IS NOT NULL;

-- Append-only transition history (R37: "Every state change is recorded with
-- its time, the previous and new state, and the work item change that
-- caused it").
CREATE TABLE IF NOT EXISTS sla_instance_transition (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instance_id UUID NOT NULL REFERENCES sla_instance(id),
    -- NULL on the initial "created" transition.
    from_state  VARCHAR(20),
    to_state    VARCHAR(20) NOT NULL,
    occurred_on TIMESTAMPTZ NOT NULL,
    field_name  VARCHAR(255),
    old_value   TEXT,
    new_value   TEXT,
    created_on  TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE sla_instance_transition IS
    'Append-only audit trail of every sla_instance state change (R37) -- never updated or deleted.';

CREATE INDEX IF NOT EXISTS sla_instance_transition_instance_id_idx
    ON sla_instance_transition (instance_id, occurred_on);

-- Append-only, "recorded once" (R42): the unique constraint is the actual
-- idempotency guarantee, not just the worker's own best effort.
CREATE TABLE IF NOT EXISTS sla_instance_threshold_crossing (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instance_id UUID NOT NULL REFERENCES sla_instance(id),
    percentage  INTEGER NOT NULL,
    due_on      TIMESTAMPTZ NOT NULL,
    detected_on TIMESTAMPTZ NOT NULL,
    CONSTRAINT sla_instance_threshold_crossing_instance_percentage_uq
        UNIQUE (instance_id, percentage)
);

COMMENT ON TABLE sla_instance_threshold_crossing IS
    'One row per threshold an sla_instance has crossed (R39-R45) -- the unique (instance_id, percentage) constraint is what makes a crossing fire at most once, even under retry (R42).';

COMMIT;
