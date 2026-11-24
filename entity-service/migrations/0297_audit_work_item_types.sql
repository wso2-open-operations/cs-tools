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

-- Start auditing the work item types (see 0295 and 0296): the tables whose id is
-- the work_item id. Each is registered with its primary key as the record id and
-- every column recorded, as work_item is. Other tables that reference work_item
-- (comments, tags, attachments, watchers ...) are not audited here; add one in
-- its own migration when it is wanted.
--
-- A table is registered only when it has no audit_config row yet, so a
-- registration made by hand, or a table an admin turned off with
-- unregister_audit, is left as it is. A table missing from the schema is skipped
-- with a notice.
DO $$
DECLARE
    t TEXT;
BEGIN
    FOREACH t IN ARRAY ARRAY[
        'announcement', 'case', 'change_request', 'change_task', 'conversation',
        'engagement', 'incident', 'incident_alert_task', 'incident_task', 'problem',
        'problem_task', 'security_report_analysis', 'service_request',
        'standard_change_proposal'
    ] LOOP
        IF to_regclass(format('%I.%I', current_schema(), t)) IS NULL THEN
            RAISE NOTICE 'audit: % does not exist in schema %, not registered', t, current_schema();
        ELSIF NOT EXISTS (SELECT 1 FROM audit_config
                           WHERE schema_name = current_schema() AND table_name = t) THEN
            PERFORM register_audit(current_schema(), t, NULL, '{}', '{}');
        END IF;
    END LOOP;
END $$;
