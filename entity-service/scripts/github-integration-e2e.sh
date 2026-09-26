#!/usr/bin/env bash
# End-to-end test of the GitHub integration against a real database and a real
# repository. Exercises every trigger and every webhook path, then puts the
# database back exactly as it found it.
#
#   ./github-integration-e2e.sh
#
# Reads its configuration from an env file (default ./staging.env, override with
# ENV_FILE). Required: DB_*, GITHUB_TOKEN, GITHUB_WEBHOOK_SECRET, GITHUB_OWNER,
# GITHUB_REPO, CSM_PORTAL_BASE_URL, TEST_ACCOUNT_NAME, TEST_CASE_ID.
#
# SAFE BY CONSTRUCTION. Two gates must BOTH be open for any trigger to enqueue:
# the account needs an active row in account_github_repo, and the case needs a
# github_issue_number. This script opens both for one account and one case, and
# closes them again in the teardown trap -- including on failure. Every row it
# writes is recorded and reverted; nothing else in the database can reach GitHub
# while it runs.
set -euo pipefail
export PATH="/opt/homebrew/bin:/opt/homebrew/opt/libpq/bin:$PATH"

ENV_FILE="${ENV_FILE:-$(dirname "$0")/staging.env}"
[ -f "$ENV_FILE" ] || { echo "no env file at $ENV_FILE" >&2; exit 2; }
set -a; . "$ENV_FILE"; set +a

for v in DB_HOST DB_PORT DB_USER DB_PASSWORD DB_NAME GITHUB_TOKEN \
         GITHUB_WEBHOOK_SECRET GITHUB_OWNER GITHUB_REPO CSM_PORTAL_BASE_URL \
         TEST_ACCOUNT_NAME TEST_CASE_ID; do
  [ -n "${!v:-}" ] || { echo "missing required setting: $v" >&2; exit 2; }
done
export PGPASSWORD="$DB_PASSWORD" PGCONNECT_TIMEOUT=15
psql() { command psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -tA "$@"; }
gh_api() { curl -sS -H "Authorization: Bearer $GITHUB_TOKEN" "$@"; }
say() { printf '\n\033[1m%s\033[0m\n' "$*"; }
ok()  { printf '  \033[32mPASS\033[0m  %s\n' "$*"; }
bad() { printf '  \033[31mFAIL\033[0m  %s\n' "$*"; FAILED=$((FAILED+1)); }
FAILED=0
WORK="$(mktemp -d)"

# ---------------------------------------------------------------- teardown
# A trap, not a tidy-up at the end: a failure midway must still close the two
# gates, or the next real change to this account would push to the test repo.
teardown() {
  local rc=$?
  say "Teardown"
  [ -n "${SVC_PID:-}" ] && kill "$SVC_PID" 2>/dev/null || true
  if [ -f "$WORK/restore.sql" ]; then
    psql -v ON_ERROR_STOP=1 -f "$WORK/restore.sql" >/dev/null 2>&1 \
      && echo "  original rows restored" || echo "  WARNING: restore failed, see $WORK/restore.sql"
  fi
  psql -c "DELETE FROM account_github_repo WHERE created_by='e2e-test';" >/dev/null 2>&1 || true
  psql -c "UPDATE \"case\" SET github_issue_number=NULL WHERE id='$TEST_CASE_ID';" >/dev/null 2>&1 || true
  psql -c "DELETE FROM comment WHERE created_by='e2e-test@wso2.com';" >/dev/null 2>&1 || true
  local gates
  gates=$(psql -c "SELECT (SELECT count(*) FROM account_github_repo)+(SELECT count(*) FROM \"case\" WHERE github_issue_number IS NOT NULL);")
  [ "$gates" = "0" ] && echo "  both gates closed — nothing can reach GitHub" \
                     || echo "  WARNING: $gates gate row(s) still open"
  exit $rc
}
trap teardown EXIT

# ---------------------------------------------------------------- preflight
say "Preflight"
psql -c "SELECT 1" >/dev/null && ok "database reachable"
gh_api "https://api.github.com/repos/$GITHUB_OWNER/$GITHUB_REPO" | grep -q '"full_name"' \
  && ok "repository $GITHUB_OWNER/$GITHUB_REPO reachable" || { bad "cannot reach the repository"; exit 1; }
