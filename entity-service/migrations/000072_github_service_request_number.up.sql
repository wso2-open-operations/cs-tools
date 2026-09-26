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

-- Numbering for service requests raised from GitHub.
--
-- Migration 000070 added the change-request equivalent and explains the
-- reasoning for a distinct prefix rather than a shared sequence; the same
-- applies here. A service request is its own record type, so it gets its own
-- sequence rather than drawing from the change-request one -- the first
-- issue-created service requests were numbered CHG-GH-000007 and CHG-GH-000008
-- before this existed, which reads as a change request to anyone scanning the
-- table.
--
-- Service requests synced from ServiceNow are numbered CS0442152, sharing a
-- prefix with cases; SR-GH-000001 cannot collide with that by construction.
CREATE SEQUENCE IF NOT EXISTS github_service_request_number_seq START 1;

CREATE OR REPLACE FUNCTION next_github_service_request_number()
RETURNS TEXT AS $$
BEGIN
    RETURN 'SR-GH-' || LPAD(nextval('github_service_request_number_seq')::TEXT, 6, '0');
END;
$$ LANGUAGE plpgsql;

-- work_item.wso2_id is required for SERVICE_REQUEST by
-- work_item_wso2_id_required_by_type (000016), and like number it has no
-- default: generating one is an undecided product choice for records raised in
-- the portal. A record raised from a GitHub issue cannot wait for that
-- decision -- the insert simply fails without a value -- and it already draws
-- its number from a sequence here, so it draws this the same way.
--
-- Separate sequence rather than reusing the number's: the two are distinct
-- identifiers and nothing should imply they stay in step.
CREATE SEQUENCE IF NOT EXISTS github_service_request_wso2_id_seq START 1;

CREATE OR REPLACE FUNCTION next_github_service_request_wso2_id()
RETURNS TEXT AS $$
BEGIN
    RETURN 'WSO2-GH-' || LPAD(nextval('github_service_request_wso2_id_seq')::TEXT, 6, '0');
END;
$$ LANGUAGE plpgsql;
