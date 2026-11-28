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

-- Saved case comment templates for the CSM portal: reusable comment and work
-- note text an engineer inserts into the comment box instead of retyping it.
--
-- owner_user_id set: a personal template, seen and changed only by its owner.
-- owner_user_id NULL: a shared template, seen by every CSM portal user who can
-- comment and changed only by an admin (the CSM portal backend's PermAdmin).
--
-- applies_to says which entry a template is offered for: COMMENT (a public,
-- customer-visible comment), WORK_NOTE (an internal note) or BOTH.
--
-- Additive and safe to re-run.
CREATE TABLE IF NOT EXISTS case_comment_template (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id UUID         REFERENCES "user" (id) ON DELETE CASCADE,
    name          VARCHAR(100) NOT NULL,
    body          TEXT         NOT NULL,
    applies_to    VARCHAR(16)  NOT NULL,
    created_on    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_by    VARCHAR(255),
    updated_on    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_by    VARCHAR(255),
    CONSTRAINT case_comment_template_applies_to_check
        CHECK (applies_to IN ('COMMENT', 'WORK_NOTE', 'BOTH'))
);

-- One name per owner, ignoring case; the shared templates (owner NULL) are one
-- more "owner" of their own, so a personal template may reuse a shared name.
CREATE UNIQUE INDEX IF NOT EXISTS case_comment_template_owner_name
    ON case_comment_template (
        COALESCE(owner_user_id, '00000000-0000-0000-0000-000000000000'::uuid),
        LOWER(name)
    );

CREATE INDEX IF NOT EXISTS idx_case_comment_template_owner
    ON case_comment_template (owner_user_id);
