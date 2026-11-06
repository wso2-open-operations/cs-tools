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
# End-to-end run of the deferred Salesforce membership write against the
# local compose stack plus scripts/csm-compose/sf-deferred-e2e.compose.yml
# (real entity-service, real sales-entity-service, fake Salesforce):
#
#   1. switch the mock sales-entity-service into an outage
#   2. invite a brand-new address -> 201 with salesforceSync PENDING, rows
#      committed with NULL sf_id, a PENDING salesforce_write_request
#   3. a role change while pending -> 409 salesforce_sync_pending
#   4. end the outage -> the real sales-entity-service consumer writes the
#      fake Salesforce and answers on salesforce-write-results; entity-service
#      stamps the ids, marks the request SUCCEEDED, publishes
#      project_contact.invited
#   5. no duplicate Salesforce records; the contact list shows no sync flag
#
# Usage: scripts/csm-compose/sf-deferred-e2e.sh up | run | run-exhaust | down
#   OUTAGE_SECONDS=60 run   keeps Salesforce down long enough for the main
#                           tier to give up and dead-letter (each failed
#                           attempt takes ~21s: sales-entity-service retries
#                           its own Salesforce query 3x with backoff first)
#   run-exhaust             keeps Salesforce down until the consumer gives up
#                           and answers failed: the request must end FAILED,
#                           never stay PENDING
set -euo pipefail
cd "$(dirname "$0")/../.."
COMPOSE=(docker compose -f docker-compose.yml -f scripts/csm-compose/sf-deferred-e2e.compose.yml)
PSQL=(docker compose exec -T postgres psql -U postgres -d csm_platform -v ON_ERROR_STOP=1 -tA)
ENTITY=${ENTITY_URL:-http://localhost:8081}
MOCK=${MOCK_SF_URL:-http://localhost:9300}   # the fake Salesforce behind the real sales-entity-service
OIDC=${MOCK_OIDC_URL:-http://localhost:9100}
PROJECT_ID=${PROJECT_ID:-00000000-0000-0000-0000-000000000401} # seeded "Example Corp Production", sf_id SF-PROJ-0001

say()  { printf '\n\033[1m== %s\033[0m\n' "$*"; }
ok()   { printf '   \033[32mOK\033[0m  %s\n' "$*"; }
fail() { printf '   \033[31mFAIL\033[0m %s\n' "$*"; exit 1; }
sql()  { "${PSQL[@]}" -c "$1" </dev/null | tr -d '\r'; }
# Whatever happens, leave the fake Salesforce up and remove the work files: a
# run that stops midway must not leave the next one (or the dead-letter
# consumer) facing an outage, nor temp files at fixed paths.
WORK="$(mktemp -d "${TMPDIR:-/tmp}/sf-deferred-e2e.XXXXXX")"
cleanup() {
  curl -fsS -X POST "$MOCK/__mock/outage" -H "Content-Type: application/json" -d '{"down":false}' >/dev/null 2>&1 || true
  rm -rf "$WORK"
}
trap cleanup EXIT

# drain waits until no request from an earlier run is still PENDING, so the
# consumers' sequential partitions are empty when this run's record arrives.
# With the real sales-entity-service each failed attempt can take ~20s (its
# own Salesforce retry policy), so a leftover backlog takes minutes.
drain() {
  for i in $(seq 1 100); do
    n="$(sql "SELECT count(*) FROM salesforce_write_request WHERE status='PENDING'")"
    [[ "$n" == "0" ]] && return 0
    [[ $i -eq 1 ]] && printf '   draining %s PENDING request(s) left by earlier runs' "$n" || printf '.'
    sleep 3
  done
  echo; fail "earlier requests are still PENDING; is sales-entity-service consuming?"
}

# An M2M caller: entity-service reads x-jwt-assertion's client_id (not
# signature-verified) and treats a client in M2M_CLIENT_IDS as internal.
token() {
  curl -fsS -X POST "$OIDC/oauth2/token" \
    -d grant_type=client_credentials -d client_id=csm-notification-service-dev-client -d client_secret=dev \
    | python3 -c 'import json,sys; print(json.load(sys.stdin)["access_token"])'
}
outage() { curl -fsS -X POST "$MOCK/__mock/outage" -H 'Content-Type: application/json' -d "{\"down\":$1}" >/dev/null; }

# prepare seeds what the local stack lacks and both scenarios need: the
# account-level roles the membership write grants, and Salesforce-shaped ids
# on the seeded project and account (the real sales-entity-service validates
# every id it puts into SOQL; the seed's SF-PROJ-0001 / SF-0001 are not ids).
prepare() {
  say "0. seed the account-level roles the membership write grants (local stack has none)"
  "${PSQL[@]}" -c "INSERT INTO role (id, created_on, updated_on, created_by, updated_by, name, description)
    SELECT gen_random_uuid(), now(), now(), 'sf-deferred-e2e', 'sf-deferred-e2e', r, 'seeded by sf-deferred-e2e.sh'
    FROM unnest(ARRAY['external','customer','partner','customer_admin','partner_admin']) AS r
    ON CONFLICT (name) DO NOTHING" </dev/null >/dev/null
  ok "roles present: $(sql "SELECT string_agg(name, ', ' ORDER BY name) FROM role WHERE name IN ('external','customer','partner','customer_admin','partner_admin')")"
  # The seed's Salesforce ids (SF-PROJ-0001, SF-0001) are not Salesforce id
  # formats; the real sales-entity-service validates every id it puts into
  # SOQL. Give the seeded project and account ids shaped like Salesforce's.
  "${PSQL[@]}" -c "UPDATE project SET sf_id='a0p000000000401AAA' WHERE id='$PROJECT_ID' AND sf_id NOT SIMILAR TO '[a-zA-Z0-9]{15}|[a-zA-Z0-9]{18}';
                   UPDATE account SET sf_id='001000000000301AAA' WHERE id=(SELECT account_id FROM project WHERE id='$PROJECT_ID') AND sf_id NOT SIMILAR TO '[a-zA-Z0-9]{15}|[a-zA-Z0-9]{18}'" </dev/null >/dev/null
  ok "project / account Salesforce ids: $(sql "SELECT p.sf_id||' / '||a.sf_id FROM project p JOIN account a ON a.id=p.account_id WHERE p.id='$PROJECT_ID'")"

}

case "${1:-run}" in
  up)
    "${COMPOSE[@]}" up -d --build entity-service sales-entity-service mock-salesforce kafka-topics-init
    ;;
  down)
    "${COMPOSE[@]}" stop entity-service sales-entity-service mock-salesforce sales-mysql
    ;;
  run)
    TOKEN="$(token)"
    EMAIL="deferred.e2e.$(date +%s)@example.com"
    say "entity-service wired for deferred writes?"
    # Captured first: with pipefail, grep -q closing the pipe early would
    # turn a found line into a failed pipeline.
    STARTUP="$(docker compose logs entity-service 2>/dev/null | grep -o 'salesforce deferred writes enabled.*' | tail -1 || true)"
    [[ -n "$STARTUP" ]] || fail "entity-service did not log 'salesforce deferred writes enabled'"
    ok "consumers started (${STARTUP:0:140}...)"

    prepare
    outage false; drain; echo
    say "1. Salesforce goes down"
    outage true; ok "fake Salesforce answers 503 to sales-entity-service (and so to entity-service)"

    say "2. invite $EMAIL while it is down"
    RESP="$(curl -sS -o "$WORK/invite.json" -w '%{http_code}' -X POST "$ENTITY/projects/$PROJECT_ID/contacts" \
      -H "x-jwt-assertion: $TOKEN" -H 'Content-Type: application/json' \
      -d "{\"email\":\"$EMAIL\",\"firstName\":\"Deferred\",\"lastName\":\"Invitee\",\"roles\":[\"Portal user\"]}")"
    [[ "$RESP" == "201" ]] || { cat "$WORK/invite.json"; fail "invite returned $RESP, want 201"; }
    grep -q '"salesforceSync":"PENDING"' "$WORK/invite.json" || { cat "$WORK/invite.json"; fail "response lacks salesforceSync PENDING"; }
    ok "201 with salesforceSync PENDING: $(cat "$WORK/invite.json")"
    PC_ID="$(sql "SELECT id FROM project_contact WHERE project_id='$PROJECT_ID' AND LOWER(email)='$EMAIL'")"
    [[ -n "$PC_ID" ]] || fail "no project_contact row"
    [[ "$(sql "SELECT COALESCE(sf_id,'<null>') FROM project_contact WHERE id='$PC_ID'")" == "<null>" ]] || fail "project_contact.sf_id should be NULL"
    [[ "$(sql "SELECT COALESCE(ac.sf_id,'<null>') FROM project_contact pc JOIN account_contact ac ON ac.id=pc.account_contact_id WHERE pc.id='$PC_ID'")" == "<null>" ]] || fail "account_contact.sf_id should be NULL"
    ok "project_contact $PC_ID committed with NULL sf_id (state $(sql "SELECT state FROM project_contact WHERE id='$PC_ID'"))"
    REQ="$(sql "SELECT id||' '||status||' '||COALESCE(published_on::text,'unpublished') FROM salesforce_write_request WHERE project_contact_id='$PC_ID'")"
    [[ "$REQ" == *" PENDING "* ]] || fail "salesforce_write_request: '$REQ'"
    [[ "$REQ" != *unpublished* ]] || fail "request was not published to Event Hub"
    ok "salesforce_write_request $REQ"
    [[ "$(sql "SELECT count(*) FROM onboarding_step WHERE email='$EMAIL'")" == "0" ]] || fail "an onboarding step was recorded without a membership id"
    ok "no onboarding_step row yet (ledger needs the Salesforce id)"
    LIST="$(curl -fsS -X POST "$ENTITY/projects/$PROJECT_ID/contacts/search" -H "x-jwt-assertion: $TOKEN" -H 'Content-Type: application/json' \
      -d "{\"filters\":{\"searchQuery\":\"$EMAIL\"},\"pagination\":{\"limit\":5,\"offset\":0}}")"
    [[ "$LIST" == *'"salesforceSync":"PENDING"'* ]] || { echo "$LIST"; fail "contact search does not show salesforceSync PENDING"; }
    ok "contact search shows salesforceSync PENDING"

    say "3. a role change while pending is refused"
    RESP="$(curl -sS -o "$WORK/patch.json" -w '%{http_code}' -X PATCH "$ENTITY/projects/$PROJECT_ID/contacts/$EMAIL" \
      -H "x-jwt-assertion: $TOKEN" -H 'Content-Type: application/json' -d '{"roles":["Admin"]}')"
    [[ "$RESP" == "409" ]] && grep -q salesforce_sync_pending "$WORK/patch.json" || { cat "$WORK/patch.json"; fail "PATCH returned $RESP"; }
    ok "409 $(cat "$WORK/patch.json")"
    TRIES="$("${COMPOSE[@]}" logs --since 3m sales-entity-service 2>/dev/null | grep -c "Salesforce unavailable; will retry" || true)"
    ok "sales-entity-service consumer has made ${TRIES:-0} failed attempt(s) so far against the down Salesforce"

    say "4. Salesforce comes back${OUTAGE_SECONDS:+ after ${OUTAGE_SECONDS}s}"
    if [[ "${OUTAGE_SECONDS:-0}" -gt 0 ]]; then
      sleep "$OUTAGE_SECONDS"
      REQ_ID="$(sql "SELECT id FROM salesforce_write_request WHERE project_contact_id='$PC_ID'")"
      # Captured, not grep -q'd: see the startup check above (pipefail).
      RETRIES="$("${COMPOSE[@]}" logs --since 5m sales-entity-service 2>/dev/null | grep "\"requestId\":\"$REQ_ID\"" | grep -c "Salesforce unavailable; will retry" || true)"
      DEADLETTERED="$("${COMPOSE[@]}" logs --since 5m sales-entity-service 2>/dev/null | grep -c "Main-tier attempts exhausted; dead-lettering" || true)"
      ONDLQ="$(docker compose exec -T kafka /opt/kafka/bin/kafka-console-consumer.sh --bootstrap-server kafka:9092 --topic salesforce-writes-dlq --from-beginning --timeout-ms 6000 2>/dev/null | grep -c "$REQ_ID" || true)"
      [[ "${RETRIES:-0}" -ge 1 ]] || fail "sales-entity-service made no failed attempt for $REQ_ID in ${OUTAGE_SECONDS}s"
      [[ "${ONDLQ:-0}" -ge 1 ]] || fail "sales-entity-service did not dead-letter $REQ_ID to salesforce-writes-dlq in ${OUTAGE_SECONDS}s"
      [[ "$(sql "SELECT status FROM salesforce_write_request WHERE id='$REQ_ID'")" == "PENDING" ]] || fail "request must still be PENDING while Salesforce is down"
      ok "sales-entity-service made $RETRIES failed attempt(s) and dead-lettered the record to salesforce-writes-dlq ($DEADLETTERED dead-letter publish(es) logged); request still PENDING"
    fi
    outage false
    for i in $(seq 1 120); do
      STATUS="$(sql "SELECT status FROM salesforce_write_request WHERE project_contact_id='$PC_ID'")"
      [[ "$STATUS" == "SUCCEEDED" ]] && break
      [[ "$STATUS" == "FAILED" ]] && fail "request FAILED: $(sql "SELECT last_error FROM salesforce_write_request WHERE project_contact_id='$PC_ID'")"
      sleep 3
    done
    [[ "$STATUS" == "SUCCEEDED" ]] || fail "request still $STATUS after 6 minutes"
    RESULTS="$(docker compose exec -T kafka /opt/kafka/bin/kafka-console-consumer.sh --bootstrap-server kafka:9092 --topic salesforce-write-results --from-beginning --timeout-ms 6000 2>/dev/null || true)"
    REQ_ID="$(sql "SELECT id FROM salesforce_write_request WHERE project_contact_id='$PC_ID'")"
    [[ "$RESULTS" == *"membership_write.completed"* && "$RESULTS" == *"$REQ_ID"* ]] || fail "no completed result for $REQ_ID on salesforce-write-results"
    ok "sales-entity-service answered membership_write.completed on salesforce-write-results; entity-service marked the request SUCCEEDED"
    SF="$(sql "SELECT pc.sf_id||' / '||ac.sf_id||' / '||COALESCE(u.sf_id,'<null>') FROM project_contact pc JOIN account_contact ac ON ac.id=pc.account_contact_id LEFT JOIN \"user\" u ON LOWER(u.user_name)=LOWER(ac.user_name) WHERE pc.id='$PC_ID'")"
    [[ "$SF" == a0eM* ]] || fail "sf ids not stamped: $SF"
    ok "sf_id stamped on project_contact / account_contact / user: $SF"
    [[ "$(sql "SELECT status FROM onboarding_step WHERE email='$EMAIL' AND step='DATABASE'")" == "SUCCEEDED" ]] || fail "DATABASE onboarding step missing"
    ok "DATABASE onboarding step recorded under the Salesforce id"

    say "5. exactly one Salesforce contact and one membership; invitation published"
    STATE="$(curl -fsS "$MOCK/__mock/state")"
    C="$(echo "$STATE" | python3 -c "
