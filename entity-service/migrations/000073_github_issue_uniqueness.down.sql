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

ALTER TABLE github_webhook_delivery
    DROP CONSTRAINT IF EXISTS github_webhook_delivery_work_item_id_fkey;

ALTER TABLE github_webhook_delivery
    ADD CONSTRAINT github_webhook_delivery_change_request_id_fkey
    FOREIGN KEY (change_request_id) REFERENCES change_request(id) ON DELETE SET NULL;

DROP INDEX IF EXISTS work_item_account_github_issue_uniq;
