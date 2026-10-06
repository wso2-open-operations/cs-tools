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

-- Rename approval_stage_approver.status -> state and normalize values to
-- UPPER_SNAKE_CASE in the sync layer (previously done at read time in entity-service).
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'approval_stage_approver' AND column_name = 'status'
    ) THEN
        ALTER TABLE approval_stage_approver RENAME COLUMN status TO state;
    END IF;
END$$;

UPDATE approval_stage_approver
SET state = CASE state
    WHEN 'requested'     THEN 'REQUESTED'
    WHEN 'approved'      THEN 'APPROVED'
    WHEN 'rejected'      THEN 'REJECTED'
    WHEN 'not requested' THEN 'NOT_REQUESTED'
    WHEN 'not_required'  THEN 'NOT_REQUIRED'
    WHEN 'cancelled'     THEN 'CANCELLED'
    WHEN 'not_entitled'  THEN 'NOT_ENTITLED'
    ELSE state
END
WHERE state IS NOT NULL;
