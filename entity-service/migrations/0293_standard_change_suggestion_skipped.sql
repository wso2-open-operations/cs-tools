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

-- A customer the standard change suggestion refresh could not compute in time
-- (too many change requests for its deadline) is skipped, not allowed to fail the
-- whole run, and the page says its suggestions were not computed in that refresh.
--
-- standard_change_suggestion_run_customer now also lists, when a run starts, every
-- customer it will compute (the account list is read once per run, not on every
-- page): change_requests is NULL until the customer is computed, and
-- skipped_reason is set when it was skipped. standard_change_suggestion_run.skipped
-- is the run's count of skipped customers, summed at publish like the others.
-- These tables are only written by the refresh, so the locks are short.
BEGIN;
SET LOCAL lock_timeout = '5s';

ALTER TABLE standard_change_suggestion_run
    ADD COLUMN IF NOT EXISTS skipped INTEGER NOT NULL DEFAULT 0;
ALTER TABLE standard_change_suggestion_run_customer
    ALTER COLUMN change_requests DROP NOT NULL;
ALTER TABLE standard_change_suggestion_run_customer
    ADD COLUMN IF NOT EXISTS skipped_reason VARCHAR(500);

COMMIT;
