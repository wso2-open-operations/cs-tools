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

-- Team Schedule: stop offering the Onboarding (ONB) tag.
--
-- Onboarding is no longer a team (that work moved to FDE), so a lead must not
-- be able to mark somebody ONB from a cell. The catalogue seeds ONBOARDING as
-- retired already, but a database whose row was activated since, or whose
-- lead added a tag of their own spelt ONB, still offers it.
--
-- Retired, not deleted: an absence recorded as ONB keeps its label on the
-- days it covers, and the tag can be switched back on if FDE ever wants it.
-- Safe to re-run: it only touches rows that are still active.
UPDATE team_schedule_absence_kind
   SET is_active = FALSE,
       updated_on = NOW(),
       updated_by = 'migration'
 WHERE upper(short_code) = 'ONB'
   AND is_active;
