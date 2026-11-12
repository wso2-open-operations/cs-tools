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

-- next_wso2_id heals a stale counter itself.
--
-- 0180's version trusts project.wso2_id_counter blindly, so it was only
-- correct once the counter had been seeded past the highest existing
-- "<key>-N" (entity-service/scripts/cutover/seed_native_numbering.sql). While
-- a source system still allocates wso2_ids the counter does not move, so the
-- first native create after cutover would collide with an existing id and
-- fail on work_item_wso2_id_key. This version checks the candidate against
-- work_item.wso2_id (through that unique index) and keeps advancing the
-- counter until the id is free, so no seed and no gap is needed.
--
-- Contract (unchanged): same name and signature, returns "<key>-<counter>",
-- raises 'next_wso2_id: no project with id %' for a missing project.
-- Additions: the counter is never lowered; the returned id is never an
-- existing one; the advance is persisted, so a stale counter is paid for
-- once, not on every create.
--
-- Concurrency: the UPDATE below takes the project row lock, held to the end
-- of the caller's transaction, so creates in one project are serialised and a
-- second create sees the first one's committed id. The unique index stays the
-- final guard against a writer that bypasses this function.
--
-- Row-level security: work_item is FORCE ROW LEVEL SECURITY and a customer
-- only sees their projects' rows, so an existence check run as the caller
-- would miss ids it cannot see and hand out a duplicate. The body therefore
-- sets app.is_internal = 'true' (the internal-staff branch of every
-- visibility policy) around the two work_item reads only, remembers the
-- caller's value first and restores it before it returns or raises, so the
-- rest of the caller's transaction sees exactly what it saw before.
-- set_config(..., true) is transaction-local, so an exception that escapes
-- (or a caught one, which rolls back its subtransaction) cannot leave the
-- elevation behind either. A function-level "SET app.is_internal" clause is
-- NOT used: attaching a custom setting to a function needs superuser or
-- GRANT SET ON PARAMETER, which the migration role does not have, whereas
-- set_config() at run time needs neither. SECURITY DEFINER would not help:
-- FORCE applies the policies to the table owner too. The function exposes
-- nothing but the id it returns: the lookups are existence tests and an
-- integer maximum, never row data. project has no row-level security, so
-- the counter UPDATE is unaffected by the setting.
-- A caller that never set the setting reads NULL before and '' after; every
-- policy treats both as "not internal", so effective visibility is identical.

SET lock_timeout = '5s';

CREATE OR REPLACE FUNCTION next_wso2_id(p_project_id UUID)
RETURNS TEXT AS $$
DECLARE
    -- Upper bound on candidates tried in one call: a bug must not spin
    -- forever. The first collision jumps the counter to the highest existing
    -- suffix, so a stale counter normally costs one extra probe, not one per
    -- existing id.
    c_max_attempts CONSTANT INTEGER := 100000;
    v_project_key  TEXT;
    v_start        INTEGER;
    v_counter      INTEGER;
    v_prefix       TEXT;
    v_highest      INTEGER;
    v_attempts     INTEGER := 0;
    v_jumped       BOOLEAN := FALSE;
    v_prev         TEXT;
BEGIN
    UPDATE project
    SET wso2_id_counter = wso2_id_counter + 1
    WHERE id = p_project_id
    RETURNING key, wso2_id_counter INTO v_project_key, v_counter;

    IF v_project_key IS NULL THEN
        RAISE EXCEPTION 'next_wso2_id: no project with id %', p_project_id;
    END IF;

    v_start  := v_counter;
    v_prefix := v_project_key || '-';

    -- Elevate only for the work_item reads below; restored on every exit.
    v_prev := current_setting('app.is_internal', true);
    PERFORM set_config('app.is_internal', 'true', true);

    WHILE EXISTS (SELECT 1 FROM work_item WHERE wso2_id = v_prefix || v_counter::TEXT) LOOP
        v_attempts := v_attempts + 1;
        IF v_attempts > c_max_attempts THEN
            PERFORM set_config('app.is_internal', COALESCE(v_prev, ''), true);
            RAISE EXCEPTION 'next_wso2_id: no free wso2_id for project % after % attempts (counter %, key %)',
                p_project_id, c_max_attempts, v_counter, v_project_key;
        END IF;

        IF NOT v_jumped THEN
            v_jumped := TRUE;
            -- Highest N among ids exactly "<key>-N": the key as a literal
            -- prefix (no pattern matching, so keys with dashes or other
            -- characters are safe), the rest 1-9 plain digits, compared as
            -- integers (10 beats 9). "<key>-PORTAL-3" and ids of a longer
            -- key never match. Nine digits always fit INTEGER. Never moves
            -- the counter down.
            SELECT max(substr(wso2_id, length(v_prefix) + 1)::INTEGER) INTO v_highest
            FROM work_item
            WHERE left(wso2_id, length(v_prefix)) = v_prefix
              AND substr(wso2_id, length(v_prefix) + 1) ~ '^[0-9]{1,9}$';
            v_counter := GREATEST(v_counter, COALESCE(v_highest, v_counter));
        END IF;

        v_counter := v_counter + 1;
    END LOOP;

    PERFORM set_config('app.is_internal', COALESCE(v_prev, ''), true);

    IF v_counter <> v_start THEN
        UPDATE project SET wso2_id_counter = v_counter WHERE id = p_project_id;
    END IF;

    RETURN v_prefix || v_counter::TEXT;
END;
$$ LANGUAGE plpgsql;

RESET lock_timeout;
