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

-- SLA calendars (SLA SPEC.md R22-R27): a calendar is a working week plus a
-- holiday list. An SLA clock only accrues elapsed time while it is inside
-- its definition's calendar's working hours (R25); a 24x7 target is simply
-- a calendar whose every day is a full working day, not a separate code
-- path.
--
-- working_week is a 7-element JSONB array on sla_calendar itself, not a
-- child table: it is always exactly 7 rows (one per day of the week, R23's
-- "exactly one working window per day") with no independent query need
-- across calendars, so a child table would only add a join for data that
-- never varies in shape. Each element is
-- {"weekday": 0-6, "isWorking": bool, "startTime": "HH:MM", "endTime": "HH:MM"}
-- with weekday using Go's time.Weekday numbering (0 = Sunday .. 6 =
-- Saturday) so entity-service's internal/slacalendar package reads it with
-- no translation. Holidays (R24, a genuinely variable-length, admin-grown
-- list) stay a real child table, sla_calendar_holiday.
--
-- A calendar's own time is *local* wall-clock time with no time zone of its
-- own (R26) -- the zone it is read in is chosen per sla_definition
-- (migration 0247), either a fixed IANA zone or the work item's account's
-- zone. This is why no timezone column exists here.
--
-- Exactly two calendars are seeded below, 24x7 and WSO2 Working Days, by
-- deliberate product decision (not every admin-creatable calendar R22
-- would technically allow) -- see this change's own plan file for the
-- reasoning. If a third is ever genuinely needed the schema already
-- supports it (is_retired, never delete, R22); only the admin UI's build
-- scope is narrowed to an editor for these two rows, not a create/retire
-- flow.
--
-- *** PLACEHOLDER SEED VALUES -- CONFIRM BEFORE RELYING ON THEM ***
-- WSO2 Working Days is seeded here as Mon-Fri 09:00-17:00 local time, with
-- an EMPTY holiday list, because no confirmed value for WSO2's actual
-- support-policy working hours (or which holiday calendar backs them) was
-- available when this migration was written. 24x7 is seeded with every day
-- fully working and also an empty holiday list -- i.e. round-the-clock
-- support is assumed to never pause for a holiday; confirm this is the
-- intended behaviour, not just the default. Both are editable afterward
-- through the admin UI with no further migration or deploy (that is the
-- entire point of these being table rows, R2) -- but do not publish a real
-- SLA definition against either calendar until the real hours and holiday
-- list have been entered.
BEGIN;
SET LOCAL lock_timeout = '5s';

CREATE TABLE IF NOT EXISTS sla_calendar (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name         VARCHAR(255) NOT NULL UNIQUE,
    is_retired   BOOLEAN NOT NULL DEFAULT FALSE,
    working_week JSONB NOT NULL,
    created_on   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_on   TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by   VARCHAR(255) NOT NULL DEFAULT 'migration:0246',
    updated_by   VARCHAR(255) NOT NULL DEFAULT 'migration:0246'
);

COMMENT ON TABLE sla_calendar IS
    'SLA calendars (R22-R23, R25-R26): a named working week (working_week JSONB, 7 entries) in local wall-clock time. Referenced, never deleted, by sla_definition and frozen as a snapshot onto every sla_instance it creates (R27).';

CREATE TABLE IF NOT EXISTS sla_calendar_holiday (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    calendar_id UUID NOT NULL REFERENCES sla_calendar(id) ON DELETE CASCADE,
    start_date  DATE NOT NULL,
    end_date    DATE NOT NULL,
    name        VARCHAR(255) NOT NULL,
    created_on  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_on  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  VARCHAR(255) NOT NULL,
    updated_by  VARCHAR(255) NOT NULL,
    CONSTRAINT sla_calendar_holiday_date_range_check CHECK (end_date >= start_date)
);

COMMENT ON TABLE sla_calendar_holiday IS
    'Named holiday dates/ranges of an sla_calendar (R24) -- the whole day is non-working regardless of working_week. Not shared between calendars in v1 (open question 6).';

CREATE INDEX IF NOT EXISTS sla_calendar_holiday_calendar_id_idx
    ON sla_calendar_holiday (calendar_id);

INSERT INTO sla_calendar (name, is_retired, working_week)
VALUES
    ('24x7', FALSE,
     '[{"weekday":0,"isWorking":true,"startTime":"00:00","endTime":"23:59"},
       {"weekday":1,"isWorking":true,"startTime":"00:00","endTime":"23:59"},
       {"weekday":2,"isWorking":true,"startTime":"00:00","endTime":"23:59"},
       {"weekday":3,"isWorking":true,"startTime":"00:00","endTime":"23:59"},
       {"weekday":4,"isWorking":true,"startTime":"00:00","endTime":"23:59"},
       {"weekday":5,"isWorking":true,"startTime":"00:00","endTime":"23:59"},
       {"weekday":6,"isWorking":true,"startTime":"00:00","endTime":"23:59"}]'::jsonb),
    ('WSO2 Working Days', FALSE,
     '[{"weekday":0,"isWorking":false,"startTime":null,"endTime":null},
       {"weekday":1,"isWorking":true,"startTime":"09:00","endTime":"17:00"},
       {"weekday":2,"isWorking":true,"startTime":"09:00","endTime":"17:00"},
       {"weekday":3,"isWorking":true,"startTime":"09:00","endTime":"17:00"},
       {"weekday":4,"isWorking":true,"startTime":"09:00","endTime":"17:00"},
       {"weekday":5,"isWorking":true,"startTime":"09:00","endTime":"17:00"},
       {"weekday":6,"isWorking":false,"startTime":null,"endTime":null}]'::jsonb)
ON CONFLICT (name) DO NOTHING;

COMMIT;
