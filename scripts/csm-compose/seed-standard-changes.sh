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
# Fill the standard change catalogue of the RUNNING LOCAL STACK with realistic
# templates, proposals and change requests raised from them -- through the real
# flow (entity-service's API, as the seeded staff personas), never by writing the
# standard change tables directly. LOCAL DEVELOPMENT ONLY: it refuses any
# entity-service or identity provider that is not on localhost.
#
# WHO PLAYS WHAT (the local seed's internal personas, all in "CAB Approval"):
#   bob.fernando@example.com   proposes every template (the proposer: he can never
#                              decide his own proposal) and owns what gets approved
#   carol.silva@example.com    the CAB member who decides (approves / rejects)
#   alice.perera@example.com   the engineer who raises and walks change requests
#                              from the templates (and is their "Requested by")
#
# WHAT IT LEAVES (so every screen of the feature has something to show):
#   * five templates, each proposed by bob, submitted and approved by carol, with
#     generalised plans using {{key}} placeholders and parameters of every type
#     (text, textarea, number, date, select), every one of them required: an
#     optional value inside a sentence would render "... no later than ." when
#     left empty;
#   * evidence that relates: a NEW proposal cites only closed change requests
#     whose subject matches its own keywords (never "[E2E] ..." or "QA-STD-E2E"
#     test leftovers), and starts from the first of them when its spec says so;
#     with no match it cites nothing;
#   * "Scale a Kubernetes deployment's replicas" revised to version 2 through an
#     approved MODIFY proposal, which cites the two change requests alice raised
#     from version 1 and walked to Closed, each recording the deviation that
#     motivated version 2 (the autoscaler undid the scale);
#   * "Restart an API Manager node" SUSPENDED by its owner, with the reason;
#   * one proposal REJECTED by the CAB (with its comment), one still IN CAB
#     REVIEW, one DRAFT;
#   * change requests raised from templates by alice: one left in New, one
#     Request-Approved (a standard change goes straight to Scheduled), the two
#     closed ones above, and one walked Scheduled -> Implement -> Review ->
#     ROLLED BACK, which raises "Review needed" on its template.
#
# IDEMPOTENT: every step looks first and does only what is missing. A template
# whose name already exists is skipped; a proposal is found again by its name
# among bob's own proposals and carried on from the state it is in (a draft is
# completed and submitted, one in CAB review is decided) rather than duplicated;
# a change request is found again by its rendered subject and walked on from its
# state. Run it twice: the second run reports "0 created, 0 write request(s)".
#
# --reset (off by default) first DELETES, in one transaction on the local
# Postgres, what an earlier run of this script left, so the next run rebuilds it
# from scratch:
#   * the templates with this script's names whose NEW proposal bob raised, with
#     their versions;
#   * bob's proposals with this script's names, and his proposals about those
#     templates (the MODIFY);
#   * every change request raised from those templates;
#   * leftover "Untitled standard change" drafts (NEW, still DRAFT, never named:
#     what opening "Propose" and walking away leaves), whoever started them;
#   * the event-outbox rows of the change requests deleted.
# A proposal or change request is a work item: deleting the work item takes its
# approval stages and approvers, evidence, comments and activity with it. Nothing
# else is touched (other templates, the E2E spec's QA-STD-E2E rows, change
# requests not raised from these templates). The STD and SCP number series are
# then set back to the highest number still in use, so the rebuilt templates
# start again at STD0000001 when nothing else holds a number.
#
# Usage (the stock local stack: entity-service :8081, mock-oidc :9100):
#   scripts/csm-compose/seed-standard-changes.sh            # seed / complete
#   scripts/csm-compose/seed-standard-changes.sh --reset    # delete the seed's rows, then seed
# Overrides: ENTITY_SERVICE_URL, OIDC_URL. Needs bash, curl and jq.
# --reset reaches Postgres with `docker compose exec -T postgres psql` on the
# repository's docker-compose.yml (falling back to docker-compose); a stack
# started under another project name needs COMPOSE_PROJECT_NAME=<name>, or name
# the container outright with PG_CONTAINER=<name> (docker exec).
#
# entity-service resolves an INTERNAL user from the forwarded x-user-id-token
# alone in this stack, so the script calls it directly with an ID token the mock
# provider signs for each persona (no password is checked there).

set -euo pipefail

RESET="no"
for arg in "$@"; do
  case "$arg" in
    --reset) RESET="yes" ;;
    -h | --help) sed -n '18,89p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "unknown argument: $arg (try --help)" >&2; exit 2 ;;
  esac
done

ES_URL="${ENTITY_SERVICE_URL:-http://localhost:8081}"
OIDC_URL="${OIDC_URL:-http://localhost:9100}"
OIDC_CLIENT="csm-portal-webapp"
OIDC_REDIRECT="http://localhost:3001"
REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

PROPOSER="bob.fernando@example.com"
CAB_DECIDER="carol.silva@example.com"
ENGINEER="alice.perera@example.com"

# The local seed's fixed rows: the assigned team every fixture uses, and the
# customer project the change requests are raised for.
EXAMPLE_CORP_ABT_GROUP_ID="00000000-0000-0000-0000-000000000901"
EXAMPLE_CORP_PROJECT_ID="00000000-0000-0000-0000-000000000401"

for url in "$ES_URL" "$OIDC_URL"; do
  case "$url" in
    http://localhost:* | http://127.0.0.1:* | http://\[::1\]:*) ;;
    *) echo "refusing $url: this seeds the LOCAL stack only" >&2; exit 2 ;;
  esac
done
command -v jq >/dev/null || { echo "jq is required" >&2; exit 2; }

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

CREATED=0 # records this run created
WRITES=0  # every write request this run sent (not counting searches and reads)

log() { printf '%s\n' "$*"; }
step() { printf '  - %s\n' "$*"; }
die() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }

# ---------------------------------------------------------------------------
# HTTP

# id_token EMAIL: an ID token from the mock provider (internal staff: the CSM
# portal's engineer group).
id_token() {
  local email="$1" location code
  location="$(curl -sS -o /dev/null -w '%{redirect_url}' -X POST "$OIDC_URL/oauth2/authorize" \
    --data-urlencode "client_id=$OIDC_CLIENT" --data-urlencode "redirect_uri=$OIDC_REDIRECT" \
    --data-urlencode "state=seed" --data-urlencode "scope=openid" \
    --data-urlencode "email=$email" --data-urlencode "groups=cs_engineer")"
  code="$(printf '%s' "$location" | sed -n 's/.*[?&]code=\([^&]*\).*/\1/p')"
  [ -n "$code" ] || die "mock-oidc gave no authorization code for $email"
  curl -sS -X POST "$OIDC_URL/oauth2/token" \
    --data-urlencode "grant_type=authorization_code" --data-urlencode "code=$code" \
    --data-urlencode "client_id=$OIDC_CLIENT" --data-urlencode "redirect_uri=$OIDC_REDIRECT" |
    jq -er '.id_token' || die "mock-oidc gave no ID token for $email"
}

# call METHOD PATH TOKEN [JSON]: sets STATUS and BODY (no subshell, so both survive).
STATUS=""
BODY=""
call() {
  local method="$1" path="$2" token="$3" data="${4:-}"
  if [ -n "$data" ]; then
    STATUS="$(curl -sS -o "$WORK/body" -w '%{http_code}' -X "$method" "$ES_URL$path" \
      -H "x-user-id-token: $token" -H 'content-type: application/json' --data-binary "$data")"
  else
    STATUS="$(curl -sS -o "$WORK/body" -w '%{http_code}' -X "$method" "$ES_URL$path" \
      -H "x-user-id-token: $token")"
  fi
  BODY="$(cat "$WORK/body")"
}

