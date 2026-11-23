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

-- Field-level audit log. An AFTER UPDATE/DELETE row trigger writes one row per
-- changed field into csm_audit_log, in the same transaction as the change, for the
-- tables listed in audit_config. Each business table calls register_audit().
--
-- Inserts are not audited: a row's own created_by and created_on already say who
-- added it. updated_on and updated_by are never logged either: every log row
-- already has the actor and the time, and jobs that rewrite them on every run
-- would only add noise. The trigger function still accepts INSERT and the action
-- check still allows 'INSERT', so a database with older triggers keeps working.
-- Requires PostgreSQL 18 (uuidv7()). Safe to re-run.
--
-- Everything is created in the schema this migration runs in. The SECURITY DEFINER
-- functions are installed through the DO blocks below, which replace __SCHEMA__ and
-- __SCHEMA_LIT__ with that schema, so the file works in public or in a schema of
-- its own.

-- Registry: what is audited.
CREATE TABLE IF NOT EXISTS audit_config (
    schema_name       TEXT        NOT NULL,
    table_name        TEXT        NOT NULL,
    key_columns       TEXT[]      NOT NULL DEFAULT '{}',  -- columns that identify a row
    excluded_columns  TEXT[]      NOT NULL DEFAULT '{}',  -- never logged
    masked_columns    TEXT[]      NOT NULL DEFAULT '{}',  -- logged as changed, value hidden
    enabled           BOOLEAN     NOT NULL DEFAULT true,
    reason            TEXT,                               -- why auditing is off
    registered_on     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (schema_name, table_name),
    CHECK (enabled OR reason IS NOT NULL),
    CHECK (NOT enabled OR cardinality(key_columns) > 0)
);

-- Audit log, one partition per UTC month. The partition column has to be in the
-- primary key, so id alone is not enforced unique across months (UUIDv7 makes a
-- clash practically impossible).
CREATE TABLE IF NOT EXISTS csm_audit_log (
    id             UUID        NOT NULL DEFAULT uuidv7(),
    created_on     TIMESTAMPTZ NOT NULL DEFAULT now(),
    schema_name    TEXT        NOT NULL,
    table_name     TEXT        NOT NULL,
    record_id      TEXT        NOT NULL,
    field_name     TEXT        NOT NULL,
    action         TEXT        NOT NULL CHECK (action IN ('INSERT', 'UPDATE', 'DELETE')),
    old_value      JSONB,
    new_value      JSONB,
    created_by     TEXT        NOT NULL,
    db_role        TEXT        NOT NULL,
    transaction_id BIGINT      NOT NULL,
    request_id     TEXT,
    PRIMARY KEY (id, created_on)
) PARTITION BY RANGE (created_on);

-- Catch-all, so a missed month never fails a business write. Rows here are an alert.
CREATE TABLE IF NOT EXISTS csm_audit_log_default PARTITION OF csm_audit_log DEFAULT;

-- History of one field on one record, newest first.
CREATE INDEX IF NOT EXISTS idx_csm_audit_log_record_field
    ON csm_audit_log (table_name, record_id, field_name, created_on DESC);
-- What did this person change?
CREATE INDEX IF NOT EXISTS idx_csm_audit_log_created_by
    ON csm_audit_log (created_by, created_on DESC);

-- Access: nobody writes except the trigger; a separate role reads.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'csm_audit_reader') THEN
        CREATE ROLE csm_audit_reader NOLOGIN;
    END IF;
EXCEPTION WHEN insufficient_privilege THEN
    RAISE NOTICE 'audit: cannot create role csm_audit_reader; create it and grant SELECT on csm_audit_log';
END $$;

REVOKE ALL ON csm_audit_log, audit_config FROM PUBLIC;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'csm_audit_reader') THEN
        GRANT SELECT ON csm_audit_log TO csm_audit_reader;
    END IF;
END $$;

-- The log is append-only, even for its owner. Retiring a partition (detach, then
-- drop) fires no row triggers, so it needs no exception.
CREATE OR REPLACE FUNCTION audit_log_reject_change() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'csm_audit_log is append-only: % is not allowed', TG_OP
        USING ERRCODE = 'insufficient_privilege';
END $$;

