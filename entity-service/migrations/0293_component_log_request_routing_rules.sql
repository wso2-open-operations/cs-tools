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

-- Who is offered the "Component Log Request" catalog item (migration 0292):
--
--   PDP + CHOREO  private data plane customers' Choreo deployed products,
--                 next to that pair's existing log items (Request LB Logs 50,
--                 Request Product Logs 60).
--   CL  + CHOREO  cloud (CDP) customers' "WSO2 Developer Platform".
--
-- How far these reach depends on product.unit (catalog_repo.go,
-- availableCatalogItemsCTE): a deployed product whose product has no unit
-- matches a rule of any unit. Product units are not populated yet, so today
-- CL + CHOREO also reaches CL products with no unit (Asgardeo, Bijira,
-- Devant), but not those with a known other unit (Integrator: MI,
-- API Manager). Once units are populated it narrows to CHOREO products with
-- no further change here. A CL rule with no unit would instead keep matching
-- every CL product.
--
-- "order" is display order in ServiceNow; the Postgres catalog sorts items by
-- name. Fixed ids keep the rows identical across environments and the
-- migration safe to re-run.

INSERT INTO sr_category_routing_rule (id, created_on, updated_on, created_by, updated_by,
                                      "order", classification, product_unit, catalog_item_id)
SELECT r.id::UUID, now(), now(), 'system', 'system',
       r.ord, r.classification::sr_category_routing_rule_classification_enum,
       'CHOREO'::product_unit_enum, ci.id
FROM (VALUES
        ('546f92c4-9b38-4e7a-81ce-cb8451af87c6', 65, 'PDP'),
        ('c01dbd38-a398-40d3-b54e-4e2fe1a23db1', 65, 'CL')
     ) AS r (id, ord, classification)
JOIN catalog_item ci ON ci.id = '9d48f18f-0ae3-466b-b0ab-fdd14038864b'
ON CONFLICT (id) DO NOTHING;
