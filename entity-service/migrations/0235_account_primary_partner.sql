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

-- The Salesforce Id of the account's primary partner (Salesforce
-- Primary_Partner_Name2__c, ServiceNow u_primary_partner_account_id), written
-- by the Salesforce Account ingest. hasPrimaryPartner is true when it is set.
ALTER TABLE account ADD COLUMN IF NOT EXISTS primary_partner_sf_id VARCHAR(255);