TRIGGERS=$(psql -c "SELECT count(*) FROM pg_trigger WHERE NOT tgisinternal AND tgname LIKE '%github%';")
[ "$TRIGGERS" = "4" ] && ok "all 4 triggers installed" \
  || bad "expected 4 github triggers, found $TRIGGERS — migrations 000067-000070 applied?"

# ---------------------------------------------------------------- arm
say "Arming (opening both gates for one account and one case)"
ISSUE=$(gh_api -X POST "https://api.github.com/repos/$GITHUB_OWNER/$GITHUB_REPO/issues" \
  -d '{"title":"[e2e] automated integration test","body":"Created by github-integration-e2e.sh. Safe to delete.","labels":["Type/ChangeRequest"]}' \
  | python3 -c "import json,sys; print(json.load(sys.stdin)['number'])")
ok "created issue #$ISSUE"

# Capture what we are about to change, as executable SQL.
psql <<SQL > "$WORK/restore.sql"
SELECT format('UPDATE work_item SET assigned_to_id=%L WHERE id=%L;', assigned_to_id, id)
FROM work_item WHERE id='$TEST_CASE_ID' OR parent_id='$TEST_CASE_ID';
SELECT format('UPDATE "case" SET state=%L, close_notes=%L WHERE id=%L;', state, close_notes, id)
FROM "case" WHERE id='$TEST_CASE_ID';
SELECT format('UPDATE change_request SET state=%L WHERE id=%L;', state, id)
FROM change_request WHERE id IN (SELECT id FROM work_item WHERE parent_id='$TEST_CASE_ID');
SQL
ok "captured $(grep -c UPDATE "$WORK/restore.sql") rows for restore"

psql -v ON_ERROR_STOP=1 <<SQL >/dev/null
INSERT INTO account_github_repo (id, created_by, updated_by, account_id, owner, repository, credential_ref)
SELECT gen_random_uuid(), 'e2e-test', 'e2e-test', a.id, '$GITHUB_OWNER', '$GITHUB_REPO', 'github-test-pat'
FROM account a WHERE a.name = '$TEST_ACCOUNT_NAME'
ON CONFLICT (account_id) DO UPDATE SET owner=EXCLUDED.owner, repository=EXCLUDED.repository, created_by='e2e-test';
UPDATE "case" SET github_issue_number = $ISSUE WHERE id = '$TEST_CASE_ID';
SQL
ok "mapped $TEST_ACCOUNT_NAME -> $GITHUB_OWNER/$GITHUB_REPO, case linked to #$ISSUE"

BEFORE=$(psql -c "SELECT count(*) FROM github_outbound_queue;")
[ "$BEFORE" = "$(psql -c 'SELECT count(*) FROM github_outbound_queue;')" ] && ok "arming alone enqueued nothing"

# ---------------------------------------------------------------- outbound
say "Outbound: firing all five triggers"
CR=$(psql -c "SELECT id FROM work_item WHERE parent_id='$TEST_CASE_ID' AND type='CHANGE_REQUEST' LIMIT 1;")
[ -n "$CR" ] || { bad "the test case has no change request under it"; exit 1; }
psql -v ON_ERROR_STOP=1 <<SQL >/dev/null
CREATE TEMP TABLE picked AS SELECT id FROM "user" WHERE name IS NOT NULL AND TRIM(name)<>'' LIMIT 1;
UPDATE change_request SET state='AUTHORIZE' WHERE id='$CR';
UPDATE work_item SET assigned_to_id=(SELECT id FROM picked) WHERE id='$CR';
UPDATE work_item SET assigned_to_id=(SELECT id FROM picked) WHERE id='$TEST_CASE_ID';
INSERT INTO comment (id, created_on, created_by, work_item_id, content, type)
VALUES (gen_random_uuid(), now(), 'e2e-test@wso2.com', '$TEST_CASE_ID',
        'Scheduling the upgrade for Friday 22:00 UTC.', 'COMMENT');
