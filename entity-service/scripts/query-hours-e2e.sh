#!/bin/zsh
# Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
#
# End-to-end test for the query-hour port (ServiceNow `[Query Hour]
# UpdateTime Card`). Stands up a THROWAWAY PostgreSQL cluster, applies the
# real csm-sync-service migrations plus this repo's 000084, seeds five
# scenarios, runs entity-service and a mock Choreo, drives every endpoint and
# asserts the results. Touches no existing database and no real Choreo.
#
#   ./scripts/query-hours-e2e.sh /path/to/digiops-cs
#   ./scripts/query-hours-e2e.sh /path/to/digiops-cs --keep
#
# --keep leaves Postgres, entity-service and the mock Choreo running after the
# assertions so you can drive the endpoints by hand. It prints the curl
# commands and a shutdown line when it finishes.
#
# Everything lands under /tmp/cqh and is removed on exit. The short path is
# deliberate: a Unix socket path is capped at 103 bytes, which rules out most
# temp directories.
set -u

SELF="${0:A}"

# --stop tears down a stack left running by --keep.
if [[ "${1:-}" == "--stop" ]]; then
  PGBIN="${PGBIN:-/opt/homebrew/opt/postgresql@15/bin}"
  pkill -f "cqh/entity-api" 2>/dev/null
  pkill -f "cqh/choreo.py" 2>/dev/null
  [[ -d /tmp/cqh/data ]] && $PGBIN/pg_ctl -D /tmp/cqh/data stop >/dev/null 2>&1
  rm -rf /tmp/cqh
  echo "stopped and removed /tmp/cqh"
  exit 0
fi

KEEP=0
args=()
for a in "$@"; do
  case "$a" in
    --keep) KEEP=1 ;;
    *) args+=("$a") ;;
  esac
done
set -- "${args[@]}"

SYNC_REPO="${1:-}"
if [[ -z "$SYNC_REPO" ]]; then
  echo "usage: $0 /path/to/digiops-cs [--keep]" >&2
  echo "  (the checkout containing operations/csm-sync-service/migrations)" >&2
  exit 2
fi
MIGRATIONS="$SYNC_REPO/operations/csm-sync-service/migrations"
if [[ ! -d "$MIGRATIONS" ]]; then
  echo "not found: $MIGRATIONS" >&2
  exit 2
fi

export LANG=C LC_ALL=C
PGBIN="${PGBIN:-/opt/homebrew/opt/postgresql@15/bin}"
GOBIN_DIR="${GOBIN_DIR:-/opt/homebrew/bin}"
export PATH="$GOBIN_DIR:$PATH"
BASE=/tmp/cqh
PGPORT=55432
APP_PORT=8091
CHOREO_PORT=8099

# Every query-hour endpoint resolves the caller's scope, and a request with no
# credentials at all is rejected -- 401 for the per-project routes, 403 for the
# sweep, which is internal-only. So the script has to present a credential.
#
# x-jwt-assertion is DECODED, never signature-verified (see
# auth.Validator.ExtractClientID), so a locally minted, unsigned assertion
# carrying a client_id is enough, and the service is told that client id is
# internal via AUTH_INTERNAL_CLIENT_IDS. Nothing here weakens the real
# deployment: it only exercises the same code path a real internal service
# takes. An earlier version of this script predates the access checks entirely
# and could not reach any of these endpoints.
E2E_CLIENT_ID=query-hours-e2e
b64url() { python3 -c "import base64,sys;sys.stdout.write(base64.urlsafe_b64encode(sys.stdin.buffer.read()).decode().rstrip('='))" }
E2E_ASSERTION="$(printf '%s' '{"alg":"none","typ":"JWT"}' | b64url).$(printf '%s' "{\"client_id\":\"$E2E_CLIENT_ID\"}" | b64url)."
AUTH_HEADER="x-jwt-assertion: $E2E_ASSERTION"
ENTITY_DIR="${0:A:h}/.."

# The five seeded projects (see testdata/query_hours_seed.sql). Declared here
# rather than in the assertions because --keep's banner references them too.
P1=11111111-1111-1111-1111-111111111111  # 10%            -> state 0
P2=22222222-2222-2222-2222-222222222222  # 75%            -> state 1
P3=33333333-3333-3333-3333-333333333333  # 90%            -> state 2
P4=44444444-4444-4444-4444-444444444444  # 125%           -> state 3
P5=55555555-5555-5555-5555-555555555555  # no entitlement -> state 0

pass=0; fail=0
ok()   { print -P "  %F{green}PASS%f $1"; pass=$((pass+1)) }
bad()  { print -P "  %F{red}FAIL%f $1"; fail=$((fail+1)) }
step() { print -P "\n%F{cyan}=== $1 ===%f" }

