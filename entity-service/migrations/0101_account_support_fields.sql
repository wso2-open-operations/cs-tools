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

-- Value sets verified against ServiceNow's Choices table for u_support_timezone/
-- u_support_tier on Account; see CLAUDE.md section 8's enum-column convention.
DO $$ BEGIN
    CREATE TYPE support_timezone_enum AS ENUM ('M_F_ET', 'M_F_GMT', 'M_F_IST', 'T_S_GMT');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE support_tier_enum AS ENUM ('BASIC', 'ENTERPRISE');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

ALTER TABLE account ADD COLUMN IF NOT EXISTS support_timezone support_timezone_enum;
ALTER TABLE account ADD COLUMN IF NOT EXISTS support_tier support_tier_enum;
