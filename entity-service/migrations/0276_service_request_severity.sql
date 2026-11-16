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


-- Internal severity for service requests (S1-S4). Every service request has
-- one: existing rows become S4 and a writer that omits the column (a sync job,
-- an older entity-service build) gets S4 as well.
--
-- Reuses case_severity_enum (migration 0023) so a typo is a migration-time
-- error, not a stored string. The enum also carries S0 (catastrophic), which
-- a service request must never hold, so a CHECK excludes it; a separate enum
-- would have forced a second mapping table for the same S1-S4 labels.
--
-- ADD COLUMN with a constant DEFAULT is metadata-only on PostgreSQL 11 and
-- later (no table rewrite): the existing rows read the default from the
-- catalog. This assumes PostgreSQL 11+ (every supported environment runs 14+).
-- The CHECK is added NOT VALID and then validated, so the validation scan
-- holds only a SHARE UPDATE EXCLUSIVE lock and does not block writes.
--
-- Idempotent: re-running adds nothing and changes no value.
ALTER TABLE service_request
    ADD COLUMN IF NOT EXISTS severity case_severity_enum NOT NULL DEFAULT 'S4';

DO $$ BEGIN
    ALTER TABLE service_request
        ADD CONSTRAINT service_request_severity_not_catastrophic
        CHECK (severity <> 'S0') NOT VALID;
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

ALTER TABLE service_request VALIDATE CONSTRAINT service_request_severity_not_catastrophic;