ENTITY_PID=""; CHOREO_PID=""
keep_banner() {
    print -P "\n%F{yellow}--keep: leaving the stack running.%f"
    print    "  entity-service : http://127.0.0.1:$APP_PORT"
    print    "  mock Choreo    : http://127.0.0.1:$CHOREO_PORT   (GET / lists every push it received)"
    print    "  postgres       : psql -h 127.0.0.1 -p $PGPORT -U postgres -d csm   (password: postgres)"
    print    ""
    print    "  # the five seeded projects"
    print    "  psql -h 127.0.0.1 -p $PGPORT -U postgres -d csm -c \\"
    print    "    \"SELECT key, id FROM project WHERE created_by='query-hours-seed' ORDER BY key;\""
    print    ""
    print    "  # recompute one project (P2 is the 75% / state-1 case)"
    print    "  curl -s -X POST http://127.0.0.1:$APP_PORT/projects/$P2/query-hours/recompute | python3 -m json.tool"
    print    ""
    print    "  # read it back without recomputing"
    print    "  curl -s http://127.0.0.1:$APP_PORT/projects/$P2/query-hours | python3 -m json.tool"
    print    ""
    print    "  # recompute via a time card (proves the fan-out is gone)"
    print    "  curl -s -X POST http://127.0.0.1:$APP_PORT/time-cards/dddddddd-0000-0000-0000-000000000002/query-hours/recompute | python3 -m json.tool"
    print    ""
    print    "  # sweep the stale ones"
    print    "  curl -s -X POST 'http://127.0.0.1:$APP_PORT/query-hours/sweep?staleForMinutes=0&limit=50' | python3 -m json.tool"
    print    ""
    print    "  # what Choreo was told"
    print    "  curl -s http://127.0.0.1:$CHOREO_PORT/ | python3 -m json.tool"
    print    ""
    print    "  # change consumption and watch the state move"
    print    "  psql -h 127.0.0.1 -p $PGPORT -U postgres -d csm -c \\"
    print    "    \"UPDATE time_card SET analyzing_minutes=5900 WHERE id='dddddddd-0000-0000-0000-000000000001';\""
    print    "  curl -s -X POST http://127.0.0.1:$APP_PORT/projects/$P1/query-hours/recompute | python3 -m json.tool"
    print    ""
    print -P "%F{yellow}  SHUT DOWN WHEN DONE:%f  $SELF --stop"
    return
}

# The EXIT trap decides; `cleanup` itself always really cleans, because step 1
# calls it directly to clear any leftovers from an aborted run.
on_exit() {
  if [[ "$KEEP" == "1" ]]; then keep_banner; else cleanup; fi
}

cleanup() {
  # go run spawns the compiled binary as a separate child; kill both, but
  # never by process group — this script shares its group with them.
  [[ -n "$ENTITY_PID" ]] && kill $ENTITY_PID 2>/dev/null
  [[ -n "$CHOREO_PID" ]] && kill $CHOREO_PID 2>/dev/null
  pkill -f "cqh/choreo.py" 2>/dev/null
  pkill -f "cqh/entity-api" 2>/dev/null
  [[ -d $BASE/data ]] && $PGBIN/pg_ctl -D $BASE/data stop >/dev/null 2>&1
  rm -rf $BASE
}
trap on_exit EXIT INT TERM

q()  { $PGBIN/psql -h 127.0.0.1 -p $PGPORT -U postgres -d csm -t -A "$@" }
# -1 wraps each file in a single transaction, so a DDL failure partway through
# rolls the whole file back rather than leaving half a migration applied. The
# compose migration runner already does this; this script now matches it.
qq() { $PGBIN/psql -h 127.0.0.1 -p $PGPORT -U postgres -d csm -v ON_ERROR_STOP=1 -1 -q "$@" }

# A previous aborted run can leave the ports held, which makes the service
# die instantly with "address already in use" and look like a build failure.
free_port() {
  local held=$(lsof -ti tcp:$1 2>/dev/null)
  [[ -n "$held" ]] && { echo "  freeing port $1 (held by $held)"; echo $held | xargs kill -9 2>/dev/null; sleep 1 }
}

step "1/7  throwaway PostgreSQL"
cleanup
for p in $PGPORT $APP_PORT $CHOREO_PORT; do free_port $p; done
mkdir -p $BASE/data
$PGBIN/initdb -D $BASE/data -U postgres --auth=trust --locale=C --encoding=UTF8 >/dev/null 2>&1 \
  || { echo "initdb failed"; exit 1; }
$PGBIN/pg_ctl -D $BASE/data \
  -o "-p $PGPORT -k $BASE -c listen_addresses=127.0.0.1" -l $BASE/pg.log start >/dev/null 2>&1
