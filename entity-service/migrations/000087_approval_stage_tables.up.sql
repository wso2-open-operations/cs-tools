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

-- approval_stage / approval_stage_approver mirror ServiceNow's generic
-- sysapproval_group / sysapproval_approver approval-workflow tables (see
-- digiops-cs/operations/csm-sync-service/configs/mappings/
-- sysapproval_group.yaml / sysapproval_approver.yaml -- content-synced with
-- that repo's migrations/0089_approval_stage_tables.sql, same convention as
-- "group"/000073_group_table.up.sql). Generic, not change-request-specific:
-- work_item_id can point at any approvable work_item type, same as
-- sysapproval_group.parent / sysapproval_approver.sysapproval can
-- reference any approvable SN task. Nullable FKs throughout (unlike
-- work_item_watcher's NOT NULL pattern): both mappings use
-- on_overflow: null_and_warn, since a row whose parent isn't a
-- currently-synced work_item type (e.g. a catalog request) is expected,
-- not an error.
CREATE TABLE IF NOT EXISTS approval_stage (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    work_item_id UUID REFERENCES work_item(id) ON DELETE CASCADE,
    assignment_group_id UUID REFERENCES "group"(id) ON DELETE SET NULL,
    -- Raw ServiceNow sysapproval_group.approval passthrough (e.g.
    -- "approved", "requested", "rejected", "not requested") - not the
    -- domain-facing per-stage status served to the CSM webapp, which is
    -- derived at read time from this stage's approval_stage_approver rows
    -- (first-responder-wins), mirroring ChangeRequestUtils._deriveStageStatus
    -- in apps/csm-portal/backend's SN adapter.
    raw_status VARCHAR(50)
);

CREATE INDEX IF NOT EXISTS idx_approval_stage_work_item_id ON approval_stage (work_item_id);
CREATE INDEX IF NOT EXISTS idx_approval_stage_assignment_group_id ON approval_stage (assignment_group_id);

-- work_item_id is denormalized alongside stage_id (rather than requiring
-- every reader to join through approval_stage to find the change request)
-- since the entity-service read path queries "all approvals for change
-- request X" directly - same shape as work_item_tag.work_item_id.
CREATE TABLE IF NOT EXISTS approval_stage_approver (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    stage_id UUID REFERENCES approval_stage(id) ON DELETE CASCADE,
    work_item_id UUID REFERENCES work_item(id) ON DELETE CASCADE,
    approver_user_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    -- Raw ServiceNow sysapproval_approver.state passthrough (requested /
    -- approved / rejected / not_required / cancelled / no_consensus) -
    -- normalised to the UPPER_SNAKE_CASE domain enum at read time in
    -- entity-service, not here.
    status VARCHAR(50),
    comments TEXT
);

CREATE INDEX IF NOT EXISTS idx_approval_stage_approver_stage_id ON approval_stage_approver (stage_id);
CREATE INDEX IF NOT EXISTS idx_approval_stage_approver_work_item_id ON approval_stage_approver (work_item_id);
CREATE INDEX IF NOT EXISTS idx_approval_stage_approver_approver_user_id ON approval_stage_approver (approver_user_id);
