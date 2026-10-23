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
# local compose stack plus scripts/csm-compose/sf-deferred-e2e.compose.yml:
#
#   1. switch the mock sales-entity-service into an outage
#   2. invite a brand-new address -> 201 with salesforceSync PENDING, rows
#      committed with NULL sf_id, a PENDING salesforce_write_request
#   3. a role change while pending -> 409 SALESFORCE_SYNC_PENDING
#   4. end the outage -> the consumer writes the mock Salesforce, stamps the
#      ids, marks the request SUCCEEDED, publishes project_contact.invited
#   5. no duplicate Salesforce records; the contact list shows no sync flag
#
# Usage: scripts/csm-compose/sf-deferred-e2e.sh up | run | run-exhaust | down
#   OUTAGE_SECONDS=15 run   keeps Salesforce down long enough for the main
#                           tier to give up, so the record is dead-lettered
#                           and finished by the dead-letter consumer
#   run-exhaust             keeps Salesforce down until both tiers give up:
#                           the request must end FAILED, never stay PENDING
set -euo pipefail
cd "$(dirname "$0")/../.."
COMPOSE=(docker compose -f docker-compose.yml -f scripts/csm-compose/sf-deferred-e2e.compose.yml)
PSQL=(docker compose exec -T postgres psql -U postgres -d csm_platform -v ON_ERROR_STOP=1 -tA)
ENTITY=${ENTITY_URL:-http://localhost:8081}
MOCK=${MOCK_SF_URL:-http://localhost:9200}
OIDC=${MOCK_OIDC_URL:-http://localhost:9100}
PROJECT_ID=${PROJECT_ID:-00000000-0000-0000-0000-000000000401} # seeded "Example Corp Production", sf_id SF-PROJ-0001

say()  { printf '\n\033[1m== %s\033[0m\n' "$*"; }
ok()   { printf '   \033[32mOK\033[0m  %s\n' "$*"; }
fail() { printf '   \033[31mFAIL\033[0m %s\n' "$*"; exit 1; }
sql()  { "${PSQL[@]}" -c "$1" </dev/null | tr -d '\r'; }

# An M2M caller: entity-service reads x-jwt-assertion's client_id (not
# signature-verified) and treats a client in M2M_CLIENT_IDS as internal.
token() {
  curl -fsS -X POST "$OIDC/oauth2/token" \
    -d grant_type=client_credentials -d client_id=csm-notification-service-dev-client -d client_secret=dev \
    | python3 -c 'import json,sys; print(json.load(sys.stdin)["access_token"])'
}
outage() { curl -fsS -X POST "$MOCK/__mock/outage" -H 'Content-Type: application/json' -d "{\"down\":$1}" >/dev/null; }

case "${1:-run}" in
  up)
    "${COMPOSE[@]}" up -d --build entity-service mock-sales-entity kafka-topics-init
    ;;
  down)
    "${COMPOSE[@]}" stop entity-service mock-sales-entity
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

    say "0. seed the account-level roles the membership write grants (local stack has none)"
    "${PSQL[@]}" -c "INSERT INTO role (id, created_on, updated_on, created_by, updated_by, name, description)
      SELECT gen_random_uuid(), now(), now(), 'sf-deferred-e2e', 'sf-deferred-e2e', r, 'seeded by sf-deferred-e2e.sh'
      FROM unnest(ARRAY['external','customer','partner','customer_admin','partner_admin']) AS r
      ON CONFLICT (name) DO NOTHING" </dev/null >/dev/null
    ok "roles present: $(sql "SELECT string_agg(name, ', ' ORDER BY name) FROM role WHERE name IN ('external','customer','partner','customer_admin','partner_admin')")"

    say "1. Salesforce goes down"
    outage true; ok "mock sales-entity answers 503"

    say "2. invite $EMAIL while it is down"
    RESP="$(curl -sS -o /tmp/sf-e2e-invite.json -w '%{http_code}' -X POST "$ENTITY/projects/$PROJECT_ID/contacts" \
      -H "x-jwt-assertion: $TOKEN" -H 'Content-Type: application/json' \
      -d "{\"email\":\"$EMAIL\",\"firstName\":\"Deferred\",\"lastName\":\"Invitee\",\"roles\":[\"Portal user\"]}")"
    [[ "$RESP" == "201" ]] || { cat /tmp/sf-e2e-invite.json; fail "invite returned $RESP, want 201"; }
    grep -q '"salesforceSync":"PENDING"' /tmp/sf-e2e-invite.json || { cat /tmp/sf-e2e-invite.json; fail "response lacks salesforceSync PENDING"; }
    ok "201 with salesforceSync PENDING: $(cat /tmp/sf-e2e-invite.json)"
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
    RESP="$(curl -sS -o /tmp/sf-e2e-patch.json -w '%{http_code}' -X PATCH "$ENTITY/projects/$PROJECT_ID/contacts/$EMAIL" \
      -H "x-jwt-assertion: $TOKEN" -H 'Content-Type: application/json' -d '{"roles":["Admin"]}')"
    [[ "$RESP" == "409" ]] && grep -q SALESFORCE_SYNC_PENDING /tmp/sf-e2e-patch.json || { cat /tmp/sf-e2e-patch.json; fail "PATCH returned $RESP"; }
    ok "409 $(cat /tmp/sf-e2e-patch.json)"
    ATTEMPTS="$(sql "SELECT attempts FROM salesforce_write_request WHERE project_contact_id='$PC_ID'")"
    ok "consumer has made $ATTEMPTS failed attempt(s) so far (main tier, then dead-letter tier)"

    say "4. Salesforce comes back${OUTAGE_SECONDS:+ after ${OUTAGE_SECONDS}s}"
    if [[ "${OUTAGE_SECONDS:-0}" -gt 0 ]]; then
      sleep "$OUTAGE_SECONDS"
      REQ_ID="$(sql "SELECT id FROM salesforce_write_request WHERE project_contact_id='$PC_ID'")"
      # Captured, not grep -q'd: see the startup check above (pipefail).
      DEADLETTERED="$(docker compose logs --since 5m entity-service 2>/dev/null | grep -c "main-topic attempts exhausted; dead-lettering" || true)"
      [[ "${DEADLETTERED:-0}" -gt 0 ]] || fail "the main tier never dead-lettered the record (is the outage long enough for ${OUTAGE_SECONDS}s?)"
      DLQ="$(docker compose exec -T kafka /opt/kafka/bin/kafka-console-consumer.sh --bootstrap-server kafka:9092 --topic salesforce-writes-dlq --from-beginning --timeout-ms 6000 2>/dev/null || true)"
      [[ "$DLQ" == *"$REQ_ID"* ]] || fail "request $REQ_ID is not on salesforce-writes-dlq"
      ok "main tier gave up and dead-lettered the record; the dead-letter consumer is retrying it ($(sql "SELECT attempts FROM salesforce_write_request WHERE id='$REQ_ID'") attempts so far)"
    fi
    outage false
    for i in $(seq 1 40); do
      STATUS="$(sql "SELECT status FROM salesforce_write_request WHERE project_contact_id='$PC_ID'")"
      [[ "$STATUS" == "SUCCEEDED" ]] && break
      [[ "$STATUS" == "FAILED" ]] && fail "request FAILED: $(sql "SELECT last_error FROM salesforce_write_request WHERE project_contact_id='$PC_ID'")"
      sleep 3
    done
    [[ "$STATUS" == "SUCCEEDED" ]] || fail "request still $STATUS after 2 minutes"
    ok "salesforce_write_request SUCCEEDED after $(sql "SELECT attempts FROM salesforce_write_request WHERE project_contact_id='$PC_ID'") attempt(s)"
    SF="$(sql "SELECT pc.sf_id||' / '||ac.sf_id||' / '||COALESCE(u.sf_id,'<null>') FROM project_contact pc JOIN account_contact ac ON ac.id=pc.account_contact_id LEFT JOIN \"user\" u ON LOWER(u.user_name)=LOWER(ac.user_name) WHERE pc.id='$PC_ID'")"
    [[ "$SF" == a0eMOCK* ]] || fail "sf ids not stamped: $SF"
    ok "sf_id stamped on project_contact / account_contact / user: $SF"
    [[ "$(sql "SELECT status FROM onboarding_step WHERE email='$EMAIL' AND step='DATABASE'")" == "SUCCEEDED" ]] || fail "DATABASE onboarding step missing"
    ok "DATABASE onboarding step recorded under the Salesforce id"

    say "5. exactly one Salesforce contact and one membership; invitation published"
    STATE="$(curl -fsS "$MOCK/__mock/state")"
    C="$(echo "$STATE" | python3 -c "
