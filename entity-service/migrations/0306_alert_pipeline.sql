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

-- alert.pipeline: the processor that turns the alert into incidents, set once by the consumer from the alert's ria label. Safe to run again.
ALTER TABLE alert ADD COLUMN IF NOT EXISTS pipeline TEXT NOT NULL DEFAULT 'core';

DO $$ BEGIN
    ALTER TABLE alert ADD CONSTRAINT alert_pipeline_check CHECK (pipeline IN ('core', 'ria'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- Each processor seeds its claim from the oldest unprocessed ids of its own pipeline.
CREATE INDEX IF NOT EXISTS alert_unprocessed_pipeline_idx ON alert (pipeline, id) WHERE processed_on IS NULL;

COMMENT ON COLUMN alert.pipeline IS
    'core for the fingerprint fold, ria for the RIA Creator; decided when the alert is stored and never changed.';
