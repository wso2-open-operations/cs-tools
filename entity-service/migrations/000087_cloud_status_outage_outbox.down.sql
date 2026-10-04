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

DROP TRIGGER IF EXISTS outage_affected_ci_outbox_update ON outage_affected_ci;
DROP TRIGGER IF EXISTS outage_affected_ci_outbox ON outage_affected_ci;
DROP TRIGGER IF EXISTS outage_outbox_insert ON outage;
DROP TRIGGER IF EXISTS outage_outbox ON outage;
DROP FUNCTION IF EXISTS trg_event_outbox_insert();
