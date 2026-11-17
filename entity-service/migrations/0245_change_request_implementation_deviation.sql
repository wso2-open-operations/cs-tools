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

-- "Deviation from the implementation plan": what the implementation (or the
-- review) actually did differently from the change request's implementation
-- plan, as rich text. Unlike the plan itself it is written after the approval:
-- it is recorded from Implement on, until the change request is closed, canceled
-- or rolled back (PatchChangeRequest; it is not one of approvedLockFields). It is
-- internal: only WSO2 staff read or write it. The previous system has no such
-- field, so it lives here only and is never mirrored.
--
-- Additive and idempotent: safe to re-run against a database that already has it.
-- Adding the column locks change_request against reads and writes for a moment (a
-- nullable column with no default rewrites nothing), so the wait for that lock is
-- bounded.
SET lock_timeout = '5s';

ALTER TABLE change_request ADD COLUMN IF NOT EXISTS implementation_deviation TEXT;

RESET lock_timeout;
