#!/usr/bin/env bash
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com). All Rights Reserved.
#
# This software is the property of WSO2 LLC. and its suppliers, if any.
# Dissemination of any information or reproduction of any material contained
# herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
# You may not alter or remove any copyright or other notice from copies of this content.

# Concatenates migrations/*.sql, in the same order `make migrate` applies
# them, into two handoff files for a manual run by someone without the
# Go/make/repo-clone toolchain (e.g. a DB admin applying it directly via
# psql or a GUI SQL client): a schema file (everything except the RLS track)
# and an RLS file (migrations/1NNNNN_*.sql, the dedicated 100000+ number
# space CREATE POLICY / ALTER POLICY / ROW LEVEL SECURITY migrations live in
# - see entity-service/CLAUDE.md's "Database migrations" / the RLS-migration
# renumbering this followed). The schema file must be run first: every RLS
# migration depends on a table some schema migration already created, never
# the other way round - each file's own header says so. Ported from
# operations/csm-sync-service's own script of the same name and output
# shape (that service has no RLS track of its own, hence no split there) -
# both services migrate the same shared database, so the schema file stays
# directly comparable between the two.
#
# File order is NOT a plain alphabetical glob, for the same reason `make
# migrate` isn't (see that target's own comment in the Makefile): three
# phases, in order -
#   1. Every 4-digit NNNN_*.sql file (the convention), ascending.
#   2. Every 6-digit 1NNNNN_*.sql file numbered 100000+ (the RLS track) -
#      written to the RLS file, not the schema file.
#   3. The legacy 6-digit 0NNNNN_*.up.sql stragglers (000020-000105) that
#      predate this whole scheme and still depend on tables the 4-digit
#      files create. *.down.sql files are rollbacks and never run here.
# A plain `migrations/*.sql` glob sorts every 6-digit file in the 000020-
# 000105 range *before* 0001 (leading zeros) and would fail outright applied
# against a from-scratch database - this script used to do exactly that,
# silently, before this fix (it only ever LOOKED sorted because the 100000+
# RLS files happen to sort after everything 4-digit by coincidence of
# starting with "1"; the 000020-000105 legacy files got no such luck).
#
# Default output paths include a UTC timestamp (e.g.
# dist/schema_bootstrap_20260924_153000.sql,
# dist/rls_bootstrap_20260924_153000.sql) so re-generating never silently
# overwrites a previous run - pass an explicit output_file to override the
# schema file's name; the RLS file's name is derived from it (see below).
#
# Modes (combine --from/--to freely; --since and --from are alternatives,
# not both) - the same numeric range applies to BOTH files, since a delta or
# range run is about "what's new", not about schema vs. RLS:
#   ./generate_schema_bootstrap.sh
#       Full bootstrap: every migration file, for a DB that has never run
#       any of them - first-time prod, or a staging reset. Use the SAME
#       generated files for both to keep stg/prod schemas in sync.
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
# ("0101_account_support_fields.sql"). The RLS track's own 100000+ numbers
# are a disjoint range: --to 0110 excludes every RLS file the same way it
# excludes migration 0150, and a range entirely inside 100000+ (e.g.
# --from 100001 --to 100010) naturally produces an empty schema file, which
# is then not written at all (see "A bucket with nothing in it" below).
#
# output_file, when given, names the SCHEMA file; the RLS file's name is
# derived from it by replacing "schema_bootstrap" with "rls_bootstrap" if
# that substring is present, otherwise by prefixing the basename with
# "rls_". This keeps the common case (no argument at all) simple while still
# letting a caller that already names an explicit schema file end up with a
# predictably-named RLS sibling next to it.
#
# A bucket with nothing in it (e.g. --to 0110 with no RLS file numbered that
# low) gets no file at all, matching `make migrate`'s own review-bundle
# behaviour - never an empty, confusing placeholder.
#
# Whichever mode, re-run this right before the files actually get handed
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
# csm_migration_applied_migration is created (if missing, in BOTH files) and
# seeded one filename at a time as each file's statements complete, so: (a)
# if a run fails partway through, a subsequent `make migrate` or delta run
# resumes from exactly that point, and (b) a later `make migrate` against
# the same DB correctly skips everything these files already covered.
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

out_schema="${out:-dist/schema_bootstrap${label}_${file_timestamp}.sql}"
schema_dir="$(dirname "$out_schema")"
schema_base="$(basename "$out_schema")"
if [[ "$schema_base" == *schema_bootstrap* ]]; then
	rls_base="${schema_base/schema_bootstrap/rls_bootstrap}"
else
	rls_base="rls_${schema_base}"
fi
out_rls="${schema_dir}/${rls_base}"
mkdir -p "$schema_dir"

mode_comment() {
	if [[ -n "$since_arg" ]]; then
		printf -- "-- Delta: migrations numbered after %04d only. For a DB that already\n" "$lower_num"
		printf -- "-- has everything up to and including %04d applied.\n" "$lower_num"
	elif [[ -n "$from_arg" || -n "$to_arg" ]]; then
		echo "-- Range: migrations $([[ -n "$from_arg" ]] && printf '%04d' "$lower_num" || echo "(start)") through $([[ -n "$to_arg" ]] && printf '%04d' "$upper_num" || echo "(end)"), both ends inclusive."
	else
		echo "-- Full bootstrap: every migration, for a DB that has never run any of these -"
		echo "-- first-time prod, or a staging reset. Use the SAME files for both to keep"
		echo "-- them in sync."
	fi
}

