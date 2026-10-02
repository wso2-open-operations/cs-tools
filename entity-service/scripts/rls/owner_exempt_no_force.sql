-- Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com). All Rights Reserved.
--
-- This software is the property of WSO2 LLC. and its suppliers, if any.
-- Dissemination of any information or reproduction of any material contained
-- herein in any form is strictly forbidden, unless permitted by WSO2 expressly.

-- Stop at the first error instead of carrying on with a half-applied script.
\set ON_ERROR_STOP on

-- STEP 3: let the OWNER login see all the data (DBeaver, pgAdmin, sync/migration services) by removing FORCE.
-- RLS stays ENABLED with all 85 policies, so every other role, i.e. the application role, is still fully bound.
--
--   psql -X -v schema=csmpd_stg_user -v app_role=csm_entity_app -f scripts/rls/owner_exempt_no_force.sql
--
-- Run as the table owner. Takes effect immediately for open connections. One transaction, all or nothing.
-- Undo any time with scripts/rls/force_all_tables.sql (it puts FORCE back).
--
-- !! ORDER MATTERS. FORCE is the only thing that binds the OWNER. If the application is still connecting as the
-- owner when this runs, RLS silently stops protecting it (back to how things were before RLS). So this script
-- refuses to run unless the application role already has a live connection to this database, i.e. entity-service
-- has been switched to it first.
\if :{?schema}\else \set schema csmpd_stg_user \endif
\if :{?app_role}\else \set app_role csm_entity_app \endif
SELECT count(*) > 0 AS app_connected FROM pg_stat_activity
 WHERE usename = :'app_role' AND datname = current_database() \gset
\if :app_connected
  \echo 'OK: the application role is connected; continuing.'
\else
  \echo 'REFUSING: no live connection from role ' :app_role ' to ' :DBNAME '. Switch entity-service to that role first (Choreo DB_USER / DB_PASSWORD), confirm it works, then re-run. Nothing was changed.'
  \quit
\endif

BEGIN;
SELECT set_config('search_path', :'schema', true);
ALTER TABLE announcement NO FORCE ROW LEVEL SECURITY;
ALTER TABLE approval_stage NO FORCE ROW LEVEL SECURITY;
ALTER TABLE approval_stage_approver NO FORCE ROW LEVEL SECURITY;
ALTER TABLE "case" NO FORCE ROW LEVEL SECURITY;
ALTER TABLE case_attachment NO FORCE ROW LEVEL SECURITY;
ALTER TABLE case_escalation NO FORCE ROW LEVEL SECURITY;
ALTER TABLE case_escalation_notification_list NO FORCE ROW LEVEL SECURITY;
ALTER TABLE change_request NO FORCE ROW LEVEL SECURITY;
ALTER TABLE comment NO FORCE ROW LEVEL SECURITY;
ALTER TABLE comment_edit_history NO FORCE ROW LEVEL SECURITY;
ALTER TABLE conversation NO FORCE ROW LEVEL SECURITY;
ALTER TABLE customer_call NO FORCE ROW LEVEL SECURITY;
ALTER TABLE deployed_product NO FORCE ROW LEVEL SECURITY;
ALTER TABLE deployment NO FORCE ROW LEVEL SECURITY;
ALTER TABLE engagement NO FORCE ROW LEVEL SECURITY;
ALTER TABLE security_report_analysis NO FORCE ROW LEVEL SECURITY;
ALTER TABLE service_request NO FORCE ROW LEVEL SECURITY;
ALTER TABLE time_card NO FORCE ROW LEVEL SECURITY;
ALTER TABLE time_card_approver NO FORCE ROW LEVEL SECURITY;
ALTER TABLE work_item NO FORCE ROW LEVEL SECURITY;
ALTER TABLE work_item_activity NO FORCE ROW LEVEL SECURITY;
ALTER TABLE work_item_tag NO FORCE ROW LEVEL SECURITY;
ALTER TABLE work_item_watcher NO FORCE ROW LEVEL SECURITY;
COMMIT;

-- Check: RLS enabled = t on all 23, forced = f on all 23
SELECT count(*) FILTER (WHERE relrowsecurity) AS rls_enabled, count(*) FILTER (WHERE relforcerowsecurity) AS forced,
       (SELECT count(*) FROM pg_policies WHERE schemaname = :'schema') AS policies
FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = :'schema' AND c.relkind = 'r'
  AND c.relname IN ('announcement','approval_stage','approval_stage_approver','case','case_attachment','case_escalation','case_escalation_notification_list','change_request','comment','comment_edit_history','conversation','customer_call','deployed_product','deployment','engagement','security_report_analysis','service_request','time_card','time_card_approver','work_item','work_item_activity','work_item_tag','work_item_watcher');
