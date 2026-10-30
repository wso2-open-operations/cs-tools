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

-- Service request product categories for CSM staff.
--
-- sr_product_categories (0130) is the customer-facing allow-list: which
-- deployed products a service request may be raised against. Both portals
-- read it, so widening it would also change the customer portal.
-- csm_sr_product_categories is the list used instead when WSO2 staff raise
-- the request from the CSM portal. NULL or empty means "same as
-- sr_product_categories", so every other project type is unchanged.
--
-- Cloud Support adds CL: Cloud Support projects without a PDP subscription
-- (CDP customers) hold "WSO2 Developer Platform", Asgardeo and Devant
-- deployed products categorised CL, so a CRE engineer can raise a service
-- request (e.g. a component log request) for them. Customers still get
-- {PDP} only.

ALTER TABLE project_type
    ADD COLUMN IF NOT EXISTS csm_sr_product_categories deployed_product_category_enum[];

DO $$
BEGIN
    UPDATE project_type
       SET csm_sr_product_categories = ARRAY['PDP', 'CL']::deployed_product_category_enum[]
     WHERE name = 'Cloud Support';
    IF NOT FOUND THEN
        RAISE NOTICE 'migration 0264: no project_type row ''Cloud Support'' -- csm_sr_product_categories left unset';
    END IF;
END $$;
