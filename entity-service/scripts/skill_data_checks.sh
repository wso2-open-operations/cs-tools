#!/usr/bin/env bash
# Skill-management revamp: read-only data checks for migrations 0282-0286.
#
# Run against staging and production BEFORE applying migrations 0282-0286.
# Every check is a SELECT inside a READ ONLY transaction, with the session also
# forced read-only and a statement timeout, so it cannot change data.
#
# Usage:
#   ./skill_data_checks.sh "postgresql://user@host:5432/db?sslmode=require"
#   ./skill_data_checks.sh                      # uses PGHOST/PGUSER/PGDATABASE/...
#   PSQL="docker exec -i csm-platform-postgres-1 psql -U postgres -d csm_platform" ./skill_data_checks.sh
#
# Options (env):
#   PSQL          psql command to run (default: psql)
#   DETAIL_LIMIT  max offending rows printed per failed check (default: 20)
#   STMT_TIMEOUT  per-statement timeout (default: 60s)
#   SCHEMA        search_path to use, e.g. "csm" or "csm,public" (default: as configured)
#
# Exit code: 0 = no BLOCKER found, 1 = at least one BLOCKER, 2 = could not run.
#
# Result levels:
#   BLOCKER  a new unique index or the history backfill will fail; fix the data first
#   REVIEW   the migration still runs, but a decision or a cleanup is needed
#   INFO     numbers for the release notes
#
# Output contains user ids (uuids) only, never names or emails.

set -euo pipefail

PSQL_CMD=${PSQL:-psql}
DETAIL_LIMIT=${DETAIL_LIMIT:-20}
STMT_TIMEOUT=${STMT_TIMEOUT:-60s}
CONN=${1:-}

if ! [[ "$DETAIL_LIMIT" =~ ^[0-9]+$ ]]; then
  echo "DETAIL_LIMIT must be a whole number" >&2; exit 2
fi
if [[ -n "${SCHEMA:-}" && ! "$SCHEMA" =~ ^[A-Za-z_][A-Za-z0-9_]*(,[A-Za-z_][A-Za-z0-9_]*)*$ ]]; then
  echo "SCHEMA must be a schema name or a comma-separated list of them" >&2; exit 2
fi

read -r -a psql_argv <<<"$PSQL_CMD"
[[ -n "$CONN" ]] && psql_argv+=("$CONN")

# Read-only is enforced three times: the session default (PGOPTIONS, honoured by a
# local psql), SET SESSION below (also works through docker exec, which does not
# pass PGOPTIONS), and BEGIN READ ONLY.
export PGOPTIONS="${PGOPTIONS:-} -c default_transaction_read_only=on -c statement_timeout=${STMT_TIMEOUT}"

out=$(mktemp)
trap 'rm -f "$out"' EXIT

set +e
"${psql_argv[@]}" -X -q -A -F ' | ' -P footer=off -v ON_ERROR_STOP=1 \
  -v lim="$DETAIL_LIMIT" -v tmo="$STMT_TIMEOUT" -v schema="${SCHEMA:-}" <<'SQL' 2>&1 | tee "$out"
SET SESSION CHARACTERISTICS AS TRANSACTION READ ONLY;
SET statement_timeout = :'tmo';
SELECT (:'schema' <> '') AS has_schema \gset
\if :has_schema
SELECT set_config('search_path', :'schema', false) \g /dev/null
\endif
BEGIN READ ONLY;

\echo '== Skill data checks (read-only) =='
SELECT current_database() AS db, current_setting('server_version') AS pg,
       current_setting('transaction_read_only') AS read_only, now()::timestamp(0) AS at;

-- ---------------------------------------------------------------- BLOCKERS

\echo ''
\echo '--- B1 Duplicate skill names, ignoring case and outer spaces (M2 unique index)'
SELECT count(*) AS n, count(*) > 0 AS bad FROM (
  SELECT 1 FROM skill GROUP BY lower(btrim(name)) HAVING count(*) > 1) d \gset
\if :bad
\echo 'BLOCKER  B1 duplicate name groups:' :n
SELECT lower(btrim(name)) AS name, count(*) AS copies,
       count(*) FILTER (WHERE active) AS active_copies,
       string_agg(id::text, ', ' ORDER BY created_on) AS skill_ids
