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

-- Point product_repo_mapping at the catalogue names cases actually send.
-- 0120 stored the sheet's short label in product_name, so MatchProductRepo
-- never hit those rows. 0120 has already run, and ON CONFLICT DO NOTHING
-- will not revisit those rows. 0139 already set github_label from the new
-- sheet, keyed by the short names, so these updates leave github_label alone.
--
-- WSO2 Developer Platform (Choreo) stays. The lookup strips a trailing
-- "(...)", which is how a case on "WSO2 Developer Platform" resolves.
-- "Choreo" is a separate catalogue product and needs its own row.
--
-- "WSO2 Integrator" is also a separate catalogue product from
-- "WSO2 Integrator: MI". The sheet has a repo only for Integrator: MI, so
-- this row files "WSO2 Integrator" in that same repo with Product/mi.
-- The two names do not prefix-match: the next character is ":", not a space.

UPDATE product_repo_mapping SET product_name = 'Asgardeo', updated_by = 'migration', updated_on = NOW() WHERE product_name = 'WSO2 Identity Platform (Asgardeo)';
UPDATE product_repo_mapping SET product_name = 'Choreo Analytics', updated_by = 'migration', updated_on = NOW() WHERE product_name = 'Choreo-Analytics';
UPDATE product_repo_mapping SET product_name = 'WSO2 Choreo Connect', repository = 'wso2-apim-internal', updated_by = 'migration', updated_on = NOW() WHERE product_name = 'Choreo-Connect';
UPDATE product_repo_mapping SET product_name = 'WSO2 Microgateway', updated_by = 'migration', updated_on = NOW() WHERE product_name = 'Microgateway';
UPDATE product_repo_mapping SET product_name = 'WSO2 Customer Service Portal', abbreviation = 'CSP', updated_by = 'migration', updated_on = NOW() WHERE product_name = 'CSP';

UPDATE product_repo_mapping SET product_name = 'WSO2 API Platform AI Gateway', abbreviation = 'APAG', updated_by = 'migration', updated_on = NOW() WHERE product_name = 'APAG';
UPDATE product_repo_mapping SET product_name = 'WSO2 API Platform for Kubernetes', abbreviation = 'APK', updated_by = 'migration', updated_on = NOW() WHERE product_name = 'APK';
UPDATE product_repo_mapping SET product_name = 'WSO2 API Platform Gateway', abbreviation = 'APG', updated_by = 'migration', updated_on = NOW() WHERE product_name = 'APG';

UPDATE product_repo_mapping SET product_name = 'WSO2 Enterprise Integrator', abbreviation = 'EI', updated_by = 'migration', updated_on = NOW() WHERE product_name = 'EI';
UPDATE product_repo_mapping SET product_name = 'WSO2 Enterprise Integrator Analytics', abbreviation = 'EI-Analytics', updated_by = 'migration', updated_on = NOW() WHERE product_name = 'EI-Analytics';
UPDATE product_repo_mapping SET product_name = 'WSO2 Enterprise Service Bus', abbreviation = 'ESB', updated_by = 'migration', updated_on = NOW() WHERE product_name = 'ESB';
UPDATE product_repo_mapping SET product_name = 'WSO2 Integrator: BI', abbreviation = 'BI', updated_by = 'migration', updated_on = NOW() WHERE product_name = 'BI';
UPDATE product_repo_mapping SET product_name = 'WSO2 Integrator: ICP', abbreviation = 'ICP', updated_by = 'migration', updated_on = NOW() WHERE product_name = 'ICP';
UPDATE product_repo_mapping SET product_name = 'WSO2 Integrator: MI', abbreviation = 'MI', updated_by = 'migration', updated_on = NOW() WHERE product_name = 'MI';
UPDATE product_repo_mapping SET product_name = 'WSO2 Integrator: SI', abbreviation = 'SI', updated_by = 'migration', updated_on = NOW() WHERE product_name = 'SI';

UPDATE product_repo_mapping SET product_name = 'WSO2 Open Banking AM', abbreviation = 'OB-AM', updated_by = 'migration', updated_on = NOW() WHERE product_name = 'OB-AM';
UPDATE product_repo_mapping SET product_name = 'WSO2 Open Banking BI', abbreviation = 'OB-BI', updated_by = 'migration', updated_on = NOW() WHERE product_name = 'OB-BI';
UPDATE product_repo_mapping SET product_name = 'WSO2 Open Banking IAM', abbreviation = 'OB-IAM', updated_by = 'migration', updated_on = NOW() WHERE product_name = 'OB-IAM';
UPDATE product_repo_mapping SET product_name = 'WSO2 Open Banking KM', abbreviation = 'OB-KM', updated_by = 'migration', updated_on = NOW() WHERE product_name = 'OB-KM';

INSERT INTO product_repo_mapping (
    created_by, updated_by, product_name, abbreviation, owner, repository, github_label
) VALUES
    ('migration', 'migration', 'Choreo', NULL, 'wso2-enterprise', 'choreo', 'product/choreo'),
    ('migration', 'migration', 'WSO2 Integrator', NULL, 'wso2-enterprise', 'wso2-integration-internal', 'Product/mi'),
    ('migration', 'migration', 'MI Dashboard', NULL, 'wso2-enterprise', 'wso2-integration-internal', 'Product/icp'),
    ('migration', 'migration', 'WSO2 MI Monitoring Dashboard', NULL, 'wso2-enterprise', 'wso2-integration-internal', 'Product/icp')
ON CONFLICT (product_name) DO NOTHING;
