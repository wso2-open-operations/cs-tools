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
# One-shot, run-once script: moves every migration that defines or supports
# an RLS policy (CREATE POLICY / ALTER POLICY / ENABLE|FORCE ROW LEVEL
# SECURITY, plus the is_project_member()-family functions that exist only to
# serve those policies) into a dedicated 6-digit number space starting at
# 100001, preserving their exact current relative order (dependency-checked
# by hand against every cross-reference these files make to each other).
# Content is otherwise untouched -- every file was already made idempotent
# (DROP POLICY IF EXISTS / safe-to-re-run by construction) before this script
# was written, specifically so a rename can never turn a database that
# already has these policies applied under the old filename into a hard
# failure: at worst it is a harmless, idempotent re-apply under the new name.
#
# Excluded on purpose:
#   - 0191_change_request_project_links.sql: creates the tables its own RLS
#     policies protect in the SAME file (plus four ALTER TYPE ADD VALUE
#     statements) -- a mixed schema+RLS migration, not pure RLS. This script
#     leaves it renumbering-untouched (it stays 0191, schema-only). ITS RLS
#     PORTION WAS LATER SPLIT OUT BY HAND, in a follow-up pass after this
#     script ran, into 100024_change_request_project_links_rls.sql -- see
#     that file's own header. That follow-up edit is not part of this script
#     and is not reproduced by re-running it.
#   - The legacy 000020-000105 stragglers (KB tables, cloud_status_events,
#     outage_communications): not RLS at all, and at least one of them
#     (000020-000027) has FK targets that never existed in this schema at
#     all, so renumbering it needs its own, separate dependency audit against
#     real database state -- see the conversation this script came out of.
#
# Run this from the entity-service directory, with a clean working tree
# (git status should show nothing else pending) so the diff this produces is
# reviewable on its own:
#
#   ./scripts/renumber_rls_migrations.sh
#
# It is NOT idempotent by itself -- it is meant to run exactly once. Running
# it again against an already-renumbered tree will fail outright on the first
# `git mv` (source file no longer exists at the old name), which is a safe,
# obvious failure mode, not a silent corruption -- but do not rely on that;
# this is a one-shot migration of the migrations.
set -euo pipefail
cd "$(dirname "$0")/.."

if [ -n "$(git status --porcelain)" ]; then
  echo "Working tree is not clean. Commit or stash first, so this script's own diff is reviewable on its own." >&2
  exit 1
fi

M=migrations

echo "== Renaming 23 files into the 100001+ RLS number space =="

git mv "$M/0085_announcement_visibility_rls.sql"                              "$M/100001_announcement_visibility_rls.sql"
git mv "$M/0141_case_escalation_rls.sql"                                      "$M/100002_case_escalation_rls.sql"
git mv "$M/0142_sla_rls.sql"                                                  "$M/100003_sla_rls.sql"
git mv "$M/0143_customer_call_rls.sql"                                        "$M/100004_customer_call_rls.sql"
git mv "$M/0144_time_card_rls.sql"                                            "$M/100005_time_card_rls.sql"
git mv "$M/0145_change_request_rls.sql"                                       "$M/100006_change_request_rls.sql"
git mv "$M/0146_conversation_rls.sql"                                         "$M/100007_conversation_rls.sql"
git mv "$M/0147_case_adjacent_rls.sql"                                        "$M/100008_case_adjacent_rls.sql"
git mv "$M/0148_incident_problem_deny_all_rls.sql"                            "$M/100009_incident_problem_deny_all_rls.sql"
git mv "$M/0149_announcement_security_fallback_no_contact.sql"                "$M/100010_announcement_security_fallback_no_contact.sql"
git mv "$M/0150_case_escalation_write_widen_to_project_members.sql"          "$M/100011_case_escalation_write_widen_to_project_members.sql"
git mv "$M/0151_case_like_extension_tables_rls.sql"                           "$M/100012_case_like_extension_tables_rls.sql"
git mv "$M/0152_is_project_member_cached_array.sql"                          "$M/100013_is_project_member_cached_array.sql"
git mv "$M/0153_remove_rls_sla_incident_problem.sql"                          "$M/100014_remove_rls_sla_incident_problem.sql"
git mv "$M/0154_rls_internal_check_initplan.sql"                              "$M/100015_rls_internal_check_initplan.sql"
git mv "$M/0175_announcement_child_table_visibility.sql"                      "$M/100016_announcement_child_table_visibility.sql"
git mv "$M/0176_deployment_rls.sql"                                           "$M/100017_deployment_rls.sql"
git mv "$M/0177_rls_policy_functions_parallel_safe.sql"                       "$M/100018_rls_policy_functions_parallel_safe.sql"
git mv "$M/0178_announcement_visibility_security_contacts_see_all.sql"        "$M/100019_announcement_visibility_security_contacts_see_all.sql"
git mv "$M/0190_conversation_customer_insert_rls.sql"                         "$M/100020_conversation_customer_insert_rls.sql"
git mv "$M/0190_rls_internal_write_policies_for_sync.sql"                     "$M/100021_rls_internal_write_policies_for_sync.sql"
git mv "$M/0191_comment_work_notes_internal_only.sql"                         "$M/100022_comment_work_notes_internal_only.sql"
git mv "$M/0205_change_request_deployment_internal_update_policy.sql"         "$M/100023_change_request_deployment_internal_update_policy.sql"

