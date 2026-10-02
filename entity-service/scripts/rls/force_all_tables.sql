-- Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com). All Rights Reserved.
--
-- This software is the property of WSO2 LLC. and its suppliers, if any.
-- Dissemination of any information or reproduction of any material contained
-- herein in any form is strictly forbidden, unless permitted by WSO2 expressly.

-- Stop at the first error instead of carrying on with a half-applied script.
\set ON_ERROR_STOP on

-- Switch row-level security back ON (23 tables), exactly as the PR's migrations left it (ENABLE + FORCE).
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
ALTER TABLE announcement ENABLE ROW LEVEL SECURITY;
ALTER TABLE announcement FORCE ROW LEVEL SECURITY;
ALTER TABLE approval_stage ENABLE ROW LEVEL SECURITY;
ALTER TABLE approval_stage FORCE ROW LEVEL SECURITY;
ALTER TABLE approval_stage_approver ENABLE ROW LEVEL SECURITY;
ALTER TABLE approval_stage_approver FORCE ROW LEVEL SECURITY;
ALTER TABLE "case" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "case" FORCE ROW LEVEL SECURITY;
ALTER TABLE case_attachment ENABLE ROW LEVEL SECURITY;
ALTER TABLE case_attachment FORCE ROW LEVEL SECURITY;
ALTER TABLE case_escalation ENABLE ROW LEVEL SECURITY;
ALTER TABLE case_escalation FORCE ROW LEVEL SECURITY;
ALTER TABLE case_escalation_notification_list ENABLE ROW LEVEL SECURITY;
ALTER TABLE case_escalation_notification_list FORCE ROW LEVEL SECURITY;
ALTER TABLE change_request ENABLE ROW LEVEL SECURITY;
ALTER TABLE change_request FORCE ROW LEVEL SECURITY;
ALTER TABLE comment ENABLE ROW LEVEL SECURITY;
ALTER TABLE comment FORCE ROW LEVEL SECURITY;
ALTER TABLE comment_edit_history ENABLE ROW LEVEL SECURITY;
ALTER TABLE comment_edit_history FORCE ROW LEVEL SECURITY;
ALTER TABLE conversation ENABLE ROW LEVEL SECURITY;
ALTER TABLE conversation FORCE ROW LEVEL SECURITY;
ALTER TABLE customer_call ENABLE ROW LEVEL SECURITY;
ALTER TABLE customer_call FORCE ROW LEVEL SECURITY;
ALTER TABLE deployed_product ENABLE ROW LEVEL SECURITY;
ALTER TABLE deployed_product FORCE ROW LEVEL SECURITY;
ALTER TABLE deployment ENABLE ROW LEVEL SECURITY;
ALTER TABLE deployment FORCE ROW LEVEL SECURITY;
ALTER TABLE engagement ENABLE ROW LEVEL SECURITY;
ALTER TABLE engagement FORCE ROW LEVEL SECURITY;
ALTER TABLE security_report_analysis ENABLE ROW LEVEL SECURITY;
ALTER TABLE security_report_analysis FORCE ROW LEVEL SECURITY;
ALTER TABLE service_request ENABLE ROW LEVEL SECURITY;
ALTER TABLE service_request FORCE ROW LEVEL SECURITY;
ALTER TABLE time_card ENABLE ROW LEVEL SECURITY;
ALTER TABLE time_card FORCE ROW LEVEL SECURITY;
ALTER TABLE time_card_approver ENABLE ROW LEVEL SECURITY;
ALTER TABLE time_card_approver FORCE ROW LEVEL SECURITY;
ALTER TABLE work_item ENABLE ROW LEVEL SECURITY;
ALTER TABLE work_item FORCE ROW LEVEL SECURITY;
ALTER TABLE work_item_activity ENABLE ROW LEVEL SECURITY;
ALTER TABLE work_item_activity FORCE ROW LEVEL SECURITY;
ALTER TABLE work_item_tag ENABLE ROW LEVEL SECURITY;
ALTER TABLE work_item_tag FORCE ROW LEVEL SECURITY;
ALTER TABLE work_item_watcher ENABLE ROW LEVEL SECURITY;
ALTER TABLE work_item_watcher FORCE ROW LEVEL SECURITY;
COMMIT;

-- Check: every row below should read relrowsecurity = t, relforcerowsecurity = t
SELECT c.relname, c.relrowsecurity, c.relforcerowsecurity
FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = :'schema' AND c.relkind = 'r'
  AND c.relname IN ('announcement', 'approval_stage', 'approval_stage_approver', 'case', 'case_attachment', 'case_escalation', 'case_escalation_notification_list', 'change_request', 'comment', 'comment_edit_history', 'conversation', 'customer_call', 'deployed_product', 'deployment', 'engagement', 'security_report_analysis', 'service_request', 'time_card', 'time_card_approver', 'work_item', 'work_item_activity', 'work_item_tag', 'work_item_watcher')
ORDER BY 1;
