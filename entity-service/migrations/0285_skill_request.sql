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

-- Skill management revamp (M5): an engineer asks for a skill the catalogue does
-- not have; a reviewer creates it, reactivates an inactive one, points at one
-- that already exists, or rejects the request with a reason. On approval the
-- skill is added to the requester's profile at the level they asked for
-- (auto_added_level_id / auto_add_note say what happened).

DO $$ BEGIN
    CREATE TYPE skill_request_status_enum AS ENUM ('PENDING', 'APPROVED', 'REJECTED', 'WITHDRAWN');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS skill_request (
    id                   UUID PRIMARY KEY DEFAULT uuidv7(),
    created_on           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_on           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by           VARCHAR(255),
    updated_by           VARCHAR(255),
    -- A user is soft-deleted in practice; a hard delete takes their requests
    -- (and, by the history cascade, the history rows that name them) with it.
    requested_by_user_id UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    requested_name       VARCHAR(255) NOT NULL,
    -- lower case, trimmed, inner whitespace collapsed: what "the same name" means
    normalized_name      VARCHAR(255) NOT NULL,
    category_id          UUID REFERENCES skill_category(id),
    suggested_category   VARCHAR(255),
    description          TEXT,
    requested_level_id   UUID NOT NULL REFERENCES skill_level(id),
    status               skill_request_status_enum NOT NULL DEFAULT 'PENDING',
    reviewed_by_user_id  UUID REFERENCES "user"(id) ON DELETE SET NULL,
    reviewed_on          TIMESTAMPTZ,
    review_comment       TEXT,
    -- CREATED | REACTIVATED | EXISTING, set on approval
    resolution           VARCHAR(20),
    resolved_skill_id    UUID REFERENCES skill(id),
    auto_added_level_id  UUID REFERENCES skill_level(id),
    -- ADDED | READDED | ALREADY_HELD | LEVEL_ADJUSTED | SKIPPED_INACTIVE_USER | SKIPPED_NO_SCALE
    auto_add_note        VARCHAR(40)
);

-- One pending request per name: a second asker is pointed at the first.
CREATE UNIQUE INDEX IF NOT EXISTS ux_skill_request_pending_name
    ON skill_request (normalized_name) WHERE status = 'PENDING';
CREATE INDEX IF NOT EXISTS idx_skill_request_requester
    ON skill_request (requested_by_user_id, created_on DESC);
CREATE INDEX IF NOT EXISTS idx_skill_request_status
    ON skill_request (status, created_on);