UPDATE "case" SET state='CLOSED', close_notes='Completed and verified.' WHERE id='$TEST_CASE_ID';
SQL
for e in cr_updated case_assigned comment_added case_closed; do
  n=$(psql -c "SELECT count(*) FROM github_outbound_queue WHERE event='$e' AND issue_number=$ISSUE;")
  [ "$n" -ge 1 ] && ok "$e enqueued ($n)" || bad "$e did not enqueue"
done
# Only the fields that get RENDERED. payload.commentId is a UUID by design and
# never reaches the issue, so scanning the whole payload reports a bug that is
# not there. What must never be a UUID is the assignee, which is rendered into
# a comment a customer reads.
n=$(psql -c "SELECT count(*) FROM github_outbound_queue
             WHERE issue_number=$ISSUE
               AND (payload->>'assignedTo' ~ '^[0-9a-f]{8}-[0-9a-f]{4}-'
                 OR payload #>> '{changes,assigned_to_id,to}'   ~ '^[0-9a-f]{8}-[0-9a-f]{4}-'
                 OR payload #>> '{changes,assigned_to_id,from}' ~ '^[0-9a-f]{8}-[0-9a-f]{4}-');")
[ "$n" = "0" ] && ok "assignees resolved to names, not UUIDs" || bad "$n payload(s) render a raw UUID as the assignee"

# ---------------------------------------------------------------- deliver
say "Delivering (running the service)"
BIN="${ENTITY_SERVICE_BIN:-}"
if [ -z "$BIN" ]; then
  BIN="$WORK/entity-service"
  (cd "$(dirname "$0")/.." && go build -o "$BIN" ./cmd/api)
fi
"$BIN" > "$WORK/service.log" 2>&1 &
SVC_PID=$!
for _ in $(seq 1 20); do
  sleep 2
  [ "$(psql -c "SELECT count(*) FROM github_outbound_queue WHERE issue_number=$ISSUE AND status='PENDING';")" = "0" ] && break
done
PEND=$(psql -c "SELECT count(*) FROM github_outbound_queue WHERE issue_number=$ISSUE AND status<>'DELIVERED';")
[ "$PEND" = "0" ] && ok "every queued row delivered" \
  || bad "$PEND row(s) undelivered: $(psql -c "SELECT DISTINCT COALESCE(last_error,'(no error)') FROM github_outbound_queue WHERE issue_number=$ISSUE AND status<>'DELIVERED';")"

say "Verifying what landed on the issue"
gh_api "https://api.github.com/repos/$GITHUB_OWNER/$GITHUB_REPO/issues/$ISSUE/comments" > "$WORK/comments.json"
COUNT=$(python3 -c "import json;print(len(json.load(open('$WORK/comments.json'))))")
[ "$COUNT" -ge 5 ] && ok "$COUNT comments posted" || bad "only $COUNT comments posted, want 5"
BODIES=$(python3 -c "import json;print('\n'.join(c['body'] for c in json.load(open('$WORK/comments.json'))))")
grep -q "$CSM_PORTAL_BASE_URL" <<<"$BODIES" && ok "comments link to the configured portal" \
  || bad "no comment links to CSM_PORTAL_BASE_URL ($CSM_PORTAL_BASE_URL)"
grep -qE '/cases/[0-9a-f-]+' <<<"$BODIES" && ok "case events link to the case route" || bad "case events do not link to /cases/"
grep -qiE '@[a-z0-9.-]+\.(com|org|net)' <<<"$BODIES" && bad "an email address reached a public issue" \
  || ok "no email address in any comment"
# A UUID inside a portal link is expected; one as a rendered VALUE is the bug.
grep -qE 'Assigned to\*\*: .*[0-9a-f]{8}-[0-9a-f]{4}-' <<<"$BODIES" \
  && bad "an assignee rendered as a UUID on the issue" || ok "no UUID rendered as a value"

# ---------------------------------------------------------------- inbound
say "Inbound: webhook paths"
WH_URL="${WEBHOOK_URL:-http://localhost:${SERVER_PORT:-18590}/webhooks/github}"
cat > "$WORK/hook.py" <<'PY'
import hashlib, hmac, json, os, sys, urllib.request, urllib.error
body = open(sys.argv[1],'rb').read()
sig  = "sha256=" + hmac.new(os.environ["GITHUB_WEBHOOK_SECRET"].encode(), body, hashlib.sha256).hexdigest()
if len(sys.argv) > 4 and sys.argv[4] == "badsig": sig = "sha256=" + "0"*64
req = urllib.request.Request(os.environ["WH_URL"], data=body, method="POST", headers={
    "Content-Type":"application/json", "X-Hub-Signature-256":sig,
    "X-GitHub-Event":sys.argv[2], "X-GitHub-Delivery":sys.argv[3]})
try:
    with urllib.request.urlopen(req, timeout=30) as r: print(r.status, r.read().decode().strip())
except urllib.error.HTTPError as e: print(e.code, e.read().decode().strip())
PY
NEW=$(gh_api -X POST "https://api.github.com/repos/$GITHUB_OWNER/$GITHUB_REPO/issues" \
  -d '{"title":"[e2e] inbound change request","body":"Created by the e2e script.","labels":["Type/ChangeRequest","CRType/Normal","CRScope/Infrastructure"]}' \
  | python3 -c "import json,sys; print(json.load(sys.stdin)['number'])")
python3 - "$NEW" "$GITHUB_OWNER" "$GITHUB_REPO" > "$WORK/wh.json" <<'PY'
import json,sys
n,o,r = int(sys.argv[1]), sys.argv[2], sys.argv[3]
print(json.dumps({"action":"labeled",
 "issue":{"number":n,"title":"[e2e] inbound change request","body":"Created by the e2e script.","state":"open",
   "html_url":f"https://github.com/{o}/{r}/issues/{n}",
   "labels":[{"name":"Type/ChangeRequest"},{"name":"CRType/Normal"},{"name":"CRScope/Infrastructure"}]},
 "label":{"name":"Type/ChangeRequest"},
 "repository":{"name":r,"owner":{"login":o}}, "sender":{"login":"a-human"}}))
PY
export WH_URL
R1=$(python3 "$WORK/hook.py" "$WORK/wh.json" issues "e2e-$ISSUE-1")
grep -q '^200' <<<"$R1" && ok "first delivery accepted" || bad "first delivery: $R1"
R2=$(python3 "$WORK/hook.py" "$WORK/wh.json" issues "e2e-$ISSUE-1")
grep -q 'duplicate delivery' <<<"$R2" && ok "replay refused as a duplicate" || bad "replay was reprocessed: $R2"
R3=$(python3 "$WORK/hook.py" "$WORK/wh.json" issues "e2e-$ISSUE-bad" badsig)
grep -q '^401' <<<"$R3" && ok "bad signature rejected (401)" || bad "bad signature: $R3"
grep -qi 'sha256=[0-9a-f]' <<<"$R3" && bad "the 401 body leaked a signature" || ok "401 leaks nothing"
python3 -c "
import json; d=json.load(open('$WORK/wh.json')); d['sender']['login']='${GITHUB_INTEGRATION_LOGIN:-csm-sync-bot}'
json.dump(d, open('$WORK/wh-loop.json','w'))"
R4=$(python3 "$WORK/hook.py" "$WORK/wh-loop.json" issues "e2e-$ISSUE-loop")
grep -q 'integration account' <<<"$R4" && ok "loop guard dropped our own event" || bad "loop guard: $R4"
CRN=$(psql -c "SELECT count(*) FROM change_request WHERE git_reference='https://github.com/$GITHUB_OWNER/$GITHUB_REPO/issues/$NEW';")
[ "$CRN" = "1" ] && ok "exactly one change request created for issue #$NEW" || bad "$CRN change requests created, want 1"

say "Result"
if [ "$FAILED" = "0" ]; then
  echo "  ALL CHECKS PASSED"
  echo "  outbound: https://github.com/$GITHUB_OWNER/$GITHUB_REPO/issues/$ISSUE"
  echo "  inbound : https://github.com/$GITHUB_OWNER/$GITHUB_REPO/issues/$NEW"
else
  echo "  $FAILED CHECK(S) FAILED — service log: $WORK/service.log"
fi
exit "$FAILED"
