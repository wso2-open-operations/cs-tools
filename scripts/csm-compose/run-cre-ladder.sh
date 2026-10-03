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
#
# Run ONE CRE escalation ladder end to end, the way a real incident would.
#
# The real engine, the real Team Schedule (people, rota, nominees), real
# acknowledgement events, and a real clock unless you ask for a fast one. The
# shift is derived by the engine from the time the incident was reported,
# exactly as it is in production.
#
# By default NOBODY IS CONTACTED: --channel log prints who each rung would have
# reached. --channel call goes to a local stub unless --live is also given, and
# --live sends every rung to the one number in --to, so it can only ever ring
# your own phone. --channel chat posts to a real Google Chat space and asks
# first.
#
# Needs the compose stack's postgres, entity-service and mock-oidc running, and
# docker + go. It starts its own Redis on its own port, so it never works the
# same ladder keys as the running notification service.

set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: run-cre-ladder.sh [options]

What the incident is
  -s, --severity S0..S4     severity (P0..P4 also accepted)               default S0
  -a, --abt TEAM            the ABT it is assigned to; omit for UNASSIGNED
                            (one of the CRE ABTs in escalation.yaml)

When it was reported -- pick one; the engine derives the shift from it
      --shift SHIFT         LK | LK_MORNING | LK_EVENING | LK_WEEKEND | USA | USA_WEEKEND
  -t, --at TIME             IST: HH:MM (next weekday) or YYYY-MM-DDTHH:MM
      --weekend             with --at HH:MM, the next Saturday/Sunday instead
                                                                          default --shift LK

How a rung reaches people
  -c, --channel CHANNEL     log | call | chat | both (= call and chat)    default log
      --live                actually place calls through Twilio (needs --to)
      --to NUMBER           E.164; with --live, EVERY rung rings this number

Acknowledging it
      --ack-at LEVEL_n      acknowledge once that rung has been called
      --ack-by GESTURE      both | status | comment                       default both
                            (a CRE ladder stops only on BOTH; the other two
                             show it keep climbing)

Speed
  -m, --minute DURATION     how long one ladder minute lasts              default 1m (real)
      --fast                one ladder minute = 150ms
      --max-calls N         refuse a plan larger than this                default 80

Other
  -y, --yes                 do not ask before posting to chat or dialling for real
      --keep-redis          leave this run's Redis running afterwards
  -h, --help

Examples
  run-cre-ladder.sh                                    # S0, unassigned, LK, logged, real time
  run-cre-ladder.sh -s S1 -a vega --fast               # R2 at S1, in seconds
  run-cre-ladder.sh -s S3 -a vega --at 19:30           # an LK_EVENING S3, by time
  run-cre-ladder.sh -s S0 -a vega --ack-at LEVEL_2     # does it stop at Level 2?
  run-cre-ladder.sh -s S0 --at 10:00 --weekend         # LK_WEEKEND, unassigned
  run-cre-ladder.sh -s S0 -a vega -c call --live --to +94770000000   # rings YOUR phone
USAGE
}

die() { echo "error: $*" >&2; exit 1; }

# -- defaults ---------------------------------------------------------------

SEVERITY=S0
ABT=""
SHIFT=""
AT=""
WEEKEND=""
CHANNEL=log
LIVE=""
TO=""
ACK_AT=""
ACK_BY=both
MINUTE=1m
TICK=""
MAX_CALLS=80
ASSUME_YES=""
KEEP_REDIS=""

REDIS_NAME="cre-ladder-run-redis"
REDIS_PORT="${REDIS_PORT:-16393}"
ENTITY_URL="${CUSTOMER_ENTITY_BASE_URL_OVERRIDE:-http://localhost:8081}"
TOKEN_URL="http://localhost:9100/oauth2/token"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
service_dir="${repo_root}/integrations/csm-notification-service"
config_file="${repo_root}/scripts/csm-compose/escalation.yaml"

# -- flags ------------------------------------------------------------------

need() { [ $# -ge 2 ] && [ -n "$2" ] || die "$1 needs a value"; }

while [ $# -gt 0 ]; do
  case "$1" in
    -s|--severity)  need "$@"; SEVERITY="$2"; shift 2 ;;
    -a|--abt)       need "$@"; ABT="$2"; shift 2 ;;
    --shift)        need "$@"; SHIFT="$2"; shift 2 ;;
    -t|--at)        need "$@"; AT="$2"; shift 2 ;;
    --weekend)      WEEKEND=1; shift ;;
    -c|--channel)   need "$@"; CHANNEL="$2"; shift 2 ;;
    --live)         LIVE=1; shift ;;
    --to)           need "$@"; TO="$2"; shift 2 ;;
    --ack-at)       need "$@"; ACK_AT="$2"; shift 2 ;;
    --ack-by)       need "$@"; ACK_BY="$2"; shift 2 ;;
    -m|--minute)    need "$@"; MINUTE="$2"; shift 2 ;;
    --fast)         MINUTE=150ms; shift ;;
    --max-calls)    need "$@"; MAX_CALLS="$2"; shift 2 ;;
    -y|--yes)       ASSUME_YES=1; shift ;;
    --keep-redis)   KEEP_REDIS=1; shift ;;
    -h|--help)      usage; exit 0 ;;
    *)              echo "unknown option: $1" >&2; echo >&2; usage >&2; exit 2 ;;
  esac