import json,sys
s=json.load(sys.stdin)
contacts=[c for c in s['contacts'].values() if (c.get('Email') or '').lower()=='$EMAIL']
ids={c['Id'] for c in contacts}
memberships=sum(1 for m in s['memberships'].values() if m.get('Contact__c') in ids)
print(len(contacts), memberships)")"
    [[ "$C" == "1 1" ]] || fail "fake Salesforce holds contacts/memberships = $C for $EMAIL (want 1 1)"
    ok "fake Salesforce holds 1 Contact for $EMAIL and 1 Project_Contact__c"
    EVENTS="$(docker compose exec -T kafka /opt/kafka/bin/kafka-console-consumer.sh --bootstrap-server kafka:9092 --topic project-events --from-beginning --timeout-ms 8000 2>/dev/null || true)"
    # One record must carry both the type and this run's address; two
    # separate greps could match two different events. No pipe into grep -q:
    # with pipefail a large $EVENTS can SIGPIPE the writer and fail the check.
    INVITED="$(grep -F "project_contact.invited" <<<"$EVENTS" | grep -c -F "$EMAIL" || true)"
    [[ "${INVITED:-0}" -ge 1 ]] || fail "project_contact.invited for $EMAIL not found on project-events"
    ok "project_contact.invited for $EMAIL is on project-events"
    LIST="$(curl -fsS -X POST "$ENTITY/projects/$PROJECT_ID/contacts/search" -H "x-jwt-assertion: $TOKEN" -H 'Content-Type: application/json' \
      -d "{\"filters\":{\"searchQuery\":\"$EMAIL\"},\"pagination\":{\"limit\":5,\"offset\":0}}")"
    [[ "$LIST" != *salesforceSync* ]] || { echo "$LIST"; fail "contact search still shows a sync flag"; }
    ok "contact search no longer shows a sync flag"
    say "all checks passed for $EMAIL"
    ;;
  run-exhaust)
    TOKEN="$(token)"
    EMAIL="deferred.exhaust.$(date +%s)@example.com"
    prepare
    outage false; drain; echo
    say "both tiers give up: the request must end FAILED"
    outage true
    RESP="$(curl -sS -o "$WORK/invite.json" -w '%{http_code}' -X POST "$ENTITY/projects/$PROJECT_ID/contacts" \
      -H "x-jwt-assertion: $TOKEN" -H 'Content-Type: application/json' \
      -d "{\"email\":\"$EMAIL\",\"firstName\":\"Exhausted\",\"lastName\":\"Invitee\",\"roles\":[\"Portal user\"]}")"
    [[ "$RESP" == "201" ]] || { cat "$WORK/invite.json"; fail "invite returned $RESP"; }
    PC_ID="$(sql "SELECT id FROM project_contact WHERE project_id='$PROJECT_ID' AND LOWER(email)='$EMAIL'")"
    ok "invited $EMAIL while down (project_contact $PC_ID)"
    for i in $(seq 1 120); do
      STATUS="$(sql "SELECT status FROM salesforce_write_request WHERE project_contact_id='$PC_ID'")"
      [[ "$STATUS" == "PENDING" ]] || break
      sleep 5
    done
    [[ "$STATUS" == "FAILED" ]] || fail "request is $STATUS after 10 minutes; want FAILED once the consumer gave up"
    ok "sales-entity-service gave up and answered membership_write.failed; request FAILED: $(sql "SELECT last_error FROM salesforce_write_request WHERE project_contact_id='$PC_ID'" | cut -c1-120)"
    LIST="$(curl -fsS -X POST "$ENTITY/projects/$PROJECT_ID/contacts/search" -H "x-jwt-assertion: $TOKEN" -H 'Content-Type: application/json' \
      -d "{\"filters\":{\"searchQuery\":\"$EMAIL\"},\"pagination\":{\"limit\":5,\"offset\":0}}")"
    [[ "$LIST" == *'"salesforceSync":"FAILED"'* ]] || { echo "$LIST"; fail "contact search does not show salesforceSync FAILED"; }
    ok "contact search shows salesforceSync FAILED; rows kept with NULL sf_id ($(sql "SELECT COALESCE(sf_id,'<null>') FROM project_contact WHERE id='$PC_ID'"))"
    [[ "$(sql "SELECT retryable FROM salesforce_write_request WHERE project_contact_id='$PC_ID'")" == "t" ]] || fail "an outage that outlived the retries must be recorded as retryable"
    ok "request recorded as retryable (an outage, not a refusal)"
    RESP="$(curl -sS -o "$WORK/patch.json" -w '%{http_code}' -X PATCH "$ENTITY/projects/$PROJECT_ID/contacts/$EMAIL" \
      -H "x-jwt-assertion: $TOKEN" -H 'Content-Type: application/json' -d '{"roles":["Admin"]}')"
    [[ "$RESP" == "409" ]] && grep -q salesforce_sync_failed "$WORK/patch.json" || { cat "$WORK/patch.json"; fail "PATCH on a failed row returned $RESP"; }
    ok "role change on the failed row -> 409 salesforce_sync_failed"
    RESP="$(curl -sS -o "$WORK/delete.json" -w '%{http_code}' -X DELETE "$ENTITY/projects/$PROJECT_ID/contacts/$EMAIL" -H "x-jwt-assertion: $TOKEN")"
    [[ "$RESP" == "204" ]] || { cat "$WORK/delete.json"; fail "DELETE on the failed row returned $RESP, want 204"; }
    [[ "$(sql "SELECT state FROM project_contact WHERE id='$PC_ID'")" == "DEACTIVATED" ]] || fail "the row was not deactivated"
    [[ "$(sql "SELECT status FROM salesforce_write_request WHERE project_contact_id='$PC_ID'")" == "CANCELLED" ]] || fail "the request was not cancelled"
    ok "the admin removed the contact locally: row DEACTIVATED, request CANCELLED, Salesforce untouched"
    outage false
    say "exhaust scenario passed for $EMAIL"
    ;;
  *) echo "usage: $0 up | run | run-exhaust | down" >&2; exit 2 ;;
esac