FROM skill GROUP BY 1 HAVING count(*) > 1 ORDER BY 2 DESC, 1 LIMIT :lim;
\else
\echo 'PASS     B1'
\endif

\echo ''
\echo '--- B2 Sibling categories with the same name (M1 unique index; root level counted as one parent)'
SELECT count(*) AS n, count(*) > 0 AS bad FROM (
  SELECT 1 FROM skill_category GROUP BY parent_id, lower(btrim(name)) HAVING count(*) > 1) d \gset
\if :bad
\echo 'BLOCKER  B2 duplicate sibling groups:' :n
SELECT coalesce(parent_id::text, '(root)') AS parent, lower(btrim(name)) AS name, count(*) AS copies,
       string_agg(id::text, ', ' ORDER BY created_on) AS category_ids
FROM skill_category GROUP BY parent_id, lower(btrim(name)) HAVING count(*) > 1
ORDER BY 3 DESC LIMIT :lim;
\else
\echo 'PASS     B2'
\endif

\echo ''
\echo '--- B3 A person holding the same skill twice as their own active skill (M3 unique index)'
SELECT count(*) AS n, count(*) > 0 AS bad FROM (
  SELECT 1 FROM user_skill WHERE active AND NOT inherited
  GROUP BY user_id, skill_id HAVING count(*) > 1) d \gset
\if :bad
\echo 'BLOCKER  B3 duplicate (user, skill) pairs:' :n
SELECT user_id, skill_id, count(*) AS rows,
       count(DISTINCT skill_level_id) AS distinct_levels,
       string_agg(id::text, ', ' ORDER BY updated_on DESC) AS user_skill_ids_newest_first
FROM user_skill WHERE active AND NOT inherited
GROUP BY user_id, skill_id HAVING count(*) > 1 ORDER BY 3 DESC LIMIT :lim;
\else
\echo 'PASS     B3'
\endif

\echo ''
\echo '--- B4 History rows with neither a from nor a to level (the M4 action backfill cannot classify them)'
SELECT count(*) AS n, count(*) > 0 AS bad
FROM user_skill_history WHERE level_from_id IS NULL AND level_to_id IS NULL \gset
\if :bad
\echo 'BLOCKER  B4 unclassifiable history rows:' :n
SELECT id, user_id, skill_id, created_on, created_by
FROM user_skill_history WHERE level_from_id IS NULL AND level_to_id IS NULL
ORDER BY created_on DESC LIMIT :lim;
\else
\echo 'PASS     B4'
\endif

\echo ''
\echo '--- B5 History rows whose from and to are the same level (backfill would call them LEVEL_CHANGED)'
SELECT count(*) AS n, count(*) > 0 AS bad
FROM user_skill_history WHERE level_from_id = level_to_id \gset
\if :bad
\echo 'BLOCKER  B5 no-op history rows (decide: backfill as LEVEL_CHANGED or exclude):' :n
SELECT id, user_id, skill_id, created_on FROM user_skill_history
WHERE level_from_id = level_to_id ORDER BY created_on DESC LIMIT :lim;
\else
\echo 'PASS     B5'
\endif

-- ---------------------------------------------------------------- REVIEW

\echo ''
\echo '--- R1 Proficiency scales in use (M2 picks ONE default; the directory compares level values across scales)'
SELECT count(*) AS n, count(*) > 1 AS bad FROM (
  SELECT DISTINCT level_type_id FROM skill WHERE level_type_id IS NOT NULL) d \gset
SELECT lt.id AS scale_id, lt.name AS scale,
       (SELECT count(*) FROM skill s WHERE s.level_type_id = lt.id) AS skills,
       (SELECT count(*) FROM skill s WHERE s.level_type_id = lt.id AND s.active) AS active_skills,
       (SELECT string_agg(sl.name || '=' || coalesce(sl.value::text, 'null'), ', ' ORDER BY sl.value NULLS LAST)
          FROM skill_level sl WHERE sl.level_type_id = lt.id) AS levels
