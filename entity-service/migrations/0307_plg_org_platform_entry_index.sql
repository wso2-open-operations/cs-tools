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

-- PLG: an organisation's entry registration is its earliest pairing. The
-- dashboard finds it with DISTINCT ON (organization_id) ordered by
-- (registered_on, id); this index serves that order without a sort. Safe to re-run.
--
-- Built CONCURRENTLY so registration writes are not blocked while it builds. That
-- needs a single statement outside a transaction, as make migrate runs it. A failed
-- concurrent build can leave an INVALID index that IF NOT EXISTS would then skip, so
-- before re-running after a failure, drop it:
--   DROP INDEX CONCURRENTLY IF EXISTS idx_plg_org_platform_org_registered;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_plg_org_platform_org_registered
    ON plg_org_platform (organization_id, registered_on, id);
