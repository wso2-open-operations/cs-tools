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
# Put one customer case (case.created) on the event topic -- CRE paging starts
# from cases, not incidents -- and watch the escalation ladder
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
#   -f  seconds    how long to follow the logs afterwards           (default 25,
#                  or until the ladder ends on a chat or call run)
#   -c  channel    log | chat | call | both (= call and chat)        (default log)
#                  log    nobody is contacted; each rung is a log line
#                  chat   a card per rung in the REAL space GOOGLE_CHAT_SPACES names
#                  call   real Twilio calls -- needs -m, and rings ONLY that number
#   -m  number     E.164, required with -c call|both. The run's copy of
#                  escalation.yaml makes the CRE and CS heads you, and limits
#                  safety.allowedNumbers to this one number, so no other phone
#                  can ring. Rota members, nominees and team leads have no
#                  numbers at all ("user" has no phone column), so on a call
#                  run LEVEL_0-LEVEL_2 are recorded NO_NUMBER and only the
#                  heads' rungs -- LEVEL_3 and LEVEL_4 -- actually dial.
#   -y             do not ask before posting to chat or placing calls
#
#   trigger-escalation.sh                                  # P0, now, Vega
#   trigger-escalation.sh -p P2 -t Rigel -A "Globex"
#   trigger-escalation.sh -a 2026-09-28T19:30:00+05:30     # an LK_EVENING one
#   trigger-escalation.sh -a 2026-09-26T23:00:00+05:30     # a USA_WEEKEND one
#   trigger-escalation.sh -c chat                          # cards in your space
#   trigger-escalation.sh -c call -m +94770000000          # YOUR phone, at L3/L4
#   trigger-escalation.sh -c both -m +94770000000          # both of the above
#
# A chat or call run mounts its own copy of escalation.yaml for the length of
# the ladder and puts the service back on the committed file -- channel log,
# calls off -- when it ends, including on Ctrl-C. Stopping early stops the
# remaining rungs too, which is the safe direction.
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
FOLLOW_SET=""
CHANNEL=log
CALL_ME=""
ASSUME_YES=""

usage() { sed -n '/^# Usage:/,/^set -euo/p' "$0" | sed 's/^# \{0,1\}//;$d'; exit "${1:-0}"; }

while getopts ":p:t:r:a:A:T:i:f:c:m:yh" opt; do
  case $opt in
    p) PRIORITY=$OPTARG ;;
    t) TEAM=$OPTARG ;;
    r) PRODUCT=$OPTARG ;;
    a) REPORTED=$OPTARG ;;
    A) ACCOUNT=$OPTARG ;;
    T) TITLE=$OPTARG ;;
    i) INCIDENT_ID=$OPTARG ;;
    f) FOLLOW_SECONDS=$OPTARG; FOLLOW_SET=1 ;;
    c) CHANNEL=$(printf '%s' "$OPTARG" | tr '[:upper:]' '[:lower:]') ;;
    m) CALL_ME=$OPTARG ;;
    y) ASSUME_YES=1 ;;
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
case "$CHANNEL" in
  log|chat|call|both) ;;
  *) echo "-c must be log, chat, call or both" >&2; exit 2 ;;
esac
uses_chat=""; uses_call=""
case "$CHANNEL" in chat|both) uses_chat=1 ;; esac
case "$CHANNEL" in call|both) uses_call=1 ;; esac

if [[ -z "${CRE_CHAT_WEBHOOK_URL:-}" && -f "$ENV_FILE" ]]; then
  CRE_CHAT_WEBHOOK_URL="$(grep '^CRE_CHAT_WEBHOOK_URL=' "$ENV_FILE" | head -1 | cut -d= -f2- || true)"
  export CRE_CHAT_WEBHOOK_URL
fi
if [[ -n "$uses_chat" && -z "${CRE_CHAT_WEBHOOK_URL:-}" && -z "${GOOGLE_CHAT_SPACES:-}" ]]; then
  echo "-c $CHANNEL needs CRE_CHAT_WEBHOOK_URL (escalation.yaml chat.webhookUrlEnv) or GOOGLE_CHAT_SPACES: export it, or point ESCALATION_ENV_FILE at the .env that has it." >&2
  exit 1
fi

