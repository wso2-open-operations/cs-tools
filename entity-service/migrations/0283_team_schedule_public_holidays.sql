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

-- Public holidays on the Team Schedule (Sri Lanka, India, and any other
-- country a member is located in).
--
-- A local copy of the shared calendar-events service the WSO2 leave app reads
-- (GET /holidays?countries=...): the daily refresh (and
-- POST /team-schedule/public-holidays/refresh) upserts the current and the
-- next year for every country in use. The roster reads only these tables, so
-- the Team Schedule keeps working while that service is down.
--
-- team_schedule_person_location is each member's location as the employee
-- service gives it (the leave app's rule). A member with no row falls back to
-- their team's country: India for the India team, otherwise Sri Lanka.
--
-- Additive and safe to re-run.
CREATE TABLE IF NOT EXISTS team_schedule_public_holiday (
    country      VARCHAR(100) NOT NULL,
    holiday_date DATE         NOT NULL,
    title        TEXT         NOT NULL,
    fetched_on   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT team_schedule_public_holiday_country_day_unique
        UNIQUE (country, holiday_date)
);

CREATE INDEX IF NOT EXISTS idx_team_schedule_public_holiday_day
    ON team_schedule_public_holiday (holiday_date);

CREATE TABLE IF NOT EXISTS team_schedule_person_location (
    user_id    UUID         PRIMARY KEY REFERENCES "user" (id) ON DELETE CASCADE,
    location   VARCHAR(100) NOT NULL,
    fetched_on TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- Working on a public holiday (CRE only).
--
-- A CRE member is off on a public holiday of their country: the Team
-- Schedule places a "Public holiday" tag (PH) on them for that day, three
-- weeks ahead. The tag is an allocation that takes the day off the timesheet
-- (excludes_from_timesheet) and, like every tag, takes the person off the
-- rota that day, so Case Alerts does not call them. A team lead or rota
-- manager makes somebody available by removing their tag; that person is
-- then on the rota and called as usual.
INSERT INTO team_schedule_absence_kind
    (code, short_code, label, bucket, colour_token, sort_order, is_active, family,
     excludes_from_timesheet, description, created_by, updated_by)
VALUES
    ('PH', 'PH', 'Public holiday', 'ALLOCATION', 'EXC', 74, TRUE, 'CRE', TRUE,
     'A public holiday in the member''s country, placed by the Team Schedule. Remove it to make the member available that day.',
     'migration', 'migration')
ON CONFLICT (code) DO NOTHING;

-- One row per person and holiday the tag was placed for, kept after the tag
-- is removed, so a tag a lead removed is never placed again.
CREATE TABLE IF NOT EXISTS team_schedule_holiday_tag_placed (
    user_id      UUID         NOT NULL REFERENCES "user" (id) ON DELETE CASCADE,
    holiday_date DATE         NOT NULL,
    country      VARCHAR(100) NOT NULL,
    team_key     VARCHAR(64)  NOT NULL,
    absence_id   UUID,
    title        TEXT         NOT NULL,
    placed_on    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, holiday_date)
);

CREATE INDEX IF NOT EXISTS idx_team_schedule_holiday_tag_placed_team_day
    ON team_schedule_holiday_tag_placed (team_key, holiday_date);

-- A team's lead, rota manager or a CRE rota admin confirms that the team's
-- availability for a holiday is marked. The reminders stop once every rota
-- turn the team holds that day has somebody available and this row exists.
CREATE TABLE IF NOT EXISTS team_schedule_holiday_confirmation (
    team_key     VARCHAR(64)  NOT NULL,
    holiday_date DATE         NOT NULL,
    confirmed_by VARCHAR(255) NOT NULL,
    confirmed_on TIMESTAMPTZ  NOT NULL DEFAULT now(),
    PRIMARY KEY (team_key, holiday_date)
);

-- Which holiday availability reminders went to which team, so each is sent
-- once: the 14-day notice, then 7 days, 2 and 1 working days before while
-- the team is not fully marked.
CREATE TABLE IF NOT EXISTS team_schedule_holiday_reminder_sent (
    team_key     VARCHAR(64)  NOT NULL,
    holiday_date DATE         NOT NULL,
    stage        VARCHAR(16)  NOT NULL,
    sent_on      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    PRIMARY KEY (team_key, holiday_date, stage),
    CONSTRAINT team_schedule_holiday_reminder_sent_stage_check
        CHECK (stage IN ('notice', 'week', 'two_days', 'one_day'))
);
