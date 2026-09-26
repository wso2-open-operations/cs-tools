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

-- Adds sys_user_group ("group") FK columns across the tables that carry a
-- reference to it, now that the full group table exists (000073_group_table).
-- All ON DELETE SET NULL: a deleted group shouldn't take the referencing row
-- down with it. Each gets a matching index - group has delete_sync enabled
-- (sys_user_group_all.yaml), so an ON DELETE SET NULL fires a scan of every
-- referencing table on each group deletion, and that scan needs to hit an
-- index rather than a full table scan.
--
-- schedule_span and knowledge_article are intentionally omitted here: neither
-- table exists anywhere in this schema (no migration creates them), so there
-- is nothing to add these two columns to.
ALTER TABLE work_item ADD COLUMN IF NOT EXISTS assignment_group_id UUID REFERENCES "group"(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_work_item_assignment_group_id ON work_item (assignment_group_id);
ALTER TABLE change_request ADD COLUMN IF NOT EXISTS customer_group_id UUID REFERENCES "group"(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_change_request_customer_group_id ON change_request (customer_group_id);
ALTER TABLE service ADD COLUMN IF NOT EXISTS assignment_group_id UUID REFERENCES "group"(id) ON DELETE SET NULL;
ALTER TABLE service ADD COLUMN IF NOT EXISTS support_group_id UUID REFERENCES "group"(id) ON DELETE SET NULL;
ALTER TABLE service ADD COLUMN IF NOT EXISTS managed_by_group_id UUID REFERENCES "group"(id) ON DELETE SET NULL;
ALTER TABLE service ADD COLUMN IF NOT EXISTS approval_group_id UUID REFERENCES "group"(id) ON DELETE SET NULL;
ALTER TABLE service ADD COLUMN IF NOT EXISTS user_group_id UUID REFERENCES "group"(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_service_assignment_group_id ON service (assignment_group_id);
CREATE INDEX IF NOT EXISTS idx_service_support_group_id ON service (support_group_id);
CREATE INDEX IF NOT EXISTS idx_service_managed_by_group_id ON service (managed_by_group_id);
CREATE INDEX IF NOT EXISTS idx_service_approval_group_id ON service (approval_group_id);
CREATE INDEX IF NOT EXISTS idx_service_user_group_id ON service (user_group_id);
ALTER TABLE communication_plan ADD COLUMN IF NOT EXISTS assignment_group_id UUID REFERENCES "group"(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_communication_plan_assignment_group_id ON communication_plan (assignment_group_id);
ALTER TABLE account ADD COLUMN IF NOT EXISTS cre_team_id UUID REFERENCES "group"(id) ON DELETE SET NULL;
ALTER TABLE account ADD COLUMN IF NOT EXISTS sre_team_id UUID REFERENCES "group"(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_account_cre_team_id ON account (cre_team_id);
CREATE INDEX IF NOT EXISTS idx_account_sre_team_id ON account (sre_team_id);
ALTER TABLE project ADD COLUMN IF NOT EXISTS assignment_group_id UUID REFERENCES "group"(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_project_assignment_group_id ON project (assignment_group_id);
ALTER TABLE incident_alert ADD COLUMN IF NOT EXISTS assignment_group_id UUID REFERENCES "group"(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_incident_alert_assignment_group_id ON incident_alert (assignment_group_id);
ALTER TABLE catalog_item ADD COLUMN IF NOT EXISTS group_id UUID REFERENCES "group"(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_catalog_item_group_id ON catalog_item (group_id);
ALTER TABLE service_offering ADD COLUMN IF NOT EXISTS assignment_group_id UUID REFERENCES "group"(id) ON DELETE SET NULL;
ALTER TABLE service_offering ADD COLUMN IF NOT EXISTS support_group_id UUID REFERENCES "group"(id) ON DELETE SET NULL;
ALTER TABLE service_offering ADD COLUMN IF NOT EXISTS managed_by_group_id UUID REFERENCES "group"(id) ON DELETE SET NULL;
ALTER TABLE service_offering ADD COLUMN IF NOT EXISTS approval_group_id UUID REFERENCES "group"(id) ON DELETE SET NULL;
ALTER TABLE service_offering ADD COLUMN IF NOT EXISTS user_group_id UUID REFERENCES "group"(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_service_offering_assignment_group_id ON service_offering (assignment_group_id);
CREATE INDEX IF NOT EXISTS idx_service_offering_support_group_id ON service_offering (support_group_id);
CREATE INDEX IF NOT EXISTS idx_service_offering_managed_by_group_id ON service_offering (managed_by_group_id);
CREATE INDEX IF NOT EXISTS idx_service_offering_approval_group_id ON service_offering (approval_group_id);
CREATE INDEX IF NOT EXISTS idx_service_offering_user_group_id ON service_offering (user_group_id);
ALTER TABLE customer_call ADD COLUMN IF NOT EXISTS assignment_group_id UUID REFERENCES "group"(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_customer_call_assignment_group_id ON customer_call (assignment_group_id);
ALTER TABLE team_member ADD COLUMN IF NOT EXISTS group_id UUID REFERENCES "group"(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_team_member_group_id ON team_member (group_id);
ALTER TABLE deployment ADD COLUMN IF NOT EXISTS assignment_group_id UUID REFERENCES "group"(id) ON DELETE SET NULL;
ALTER TABLE deployment ADD COLUMN IF NOT EXISTS support_group_id UUID REFERENCES "group"(id) ON DELETE SET NULL;
ALTER TABLE deployment ADD COLUMN IF NOT EXISTS managed_by_group_id UUID REFERENCES "group"(id) ON DELETE SET NULL;
ALTER TABLE deployment ADD COLUMN IF NOT EXISTS approval_group_id UUID REFERENCES "group"(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_deployment_assignment_group_id ON deployment (assignment_group_id);
CREATE INDEX IF NOT EXISTS idx_deployment_support_group_id ON deployment (support_group_id);
CREATE INDEX IF NOT EXISTS idx_deployment_managed_by_group_id ON deployment (managed_by_group_id);
CREATE INDEX IF NOT EXISTS idx_deployment_approval_group_id ON deployment (approval_group_id);