# must WHAT METHOD PATH TOKEN [JSON]: call, and stop on anything but 2xx.
must() {
  local what="$1"
  shift
  call "$@"
  case "$STATUS" in
    2??) ;;
    *) die "$what: $1 $2 answered HTTP $STATUS: $BODY" ;;
  esac
  case "$1 $2" in
    GET\ * | *\ */search) ;;
    *) WRITES=$((WRITES + 1)) ;;
  esac
}

# ---------------------------------------------------------------------------
# Lookups

# template_json NAME: the template of that name (any state, ignoring case), or "".
template_json() {
  local name="$1"
  must "search templates" POST /standard-change-templates/search "$TOK_BOB" \
    "$(jq -nc --arg q "$name" '{states:["active","suspended","retired"], query:$q, pagination:{limit:50, offset:0}}')"
  printf '%s' "$BODY" | jq -c --arg n "$name" \
    '[.templates[] | select((.name|ascii_downcase) == ($n|ascii_downcase))][0] // empty'
}

# proposal_json NAME KIND: bob's latest proposal of that name and kind that was
# not withdrawn, or "".
proposal_json() {
  local name="$1" kind="$2"
  must "search proposals" POST /standard-change-proposals/search "$TOK_BOB" \
    "$(jq -nc --arg q "$name" --arg k "$kind" '{kinds:[$k], proposedByMe:true, query:$q, pagination:{limit:50, offset:0}}')"
  printf '%s' "$BODY" | jq -c --arg n "$name" \
    '[.proposals[] | select(.name == $n and .state != "withdrawn")] | sort_by(.createdOn) | last // empty'
}

# change_request_json SUBJECT: the change request with exactly that subject, or "".
change_request_json() {
  local subject="$1"
  must "search change requests" POST /change-requests/search "$TOK_ALICE" \
    "$(jq -nc --arg q "$subject" '{filters:{searchQuery:$q}, pagination:{limit:50, offset:0}}')"
  printf '%s' "$BODY" | jq -c --arg s "$subject" \
    '[(.changeRequests // .results // .items // [])[] | select(.subject == $s)][0] // empty'
}

# load_closed_change_requests: CLOSED_CRS, the closed change requests on this
# stack that may serve as evidence, as a JSON array of {id, number, subject}:
# test leftovers ("[E2E] ..." and the standard change spec's QA-STD-E2E rows)
# are never evidence.
load_closed_change_requests() {
  local offset=0 page all="[]"
  while :; do
    must "search closed change requests" POST /change-requests/search "$TOK_ALICE" \
      "$(jq -nc --argjson o "$offset" '{filters:{states:["closed"]}, pagination:{limit:50, offset:$o}}')"
    page="$(printf '%s' "$BODY" | jq -c '[(.changeRequests // .results // .items // [])[] | {id, number, subject}]')"
    all="$(jq -nc --argjson a "$all" --argjson p "$page" '$a + $p')"
    [ "$(printf '%s' "$page" | jq length)" -eq 50 ] || break
    offset=$((offset + 50))
  done
  CLOSED_CRS="$(printf '%s' "$all" | jq -c \
    '[.[] | select((.subject // "") as $s | ($s | startswith("[E2E]") | not) and ($s | contains("QA-STD-E2E") | not))]')"
}

# related_evidence REGEX: the ids of the closed change requests whose subject
# matches REGEX (ignoring case), oldest number first, at most three; "[]" when
# none relates.
related_evidence() {
  printf '%s' "$CLOSED_CRS" | jq -c --arg re "$1" \
    '[.[] | select(.subject | test($re; "i"))] | sort_by(.number) | [.[].id][0:3]'
}

# ---------------------------------------------------------------------------
# Proposals

# drive_proposal ID TARGET SPEC_FILE EVIDENCE_IDS_JSON [REJECT_COMMENT]
# Carries a proposal of bob's from whatever state it is in to TARGET (draft,
# cab_review, approved or rejected): a draft is filled from the spec (its
# evidence set to exactly EVIDENCE_IDS_JSON) and submitted, one in CAB review is
# decided by carol.
drive_proposal() {
  local id="$1" target="$2" spec="$3" evidence="$4" comment="${5:-}" state patch number
  must "read proposal" GET "/standard-change-proposals/$id" "$TOK_BOB"
  state="$(printf '%s' "$BODY" | jq -r .state)"
  number="$(printf '%s' "$BODY" | jq -r .number)"
  if [ "$state" = "draft" ] && [ "$target" = "draft" ] && [ "${FOUND_EXISTING:-}" = "yes" ]; then
    # A draft found again is left alone: someone may be editing it in the portal.
    step "$number is already a draft: left as it is"
    return 0
  fi
  if [ "$state" = "draft" ]; then
    patch="$(jq -c --argjson ev "$evidence" '.patch + {evidenceChangeRequestIds: $ev}' "$spec")"
    if [ "$evidence" != "[]" ]; then
      # Say what the evidence is only when there is some.
      patch="$(printf '%s' "$patch" | jq -c --arg note "$(jq -r '.evidenceNote // ""' "$spec")" '.rationale += $note')"
    fi
    must "fill $number" PATCH "/standard-change-proposals/$id" "$TOK_BOB" "$patch"
    step "$number filled in as a draft, citing $(printf '%s' "$evidence" | jq length) change request(s) as evidence"
    [ "$target" = "draft" ] && return 0
    must "submit $number" POST "/standard-change-proposals/$id/submit" "$TOK_BOB"
    step "$number submitted to the CAB"
    state="cab_review"
  fi
  if [ "$state" = "cab_review" ]; then
    case "$target" in
      cab_review) step "$number is in CAB review"; return 0 ;;
      approved)
        must "approve $number" POST "/standard-change-proposals/$id/decision" "$TOK_CAROL" '{"decision":"approved"}'
        step "$number approved by carol" ;;
      rejected)
        must "reject $number" POST "/standard-change-proposals/$id/decision" "$TOK_CAROL" \
          "$(jq -nc --arg c "$comment" '{decision:"rejected", comment:$c}')"
        step "$number rejected by carol" ;;
    esac
    return 0
  fi
  step "$number is already $state"
}

# ensure_new_proposal SPEC_FILE TARGET [REJECT_COMMENT]
# The spec's "evidenceMatch" picks the evidence (related_evidence); with
# "startFromEvidence" the proposal is started from the first of it, the way
# "Propose as standard change" on a closed change request starts one.
ensure_new_proposal() {
  local spec="$1" target="$2" comment="${3:-}" name found body id evidence source
  name="$(jq -r .patch.name "$spec")"
  log "* $name  (target: $target)"
  FOUND_EXISTING="no"
  if [ "$target" = "approved" ] && [ -n "$(template_json "$name")" ]; then
    step "template already exists: skipped"
    return 0
  fi
  evidence="$(related_evidence "$(jq -r '.evidenceMatch // "(?!)"' "$spec")")"
  source=""
  [ "$(jq -r '.startFromEvidence // false' "$spec")" = "true" ] && source="$(printf '%s' "$evidence" | jq -r '.[0] // empty')"
  found="$(proposal_json "$name" new)"
  if [ -n "$found" ]; then
    FOUND_EXISTING="yes"
    id="$(printf '%s' "$found" | jq -r .id)"
    step "found $(printf '%s' "$found" | jq -r .number) ($(printf '%s' "$found" | jq -r .state))"
  else
    if [ -n "$source" ]; then
      body="$(jq -nc --arg n "$name" --arg s "$source" '{kind:"new", name:$n, sourceChangeRequestId:$s}')"
    else
      body="$(jq -nc --arg n "$name" '{kind:"new", name:$n}')"
    fi
    must "create proposal" POST /standard-change-proposals "$TOK_BOB" "$body"
    id="$(printf '%s' "$BODY" | jq -r '.id // .proposal.id')"
    CREATED=$((CREATED + 1))
    step "created $(printf '%s' "$BODY" | jq -r '.number // .proposal.number')${source:+ from closed change request $(printf '%s' "$CLOSED_CRS" | jq -r --arg s "$source" '.[] | select(.id == $s) | "\(.number) \"\(.subject)\""')}"
  fi
  drive_proposal "$id" "$target" "$spec" "$evidence" "$comment"
}