DROP TRIGGER IF EXISTS csm_audit_log_no_row_change ON csm_audit_log;
CREATE TRIGGER csm_audit_log_no_row_change
    BEFORE UPDATE OR DELETE ON csm_audit_log
    FOR EACH ROW EXECUTE FUNCTION audit_log_reject_change();

DROP TRIGGER IF EXISTS csm_audit_log_no_truncate ON csm_audit_log;
CREATE TRIGGER csm_audit_log_no_truncate
    BEFORE TRUNCATE ON csm_audit_log
    FOR EACH STATEMENT EXECUTE FUNCTION audit_log_reject_change();

-- The statement trigger above does not reach the partitions, and TRUNCATE on a
-- partition skips the parent's, so every partition gets its own guard. New
-- months get theirs in audit_create_partitions.
DO $$
DECLARE
    part regclass;
BEGIN
    FOR part IN SELECT inhrelid::regclass FROM pg_inherits WHERE inhparent = 'csm_audit_log'::regclass LOOP
        EXECUTE format('DROP TRIGGER IF EXISTS csm_audit_log_no_truncate ON %s', part);
        EXECUTE format('CREATE TRIGGER csm_audit_log_no_truncate BEFORE TRUNCATE ON %s
                          FOR EACH STATEMENT EXECUTE FUNCTION audit_log_reject_change()', part);
    END LOOP;
END $$;

-- A row's key as text: the plain value for a one-column key ('42'), a JSON array
-- for a composite one ('["acme","prod"]'), so two different keys never give the
-- same text.
CREATE OR REPLACE FUNCTION audit_record_key(p_row JSONB, p_key_columns TEXT[]) RETURNS TEXT
LANGUAGE sql IMMUTABLE AS $$
    SELECT CASE WHEN cardinality(p_key_columns) = 1
                THEN COALESCE(p_row ->> p_key_columns[1], '')
                ELSE (SELECT jsonb_agg(COALESCE(p_row -> k, 'null'::jsonb) ORDER BY ord)::text
                        FROM unnest(p_key_columns) WITH ORDINALITY AS t(k, ord))
           END
$$;

-- The trigger function shared by every registered table.
-- Arguments (fixed when the trigger is created): key, excluded and masked
-- columns, each comma-separated. SECURITY DEFINER, so callers need no INSERT right.
DO $install$
BEGIN
    EXECUTE replace(replace($fn$
CREATE OR REPLACE FUNCTION audit_log_row_change() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER
SET search_path = pg_catalog, __SCHEMA__, pg_temp AS $$
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
BEGIN
    FOR field IN SELECT jsonb_object_keys(COALESCE(new_row, old_row)) LOOP
        CONTINUE WHEN field = ANY (excluded_columns);

        old_value := old_row -> field;
        new_value := new_row -> field;

        IF TG_OP = 'UPDATE' AND NOT (old_value IS DISTINCT FROM new_value) THEN
            CONTINUE;  -- unchanged
        END IF;
        IF TG_OP <> 'UPDATE' AND COALESCE(old_value, new_value) = 'null'::jsonb THEN
            CONTINUE;  -- a NULL column on insert or delete says nothing
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

    RETURN NULL;
END $$;
$fn$,
        '__SCHEMA__', quote_ident(current_schema())),
        '__SCHEMA_LIT__', quote_literal(current_schema()));
END $install$;

-- Registration. Only these functions write audit_config.
DO $install$
BEGIN
    EXECUTE replace(replace($fn$
CREATE OR REPLACE FUNCTION register_audit(
    p_schema   TEXT,
    p_table    TEXT,
    p_keys     TEXT[] DEFAULT NULL,  -- default: the table's primary key
    p_excluded TEXT[] DEFAULT '{}',
    p_masked   TEXT[] DEFAULT '{}'
) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER
SET search_path = pg_catalog, __SCHEMA__, pg_temp AS $$
DECLARE
    table_oid  regclass := format('%I.%I', p_schema, p_table)::regclass;
    keys       TEXT[]   := p_keys;
    excluded   TEXT[];
    bad_columns TEXT[];
BEGIN
    IF p_table LIKE 'csm_audit_log%' THEN
        RAISE EXCEPTION 'audit: the audit log must not audit itself';
    END IF;

    IF keys IS NULL THEN
        SELECT array_agg(a.attname::text ORDER BY k.ord) INTO keys
          FROM pg_index i
          CROSS JOIN LATERAL unnest(i.indkey) WITH ORDINALITY AS k(attnum, ord)
          JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = k.attnum
         WHERE i.indrelid = table_oid AND i.indisprimary
           AND k.ord <= i.indnkeyatts;  -- key columns only, not INCLUDE columns
    END IF;
    IF keys IS NULL THEN
        RAISE EXCEPTION 'audit: %.% has no primary key; pass p_keys', p_schema, p_table;
    END IF;
    -- The record id is built from the key columns and stored in clear.
    IF keys && (p_excluded || p_masked) THEN
        RAISE EXCEPTION 'audit: a key column of %.% cannot be excluded or masked', p_schema, p_table;
    END IF;

    -- The trigger gets these lists comma-separated, so a name with a comma would be
    -- split and its column logged in clear.
    IF EXISTS (SELECT 1 FROM unnest(keys || p_excluded || p_masked) AS c WHERE c LIKE '%,%') THEN
        RAISE EXCEPTION 'audit: a column name with a comma on %.% cannot be audited', p_schema, p_table;
    END IF;

    SELECT array_agg(c) INTO bad_columns
      FROM unnest(keys || p_excluded || p_masked) AS c
     WHERE NOT EXISTS (SELECT 1 FROM pg_attribute a
                        WHERE a.attrelid = table_oid AND a.attname = c
                          AND a.attnum > 0 AND NOT a.attisdropped);
    IF bad_columns IS NOT NULL THEN
        RAISE EXCEPTION 'audit: unknown column(s) % on %.%', bad_columns, p_schema, p_table;
    END IF;

    -- Binary columns are never logged.
    SELECT p_excluded || COALESCE(array_agg(a.attname::text), '{}') INTO excluded
      FROM pg_attribute a
     WHERE a.attrelid = table_oid AND a.attnum > 0 AND NOT a.attisdropped
       AND a.atttypid = 'bytea'::regtype AND NOT (a.attname = ANY (p_excluded));
    -- A binary key would be left out of the fields but still copied into the record id.
    IF keys && excluded THEN
        RAISE EXCEPTION 'audit: a key column of %.% is binary and cannot be used as the record id', p_schema, p_table;
    END IF;

    INSERT INTO audit_config (schema_name, table_name, key_columns, excluded_columns, masked_columns)
    VALUES (p_schema, p_table, keys, excluded, p_masked)
    ON CONFLICT (schema_name, table_name) DO UPDATE
       SET key_columns = EXCLUDED.key_columns,
           excluded_columns = EXCLUDED.excluded_columns,
           masked_columns = EXCLUDED.masked_columns,
           enabled = true, reason = NULL;

    -- The settings live in the trigger, so a change rebuilds it.
    EXECUTE format('DROP TRIGGER IF EXISTS csm_audit_row_change ON %s', table_oid);
    -- Updates and deletes only: an insert is not audited, for any table.
    EXECUTE format(
        'CREATE TRIGGER csm_audit_row_change AFTER UPDATE OR DELETE ON %s
           FOR EACH ROW EXECUTE FUNCTION audit_log_row_change(%L, %L, %L)',
        table_oid, array_to_string(keys, ','), array_to_string(excluded, ','), array_to_string(p_masked, ','));
    -- ALWAYS, so it still fires when session_replication_role = replica.
    EXECUTE format('ALTER TABLE %s ENABLE ALWAYS TRIGGER csm_audit_row_change', table_oid);
END $$;
$fn$,
        '__SCHEMA__', quote_ident(current_schema())),
        '__SCHEMA_LIT__', quote_literal(current_schema()));
END $install$;

-- Turn auditing off. A reason is required; old history stays.
DO $install$
BEGIN
    EXECUTE replace(replace($fn$
CREATE OR REPLACE FUNCTION unregister_audit(p_schema TEXT, p_table TEXT, p_reason TEXT) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER
SET search_path = pg_catalog, __SCHEMA__, pg_temp AS $$
DECLARE
    table_oid regclass := to_regclass(format('%I.%I', p_schema, p_table));
BEGIN
    IF NULLIF(btrim(p_reason), '') IS NULL THEN
        RAISE EXCEPTION 'audit: a reason is required to turn auditing off';
    END IF;

    UPDATE audit_config SET enabled = false, reason = p_reason
     WHERE schema_name = p_schema AND table_name = p_table;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'audit: %.% is not registered', p_schema, p_table;
    END IF;

    IF table_oid IS NOT NULL THEN
        EXECUTE format('DROP TRIGGER IF EXISTS csm_audit_row_change ON %s', table_oid);
    END IF;
END $$;
$fn$,
        '__SCHEMA__', quote_ident(current_schema())),
        '__SCHEMA_LIT__', quote_literal(current_schema()));
END $install$;

-- Mark a table as deliberately not audited (staging, queues, ...).
DO $install$
BEGIN
    EXECUTE replace(replace($fn$
CREATE OR REPLACE FUNCTION exclude_from_audit(p_schema TEXT, p_table TEXT, p_reason TEXT) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER
SET search_path = pg_catalog, __SCHEMA__, pg_temp AS $$
DECLARE
    table_oid regclass := to_regclass(format('%I.%I', p_schema, p_table));
BEGIN
    IF NULLIF(btrim(p_reason), '') IS NULL THEN
        RAISE EXCEPTION 'audit: a reason is required to exclude a table';
    END IF;

    INSERT INTO audit_config (schema_name, table_name, enabled, reason)
    VALUES (p_schema, p_table, false, p_reason)
    ON CONFLICT (schema_name, table_name) DO UPDATE SET enabled = false, reason = EXCLUDED.reason;

    -- A table that was audited stops being audited, as in unregister_audit.
    IF table_oid IS NOT NULL THEN
        EXECUTE format('DROP TRIGGER IF EXISTS csm_audit_row_change ON %s', table_oid);
    END IF;
END $$;
$fn$,
        '__SCHEMA__', quote_ident(current_schema())),
        '__SCHEMA_LIT__', quote_literal(current_schema()));
END $install$;

-- Partitions. Safe to run twice and from several replicas (advisory lock).
-- Month boundaries are UTC.
DO $install$
BEGIN
    EXECUTE replace(replace($fn$
CREATE OR REPLACE FUNCTION audit_create_partitions(p_months_ahead INT DEFAULT 3) RETURNS INT
LANGUAGE plpgsql SECURITY DEFINER
SET search_path = pg_catalog, __SCHEMA__, pg_temp AS $$
DECLARE
    this_month     DATE := date_trunc('month', now() AT TIME ZONE 'UTC')::date;
    month_start    DATE;
    month_end      DATE;
    partition_name TEXT;
    created_count  INT := 0;
BEGIN
    -- Give up instead of queueing behind audit writes (the caller retries). Set
    -- before the advisory lock so it covers that wait too.
    PERFORM set_config('lock_timeout', '2s', true);
    PERFORM pg_advisory_xact_lock(hashtextextended('csm_audit_log_partitions', 0));

    FOR i IN 0..p_months_ahead LOOP
        month_start    := (this_month + make_interval(months => i))::date;
        month_end      := (this_month + make_interval(months => i + 1))::date;
        partition_name := 'csm_audit_log_' || to_char(month_start, 'YYYY_MM');

        IF to_regclass(format('%I.%I', __SCHEMA_LIT__, partition_name)) IS NULL THEN
            -- Rows for this month in the catch-all would make the CREATE fail and stop
            -- the later months too.
            IF EXISTS (SELECT 1 FROM csm_audit_log_default
                        WHERE created_on >= month_start::timestamp AT TIME ZONE 'UTC'
                          AND created_on <  month_end::timestamp AT TIME ZONE 'UTC') THEN
                RAISE WARNING 'audit: not creating %: rows for that month are in csm_audit_log_default; move them first', partition_name;
                CONTINUE;
            END IF;

            BEGIN
                -- The schema goes in as an argument, never into the format string.
                EXECUTE format(
                    'CREATE TABLE %1$I.%2$I PARTITION OF %1$I.csm_audit_log FOR VALUES FROM (%3$L) TO (%4$L)',
                    __SCHEMA_LIT__, partition_name, month_start || ' 00:00:00+00', month_end || ' 00:00:00+00');
                EXECUTE format(
                    'CREATE TRIGGER csm_audit_log_no_truncate BEFORE TRUNCATE ON %I.%I
                       FOR EACH STATEMENT EXECUTE FUNCTION audit_log_reject_change()',
                    __SCHEMA_LIT__, partition_name);
                created_count := created_count + 1;
            EXCEPTION WHEN OTHERS THEN
                -- A write into the catch-all between the check above and the CREATE makes
                -- the CREATE fail. Skip the month only when those rows explain it, as the
                -- check above does; any other error is raised.
                IF EXISTS (SELECT 1 FROM csm_audit_log_default
                            WHERE created_on >= month_start::timestamp AT TIME ZONE 'UTC'
                              AND created_on <  month_end::timestamp AT TIME ZONE 'UTC') THEN
                    RAISE WARNING 'audit: not creating %: rows for that month reached csm_audit_log_default first; move them', partition_name;
                ELSE
                    RAISE;
                END IF;
            END;
        END IF;
    END LOOP;

    RETURN created_count;
END $$;
$fn$,
        '__SCHEMA__', quote_ident(current_schema())),
        '__SCHEMA_LIT__', quote_literal(current_schema()));
END $install$;

-- First month to keep: anything that ended before it is past retention. Refuses a
-- retention below 1 month, which would delete everything.
CREATE OR REPLACE FUNCTION audit_retention_cutoff(p_retention_months INT) RETURNS TIMESTAMPTZ
LANGUAGE plpgsql STABLE AS $$
BEGIN
    IF p_retention_months IS NULL OR p_retention_months < 1 THEN
        RAISE EXCEPTION 'audit: retention must be at least 1 month, got %', p_retention_months;
    END IF;
    RETURN (date_trunc('month', now() AT TIME ZONE 'UTC') - make_interval(months => p_retention_months)) AT TIME ZONE 'UTC';
END $$;

-- Delete the monthly partitions that ended before the retention cutoff and return
-- their names. Each is detached and dropped in one transaction, so a failed drop puts
-- it back. Nothing is archived. The detach locks csm_audit_log briefly; lock_timeout
-- makes it give up instead of blocking writes, and the next run tries again.
DROP FUNCTION IF EXISTS audit_retire_partitions(INT, TEXT);
DO $install$
BEGIN
    EXECUTE replace(replace($fn$
CREATE OR REPLACE FUNCTION audit_delete_expired_partitions(
    p_retention_months INT  DEFAULT 18,
    p_lock_timeout     TEXT DEFAULT '2s'
) RETURNS SETOF TEXT
LANGUAGE plpgsql SECURITY DEFINER
SET search_path = pg_catalog, __SCHEMA__, pg_temp AS $$
DECLARE
    cutoff        TIMESTAMPTZ := audit_retention_cutoff(p_retention_months);
    old_partition TEXT;
BEGIN
    -- Set before the advisory lock, so the timeout covers waiting for it too.
    PERFORM set_config('lock_timeout', p_lock_timeout, true);
    PERFORM pg_advisory_xact_lock(hashtextextended('csm_audit_log_partitions', 0));

    -- The end of each partition is read from its real range, not its name.
    FOR old_partition IN
        SELECT c.relname
          FROM pg_inherits i
          JOIN pg_class c ON c.oid = i.inhrelid
         WHERE i.inhparent = format('%I.csm_audit_log', __SCHEMA_LIT__)::regclass
           AND pg_get_expr(c.relpartbound, c.oid) <> 'DEFAULT'
           AND (regexp_match(pg_get_expr(c.relpartbound, c.oid), 'TO \(''([^'']+)''\)'))[1]::timestamptz <= cutoff
    LOOP
        EXECUTE format('ALTER TABLE %1$I.csm_audit_log DETACH PARTITION %1$I.%2$I', __SCHEMA_LIT__, old_partition);
        EXECUTE format('DROP TABLE %1$I.%2$I', __SCHEMA_LIT__, old_partition);
        RETURN NEXT old_partition;
    END LOOP;
END $$;
$fn$,
        '__SCHEMA__', quote_ident(current_schema())),
        '__SCHEMA_LIT__', quote_literal(current_schema()));
END $install$;

-- Delete the catch-all partition's rows that are past retention and return how many.
-- The log is append-only, so a DELETE is refused: the catch-all is swapped instead.
-- Detach it, create a fresh one, copy the younger rows back with one INSERT ... SELECT
-- and drop the old table, in one transaction. If the counts do not add up it rolls
-- back. With nothing expired, the usual case, it takes no table lock.
DROP FUNCTION IF EXISTS audit_purge_default_partition(INT, TEXT);
DO $install$
BEGIN
    EXECUTE replace(replace($fn$
CREATE OR REPLACE FUNCTION audit_delete_expired_default_rows(
    p_retention_months INT  DEFAULT 18,
    p_lock_timeout     TEXT DEFAULT '2s'
) RETURNS BIGINT
LANGUAGE plpgsql SECURITY DEFINER
SET search_path = pg_catalog, __SCHEMA__, pg_temp AS $$
DECLARE
    cutoff       TIMESTAMPTZ := audit_retention_cutoff(p_retention_months);
    total_rows   BIGINT;
    expired_rows BIGINT;
    kept_rows    BIGINT;
BEGIN
    -- Timeout first, so it covers the advisory lock wait. Lock before the check, so
    -- the check and the swap are one decision.
    PERFORM set_config('lock_timeout', p_lock_timeout, true);
    PERFORM pg_advisory_xact_lock(hashtextextended('csm_audit_log_partitions', 0));

    IF NOT EXISTS (SELECT 1 FROM csm_audit_log_default WHERE created_on < cutoff) THEN
        RETURN 0;
    END IF;

    EXECUTE format('ALTER TABLE %1$I.csm_audit_log DETACH PARTITION %1$I.csm_audit_log_default', __SCHEMA_LIT__);
    EXECUTE format('ALTER TABLE %1$I.csm_audit_log_default RENAME TO csm_audit_log_default_purge', __SCHEMA_LIT__);
    EXECUTE format('CREATE TABLE %1$I.csm_audit_log_default PARTITION OF %1$I.csm_audit_log DEFAULT', __SCHEMA_LIT__);
    EXECUTE format(
        'CREATE TRIGGER csm_audit_log_no_truncate BEFORE TRUNCATE ON %I.csm_audit_log_default
           FOR EACH STATEMENT EXECUTE FUNCTION audit_log_reject_change()', __SCHEMA_LIT__);

    SELECT count(*), count(*) FILTER (WHERE created_on < cutoff)
      INTO total_rows, expired_rows
      FROM csm_audit_log_default_purge;

    -- Through the parent, so a row whose month has a partition by now lands there.
    INSERT INTO csm_audit_log SELECT * FROM csm_audit_log_default_purge WHERE created_on >= cutoff;
    GET DIAGNOSTICS kept_rows = ROW_COUNT;

    IF kept_rows + expired_rows <> total_rows THEN
        RAISE EXCEPTION 'audit_delete_expired_default_rows: % rows kept and % expired do not add up to %, rolled back',
            kept_rows, expired_rows, total_rows;
    END IF;

    EXECUTE format('DROP TABLE %I.csm_audit_log_default_purge', __SCHEMA_LIT__);
    RETURN expired_rows;
END $$;
$fn$,
        '__SCHEMA__', quote_ident(current_schema())),
        '__SCHEMA_LIT__', quote_literal(current_schema()));
END $install$;

-- Empty result = healthy. Reports rows in the catch-all partition, and the current
-- month and any of the next p_months_ahead months with no monthly partition, checked
-- month by month so a missed month is reported the day it happens. Data past retention is not reported:
-- a failed deletion is already an error in the weekly run. With no arguments it
-- checks 2 months ahead.
DROP FUNCTION IF EXISTS audit_partition_health();
DROP FUNCTION IF EXISTS audit_partition_health(INT, INT);
DO $install$
BEGIN
    EXECUTE replace(replace($fn$
CREATE OR REPLACE FUNCTION audit_partition_health(p_months_ahead INT DEFAULT 2) RETURNS TABLE (problem TEXT)
LANGUAGE plpgsql STABLE SECURITY DEFINER
SET search_path = pg_catalog, __SCHEMA__, pg_temp AS $$
DECLARE
    this_month DATE := date_trunc('month', now() AT TIME ZONE 'UTC')::date;
    missing    TEXT;
BEGIN
    IF EXISTS (SELECT 1 FROM csm_audit_log_default) THEN
        problem := 'rows landed in the catch-all partition csm_audit_log_default';
        RETURN NEXT;
    END IF;

    -- Named as audit_create_partitions names them, and attached to the audit log.
    SELECT string_agg(to_char(m.month_start, 'YYYY_MM'), ', ' ORDER BY m.month_start) INTO missing
      FROM generate_series(0, p_months_ahead) AS g(n)
     CROSS JOIN LATERAL (SELECT (this_month + make_interval(months => g.n))::date AS month_start) AS m
     WHERE NOT EXISTS (SELECT 1
                         FROM pg_inherits i
                         JOIN pg_class c ON c.oid = i.inhrelid
                        WHERE i.inhparent = format('%I.csm_audit_log', __SCHEMA_LIT__)::regclass
                          AND c.relname = 'csm_audit_log_' || to_char(m.month_start, 'YYYY_MM'));
    IF missing IS NOT NULL THEN
        problem := format('missing monthly partition(s) from this month to %s months ahead: %s', p_months_ahead, missing);
        RETURN NEXT;
    END IF;
END $$;
$fn$,
        '__SCHEMA__', quote_ident(current_schema())),
        '__SCHEMA_LIT__', quote_literal(current_schema()));
END $install$;

-- Compares the registry with the triggers that really exist. Empty result =
-- consistent. With p_repair, missing or disabled triggers are put back; the rest
-- is only reported.
DO $install$
BEGIN
    EXECUTE replace(replace($fn$
CREATE OR REPLACE FUNCTION audit_reconcile(p_repair BOOLEAN DEFAULT false)
RETURNS TABLE (schema_name TEXT, table_name TEXT, problem TEXT)
LANGUAGE plpgsql SECURITY DEFINER
SET search_path = pg_catalog, __SCHEMA__, pg_temp AS $$
DECLARE
    registered RECORD;
    locked     audit_config%ROWTYPE;
    trigger_state "char";
    trigger_matches BOOLEAN;
BEGIN
    IF p_repair THEN
        -- Re-creating a trigger locks the business table; give up rather than queue
        -- behind its writers.
        PERFORM set_config('lock_timeout', '2s', true);
    END IF;

    FOR registered IN
        SELECT ac.*, to_regclass(format('%I.%I', ac.schema_name, ac.table_name)) AS table_oid
          FROM audit_config ac
         WHERE ac.enabled
    LOOP
        schema_name := registered.schema_name;
        table_name  := registered.table_name;

        IF registered.table_oid IS NULL THEN
            problem := 'registered table was renamed or dropped';
            RETURN NEXT;
            CONTINUE;
        END IF;

        -- The trigger must be what register_audit builds: audit_log_row_change, AFTER
        -- UPDATE OR DELETE per row (tgtype 1 + 8 + 16), with the registered key,
        -- excluded and masked lists as its arguments (NUL-terminated).
        SELECT g.tgenabled,
               g.tgfoid = 'audit_log_row_change'::regproc AND g.tgtype = 25
               AND g.tgargs = convert_to(array_to_string(registered.key_columns, ','), 'UTF8') || '\x00'::bytea
                           || convert_to(array_to_string(registered.excluded_columns, ','), 'UTF8') || '\x00'::bytea
                           || convert_to(array_to_string(registered.masked_columns, ','), 'UTF8') || '\x00'::bytea
          INTO trigger_state, trigger_matches
          FROM pg_trigger g
         WHERE g.tgrelid = registered.table_oid AND g.tgname = 'csm_audit_row_change';

        IF trigger_state IS NULL THEN
            problem := 'registered table has no audit trigger';
        ELSIF trigger_state <> 'A' THEN
            problem := 'audit trigger is disabled';
        ELSIF NOT trigger_matches THEN
            problem := 'audit trigger differs from its registration';
        ELSE
            CONTINUE;
        END IF;
        RETURN NEXT;

        IF p_repair THEN
            -- One table that cannot be repaired must not stop the others. It stays
            -- wrong, so the next reconcile without repair reports it.
            BEGIN
                -- Re-read the row under a lock, so a concurrent unregister_audit cannot be
                -- undone: register_audit turns the table back on.
                SELECT * INTO locked FROM audit_config ac
                 WHERE ac.schema_name = registered.schema_name AND ac.table_name = registered.table_name
                   AND ac.enabled
                   FOR UPDATE;
                IF FOUND THEN
                    PERFORM register_audit(locked.schema_name, locked.table_name, locked.key_columns,
                                           locked.excluded_columns, locked.masked_columns);
                END IF;
            EXCEPTION WHEN OTHERS THEN
                RAISE WARNING 'audit: could not repair the trigger on %.%: %',
                    registered.schema_name, registered.table_name, SQLERRM;
            END;
        END IF;
    END LOOP;

    RETURN QUERY
    SELECT n.nspname::text, c.relname::text, 'audit trigger on an unregistered table'::text
      FROM pg_trigger g
      JOIN pg_class c ON c.oid = g.tgrelid
      JOIN pg_namespace n ON n.oid = c.relnamespace
     WHERE g.tgfoid = 'audit_log_row_change'::regproc
       AND g.tgparentid = 0  -- not a copy of a partitioned table's trigger
       AND NOT EXISTS (SELECT 1 FROM audit_config ac
                        WHERE ac.schema_name = n.nspname AND ac.table_name = c.relname AND ac.enabled);
END $$;
$fn$,
        '__SCHEMA__', quote_ident(current_schema())),
        '__SCHEMA_LIT__', quote_literal(current_schema()));
END $install$;

-- Only the owner (and SECURITY DEFINER callers) may run the maintenance functions.
REVOKE ALL ON FUNCTION register_audit(TEXT, TEXT, TEXT[], TEXT[], TEXT[]) FROM PUBLIC;
REVOKE ALL ON FUNCTION unregister_audit(TEXT, TEXT, TEXT) FROM PUBLIC;
REVOKE ALL ON FUNCTION exclude_from_audit(TEXT, TEXT, TEXT) FROM PUBLIC;
REVOKE ALL ON FUNCTION audit_partition_health(INT) FROM PUBLIC;
REVOKE ALL ON FUNCTION audit_create_partitions(INT) FROM PUBLIC;
REVOKE ALL ON FUNCTION audit_delete_expired_partitions(INT, TEXT) FROM PUBLIC;
REVOKE ALL ON FUNCTION audit_delete_expired_default_rows(INT, TEXT) FROM PUBLIC;
REVOKE ALL ON FUNCTION audit_reconcile(BOOLEAN) FROM PUBLIC;
REVOKE ALL ON FUNCTION audit_log_row_change() FROM PUBLIC;

-- New-table alert. Attaches nothing; only warns when a new table is neither
-- registered nor excluded. SECURITY DEFINER because every role that creates a
-- table runs it, and none of them may read audit_config.
-- Watches the schema this migration ran in; other schemas still to be decided.
DO $install$
BEGIN
    EXECUTE replace(replace($fn$
CREATE OR REPLACE FUNCTION audit_warn_unregistered_table() RETURNS event_trigger
LANGUAGE plpgsql SECURITY DEFINER
SET search_path = pg_catalog, __SCHEMA__, pg_temp AS $$
DECLARE
    new_table RECORD;
BEGIN
    FOR new_table IN
        SELECT d.schema_name, c.relname AS table_name
          FROM pg_event_trigger_ddl_commands() d
          JOIN pg_class c ON c.oid = d.objid
         WHERE d.object_type = 'table'
           AND d.schema_name = __SCHEMA_LIT__
           AND NOT c.relispartition
           AND c.relname NOT LIKE 'csm\_audit\_log%'
    LOOP
        IF NOT EXISTS (SELECT 1 FROM audit_config ac
                        WHERE ac.schema_name = new_table.schema_name AND ac.table_name = new_table.table_name) THEN
            RAISE WARNING 'audit: new table %.% is neither registered (register_audit) nor excluded (exclude_from_audit)',
                new_table.schema_name, new_table.table_name;
        END IF;
    END LOOP;
END $$;
$fn$,
        '__SCHEMA__', quote_ident(current_schema())),
        '__SCHEMA_LIT__', quote_literal(current_schema()));
END $install$;

-- Needs superuser; skipped with a notice otherwise.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_event_trigger WHERE evtname = 'csm_audit_new_table_alert') THEN
        CREATE EVENT TRIGGER csm_audit_new_table_alert ON ddl_command_end
            WHEN TAG IN ('CREATE TABLE', 'CREATE TABLE AS', 'SELECT INTO')
            EXECUTE FUNCTION audit_warn_unregistered_table();
    END IF;
EXCEPTION WHEN insufficient_privilege THEN
    RAISE NOTICE 'audit: cannot create event trigger csm_audit_new_table_alert (needs superuser)';
END $$;

-- First partitions, and the registry audits its own changes.
SELECT audit_create_partitions();
SELECT register_audit(current_schema(), 'audit_config', ARRAY['schema_name', 'table_name']);
