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

-- Standard change templates and proposals (migration 0252) are internal: only
-- WSO2 staff read or write them. A customer reads a change request raised from
-- a template like any other; the template itself is not theirs to see.
--
-- standard_change_proposal needs no extra rule for the work_item row it
-- extends: a proposal's work item has no project_id, so work_item_visibility
-- (100008) already hides it from everyone but internal callers.
--
-- One transaction, every CREATE POLICY preceded by DROP POLICY IF EXISTS, as
-- elsewhere in this track.
BEGIN;

DO $$
DECLARE
    t TEXT;
BEGIN
    FOREACH t IN ARRAY ARRAY[
        'standard_change_template',
        'standard_change_template_version',
        'standard_change_proposal',
        'standard_change_proposal_evidence'
    ] LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
        EXECUTE format('DROP POLICY IF EXISTS %I ON %I', t || '_internal_only', t);
        EXECUTE format(
            'CREATE POLICY %I ON %I FOR ALL USING (current_setting(''app.is_internal'', true) = ''true'') WITH CHECK (current_setting(''app.is_internal'', true) = ''true'')',
            t || '_internal_only', t);
    END LOOP;
END $$;

COMMIT;
