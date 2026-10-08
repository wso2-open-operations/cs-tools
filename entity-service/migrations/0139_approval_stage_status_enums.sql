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

DO $$ BEGIN
    CREATE TYPE approval_stage_status_enum AS ENUM (
        'REQUESTED',
        'APPROVED',
        'REJECTED',
        'NOT_REQUESTED',
        'NOT_REQUIRED',
        'DUPLICATE',
        'CANCELLED',
        'UNKNOWN'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE approval_stage_approver_state_enum AS ENUM (
        'REQUESTED',
        'APPROVED',
        'REJECTED',
        'NOT_REQUESTED',
        'NOT_REQUIRED',
        'NOT_ENTITLED',
        'CANCELLED',
        'NO_CONSENSUS',
        'UNKNOWN'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

UPDATE approval_stage
SET raw_status = CASE lower(trim(raw_status))
    WHEN 'requested' THEN 'REQUESTED'
    WHEN 'approved' THEN 'APPROVED'
    WHEN 'rejected' THEN 'REJECTED'
    WHEN 'not requested' THEN 'NOT_REQUESTED'
    WHEN 'not_requested' THEN 'NOT_REQUESTED'
    WHEN 'not required' THEN 'NOT_REQUIRED'
    WHEN 'not_required' THEN 'NOT_REQUIRED'
    WHEN 'duplicate' THEN 'DUPLICATE'
    WHEN 'cancelled' THEN 'CANCELLED'
    WHEN 'unknown' THEN 'UNKNOWN'
    ELSE 'UNKNOWN'
END
WHERE raw_status IS NOT NULL;

UPDATE approval_stage_approver
SET state = CASE lower(trim(state))
    WHEN 'requested' THEN 'REQUESTED'
    WHEN 'approved' THEN 'APPROVED'
    WHEN 'rejected' THEN 'REJECTED'
    WHEN 'not requested' THEN 'NOT_REQUESTED'
    WHEN 'not_requested' THEN 'NOT_REQUESTED'
    WHEN 'not required' THEN 'NOT_REQUIRED'
    WHEN 'not_required' THEN 'NOT_REQUIRED'
    WHEN 'not entitled' THEN 'NOT_ENTITLED'
    WHEN 'not_entitled' THEN 'NOT_ENTITLED'
    WHEN 'cancelled' THEN 'CANCELLED'
    WHEN 'no consensus' THEN 'NO_CONSENSUS'
    WHEN 'no_consensus' THEN 'NO_CONSENSUS'
    WHEN 'unknown' THEN 'UNKNOWN'
    ELSE 'UNKNOWN'
END
WHERE state IS NOT NULL;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'approval_stage'
          AND column_name = 'raw_status'
          AND udt_name <> 'approval_stage_status_enum'
    ) THEN
        ALTER TABLE approval_stage
        ALTER COLUMN raw_status TYPE approval_stage_status_enum
        USING raw_status::approval_stage_status_enum;
    END IF;
END$$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'approval_stage_approver'
          AND column_name = 'state'
          AND udt_name <> 'approval_stage_approver_state_enum'
    ) THEN
        ALTER TABLE approval_stage_approver
        ALTER COLUMN state TYPE approval_stage_approver_state_enum
        USING state::approval_stage_approver_state_enum;
    END IF;
END$$;