import json,sys
s=json.load(sys.stdin)
contacts=sum(1 for c in s['contacts'].values() if (c.get('email') or '').lower()=='$EMAIL')
memberships=sum(1 for m in s['memberships'].values() if ((m.get('contact') or {}).get('email') or '').lower()=='$EMAIL')
print(contacts, memberships)")"
    [[ "$C" == "1 1" ]] || fail "mock holds contacts/memberships = $C for $EMAIL (want 1 1)"
    ok "mock Salesforce holds 1 contact for $EMAIL and 1 membership"
    EVENTS="$(docker compose exec -T kafka /opt/kafka/bin/kafka-console-consumer.sh --bootstrap-server kafka:9092 --topic project-events --from-beginning --timeout-ms 8000 2>/dev/null || true)"
    echo "$EVENTS" | grep -q "project_contact.invited" && echo "$EVENTS" | grep -q "$EMAIL" || fail "project_contact.invited for $EMAIL not found on project-events"
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
    say "both tiers give up: the request must end FAILED"
    outage true
    RESP="$(curl -sS -o /tmp/sf-e2e-invite.json -w '%{http_code}' -X POST "$ENTITY/projects/$PROJECT_ID/contacts" \
      -H "x-jwt-assertion: $TOKEN" -H 'Content-Type: application/json' \
      -d "{\"email\":\"$EMAIL\",\"firstName\":\"Exhausted\",\"lastName\":\"Invitee\",\"roles\":[\"Portal user\"]}")"
    [[ "$RESP" == "201" ]] || { cat /tmp/sf-e2e-invite.json; fail "invite returned $RESP"; }
    PC_ID="$(sql "SELECT id FROM project_contact WHERE project_id='$PROJECT_ID' AND LOWER(email)='$EMAIL'")"
    ok "invited $EMAIL while down (project_contact $PC_ID)"
    for i in $(seq 1 60); do
      STATUS="$(sql "SELECT status FROM salesforce_write_request WHERE project_contact_id='$PC_ID'")"
      [[ "$STATUS" == "PENDING" ]] || break
      sleep 5
    done
    [[ "$STATUS" == "FAILED" ]] || fail "request is $STATUS after 5 minutes; want FAILED once both tiers are exhausted"
    ok "request FAILED after $(sql "SELECT attempts FROM salesforce_write_request WHERE project_contact_id='$PC_ID'") attempts: $(sql "SELECT last_error FROM salesforce_write_request WHERE project_contact_id='$PC_ID'" | cut -c1-120)"
    LIST="$(curl -fsS -X POST "$ENTITY/projects/$PROJECT_ID/contacts/search" -H "x-jwt-assertion: $TOKEN" -H 'Content-Type: application/json' \
      -d "{\"filters\":{\"searchQuery\":\"$EMAIL\"},\"pagination\":{\"limit\":5,\"offset\":0}}")"
    [[ "$LIST" == *'"salesforceSync":"FAILED"'* ]] || { echo "$LIST"; fail "contact search does not show salesforceSync FAILED"; }
    ok "contact search shows salesforceSync FAILED; rows kept with NULL sf_id ($(sql "SELECT COALESCE(sf_id,'<null>') FROM project_contact WHERE id='$PC_ID'"))"
    outage false
    say "exhaust scenario passed for $EMAIL"
    ;;
  *) echo "usage: $0 up | run | run-exhaust | down" >&2; exit 2 ;;
esac
