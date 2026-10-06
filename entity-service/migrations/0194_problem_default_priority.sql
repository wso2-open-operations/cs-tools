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

-- Give problems created without a priority the one ServiceNow would have:
-- its "Priority Problem Lookup" derives a problem's priority from impact x
-- urgency (dl_problem_priority, the same nine rows as the incident lookup --
-- discovery scripts 63-65), and a new problem's impact and urgency default
-- to 3 - Low. Postgres-only creates set no priority, and dual-write creates
-- dropped the one ServiceNow returned; both now set it at create time, and
-- this fills in the problems created before that.
--
-- Only rows with no priority are touched: a synced problem always carries
-- ServiceNow's own. A missing impact or urgency is taken as Low and stored.
-- Re-running it changes nothing.

UPDATE problem
SET impact   = COALESCE(impact, 'LOW'::problem_impact_enum),
    urgency  = COALESCE(urgency, 'LOW'::problem_urgency_enum),
    priority = (ARRAY['CRITICAL', 'HIGH', 'MODERATE', 'LOW', 'PLANNING']::problem_priority_enum[])[
                 1 + (CASE COALESCE(impact::text, 'LOW')  WHEN 'HIGH' THEN 0 WHEN 'MEDIUM' THEN 1 ELSE 2 END)
                   + (CASE COALESCE(urgency::text, 'LOW') WHEN 'HIGH' THEN 0 WHEN 'MEDIUM' THEN 1 ELSE 2 END)
               ]
WHERE priority IS NULL;
