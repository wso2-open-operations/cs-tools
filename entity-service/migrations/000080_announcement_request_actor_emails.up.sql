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

-- Display-only companions to their own *_by column, which stores the IdP's
-- stable per-account "userid" -- opaque and not human-readable in the
-- portal's own UI (reported live: the Pending list's "Created by" column
-- showed a raw account id instead of anything readable). The *_by column
-- itself remains the real identity used for every creator-only check
-- (Publish/AddUpdate) and is untouched by this migration -- these columns
-- are purely for display, captured from the same actor's resolved email at
-- the moment of each action. NULL for every row written before this
-- migration, and for any actor whose email the caller couldn't resolve.
ALTER TABLE announcement_requests
  ADD COLUMN IF NOT EXISTS created_by_email TEXT,
  ADD COLUMN IF NOT EXISTS submitted_by_email TEXT,
  ADD COLUMN IF NOT EXISTS approved_by_email TEXT,
  ADD COLUMN IF NOT EXISTS published_by_email TEXT;

ALTER TABLE announcement_request_updates
  ADD COLUMN IF NOT EXISTS created_by_email TEXT;
