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

-- Start auditing work_item (see 0295_audit_log_foundation.sql).
--
--   key        the primary key (id)
--   excluded   none of its own; the trigger always leaves out updated_on and updated_by
--   masked     none: description is recorded in clear (binary columns are never
--              logged; register_audit leaves them out on its own)
--
-- Registered only when work_item has no audit_config row yet, so a registration
-- made by hand (or a table an admin turned off with unregister_audit) is left as
-- it is. To change a registered table's settings later, call register_audit
-- directly (it is an upsert that rebuilds the trigger).
DO $$
BEGIN
    IF to_regclass(format('%I.work_item', current_schema())) IS NULL THEN
        RAISE NOTICE 'audit: work_item does not exist in schema %, not registered', current_schema();
    ELSIF NOT EXISTS (SELECT 1 FROM audit_config
                       WHERE schema_name = current_schema() AND table_name = 'work_item') THEN
        PERFORM register_audit(current_schema(), 'work_item', NULL, '{}', '{}');
    END IF;
END $$;
