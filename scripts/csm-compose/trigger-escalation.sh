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
# Put one incident.created on the event topic and watch the escalation ladder
# react, against the running local stack.
#
# WHY THIS EXISTS, AND WHAT IT IS NOT
#
# POST /incidents returns 503 on DATA_SOURCE=postgres, so a local stack cannot
# create an incident the way production does. Everything after the publish is
# real and running, though, so this skips only the creation: the topic, the
# consumer group, the decode and the engine are all the live ones.
#
# It publishes using entity-service's OWN copy of the event struct. The two
# services keep separate copies, synchronised by hand, so building the event
# from the consumer's copy would be the consumer talking to itself and could
# never show the two drifting apart. This can, and has.
#
# Usage:
#   -p  priority   P0..P4, or CATASTROPHIC/CRITICAL/HIGH/MEDIUM/MODERATE/LOW
#   -t  team       the assigned ABT, as the incident names it       (default Vega)
#   -a  reported   RFC3339 report time; decides the shift           (default now)
#   -r  product    routes the Chat space; empty uses the default
#   -A  account    the customer on the incident
#   -T  title      the incident's own title
#   -i  id         reuse an incident id, to re-trigger the same one
#   -f  seconds    how long to follow the logs afterwards           (default 25)
#
#   trigger-escalation.sh                                  # P0, now, Vega
#   trigger-escalation.sh -p P2 -t Rigel -A "Globex"
#   trigger-escalation.sh -a 2026-09-28T19:30:00+05:30     # an LK_EVENING one
#   trigger-escalation.sh -a 2026-09-26T23:00:00+05:30     # a USA_WEEKEND one
#
# The report time is the interesting one. It decides the shift, the shift
# decides the rule row, and the row decides who is called: 12:30 IST is LK,
# 19:30 is LK_EVENING, 07:00 is LK_MORNING, 23:00 is USA, and the same hours
# on a Saturday or Sunday are the weekend variants. Passing -a is the only way
# to exercise a rotation that is not the one happening right now.
set -euo pipefail

PRIORITY=P0
TEAM=Vega
PRODUCT=""
REPORTED=""
ACCOUNT=""
TITLE=""
INCIDENT_ID=""
FOLLOW_SECONDS=25

usage() { sed -n '/^# Usage:/,/^set -euo/p' "$0" | sed 's/^# \{0,1\}//;$d'; exit "${1:-0}"; }

while getopts ":p:t:r:a:A:T:i:f:h" opt; do
  case $opt in
    p) PRIORITY=$OPTARG ;;
    t) TEAM=$OPTARG ;;
    r) PRODUCT=$OPTARG ;;
    a) REPORTED=$OPTARG ;;
    A) ACCOUNT=$OPTARG ;;
    T) TITLE=$OPTARG ;;
    i) INCIDENT_ID=$OPTARG ;;
    f) FOLLOW_SECONDS=$OPTARG ;;
    h) usage 0 ;;
    *) usage 1 ;;
  esac
done

cd "$(dirname "$0")/../.."

# The ladder needs Redis, a chat webhook and a roster, none of which the stack
# carries by default. GOOGLE_CHAT_SPACES holds a credential, so it comes from
# the environment or from the service's own git-ignored .env - never from here.
# ESCALATION_ENV_FILE overrides where to look. The default is this checkout's
# own, which a git worktree will not have: .env is git-ignored, so it exists
# only where you first put it. Point this at that copy, or export the variable.
ENV_FILE="${ESCALATION_ENV_FILE:-integrations/csm-notification-service/.env}"
if [[ -z "${GOOGLE_CHAT_SPACES:-}" && -f "$ENV_FILE" ]]; then
  # cut, not `source`: the value is unquoted JSON and sourcing strips the
  # inner quotes, which leaves the consumer unable to parse it and silently
  # unable to post anything.
  GOOGLE_CHAT_SPACES="$(grep '^GOOGLE_CHAT_SPACES=' "$ENV_FILE" | cut -d= -f2- || true)"
  export GOOGLE_CHAT_SPACES
fi
if [[ -z "${GOOGLE_CHAT_SPACES:-}" ]]; then
  echo "warning: GOOGLE_CHAT_SPACES is not set, so the ladder will run and post nothing." >&2
  echo "         export it, or point ESCALATION_ENV_FILE at the .env that has it." >&2
fi

# Stand-in recipients, one per rung, so a run reaches the whole ladder without
# needing the Team Schedule. They carry numbers so the same roster works on a
# call run too; a chat run no longer needs them.
if [[ -z "${INCIDENT_ESCALATION_ROSTER:-}" ]]; then
  INCIDENT_ESCALATION_ROSTER='{"default":{"LEVEL_0":[{"email":"rota.sublead@example.com","name":"Rota Sub Lead","phone":"+10000000000"}],"LEVEL_1":[{"email":"abt.sublead@example.com","name":"ABT Sub Lead","phone":"+10000000001"}],"LEVEL_2":[{"email":"abt.lead@example.com","name":"ABT Lead","phone":"+10000000002"}],"LEVEL_3":[{"email":"cre.head@example.com","name":"CRE Head","phone":"+10000000003"}],"LEVEL_4":[{"email":"cs.head@example.com","name":"CS Head","phone":"+10000000004"}]}}'
