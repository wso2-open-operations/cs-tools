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

-- rollup_monthly_usage again, changed in one place: where a node-type-day has more
-- than one daily row, the live (API_CALL) one is picked with an IS TRUE check.
-- daily_usage_summary.data_source is nullable (migration 0054), and under DESC a NULL
-- comparison sorts ahead of TRUE, so a row with no source would have beaten the live
-- row. It now ranks with the uploaded rows, as the daily reads in
-- internal/repository (dailyUsageSourcePreference) rank it. 0245 has already run on
-- servers, so the change is a new migration rather than an edit there.
-- Safe to run again (CREATE OR REPLACE).

-- Recomputes one calendar month of monthly_usage_summary from daily_usage_summary:
-- the sum of the days for a counter (capped at what the INTEGER column holds), the
-- latest day's value for any other type. A node-type-day with both a live (API_CALL)
-- row and an uploaded (FILE_UPLOAD) row counts once, the live one; a node that only
-- has uploaded days (an air-gapped product) is included from them. A count type
-- longer than the monthly column holds (50) is skipped. Returns the number of rows
-- written; months other than the one given are never touched.
CREATE OR REPLACE FUNCTION rollup_monthly_usage(p_year integer, p_month integer)
RETURNS integer
LANGUAGE plpgsql
AS $$
DECLARE
    v_first   date := make_date(p_year, p_month, 1);
    v_next    date := (make_date(p_year, p_month, 1) + INTERVAL '1 month')::date;
    v_written integer;
BEGIN
    -- monthly_usage_summary has no unique key, so two runs for the same month would
    -- both pass the NOT EXISTS check and insert twice. Serialise them per month.
    PERFORM pg_advisory_xact_lock(hashtextextended('usage-rollup-month:' || p_year || '-' || p_month, 0));

    WITH per_day AS (
        SELECT DISTINCT ON (d.deployment_node_id, d.count_type, d.summary_date)
               d.deployment_node_id AS node_uuid, d.count_type, d.summary_date, d.value
        FROM daily_usage_summary d
        WHERE d.deployment_node_id IS NOT NULL
          AND d.value IS NOT NULL
          AND length(d.count_type) <= 50
          AND d.summary_date >= v_first AND d.summary_date < v_next
        ORDER BY d.deployment_node_id, d.count_type, d.summary_date,
                 ((d.data_source = 'API_CALL') IS TRUE) DESC, d.updated_on DESC, d.id
    ), per_node AS (
        SELECT node_uuid, count_type, LEAST(SUM(value), 2147483647)::integer AS value
        FROM per_day
        WHERE usage_count_type_is_counter(count_type)
        GROUP BY node_uuid, count_type
        UNION ALL
        (SELECT DISTINCT ON (node_uuid, count_type) node_uuid, count_type, value
         FROM per_day
         WHERE NOT usage_count_type_is_counter(count_type)
         ORDER BY node_uuid, count_type, summary_date DESC)
    ), computed AS (
        -- monthly_usage_summary is keyed by the node's own id string, which is not
        -- unique in deployment_node: fold nodes sharing one into a single row.
        SELECT dn.node_id,
               p.count_type,
               CASE WHEN usage_count_type_is_counter(p.count_type)
                    THEN LEAST(SUM(p.value::bigint), 2147483647)::integer
                    ELSE MAX(p.value) END AS value
        FROM per_node p
        JOIN deployment_node dn ON dn.id = p.node_uuid
        GROUP BY dn.node_id, p.count_type
    ), updated AS (
        UPDATE monthly_usage_summary m
        SET count = c.value, updated_by = 'usage-rollup', updated_on = NOW()
        FROM computed c
        WHERE m.node_id = c.node_id
          AND m.count_type = c.count_type
          AND m.year = p_year AND m.month = p_month
          AND m.count IS DISTINCT FROM c.value
        RETURNING m.id
    ), inserted AS (
        INSERT INTO monthly_usage_summary (
            id, created_on, updated_on, created_by, updated_by,
            node_id, count_type, count, year, month
        )
        SELECT gen_random_uuid(), NOW(), NOW(), 'usage-rollup', 'usage-rollup',
               c.node_id, c.count_type, c.value, p_year, p_month
        FROM computed c
        WHERE NOT EXISTS (
            SELECT 1 FROM monthly_usage_summary m
            WHERE m.node_id = c.node_id
              AND m.count_type = c.count_type
              AND m.year = p_year AND m.month = p_month)
        RETURNING id
    )
    SELECT (SELECT count(*) FROM updated) + (SELECT count(*) FROM inserted) INTO v_written;

    RETURN v_written;
END;
$$;