sleep 2
$PGBIN/psql -h 127.0.0.1 -p $PGPORT -U postgres -q -c "CREATE DATABASE csm;" || exit 1
$PGBIN/psql -h 127.0.0.1 -p $PGPORT -U postgres -q -c "ALTER USER postgres WITH PASSWORD 'postgres';"
echo "  up on 127.0.0.1:$PGPORT"

step "2/7  schema (real csm-sync-service migrations, then 000084)"
applied=0
for f in $(ls $MIGRATIONS/*.sql | sort); do
  qq -f "$f" >/dev/null 2>>$BASE/mig.err && applied=$((applied+1))
done
echo "  csm-sync-service: $applied applied"
qq -f "$ENTITY_DIR/migrations/000084_create_project_query_hours.up.sql" >/dev/null \
  && echo "  000084_create_project_query_hours: applied"
[[ $(q -c "SELECT to_regclass('project_query_hours') IS NOT NULL;") == "t" ]] \
  && ok "project_query_hours exists" || bad "project_query_hours missing"

step "3/7  seed"
qq -f "$ENTITY_DIR/testdata/query_hours_seed.sql" >/dev/null && echo "  5 scenarios seeded"

step "4/7  mock Choreo on :$CHOREO_PORT"
cat > $BASE/choreo.py <<'PY'
import json
from http.server import BaseHTTPRequestHandler, HTTPServer
CALLS = []
class H(BaseHTTPRequestHandler):
    def do_PUT(self):
        n = int(self.headers.get('Content-Length') or 0)
        CALLS.append({"path": self.path, "body": self.rfile.read(n).decode() if n else ''})
        self.send_response(200); self.send_header('Content-Type','application/json'); self.end_headers()
        self.wfile.write(b'{"status":"Project state changed"}')
    def do_GET(self):
        b = json.dumps(CALLS).encode()
        self.send_response(200); self.send_header('Content-Type','application/json'); self.end_headers()
        self.wfile.write(b)
    def log_message(self, *a): pass
HTTPServer(('127.0.0.1', 8099), H).serve_forever()
PY
python3 $BASE/choreo.py > $BASE/choreo.log 2>&1 &
CHOREO_PID=$!
sleep 1
calls() { curl -s http://127.0.0.1:$CHOREO_PORT/ | python3 -c "import json,sys;print(len(json.load(sys.stdin)))" }
[[ $(calls) == "0" ]] && ok "mock Choreo up" || bad "mock Choreo not responding"

step "5/7  entity-service on :$APP_PORT"
# Built ahead of time rather than `go run`: compilation inside the readiness
# wait made startup look like a failure on a cold build cache.
( cd "$ENTITY_DIR" && go build -o $BASE/entity-api ./cmd/api ) \
  || { bad "entity-service failed to build"; exit 1 }
echo "  built"
( cd "$ENTITY_DIR" && \
  DATA_SOURCE=postgres DB_HOST=127.0.0.1 DB_PORT=$PGPORT DB_USER=postgres \
  DB_PASSWORD=postgres DB_NAME=csm DB_SSLMODE=disable SERVER_PORT=$APP_PORT \
  QUERY_HOUR_CHOREO_BASE_URL=http://127.0.0.1:$CHOREO_PORT \
  QUERY_HOUR_CHOREO_API_KEY=test-key \
  AUTH_INTERNAL_CLIENT_IDS=$E2E_CLIENT_ID \
  $BASE/entity-api ) > $BASE/entity.log 2>&1 &
ENTITY_PID=$!
B="http://127.0.0.1:$APP_PORT"
for i in {1..30}; do
  curl -s -o /dev/null -H "$AUTH_HEADER" "$B/projects/11111111-1111-1111-1111-111111111111/query-hours" && break
  sleep 1
done
sleep 1
grep -q "started in PORT" $BASE/entity.log && ! grep -q "server error" $BASE/entity.log \
  && ok "entity-service started" \
  || { bad "entity-service did not start"; cp $BASE/entity.log /tmp/query-hours-e2e-entity.log 2>/dev/null;
       echo "  --- entity.log (copied to /tmp/query-hours-e2e-entity.log) ---"; tail -15 $BASE/entity.log; exit 1 }

step "6/7  behaviour"
field() { python3 -c "import json,sys;print(json.load(sys.stdin)['$1'])" }
recompute() { curl -s -X POST -H "$AUTH_HEADER" "$B/projects/$1/query-hours/recompute" }

for spec in "$P1 0 600" "$P2 1 4500" "$P3 2 5400" "$P4 3 7500" "$P5 0 300"; do
  set -- ${=spec}
  r=$(recompute $1)
  st=$(echo $r | field queryHourState); cm=$(echo $r | field consumedMinutes)
  [[ "$st" == "$2" && "$cm" == "$3" ]] \
    && ok "state=$st consumed=$cm" || bad "state=$st (want $2) consumed=$cm (want $3)"
done

# The SUBMITTED 9000-minute decoy on P1 must never be counted.
[[ $(recompute $P1 | field consumedMinutes) == "600" ]] \
  && ok "SUBMITTED cards excluded" || bad "SUBMITTED card leaked into the total"

# Re-running unchanged must not re-push.
before=$(calls); recompute $P4 >/dev/null; after=$(calls)
[[ "$before" == "$after" ]] && ok "no re-push when nothing changed" || bad "re-pushed unnecessarily"

# The ServiceNow divergence: state must fall when consumption falls.
qq -c "UPDATE time_card SET analyzing_minutes=3000, setting_up_minutes=0,
        reproducing_debugging_minutes=0, providing_solution_minutes=0, patching_minutes=0
       WHERE id='dddddddd-0000-0000-0000-000000000004';" >/dev/null
[[ $(recompute $P4 | field queryHourState) == "0" ]] \
  && ok "state walks back down 3->0 (ServiceNow stayed at 3)" || bad "state did not walk down"

# The other divergence: no account-wide fan-out. All five share one account.
qq -c "UPDATE project_query_hours SET computed_at = NOW() - INTERVAL '10 days';" >/dev/null
curl -s -X POST -H "$AUTH_HEADER" "$B/time-cards/dddddddd-0000-0000-0000-000000000002/query-hours/recompute" >/dev/null
moved=$(q -c "SELECT count(*) FROM project_query_hours WHERE computed_at > NOW() - INTERVAL '1 minute';")
[[ "$moved" == "1" ]] && ok "one time card recomputed exactly 1 of 5 projects" \
                      || bad "recomputed $moved projects, want 1"

# Sweep, then immediately again: nothing left stale.
qq -c "UPDATE project_query_hours SET computed_at = NOW() - INTERVAL '10 days';" >/dev/null
s1=$(curl -s -X POST -H "$AUTH_HEADER" "$B/query-hours/sweep?staleForMinutes=60&limit=50" | field succeeded)
s2=$(curl -s -X POST -H "$AUTH_HEADER" "$B/query-hours/sweep?staleForMinutes=60&limit=50" | field requested)
[[ "$s1" == "5" && "$s2" == "0" ]] && ok "sweep did 5, then 0 (no redundant work)" \
                                   || bad "sweep did $s1 then requested $s2"

# A dead Choreo must not fail the recompute; the next one must retry.
kill $CHOREO_PID 2>/dev/null; sleep 1
qq -c "UPDATE time_card SET analyzing_minutes=5700 WHERE id='dddddddd-0000-0000-0000-000000000003';" >/dev/null
code=$(curl -s -o $BASE/pf.json -w "%{http_code}" -X POST -H "$AUTH_HEADER" "$B/projects/$P3/query-hours/recompute")
[[ "$code" == "200" ]] && ok "push failure returns 200, position still stored" \
                       || bad "push failure returned $code"
[[ $(q -c "SELECT last_pushed_state <> query_hour_state FROM project_query_hours WHERE project_id='$P3';") == "t" ]] \
  && ok "push left pending for retry" || bad "push not marked pending"
python3 $BASE/choreo.py > $BASE/choreo2.log 2>&1 &
CHOREO_PID=$!
sleep 1.5
[[ $(recompute $P3 | field pushed) == "True" ]] \
  && ok "retried the pending push once Choreo returned" || bad "did not retry"

# Error paths.
for spec in "POST /projects/99999999-9999-9999-9999-999999999999/query-hours/recompute 404" \
            "POST /time-cards/99999999-9999-9999-9999-999999999999/query-hours/recompute 404" \
            "POST /query-hours/sweep?limit=0 400" \
            "POST /query-hours/sweep?limit=99999 400"; do
  set -- ${=spec}
  got=$(curl -s -o /dev/null -w "%{http_code}" -X $1 -H "$AUTH_HEADER" "$B$2")
  [[ "$got" == "$3" ]] && ok "$2 -> $got" || bad "$2 -> $got (want $3)"
done

# The design constraint: the sync-owned column must never be written.
[[ $(q -c "SELECT count(*) FROM project WHERE created_by='query-hours-seed' AND consumed_duration IS NOT NULL;") == "0" ]] \
  && ok "project.consumed_duration untouched (sync-owned)" || bad "Go wrote a sync-owned column"

step "7/7  result"
print -P "  %F{green}$pass passed%f, %F{red}$fail failed%f"
exit $(( fail > 0 ))