# ensure_modified TEMPLATE_NAME SPEC_FILE EVIDENCE_IDS_JSON: the template at
# version 2 or later, through a MODIFY proposal bob raises and carol approves.
ensure_modified() {
  local name="$1" spec="$2" evidence="$3" t version open id
  log "* $name  (target: version 2 through a modification)"
  FOUND_EXISTING="no"
  t="$(template_json "$name")"
  [ -n "$t" ] || die "template \"$name\" does not exist"
  version="$(printf '%s' "$t" | jq -r '.currentVersion.version')"
  if [ "$version" -ge 2 ]; then
    step "already at version $version: skipped"
    return 0
  fi
  open="$(printf '%s' "$t" | jq -r '.openProposal.id // empty')"
  if [ -n "$open" ]; then
    FOUND_EXISTING="yes"
    id="$open"
    step "found the open modification"
  else
    must "create modification" POST /standard-change-proposals "$TOK_BOB" \
      "$(jq -nc --arg t "$(printf '%s' "$t" | jq -r .id)" '{kind:"modify", templateId:$t}')"
    id="$(printf '%s' "$BODY" | jq -r '.id // .proposal.id')"
    CREATED=$((CREATED + 1))
    step "created $(printf '%s' "$BODY" | jq -r '.number // .proposal.number') from version $version"
  fi
  drive_proposal "$id" approved "$spec" "$evidence"
}

# ensure_suspended TEMPLATE_NAME REASON: suspended by its owner (bob), with REASON.
ensure_suspended() {
  local name="$1" reason="$2" t
  log "* $name  (target: suspended)"
  t="$(template_json "$name")"
  [ -n "$t" ] || die "template \"$name\" does not exist"
  case "$(printf '%s' "$t" | jq -r .state)" in
    suspended)
      step "already suspended: skipped (reason: $(printf '%s' "$t" | jq -r '.suspendedReason // "none given"'))" ;;
    active)
      must "suspend $name" POST "/standard-change-templates/$(printf '%s' "$t" | jq -r .id)/suspend" "$TOK_BOB" \
        "$(jq -nc --arg r "$reason" '{reason:$r}')"
      step "suspended by its owner (reason stored: $(printf '%s' "$BODY" | jq -r '.suspendedReason // .template.suspendedReason // "?" | .[0:60]')...)" ;;
    *) step "is $(printf '%s' "$t" | jq -r .state): left as it is" ;;
  esac
}

# ---------------------------------------------------------------------------
# Change requests raised from templates

# patch_state ID STATE NUMBER
patch_state() {
  must "move $3 to $2" PATCH "/change-requests/$1" "$TOK_ALICE" "$(jq -nc --arg s "$2" '{state:$s}')"
  step "$3 -> $(printf '%s' "$BODY" | jq -r '.state // .changeRequest.state // "?"')"
}

# ensure_change_request TEMPLATE_NAME SUBJECT PARAMETERS_JSON TARGET START END [DEVIATION] [AT_VERSION]
# TARGET: new, scheduled, closed or rollback. SUBJECT is what the template
# renders with PARAMETERS_JSON: it is how the change request is found again.
# DEVIATION is recorded in Review, before the change is closed. AT_VERSION: raise
# it only while the template is at that version (its parameters are that
# version's); otherwise a change request not found is skipped.
# Sets CR_ID to the change request's id ("" when skipped).
CR_ID=""
ensure_change_request() {
  local template="$1" subject="$2" params="$3" target="$4" start="$5" end="$6" deviation="${7:-}" at_version="${8:-}"
  local t cr id number state rendered
  CR_ID=""
  log "* $subject  (target: $target)"
  cr="$(change_request_json "$subject")"
  if [ -z "$cr" ]; then
    t="$(template_json "$template")"
    [ -n "$t" ] || die "template \"$template\" does not exist"
    if [ -n "$at_version" ] && [ "$(printf '%s' "$t" | jq -r .currentVersion.version)" != "$at_version" ]; then
      step "not raised: $(printf '%s' "$t" | jq -r .number) is no longer at version $at_version"
      return 0
    fi
    must "raise from $template" POST /change-requests "$TOK_ALICE" "$(jq -nc \
      --arg t "$(printf '%s' "$t" | jq -r .id)" --argjson p "$params" \
      --arg g "$EXAMPLE_CORP_ABT_GROUP_ID" --arg pr "$EXAMPLE_CORP_PROJECT_ID" --arg s "$start" --arg e "$end" \
      --arg rb "$ENGINEER_ID" \
      '{type:"standard", standardChangeTemplateId:$t, standardChangeParameters:$p, groupId:$g, projectId:$pr,
        requestedById:$rb, plannedStartDate:$s, plannedEndDate:$e}')"
    id="$(printf '%s' "$BODY" | jq -r '.changeRequest.id // .id')"
    number="$(printf '%s' "$BODY" | jq -r '.changeRequest.number // .number')"
    state="new"
    CREATED=$((CREATED + 1))
    step "raised $number from $(printf '%s' "$t" | jq -r .number) v$(printf '%s' "$t" | jq -r .currentVersion.version)"
    rendered="$(printf '%s' "$BODY" | jq -r '.changeRequest.subject // .subject // empty')"
    [ -z "$rendered" ] || [ "$rendered" = "$subject" ] ||
      die "$number rendered the subject \"$rendered\", not \"$subject\": fix SUBJECT so a re-run finds it"
  else
    id="$(printf '%s' "$cr" | jq -r .id)"
    number="$(printf '%s' "$cr" | jq -r .number)"
    must "read $number" GET "/change-requests/$id" "$TOK_ALICE"
    state="$(printf '%s' "$BODY" | jq -r '.state // .changeRequest.state' | tr '[:upper:]' '[:lower:]')"
    step "found $number ($state)"
  fi
  CR_ID="$id"
  [ "$target" = "new" ] && return 0
  [ "$state" = "new" ] && { patch_state "$id" assess "$number"; state="scheduled"; }
  [ "$target" = "scheduled" ] && return 0
  [ "$state" = "scheduled" ] && { patch_state "$id" implement "$number"; state="implement"; }
  [ "$state" = "implement" ] && { patch_state "$id" review "$number"; state="review"; }
  if [ "$state" = "review" ]; then
    if [ -n "$deviation" ]; then
      must "record the deviation on $number" PATCH "/change-requests/$id" "$TOK_ALICE" \
        "$(jq -nc --arg d "$deviation" '{implementationDeviation:$d}')"
      step "$number: deviation from the implementation plan recorded"
    fi
    patch_state "$id" "$target" "$number"
  fi
  return 0
}

# ===========================================================================
# The templates. Each spec is {patch: <PATCH /standard-change-proposals body>}
# plus, for a NEW proposal, how its evidence is found: "evidenceMatch" (a regex on
# the subject of closed change requests), "startFromEvidence" and "evidenceNote"
# (appended to the rationale when there is evidence).

