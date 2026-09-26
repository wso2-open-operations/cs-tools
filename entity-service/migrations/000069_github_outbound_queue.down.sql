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

DROP TRIGGER IF EXISTS case_github_outbound ON "case";
DROP FUNCTION IF EXISTS trg_github_outbound_case();
DROP TRIGGER IF EXISTS work_item_assignment_github_outbound ON work_item;
DROP FUNCTION IF EXISTS trg_github_outbound_assignment();
DROP TRIGGER IF EXISTS comment_github_outbound ON comment;
DROP TRIGGER IF EXISTS change_request_github_outbound ON change_request;
DROP FUNCTION IF EXISTS trg_github_outbound_comment();
DROP FUNCTION IF EXISTS trg_github_outbound_cr();
DROP TABLE IF EXISTS github_outbound_queue;
