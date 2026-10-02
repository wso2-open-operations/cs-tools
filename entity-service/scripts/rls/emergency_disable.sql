-- Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com). All Rights Reserved.
--
-- This software is the property of WSO2 LLC. and its suppliers, if any.
-- Dissemination of any information or reproduction of any material contained
-- herein in any form is strictly forbidden, unless permitted by WSO2 expressly.

-- Stop at the first error instead of carrying on with a half-applied script.
\set ON_ERROR_STOP on

-- EMERGENCY: switch row-level security OFF (23 tables). Policies stay defined; behaviour returns to how dev was before the RLS PR.
-- Schema: defaults to csmpd_stg_user. For a different database/schema run:
--   psql -X -v schema=<schema_name> -f "<this file>"
-- Run as the table owner (or an admin role): ALTER TABLE needs ownership.
-- Takes effect immediately, including on connections that are already open.
-- No restart and no code change is needed. Runs in one transaction: all or nothing.
\if :{?schema}
\else
  \set schema csmpd_stg_user
\endif
BEGIN;
SELECT set_config('search_path', :'schema', true);
ALTER TABLE announcement NO FORCE ROW LEVEL SECURITY;
ALTER TABLE announcement DISABLE ROW LEVEL SECURITY;
ALTER TABLE approval_stage NO FORCE ROW LEVEL SECURITY;
ALTER TABLE approval_stage DISABLE ROW LEVEL SECURITY;
ALTER TABLE approval_stage_approver NO FORCE ROW LEVEL SECURITY;
ALTER TABLE approval_stage_approver DISABLE ROW LEVEL SECURITY;
ALTER TABLE "case" NO FORCE ROW LEVEL SECURITY;
ALTER TABLE "case" DISABLE ROW LEVEL SECURITY;
ALTER TABLE case_attachment NO FORCE ROW LEVEL SECURITY;
ALTER TABLE case_attachment DISABLE ROW LEVEL SECURITY;
ALTER TABLE case_escalation NO FORCE ROW LEVEL SECURITY;
ALTER TABLE case_escalation DISABLE ROW LEVEL SECURITY;
ALTER TABLE case_escalation_notification_list NO FORCE ROW LEVEL SECURITY;
ALTER TABLE case_escalation_notification_list DISABLE ROW LEVEL SECURITY;
ALTER TABLE change_request NO FORCE ROW LEVEL SECURITY;
ALTER TABLE change_request DISABLE ROW LEVEL SECURITY;
ALTER TABLE comment NO FORCE ROW LEVEL SECURITY;
ALTER TABLE comment DISABLE ROW LEVEL SECURITY;
ALTER TABLE comment_edit_history NO FORCE ROW LEVEL SECURITY;
ALTER TABLE comment_edit_history DISABLE ROW LEVEL SECURITY;
ALTER TABLE conversation NO FORCE ROW LEVEL SECURITY;
ALTER TABLE conversation DISABLE ROW LEVEL SECURITY;
ALTER TABLE customer_call NO FORCE ROW LEVEL SECURITY;
ALTER TABLE customer_call DISABLE ROW LEVEL SECURITY;
ALTER TABLE deployed_product NO FORCE ROW LEVEL SECURITY;
ALTER TABLE deployed_product DISABLE ROW LEVEL SECURITY;
ALTER TABLE deployment NO FORCE ROW LEVEL SECURITY;
ALTER TABLE deployment DISABLE ROW LEVEL SECURITY;
ALTER TABLE engagement NO FORCE ROW LEVEL SECURITY;
ALTER TABLE engagement DISABLE ROW LEVEL SECURITY;
ALTER TABLE security_report_analysis NO FORCE ROW LEVEL SECURITY;
ALTER TABLE security_report_analysis DISABLE ROW LEVEL SECURITY;
ALTER TABLE service_request NO FORCE ROW LEVEL SECURITY;
ALTER TABLE service_request DISABLE ROW LEVEL SECURITY;
ALTER TABLE time_card NO FORCE ROW LEVEL SECURITY;
ALTER TABLE time_card DISABLE ROW LEVEL SECURITY;
ALTER TABLE time_card_approver NO FORCE ROW LEVEL SECURITY;
ALTER TABLE time_card_approver DISABLE ROW LEVEL SECURITY;
ALTER TABLE work_item NO FORCE ROW LEVEL SECURITY;
ALTER TABLE work_item DISABLE ROW LEVEL SECURITY;
ALTER TABLE work_item_activity NO FORCE ROW LEVEL SECURITY;
ALTER TABLE work_item_activity DISABLE ROW LEVEL SECURITY;
ALTER TABLE work_item_tag NO FORCE ROW LEVEL SECURITY;
ALTER TABLE work_item_tag DISABLE ROW LEVEL SECURITY;
ALTER TABLE work_item_watcher NO FORCE ROW LEVEL SECURITY;
ALTER TABLE work_item_watcher DISABLE ROW LEVEL SECURITY;
COMMIT;

-- Check: every row below should read relrowsecurity = f, relforcerowsecurity = f
SELECT c.relname, c.relrowsecurity, c.relforcerowsecurity
FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = :'schema' AND c.relkind = 'r'
  AND c.relname IN ('announcement', 'approval_stage', 'approval_stage_approver', 'case', 'case_attachment', 'case_escalation', 'case_escalation_notification_list', 'change_request', 'comment', 'comment_edit_history', 'conversation', 'customer_call', 'deployed_product', 'deployment', 'engagement', 'security_report_analysis', 'service_request', 'time_card', 'time_card_approver', 'work_item', 'work_item_activity', 'work_item_tag', 'work_item_watcher')
ORDER BY 1;
