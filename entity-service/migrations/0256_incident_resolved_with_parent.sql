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


-- Marks an incident resolved because its parent incident was: ServiceNow's
-- "Update Child Incidents" rule, ported to the incident update
-- (cascadeChildIncidentResolution). The cascade sets it on each child it
-- resolves; an incident resolved by its own update clears it. The portal shows
-- it on the incident, and the incident report drainer skips the
-- post-resolution follow-ups (workaround problem, alert tasks) for such a
-- child, so they are raised once, for the parent. Safe to run again.
ALTER TABLE incident ADD COLUMN IF NOT EXISTS resolved_with_parent BOOLEAN NOT NULL DEFAULT FALSE;
