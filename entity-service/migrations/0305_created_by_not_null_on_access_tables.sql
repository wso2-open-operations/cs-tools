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

-- Make created_by mandatory on role, team, team_member, "user" and user_role.
-- The audit log does not record inserts (see 0295), so a row's own created_by is
-- the only record of who added it, and these five tables still allowed it to be
-- NULL although no row has one.
--
-- created_by also gets DEFAULT audit_actor() (0304), the audit trigger's own rule
-- for naming the person, so an insert that leaves the column out is stamped with
-- the actor instead of failing. Without it, such inserts (assigning a team lead,
-- the local seed data) would break.
--
-- A table that still holds a NULL stops the migration before anything changes,
-- with the table and the number of rows, so they can be filled in first. Tables
-- are changed in one transaction, so it can be run again. A table missing from the
-- schema is skipped with a notice.
DO $$
DECLARE
    t           TEXT;
    null_rows   BIGINT;
BEGIN
    -- The change takes a strong lock on each table: give up after 5s rather than
    -- queue behind its writers.
    PERFORM set_config('lock_timeout', '5s', true);

    FOREACH t IN ARRAY ARRAY['role', 'team', 'team_member', 'user', 'user_role'] LOOP
        IF to_regclass(format('%I.%I', current_schema(), t)) IS NULL THEN
            RAISE NOTICE 'created_by: % does not exist in schema %, skipped', t, current_schema();
            CONTINUE;
        END IF;

        EXECUTE format('SELECT count(*) FROM %I.%I WHERE created_by IS NULL', current_schema(), t)
           INTO null_rows;
        IF null_rows > 0 THEN
            RAISE EXCEPTION 'created_by is NULL on % row(s) of %: fill them in before making it mandatory', null_rows, t;
        END IF;

        EXECUTE format('ALTER TABLE %I.%I ALTER COLUMN created_by SET DEFAULT audit_actor()', current_schema(), t);
        EXECUTE format('ALTER TABLE %I.%I ALTER COLUMN created_by SET NOT NULL', current_schema(), t);
    END LOOP;
END $$;