cat >"$WORK/tls.json" <<'JSON'
{"evidenceMatch": "\\b(tls|ssl|certificate)\\b",
 "evidenceNote": "<p>The cited evidence is earlier certificate rotation work closed on this stack.</p>",
 "patch": {
  "name": "Rotate the TLS certificate on an API gateway",
  "rationale": "<p>Certificate rotation on a gateway is done the same way every time (secret update, rolling restart one node at a time, fingerprint check) and is invisible to API consumers when done that way. Engineers raise one or more of these every week as customer certificates approach expiry; pre-authorising it removes a CAB round trip from a change whose only real risk is a bad chain, which the readiness check catches on the first node.</p>",
  "subject": "Rotate the TLS certificate on {{gateway_host}} ({{environment}})",
  "description": "<p>Replace the server certificate presented by the API gateway <strong>{{gateway_host}}</strong> in the {{environment}} environment with a renewed certificate issued by {{certificate_issuer}}. The certificate now served expires on {{current_expiry}}.</p>",
  "justification": "<p>An expired certificate makes every API call through the gateway fail its TLS handshake. Rotating it ahead of expiry keeps the customer's traffic flowing.</p>",
  "implementationPlan": "<ol><li>Confirm the renewed certificate and its private key are in the secret store at <code>{{secret_path}}</code>, that the chain is complete and that the subject alternative names cover:<br>{{subject_alt_names}}</li><li>Record the fingerprint and expiry of the certificate now served: <code>openssl s_client -connect {{gateway_host}}:443 -servername {{gateway_host}} &lt;/dev/null | openssl x509 -noout -fingerprint -enddate</code></li><li>Point the gateway's TLS secret (Kubernetes) or keystore alias (VM) at the new certificate. Keep the previous certificate as the secret's previous version.</li><li>Roll the gateway nodes, at most {{max_unavailable}} at a time, waiting for each to pass its readiness check before the next.</li><li>Confirm every node serves the new fingerprint and expiry date.</li></ol>",
  "riskImpactAnalysis": "<p><strong>Low.</strong> Nodes are rolled behind the load balancer, so capacity drops by at most {{max_unavailable}} node(s) and no request is dropped. The one failure mode, an incomplete chain or a key that does not match, fails the readiness check of the first node and stops the rollout there.</p>",
  "backoutPlan": "<ol><li>Point the TLS secret / keystore alias back at the previous certificate (still valid until {{current_expiry}}).</li><li>Roll the gateway nodes again, at most {{max_unavailable}} at a time.</li><li>Confirm the fingerprint recorded in step 2 of the implementation plan is served again.</li></ol>",
  "testPlan": "<ul><li><code>curl -vI https://{{gateway_host}}/</code> shows the new expiry date and a verified chain.</li><li>An API call through the gateway with the customer's test application returns 200.</li><li>No TLS handshake errors in the gateway logs for 15 minutes after the rollout.</li></ul>",
  "category": "network", "impact": "low", "risk": "low",
  "rollbackDurationText": "15 minutes",
  "isPlanningVisibleToCustomers": true, "customerApprovalRequired": false, "customerReviewRequired": false,
  "parameters": [
    {"key": "gateway_host", "label": "Gateway host name", "type": "text", "required": true, "helpText": "The host name clients connect to, e.g. gw.example.com"},
    {"key": "environment", "label": "Environment", "type": "select", "required": true, "options": ["Development", "Staging", "Production"]},
    {"key": "certificate_issuer", "label": "Issuer of the new certificate", "type": "select", "required": true, "options": ["DigiCert", "Let's Encrypt", "Customer-provided CA"]},
    {"key": "current_expiry", "label": "Expiry date of the current certificate", "type": "date", "required": true},
    {"key": "secret_path", "label": "Secret store path of the new certificate", "type": "text", "required": true},
    {"key": "subject_alt_names", "label": "Subject alternative names", "type": "textarea", "required": true, "helpText": "One host name per line"},
    {"key": "max_unavailable", "label": "Nodes out of rotation at once", "type": "number", "required": true, "helpText": "1 unless the gateway has more than six nodes"}
  ]
}}
JSON

cat >"$WORK/scale.json" <<'JSON'
{"evidenceMatch": "\\b(scale|scaling|replicas?)\\b",
 "evidenceNote": "<p>The cited evidence is earlier replica scaling work closed on this stack.</p>",
 "patch": {
  "name": "Scale a Kubernetes deployment's replicas",
  "rationale": "<p>Changing the replica count of a stateless deployment is routine capacity work, done ahead of customer load tests and seasonal peaks, and is reversed by setting the count back. It does not change the image, the configuration or the data.</p>",
  "subject": "Scale {{namespace}}/{{deployment}} on {{cluster}} to {{target_replicas}} replicas",
  "description": "<p>Change the number of replicas of the deployment <code>{{namespace}}/{{deployment}}</code> on cluster <strong>{{cluster}}</strong> from {{current_replicas}} to {{target_replicas}}.</p><p>Reason: {{reason}}</p>",
  "justification": "<p>{{reason}}</p>",
  "implementationPlan": "<ol><li>Check the current state: <code>kubectl --context {{cluster}} -n {{namespace}} get deploy {{deployment}}</code> shows {{current_replicas}} ready replicas.</li><li>Scale: <code>kubectl --context {{cluster}} -n {{namespace}} scale deploy/{{deployment}} --replicas={{target_replicas}}</code></li><li>Wait for the rollout: <code>kubectl --context {{cluster}} -n {{namespace}} rollout status deploy/{{deployment}} --timeout=10m</code></li></ol>",
  "riskImpactAnalysis": "<p><strong>Low.</strong> Only the replica count changes. Scaling down keeps at least the PodDisruptionBudget's minimum available.</p>",
  "backoutPlan": "<ol><li>Scale back: <code>kubectl --context {{cluster}} -n {{namespace}} scale deploy/{{deployment}} --replicas={{current_replicas}}</code></li><li>Wait for <code>rollout status</code> to report success.</li></ol>",
  "testPlan": "<ul><li><code>kubectl get deploy</code> shows {{target_replicas}}/{{target_replicas}} ready.</li><li>The service's error rate and latency dashboards show no change for 15 minutes.</li></ul>",
  "category": "cloud_computing", "impact": "low", "risk": "low",
  "rollbackDurationText": "10 minutes",
  "isPlanningVisibleToCustomers": false, "customerApprovalRequired": false, "customerReviewRequired": false,
  "parameters": [
    {"key": "cluster", "label": "Cluster (kubectl context)", "type": "text", "required": true},
    {"key": "namespace", "label": "Namespace", "type": "text", "required": true},
    {"key": "deployment", "label": "Deployment", "type": "text", "required": true},
    {"key": "current_replicas", "label": "Current replicas", "type": "number", "required": true},
    {"key": "target_replicas", "label": "Target replicas", "type": "number", "required": true},
    {"key": "reason", "label": "Why the capacity changes", "type": "textarea", "required": true}
  ]
}}
JSON

