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

-- Widen the Salesforce-synced text columns that were narrower than the upstream
-- fields they are copied from (created in 0080 and 0081).
--
-- The ingest cut an over-long value and logged a warning, so the row was kept
-- but the data was silently damaged: eng_product_code (VARCHAR(40)) receives a
-- comma-separated list of product abbreviations of 200+ characters.
--
--   sf_opportunity_product.name                  200 -> VARCHAR(376)
--   sf_opportunity_product.eng_product_code      40  -> VARCHAR(255)
--   sf_opportunity_product.product_name          100 -> VARCHAR(255)
--   sf_opportunity_product.product_code          70  -> VARCHAR(255)
--   sf_opportunity_product.product_description   250 -> TEXT (upstream allows 4000)
--   sf_invoice.description                       40  -> VARCHAR(255)
--   sf_invoice.name                              40  -> VARCHAR(80)
--
-- No view, policy, trigger, generated column or check constraint references any
-- of these columns. product_name is the leading column of
-- idx_sf_opportunity_product_entitlement; widening VARCHAR(n) to a larger
-- VARCHAR or to TEXT is binary-coercible, so PostgreSQL does not rewrite the
-- table and reuses that index's storage.
--
-- Re-runnable: setting a column to the type it already has is a no-op. No
-- explicit transaction, so each ALTER TABLE is its own autocommit transaction
-- and releases its ACCESS EXCLUSIVE lock before the next table is touched. A
-- short session lock_timeout makes the file fail after 5s behind a long-running
-- transaction instead of queueing every other query behind it (as 100021).

SET lock_timeout = '5s';

ALTER TABLE sf_opportunity_product
    ALTER COLUMN name TYPE VARCHAR(376),
    ALTER COLUMN eng_product_code TYPE VARCHAR(255),
    ALTER COLUMN product_name TYPE VARCHAR(255),
    ALTER COLUMN product_code TYPE VARCHAR(255),
    ALTER COLUMN product_description TYPE TEXT;

ALTER TABLE sf_invoice
    ALTER COLUMN description TYPE VARCHAR(255),
    ALTER COLUMN name TYPE VARCHAR(80);

RESET lock_timeout;