done

# -- validation ---------------------------------------------------------------

upper() { printf '%s' "$1" | tr '[:lower:]' '[:upper:]'; }

SEVERITY="$(upper "${SEVERITY}")"
case "${SEVERITY}" in
  S0|P0) PRIORITY=P0; LADDER_LEN=16 ;;
  S1|P1) PRIORITY=P1; LADDER_LEN=48 ;;
  S2|P2) PRIORITY=P2; LADDER_LEN=75 ;;
  S3|P3) PRIORITY=P3; LADDER_LEN=115 ;;
  S4|P4) PRIORITY=P4; LADDER_LEN=143 ;;
  *) die "--severity must be S0..S4 (or P0..P4), not '${SEVERITY}'" ;;
esac

# The CRE ABTs, read from the cre: block only -- the same file has an sre:
# block with its own abts list, and picking that up would let an SRE team pass
# as a CRE ABT.
cre_abts="$(awk '
  /^cre:/ {in_cre=1; next}
  /^[a-z]+:/ {in_cre=0}
  in_cre && /^[[:space:]]*abts:/ {
    sub(/.*\[/, ""); sub(/\].*/, ""); gsub(/[[:space:]]/, ""); print; exit
  }' "${config_file}")"
[ -n "${cre_abts}" ] || die "could not read the CRE abts list from ${config_file}"

if [ -n "${ABT}" ]; then
  ABT="$(printf '%s' "${ABT}" | tr '[:upper:]' '[:lower:]')"
  case ",${cre_abts}," in
    *",${ABT},"*) ;;
    *) die "--abt '${ABT}' is not a CRE ABT. Use one of: ${cre_abts//,/, } -- or omit --abt for an unassigned incident" ;;
  esac
fi

[ -n "${SHIFT}" ] && [ -n "${AT}" ] && die "--shift and --at are alternatives; give one (the engine derives the shift from --at)"
[ -n "${WEEKEND}" ] && [ -z "${AT}" ] && die "--weekend only applies to --at HH:MM"
if [ -n "${SHIFT}" ]; then
  SHIFT="$(upper "${SHIFT}")"
  case "${SHIFT}" in
    LK|LK_MORNING|LK_EVENING|LK_WEEKEND|USA|USA_WEEKEND) ;;
    *) die "--shift must be LK, LK_MORNING, LK_EVENING, LK_WEEKEND, USA or USA_WEEKEND" ;;
  esac
fi
[ -z "${SHIFT}" ] && [ -z "${AT}" ] && SHIFT=LK

CHANNEL="$(printf '%s' "${CHANNEL}" | tr '[:upper:]' '[:lower:]')"
case "${CHANNEL}" in
  log|call|chat|both) ;;
  *) die "--channel must be log, call, chat or both" ;;
esac

if [ -n "${LIVE}" ]; then
  case "${CHANNEL}" in call|both) ;; *) die "--live only means something with --channel call or both" ;; esac
  [ -n "${TO}" ] || die "--live needs --to: every rung rings that one number, so it cannot reach anybody else"
fi
[ -n "${TO}" ] && [ -z "${LIVE}" ] && die "--to only applies with --live; without it calls go to a local stub"

if [ -n "${ACK_AT}" ]; then
  ACK_AT="$(upper "${ACK_AT}")"
  case "${ACK_AT}" in LEVEL_[0-4]) ;; *) die "--ack-at must be LEVEL_0..LEVEL_4" ;; esac
fi
case "${ACK_BY}" in both|status|comment) ;; *) die "--ack-by must be both, status or comment" ;; esac

# A tick well under one ladder minute keeps a call from being noticed late.
case "${MINUTE}" in
  1m|60s) TICK=5s ;;
  *)      TICK=50ms ;;
esac

# -- preconditions ------------------------------------------------------------

command -v docker >/dev/null || die "docker is required"
command -v go >/dev/null     || die "go is required"
command -v curl >/dev/null   || die "curl is required"

code="$(curl -s -o /dev/null -m 3 -w '%{http_code}' "${ENTITY_URL}/health" || true)"
[ "${code}" = "000" ] && die "entity-service is not answering at ${ENTITY_URL}. Start the stack: docker compose up -d postgres mock-oidc entity-service"