# Version 2 of the scaling template: v1's `kubectl scale` is undone within a
# minute by a HorizontalPodAutoscaler (both v1 change requests below record it as
# a deviation), so v2 adjusts the autoscaler when there is one, checks node
# headroom first and records until when the change holds.
cat >"$WORK/scale-v2.json" <<'JSON'
{"patch": {
  "rationale": "<p><strong>Why version 2:</strong> the two changes raised from version 1 and cited as evidence did not hold as planned. In both, a HorizontalPodAutoscaler managed the deployment and set the replica count back within a minute of <code>kubectl scale</code>; the engineer raised the autoscaler's minimum instead, outside the plan, and recorded it as a deviation. Version 2 asks how the replicas are managed and, for an autoscaler, changes its minimum; it also checks that the nodes have room before scaling up, and records until when the new capacity is needed.</p>",
  "description": "<p>Change the number of replicas of the deployment <code>{{namespace}}/{{deployment}}</code> on cluster <strong>{{cluster}}</strong> from {{current_replicas}} to {{target_replicas}}, until {{revert_on}}.</p><p>Replicas managed by: {{scaling_mode}}</p><p>Reason: {{reason}}</p>",
  "implementationPlan": "<ol><li>Check the current state: <code>kubectl --context {{cluster}} -n {{namespace}} get deploy,hpa</code> shows {{current_replicas}} ready replicas.</li><li>Scaling up: confirm the nodes have room for the extra pods (<code>kubectl describe nodes</code>, allocatable minus requested), or that the cluster autoscaler can add a node.</li><li>Scale, according to how the replicas are managed ({{scaling_mode}}):<ul><li>The deployment: <code>kubectl --context {{cluster}} -n {{namespace}} scale deploy/{{deployment}} --replicas={{target_replicas}}</code></li><li>A HorizontalPodAutoscaler: <code>kubectl --context {{cluster}} -n {{namespace}} patch hpa {{deployment}} -p '{\"spec\":{\"minReplicas\":{{target_replicas}}}}'</code></li></ul></li><li>Wait for the rollout: <code>kubectl --context {{cluster}} -n {{namespace}} rollout status deploy/{{deployment}} --timeout=10m</code></li><li>Add a calendar reminder to revert on {{revert_on}} (a new change from this template).</li></ol>",
  "backoutPlan": "<ol><li>Set the replica count (deployment) or the autoscaler's minimum back to {{current_replicas}}, with the command used in step 3.</li><li>Wait for <code>rollout status</code> to report success.</li></ol>",
  "testPlan": "<ul><li><code>kubectl get deploy</code> shows {{target_replicas}}/{{target_replicas}} ready, and still does five minutes later (no autoscaler undid it).</li><li>The service's error rate and latency dashboards show no change for 15 minutes.</li></ul>",
  "parameters": [
    {"key": "cluster", "label": "Cluster (kubectl context)", "type": "text", "required": true},
    {"key": "namespace", "label": "Namespace", "type": "text", "required": true},
    {"key": "deployment", "label": "Deployment", "type": "text", "required": true},
    {"key": "scaling_mode", "label": "Replicas managed by", "type": "select", "required": true, "options": ["The deployment (no autoscaler)", "A HorizontalPodAutoscaler (change its minimum)"]},
    {"key": "current_replicas", "label": "Current replicas", "type": "number", "required": true},
    {"key": "target_replicas", "label": "Target replicas", "type": "number", "required": true},
    {"key": "revert_on", "label": "Needed until", "type": "date", "required": true, "helpText": "When the capacity goes back to the current count"},
    {"key": "reason", "label": "Why the capacity changes", "type": "textarea", "required": true}
  ]
}}
JSON

cat >"$WORK/update-level.json" <<'JSON'
{"evidenceMatch": "\\b(upgrade|update level|product update)\\b", "startFromEvidence": true,
 "evidenceNote": "<p>The cited evidence is earlier product upgrade work closed on this stack; the proposal was started from the first change request cited.</p>",
 "patch": {
  "name": "Apply a WSO2 product update level to a deployment",
  "rationale": "<p>Applying a WSO2 update level to a managed deployment follows the same rolling procedure on every product: pull the updated image, roll the nodes, verify. Updates that ship no database script do not change data and are reverted by rolling back to the previous image. Updates WITH a database script are excluded (see the risk analysis) and stay normal changes.</p>",
  "subject": "Apply {{product}} {{product_version}} update level {{target_update_level}} to {{deployment_name}}",
  "description": "<p>Move <strong>{{deployment_name}}</strong> ({{product}} {{product_version}}) from update level {{current_update_level}} to update level {{target_update_level}}, in the maintenance window of {{maintenance_date}}.</p><p>The customer's constraints on the window: {{customer_constraints}}</p>",
  "justification": "<p>Keeps the deployment on a supported update level and brings in the security and bug fixes released since update level {{current_update_level}}.</p>",
  "implementationPlan": "<ol><li>Check the update's release notes: it must ship <strong>no database script</strong> and no configuration change that needs a manual step. If it does, stop: this is not a standard change.</li><li>Take a configuration backup of {{deployment_name}} and record the image tag now deployed.</li><li>Build or pull the image of {{product}} {{product_version}} at update level {{target_update_level}} and push it to the deployment's registry.</li><li>Update the deployment's image tag and roll the nodes one at a time, waiting for each to pass its health check before the next.</li><li>Confirm every node reports update level {{target_update_level}} in its startup log.</li></ol>",
  "riskImpactAnalysis": "<p><strong>Moderate</strong>, medium impact: every node restarts once, so capacity drops by one node at a time during the window. No data changes (updates with database scripts are excluded), so going back to the previous image is a full revert.</p>",
  "backoutPlan": "<ol><li>Set the deployment's image tag back to the one recorded in step 2.</li><li>Roll the nodes one at a time, as in step 4.</li><li>Confirm every node reports update level {{current_update_level}} again.</li></ol>",
  "testPlan": "<ul><li>Every node's startup log shows update level {{target_update_level}} and no errors.</li><li>The deployment's smoke tests pass (login to the consoles, one API invocation or integration flow end to end).</li><li>Error rate and latency dashboards show no change for 30 minutes.</li></ul>",
  "category": "software", "impact": "medium", "risk": "moderate",
  "rollbackDurationText": "45 minutes",
  "isPlanningVisibleToCustomers": true, "customerApprovalRequired": false, "customerReviewRequired": false,
  "parameters": [
    {"key": "deployment_name", "label": "Deployment", "type": "text", "required": true},
    {"key": "product", "label": "Product", "type": "select", "required": true, "options": ["API Manager", "Identity Server", "Micro Integrator", "API Control Plane", "Universal Gateway"]},
    {"key": "product_version", "label": "Product version", "type": "text", "required": true, "helpText": "e.g. 4.3.0"},
    {"key": "current_update_level", "label": "Current update level", "type": "number", "required": true},
    {"key": "target_update_level", "label": "Target update level", "type": "number", "required": true},
    {"key": "maintenance_date", "label": "Maintenance window date", "type": "date", "required": true},
    {"key": "customer_constraints", "label": "The customer's constraints on the window", "type": "textarea", "required": true, "helpText": "What the customer asked for about the window, e.g. no restart during business hours; write None when there are none"}
  ]
}}
JSON

