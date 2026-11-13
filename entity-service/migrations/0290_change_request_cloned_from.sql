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

-- Which change request a change request was cloned from (the portal's Clone).
-- Recorded on create (POST /change-requests clonedFromChangeRequestId), never
-- changed afterwards. ON DELETE SET NULL: deleting the source leaves the clone,
-- which only loses the link. Read as `clonedFrom` on the detail, and by the
-- standard change suggestions (a clone joins its source's cluster directly).
-- Existing change requests have no recorded source: a clone made before this
-- cannot be told apart from a change typed from scratch, so nothing is backfilled.

-- One transaction with a short lock timeout: adding the column (with its foreign
-- key) takes an exclusive lock on change_request, which is written all day, so
-- rather than queue every write behind it this gives up after 5s; re-run it when
-- the table is quiet. Its index is built without blocking writes, in 0292.
BEGIN;
SET LOCAL lock_timeout = '5s';

ALTER TABLE change_request
    ADD COLUMN IF NOT EXISTS cloned_from_change_request_id UUID
        REFERENCES change_request(id) ON DELETE SET NULL;

COMMIT;