# The client id is what entity-service decides trust on. The service's own .env
# carries the real Asgardeo id, which the local container does not list in
# AUTH_INTERNAL_CLIENT_IDS -- every rung would come back RESOLVE_FAILED.
export OAUTH2_CLIENT_ID="${OAUTH2_CLIENT_ID_OVERRIDE:-csm-notification-service-dev-client}"
export OAUTH2_CLIENT_SECRET="${OAUTH2_CLIENT_SECRET_OVERRIDE:-dev-secret}"
export OAUTH2_TOKEN_URL="${TOKEN_URL}"
export CUSTOMER_ENTITY_BASE_URL="${ENTITY_URL}"
export USE_TEAM_SCHEDULE=1
# The ABT list from escalation.yaml, so a run follows the file a deployment
# reads rather than escalation-local's own fallback. The harness does not load
# the YAML itself yet; this is the one setting it can take from the
# environment, and the one that decides who LEVEL_0 and LEVEL_2 can reach.
export INCIDENT_ESCALATION_ABT_TEAMS="${cre_abts}"

token_code="$(curl -s -o /dev/null -m 3 -w '%{http_code}' -X POST "${TOKEN_URL}" \
  -d grant_type=client_credentials -d client_id="${OAUTH2_CLIENT_ID}" \
  -d client_secret="${OAUTH2_CLIENT_SECRET}" || true)"
[ "${token_code}" = "200" ] || die "mock-oidc at ${TOKEN_URL} did not issue a token (HTTP ${token_code}). Start it: docker compose up -d mock-oidc"

# -- confirm anything that reaches a real person ------------------------------

confirm() {
  [ -n "${ASSUME_YES}" ] && return 0
  printf '%s\nContinue? [y/N] ' "$1"
  read -r answer
  case "${answer}" in y|Y|yes|YES) ;; *) echo "stopped."; exit 1 ;; esac
}

case "${CHANNEL}" in
  chat|both) confirm "--channel ${CHANNEL} posts a card per rung to a REAL Google Chat space (GOOGLE_CHAT_SPACES in the service's .env), which colleagues can see." ;;
esac
[ -n "${LIVE}" ] && confirm "--live places REAL Twilio calls. Every rung rings ${TO}. This costs money."

# -- redis --------------------------------------------------------------------

cleanup() {
  if [ -n "${KEEP_REDIS}" ]; then
    echo; echo "redis left running as ${REDIS_NAME} on 127.0.0.1:${REDIS_PORT}"
    return
  fi
  docker rm -f "${REDIS_NAME}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

if ! docker ps --format '{{.Names}}' | grep -qx "${REDIS_NAME}"; then
  docker rm -f "${REDIS_NAME}" >/dev/null 2>&1 || true
  docker run -d --name "${REDIS_NAME}" -p "127.0.0.1:${REDIS_PORT}:6379" redis:7-alpine >/dev/null
  for _ in $(seq 1 20); do
    [ "$(docker exec "${REDIS_NAME}" redis-cli ping 2>/dev/null)" = "PONG" ] && break
    sleep 0.5
  done
fi

# -- the run ------------------------------------------------------------------

args=(--priority "${PRIORITY}" --channel "${CHANNEL}"
      --redis "127.0.0.1:${REDIS_PORT}" --minute "${MINUTE}" --tick "${TICK}"
      --max-calls "${MAX_CALLS}" --team "${ABT}")
if [ -n "${AT}" ]; then
  args+=(--at "${AT}")
  [ -n "${WEEKEND}" ] && args+=(--weekend)
else
  args+=(--shift "${SHIFT}")
fi
[ -n "${LIVE}" ]   && args+=(--live --to "${TO}")
[ -n "${ACK_AT}" ] && args+=(--cancel-at "${ACK_AT}" --cancel-by "${ACK_BY}")

when="${SHIFT:+shift ${SHIFT}}"
[ -n "${AT}" ] && when="at ${AT} IST${WEEKEND:+ (weekend)}"
ack="not acknowledged -- climbs every rung"
[ -n "${ACK_AT}" ] && ack="acknowledged by ${ACK_BY} once ${ACK_AT} is called"
reach="nobody (logged only)"
case "${CHANNEL}" in
  call) reach="${LIVE:+REAL calls to ${TO}}"; reach="${reach:-a local call stub (nothing dialled)}" ;;
  chat) reach="a real Google Chat space" ;;
  both) reach="${LIVE:+REAL calls to ${TO}}"; reach="${reach:-a local call stub} + a real Google Chat space" ;;
esac
speed="real time -- about ${LADDER_LEN} minutes if nobody acknowledges"
[ "${MINUTE}" != "1m" ] && speed="1 ladder minute = ${MINUTE}"

cat <<SUMMARY
------------------------------------------------------------------------------
 severity     ${SEVERITY} (${PRIORITY})
 assigned to  ${ABT:-UNASSIGNED -- no ABT}
 reported     ${when}  (the engine derives the shift)
 reaches      ${reach}
 ends         ${ack}
 clock        ${speed}
------------------------------------------------------------------------------
SUMMARY

cd "${service_dir}"
go run ./cmd/escalation-local "${args[@]}"
