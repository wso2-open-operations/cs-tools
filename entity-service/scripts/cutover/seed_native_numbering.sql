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

-- CUTOVER RUNBOOK STEP. Not a migration: never place this under migrations/.
--
-- Moves every native number sequence (migration 0180's number_series) and
-- every project's wso2_id_counter past the highest number already allocated
-- by the source system, so native allocation never reissues an existing
-- number.
--
-- WHEN: after the FINAL sync from the source system, with that sync stopped.
-- The source system keeps allocating numbers until then; anything it
-- allocates after this script runs is not accounted for. Re-run this script
-- after any late sync: it is idempotent and only ever moves values forward
-- (but see step 6 below if an outlier series was corrected by hand).
--
-- USAGE (dry run is the default and only reads):
--   psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f seed_native_numbering.sql
--   psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -v apply=1 -f seed_native_numbering.sql
--
-- Rules:
--   * A series' maximum is taken only from values matching exactly
--     ^PREFIX[0-9]+$ in that series' own column(s). Portal/GitHub marker
--     numbers (CS-PORTAL-, CHG-GH-, SR-GH-) contain hyphens and so never
--     match; neither does a longer prefix (CSTASK never counts towards CS).
--   * setval(seq, max, true): the next value issued is max + 1. A sequence
--     already at or past max is left alone: nothing ever moves backwards.
--   * wso2_id: per project, the largest integer N among ids exactly
--     "<project.key>-N" (compared as integers, so 10 beats 9), across ALL
--     work items (an id keeps its key's namespace even if the item moved
--     project). "<key>-PORTAL-<n>" ids never match. Counters only move up.
--   * Row-level security on work_item and friends would silently hide rows
--     and seed too LOW. app.is_internal = 'true' (the internal-staff branch
--     of every visibility policy) is set for this transaction; check the
--     per-source row counts printed below against the expected totals.
--
-- CUTOVER STEPS, in order, for the day the source system stops allocating
-- numbers:
--   1. Preconditions. PostgreSQL 18 everywhere migrations run; migrations 0179
--      and 0180 applied. Applying them does NOT change which numbers the
--      application issues: the create-case insert still calls the portal
--      functions (next_portal_work_item_number / next_portal_wso2_id, migration
--      0140), which produce CS-PORTAL-... and <key>-PORTAL-<n>. Before cutover
--      the application must call next_work_item_number(type) and
--      next_wso2_id(project) instead, behind a native-numbering setting, and
--      insert ids without gen_random_uuid() (omit id, or use uuidv7()),
--      otherwise the uuidv7() column defaults are bypassed. Not done as of
--      this note.
--   2. Freeze and sync. Stop creates in the source system, run the final
--      sync, then stop the sync.
--   3. Dry run (no flag). Check the per-source row counts against the expected
--      totals, then read every series line: source_max, seq_before, next_number.
--   4. Look for outliers BEFORE applying. A series whose source_max is far
--      above its neighbours (for example a placeholder such as 99999999 in
--      knowledge_article.number) would push every future number up. If there
--      is one, find the real maximum.
--   5. Apply (-v apply=1).
--   6. Correct any outlier series by hand: select setval('<sequence>',
--      <real max>, true). After that, do NOT run this script in apply mode
--      again: it only moves values forward and would push the series back up
--      to the outlier. A dry run is always safe.
--   7. Read back without consuming a number: for each row of number_series,
--      the sequence's last_value and is_called, plus project.wso2_id_counter.
--      The next issued number is last_value + 1, formatted as PREFIX plus
--      zero-padded digits.
--   8. Series this script cannot seed, as of this note: ESC (no table stores
--      an escalation number), and ALT, which is seeded from
--      alert_incident_mapping.alert_number only in practice, because
--      incident_alert numbers use the ICP prefix and no series covers it yet.
--      Decide both before relying on them.
--   9. Switch the data source / native-numbering setting, then create one
--      record of each type and check its number, wso2_id and id.
--  10. Rollback: records created natively after this point have no
--      counterpart in the source system. Going back to the source system loses
--      them unless they are backfilled first.

\set ON_ERROR_STOP on
\if :{?apply}
\else
\set apply false
\endif

\if :apply
\echo '=== APPLY: sequences and wso2_id counters will be moved forward ==='
BEGIN;
SET LOCAL lock_timeout = '5s';
-- Hold off next_wso2_id / next_portal_wso2_id while counters are computed and set.
LOCK TABLE project IN SHARE ROW EXCLUSIVE MODE;
\else
\echo '=== DRY RUN: read-only transaction, nothing is changed (pass -v apply=1 to apply) ==='
BEGIN READ ONLY;
\endif
SET LOCAL app.is_internal = 'true';

\echo ''
\echo '--- rows visible per source column (cross-check against expected totals) ---'
SELECT 'work_item.number'                    AS source, count(*) AS rows FROM work_item
UNION ALL SELECT 'customer_call.number',              count(*) FROM customer_call
UNION ALL SELECT 'project.number',                    count(*) FROM project
UNION ALL SELECT 'account.number',                    count(*) FROM account
UNION ALL SELECT 'communication_plan.number',         count(*) FROM communication_plan
UNION ALL SELECT 'knowledge_article.number',          count(*) FROM knowledge_article
UNION ALL SELECT 'outage.number',                     count(*) FROM outage
UNION ALL SELECT 'deployed_product.number',           count(*) FROM deployed_product
UNION ALL SELECT 'deployment.number',                 count(*) FROM deployment
UNION ALL SELECT 'incident_alert.number',             count(*) FROM incident_alert
UNION ALL SELECT 'alert_incident_mapping.alert_number', count(*) FROM alert_incident_mapping;

\echo ''
\echo '--- number series ---'
-- One statement for both modes: in a dry run the setval() branch is never
-- reached (and would be refused by the READ ONLY transaction if it were).
WITH series_source(series, source) AS (
    VALUES ('CS',    'work_item.number'), ('INC',   'work_item.number'),
           ('PRB',   'work_item.number'), ('CHG',   'work_item.number'),
           ('TASK',  'work_item.number'), ('CTASK', 'work_item.number'),
           ('PTASK', 'work_item.number'), ('ICT',   'work_item.number'),
           ('CHAT',  'work_item.number'),
           ('CSTASK','customer_call.number'),
           ('CSPRJ', 'project.number'),
           ('ACCT',  'account.number'),
           ('CMP',   'communication_plan.number'),
           ('KB',    'knowledge_article.number'),
           ('OUT',   'outage.number'),
           ('IBITM', 'deployed_product.number'),
           ('DEP',   'deployment.number'),
           ('ALT',   'incident_alert.number'),
           ('ALT',   'alert_incident_mapping.alert_number')
           -- ESC: no table in this schema stores an escalation number.
),
numbers(source, number) AS (
              SELECT 'work_item.number', number::text FROM work_item
    UNION ALL SELECT 'customer_call.number', number::text FROM customer_call
    UNION ALL SELECT 'project.number', number::text FROM project
    UNION ALL SELECT 'account.number', number::text FROM account
    UNION ALL SELECT 'communication_plan.number', number::text FROM communication_plan
    UNION ALL SELECT 'knowledge_article.number', number::text FROM knowledge_article
    UNION ALL SELECT 'outage.number', number::text FROM outage
    UNION ALL SELECT 'deployed_product.number', number::text FROM deployed_product
    UNION ALL SELECT 'deployment.number', number::text FROM deployment
    UNION ALL SELECT 'incident_alert.number', number::text FROM incident_alert
    UNION ALL SELECT 'alert_incident_mapping.alert_number', alert_number FROM alert_incident_mapping
),
parsed AS (
    -- One regex per row: prefix and digits of a strictly PREFIX+digits value.
    SELECT source, m[1] AS prefix, m[2]::numeric AS n
    FROM numbers, regexp_match(number, '^([A-Z]+)([0-9]+)$') AS m
),
maxima AS (
    SELECT ss.series,
           string_agg(DISTINCT ss.source, ', ') AS sources,
           max(p.n) AS source_max
    FROM series_source ss
    LEFT JOIN parsed p ON p.source = ss.source AND p.prefix = ss.series
    GROUP BY ss.series
),
plan AS (
    SELECT ns.series, ns.digits, ns.sequence_name,
           COALESCE(m.sources, '(no source column)') AS sources,
           m.source_max,
           -- Highest value already issued (or reserved) by the sequence.
           COALESCE(sq.last_value, sq.start_value - 1) AS seq_before
    FROM number_series ns
    JOIN pg_sequences sq ON sq.schemaname = current_schema() AND sq.sequencename = ns.sequence_name
    LEFT JOIN maxima m ON m.series = ns.series
),
applied AS (
    SELECT plan.*,
           CASE WHEN :'apply'::boolean AND source_max > seq_before
                THEN setval(sequence_name::regclass, source_max::bigint, true)
                ELSE GREATEST(seq_before, COALESCE(source_max, seq_before))
           END AS seq_after
    FROM plan
)
SELECT series, sequence_name, sources, source_max, seq_before, seq_after,
       CASE WHEN source_max IS NULL THEN 'unchanged (no matching rows)'
            WHEN source_max > seq_before THEN CASE WHEN :'apply'::boolean THEN 'ADVANCED' ELSE 'would advance' END
            ELSE 'unchanged (sequence already ahead)'
       END AS action,
       series || LPAD((seq_after + 1)::text, GREATEST(digits, LENGTH((seq_after + 1)::text)), '0') AS next_number
FROM applied
ORDER BY series;

\echo ''
\echo '--- wso2_id counters (projects whose counter changes) ---'
-- Split on the LAST hyphen: everything before it is the key, the digits after
-- it the counter. Keys may contain hyphens themselves (AC-ME-7 -> AC-ME, 7).
SELECT p.key, p.wso2_id_counter AS counter_before, m.source_max,
       GREATEST(p.wso2_id_counter, m.source_max) AS counter_after,
       CASE WHEN :'apply'::boolean THEN 'ADVANCED' ELSE 'would advance' END AS action
FROM project p
JOIN (
    SELECT substring(wso2_id from '^(.*)-[0-9]+$') AS key,
           max(substring(wso2_id from '-([0-9]+)$')::numeric) AS source_max
    FROM work_item
    WHERE wso2_id ~ '^.+-[0-9]+$' AND wso2_id NOT LIKE '%-PORTAL-%'
    GROUP BY 1
) m ON m.key = p.key
WHERE m.source_max > p.wso2_id_counter
ORDER BY p.key;

\if :apply
-- Same aggregate as the report above; keep the two identical.
UPDATE project p
SET wso2_id_counter = m.source_max
FROM (
    SELECT substring(wso2_id from '^(.*)-[0-9]+$') AS key,
           max(substring(wso2_id from '-([0-9]+)$')::numeric) AS source_max
    FROM work_item
    WHERE wso2_id ~ '^.+-[0-9]+$' AND wso2_id NOT LIKE '%-PORTAL-%'
    GROUP BY 1
) m
WHERE m.key = p.key AND m.source_max > p.wso2_id_counter;
COMMIT;
\echo '=== APPLIED ==='
\else
ROLLBACK;
\echo '=== DRY RUN complete: nothing changed ==='
\endif