FROM skill_level_type lt ORDER BY skills DESC;
\if :bad
\echo 'REVIEW   R1 skills use' :n 'different scales: keep them, or map onto the default scale in a reviewed data migration'
\else
\echo 'PASS     R1 at most one scale in use'
\endif

\echo ''
\echo '--- R2 Levels with no numeric value (the "at least Intermediate" filter cannot rank them)'
SELECT count(*) AS n, count(*) > 0 AS bad FROM skill_level WHERE value IS NULL \gset
\if :bad
\echo 'REVIEW   R2 levels without a value:' :n
SELECT sl.id, sl.name, lt.name AS scale FROM skill_level sl
LEFT JOIN skill_level_type lt ON lt.id = sl.level_type_id WHERE sl.value IS NULL LIMIT :lim;
\else
\echo 'PASS     R2'
\endif

\echo ''
\echo '--- R3 Skills with no scale (no level can be set on them today)'
SELECT count(*) AS n, count(*) > 0 AS bad FROM skill WHERE level_type_id IS NULL \gset
\if :bad
\echo 'REVIEW   R3 skills without a scale:' :n
SELECT id, name, active FROM skill WHERE level_type_id IS NULL ORDER BY name LIMIT :lim;
\else
\echo 'PASS     R3'
\endif

\echo ''
\echo '--- R4 Assignments whose level is not on the skill''s own scale (editing them fails checkLevelInScale)'
SELECT count(*) AS n, count(*) > 0 AS bad
FROM user_skill us JOIN skill s ON s.id = us.skill_id JOIN skill_level sl ON sl.id = us.skill_level_id
WHERE sl.level_type_id IS DISTINCT FROM s.level_type_id \gset
\if :bad
\echo 'REVIEW   R4 off-scale assignments:' :n
SELECT us.id AS user_skill_id, us.user_id, s.name AS skill, sl.name AS level, us.active, us.inherited
FROM user_skill us JOIN skill s ON s.id = us.skill_id JOIN skill_level sl ON sl.id = us.skill_level_id
WHERE sl.level_type_id IS DISTINCT FROM s.level_type_id LIMIT :lim;
\else
\echo 'PASS     R4'
\endif

\echo ''
\echo '--- R5 More than one own row per (user, skill), active or not (soft delete revives exactly one; C.7)'
SELECT count(*) AS n, count(*) > 0 AS bad FROM (
  SELECT 1 FROM user_skill WHERE NOT inherited GROUP BY user_id, skill_id HAVING count(*) > 1) d \gset
\if :bad
\echo 'REVIEW   R5 pairs with several own rows (revive picks the newest; older ones stay inactive):' :n
SELECT user_id, skill_id, count(*) AS rows, count(*) FILTER (WHERE active) AS active_rows
FROM user_skill WHERE NOT inherited GROUP BY 1, 2 HAVING count(*) > 1 ORDER BY 3 DESC LIMIT :lim;
\else
\echo 'PASS     R5'
\endif

\echo ''
\echo '--- R6 Skills with no category, or filed under a category that is not a leaf'
SELECT count(*) AS n, count(*) > 0 AS bad FROM skill s
WHERE NOT EXISTS (SELECT 1 FROM skill_category_link l WHERE l.skill_id = s.id) \gset
\if :bad
\echo 'REVIEW   R6a skills with no category (they only show under search, never in the tree):' :n
SELECT id, name, active FROM skill s
WHERE NOT EXISTS (SELECT 1 FROM skill_category_link l WHERE l.skill_id = s.id) ORDER BY name LIMIT :lim;
\else
\echo 'PASS     R6a'
\endif
SELECT count(*) AS n, count(*) > 0 AS bad FROM skill_category_link l
JOIN skill_category c ON c.id = l.category_id WHERE NOT c.is_lowest_category \gset
\if :bad
\echo 'REVIEW   R6b skill links to non-leaf categories:' :n
SELECT s.name AS skill, c.category_path FROM skill_category_link l
JOIN skill_category c ON c.id = l.category_id JOIN skill s ON s.id = l.skill_id
WHERE NOT c.is_lowest_category ORDER BY 2, 1 LIMIT :lim;
\else
\echo 'PASS     R6b'
\endif

