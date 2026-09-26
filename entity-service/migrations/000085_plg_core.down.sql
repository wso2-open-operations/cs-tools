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

DROP VIEW IF EXISTS plg_run_task_reason_v;
DROP VIEW IF EXISTS plg_work_queue_v;
DROP VIEW IF EXISTS plg_org_platform_v;
DROP VIEW IF EXISTS plg_organization_v;
DROP VIEW IF EXISTS plg_playbook_run_v;
DROP VIEW IF EXISTS plg_user_v;

DROP TABLE IF EXISTS plg_note_revision;
DROP TABLE IF EXISTS plg_ingest_failure;
DROP TABLE IF EXISTS plg_organization_attribute;
DROP TABLE IF EXISTS plg_note;
DROP TABLE IF EXISTS plg_playbook_run_task;
DROP TABLE IF EXISTS plg_playbook_run;
DROP TABLE IF EXISTS plg_playbook_task;
DROP TABLE IF EXISTS plg_playbook;
DROP TABLE IF EXISTS plg_lifecycle_history;
DROP TABLE IF EXISTS plg_org_platform;
DROP TABLE IF EXISTS plg_organization;
DROP TABLE IF EXISTS plg_person;
DROP TABLE IF EXISTS plg_lifecycle_stage;
DROP TABLE IF EXISTS plg_product;

DROP FUNCTION IF EXISTS plg_check_stage_move();
DROP FUNCTION IF EXISTS plg_check_run_product();
DROP FUNCTION IF EXISTS plg_set_updated_at();

-- Before the enum: the function's return type names it.
DROP FUNCTION IF EXISTS plg_applicable_playbook_types(plg_health_enum);
DROP FUNCTION IF EXISTS plg_current_period_end_date(
    plg_subscription_tier_enum, DATE, DATE);

DROP TYPE IF EXISTS plg_playbook_type_enum;
DROP TYPE IF EXISTS plg_health_enum;
DROP TYPE IF EXISTS plg_subscription_tier_enum;
DROP TYPE IF EXISTS plg_task_value_type_enum;
DROP TYPE IF EXISTS plg_lifecycle_stage_enum;
