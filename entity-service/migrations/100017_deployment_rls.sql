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

-- Project-membership row-level security for deployment and deployed_product.
--
-- Problem (reproduced on the local real-data copy): a customer who is a
-- registered contact on project A could list project B's deployments and
-- deployed products by calling POST /projects/{B}/deployments/search or
-- POST /deployments/{id}/products/search. Neither table had RLS, and their
-- repositories queried through the raw pool with no caller scope, so the only
-- thing "protecting" them was the caller not knowing another project's id.
--
-- Fix: same shape as every other project-owned table in this series.
--   * deployment.project_id is a real column: is_project_member(project_id).
--   * deployed_product.project_id is a real column too, but is NULL on rows
--     that were created before the column was populated (504 of 717 on the
--     dev-data copy). Those rows always have deployment_id, so fall back to
--     the owning deployment's project. The subselect only runs when
--     project_id is NULL, and it reads deployment through deployment's own
--     policy (no recursion: the deployment policy never reads
--     deployed_product).
--   * Rows whose project cannot be resolved at all (deployment.project_id
--     NULL: 1,764 rows on the copy, left behind when a project was deleted)
--     are visible to internal callers only. is_project_member(NULL) is not
--     true, so a customer never sees them; SearchDeployments already inner-
--     joins project and never returned them.
--
-- Every predicate uses the planner-friendly internal check introduced by
-- migration 100015, (SELECT current_setting('app.is_internal', true) = 'true'),
-- written out here because 0154 only rewrites policies that existed when it
-- ran.
--
-- Not covered here (follow-up): deployment_node, deployment_information and
-- the product metrics tables reach a project only through deployment_number /
-- node ids and are read by the instances endpoints. They returned no rows for
-- a foreign project in testing, but they have no RLS of their own yet.

-- One transaction: `make migrate` runs each file with `psql -f` and no
-- --single-transaction, so without this a failing CREATE POLICY would leave
-- FORCE ROW LEVEL SECURITY on a table with only some of its policies (every
-- missing one denies, so the table would silently stop working) and a retry
-- would then fail on "policy already exists". All or nothing instead.
--
-- Every CREATE POLICY is also preceded by its own DROP POLICY IF EXISTS:
-- the BEGIN/COMMIT above only protects against a partial failure within
-- THIS run -- it does nothing for a re-run against a database where these
-- policies were already applied by hand, outside csm_migration_applied_migration,
-- which would otherwise fail immediately on "policy already exists".
BEGIN;

ALTER TABLE deployment ENABLE ROW LEVEL SECURITY;
ALTER TABLE deployment FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS deployment_visibility ON deployment;
CREATE POLICY deployment_visibility ON deployment
  FOR SELECT
  USING (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR is_project_member(project_id)
  );

-- INSERT: a member may create a deployment in their own project (the portal's
-- "Add Deployment"), internal callers anywhere.
DROP POLICY IF EXISTS deployment_write ON deployment;
CREATE POLICY deployment_write ON deployment
  FOR INSERT
  WITH CHECK (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR is_project_member(project_id)
  );

-- UPDATE: USING sees the old row, WITH CHECK the new one; repeating the
-- membership test in both stops a caller moving a deployment out of, as well
-- as into, a project they belong to.
DROP POLICY IF EXISTS deployment_update ON deployment;
CREATE POLICY deployment_update ON deployment
  FOR UPDATE
  USING (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR is_project_member(project_id)
  )
  WITH CHECK (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR is_project_member(project_id)
  );

-- DELETE: nothing in production code deletes a deployment (deactivation is
-- is_active = FALSE), so internal-only, mirroring work_item_delete_internal_only.
DROP POLICY IF EXISTS deployment_delete_internal_only ON deployment;
CREATE POLICY deployment_delete_internal_only ON deployment
  FOR DELETE
  USING ((SELECT current_setting('app.is_internal', true) = 'true'));


ALTER TABLE deployed_product ENABLE ROW LEVEL SECURITY;
ALTER TABLE deployed_product FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS deployed_product_visibility ON deployed_product;
CREATE POLICY deployed_product_visibility ON deployed_product
  FOR SELECT
  USING (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR is_project_member(
         COALESCE(project_id, (SELECT d.project_id FROM deployment d WHERE d.id = deployed_product.deployment_id))
       )
  );

-- Write policies additionally require the row to be internally consistent:
-- deployed_product.project_id and deployment_id are separate foreign keys with
-- nothing tying them together, so without this a member could store their OWN
-- project_id next to ANOTHER project's deployment_id and pass the membership
-- test on project_id alone (found in review of #2094). The EXISTS reads
-- deployment as the invoker, so a foreign deployment is invisible to the
-- member and the row is rejected. Rows with a NULL project_id or NULL
-- deployment_id skip the check: their project is resolved from the other
-- column (COALESCE), so there is nothing to disagree with.
DROP POLICY IF EXISTS deployed_product_write ON deployed_product;
CREATE POLICY deployed_product_write ON deployed_product
  FOR INSERT
  WITH CHECK (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR (
      is_project_member(
        COALESCE(project_id, (SELECT d.project_id FROM deployment d WHERE d.id = deployed_product.deployment_id))
      )
      AND (
        deployed_product.project_id IS NULL
        OR deployed_product.deployment_id IS NULL
        OR EXISTS (
          SELECT 1 FROM deployment d
          WHERE d.id = deployed_product.deployment_id
            AND d.project_id = deployed_product.project_id
        )
      )
    )
  );

DROP POLICY IF EXISTS deployed_product_update ON deployed_product;
CREATE POLICY deployed_product_update ON deployed_product
  FOR UPDATE
  USING (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR (
      is_project_member(
        COALESCE(project_id, (SELECT d.project_id FROM deployment d WHERE d.id = deployed_product.deployment_id))
      )
      AND (
        deployed_product.project_id IS NULL
        OR deployed_product.deployment_id IS NULL
        OR EXISTS (
          SELECT 1 FROM deployment d
          WHERE d.id = deployed_product.deployment_id
            AND d.project_id = deployed_product.project_id
        )
      )
    )
  )
  WITH CHECK (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR (
      is_project_member(
        COALESCE(project_id, (SELECT d.project_id FROM deployment d WHERE d.id = deployed_product.deployment_id))
      )
      AND (
        deployed_product.project_id IS NULL
        OR deployed_product.deployment_id IS NULL
        OR EXISTS (
          SELECT 1 FROM deployment d
          WHERE d.id = deployed_product.deployment_id
            AND d.project_id = deployed_product.project_id
        )
      )
    )
  );

DROP POLICY IF EXISTS deployed_product_delete_internal_only ON deployed_product;
CREATE POLICY deployed_product_delete_internal_only ON deployed_product
  FOR DELETE
  USING ((SELECT current_setting('app.is_internal', true) = 'true'));

COMMIT;