if [[ -n "$uses_call" ]]; then
  [[ -n "$CALL_ME" ]] || { echo "-c $CHANNEL needs -m YOUR_NUMBER: every call is pinned to it, so nobody else can be rung." >&2; exit 2; }
  [[ "$CALL_ME" =~ ^\+[1-9][0-9]{7,14}$ ]] || { echo "-m must be an E.164 number, e.g. +94770000000" >&2; exit 2; }
  # cut, not source, as above; then strip one layer of surrounding quotes,
  # which an .env written by hand often has and Twilio would reject.
  read_env() { grep "^$1=" "$ENV_FILE" 2>/dev/null | head -1 | cut -d= -f2- | sed -E 's/^"(.*)"$/\1/' || true; }
  for k in TWILIO_ACCOUNT_SID TWILIO_AUTH_TOKEN TWILIO_FROM_NUMBER; do
    [[ -n "${!k:-}" ]] || printf -v "$k" '%s' "$(read_env "$k")"
    [[ -n "${!k:-}" ]] || { echo "-c $CHANNEL needs $k: export it, or point ESCALATION_ENV_FILE at the .env that has it." >&2; exit 1; }
    export "$k"
  done
  export CALL_SENDING_ENABLED=true
else
  [[ -z "$CALL_ME" ]] || { echo "-m only applies with -c call or both" >&2; exit 2; }
  export CALL_SENDING_ENABLED=false
fi

# A chat or call run mounts its own copy of escalation.yaml: the committed file
# says channel log, and the file wins over INCIDENT_ESCALATION_CHANNEL, so no
# environment variable could change it. The copy changes the CRE channel and,
# on a call run, makes both heads you and limits allowedNumbers to your number.
RUN_CONFIG=""
if [[ "$CHANNEL" != log ]]; then
  mkdir -p scripts/csm-compose/.run
  RUN_CONFIG="$PWD/scripts/csm-compose/.run/escalation.yaml"
  awk -v ch="$CHANNEL" -v me="$CALL_ME" '
    /^cre:/ { in_cre = 1 }
    /^sre:/ { in_cre = 0 }
    in_cre && !ch_done && /^  channel:/ { print "  channel: " ch; ch_done = 1; next }
    in_cre && me != "" && /^    cre: \{name:/ { print "    cre: {name: \"Escalation test (you)\", email: \"escalation-test@example.invalid\", phone: \"" me "\"}"; next }
    in_cre && me != "" && /^    cs: +\{name:/ { print "    cs:  {name: \"Escalation test (you)\", email: \"escalation-test@example.invalid\", phone: \"" me "\"}"; next }
    in_cre && me != "" && /^    allowedNumbers:/ { print "    allowedNumbers: [\"" me "\"]"; next }
    { print }
  ' scripts/csm-compose/escalation.yaml > "$RUN_CONFIG"

  # Fail closed. If the committed file has drifted and a substitution did not
  # land, a call run could reach somebody other than you -- so refuse to start
  # rather than run with a copy nobody has checked.
  awk '/^cre:/{c=1} /^sre:/{c=0} c' "$RUN_CONFIG" | grep -q "^  channel: $CHANNEL\$" \
    || { echo "could not set the CRE channel in the run's config; refusing to start" >&2; exit 1; }
  if [[ -n "$uses_call" ]]; then
    pinned="$(awk '/^cre:/{c=1} /^sre:/{c=0} c' "$RUN_CONFIG" | grep -cF "\"$CALL_ME\"" || true)"
    [[ "$pinned" -eq 3 ]] \
      || { echo "could not pin both heads and allowedNumbers to $CALL_ME (found $pinned of 3); refusing to place calls" >&2; exit 1; }
  fi
  export ESCALATION_CONFIG_FILE="$RUN_CONFIG"

  confirm() {
    [[ -n "$ASSUME_YES" ]] && return 0
    printf '%s\nContinue? [y/N] ' "$1"
    read -r answer
    [[ "$answer" =~ ^[yY] ]] || { echo "stopped."; exit 1; }
  }
  [[ -n "$uses_chat" ]] && confirm "-c $CHANNEL posts a card per rung to the REAL Google Chat space in GOOGLE_CHAT_SPACES."
  [[ -n "$uses_call" ]] && confirm "-c $CHANNEL places REAL Twilio calls to $CALL_ME at LEVEL_3 and LEVEL_4. This costs money."

  restore() {
    echo
    echo "==> putting the notification service back on the committed config (channel log, calls off)"
    env -u ESCALATION_CONFIG_FILE -u TWILIO_ACCOUNT_SID -u TWILIO_AUTH_TOKEN -u TWILIO_FROM_NUMBER \
        CALL_SENDING_ENABLED=false docker compose up -d csm-notification-service >/dev/null 2>&1 \
      || echo "    could not recreate it -- run: docker compose up -d csm-notification-service" >&2
    rm -f "$RUN_CONFIG"
  }
  trap restore EXIT

  # Follow the whole ladder unless told otherwise: the rungs that reach people
  # are minutes in, and leaving early would put the service back on log first.
  if [[ -z "$FOLLOW_SET" ]]; then
    case "$(printf '%s' "$PRIORITY" | tr '[:lower:]' '[:upper:]')" in
      P0|S0|CATASTROPHIC)    FOLLOW_SECONDS=$(( 18 * 60 )) ;;
      P1|S1|CRITICAL)        FOLLOW_SECONDS=$(( 50 * 60 )) ;;
      P2|S2|HIGH)            FOLLOW_SECONDS=$(( 77 * 60 )) ;;
      P3|S3|MEDIUM|MODERATE) FOLLOW_SECONDS=$(( 117 * 60 )) ;;
      *)                     FOLLOW_SECONDS=$(( 145 * 60 )) ;;
    esac
  fi
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

