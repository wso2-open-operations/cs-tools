#!/usr/bin/env bash
# Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.

# Prints, one path per line, the migration files to apply and the order to
# apply them in. `make migrate` and scripts/generate_schema_bootstrap.sh both
# iterate this list, so the two can never disagree. Run it on its own for a
# dry run of the order: ./scripts/migration_order.sh [migrations_dir]
# (or `make migrate-order`).
#
# A plain `migrations/*.sql` glob is wrong here for three reasons:
#
#   1. The legacy 6-digit 000NNN_*.up.sql files sort BEFORE 0001_ (leading
#      zeros), yet they depend on tables and types the 4-digit files create
#      (000085 uses announcement_type_enum from 0088; 000087 triggers on
#      outage from 0083 and outage_affected_ci from 0090). They cannot go
#      last either: 0149 replaces 000085's announcement_visibility policy
#      (DROP POLICY IF EXISTS, then CREATE), so 000085's bare CREATE POLICY
#      fails with "already exists" if it runs after 0149, and its CREATE OR
#      REPLACE FUNCTION would undo 0149/0177's function changes. So: the
#      4-digit files numbered below STRAGGLERS_BEFORE in version order, then
#      the 6-digit *.up.sql files in version order, then the remaining
#      4-digit files. (scripts/csm-compose/migrate-and-seed.sh puts the
#      6-digit files last, which stops at 000085 on an empty database.)
#   2. *.down.sql files are rollbacks. The convention is forward-only; they
#      are never listed, so they are never applied on the way up.
#   3. Two Team Schedule chains sit side by side. The consolidated chain
#      (0153_team_schedule_tables, 0154_team_schedule_catalogue,
#      0155_team_schedule_audit) drops the un-prefixed schedule_* tables the
#      older chain built; the older chain's remaining files then ALTER those
#      dropped tables and fail. The older files are listed in is_superseded
#      below (the same list the compose runner skips) and are skipped, with a
#      note on stderr. A DB that already recorded one of them is unaffected:
#      a recorded file is skipped either way.
#
# Files are never renamed to fix ordering: the filename is the key in
# csm_migration_applied_migration, so a rename would re-apply an
# already-applied migration.
set -euo pipefail

dir="${1:-migrations}"

# The first 4-digit migration that must run after the 6-digit files (see 1.).
STRAGGLERS_BEFORE=0149

is_superseded() {
	case "$1" in
	0152_team_schedule_tables | 0153_team_schedule_catalogue | 0154_team_schedule_vocabulary | \
		0155_schedule_customer_allocation_split | 0156_schedule_shift_is_rotation | \
		0157_schedule_shift_required_headcount | 0158_schedule_assignment_activity | \
		0159_schedule_absence_activity | 0160_schedule_absence_kind_consolidation | \
		0161_schedule_americas_weekend_names | 0162_schedule_remove_invented_windows | \
		0163_schedule_rotation_short_codes | 0164_schedule_integrity_constraints | \
		0165_schedule_team_key_catalogue | 0166_schedule_absence_bucket_enum | \
		0167_team_schedule_table_prefix | 0168_team_schedule_audit)
		return 0
		;;
	esac
	return 1
}

# LC_ALL=C keeps the order byte-wise and identical on every machine,
# whatever the caller's locale collation would do with '_' and '-'.
four_digit() {
	find "$dir" -maxdepth 1 -type f -name '[0-9][0-9][0-9][0-9]_*.sql' | LC_ALL=C sort -V
}

# four_digit_where <op>: the 4-digit files whose number satisfies
# "<number> <op> STRAGGLERS_BEFORE" (op is lt or ge).
four_digit_where() {
	local f n
	four_digit | while IFS= read -r f; do
		n="${f##*/}"
		n="${n%%_*}"
		if [[ "$1" == lt ]] && ((10#$n < 10#$STRAGGLERS_BEFORE)); then
			printf '%s\n' "$f"
		elif [[ "$1" == ge ]] && ((10#$n >= 10#$STRAGGLERS_BEFORE)); then
			printf '%s\n' "$f"
		fi
	done
}

{
	four_digit_where lt
	find "$dir" -maxdepth 1 -type f -name '[0-9][0-9][0-9][0-9][0-9][0-9]_*.up.sql' | LC_ALL=C sort -V
	four_digit_where ge
} | while IFS= read -r f; do
	name="${f##*/}"
	name="${name%.sql}"
	if is_superseded "$name"; then
		echo "skipping superseded ${name}.sql" >&2
		continue
	fi
	printf '%s\n' "$f"
done
