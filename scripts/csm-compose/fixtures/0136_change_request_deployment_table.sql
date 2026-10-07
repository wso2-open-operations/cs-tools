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

-- LOCAL DEVELOPMENT ONLY. Not a migration: no database outside this compose stack runs it.
--
-- A local volume built BEFORE csm-sync-service's 0136_change_request_deployment_table.sql was
-- mirrored here already has change_request_deployment, in the shape this repository's own first
-- 0191 gave it: no `id`, and a composite PRIMARY KEY (change_request_id, deployment_id). The
-- sync's 0136 is `CREATE TABLE IF NOT EXISTS`, so on such a volume it is a no-op, and the table
-- never gains the `id UUID PRIMARY KEY` the sync (and entity-service's writer, which inserts
-- `id`) expect: every change request write then fails with `column "id" does not exist`.
--
-- This converges that old shape to the sync's, and only when `id` is missing -- on a fresh
-- database (0136 created the table) or a sync-shaped one it does nothing. It runs straight after
-- the 0136 file, in the same pass, so it lands before anything else touches the table. Safe to
-- re-run.
--
--   * The column is added WITH a gen_random_uuid() default, which Postgres evaluates once per
--     existing row while rewriting the table: every row keeps its (change_request_id,
--     deployment_id) pair and gets its own id, with no UPDATE. That matters because the table is
--     under FORCE ROW LEVEL SECURITY and has no UPDATE policy until migration 0205, so a plain
--     `UPDATE ... SET id = gen_random_uuid()` is a silent no-op for any role RLS applies to; the
--     rewrite is not subject to row-level security at all. The default is then dropped: the
--     sync's id has none ("expand_list always supplies a deterministic id").
--   * The composite PRIMARY KEY is replaced by PRIMARY KEY (id), and the sync's
--     UNIQUE (change_request_id, deployment_id) is added unless a unique index already covers
--     that pair, so the conflict target `ON CONFLICT (change_request_id, deployment_id)` keeps
--     working.
--
-- The two indexes the sync's 0136 also declares are created by that file itself, whatever the
-- table's earlier shape. What cannot be converged without rebuilding the table is the column
-- ORDER: ADD COLUMN appends, so `id` is the last column here and the first on a fresh database.
-- Nothing depends on it: every statement over this table names its columns.
DO $$
DECLARE
  tbl        regclass := to_regclass('change_request_deployment');
  pk_name    name;
  pair_cols  int2[];
BEGIN
  -- Nothing to converge when the table does not exist yet, or already has an id.
  IF tbl IS NULL OR EXISTS (
    SELECT 1 FROM pg_attribute
    WHERE attrelid = tbl AND attname = 'id' AND attnum > 0 AND NOT attisdropped
  ) THEN
    RETURN;
  END IF;

  ALTER TABLE change_request_deployment ADD COLUMN id UUID NOT NULL DEFAULT gen_random_uuid();
  ALTER TABLE change_request_deployment ALTER COLUMN id DROP DEFAULT;

  SELECT conname INTO pk_name FROM pg_constraint WHERE conrelid = tbl AND contype = 'p';
  IF pk_name IS NOT NULL THEN
    EXECUTE format('ALTER TABLE change_request_deployment DROP CONSTRAINT %I', pk_name);
  END IF;
  ALTER TABLE change_request_deployment ADD PRIMARY KEY (id);

  SELECT array_agg(attnum ORDER BY attnum) INTO pair_cols
  FROM pg_attribute
  WHERE attrelid = tbl AND attname IN ('change_request_id', 'deployment_id') AND NOT attisdropped;
  IF NOT EXISTS (
    SELECT 1 FROM pg_index i
    WHERE i.indrelid = tbl AND i.indisunique AND i.indpred IS NULL
      AND i.indnkeyatts = 2
      AND (SELECT array_agg(k ORDER BY k) FROM unnest(i.indkey::int2[]) AS k) = pair_cols
  ) THEN
    ALTER TABLE change_request_deployment ADD UNIQUE (change_request_id, deployment_id);
  END IF;
END $$;
