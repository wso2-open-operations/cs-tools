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

-- Lets a project member start a Novera chat. Migration 0146 made conversation
-- INSERT internal-only because nothing created conversations natively then;
-- every row was synced in from ServiceNow's u_chat_conversation. Now
-- POST /conversations writes here on behalf of the customer, so the policy
-- matches case_write (migration 100008): internal, or a member of the
-- work_item's project. The work_item half is already covered by
-- work_item_write. The is_internal check is pre-wrapped in a scalar
-- subquery, the form migration 100015 rewrote every policy into.

SET lock_timeout = '5s';

DROP POLICY IF EXISTS conversation_write_internal_only ON conversation;
DROP POLICY IF EXISTS conversation_write ON conversation;

CREATE POLICY conversation_write ON conversation
  FOR INSERT WITH CHECK (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR is_project_member((SELECT wi.project_id FROM work_item wi WHERE wi.id = conversation.id))
  );
