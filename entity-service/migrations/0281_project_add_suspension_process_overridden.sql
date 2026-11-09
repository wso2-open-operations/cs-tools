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
-- CSM-owned "suspension process overridden" flag on a project: when TRUE the
-- automatic Account Closure Process (ACP) must not evaluate the project.
-- Read-only for now: set directly in the database or by the data sync.
--
-- Idempotent. Safe on a large live table: a constant DEFAULT on ADD COLUMN is
-- a metadata-only change on PostgreSQL 11 and later (no table rewrite), and
-- NOT NULL is satisfied by that default for every existing row.
ALTER TABLE project
    ADD COLUMN IF NOT EXISTS suspension_process_overridden BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN project.suspension_process_overridden IS
    'When TRUE the automatic Account Closure Process skips this project. CSM-owned, FALSE by default.';