cat >"$WORK/restart.json" <<'JSON'
{"evidenceMatch": "\\brestart(s|ed|ing)?\\b", "startFromEvidence": true,
 "evidenceNote": "<p>The cited evidence is earlier node restart work closed on this stack; the proposal was started from the first change request cited.</p>",
 "patch": {
  "name": "Restart an API Manager node",
  "rationale": "<p>Restarting one node of a clustered API Manager deployment, after draining it from the load balancer, is the first remedy for a node that leaks memory or stops picking up configuration. The cluster keeps serving from its other nodes, and the change is over in minutes.</p>",
  "subject": "Restart API Manager node {{node_name}} ({{node_profile}}) in {{deployment_name}}",
  "description": "<p>Restart the API Manager node <strong>{{node_name}}</strong> ({{node_profile}} profile) of {{deployment_name}}.</p><p>Symptom: {{symptom}}</p>",
  "justification": "<p>{{symptom}}</p>",
  "implementationPlan": "<ol><li>Confirm the cluster has another healthy node of the {{node_profile}} profile to take the traffic.</li><li>Take a thread dump and heap histogram of {{node_name}} for the problem record.</li><li>Remove {{node_name}} from the load balancer and wait {{drain_seconds}} seconds for in-flight requests to finish.</li><li>Restart the node (Kubernetes: delete the pod; VM: <code>systemctl restart wso2am</code>).</li><li>Wait for the health check to pass, then put the node back in the load balancer.</li></ol>",
  "riskImpactAnalysis": "<p><strong>Low.</strong> One node is out of rotation for a few minutes and the cluster keeps serving from the others.</p>",
  "backoutPlan": "<p>A restart cannot be undone. If the node does not come back healthy within 10 minutes, keep it out of the load balancer, raise an incident, and scale the {{node_profile}} profile up by one node to restore capacity.</p>",
  "testPlan": "<ul><li>The node's health check passes and it is back in the load balancer.</li><li>An API call routed to {{node_name}} returns 200.</li><li>The symptom is gone: {{symptom}}</li></ul>",
  "category": "software", "impact": "low", "risk": "low",
  "rollbackDurationText": "10 minutes",
  "isPlanningVisibleToCustomers": false, "customerApprovalRequired": false, "customerReviewRequired": false,
  "parameters": [
    {"key": "deployment_name", "label": "Deployment", "type": "text", "required": true},
    {"key": "node_name", "label": "Node (pod or host name)", "type": "text", "required": true},
    {"key": "node_profile", "label": "Node profile", "type": "select", "required": true, "options": ["All-in-one", "Gateway worker", "Control plane", "Traffic manager", "Key manager"]},
    {"key": "symptom", "label": "Symptom being fixed", "type": "textarea", "required": true},
    {"key": "drain_seconds", "label": "Drain time (seconds)", "type": "number", "required": true, "helpText": "At least the longest API timeout; 60 by default"}
  ]
}}
JSON

cat >"$WORK/allowlist.json" <<'JSON'
{"evidenceMatch": "\\b(allowlist|whitelist|ip range|cidr)\\b",
 "evidenceNote": "<p>The cited evidence is earlier allowlist work closed on this stack.</p>",
 "patch": {
  "name": "Add an IP range to a customer's gateway allowlist",
  "rationale": "<p>Customers regularly ask for a new office or partner IP range to be allowed through to their gateway. Adding one entry to the WAF / ingress allowlist only widens access for that range, is applied without a restart and is undone by deleting the entry.</p>",
  "subject": "Allow {{cidr}} on {{deployment_name}} ({{environment}})",
  "description": "<p>Add <code>{{cidr}}</code> to the gateway allowlist of <strong>{{deployment_name}}</strong> ({{environment}}), at priority {{rule_priority}}, as requested by {{requested_by}}. The entry is to be re-confirmed with the customer by {{review_on}}.</p>",
  "justification": "<p>{{business_reason}}</p>",
  "implementationPlan": "<ol><li>Confirm the request came from a registered contact of the customer ({{requested_by}}) and that {{cidr}} is not a private or overly broad range (no wider than /16).</li><li>Add the entry to the deployment's allowlist (WAF IP set or ingress <code>whitelist-source-range</code>) at priority {{rule_priority}}, with a comment naming this change request.</li><li>Apply the change and confirm the WAF / ingress controller reports it as in effect.</li></ol>",
  "riskImpactAnalysis": "<p><strong>Low.</strong> The change only allows one more source range; existing traffic is unaffected. The range is checked for breadth before it is added.</p>",
  "backoutPlan": "<ol><li>Delete the entry for {{cidr}} from the allowlist.</li><li>Apply and confirm the WAF / ingress controller reports it as in effect.</li></ol>",
  "testPlan": "<ul><li>A request from {{cidr}} (the customer confirms, or a probe from that range) reaches the gateway.</li><li>A request from a range not on the list is still refused.</li></ul>",
  "category": "network", "impact": "low", "risk": "low",
  "rollbackDurationText": "10 minutes",
  "isPlanningVisibleToCustomers": true, "customerApprovalRequired": false, "customerReviewRequired": false,
  "parameters": [
    {"key": "deployment_name", "label": "Deployment", "type": "text", "required": true},
    {"key": "environment", "label": "Environment", "type": "select", "required": true, "options": ["Development", "Staging", "Production"]},
    {"key": "cidr", "label": "IP range (CIDR)", "type": "text", "required": true, "helpText": "e.g. 203.0.113.0/24"},
    {"key": "rule_priority", "label": "Rule priority", "type": "number", "required": true, "helpText": "Position in the WAF IP set; lower runs first"},
    {"key": "requested_by", "label": "Requested by (customer contact)", "type": "text", "required": true},
    {"key": "review_on", "label": "Re-confirm with the customer by", "type": "date", "required": true},
    {"key": "business_reason", "label": "Business reason", "type": "textarea", "required": true}
  ]
}}
JSON

# The proposal the CAB rejects: deleting data is not a pre-authorisable change.
cat >"$WORK/purge.json" <<'JSON'
{"evidenceMatch": "\\b(purge|analytics (data|tables?))\\b",
 "evidenceNote": "<p>The cited evidence is earlier analytics purge work closed on this stack.</p>",
 "patch": {
  "name": "Purge API Manager analytics data older than a retention period",
  "rationale": "<p>Analytics tables on customer-hosted API Manager databases grow without bound and are cleaned up by hand several times a month. The delete is the same SQL every time, only the cut-off date and the database differ.</p>",
  "subject": "Purge analytics data older than {{cutoff_date}} on {{database_host}}",
  "description": "<p>Delete API Manager analytics events older than {{cutoff_date}} from the database on {{database_host}}, in batches of {{batch_size}} rows.</p>",
  "justification": "<p>The analytics tables are approaching the database volume's capacity.</p>",
  "implementationPlan": "<ol><li>Take a snapshot of the database on {{database_host}}.</li><li>Delete in batches: <code>DELETE FROM AM_USAGE_UPLOADED_FILES WHERE FILE_TIMESTAMP &lt; '{{cutoff_date}}' LIMIT {{batch_size}}</code>, repeated until no rows are left.</li><li>Run <code>VACUUM</code> / <code>OPTIMIZE TABLE</code> on the table.</li></ol>",
  "riskImpactAnalysis": "<p>Moderate: the delete holds row locks while it runs, and deleted rows are gone.</p>",
  "backoutPlan": "<p>Restore the snapshot taken in step 1.</p>",
  "testPlan": "<p>The row count older than {{cutoff_date}} is zero and the API Manager analytics pages still load.</p>",
  "category": "system_software", "impact": "low", "risk": "moderate",
  "rollbackDurationText": "2 hours",
  "isPlanningVisibleToCustomers": false, "customerApprovalRequired": false, "customerReviewRequired": false,
  "parameters": [
    {"key": "database_host", "label": "Database host", "type": "text", "required": true},
    {"key": "cutoff_date", "label": "Delete data older than", "type": "date", "required": true},
    {"key": "batch_size", "label": "Rows per batch", "type": "number", "required": true}
  ]
}}
JSON

