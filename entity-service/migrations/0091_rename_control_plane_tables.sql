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

-- Brings every control-plane table onto one shared csm_migration_ prefix, so
-- the whole control plane reads as one family of tables. The 6th
-- control-plane table, csm_migration_applied_migration (make migrate's own
-- tracking table), is renamed the same way but via Makefile, not here - see
-- CLAUDE.md §4. IF EXISTS makes each rename a no-op on an environment where
-- it's already run (or where 0001_control_plane.sql, in a future fresh
-- install, is edited to create the new names directly) - safe to re-run.
--
-- Each rename is additionally guarded on the new name NOT already existing:
-- on a database whose csm_migration_applied_migration tracking table was
-- reset (or never existed) while the schema itself was already fully built
-- - this repo's own real-world failure mode, not hypothetical - 0001 above
-- runs again first and its CREATE TABLE IF NOT EXISTS recreates an empty
-- migration_job (etc.) under the old name, since that name no longer exists
-- post-rename. Without this guard the plain ALTER TABLE IF EXISTS ... RENAME
-- below would then fail with "relation csm_migration_job already exists"
-- rather than silently doing nothing, which is what re-running an
-- already-applied rename should do.
DO $$ BEGIN
    IF to_regclass('csm_migration_job') IS NULL THEN
        ALTER TABLE IF EXISTS migration_job RENAME TO csm_migration_job;
    END IF;
END $$;

DO $$ BEGIN
    IF to_regclass('csm_migration_run') IS NULL THEN
        ALTER TABLE IF EXISTS migration_run RENAME TO csm_migration_run;
    END IF;
END $$;

DO $$ BEGIN
    IF to_regclass('csm_migration_row_error') IS NULL THEN
        ALTER TABLE IF EXISTS migration_row_error RENAME TO csm_migration_row_error;
    END IF;
END $$;

DO $$ BEGIN
    IF to_regclass('csm_migration_checkpoint') IS NULL THEN
        ALTER TABLE IF EXISTS sync_checkpoint RENAME TO csm_migration_checkpoint;
    END IF;
END $$;

DO $$ BEGIN
    IF to_regclass('csm_migration_schema_version') IS NULL THEN
        ALTER TABLE IF EXISTS schema_version RENAME TO csm_migration_schema_version;
    END IF;
END $$;
