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

-- Two more tables from the same MQ-fed Salesforce pipeline as 0080. Both are
-- standalone (no super_class), both written solely by mq-integration-user.
-- Shapes from the sys_dictionary dump of 2026-09-23 (cs-tools discovery
-- script 43, PASS 14).

-- u_sf_invoice -> sf_invoice. 306 rows on DEV. Hangs off an opportunity, so it
-- must load after sf_opportunity (0080).
--
-- Every column here is a u_ customisation apart from the sys_* six: this is a
-- WSO2 table that happens to live in the Global scope, not a baseline one.
CREATE TABLE IF NOT EXISTS sf_invoice (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    -- u_id: the Salesforce record id (a0IE… — an `a`-prefixed custom object).
    sf_id VARCHAR(40),
    -- u_name is a short code (e.g. "2508"), not a title.
    name VARCHAR(40),
    description VARCHAR(40),
    classification VARCHAR(40),
    opportunity_id UUID REFERENCES sf_opportunity(id) ON DELETE SET NULL,
    invoiced_amount NUMERIC,
    invoice_date DATE,
    -- Three separate due dates on the source, all kept: the ACP invoice-closure
    -- flows distinguish the current due date from the one originally agreed,
    -- and `paid` is what closes the loop.
    invoiced_due_date DATE,
    original_invoice_due_date DATE,
    invoiced_paid_date DATE,
    service_start_date DATE,
    service_end_date DATE,
    sync_time_stamp TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_sf_invoice_opportunity_id ON sf_invoice (opportunity_id);
CREATE INDEX IF NOT EXISTS idx_sf_invoice_sf_id ON sf_invoice (sf_id);
-- "which invoices are overdue and unpaid" is the question the closure flows
-- ask, so it gets an index rather than a scan.
CREATE INDEX IF NOT EXISTS idx_sf_invoice_due_unpaid
    ON sf_invoice (invoiced_due_date)
    WHERE invoiced_paid_date IS NULL;

-- account_relationship -> account_relationship. 58 rows on DEV.
--
-- Unlike everything else in this family this is a ServiceNow BASELINE CSM
-- table: 0 custom columns, 13 baseline. The name therefore needs no
-- de-prefixing, and the column names are ServiceNow's own.
--
-- from_company/to_company are renamed to from_account_id/to_account_id:
-- "company" is ServiceNow's legacy word for what this repo consistently calls
-- an account, and both columns reference customer_account.
CREATE TABLE IF NOT EXISTS account_relationship (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    from_account_id UUID REFERENCES account(id) ON DELETE CASCADE,
    to_account_id UUID REFERENCES account(id) ON DELETE CASCADE,
    -- relationship_type references sn_customerservice_account_relationship_type,
    -- which this repo does not sync, so it is carried as a raw sys_id string —
    -- the same treatment, and the same VARCHAR(32) `_id` naming, that 0079
    -- gives its own unsynced references (engagement_type_id, opportunity_id).
    -- The two label columns below already carry the human-readable form, so
    -- nothing is lost by not resolving it.
    relationship_type_id VARCHAR(32),
    relationship_label VARCHAR(40),
    reverse_relationship_label VARCHAR(40),
    -- A row is stored once and read from either end; this flags which
    -- direction the labels are written for.
    is_reverse_relationship BOOLEAN,
    -- "order" is a reserved word, quoted elsewhere in this repo (0063, 0065).
    -- Renamed rather than quoted here: nothing builds this table's SQL
    -- dynamically, so the quoting has no upside and a plain name is easier to
    -- use from Go.
    display_order INTEGER
);

CREATE INDEX IF NOT EXISTS idx_account_relationship_from ON account_relationship (from_account_id);
CREATE INDEX IF NOT EXISTS idx_account_relationship_to ON account_relationship (to_account_id);