# A ladder lives in Redis for its whole length, and each call goes out on
# whatever channel the service has WHEN IT COMES DUE -- so a test ladder still
# climbing from an earlier log run would start posting or dialling the moment
# this run switches the channel. Retire earlier test ladders first, and only
# those: local-inc-* is the id this script's publisher mints, so nothing else
# in Redis is touched.
if [[ "$CHANNEL" != log ]]; then
  retired="$(docker compose exec -T redis sh -c '
    n=0
    for k in $(redis-cli --scan --pattern "incident:escalation:state:local-inc-*"); do
      redis-cli del "$k" >/dev/null; n=$((n+1))
    done
    for m in $(redis-cli zrange incident:escalation:wake 0 -1 | grep "^local-inc-"); do
      redis-cli zrem incident:escalation:wake "$m" >/dev/null
    done
    echo $n' 2>/dev/null | tr -d '\r' || echo 0)"
  [[ "${retired:-0}" -gt 0 ]] && echo "    retired ${retired} earlier test ladder(s), so only this run can reach anyone"
fi
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

loaded="$(docker compose logs csm-notification-service --tail 300 2>/dev/null \
  | grep -oE 'creChannel=[a-z]+' | tail -1 | cut -d= -f2)"
[[ "$loaded" == "$CHANNEL" ]] \
  || { echo "the service loaded CRE channel '${loaded:-?}', not '$CHANNEL'; refusing to publish" >&2; exit 1; }
echo "    CRE channel: $CHANNEL${CALL_ME:+  (calls pinned to $CALL_ME)}"

echo "==> building the publisher (entity-service's own event struct)"
( cd entity-service && GOOS=linux GOARCH="$(docker compose exec -T entity-service uname -m | tr -d '\r' | sed 's/aarch64/arm64/;s/x86_64/amd64/')" \
    go build -o /tmp/publish-incident ./internal/tools/publishincident/main.go )
docker compose cp /tmp/publish-incident entity-service:/tmp/publish-incident >/dev/null

echo "==> publishing"
ARGS=(-broker kafka:9094 -topic case-events -record case -priority "$PRIORITY" -team "$TEAM")
[[ -n "$PRODUCT" ]] && ARGS+=(-product "$PRODUCT")
[[ -n "$REPORTED" ]] && ARGS+=(-reported-at "$REPORTED")
[[ -n "$ACCOUNT" ]] && ARGS+=(-account "$ACCOUNT")
[[ -n "$TITLE" ]] && ARGS+=(-title "$TITLE")
[[ -n "$INCIDENT_ID" ]] && ARGS+=(-incident-id "$INCIDENT_ID")

PUBLISHED="$(docker compose exec -T entity-service /tmp/publish-incident "${ARGS[@]}")"
echo "$PUBLISHED"
INCIDENT="$(printf '%s' "$PUBLISHED" | sed -n 's/^published case.created for \([^ ]*\) .*/\1/p')"
[[ -n "$INCIDENT" ]] || { echo "could not tell which case was published" >&2; exit 1; }

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
  printf '%s' "$MATCHED" | grep -qE 'ladder (exhausted|cancelled)' && break
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
