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

-- A customer the refresh skips (too large to compute in time) gets no snapshot
-- rows in that run, and publishing deletes the previous run, so the run after
-- would find no previous cluster keys for it and mint them afresh: a subject that
-- split could swap its bare and suffixed keys, moving a dismissal to the other
-- cluster. So a skip carries the customer's cluster keys (and their member ids)
-- over from the latest published run into this one, here, as
-- [{"key": ..., "memberIds": [...]}]. Only the next refresh reads them, to keep
-- the keys; GET and the counts never do. NULL when nothing was carried.

BEGIN;
SET LOCAL lock_timeout = '5s';

ALTER TABLE standard_change_suggestion_run_customer
    ADD COLUMN IF NOT EXISTS carried_keys JSONB;

COMMIT;