# The proposal left in CAB review.
cat >"$WORK/memory.json" <<'JSON'
{"evidenceMatch": "\\b(memory limit|oom|out of memory)\\b",
 "evidenceNote": "<p>The cited evidence is earlier memory limit work closed on this stack.</p>",
 "patch": {
  "name": "Raise the memory limit of a Choreo component",
  "rationale": "<p>Components that are OOM-killed under normal load need their memory limit raised; it is the same one-field change on the component's deployment settings every time, applied by a rolling redeploy, and reverted by setting the old value back. Limited to at most doubling the limit and to the tier's ceiling.</p>",
  "subject": "Raise the memory limit of {{component}} ({{project}}) to {{new_limit_mib}} MiB",
  "description": "<p>Raise the memory limit of the Choreo component <strong>{{component}}</strong> in project {{project}} ({{environment}}) from {{current_limit_mib}} MiB to {{new_limit_mib}} MiB.</p><p>Evidence of the memory pressure: {{evidence}}</p>",
  "justification": "<p>The component is being OOM-killed under normal load, which restarts it and drops in-flight requests.</p>",
  "implementationPlan": "<ol><li>Confirm {{new_limit_mib}} MiB is at most twice {{current_limit_mib}} MiB and within the organisation's tier ceiling. If not, stop: raise a normal change.</li><li>In the component's DevOps settings for {{environment}}, set the memory limit to {{new_limit_mib}} MiB.</li><li>Redeploy and wait for the rolling update to finish.</li></ol>",
  "riskImpactAnalysis": "<p><strong>Low.</strong> A rolling redeploy keeps the component serving; only its resource limit changes.</p>",
  "backoutPlan": "<ol><li>Set the memory limit back to {{current_limit_mib}} MiB.</li><li>Redeploy and wait for the rolling update to finish.</li></ol>",
  "testPlan": "<ul><li>The component's pods run with the new limit (<code>kubectl describe pod</code> or the Choreo observability view).</li><li>No OOM kill for 24 hours.</li></ul>",
  "category": "cloud_computing", "impact": "low", "risk": "low",
  "rollbackDurationText": "15 minutes",
  "isPlanningVisibleToCustomers": true, "customerApprovalRequired": false, "customerReviewRequired": false,
  "parameters": [
    {"key": "project", "label": "Choreo project", "type": "text", "required": true},
    {"key": "component", "label": "Component", "type": "text", "required": true},
    {"key": "environment", "label": "Environment", "type": "select", "required": true, "options": ["Development", "Production"]},
    {"key": "current_limit_mib", "label": "Current limit (MiB)", "type": "number", "required": true},
    {"key": "new_limit_mib", "label": "New limit (MiB)", "type": "number", "required": true},
    {"key": "evidence", "label": "Evidence of memory pressure", "type": "textarea", "required": true, "helpText": "OOM kill events, memory graph links"}
  ]
}}
JSON

# The proposal left as a draft: started, not finished (no backout or test plan yet).
cat >"$WORK/debug-logging.json" <<'JSON'
{"evidenceMatch": "\\bdebug log(s|ging)?\\b",
 "evidenceNote": "<p>The cited evidence is earlier debug logging work closed on this stack.</p>",
 "patch": {
  "name": "Enable debug logging on a Micro Integrator node for a fixed period",
  "rationale": "<p>Support often needs debug logs for one integration on one node for an hour or two. Draft: the backout and test plans are still to be written.</p>",
  "subject": "Enable debug logging for {{logger}} on {{node_name}} for {{duration_minutes}} minutes",
  "implementationPlan": "<ol><li>Set the log level of <code>{{logger}}</code> to DEBUG on {{node_name}} through the Micro Integrator management API.</li><li>Set a reminder to put it back after {{duration_minutes}} minutes.</li></ol>",
  "category": "software", "impact": "low", "risk": "low",
  "parameters": [
    {"key": "node_name", "label": "Node", "type": "text", "required": true},
    {"key": "logger", "label": "Logger (class or package)", "type": "text", "required": true},
    {"key": "duration_minutes", "label": "For how long (minutes)", "type": "number", "required": true}
  ]
}}
JSON

TEMPLATE_SPECS=(tls scale update-level restart allowlist)
PROPOSAL_SPECS=(tls scale update-level restart allowlist purge memory debug-logging)

# ===========================================================================
# --reset: delete what an earlier run left (see the header), LOCAL ONLY.

psql_local() {
  if [ -n "${PG_CONTAINER:-}" ]; then
    docker exec -i "$PG_CONTAINER" psql -U postgres -d csm_platform -v ON_ERROR_STOP=1 -X -q "$@"
  elif docker compose version >/dev/null 2>&1; then
    docker compose -f "$REPO_ROOT/docker-compose.yml" exec -T postgres psql -U postgres -d csm_platform -v ON_ERROR_STOP=1 -X -q "$@"
  else
    docker-compose -f "$REPO_ROOT/docker-compose.yml" exec -T postgres psql -U postgres -d csm_platform -v ON_ERROR_STOP=1 -X -q "$@"
  fi
}

reset_seed() {
  local template_names proposal_names
  template_names="$(for s in "${TEMPLATE_SPECS[@]}"; do jq -r .patch.name "$WORK/$s.json"; done | jq -Rsc 'split("\n") | map(select(. != ""))')"
  proposal_names="$(for s in "${PROPOSAL_SPECS[@]}"; do jq -r .patch.name "$WORK/$s.json"; done | jq -Rsc 'split("\n") | map(select(. != ""))')"
  log "Reset: deleting the rows an earlier run of this script left (local Postgres)"
  psql_local -v proposer="$PROPOSER" -v template_names="$template_names" -v proposal_names="$proposal_names" <<'SQL'
\pset footer off
BEGIN;

-- The seed's templates: this script's names, created by approving a NEW proposal bob raised.
CREATE TEMP TABLE reset_template ON COMMIT DROP AS
SELECT t.id, t.number, t.name
  FROM standard_change_template t
 WHERE t.name IN (SELECT jsonb_array_elements_text(:'template_names'::jsonb))
   AND EXISTS (SELECT 1 FROM standard_change_proposal p JOIN work_item w ON w.id = p.id
                WHERE p.template_id = t.id AND p.kind = 'NEW' AND p.state = 'APPROVED'
                  AND w.created_by = :'proposer');

-- The seed's proposals (bob's, by name or about the seed's templates), and the
-- leftover never-named drafts.
CREATE TEMP TABLE reset_proposal ON COMMIT DROP AS
SELECT w.id, w.number, w.subject, p.kind, p.state, w.created_by
  FROM work_item w JOIN standard_change_proposal p ON p.id = w.id
 WHERE w.type = 'STANDARD_CHANGE_PROPOSAL'
   AND ((w.created_by = :'proposer'
         AND (w.subject IN (SELECT jsonb_array_elements_text(:'proposal_names'::jsonb))
              OR p.template_id IN (SELECT id FROM reset_template)))
        OR (p.kind = 'NEW' AND p.state = 'DRAFT' AND w.subject = 'Untitled standard change'));

-- Every change request raised from the seed's templates (they hold the version
-- ON DELETE RESTRICT).
CREATE TEMP TABLE reset_change_request ON COMMIT DROP AS
SELECT w.id, w.number, w.subject, cr.state::text AS state
  FROM change_request cr
  JOIN work_item w ON w.id = cr.id
  JOIN standard_change_template_version v ON v.id = cr.standard_change_template_version_id
 WHERE v.template_id IN (SELECT id FROM reset_template);

SELECT 'template' AS deleting, number, name AS what FROM reset_template
UNION ALL
SELECT 'proposal', number, lower(kind) || ' ' || lower(state) || ', ' || created_by || ': ' || subject FROM reset_proposal
UNION ALL
SELECT 'change request', number, lower(state) || ': ' || subject FROM reset_change_request
ORDER BY 1 DESC, 2;

