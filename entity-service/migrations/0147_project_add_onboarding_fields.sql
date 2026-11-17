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
    CREATE TYPE onboarding_stage_enum AS ENUM (
        'GOLIVE_PENDING', 'LITE_SESSION_COMPLETED', 'LITE_SESSION_PENDING',
        'MILESTONE_PLAN_FINALISED', 'MILESTONE_PLAN_PENDING', 'SOLUTION_FINALISED',
        'SOLUTION_IMPLEMENTATION', 'SOLUTION_PENDING', 'SOLUTION_REVIEW_COMPLETED',
        'SOLUTION_REVIEW_PENDING'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

ALTER TABLE project ADD COLUMN IF NOT EXISTS onboarding_completion_date DATE;
ALTER TABLE project ADD COLUMN IF NOT EXISTS onboarding_start_on DATE;
ALTER TABLE project ADD COLUMN IF NOT EXISTS onboarding_stage onboarding_stage_enum;
