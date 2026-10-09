-- Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
--
-- WSO2 LLC. licenses this file to you under the Apache License,
-- Version 2.0 (the "License"); you may not use this file except
-- in compliance with the License.
-- You may obtain a copy of the License at
--
-- http://www.apache.org/licenses/LICENSE-2.0
--
-- Unless required by applicable law or agreed to in writing,
-- software distributed under the License is distributed on an
-- "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
-- KIND, either express or implied.  See the License for the
-- specific language governing permissions and limitations
-- under the License.

-- Migration 0141's case_escalation/case_escalation_notification_list write
-- policies were internal-only because, at the time, EscalationService.
-- CreateEscalation had no real implementation (always returned
-- ServiceUnavailableError -- see 100002's own doc comment). That has since
-- changed: escalation_service.go's CreateEscalation is now a genuine,
-- project-membership-authorized customer-facing write (POST /escalations
-- and POST /cases/{id}/escalations both funnel through it, and it checks
-- req.CaseID against the caller's AccessScope via CaseRepository.GetCaseByID
-- before ever calling the repository). Left as internal-only, RLS would
-- reject every one of those legitimate customer writes with a 500 (the
-- INSERT's WITH CHECK failing), even though the Go layer already confirmed
-- the caller may act on that case.
--
-- Only the INSERT policies are widened: escalation_repo.go's CreateEscalation
-- only ever INSERTs into these two tables (each escalate/de-escalate creates
-- a new case_escalation row rather than mutating an existing one); no
-- customer flow updates or deletes a case_escalation record, so UPDATE/DELETE
-- stay internal-only, unchanged from 0141.
--
-- One transaction, and IF EXISTS on both DROP POLICY statements below: a
-- plain DROP POLICY (no IF EXISTS) fails outright on a re-run, once the
-- first run has already renamed the policy away -- same "safe to re-run"
-- requirement as every CREATE POLICY in this migration series.
BEGIN;

DROP POLICY IF EXISTS case_escalation_write_internal_only ON case_escalation;
DROP POLICY IF EXISTS case_escalation_write ON case_escalation;
CREATE POLICY case_escalation_write ON case_escalation
  FOR INSERT WITH CHECK (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = work_item_id))
  );

-- case_escalation_notification_list's new row points at the case_escalation
-- row CreateEscalation just inserted in the same transaction (read-your-own-
-- write under READ COMMITTED, so the join below sees it), so the same
-- membership check is re-derived through case_escalation.work_item_id rather
-- than trusting the parent insert's own check implicitly.
DROP POLICY IF EXISTS case_escalation_notification_list_write_internal_only ON case_escalation_notification_list;
DROP POLICY IF EXISTS case_escalation_notification_list_write ON case_escalation_notification_list;
CREATE POLICY case_escalation_notification_list_write ON case_escalation_notification_list
  FOR INSERT WITH CHECK (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((
      SELECT wi.project_id
      FROM case_escalation ce
      JOIN work_item wi ON wi.id = ce.work_item_id
      WHERE ce.id = case_escalation_id
    ))
  );

COMMIT;
