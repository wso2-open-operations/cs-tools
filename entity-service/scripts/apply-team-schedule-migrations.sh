#!/usr/bin/env bash
# Apply ONLY the Team Schedule migration chain (0152-0168), in filename order,
# recording each in csm_migration_applied_migration exactly as `make migrate`
# does -- so a later full `make migrate` skips them instead of re-running.
#
# Why not just `make migrate`: that applies EVERY file not in the ledger. There
# are 211 of them, and a database whose ledger is sparse would have a very large
# number applied in one go. This narrows the blast radius to the 21 files that
# create the tables the service is currently missing.
#
# Defaults to a dry run. Set APPLY=1 to actually write.
#
#   DB_HOST=... DB_USER=... DB_PASSWORD=... DB_NAME=... ./scripts/apply-team-schedule-migrations.sh
#   DB_HOST=... ... APPLY=1 ./scripts/apply-team-schedule-migrations.sh
set -euo pipefail
cd "$(dirname "$0")/.."

set -a; [ -f .env ] && . ./.env; set +a
: "${DB_HOST:?DB_HOST is required}"; : "${DB_USER:?DB_USER is required}"
: "${DB_NAME:?DB_NAME is required}"; : "${DB_PASSWORD:?DB_PASSWORD is required}"
export PGHOST="$DB_HOST" PGPORT="${DB_PORT:-5432}" PGUSER="$DB_USER" \
       PGPASSWORD="$DB_PASSWORD" PGDATABASE="$DB_NAME" PGSSLMODE="${DB_SSLMODE:-require}"

APPLY="${APPLY:-0}"
# Exactly the Team Schedule chain: 0152-0168 and the rotas, 0199-0200. An
# open-ended "schedule" match also caught 0183_service_commitment_schedule_id,
# which is not Team Schedule at all.
files=$(ls migrations/*.sql | sort | awk -F/ '
  /schedule/ && (($NF >= "0152" && $NF < "0169") || ($NF >= "0199" && $NF < "0201"))')

echo "target : $PGUSER@$PGHOST/$PGDATABASE"
echo "mode   : $([ "$APPLY" = 1 ] && echo 'APPLY (writes)' || echo 'dry run (no writes)')"
echo "schema : $(psql -tAc 'SELECT current_schemas(true)')"
echo

ledger_exists=$(psql -tAc "SELECT to_regclass('csm_migration_applied_migration') IS NOT NULL;")
if [ "$ledger_exists" != "t" ]; then
    echo "note   : no csm_migration_applied_migration table on this database"
    [ "$APPLY" = 1 ] && psql -v ON_ERROR_STOP=1 -q -c \
      "CREATE TABLE IF NOT EXISTS csm_migration_applied_migration (filename TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now());"
fi

pending=0
for f in $files; do
    name=$(basename "$f")
    esc=${name//\'/\'\'}
    applied=""
    [ "$ledger_exists" = "t" ] && applied=$(psql -tAc \
        "SELECT 1 FROM csm_migration_applied_migration WHERE filename = '$esc';")
    if [ "$applied" = "1" ]; then printf '  skip        %s\n' "$name"; continue; fi
    pending=$((pending+1))
    if [ "$APPLY" != 1 ]; then printf '  WOULD APPLY %s\n' "$name"; continue; fi
    printf '  applying    %s\n' "$name"
    psql -v ON_ERROR_STOP=1 -q -f "$f"
    psql -v ON_ERROR_STOP=1 -q -c \
        "INSERT INTO csm_migration_applied_migration (filename) VALUES ('$esc') ON CONFLICT DO NOTHING;"
done

echo
echo "pending: $pending"
[ "$APPLY" = 1 ] && echo "done." || echo "dry run only -- re-run with APPLY=1 to write."
