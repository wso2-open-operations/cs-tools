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
# Exercise the CRE call-escalation ladder end to end, contacting nobody.
#
# It runs the REAL engine -- the same code cmd/server runs -- against a real
# Redis, on the `log` channel. Event decoding, shift derivation, rule matching,
# the priority clock, the durable state, the wake loop, cancellation and the
# work note are all genuinely exercised. Nothing is dialled and nothing is
# posted to a chat space, so this needs no Twilio account and interrupts
# nobody.
#
# What it does NOT exercise: who each rung resolves to. Recipients come from a
# local roster whose names ARE the rung names, so you see which rung fires
# rather than which person. Resolving real people needs entity-service
# reachable and CUSTOMER_ENTITY_BASE_URL set.
#
# Usage:
#   ./scripts/csm-compose/test-cre-ladder.sh p0           # one P0 ladder
#   ./scripts/csm-compose/test-cre-ladder.sh              # every scenario
#   ./scripts/csm-compose/test-cre-ladder.sh timings      # just one
#   KEEP_REDIS=1 ./scripts/csm-compose/test-cre-ladder.sh # leave Redis running
#   MINUTE=1s ./scripts/csm-compose/test-cre-ladder.sh p0 # slower clock
#
# Scenarios: p0 | timings | ack | half-ack | shifts | not-abt | realtime | all
#
# realtime is excluded from "all": it runs for about sixteen minutes.

set -euo pipefail

REDIS_NAME="${REDIS_NAME:-cre-ladder-redis}"
REDIS_PORT="${REDIS_PORT:-16379}"
REDIS_ADDR="127.0.0.1:${REDIS_PORT}"

# One ladder minute in wall-clock time. 150ms turns a 44-minute P1 ladder into
# about seven seconds; unset it to watch the real thing.
MINUTE="${MINUTE:-150ms}"
TICK="${TICK:-60ms}"

# Which ABT the incident is assigned to. An ABT key routes an ABT rule (R2 on
# LK, whose LEVEL_0 is that team's own T1/T2/T3 nominees); anything else routes
# the not-an-ABT row. Only consulted in USE_TEAM_SCHEDULE mode -- the local
# roster has one fixed recipient per rung and no notion of a team.
TEAM="${TEAM:-vega}"

# R2 at P0 plans 33 calls, well past escalation-local's own default cap of 20,
# so the realistic scenario refused to run at all. The cap is a guard against a
# runaway plan, not a statement about this one.
MAX_CALLS="${MAX_CALLS:-60}"

# Empty means "use the local roster". Set USE_TEAM_SCHEDULE=1 to resolve real
# people from entity-service instead.
#
# That path needs a token whose client_id entity-service lists in
# AUTH_INTERNAL_CLIENT_IDS -- it reads the caller's identity from
# x-jwt-assertion, which only Choreo's gateway sets in a real deployment. The
# service's own .env carries the real Asgardeo client id, which the local
# container does NOT trust, so every rung came back RESOLVE_FAILED and the
# cause was invisible. Default to the dev client the compose stack's mock-oidc
# mints and entity-service accepts; override any of these to point elsewhere.
ENTITY_URL=""
if [ -n "${USE_TEAM_SCHEDULE:-}" ]; then
  ENTITY_URL="${CUSTOMER_ENTITY_BASE_URL:-http://localhost:8081}"
  export OAUTH2_CLIENT_ID="${OAUTH2_CLIENT_ID:-csm-notification-service-dev-client}"
  export OAUTH2_CLIENT_SECRET="${OAUTH2_CLIENT_SECRET:-dev-secret}"
  export OAUTH2_TOKEN_URL="${OAUTH2_TOKEN_URL:-http://localhost:9100/oauth2/token}"
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
service_dir="${repo_root}/integrations/csm-notification-service"

