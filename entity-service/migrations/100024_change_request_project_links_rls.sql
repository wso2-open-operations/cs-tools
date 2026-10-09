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

-- Row-level security for change_request_deployment, change_request_environment
-- and change_request_deployed_product (0191_change_request_project_links.sql),
-- split out into the dedicated RLS migration track (migrations/1NNNNN_*.sql,
-- see entity-service/CLAUDE.md's "Database migrations" section) rather than
-- living inline in the migration that creates these tables -- every other
-- RLS statement in this codebase lives in that track, and 0191 originally
-- mixed schema and RLS in one file only because it predates the track's
-- existence.
--
-- The three join tables are under FORCE ROW LEVEL SECURITY with the same
-- project-membership rule as work_item_tag / work_item_watcher (migration
-- 100008): internal callers see everything, a member sees the links of
-- change requests of their own project. Update is deliberately not a
-- policy: the repository only ever deletes and re-inserts the whole list.
-- (change_request_deployment is the one exception -- csm-sync-service's own
-- writer needs an UPDATE policy too; see
-- 100023_change_request_deployment_internal_update_policy.sql, which adds it
-- separately and does not depend on this file running before it: CREATE
-- POLICY has no dependency on another policy, or on ENABLE/FORCE ROW LEVEL
-- SECURITY, already having run against the same table.)
--
-- One transaction, and every CREATE POLICY preceded by its own DROP POLICY
-- IF EXISTS, matching every other migration in this track -- see
-- 100002_case_escalation_rls.sql's identical note for why (Postgres has no
-- CREATE POLICY IF NOT EXISTS).
BEGIN;

ALTER TABLE change_request_deployment ENABLE ROW LEVEL SECURITY;
ALTER TABLE change_request_deployment FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS change_request_deployment_visibility ON change_request_deployment;
CREATE POLICY change_request_deployment_visibility ON change_request_deployment
    FOR SELECT USING (
        (SELECT current_setting('app.is_internal', true) = 'true')
        OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = change_request_deployment.change_request_id))
    );
DROP POLICY IF EXISTS change_request_deployment_write ON change_request_deployment;
CREATE POLICY change_request_deployment_write ON change_request_deployment
    FOR INSERT WITH CHECK (
        (SELECT current_setting('app.is_internal', true) = 'true')
        OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = change_request_id))
    );
DROP POLICY IF EXISTS change_request_deployment_delete ON change_request_deployment;
CREATE POLICY change_request_deployment_delete ON change_request_deployment
    FOR DELETE USING (
        (SELECT current_setting('app.is_internal', true) = 'true')
        OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = change_request_deployment.change_request_id))
    );

ALTER TABLE change_request_environment ENABLE ROW LEVEL SECURITY;
ALTER TABLE change_request_environment FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS change_request_environment_visibility ON change_request_environment;
CREATE POLICY change_request_environment_visibility ON change_request_environment
    FOR SELECT USING (
        (SELECT current_setting('app.is_internal', true) = 'true')
        OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = change_request_environment.change_request_id))
    );
DROP POLICY IF EXISTS change_request_environment_write ON change_request_environment;
CREATE POLICY change_request_environment_write ON change_request_environment
    FOR INSERT WITH CHECK (
        (SELECT current_setting('app.is_internal', true) = 'true')
        OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = change_request_id))
    );
DROP POLICY IF EXISTS change_request_environment_delete ON change_request_environment;
CREATE POLICY change_request_environment_delete ON change_request_environment
    FOR DELETE USING (
        (SELECT current_setting('app.is_internal', true) = 'true')
        OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = change_request_environment.change_request_id))
    );

ALTER TABLE change_request_deployed_product ENABLE ROW LEVEL SECURITY;
ALTER TABLE change_request_deployed_product FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS change_request_deployed_product_visibility ON change_request_deployed_product;
CREATE POLICY change_request_deployed_product_visibility ON change_request_deployed_product
    FOR SELECT USING (
        (SELECT current_setting('app.is_internal', true) = 'true')
        OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = change_request_deployed_product.change_request_id))
    );
DROP POLICY IF EXISTS change_request_deployed_product_write ON change_request_deployed_product;
CREATE POLICY change_request_deployed_product_write ON change_request_deployed_product
    FOR INSERT WITH CHECK (
        (SELECT current_setting('app.is_internal', true) = 'true')
        OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = change_request_id))
    );
DROP POLICY IF EXISTS change_request_deployed_product_delete ON change_request_deployed_product;
CREATE POLICY change_request_deployed_product_delete ON change_request_deployed_product
    FOR DELETE USING (
        (SELECT current_setting('app.is_internal', true) = 'true')
        OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = change_request_deployed_product.change_request_id))
    );

COMMIT;
