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

-- Start auditing the core people, access and customer tables (see 0295): who a
-- user is, which roles they hold, and who belongs to which account and project.
-- Each is registered with its primary key as the record id and every column
-- recorded, except account.sync_time_stamp: the sync jobs rewrite it on every run,
-- so recording it would add a row per account per run and no business change.
-- project_contact_group is the table that gives a project contact their groups,
-- and so their project roles.
--
-- A table is registered only when it has no audit_config row yet, so a
-- registration made by hand, or a table an admin turned off with
-- unregister_audit, is left as it is. A table missing from the schema is skipped
-- with a notice.
DO $$
DECLARE
    t RECORD;
BEGIN
    FOR t IN
        SELECT * FROM (VALUES
            ('account',               ARRAY['sync_time_stamp']),
            ('account_contact',       '{}'::text[]),
            ('project',               '{}'::text[]),
            ('project_contact',       '{}'::text[]),
            ('project_contact_group', '{}'::text[]),
            ('user',                  '{}'::text[]),
            ('role',                  '{}'::text[]),
            ('user_role',             '{}'::text[])
        ) AS v(table_name, excluded)
    LOOP
        IF to_regclass(format('%I.%I', current_schema(), t.table_name)) IS NULL THEN
            RAISE NOTICE 'audit: % does not exist in schema %, not registered', t.table_name, current_schema();
        ELSIF NOT EXISTS (SELECT 1 FROM audit_config
                           WHERE schema_name = current_schema() AND table_name = t.table_name) THEN
            PERFORM register_audit(current_schema(), t.table_name, NULL, t.excluded, '{}');
        END IF;
    END LOOP;
END $$;
