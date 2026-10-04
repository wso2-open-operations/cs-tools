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

CREATE TABLE IF NOT EXISTS migration_row_error (
    id BIGSERIAL PRIMARY KEY,
    job_id BIGINT REFERENCES migration_job(id),
    table_name TEXT NOT NULL,
    source_sys_id TEXT,
    stage TEXT NOT NULL,        -- 'transform' | 'load'
    error_message TEXT NOT NULL,
    raw_record JSONB,
    severity TEXT NOT NULL DEFAULT 'error',  -- 'error' (row rejected) | 'warning' (row loaded, degraded)
    failed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_row_errors_job ON migration_row_error (job_id);