run() {
  # --channel log is what makes this safe: the ladder runs in full and reaches
  # nobody. Every other flag is about which ladder to run.
  #
  # CUSTOMER_ENTITY_BASE_URL is blanked unless USE_TEAM_SCHEDULE is set. The
  # service's own .env points it at a local entity-service, and the harness
  # would then resolve rungs from the real Team Schedule -- which cannot
  # authenticate yet, so every rung comes back RESOLVE_FAILED and no ladder is
  # scheduled at all. Blank it and the local roster answers instead, which is
  # what a dry run wants: you see which rung fires, not which person.
  local team_args=()
  [ -n "${USE_TEAM_SCHEDULE:-}" ] && team_args=(--team "${TEAM}")

  # RUN_STDERR sends stderr somewhere specific -- the half-ack scenario wants
  # it inline, because the line it checks for is an engine log.
  if [ -n "${RUN_STDERR:-}" ]; then
    (cd "${service_dir}" && CUSTOMER_ENTITY_BASE_URL="${ENTITY_URL}" \
        go run ./cmd/escalation-local \
        --channel log --redis "${REDIS_ADDR}" --max-calls "${MAX_CALLS}" \
        "${team_args[@]+"${team_args[@]}"}" \
        --minute "${MINUTE}" --tick "${TICK}" "$@" 2>"${RUN_STDERR}")
    return
  fi

  # Otherwise capture stderr rather than discard it, and show it only if the
  # run fails.
  #
  # It cannot simply be left on the terminal: escalation-local writes the
  # engine's own INFO logs there, and a few hundred slog lines bury the
  # formatted ladder each scenario exists to show. It must not be thrown away
  # either -- a compile error or a failed rung then produced an empty scenario
  # and no reason, which is the review finding this answers. Quiet when it
  # works, complete when it does not.
  # `|| status=$?` rather than a bare call then `$?`: under `set -e` a failing
  # subshell aborts the script at that line, so the status check never ran and
  # the diagnostics were still never printed. Making it a compound command is
  # what lets the failure be handled here instead of ending the run.
  local err status=0
  err="$(mktemp)"
  (cd "${service_dir}" && CUSTOMER_ENTITY_BASE_URL="${ENTITY_URL}" \
      go run ./cmd/escalation-local \
      --channel log --redis "${REDIS_ADDR}" --max-calls "${MAX_CALLS}" \
      "${team_args[@]+"${team_args[@]}"}" \
      --minute "${MINUTE}" --tick "${TICK}" "$@" 2>"${err}") || status=$?
  if [ "${status}" -ne 0 ]; then
    echo >&2
    echo "  the run failed (exit ${status}); its diagnostics follow:" >&2
    sed 's/^/    /' "${err}" >&2
  fi
  rm -f "${err}"
  return "${status}"
}

heading() {
  printf '\n\033[1m%s\033[0m\n' "$1"
  printf '%s\n' "$(printf '%.0s-' $(seq 1 ${#1}))"
}

start_redis() {
  if docker ps --format '{{.Names}}' | grep -qx "${REDIS_NAME}"; then
    echo "redis: already running as ${REDIS_NAME}"
    return
  fi
  docker rm -f "${REDIS_NAME}" >/dev/null 2>&1 || true
  docker run -d --name "${REDIS_NAME}" -p "127.0.0.1:${REDIS_PORT}:6379" \
    redis:7-alpine >/dev/null
  # The ladder's whole state lives here, so fail early rather than half way
  # through a scenario.
  for _ in $(seq 1 20); do
    if [ "$(docker exec "${REDIS_NAME}" redis-cli ping 2>/dev/null)" = "PONG" ]; then
      echo "redis: up on ${REDIS_ADDR}"
      return
    fi
    sleep 0.5
  done
  echo "redis did not come up on ${REDIS_ADDR}" >&2
  exit 1
}

cleanup() {
  [ -n "${KEEP_REDIS:-}" ] && { echo; echo "redis left running as ${REDIS_NAME}"; return; }
  docker rm -f "${REDIS_NAME}" >/dev/null 2>&1 || true
  echo
  echo "redis removed"
}

scenario_p0() {
  heading "A P0 incident, raised during business hours"
  echo "The whole ladder, start to finish. Nothing is dialled."
  if [ -n "${USE_TEAM_SCHEDULE:-}" ]; then
    echo "resolver: the real Team Schedule, incident assigned to ${TEAM}"
    echo "          LEVEL_0 is that ABT's own T1/T2/T3 nominees"
  else
    echo "resolver: the local stand-in roster -- one fixed recipient per rung."
    echo "          It has no rule table, so this shows the CLOCK and the rung"
    echo "          order, not which row of R1a-R6 an incident routes by."
    echo "          Re-run with USE_TEAM_SCHEDULE=1 for real people and rules."
  fi
  run --priority P0 --shift LK
}

scenario_realtime() {
  heading "A P0 at REAL timing -- about 16 minutes"
  cat <<'NOTE'
This is the one thing a compressed run cannot prove. The compressed clock
scales playback speed, not the scheduled offsets, so the plan's "+8m0s" is
already the real figure and a wrong interval shows up there immediately. What
only a real run exercises is the wall-clock plumbing underneath: the engine's
own ticker, and the Redis wake-index scores a due call is found by.

It will sit here for about sixteen minutes. Rungs should appear at +0, +1, +4,
+8 and +12 minutes of actual wall clock. Ctrl-C is safe; the ladder is retired
on exit.
NOTE
  MINUTE=1m TICK=5s run --priority P0 --shift LK
}

