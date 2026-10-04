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

-- "user" has no index on its email, yet every request resolves its caller with
-- WHERE LOWER(email) = LOWER($1) (access_repo.go), and the case comment list,
-- activity feed and case detail join "user" ON LOWER(u.email) = LOWER(...).
-- Each of those was a full scan of "user". Not unique: the same address can
-- legitimately sit on more than one row today.
--
-- CONCURRENTLY, so writes to the table are not blocked while it builds; it
-- cannot run inside a transaction block, so this is the only statement in the
-- file (`make migrate` applies each file with plain autocommit `psql -f`, the
-- compose runner skips its -1 for CONCURRENTLY files). If a build is
-- interrupted it leaves an INVALID index that IF NOT EXISTS would then skip:
-- drop it and re-run this file.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_user_lower_email
    ON "user" (LOWER(email));
