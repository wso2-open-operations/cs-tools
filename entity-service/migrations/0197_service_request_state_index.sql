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

-- See migration 0195's own doc comment -- the third of the four indexes
-- kept for whenever SearchCases' state filter is reattempted as a sargable
-- rewrite; not currently usable by the live COALESCE-based predicate.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_service_request_state ON service_request (state);
