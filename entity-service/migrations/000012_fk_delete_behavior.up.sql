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

-- Replaces the default RESTRICT delete behavior with CASCADE (composition:
-- row is meaningless without its parent) or SET NULL (the column is just an
-- attribute of the row) per relationship.

ALTER TABLE user_role DROP CONSTRAINT IF EXISTS user_role_user_id_fkey;
ALTER TABLE user_role ADD CONSTRAINT user_role_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES "user"(id) ON DELETE CASCADE;

ALTER TABLE user_role DROP CONSTRAINT IF EXISTS user_role_role_id_fkey;
ALTER TABLE user_role ADD CONSTRAINT user_role_role_id_fkey
    FOREIGN KEY (role_id) REFERENCES role(id) ON DELETE CASCADE;

ALTER TABLE product_version DROP CONSTRAINT IF EXISTS product_version_product_id_fkey;
ALTER TABLE product_version ADD CONSTRAINT product_version_product_id_fkey
    FOREIGN KEY (product_id) REFERENCES product(id) ON DELETE CASCADE;

ALTER TABLE project DROP CONSTRAINT IF EXISTS project_account_id_fkey;
ALTER TABLE project ADD CONSTRAINT project_account_id_fkey
    FOREIGN KEY (account_id) REFERENCES account(id) ON DELETE CASCADE;

ALTER TABLE account DROP CONSTRAINT IF EXISTS account_customer_success_manager_id_fkey;
ALTER TABLE account ADD CONSTRAINT account_customer_success_manager_id_fkey
    FOREIGN KEY (customer_success_manager_id) REFERENCES "user"(id) ON DELETE SET NULL;

ALTER TABLE account DROP CONSTRAINT IF EXISTS account_technical_owner_id_fkey;
ALTER TABLE account ADD CONSTRAINT account_technical_owner_id_fkey
    FOREIGN KEY (technical_owner_id) REFERENCES "user"(id) ON DELETE SET NULL;

ALTER TABLE account DROP CONSTRAINT IF EXISTS account_secondary_technical_owner_id_fkey;
ALTER TABLE account ADD CONSTRAINT account_secondary_technical_owner_id_fkey
    FOREIGN KEY (secondary_technical_owner_id) REFERENCES "user"(id) ON DELETE SET NULL;

ALTER TABLE account DROP CONSTRAINT IF EXISTS account_account_manager_id_fkey;
ALTER TABLE account ADD CONSTRAINT account_account_manager_id_fkey
    FOREIGN KEY (account_manager_id) REFERENCES "user"(id) ON DELETE SET NULL;
