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

-- Display-only columns, populated once at registration time, so
-- GET /cases/{caseId}/sla-clocks/{clockType} can return everything a Google
-- Chat breach card needs (case reference, title, type, product, team,
-- priority, state-at-registration) without csm-notification-service's
-- slaengine needing a second lookup at tick time — it has no other way to
-- reach case data. None of these participate in scheduling/breach logic.
ALTER TABLE sla_clocks
    ADD COLUMN case_number TEXT,
    ADD COLUMN wso2_case_id TEXT,
    ADD COLUMN case_title TEXT,
    ADD COLUMN case_type TEXT,
    ADD COLUMN product TEXT,
    ADD COLUMN team TEXT,
    ADD COLUMN priority TEXT,
    ADD COLUMN state TEXT;
