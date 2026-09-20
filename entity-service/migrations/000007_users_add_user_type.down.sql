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

DROP TRIGGER IF EXISTS role_name_change_recompute_type ON role;
DROP TRIGGER IF EXISTS user_roles_change_recompute_type ON user_role;
DROP TRIGGER IF EXISTS users_is_system_user_change ON "user";
DROP FUNCTION IF EXISTS trg_role_name_recompute_type();
DROP FUNCTION IF EXISTS trg_user_roles_recompute_type();
DROP FUNCTION IF EXISTS trg_users_recompute_type();
DROP FUNCTION IF EXISTS recompute_user_type(UUID);
ALTER TABLE "user" DROP COLUMN IF EXISTS user_type;
DROP TYPE IF EXISTS user_type_enum;
