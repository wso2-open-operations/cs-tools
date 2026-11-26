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

-- A delete is logged as ONE row per deleted record, not one per column: field_name
-- is '__row__', action DELETE, and old_value holds the whole old row as a JSON
-- object (new_value is NULL). The row stays readable after it is gone: who was
-- removed from a watch list, which case a comment was on. The same rules as for an
-- update apply to each column inside it: NULL columns, excluded columns and
-- updated_on / updated_by are left out, a masked column reads "[hidden]", and a
-- value over 8192 bytes reads "[value too large]" (each column on its own, so one
-- large field does not hide the rest). An update still logs one row per changed
-- field. Replaces the function installed by 0295 and changed by 0306, in place, so
-- every existing trigger picks it up and keeps its grants. Safe to re-run.

-- This replaces the function in the schema the migration runs in. Stop with a clear
-- message when that is not the schema holding the audit log (a session whose
-- search_path points elsewhere), instead of creating a second, unused copy there.
DO $$
DECLARE
    found_in TEXT;
BEGIN
    IF to_regprocedure(format('%I.audit_log_row_change()', current_schema())) IS NULL THEN
        SELECT string_agg(DISTINCT n.nspname, ', ' ORDER BY n.nspname) INTO found_in
          FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
         WHERE p.proname = 'audit_log_row_change' AND p.pronargs = 0;
        IF found_in IS NULL THEN
            RAISE EXCEPTION 'audit: audit_log_row_change() exists in no schema of this database. Apply 0295 first.';
        END IF;
        RAISE EXCEPTION 'audit: audit_log_row_change() is not in the current schema "%". It is in: %. SET search_path TO <that schema> and run this again.',
            current_schema(), found_in;
    END IF;
END $$;

