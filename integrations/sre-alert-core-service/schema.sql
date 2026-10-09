-- Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

-- Applied by alert-core-service at startup (schema.go, internal/postgres.Migrate); every statement must stay idempotent.

-- Written by sre-alert-ingestion-service; claimed and marked processed by alert-core-service.
CREATE TABLE IF NOT EXISTS alerts (
  id             text PRIMARY KEY,
  source         text NOT NULL,
  alert          jsonb NOT NULL,
  fingerprint    text,
  created_at     timestamptz NOT NULL DEFAULT now(),
  claimed_by     text,
  claimed_until  timestamptz,
  processed_at   timestamptz
) WITH (fillfactor = 85, autovacuum_vacuum_scale_factor = 0.05, autovacuum_vacuum_insert_scale_factor = 0.05);

-- Claim seeds come from the oldest unprocessed ids; both indexes shrink to the unprocessed tail.
CREATE INDEX IF NOT EXISTS alerts_unclaimed_idx ON alerts (id) WHERE processed_at IS NULL;
-- Pulls the rest of a seed's fingerprint into the same claim, so one incident's alerts fold together.
CREATE INDEX IF NOT EXISTS alerts_unclaimed_fp_idx ON alerts (fingerprint) WHERE processed_at IS NULL;

CREATE SEQUENCE IF NOT EXISTS alert_seq AS bigint START 1;

-- One row per incident; a fingerprint gets a new row each time an alert arrives after engine.dedup_window.
CREATE TABLE IF NOT EXISTS incidents_processed (
  id                      bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  fingerprint             text NOT NULL,
  incident_id             text NOT NULL DEFAULT '',
  incident_number         text NOT NULL,
  status                  text NOT NULL DEFAULT 'new',
  severity                int  NOT NULL,
  impact                  text NOT NULL,
  urgency                 text NOT NULL,
  service                 text NOT NULL DEFAULT '',
  metric_name             text NOT NULL DEFAULT '',
  category                text NOT NULL DEFAULT '',
  environment             text NOT NULL DEFAULT '',
  source                  text NOT NULL DEFAULT '',
  alert_count             int  NOT NULL DEFAULT 1,
  first_seen              timestamptz NOT NULL,
  last_seen               timestamptz NOT NULL,
  state_checked_at        timestamptz NOT NULL DEFAULT to_timestamp(0),
  fallback                boolean NOT NULL DEFAULT false,
  csm_confirmed           boolean NOT NULL DEFAULT false,
  csm_attempts            int NOT NULL DEFAULT 0,
  csm_permanently_failed  boolean NOT NULL DEFAULT false,
  csm_last_attempt_at     timestamptz NOT NULL DEFAULT to_timestamp(0),
  delivery_due_at         timestamptz,
  fold_version            bigint NOT NULL DEFAULT 0,
  created_at              timestamptz NOT NULL DEFAULT now(),
  -- The first alert's routing signals (an alarm-named group, the SNS topic, the AWS account), recorded for reference; not used for routing (entity-service assigns the group from the service).
  assignment_group        text NOT NULL DEFAULT '',
  source_topic            text NOT NULL DEFAULT '',
  source_account          text NOT NULL DEFAULT '',
  UNIQUE (fingerprint, first_seen)
) WITH (fillfactor = 80, autovacuum_vacuum_scale_factor = 0.05);

-- For a database created before the routing columns existed.
ALTER TABLE incidents_processed ADD COLUMN IF NOT EXISTS assignment_group text NOT NULL DEFAULT '';
ALTER TABLE incidents_processed ADD COLUMN IF NOT EXISTS source_topic text NOT NULL DEFAULT '';
ALTER TABLE incidents_processed ADD COLUMN IF NOT EXISTS source_account text NOT NULL DEFAULT '';

-- The delivery loop reads only incidents whose CSM/Chat work is due.
CREATE INDEX IF NOT EXISTS incidents_processed_due_idx ON incidents_processed (delivery_due_at) WHERE delivery_due_at IS NOT NULL;

-- Append-only log of every alert folded into an incident; alert_id makes a replayed alert a no-op.
CREATE TABLE IF NOT EXISTS incident_notes (
  id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  incident      bigint NOT NULL REFERENCES incidents_processed (id) ON DELETE CASCADE,
  alert_id      text NOT NULL UNIQUE,
  kind          text NOT NULL,
  note          text NOT NULL,
  created_at    timestamptz NOT NULL DEFAULT now(),
  csm_pending   boolean NOT NULL DEFAULT true,
  chat_pending  boolean NOT NULL DEFAULT false
) WITH (fillfactor = 90, autovacuum_vacuum_scale_factor = 0.05);

CREATE INDEX IF NOT EXISTS incident_notes_incident_idx ON incident_notes (incident);
CREATE INDEX IF NOT EXISTS incident_notes_pending_idx ON incident_notes (incident, id) WHERE csm_pending OR chat_pending;

-- Service accounts for webhook auth (ingestion) and POST /alertz (core); managed with cmd/user.
CREATE TABLE IF NOT EXISTS integration_users (
  username           text PRIMARY KEY,
  id                 uuid NOT NULL DEFAULT gen_random_uuid(),
  secret_hash        text NOT NULL,
  salt               text NOT NULL,
  iterations         int  NOT NULL,
  enabled            boolean NOT NULL DEFAULT true,
  created_at         timestamptz NOT NULL DEFAULT now(),
  created_by         text NOT NULL DEFAULT '',
  updated_at         timestamptz NOT NULL DEFAULT now(),
  secret_rotated_at  timestamptz NOT NULL DEFAULT to_timestamp(0),
  last_used_at       timestamptz NOT NULL DEFAULT to_timestamp(0)
);

-- Raw webhook bodies exactly as received, before any transform; sre-alert-ingestion-service writes them in batches.
CREATE TABLE IF NOT EXISTS raw_alerts (
  id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  received_at  timestamptz NOT NULL,
  payload      jsonb NOT NULL
);
-- Retention deletes by age, oldest first.
CREATE INDEX IF NOT EXISTS raw_alerts_received_at_idx ON raw_alerts (received_at);
