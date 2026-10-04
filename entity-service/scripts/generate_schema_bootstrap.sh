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

# Concatenates the migrations listed by scripts/migration_order.sh, in the
# same order `make migrate` applies them, into one handoff file for a manual
# run by someone without the
# Go/make/repo-clone toolchain (e.g. a DB admin applying it directly via
# psql or a GUI SQL client). Ported from operations/csm-sync-service's own
# script of the same name and output shape - both services migrate the same
# shared database, so their bootstrap files stay directly comparable.
#
# Default output path includes a UTC timestamp (e.g.
# dist/schema_bootstrap_20260924_153000.sql) so re-generating never silently
# overwrites a previous file - pass an explicit output_file to override this.
#
# Modes (combine --from/--to freely; --since and --from are alternatives,
# not both):
#   ./generate_schema_bootstrap.sh
#       Full bootstrap: every migration file, for a DB that has never run
#       any of them - first-time prod, or a staging reset. Use the SAME
#       generated file for both to keep stg/prod schemas in sync.
#   ./generate_schema_bootstrap.sh --since 0100 [output_file]
#       Delta: only migrations numbered AFTER 0100 (exclusive), for a DB
#       that already has everything up to and including 0100 applied -
#       "give me whatever's new since the last thing I ran".
#   ./generate_schema_bootstrap.sh --from 0101 --to 0110 [output_file]
#       Exact range, both ends INCLUSIVE - e.g. "just this batch",
#       regardless of what's landed on the branch after 0110 since. Either
#       bound can be omitted: --from alone means "0101 onward", --to alone
#       means "everything up to and including 0110".
# --since/--from/--to all accept a bare number ("101") or a full filename
# ("0101_account_support_fields.sql").
#
# Whichever mode, re-run this right before the file actually gets handed
# off if new migration files have landed since - it always reflects
# migrations/*.sql as it exists right now, never a cached prior run.
#
# Deliberately NOT wrapped in an overall BEGIN/COMMIT: some migration files
# do a bare ALTER TYPE ... ADD VALUE, which Postgres refuses to let a later
# statement in the *same* transaction use (see e.g.
# 0097_deployed_product_status_add_retired.sql). Running this with plain
# psql autocommit (one implicit transaction per statement) reproduces
# exactly what `make migrate` already does file-by-file today. Do not wrap
# the run in BEGIN/COMMIT yourself for the same reason - this applies to any
# future migration added the same way, not just today's.
#
# csm_migration_applied_migration is created (if missing) and seeded one
# filename at a time as each file's statements complete, so: (a) if this run
# fails partway through, a subsequent `make migrate` or delta run resumes
# from exactly that point, and (b) a later `make migrate` against the same
# DB correctly skips everything this file already covered.
set -euo pipefail

cd "$(dirname "$0")/.."

# parse_num extracts the leading digit run from either a bare number or a
# full migration filename, then forces base-10 parsing - a leading zero
# would otherwise make bash treat e.g. 0101 as an (invalid) octal literal.
parse_num() {
	local raw="${1%%_*}"
	echo $((10#$raw))
}

since_arg="" from_arg="" to_arg="" out=""
while [[ $# -gt 0 ]]; do
	case "$1" in
	--since)
		since_arg="$2"
		shift 2
		;;
	--from)
		from_arg="$2"
		shift 2
		;;
	--to)
		to_arg="$2"
		shift 2
		;;
	*)
		out="$1"
		shift
		;;
	esac
done

if [[ -n "$since_arg" && -n "$from_arg" ]]; then
	echo "error: --since and --from are alternatives, pass only one" >&2
	exit 1
fi

lower_num=0    # smallest included number, inclusive
lower_exclusive=0
label=""
if [[ -n "$since_arg" ]]; then
	lower_num=$(parse_num "$since_arg")
	lower_exclusive=1
	label="_since_$(printf '%04d' "$lower_num")"
elif [[ -n "$from_arg" ]]; then
	lower_num=$(parse_num "$from_arg")
fi

upper_num=999999
if [[ -n "$to_arg" ]]; then
	upper_num=$(parse_num "$to_arg")
fi

if [[ -n "$from_arg" || -n "$to_arg" ]]; then
	from_label=$([[ -n "$from_arg" ]] && printf '%04d' "$lower_num" || echo "start")
	to_label=$([[ -n "$to_arg" ]] && printf '%04d' "$upper_num" || echo "end")
	label="_${from_label}_to_${to_label}"
fi

commit="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
generated_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
# Filename-safe (no colons) timestamp, so the default output name is unique
# per run instead of always overwriting the last dist/schema_bootstrap.sql -
# keeps a paper trail of what was generated when if you generate more than
# once before actually handing a file off.
file_timestamp="$(date -u +%Y%m%d_%H%M%S)"

out="${out:-dist/schema_bootstrap${label}_${file_timestamp}.sql}"
mkdir -p "$(dirname "$out")"

{
	echo "-- Generated ${generated_at} by scripts/generate_schema_bootstrap.sh from scripts/migration_order.sh at commit ${commit}."
	if [[ -n "$since_arg" ]]; then
		printf -- "-- Delta: migrations numbered after %04d only. For a DB that already\n" "$lower_num"
		printf -- "-- has everything up to and including %04d applied.\n" "$lower_num"
	elif [[ -n "$from_arg" || -n "$to_arg" ]]; then
		echo "-- Range: migrations $([[ -n "$from_arg" ]] && printf '%04d' "$lower_num" || echo "(start)") through $([[ -n "$to_arg" ]] && printf '%04d' "$upper_num" || echo "(end)"), both ends inclusive."
	else
		echo "-- Full bootstrap: every migration, for a DB that has never run any of these -"
		echo "-- first-time prod, or a staging reset. Use the SAME file for both to keep"
		echo "-- them in sync."
	fi
	echo "--"
	echo "-- Run with plain autocommit, e.g.:"
	echo "--   psql \"\$DATABASE_URL\" -v ON_ERROR_STOP=1 -f $(basename "$out")"
	echo "-- Do NOT wrap this in BEGIN/COMMIT or run it with psql -1/--single-transaction -"
	echo "-- see the ALTER TYPE ... ADD VALUE note in scripts/generate_schema_bootstrap.sh."
	echo "-- Do not commit or re-run this file via migrations/*.sql - it is a generated"
	echo "-- one-time handoff artifact, not an ongoing migration file."
	echo
	echo "CREATE TABLE IF NOT EXISTS csm_migration_applied_migration (filename TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now());"
	echo

	count=0
	while IFS= read -r f; do
		name="$(basename "$f")"
		num=$(parse_num "$name")

		if ((lower_exclusive)); then
			((num <= lower_num)) && continue
		else
			((num < lower_num)) && continue
		fi
		((num > upper_num)) && continue

		name_escaped="$(printf '%s' "$name" | sed "s/'/''/g")"
		echo "-- ===== ${name} ====="
		cat "$f"
		echo
		echo "INSERT INTO csm_migration_applied_migration (filename) VALUES ('${name_escaped}');"
		echo
		count=$((count + 1))
	done < <(./scripts/migration_order.sh)

	echo "-- ${count} migrations combined."
} >"$out"

echo "wrote $out ($count migrations combined)"