-- The trigger function shared by every registered table, as 0306 defined it (a
-- failed log write does not undo the business change), now logging a delete as one
-- row.
-- Arguments (fixed when the trigger is created): key, excluded and masked
-- columns, each comma-separated. SECURITY DEFINER, so callers need no INSERT right.
DO $install$
BEGIN
    EXECUTE replace(replace($fn$
CREATE OR REPLACE FUNCTION audit_log_row_change() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER
SET search_path = pg_catalog, __SCHEMA__, pg_temp AS $$
DECLARE
    failed_error   TEXT;
    failed_state   TEXT;
    failed_record  TEXT := '(unknown)';
    failed_columns TEXT := '(unknown)';
    failed_old     JSONB;
    failed_new     JSONB;
BEGIN
    -- A failure to write the log must not undo the business change, so everything
    -- below runs in a block with a handler. The block is a subtransaction: a row
    -- change that fails part way logs none of its fields. The handler leaves no row
    -- behind (the log is what failed), only one WARNING per row change in the
    -- database log, naming the table, the record and the columns, never their
    -- values; alert on "audit: could not log".
    DECLARE
        max_value_bytes CONSTANT INT := 8192;
        key_columns      TEXT[] := string_to_array(TG_ARGV[0], ',');
        -- updated_on and updated_by are always left out, whatever the table registered.
        excluded_columns TEXT[] := COALESCE(string_to_array(NULLIF(TG_ARGV[1], ''), ','), '{}')
                                   || ARRAY['updated_on', 'updated_by'];
        masked_columns   TEXT[] := COALESCE(string_to_array(NULLIF(TG_ARGV[2], ''), ','), '{}');
        old_row    JSONB  := CASE WHEN TG_OP <> 'INSERT' THEN to_jsonb(OLD) END;
        new_row    JSONB  := CASE WHEN TG_OP <> 'DELETE' THEN to_jsonb(NEW) END;
        record_id  TEXT   := audit_record_key(COALESCE(new_row, old_row), key_columns);
        -- app.audit_actor is the person behind the request even when the code runs
        -- with an escalated (system) identity, which clears app.viewer_email.
        actor      TEXT   := COALESCE(NULLIF(current_setting('app.audit_actor', true), ''),
                                      NULLIF(current_setting('app.viewer_email', true), ''),
                                      session_user::text);
        request_id TEXT   := NULLIF(current_setting('app.request_id', true), '');
        field      TEXT;
        old_value  JSONB;
        new_value  JSONB;
        deleted_row JSONB;
    BEGIN
        IF TG_OP = 'DELETE' THEN
            -- One row for the whole delete: the old row as a JSON object, without its
            -- NULL and excluded columns, masked and oversized columns replaced by a
            -- marker each.
            SELECT COALESCE(jsonb_object_agg(f.name,
                       CASE WHEN f.name = ANY (masked_columns) THEN '"[hidden]"'::jsonb
                            WHEN octet_length(f.value::text) > max_value_bytes THEN '"[value too large]"'::jsonb
                            ELSE f.value END), '{}'::jsonb)
              INTO deleted_row
              FROM jsonb_each(old_row) AS f(name, value)
             WHERE f.name <> ALL (excluded_columns) AND f.value <> 'null'::jsonb;

            INSERT INTO __SCHEMA__.csm_audit_log
                (schema_name, table_name, record_id, field_name, action, old_value, new_value,
                 created_by, db_role, transaction_id, request_id)
            VALUES
                (TG_TABLE_SCHEMA, TG_TABLE_NAME, record_id, '__row__', 'DELETE', deleted_row, NULL,
                 actor, session_user::text, txid_current(), request_id);
        ELSE
            FOR field IN SELECT jsonb_object_keys(COALESCE(new_row, old_row)) LOOP
                CONTINUE WHEN field = ANY (excluded_columns);

                old_value := old_row -> field;
                new_value := new_row -> field;

                IF TG_OP = 'UPDATE' AND NOT (old_value IS DISTINCT FROM new_value) THEN
                    CONTINUE;  -- unchanged
                END IF;
                IF TG_OP <> 'UPDATE' AND COALESCE(old_value, new_value) = 'null'::jsonb THEN
                    CONTINUE;  -- a NULL column on insert says nothing
                END IF;

                IF field = ANY (masked_columns) THEN
                    old_value := CASE WHEN old_value IN ('null'::jsonb) OR old_value IS NULL THEN old_value ELSE '"[hidden]"' END;
                    new_value := CASE WHEN new_value IN ('null'::jsonb) OR new_value IS NULL THEN new_value ELSE '"[hidden]"' END;
                END IF;
                IF octet_length(old_value::text) > max_value_bytes THEN old_value := '"[value too large]"'; END IF;
                IF octet_length(new_value::text) > max_value_bytes THEN new_value := '"[value too large]"'; END IF;

                INSERT INTO __SCHEMA__.csm_audit_log
                    (schema_name, table_name, record_id, field_name, action, old_value, new_value,
                     created_by, db_role, transaction_id, request_id)
                VALUES
                    (TG_TABLE_SCHEMA, TG_TABLE_NAME, record_id, field, TG_OP, old_value, new_value,
                     actor, session_user::text, txid_current(), request_id);
            END LOOP;
        END IF;

        -- A changed key is also logged under the old key, so history can be followed.
        IF TG_OP = 'UPDATE' AND audit_record_key(old_row, key_columns) <> record_id THEN
            INSERT INTO __SCHEMA__.csm_audit_log
                (schema_name, table_name, record_id, field_name, action, old_value, new_value,
                 created_by, db_role, transaction_id, request_id)
            VALUES
                (TG_TABLE_SCHEMA, TG_TABLE_NAME, audit_record_key(old_row, key_columns), '__record_key__', 'UPDATE',
                 to_jsonb(audit_record_key(old_row, key_columns)), to_jsonb(record_id),
                 actor, session_user::text, txid_current(), request_id);
        END IF;
    END;

    RETURN NULL;
EXCEPTION WHEN OTHERS THEN
    failed_error := SQLERRM;
    failed_state := SQLSTATE;
    -- Name the record and the columns that were to be logged. Working them out can
    -- fail for the same reason the log did, so it has a handler of its own and the
    -- warning still goes out with "(unknown)".
    BEGIN
        failed_old := CASE WHEN TG_OP <> 'INSERT' THEN to_jsonb(OLD) END;
        failed_new := CASE WHEN TG_OP <> 'DELETE' THEN to_jsonb(NEW) END;
        failed_record := audit_record_key(COALESCE(failed_new, failed_old), string_to_array(TG_ARGV[0], ','));
        SELECT left(string_agg(f.name, ', ' ORDER BY f.name), 300) INTO failed_columns
          FROM jsonb_object_keys(COALESCE(failed_new, failed_old)) AS f(name)
         WHERE f.name <> ALL (COALESCE(string_to_array(NULLIF(TG_ARGV[1], ''), ','), '{}')
                              || ARRAY['updated_on', 'updated_by'])
           AND CASE WHEN TG_OP = 'UPDATE' THEN (failed_old -> f.name) IS DISTINCT FROM (failed_new -> f.name)
                    ELSE (COALESCE(failed_old, failed_new) -> f.name) IS DISTINCT FROM 'null'::jsonb END;
    EXCEPTION WHEN OTHERS THEN
        NULL;
    END;
    RAISE WARNING 'audit: could not log % on %.% record % columns [%]: % (SQLSTATE %)',
        TG_OP, TG_TABLE_SCHEMA, TG_TABLE_NAME, failed_record, COALESCE(failed_columns, '(none)'),
        failed_error, failed_state;
    RETURN NULL;
END $$;
$fn$,
        '__SCHEMA__', quote_ident(current_schema())),
        '__SCHEMA_LIT__', quote_literal(current_schema()));
END $install$;
