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

-- The audit log no longer records inserts (see 0295), so a row has to say for
-- itself who created it and when, and who last changed it and when. Three audited
-- tables lacked some of that; this adds it, as nullable columns:
--
--   work_item_watcher, case_escalation_notification_list   all four
--   comment                                                updated_on, updated_by
--
-- created_on and created_by are defaults, so no insert changes: now() and the
-- person behind the request (the audit trigger's actor rule, audit_actor()).
-- updated_on and updated_by are set by a BEFORE UPDATE trigger on every update, so
-- no update statement changes either; they stay NULL until a row is first
-- updated. Rows that exist today stay NULL: when they were created is not known.
--
-- Left as they are: the 14 work item type tables share the work_item id and
-- work_item holds all four columns; case_attachment has created_on, uploaded_by,
-- updated_on and updated_by; plg_note_revision is written once (edited_on,
-- edited_by); audit_config has registered_on. Safe to re-run.

-- The actor the audit trigger names: app.audit_actor, then app.viewer_email, then
-- the database role.
CREATE OR REPLACE FUNCTION audit_actor() RETURNS text
LANGUAGE sql STABLE AS $$
    SELECT COALESCE(NULLIF(pg_catalog.current_setting('app.audit_actor', true), ''),
                    NULLIF(pg_catalog.current_setting('app.viewer_email', true), ''),
                    session_user::text)
$$;

CREATE OR REPLACE FUNCTION audit_stamp_update() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_on := now();
    NEW.updated_by := audit_actor();
    RETURN NEW;
END $$;

DO $$
DECLARE
    t        RECORD;
BEGIN
    PERFORM set_config('lock_timeout', '5s', true);
    FOR t IN
        SELECT * FROM (VALUES
            ('work_item_watcher',                 true),
            ('case_escalation_notification_list', true),
            ('comment',                           false)
        ) AS v(table_name, add_created)
    LOOP
        IF to_regclass(format('%I.%I', current_schema(), t.table_name)) IS NULL THEN
            RAISE NOTICE 'audit: % does not exist in schema %, columns not added', t.table_name, current_schema();
            CONTINUE;
        END IF;

        IF t.add_created THEN
            EXECUTE format('ALTER TABLE %I.%I ADD COLUMN IF NOT EXISTS created_on TIMESTAMPTZ', current_schema(), t.table_name);
            EXECUTE format('ALTER TABLE %I.%I ADD COLUMN IF NOT EXISTS created_by VARCHAR(255)', current_schema(), t.table_name);
            EXECUTE format('ALTER TABLE %I.%I ALTER COLUMN created_on SET DEFAULT now()', current_schema(), t.table_name);
            EXECUTE format('ALTER TABLE %I.%I ALTER COLUMN created_by SET DEFAULT audit_actor()', current_schema(), t.table_name);
        END IF;

        EXECUTE format('ALTER TABLE %I.%I ADD COLUMN IF NOT EXISTS updated_on TIMESTAMPTZ', current_schema(), t.table_name);
        EXECUTE format('ALTER TABLE %I.%I ADD COLUMN IF NOT EXISTS updated_by VARCHAR(255)', current_schema(), t.table_name);

        EXECUTE format('DROP TRIGGER IF EXISTS csm_stamp_update ON %I.%I', current_schema(), t.table_name);
        EXECUTE format('CREATE TRIGGER csm_stamp_update BEFORE UPDATE ON %I.%I
                          FOR EACH ROW EXECUTE FUNCTION audit_stamp_update()', current_schema(), t.table_name);
    END LOOP;
END $$;
