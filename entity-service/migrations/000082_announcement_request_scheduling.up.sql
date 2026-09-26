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

-- due_on: set once by Submit (now() + one month) -- purely informational
-- display in this slice, never enforced or acted on by this service.
--
-- scheduled_on: set/cleared only via the dedicated Schedule action, never by
-- the generic content Update. When set on an approved row,
-- operations/csm-scheduled-tasks' "publish_scheduled_announcements" sub-cron
-- publishes it automatically once this time arrives -- exactly as if a human
-- had clicked Publish. Left as-is after MarkPublished (a harmless historical
-- value; the read side never queries it once the row leaves "approved").
ALTER TABLE announcement_requests ADD COLUMN due_on TIMESTAMPTZ;
ALTER TABLE announcement_requests ADD COLUMN scheduled_on TIMESTAMPTZ;
