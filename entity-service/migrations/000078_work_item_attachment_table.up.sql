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

-- Mirrors operations/csm-sync-service's work_item_attachment table. No
-- endpoint reads or writes this yet -- case_attachment (migration 000043)
-- already covers this service's own Postgres-backed attachment upload flow
-- for cases; this table is the generic, work_item-wide equivalent the sync
-- service populates from ServiceNow's own attachment records, added here
-- purely for schema parity so a future feature (e.g. reading synced
-- attachment metadata for non-case work items) doesn't have to add it from
-- scratch.
DO $$ BEGIN
    CREATE TYPE work_item_attachment_state_enum AS ENUM (
        'AVAILABLE', 'AVAILABLE_CONDITIONALLY', 'NOT_AVAILABLE', 'PENDING'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS work_item_attachment (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    name VARCHAR(255),
    content_type VARCHAR(255),
    work_item_id UUID NOT NULL REFERENCES work_item(id) ON DELETE CASCADE,
    size_bytes BIGINT,
    compressed_size_bytes BIGINT,
    is_compressed BOOLEAN,
    hash VARCHAR(100),
    state work_item_attachment_state_enum,
    chunk_size_bytes INTEGER
);

CREATE INDEX IF NOT EXISTS idx_work_item_attachment_work_item_id ON work_item_attachment (work_item_id);