# Three-phase file order - see this script's own header for why a plain
# `migrations/*.sql` glob is wrong. Mirrors the Makefile's `migrate` target
# exactly, so the two can never disagree about what order migrations apply
# in.
files=$( { ls migrations/[0-9][0-9][0-9][0-9]_*.sql 2>/dev/null | sort; \
            ls migrations/1[0-9][0-9][0-9][0-9][0-9]_*.sql 2>/dev/null | sort; \
            ls migrations/0[0-9][0-9][0-9][0-9][0-9]_*.up.sql 2>/dev/null | sort; } )

schema_body="$(mktemp)"
rls_body="$(mktemp)"
trap 'rm -f "$schema_body" "$rls_body"' EXIT

schema_count=0
rls_count=0
# Set the first time a legacy 0NNNNN_*.up.sql straggler is written to
# schema_body, so that boundary gets one clear marker instead of the
# 4-digit schema files and the legacy stragglers running together with
# nothing in the file itself saying where one ends and the other begins.
legacy_marker_written=0
for f in $files; do
	name="$(basename "$f")"
	num=$(parse_num "$name")

	if ((lower_exclusive)); then
		((num <= lower_num)) && continue
	else
		((num < lower_num)) && continue
	fi
	((num > upper_num)) && continue

	name_escaped="$(printf '%s' "$name" | sed "s/'/''/g")"
	block="$(mktemp)"
	{
		echo "-- ===== ${name} ====="
		cat "$f"
		echo
		echo "INSERT INTO csm_migration_applied_migration (filename) VALUES ('${name_escaped}');"
		echo
	} >"$block"

	case "$name" in
	1[0-9][0-9][0-9][0-9][0-9]_*.sql)
		cat "$block" >>"$rls_body"
		rls_count=$((rls_count + 1))
		;;
	0[0-9][0-9][0-9][0-9][0-9]_*.up.sql)
		# The legacy stragglers (phase 3) still land in the schema file, not a
		# third output file - there is no dependency between this cluster and
		# the RLS track either way (confirmed: it covers KB tables,
		# cloud_status_events and outage_communications, none of which any RLS
		# migration touches), so which of the two files they end up in has no
		# functional effect. This marker exists purely so a human reading the
		# schema file can see where phase 1 (ordinary schema migrations) ends
		# and phase 3 (the legacy, pre-NNNN_*.sql stragglers) begins, instead
		# of the two running together indistinguishably.
		if ((!legacy_marker_written)); then
			{
				echo "-- ===== legacy 6-digit stragglers below (000020-000105; predate the"
				echo "-- NNNN_*.sql convention; see entity-service/CLAUDE.md's \"Database"
				echo "-- migrations\" section) ====="
				echo
			} >>"$schema_body"
			legacy_marker_written=1
		fi
		cat "$block" >>"$schema_body"
		schema_count=$((schema_count + 1))
		;;
	*)
		cat "$block" >>"$schema_body"
		schema_count=$((schema_count + 1))
		;;
	esac
	rm -f "$block"
done

if ((schema_count > 0)); then
	{
		echo "-- Generated ${generated_at} by scripts/generate_schema_bootstrap.sh from migrations/*.sql at commit ${commit}."
		echo "-- SCHEMA file: everything except the dedicated RLS track (migrations/1NNNNN_*.sql)."
		echo "-- Run this FIRST - the RLS file (see its own sibling output) depends on tables this one creates."
		mode_comment
		echo "--"
		echo "-- Run with plain autocommit, e.g.:"
		echo "--   psql \"\$DATABASE_URL\" -v ON_ERROR_STOP=1 -f $(basename "$out_schema")"
		echo "-- Do NOT wrap this in BEGIN/COMMIT or run it with psql -1/--single-transaction -"
		echo "-- see the ALTER TYPE ... ADD VALUE note in scripts/generate_schema_bootstrap.sh."
		echo "-- Do not commit or re-run this file via migrations/*.sql - it is a generated"
		echo "-- one-time handoff artifact, not an ongoing migration file."
		echo
		echo "CREATE TABLE IF NOT EXISTS csm_migration_applied_migration (filename TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now());"
		echo
		cat "$schema_body"
		echo "-- ${schema_count} migrations combined."
	} >"$out_schema"
else
	out_schema=""
fi

if ((rls_count > 0)); then
	{
		echo "-- Generated ${generated_at} by scripts/generate_schema_bootstrap.sh from migrations/*.sql at commit ${commit}."
		echo "-- RLS file: migrations/1NNNNN_*.sql only (CREATE POLICY / ALTER POLICY / ROW LEVEL"
		echo "-- SECURITY and the functions that support them). Run this SECOND, after the schema"
		echo "-- file (see its own sibling output) has already created every table these policies"
		echo "-- protect - never the other way round."
		mode_comment
		echo "--"
		echo "-- Run with plain autocommit, e.g.:"
		echo "--   psql \"\$DATABASE_URL\" -v ON_ERROR_STOP=1 -f $(basename "$out_rls")"
		echo "-- Do NOT wrap this in BEGIN/COMMIT or run it with psql -1/--single-transaction -"
		echo "-- see the ALTER TYPE ... ADD VALUE note in scripts/generate_schema_bootstrap.sh."
		echo "-- Do not commit or re-run this file via migrations/*.sql - it is a generated"
		echo "-- one-time handoff artifact, not an ongoing migration file."
		echo
		echo "CREATE TABLE IF NOT EXISTS csm_migration_applied_migration (filename TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now());"
		echo
		cat "$rls_body"
		echo "-- ${rls_count} migrations combined."
	} >"$out_rls"
else
	out_rls=""
fi

[[ -n "$out_schema" ]] && echo "wrote $out_schema ($schema_count migrations combined)"
[[ -n "$out_rls" ]] && echo "wrote $out_rls ($rls_count migrations combined)"
if [[ -z "$out_schema" && -z "$out_rls" ]]; then
	echo "no migrations matched the given range - nothing written" >&2
	exit 1
fi