fi
export INCIDENT_ESCALATION_ROSTER

echo "==> making sure redis and the notification service are up to date"
docker compose up -d redis >/dev/null
docker compose up -d --build csm-notification-service >/dev/null
# Poll rather than read once. `up -d` returns when the container has been
# STARTED, not when the service inside it is ready, and on a cold start the
# engine's line lands a few seconds later -- so a single read reported "did not
# start" for a service that was starting perfectly well, and the whole run had
# to be repeated. Thirty seconds is well past a warm start and still short
# enough to fail quickly when the engine really is disabled.
for _ in $(seq 1 30); do
  if docker compose logs csm-notification-service --tail 200 2>/dev/null \
       | grep -q 'incident call escalation is enabled'; then
    escalation_up=1
    break
  fi
  sleep 1
done
[[ -n "${escalation_up:-}" ]] \
  || { echo "the escalation engine did not start; check REDIS_ADDR and the roster" >&2; exit 1; }

echo "==> building the publisher (entity-service's own event struct)"
( cd entity-service && GOOS=linux GOARCH="$(docker compose exec -T entity-service uname -m | tr -d '\r' | sed 's/aarch64/arm64/;s/x86_64/amd64/')" \
    go build -o /tmp/publish-incident ./cmd/publish-incident )
docker compose cp /tmp/publish-incident entity-service:/tmp/publish-incident >/dev/null

echo "==> publishing"
ARGS=(-broker kafka:9094 -topic case-events -priority "$PRIORITY" -team "$TEAM")
[[ -n "$PRODUCT" ]] && ARGS+=(-product "$PRODUCT")
[[ -n "$REPORTED" ]] && ARGS+=(-reported-at "$REPORTED")
[[ -n "$ACCOUNT" ]] && ARGS+=(-account "$ACCOUNT")
[[ -n "$TITLE" ]] && ARGS+=(-title "$TITLE")
[[ -n "$INCIDENT_ID" ]] && ARGS+=(-incident-id "$INCIDENT_ID")

PUBLISHED="$(docker compose exec -T entity-service /tmp/publish-incident "${ARGS[@]}")"
echo "$PUBLISHED"
INCIDENT="$(printf '%s' "$PUBLISHED" | sed -n 's/^published incident.created for \([^ ]*\) .*/\1/p')"
[[ -n "$INCIDENT" ]] || { echo "could not tell which incident was published" >&2; exit 1; }

cat <<BANNER

==> following ${INCIDENT} for ${FOLLOW_SECONDS}s

    What to look for, in the order it should happen:

      consumed          the escalation consumer group picked the record up
      rule=Rn           which row of the rule table routed it, from the shift
      scheduled         a plan exists: the rungs and when each opens
      placed / posted   a rung actually reached somebody
      NO_NUMBER         a recipient had no number on a call run, so could
                        not be reached; a chat rung needs none
      cancelled         an acknowledgement stopped it
      exhausted         it climbed every rung and nobody answered

BANNER

# No `timeout` on macOS, and `docker compose logs -f` cannot be asked to stop
# on its own. Poll instead, printing only what is new since the last pass.
END=$(( $(date +%s) + FOLLOW_SECONDS ))
PRINTED=0
while [[ "$(date +%s)" -lt "$END" ]]; do
  MATCHED="$(docker compose logs csm-notification-service --since "$(( FOLLOW_SECONDS + 30 ))s" --no-log-prefix 2>/dev/null \
    | grep -F "$INCIDENT" || true)"
  TOTAL="$(printf '%s' "$MATCHED" | grep -c . || true)"
  if [[ "$TOTAL" -gt "$PRINTED" ]]; then
    printf '%s\n' "$MATCHED" | tail -n "+$(( PRINTED + 1 ))" \
      | sed -E 's/^time=[^ ]* //; s/level=([A-Z]+)/[\1]/; s/msg="?escalation: //; s/"$//' \
      | sed 's/^/    /'
    PRINTED="$TOTAL"
  fi
  sleep 2
done

if [[ "$PRINTED" -eq 0 ]]; then
  cat >&2 <<'NOTHING'

    Nothing mentioned this incident. Usually one of:
      - the engine is not enabled (no REDIS_ADDR, or an unparseable roster)
      - the record went to a different topic than the consumer reads
      - the consumer is still catching up; raise -f and try again
NOTHING
fi

echo
echo "==> the ladder's own state in redis"
docker compose exec -T redis redis-cli --scan --pattern "*${INCIDENT}*" 2>/dev/null | sed 's/^/    /' || true
echo "    (empty once the ladder has finished or been acknowledged)"
