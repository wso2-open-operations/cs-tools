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

-- Native enum: u_announcement_type is a 2-value SN choice field (General=1,
-- Security=2), confirmed against the field's actual choice list.
DO $$ BEGIN
    CREATE TYPE announcement_type_enum AS ENUM ('GENERAL', 'SECURITY');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

ALTER TABLE announcement ADD COLUMN IF NOT EXISTS announcement_type announcement_type_enum NOT NULL DEFAULT 'GENERAL';
