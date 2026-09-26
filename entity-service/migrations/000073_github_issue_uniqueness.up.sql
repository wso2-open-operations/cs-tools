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

-- Two fixes that both come from one observed failure.
--
-- Opening a single issue produced TWO service requests, 480ms apart, from two
-- distinct GitHub deliveries (an issue arrives as more than one event: opened,
-- then labeled). The code checked "does a record exist for this issue" and then
-- inserted; both deliveries passed the check before either insert landed. A
-- read cannot close that window -- the constraint has to live here, so the
-- second writer loses regardless of timing.
--
-- Scoped to the account, not global: two accounts may legitimately map
-- repositories whose issue numbers collide, since an issue number is only
-- unique within a repository. Partial, because most work items have no issue.
CREATE UNIQUE INDEX IF NOT EXISTS work_item_account_github_issue_uniq
    ON work_item (account_id, github_issue_number)
    WHERE github_issue_number IS NOT NULL;

-- The same delivery also exposed a stale foreign key. This column was added
-- when an issue could only ever become a change request; issues now become
-- service requests, which have no change_request row, so linking one failed
-- with a foreign-key violation and the replay protection quietly stopped
-- recording what a delivery did. work_item is the table both types share.
ALTER TABLE github_webhook_delivery
    DROP CONSTRAINT IF EXISTS github_webhook_delivery_change_request_id_fkey;

ALTER TABLE github_webhook_delivery
    ADD CONSTRAINT github_webhook_delivery_work_item_id_fkey
    FOREIGN KEY (change_request_id) REFERENCES work_item(id) ON DELETE SET NULL;

COMMENT ON COLUMN github_webhook_delivery.change_request_id IS
    'The work item this delivery resolved to -- a change request or a service '
    'request. Named change_request_id from when only the former was possible.';
