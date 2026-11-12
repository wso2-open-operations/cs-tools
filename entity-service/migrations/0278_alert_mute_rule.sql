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

-- alert_mute_rule: an alert mute rule (Operations > Alert mute rules), a one-time window during which SRE alerts matching every
-- condition are muted (recorded by alert-core, but no incident, page or chat post). The state (scheduled, active, expired,
-- ended) is derived from start_on, end_on and ended_on, never stored. Safe to run again.
CREATE SEQUENCE IF NOT EXISTS amr_number_seq START 1;

INSERT INTO number_series (series, digits, sequence_name) VALUES ('AMR', 7, 'amr_number_seq')
ON CONFLICT (series) DO NOTHING;

CREATE TABLE IF NOT EXISTS alert_mute_rule (
    id          UUID          PRIMARY KEY DEFAULT uuidv7(),
    number      TEXT          NOT NULL DEFAULT next_series_number('AMR'),
    name        VARCHAR(255)  NOT NULL CHECK (btrim(name) <> ''),
    description VARCHAR(1024) NOT NULL DEFAULT '',
    time_zone   TEXT          NOT NULL CHECK (btrim(time_zone) <> ''),
    start_on    TIMESTAMPTZ   NOT NULL,
    end_on      TIMESTAMPTZ   NOT NULL,
    -- A JSON array of {field, value} objects. entity-service checks which fields are allowed.
    conditions  JSONB         NOT NULL CHECK (jsonb_typeof(conditions) = 'array' AND jsonb_array_length(conditions) > 0),
    created_by  TEXT          NOT NULL,
    created_on  TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_by  TEXT          NOT NULL,
    updated_on  TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    ended_by    TEXT,
    ended_on    TIMESTAMPTZ,
    CONSTRAINT alert_mute_rule_window_check CHECK (end_on > start_on AND end_on - start_on <= INTERVAL '72 hours'),
    CONSTRAINT alert_mute_rule_ended_check  CHECK ((ended_on IS NULL) = (ended_by IS NULL))
);

CREATE UNIQUE INDEX IF NOT EXISTS alert_mute_rule_number_key ON alert_mute_rule (number);
-- GET /alert-mute-rules/active (polled by alert-core): rules not ended early whose window has not closed.
CREATE INDEX IF NOT EXISTS alert_mute_rule_open_end_on_idx ON alert_mute_rule (end_on) WHERE ended_on IS NULL;
-- POST /alert-mute-rules/search: newest created first.
CREATE INDEX IF NOT EXISTS alert_mute_rule_created_on_idx ON alert_mute_rule (created_on DESC, id DESC);

COMMENT ON TABLE alert_mute_rule IS
    'Alert mute rules: one-time windows during which SRE alerts matching every condition are muted. State is derived, not stored.';
