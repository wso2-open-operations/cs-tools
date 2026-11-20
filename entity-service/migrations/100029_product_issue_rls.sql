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

-- Row-level security for Product Issues (migrations 0303-0305). Internal only.
--
-- 1. Every new table is readable and writable by internal sessions only
--    (app.is_internal = 'true'), the shape of 100026_standard_change_rls.sql.
--
-- 2. work_item. A product issue's work_item row copies its case's project_id, and
--    work_item_visibility (100008) lets any member of that project read a row, so
--    a customer contact would see it. A RESTRICTIVE policy is ANDed with the
--    permissive ones, for every command, so it hides the row whatever else
--    allows it. The types to hide are listed in one place: the array in
--    work_item_type_is_internal_only() below. A later internal-only type is added
--    there (a new migration that redefines the function), nowhere else.
--
--    The children of work_item (comment, comment_edit_history, work_item_activity,
--    work_item_attachment, work_item_tag, work_item_watcher, case_attachment ...)
--    reach project_id through "SELECT project_id FROM work_item", and that
--    subquery is itself filtered by work_item's policies, so their rows for a
--    hidden work item are invisible too. product_issue_migration_integration_test.go
--    asserts this for comment and work_item_activity.
--
-- One transaction, every CREATE POLICY preceded by DROP POLICY IF EXISTS, as
-- elsewhere in this track.
BEGIN;

DO $$
DECLARE
    t TEXT;
BEGIN
    FOREACH t IN ARRAY ARRAY[
        'product_issue',
        'github_issue_link',
        'work_item_link',
        'work_item_status_event',
        'comment_github_ref',
        'board_status_map',
        'work_item_creation_request'
    ] LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
        EXECUTE format('DROP POLICY IF EXISTS %I ON %I', t || '_internal_only', t);
        EXECUTE format(
            'CREATE POLICY %I ON %I FOR ALL USING (current_setting(''app.is_internal'', true) = ''true'') WITH CHECK (current_setting(''app.is_internal'', true) = ''true'')',
            t || '_internal_only', t);
    END LOOP;
END $$;

-- The one list of work item types customers never see. The policy above wraps
-- the setting in (SELECT ...) like work_item's other policies, so Postgres
-- evaluates it once per statement, not once per row.
CREATE OR REPLACE FUNCTION work_item_type_is_internal_only(p_type work_item_type_enum)
RETURNS boolean AS $$
    SELECT p_type = ANY (ARRAY['PRODUCT_ISSUE']::work_item_type_enum[]);
$$ LANGUAGE sql IMMUTABLE PARALLEL SAFE;

DROP POLICY IF EXISTS work_item_internal_only_types ON work_item;
CREATE POLICY work_item_internal_only_types ON work_item
  AS RESTRICTIVE
  FOR ALL
  USING (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR NOT work_item_type_is_internal_only(type)
  )
  WITH CHECK (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR NOT work_item_type_is_internal_only(type)
  );

COMMIT;
