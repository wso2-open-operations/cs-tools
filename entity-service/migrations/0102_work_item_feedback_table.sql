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

-- id is the assessment instance's own sys_id, not shared with work_item -
-- rating/comment are patched in later by separate asmt_metric_result
-- mappings (see work_item_feedback_rating.yaml/work_item_feedback_comment.yaml),
-- so work_item_id is a normal FK, not a shared primary key.
CREATE TABLE IF NOT EXISTS work_item_feedback (
    id UUID PRIMARY KEY,
    work_item_id UUID NOT NULL UNIQUE REFERENCES work_item(id) ON DELETE CASCADE,
    rating INTEGER CONSTRAINT chk_work_item_feedback_rating CHECK (rating BETWEEN 1 AND 5),
    rating_label VARCHAR(100),
    comment TEXT,
    submitted_by_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    submitted_at TIMESTAMPTZ,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_work_item_feedback_submitted_by_id ON work_item_feedback (submitted_by_id);
