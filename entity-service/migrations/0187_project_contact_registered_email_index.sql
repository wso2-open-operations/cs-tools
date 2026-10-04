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

-- Every statement a non-internal caller runs through repository.Scoped first
-- resolves that caller's projects (rls.go, setViewerProjectIDsSQL):
--   SELECT array_agg(pc.project_id) FROM project_contact pc
--   WHERE LOWER(pc.email) = LOWER($1) AND pc.state = 'REGISTERED'
-- The only email index, (project_id, LOWER(email)), leads on project_id and
-- cannot serve a lookup by email alone, so this was a full scan of
-- project_contact on every customer statement. Partial on REGISTERED (the
-- only rows that lookup reads), covering project_id so it is index-only.
--
-- CONCURRENTLY, so writes to the table are not blocked while it builds; it
-- cannot run inside a transaction block, so this is the only statement in the
-- file (`make migrate` applies each file with plain autocommit `psql -f`, the
-- compose runner skips its -1 for CONCURRENTLY files). If a build is
-- interrupted it leaves an INVALID index that IF NOT EXISTS would then skip:
-- drop it and re-run this file.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_project_contact_lower_email_registered
    ON project_contact (LOWER(email)) INCLUDE (project_id)
    WHERE state = 'REGISTERED';
