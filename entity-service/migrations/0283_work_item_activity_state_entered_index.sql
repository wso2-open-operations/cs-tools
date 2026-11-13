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

-- Index for the case search's stateEntered / stateEnteredOn filters
-- ("cases moved to Waiting on WSO2 today").
--
-- Why: case_repo.go's buildCaseSearchWhere adds
--   EXISTS (SELECT 1 FROM work_item_activity sa WHERE sa.work_item_id = wi.id
--     AND sa.field_name = 'state' AND LOWER(sa.new_value) = ANY($n)
--     AND sa.created_on >= $m ...)
-- and the only index on work_item_activity is on work_item_id. Over all cases
-- the planner seq-scans the whole table (severity, assignee and every other
-- field change included) and hash-joins it to the cases: ~100 ms on 1.5M
-- activity rows, growing with the table. This partial index holds only the
-- state rows, keyed by the lower-cased label and the time, so the planner
-- reads just the window's changes into those states: 2-3 ms for "today",
-- ~25 ms for "this month" on the same data (35 MB index for 1.5M rows).
--
-- Applied by `make migrate` as one psql -f per file inside the runner's own
-- bookkeeping, so CONCURRENTLY is not used, as in
-- 0278_user_user_name_lower_index.sql. The build took 0.5 s on 1.5M rows;
-- inserts into work_item_activity wait for it, reads do not.
CREATE INDEX IF NOT EXISTS idx_work_item_activity_state_entered
    ON work_item_activity (LOWER(new_value), created_on)
    WHERE field_name = 'state';