DELETE FROM event_outbox WHERE entity_id IN (SELECT id FROM reset_change_request UNION SELECT id FROM reset_proposal);
DELETE FROM event_publish_failures WHERE entity_id IN (SELECT id::text FROM reset_change_request UNION SELECT id::text FROM reset_proposal);
DELETE FROM sn_writeback_failures WHERE entity_id IN (SELECT id FROM reset_change_request UNION SELECT id FROM reset_proposal);
-- Work items: their change_request / standard_change_proposal rows, approval
-- stages and approvers, evidence, comments and activity cascade.
DELETE FROM work_item WHERE id IN (SELECT id FROM reset_change_request);
DELETE FROM work_item WHERE id IN (SELECT id FROM reset_proposal);
-- Templates: their versions cascade.
DELETE FROM standard_change_template WHERE id IN (SELECT id FROM reset_template);

-- The number series back to the highest number still in use.
SELECT setval('std_number_seq', COALESCE(MAX(substring(number FROM 4)::bigint), 0) + 1, false) AS next_std
  FROM standard_change_template WHERE number ~ '^STD[0-9]+$';
SELECT setval('scp_number_seq', COALESCE(MAX(substring(number FROM 4)::bigint), 0) + 1, false) AS next_scp
  FROM work_item WHERE type = 'STANDARD_CHANGE_PROPOSAL' AND number ~ '^SCP[0-9]+$';

COMMIT;
SQL
  log ""
}

[ "$RESET" = "yes" ] && reset_seed

# ===========================================================================
# Run

TOK_BOB="$(id_token "$PROPOSER")"
TOK_CAROL="$(id_token "$CAB_DECIDER")"
TOK_ALICE="$(id_token "$ENGINEER")"

# The engineer's own user id: "Requested by" on the change requests she raises,
# as the portal's create form sends it.
must "read the engineer" GET /users/me "$TOK_ALICE"
ENGINEER_ID="$(printf '%s' "$BODY" | jq -er '.id // .user.id')" || die "no user id for $ENGINEER"

load_closed_change_requests

log "Seeding standard changes on $ES_URL"
log "  proposer: $PROPOSER   CAB: $CAB_DECIDER   engineer: $ENGINEER"
log "  closed change requests that may serve as evidence: $(printf '%s' "$CLOSED_CRS" | jq length)"
log ""

log "Templates"
ensure_new_proposal "$WORK/tls.json" approved
ensure_new_proposal "$WORK/scale.json" approved
ensure_new_proposal "$WORK/update-level.json" approved
ensure_new_proposal "$WORK/restart.json" approved
ensure_new_proposal "$WORK/allowlist.json" approved
log ""

SCALE="Scale a Kubernetes deployment's replicas"
log "Version 1 of the scaling template in use (closed, each with a deviation)"
SCALE_EVIDENCE="[]"
ensure_change_request "$SCALE" \
  "Scale apim/apim-gateway on examplecorp-prod-eks to 6 replicas" \
  '{"cluster":"examplecorp-prod-eks","namespace":"apim","deployment":"apim-gateway","current_replicas":"3","target_replicas":"6","reason":"Example Corp load-tests its new mobile banking API on 14 October at twice the usual peak.\nThe gateway needs twice its capacity for the test."}' \
  closed "2026-10-14 01:30:00" "2026-10-14 02:00:00" \
  "kubectl scale took the deployment to 6 replicas, but the HorizontalPodAutoscaler apim-gateway (min 3, max 8) set it back to 3 within a minute. Raised the autoscaler's minReplicas to 6 instead (kubectl patch hpa apim-gateway), which this template's plan does not cover, and set it back to 3 after the test." \
  1
[ -z "$CR_ID" ] || SCALE_EVIDENCE="$(jq -nc --argjson a "$SCALE_EVIDENCE" --arg i "$CR_ID" '$a + [$i]')"
ensure_change_request "$SCALE" \
  "Scale integration/mi-runtime on examplecorp-prod-eks to 4 replicas" \
  '{"cluster":"examplecorp-prod-eks","namespace":"integration","deployment":"mi-runtime","current_replicas":"2","target_replicas":"4","reason":"Month-end batch integrations for Example Corp run from 24 to 26 October.\nLast month they queued for hours on two Micro Integrator replicas."}' \
  closed "2026-10-23 22:00:00" "2026-10-23 22:30:00" \
  "The HorizontalPodAutoscaler mi-runtime (min 2, max 6) scaled the deployment back to 2 replicas within a minute of kubectl scale. Raised its minReplicas to 4 instead, outside the plan, and back to 2 on 27 October." \
  1
[ -z "$CR_ID" ] || SCALE_EVIDENCE="$(jq -nc --argjson a "$SCALE_EVIDENCE" --arg i "$CR_ID" '$a + [$i]')"
log ""

log "Template lifecycle"
ensure_modified "$SCALE" "$WORK/scale-v2.json" "$SCALE_EVIDENCE"
ensure_suspended "Restart an API Manager node" \
  "A Traffic Manager restart on a clustered deployment dropped its throttling counters. The plan is being revised to exclude the Traffic Manager profile; raise node restarts as normal changes until then."
log ""

log "Proposals in other states"
ensure_new_proposal "$WORK/purge.json" rejected \
  "Rejected: deleting data cannot be pre-authorised. The backout plan depends on restoring a snapshot that is not tested per customer, and a mistaken cut-off date loses data for good. Keep raising these as normal changes."
ensure_new_proposal "$WORK/memory.json" cab_review
ensure_new_proposal "$WORK/debug-logging.json" draft
log ""

log "Change requests raised from templates"
ensure_change_request "Add an IP range to a customer's gateway allowlist" \
  "Allow 203.0.113.0/24 on examplecorp-prod-apim (Production)" \
  '{"deployment_name":"examplecorp-prod-apim","environment":"Production","cidr":"203.0.113.0/24","rule_priority":"120","requested_by":"dave.mendis@examplecorp.com","review_on":"2027-04-30","business_reason":"Example Corp opened a new office in Colombo.\nIts egress range needs to reach the production gateway."}' \
  new "2026-11-10 03:00:00" "2026-11-10 04:00:00"
ensure_change_request "Rotate the TLS certificate on an API gateway" \
  "Rotate the TLS certificate on gw.examplecorp.com (Production)" \
  '{"gateway_host":"gw.examplecorp.com","environment":"Production","certificate_issuer":"DigiCert","current_expiry":"2026-11-30","secret_path":"kv/examplecorp/prod/gateway-tls","subject_alt_names":"gw.examplecorp.com\napi.examplecorp.com","max_unavailable":"1"}' \
  scheduled "2026-11-12 02:00:00" "2026-11-12 03:00:00"
ensure_change_request "Apply a WSO2 product update level to a deployment" \
  "Apply API Manager 4.3.0 update level 42 to examplecorp-prod-apim" \
  '{"deployment_name":"examplecorp-prod-apim","product":"API Manager","product_version":"4.3.0","current_update_level":"37","target_update_level":"42","maintenance_date":"2026-10-27","customer_constraints":"No restart between 08:00 and 20:00 Colombo time; avoid the first business day of the month."}' \
  rollback "2026-10-27 01:00:00" "2026-10-27 03:00:00"
log ""

log "Catalogue now"
must "list templates" POST /standard-change-templates/search "$TOK_BOB" \
  '{"states":["active","suspended","retired"],"pagination":{"limit":50,"offset":0}}'
printf '%s' "$BODY" | jq -r '.templates[] | "  \(.number)  v\(.currentVersion.version)  \(.state)\(if .reviewRequired then "  REVIEW NEEDED" else "" end)  used \(.usage.total)x  \(.name)"'
must "list proposals" POST /standard-change-proposals/search "$TOK_BOB" \
  '{"pagination":{"limit":50,"offset":0}}'
printf '%s' "$BODY" | jq -r '.proposals[] | "  \(.number)  \(.kind)  \(.state)  \(.name)"'
log ""
log "Done: $CREATED created, $WRITES write request(s) in all."