\echo ''
\echo '--- R7 is_lowest_category disagrees with the tree (the category admin trusts it)'
SELECT count(*) AS n, count(*) > 0 AS bad FROM skill_category c
WHERE c.is_lowest_category = EXISTS (SELECT 1 FROM skill_category ch WHERE ch.parent_id = c.id) \gset
\if :bad
\echo 'REVIEW   R7 categories flagged wrongly:' :n
SELECT c.id, c.category_path, c.is_lowest_category,
       (SELECT count(*) FROM skill_category ch WHERE ch.parent_id = c.id) AS children
FROM skill_category c
WHERE c.is_lowest_category = EXISTS (SELECT 1 FROM skill_category ch WHERE ch.parent_id = c.id)
ORDER BY 2 LIMIT :lim;
\else
\echo 'PASS     R7'
\endif

\echo ''
\echo '--- R8 Inherited rows with no source group'
SELECT count(*) AS n, count(*) > 0 AS bad FROM user_skill
WHERE inherited AND inherited_from_group_id IS NULL \gset
\if :bad
\echo 'REVIEW   R8 inherited rows without a group (read-only to the user, but no one can explain them):' :n
\else
\echo 'PASS     R8'
\endif

\echo ''
\echo '--- R9 Active assignments with no level'
SELECT count(*) AS n, count(*) > 0 AS bad FROM user_skill WHERE active AND skill_level_id IS NULL \gset
\if :bad
\echo 'REVIEW   R9 assignments without a level (never match a minimum-level filter):' :n
\else
\echo 'PASS     R9'
\endif

-- ---------------------------------------------------------------- INFO

\echo ''
\echo '--- I1 Catalogue size'
SELECT (SELECT count(*) FROM skill) AS skills,
       (SELECT count(*) FROM skill WHERE active) AS active_skills,
       (SELECT count(*) FROM skill_category) AS categories,
       (SELECT count(*) FROM skill_category WHERE is_lowest_category) AS leaf_categories,
       (SELECT max(length(category_path)) FROM skill_category) AS longest_category_path;

\echo ''
\echo '--- I2 Assignments by state'
SELECT active, inherited, count(*) AS rows, count(DISTINCT user_id) AS people
FROM user_skill GROUP BY 1, 2 ORDER BY 1 DESC, 2;

\echo ''
\echo '--- I3 Active assignments of inactive skills or inactive people (kept, hidden by the new reads)'
SELECT count(*) FILTER (WHERE NOT s.active) AS on_inactive_skills,
       count(*) FILTER (WHERE NOT u.is_active) AS of_inactive_people
FROM user_skill us JOIN skill s ON s.id = us.skill_id JOIN "user" u ON u.id = us.user_id
WHERE us.active;

\echo ''
\echo '--- I4 History as the M4 backfill would classify it'
WITH h AS (
  SELECT h.*, row_number() OVER (PARTITION BY user_id, skill_id ORDER BY created_on, id) AS seq
  FROM user_skill_history h)
SELECT CASE
         WHEN level_from_id IS NULL AND level_to_id IS NULL THEN '(unclassifiable, B4)'
         WHEN level_from_id IS NULL AND seq = 1           THEN 'ADDED'
         WHEN level_from_id IS NULL                        THEN 'READDED'
         WHEN level_to_id IS NULL                          THEN 'REMOVED'
         ELSE 'LEVEL_CHANGED' END AS action,
       count(*) AS rows, min(created_on)::date AS first, max(created_on)::date AS last
FROM h GROUP BY 1 ORDER BY 2 DESC;

ROLLBACK;
\echo ''
\echo '== Done. Nothing was written. =='
SQL
rc=${PIPESTATUS[0]}
set -e

if [[ $rc -ne 0 ]]; then
  echo "psql failed (exit $rc); the checks did not complete." >&2
  exit 2
fi

blockers=$(grep -c '^BLOCKER' "$out" || true)
reviews=$(grep -c '^REVIEW' "$out" || true)
echo
echo "Summary: ${blockers} blocker(s), ${reviews} item(s) to review."
[[ $blockers -eq 0 ]]
