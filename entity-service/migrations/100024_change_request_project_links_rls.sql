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

-- Row-level policies for the join tables in
-- 0191_change_request_project_links.sql: change_request_deployment and
-- change_request_deployed_product, and change_request_environment only where
-- 0192 has not dropped it (see below).
-- Split out into the dedicated RLS migration track (migrations/1NNNNN_*.sql,
-- see entity-service/CLAUDE.md's "Database migrations" section) rather than
-- living inline in the migration that creates these tables -- every other
-- RLS statement in this codebase lives in that track, and 0191 originally
-- mixed schema and RLS in one file only because it predates the track's
-- existence.
--
-- Each table this file acts on gets ENABLE and FORCE ROW LEVEL SECURITY and
-- three policies, for SELECT, INSERT and DELETE, with the same
-- project-membership rule as work_item_tag / work_item_watcher (migration
-- 100008): internal callers see everything, a member sees the links of change
-- requests of their own project. Update is deliberately not a policy: the
-- repository only ever deletes and re-inserts the whole list.
-- (change_request_deployment is the one exception -- csm-sync-service's own
-- writer needs an UPDATE policy too; see
-- 100023_change_request_deployment_internal_update_policy.sql, which adds it
-- separately and does not depend on this file running before it: CREATE
-- POLICY has no dependency on another policy, or on ENABLE/FORCE ROW LEVEL
-- SECURITY, already having run against the same table.)
--
-- 0192_change_request_drop_environments drops change_request_environment, and
-- make migrate applies it in phase 1, before this track, so this file skips
-- that table's block (see the comment above it). A database make migrate
-- builds therefore ends with two tables under ENABLE and FORCE ROW LEVEL
-- SECURITY, change_request_deployment and change_request_deployed_product,
-- and six policies from this file, three on each; with 100023's UPDATE
-- policy, change_request_deployment has four, seven in all. A database that
-- still has change_request_environment (0191 applied, 0192 not) ends the
-- same way, because make migrate applies 0192 before this file in the same
-- run. The block acts only where this file runs while the table still exists
-- (applied by hand ahead of 0192, say), and 0192's DROP TABLE then removes
-- the table together with its three policies.
--
-- One transaction, and every CREATE POLICY preceded by its own DROP POLICY
-- IF EXISTS, as in 100002_case_escalation_rls.sql -- see its note for why
-- (Postgres has no CREATE POLICY IF NOT EXISTS).
--
-- ENABLE / FORCE ROW LEVEL SECURITY, DROP POLICY and CREATE POLICY each take
-- an ACCESS EXCLUSIVE lock on their table, which waits behind every open
-- transaction that has read or written it and holds up every reader and
-- writer queued after it; csm-sync-service writes change_request_deployment
-- too. lock_timeout makes a busy table fail the file instead, and the
-- transaction keeps nothing; run it again.
BEGIN;
SET LOCAL lock_timeout = '5s';

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

-- Only where the table still exists. 0192_change_request_drop_environments
-- drops it, and make migrate applies this track after every 4-digit file, so
-- on a database that has not recorded this file yet (a fresh one included)
-- the table is already gone and an unguarded ALTER TABLE stops the run with
-- `relation "change_request_environment" does not exist`.
DO $$ BEGIN
    IF to_regclass('change_request_environment') IS NOT NULL THEN
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
    END IF;
END $$;

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
