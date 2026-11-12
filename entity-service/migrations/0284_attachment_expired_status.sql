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


-- A work item's attachments are removed from storage a set time after the work item
-- is closed (the attachment retention job). The row stays, so a list can say why the
-- file is gone, and is told apart from a delete by a person. status gains EXPIRED.

ALTER TYPE attachment_status_enum ADD VALUE IF NOT EXISTS 'EXPIRED';
