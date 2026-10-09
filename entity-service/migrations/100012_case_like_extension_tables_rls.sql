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

-- Corrects migration 100008's own decision to leave engagement/
-- service_request/security_report_analysis (the three other case-like
-- work_item extension tables, alongside "case" and announcement) with NO
-- RLS of their own. That migration's stated reasoning was that protecting
-- them "would only risk infinite-recursion policy errors for zero
-- additional safety," borrowing project_contact's own exclusion reasoning
-- (migration 100002) -- but that reasoning does not actually transfer here.
-- project_contact is excluded because is_project_member() itself queries
-- project_contact, so a policy ON project_contact that also called
-- is_project_member() would recurse. Nothing about is_project_member()
-- queries engagement/service_request/security_report_analysis, so no such
-- recursion is possible for them -- they are structurally identical to
-- "case"/comment/case_attachment (migration 100008's own other tables),
-- which already use this exact is_project_member-via-work_item-subquery
-- shape with zero recursion issue.
--
-- Confirmed before writing this migration (not assumed): the only read of
-- these three tables in the whole codebase is case_repo.go's caseLikeJoins,
-- a LEFT JOIN driven from the already-RLS-protected work_item; the only
-- customer-triggered write is inside the same statement as work_item's own
-- INSERT (so work_item's own WITH CHECK already gates it); the remaining
-- write (github_mutation_repo.go's service_request UPDATE) only ever runs
-- under the GitHub webhook handler's system identity. So this migration
-- closes a real structural gap without changing today's actual behavior --
-- it makes safe-by-Go-code-accident into safe-by-database-guarantee, the
-- same reasoning this entire migration series exists for.
--
-- One transaction, and every CREATE POLICY preceded by its own DROP POLICY
-- IF EXISTS -- see migration 100002's identical note.
BEGIN;

ALTER TABLE engagement ENABLE ROW LEVEL SECURITY;
ALTER TABLE engagement FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS engagement_visibility ON engagement;
CREATE POLICY engagement_visibility ON engagement
  FOR SELECT
  USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = engagement.id))
  );

DROP POLICY IF EXISTS engagement_update ON engagement;
CREATE POLICY engagement_update ON engagement
  FOR UPDATE USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = engagement.id))
  )
  WITH CHECK (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = engagement.id))
  );

DROP POLICY IF EXISTS engagement_write ON engagement;
CREATE POLICY engagement_write ON engagement
  FOR INSERT WITH CHECK (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = engagement.id))
  );
-- Internal-only, not omitted entirely -- same sla_delete-style test/admin
-- cleanup reasoning as migration 100008's other tables.
DROP POLICY IF EXISTS engagement_delete_internal_only ON engagement;
CREATE POLICY engagement_delete_internal_only ON engagement
  FOR DELETE USING (current_setting('app.is_internal', true) = 'true');

ALTER TABLE service_request ENABLE ROW LEVEL SECURITY;
ALTER TABLE service_request FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS service_request_visibility ON service_request;
CREATE POLICY service_request_visibility ON service_request
  FOR SELECT
  USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = service_request.id))
  );

DROP POLICY IF EXISTS service_request_update ON service_request;
CREATE POLICY service_request_update ON service_request
  FOR UPDATE USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = service_request.id))
  )
  WITH CHECK (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = service_request.id))
  );

DROP POLICY IF EXISTS service_request_write ON service_request;
CREATE POLICY service_request_write ON service_request
  FOR INSERT WITH CHECK (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = service_request.id))
  );
DROP POLICY IF EXISTS service_request_delete_internal_only ON service_request;
CREATE POLICY service_request_delete_internal_only ON service_request
  FOR DELETE USING (current_setting('app.is_internal', true) = 'true');

ALTER TABLE security_report_analysis ENABLE ROW LEVEL SECURITY;
ALTER TABLE security_report_analysis FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS security_report_analysis_visibility ON security_report_analysis;
CREATE POLICY security_report_analysis_visibility ON security_report_analysis
  FOR SELECT
  USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = security_report_analysis.id))
  );

DROP POLICY IF EXISTS security_report_analysis_update ON security_report_analysis;
CREATE POLICY security_report_analysis_update ON security_report_analysis
  FOR UPDATE USING (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = security_report_analysis.id))
  )
  WITH CHECK (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = security_report_analysis.id))
  );

DROP POLICY IF EXISTS security_report_analysis_write ON security_report_analysis;
CREATE POLICY security_report_analysis_write ON security_report_analysis
  FOR INSERT WITH CHECK (
    current_setting('app.is_internal', true) = 'true'
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = security_report_analysis.id))
  );
DROP POLICY IF EXISTS security_report_analysis_delete_internal_only ON security_report_analysis;
CREATE POLICY security_report_analysis_delete_internal_only ON security_report_analysis
  FOR DELETE USING (current_setting('app.is_internal', true) = 'true');

COMMIT;