scenario_timings() {
  heading "1. The clock, per priority"
  cat <<'NOTE'
The rung order is the thing to check:
  first responders -> the team's own lead -> three team leads -> CRE head -> CS head
If LEVEL_1 and LEVEL_2 look swapped, it is routing by the previous model.

Expected openings, from the specification:
  S0   0  +1  +4  +8  +12
  S1  +6  +9 +18 +28  +38
NOTE
  for p in P0 P1; do
    printf '\n  --- %s ---\n' "${p}"
    run --priority "${p}" --shift LK | sed -n '/the ladder the engine scheduled/,/^$/p'
  done
}

scenario_ack() {
  heading "2. Acknowledgement stops it"
  cat <<'NOTE'
Acknowledgement is BOTH gestures: a move out of NEW and a public comment.
This sends both, so the outstanding calls are cancelled.

It acknowledges once LEVEL_1 has been called, rather than at a wall-clock
offset: a compressed ladder minute makes an offset unreliable, because this
tool's own startup (a go build, and a token fetch in USE_TEAM_SCHEDULE mode)
can cost more ladder time than the rung it was aiming at. -cancel-at names
the rung instead. Use ACK_AT to pick another one.
NOTE
  run --priority P0 --shift LK --cancel-at "${ACK_AT:-LEVEL_1}" | tail -8
}

scenario_half_ack() {
  heading "3. One gesture alone does NOT stop it"
  cat <<'NOTE'
Acknowledgement is a move out of NEW AND a public comment. A status change on
its own is what a dispatcher does while triaging a queue, so the ladder must
keep climbing. Watch for "half acknowledged" in the log and calls continuing.
NOTE
  # Through run(), not a hand-rolled `go run`. This scenario used to build its
  # own command so it could capture stderr, and in doing so it dropped run()'s
  # CUSTOMER_ENTITY_BASE_URL handling -- so it alone resolved rungs from the
  # real Team Schedule, every rung came back RESOLVE_FAILED, escalation-local
  # exited 1, and `set -e` aborted the whole suite here. Scenarios 4 and 5
  # never ran, which is the sort of thing a harness must not do quietly.
  local out
  out="$(RUN_STDERR=/dev/stdout run --priority P0 --shift LK --cancel-after 1s --cancel-by status)"

  printf '\n  the engine says:\n'
  echo "${out}" | grep -i 'half acknowledged' | sed 's/^/    /' || true

  printf '\n  and the rungs after it:\n'
  echo "${out}" | grep -oE 'LEVEL_[0-4]' | sort -u | tr '\n' ' ' | sed 's/^/    /'
  printf '\n'

  cat <<'NOTE'

  PASS when "half acknowledged ... stillNeeds a public comment" appears and
  rungs above the one it had reached still fire.

  Ignore the "Acknowledged : N call(s) cancelled" line at the very end -- that
  is this harness retiring its own ladder on exit so a later run does not
  resume it, not the engine accepting one gesture.
NOTE
}

scenario_shifts() {
  heading "4. Each shift routes by its own rule"
  echo "LK_MORNING -> R1a, LK_EVENING -> R4a, USA -> R5, USA_WEEKEND -> R6."
  for s in LK_MORNING LK_EVENING USA USA_WEEKEND; do
    printf '\n  --- %s ---\n' "${s}"
    run --priority P1 --shift "${s}" | sed -n '/the ladder the engine scheduled/,/^$/p' | head -8
  done
}

scenario_not_abt() {
  heading "5. An incident on no ABT team"
  echo "Routes R3 rather than R2: LEVEL_0 becomes one nominee from each ABT team."
  run --priority P1 --shift LK --not-abt | sed -n '/the ladder the engine scheduled/,/^$/p' | head -8
}

main() {
  command -v docker >/dev/null || { echo "docker is required" >&2; exit 1; }
  command -v go >/dev/null || { echo "go is required" >&2; exit 1; }

  start_redis
  trap cleanup EXIT

  case "${1:-all}" in
    p0)       scenario_p0 ;;
    timings)  scenario_timings ;;
    ack)      scenario_ack ;;
    half-ack) scenario_half_ack ;;
    shifts)   scenario_shifts ;;
    not-abt)  scenario_not_abt ;;
    realtime) scenario_realtime ;;
    all)
      scenario_timings
      scenario_ack
      scenario_half_ack
      scenario_shifts
      scenario_not_abt
      ;;
    *)
      echo "unknown scenario: $1" >&2
      echo "use one of: p0 timings ack half-ack shifts not-abt all" >&2
      exit 2
      ;;
  esac
}

main "$@"