echo "== Updating cross-reference comments to the new numbers =="
# Every one of these was found by grepping the 23 files (plus everything that
# references them) for "migration NNNN" / "NNNN's own" / "see NNNN" style
# phrases, by hand, before this script was written -- not a blind global
# find-and-replace of a bare number, which would risk rewriting an unrelated
# coincidental digit sequence. Two deliberately NOT rewritten, because they
# refer to files THIS script does not rename:
#   - "migration 000084" in 100001 (refers to the actor stamp in
#     0128_project_admin_role_group.sql, a different, untouched migration)
#   - "migration 000087" in 100006 (refers to whatever created approval_stage
#     originally -- not 000087_cloud_status_outage_outbox.sql, and not
#     renamed by this script either way)
#   - "migration 0191" in 100023 (refers to 0191_change_request_project_links.sql,
#     excluded above -- stays 0191, unrenamed)
# GNU sed takes `-i[SUFFIX]` (no space, no argument for "no backup"); BSD/macOS
# sed requires the backup-suffix argument even when it's empty (`-i ''`), and
# treats a bare `-i` as consuming the next argument as that suffix instead of
# editing in place. Detect which one we have rather than hardcoding either -
# `sed --version` succeeds (and prints "GNU sed") only on GNU sed; BSD/macOS
# sed exits non-zero on an unrecognized `--version` flag.
if sed --version >/dev/null 2>&1; then
  sed_i() { sed -i "$@"; }        # GNU sed
else
  sed_i() { sed -i '' "$@"; }     # BSD/macOS sed
fi

sed_i -e "s/migration 000085/migration 100001/g" "$M/100002_case_escalation_rls.sql"

sed_i -e "s/migration 0141/migration 100002/g" "$M/100003_sla_rls.sql"
sed_i -e "s/migration 0141/migration 100002/g" "$M/100004_customer_call_rls.sql"
sed_i -e "s/migration 0141/migration 100002/g" "$M/100005_time_card_rls.sql"

sed_i -e "s/0144's own/100005's own/g" \
      -e "s/migration 0141/migration 100002/g" \
      -e "s/migrations 0142-0144/migrations 100003-100005/g" \
      "$M/100006_change_request_rls.sql"

sed_i -e "s/migration 0145/migration 100006/g" \
      -e "s/migration 0141/migration 100002/g" \
      "$M/100007_conversation_rls.sql"

sed_i -e "s/migration 0151/migration 100012/g" \
      -e "s/migration 0141/migration 100002/g" \
      -e "s/0145's own/100006's own/g" \
      -e "s/migration 0144/migration 100005/g" \
      "$M/100008_case_adjacent_rls.sql"

sed_i -e "s/migration 0141/migration 100002/g" "$M/100009_incident_problem_deny_all_rls.sql"

sed_i -e "s/migration 000085/migration 100001/g" \
      -e "s/000085's/100001's/g" \
      -e "s/migration 0141/migration 100002/g" \
      "$M/100010_announcement_security_fallback_no_contact.sql"

sed_i -e "s/0141's own/100002's own/g" "$M/100011_case_escalation_write_widen_to_project_members.sql"

sed_i -e "s/migration 0147/migration 100008/g" \
      -e "s/migration 0141/migration 100002/g" \
      "$M/100012_case_like_extension_tables_rls.sql"

sed_i -e "s/migrations 0142 and 0148/migrations 100003 and 100009/g" "$M/100014_remove_rls_sla_incident_problem.sql"

sed_i -e "s/migration 0149/migration 100010/g" "$M/100016_announcement_child_table_visibility.sql"

sed_i -e "s/migration 0154/migration 100015/g" "$M/100017_deployment_rls.sql"

sed_i -e "s/migration 0149/migration 100010/g" \
      -e "s/migration 000085/migration 100001/g" \
      -e "s/migration 0154/migration 100015/g" \
      -e "s/migration 0175/migration 100016/g" \
      "$M/100019_announcement_visibility_security_contacts_see_all.sql"

sed_i -e "s/migration 0147/migration 100008/g" \
      -e "s/migration 0154/migration 100015/g" \
      "$M/100020_conversation_customer_insert_rls.sql"

sed_i -e "s/migration 0154/migration 100015/g" "$M/100021_rls_internal_write_policies_for_sync.sql"

sed_i -e "s#0147/0175#100008/100016#g" \
      -e "s/see 0175/see 100016/g" \
      "$M/100022_comment_work_notes_internal_only.sql"

sed_i -e "s/migration 0190/migration 100021/g" \
      -e "s/migration 0154/migration 100015/g" \
      "$M/100023_change_request_deployment_internal_update_policy.sql"

echo "== Done. Review the diff (git diff --stat, git log --follow on a couple of files to confirm rename detection), then: =="
echo "   - update scripts/record_manually_applied_rls_migrations.sql to the new filenames (separate, already-prepared edit)"
echo "   - run entity-service's test suite"
echo "   - commit"
