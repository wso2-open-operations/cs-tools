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

-- PLG: health defaults to NONE (shown as "-") until an engineer marks it
-- HEALTHY or AT_RISK. A pairing whose health no engineer ever set, still at the
-- old HEALTHY default with no health change in its history, becomes NONE.
-- Playbooks are unchanged: NONE offers what HEALTHY did. Safe to re-run.

ALTER TABLE plg_org_platform ALTER COLUMN health_state SET DEFAULT 'NONE';

UPDATE plg_org_platform op
SET    health_state = 'NONE'
WHERE  op.health_state = 'HEALTHY'
  AND  op.health_updated_by IS NULL
  AND  NOT EXISTS (SELECT 1 FROM plg_lifecycle_history h
                   WHERE  h.org_platform_id = op.id AND h.to_health IS NOT NULL);
