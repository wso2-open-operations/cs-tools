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

-- Audit the product issue tables (see 0295, 0297): product_issue as the other
-- work item type tables, and work_item_link, every column recorded. The sync
-- tables (github_issue_link, work_item_status_event, comment_github_ref,
-- work_item_creation_request, board_status_map) are rewritten by every GitHub
-- sync pass and are not audited.
--
-- A table is registered only when it has no audit_config row yet, so a
-- registration made by hand, or a table an admin turned off with
-- unregister_audit, is left as it is. The sync tables are recorded as excluded,
-- so the new-table warning (0295) does not name them.
DO $$
DECLARE
    t TEXT;
BEGIN
    FOREACH t IN ARRAY ARRAY['product_issue', 'work_item_link'] LOOP
        IF to_regclass(format('%I.%I', current_schema(), t)) IS NULL THEN
            RAISE NOTICE 'audit: % does not exist in schema %, not registered', t, current_schema();
        ELSIF NOT EXISTS (SELECT 1 FROM audit_config
                           WHERE schema_name = current_schema() AND table_name = t) THEN
            PERFORM register_audit(current_schema(), t, NULL, '{}', '{}');
        END IF;
    END LOOP;
END $$;

DO $$
DECLARE
    t TEXT;
BEGIN
    FOREACH t IN ARRAY ARRAY[
        'github_issue_link', 'work_item_status_event', 'comment_github_ref',
        'work_item_creation_request', 'board_status_map'
    ] LOOP
        IF to_regclass(format('%I.%I', current_schema(), t)) IS NOT NULL
           AND NOT EXISTS (SELECT 1 FROM audit_config
                            WHERE schema_name = current_schema() AND table_name = t) THEN
            PERFORM exclude_from_audit(current_schema(), t,
                'rewritten by every GitHub sync pass or request retry; product issue changes are audited on product_issue and work_item_link');
        END IF;
    END LOOP;
END $$;
