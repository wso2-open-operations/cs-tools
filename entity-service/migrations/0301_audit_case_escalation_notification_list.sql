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

-- Start auditing case_escalation_notification_list (see 0295 to 0300): who is
-- told when a case is escalated. The primary key is the record id and every
-- column is recorded.
--
-- Registered only when it has no audit_config row yet, so a registration made by
-- hand, or a table an admin turned off with unregister_audit, is left as it is.
DO $$
BEGIN
    IF to_regclass(format('%I.case_escalation_notification_list', current_schema())) IS NULL THEN
        RAISE NOTICE 'audit: case_escalation_notification_list does not exist in schema %, not registered', current_schema();
    ELSIF NOT EXISTS (SELECT 1 FROM audit_config
                       WHERE schema_name = current_schema() AND table_name = 'case_escalation_notification_list') THEN
        PERFORM register_audit(current_schema(), 'case_escalation_notification_list', NULL, '{}', '{}');
    END IF;
END $$;
