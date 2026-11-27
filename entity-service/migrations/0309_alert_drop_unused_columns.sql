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

-- Drops the alert table's claim columns (claimed_by, claim_expires_on, processed_on) and their partial indexes. Nothing in entity-service reads or writes them; incident_id records which incident an alert belongs to. Safe to run again.
-- Run only after the data migration that needs them has finished: dropping a column discards its values.

DROP INDEX IF EXISTS alert_unprocessed_idx;
DROP INDEX IF EXISTS alert_unprocessed_fingerprint_idx;

ALTER TABLE alert
    DROP COLUMN IF EXISTS claimed_by,
    DROP COLUMN IF EXISTS claim_expires_on,
    DROP COLUMN IF EXISTS processed_on;
