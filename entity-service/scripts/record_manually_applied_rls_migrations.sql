-- Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com). All Rights Reserved.
--
-- This software is the property of WSO2 LLC. and its suppliers, if any.
-- Dissemination of any information or reproduction of any material contained
-- herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
-- You may not alter or remove any copyright or other notice from copies of this content.

-- Records the row-level-security migrations in csm_migration_applied_migration on a database where they were applied BY HAND
-- (psql / the RLS apply script) and therefore never got a tracking row.
--
-- Why: `make migrate` applies every migrations/*.sql file that has no tracking row, stops at the first failure, and skips everything after
-- it. On such a database the first untracked RLS file (100001_announcement_visibility_rls.sql, the renumbered form of what used to be
-- 000085_announcement_visibility_rls.up.sql -- see scripts/renumber_rls_migrations.sh) fails with "policy ... already exists",
-- so no later migration runs. Recording the files that are really applied stops the RLS series from being re-run.
--
-- NOTE: every RLS migration this script records is now idempotent (DROP POLICY IF EXISTS / safe to re-run), so on a database where this
-- script is never run, `make migrate` attempting one of these files again is a harmless no-op, not a hard failure -- this script is a
-- convenience that skips that one redundant re-apply, not a correctness requirement.
--
-- What it does NOT do: it does not make `make migrate` safe on that database. Other migration files can be untracked as well (applied by
-- hand earlier, or still pending), and this script deliberately records only the RLS series it can verify. Check the untracked list
-- (compare `ls migrations` with the tracking table) before relying on `make migrate` there.
--
--   psql -X -v schema=<schema holding the tables and csm_migration_applied_migration> -f scripts/record_manually_applied_rls_migrations.sql
--
-- Run as the table owner. Inserts rows only (INSERT ... ON CONFLICT DO NOTHING); safe to re-run; changes no table, policy or role.
-- It REFUSES, and records nothing, unless the effect of the series is really present in the database: the policies, the helper functions
-- in their final form, and the removal of RLS from sla/incident/problem. Migration 0190 is recorded only when its seven policies exist.
-- Undo: DELETE FROM csm_migration_applied_migration WHERE filename IN (<the names listed below>);
\set ON_ERROR_STOP on
\if :{?schema}\else \echo 'ERROR: pass -v schema=<schema>' \quit \endif
SELECT set_config('search_path', :'schema', false);

SELECT to_regclass('csm_migration_applied_migration') IS NOT NULL AS has_tracking \gset
\if :has_tracking
\else
  \echo 'REFUSING: there is no csm_migration_applied_migration table in schema' :schema '. Nothing was changed.'
  \quit
\endif

SELECT
  EXISTS (SELECT 1 FROM pg_enum e JOIN pg_type t ON t.oid = e.enumtypid WHERE t.typname = 'end_date_closure_state_enum' AND e.enumlabel = 'SUSPENDED')                       -- 0175_end_date (untouched, not part of the RLS renumbering)
  AND (SELECT count(*) FROM pg_policies WHERE schemaname = :'schema' AND tablename = 'announcement'
         AND policyname IN ('announcement_visibility','announcement_write_unrestricted','announcement_update_unrestricted','announcement_delete_unrestricted')) = 4        -- 100001 (was 000085)
  AND COALESCE((SELECT qual LIKE '%SECURITY_CONTACT%' AND qual LIKE '%project_has_security_contact%' FROM pg_policies
         WHERE schemaname = :'schema' AND tablename = 'announcement' AND policyname = 'announcement_visibility'), false)                                                  -- 100010 + 100019 (was 0149 + 0178)
  AND (SELECT count(*) FROM pg_policies WHERE schemaname = :'schema' AND tablename IN ('case_escalation','case_escalation_notification_list')) >= 8                       -- 100002 + 100011 (was 0141 + 0150)
  AND (SELECT count(DISTINCT tablename) FROM pg_policies WHERE schemaname = :'schema'
         AND tablename IN ('customer_call','time_card','time_card_approver','change_request','approval_stage','approval_stage_approver','conversation')) = 7              -- 100004-100007 (was 0143-0146)
  AND (SELECT count(DISTINCT tablename) FROM pg_policies WHERE schemaname = :'schema'
         AND tablename IN ('work_item','case','comment','comment_edit_history','case_attachment','work_item_tag','work_item_watcher','work_item_activity')) = 8             -- 100008 (was 0147)
  AND (SELECT count(DISTINCT tablename) FROM pg_policies WHERE schemaname = :'schema'
         AND tablename IN ('engagement','service_request','security_report_analysis')) = 3                                                                                  -- 100012 (was 0151)
  AND COALESCE((SELECT prosrc LIKE '%viewer_project_ids%' FROM pg_proc WHERE proname = 'is_project_member' AND pronamespace = :'schema'::regnamespace), false)             -- 100013 (was 0152)
  AND NOT EXISTS (SELECT 1 FROM pg_class WHERE relnamespace = :'schema'::regnamespace AND relname IN ('sla','incident','incident_task','problem') AND relrowsecurity)       -- 100003/100009 then 100014 (was 0142/0148 then 0153)
  AND NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = :'schema' AND qual LIKE '%(current_setting(''app.is_internal''::text, true) = ''true''::text)%'
         AND qual NOT LIKE '%SELECT (current_setting%' AND qual NOT LIKE '%SELECT current_setting%')                                                                        -- 100015 (was 0154)
  AND COALESCE((SELECT qual LIKE '%announcement a%' FROM pg_policies WHERE schemaname = :'schema' AND tablename = 'comment' AND policyname = 'comment_visibility'), false)   -- 100016 (was 0175_announcement_child_table_visibility)
  AND (SELECT count(DISTINCT tablename) FROM pg_policies WHERE schemaname = :'schema' AND tablename IN ('deployment','deployed_product')) = 2                              -- 100017 (was 0176)
  AND COALESCE((SELECT bool_and(proparallel = 's') FROM pg_proc WHERE pronamespace = :'schema'::regnamespace
         AND proname IN ('is_project_member','announcement_is_security','project_has_security_contact')), false)                                                          -- 100018 (was 0177)
  AS series_applied \gset

SELECT (SELECT count(*) FROM pg_policies WHERE schemaname = :'schema'
          AND policyname IN ('work_item_watcher_update_internal_only','work_item_activity_update_internal_only','comment_edit_history_update_internal_only',
                             'time_card_approver_update_internal_only','change_request_delete_internal_only','conversation_delete_internal_only',
                             'customer_call_delete_internal_only')) = 7 AS has_0190 \gset

\if :series_applied
  \echo 'OK: the RLS series is applied in schema' :schema '. Recording it.'
\else
  \echo 'REFUSING: the effect of the RLS migration series (policies, helper functions, sla/incident/problem cleanup) is not fully present in schema' :schema '. Nothing was recorded.'
  \quit
\endif

BEGIN;
INSERT INTO csm_migration_applied_migration (filename) VALUES
  ('0175_end_date_closure_state_suspended.sql'),
  ('100001_announcement_visibility_rls.sql'),
  ('100002_case_escalation_rls.sql'),
  ('100003_sla_rls.sql'),
  ('100004_customer_call_rls.sql'),
  ('100005_time_card_rls.sql'),
  ('100006_change_request_rls.sql'),
  ('100007_conversation_rls.sql'),
  ('100008_case_adjacent_rls.sql'),
  ('100009_incident_problem_deny_all_rls.sql'),
  ('100010_announcement_security_fallback_no_contact.sql'),
  ('100011_case_escalation_write_widen_to_project_members.sql'),
  ('100012_case_like_extension_tables_rls.sql'),
  ('100013_is_project_member_cached_array.sql'),
  ('100014_remove_rls_sla_incident_problem.sql'),
  ('100015_rls_internal_check_initplan.sql'),
  ('100016_announcement_child_table_visibility.sql'),
  ('100017_deployment_rls.sql'),
  ('100018_rls_policy_functions_parallel_safe.sql'),
  ('100019_announcement_visibility_security_contacts_see_all.sql')
ON CONFLICT (filename) DO NOTHING;
\if :has_0190
  INSERT INTO csm_migration_applied_migration (filename) VALUES ('100021_rls_internal_write_policies_for_sync.sql') ON CONFLICT (filename) DO NOTHING;
\else
  \echo 'note: the seven migration-0190 policies are not all present, so 100021 (the renumbered 0190_rls_internal_write_policies_for_sync.sql) was NOT recorded.'
\endif
COMMIT;

SELECT count(*) AS rls_files_now_tracked FROM csm_migration_applied_migration
 WHERE filename ~ '^(100001_announcement_visibility_rls|10000[2-9]_.*_rls|100009_incident_problem_deny_all_rls|100010_announcement_security_fallback_no_contact|100011_case_escalation_write_widen_to_project_members|100012_case_like_extension_tables_rls|100013_is_project_member_cached_array|100014_remove_rls_sla_incident_problem|100015_rls_internal_check_initplan|100016_announcement_child_table_visibility|0175_end_date_closure_state_suspended|100017_deployment_rls|100018_rls_policy_functions_parallel_safe|100019_announcement_visibility_security_contacts_see_all|100021_rls_internal_write_policies_for_sync)';
