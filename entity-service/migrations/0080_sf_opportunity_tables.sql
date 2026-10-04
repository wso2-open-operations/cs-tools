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

-- The Salesforce opportunity tables, mirrored so query-hour ENTITLEMENT can be
-- computed in Postgres. Consumption already can be (time_card, migration 0041);
-- entitlement cannot, because it is derived from opportunity product lines and
-- nothing here carries them. That gap is what blocks the port of ServiceNow's
-- `[Query Hour] Update Opportunity Line`.
--
-- Names drop the `u_`/`u_sf_` shape of the source per this repo's convention.
-- `u_sf_link_opportunity` becomes `sf_opportunity_link`: the source name reads
-- backwards, and the precedent for de-awkwarding is already set
-- (u_deployment_meta_information -> deployment_node,
--  u_product_vulnerabilities -> product_vulnerability).

-- Choice values confirmed on u_sf_opportunity.u_query_hour_state (DEV,
-- 2026-09-23): 1 = Notified (75%), 2 = Notified (90%),
-- 3 = Closure Notice (100%/Exceed). There is no explicit "under 75%" choice —
-- the field is simply empty until a threshold is crossed, so the enum has no
-- NORMAL member and the column stays nullable.
DO $$ BEGIN
    CREATE TYPE query_hour_state_enum AS ENUM (
        'NOTIFIED_75', 'NOTIFIED_90', 'CLOSURE_NOTICE_100'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS sf_opportunity (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    -- u_opportunity_id: the Salesforce Opportunity id (006E…). Distinct from
    -- `id`, which is the ServiceNow sys_id rewritten as a UUID.
    sf_id VARCHAR(40),
    name VARCHAR(200),
    account_id UUID REFERENCES account(id) ON DELETE SET NULL,
    stage VARCHAR(40),
    type VARCHAR(40),
    is_won BOOLEAN,
    close_date DATE,
    -- u_owner is a plain string on the source, not a reference to sys_user, so
    -- it cannot be an FK to "user" without a lookup this sync does not do.
    owner VARCHAR(40),
    engagement_code VARCHAR(40),
    eula_version VARCHAR(160),
    eula_version_decimal NUMERIC,
    query_hour_state query_hour_state_enum,
    sync_time_stamp TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_sf_opportunity_account_id ON sf_opportunity (account_id);
CREATE INDEX IF NOT EXISTS idx_sf_opportunity_sf_id ON sf_opportunity (sf_id);

CREATE TABLE IF NOT EXISTS sf_opportunity_product (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    -- u_id: the Salesforce OpportunityLineItem id (00kE…), used in the
    -- lightning deep-links ServiceNow builds into its emails.
    line_item_sf_id VARCHAR(40),
    name VARCHAR(200),
    opportunity_id UUID REFERENCES sf_opportunity(id) ON DELETE CASCADE,
    -- The entitlement inputs. quantity x hours-per-pack is the whole
    -- calculation; ServiceNow derives hours-per-pack by string-matching
    -- product_name against "Development Support - N hours".
    product_name VARCHAR(100),
    quantity NUMERIC,
    service_start_date DATE,
    service_end_date DATE,
    -- u_development_support_hours: a DECIMAL column that already holds the
    -- number ServiceNow re-derives from the product name. Mirrored so the port
    -- can prefer it over the string matching if it turns out to be populated.
    development_support_hours NUMERIC,
    product_code VARCHAR(70),
    product_description VARCHAR(250),
    product_family VARCHAR(100),
    -- u_product_id is the Salesforce Product2 id, a string; u_product is a
    -- reference to cmdb_model, which this repo does not mirror, so only the
    -- Salesforce id is carried.
    product_sf_id VARCHAR(40),
    product_unit VARCHAR(40),
    classification VARCHAR(40),
    engagement_code VARCHAR(40),
    eng_product_code VARCHAR(40),
    environment VARCHAR(40),
    total_price NUMERIC,
    sync_time_stamp TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_sf_opportunity_product_opportunity_id
    ON sf_opportunity_product (opportunity_id);
-- The entitlement query filters on product_name and the service window, so it
-- is served by this rather than by a scan.
CREATE INDEX IF NOT EXISTS idx_sf_opportunity_product_entitlement
    ON sf_opportunity_product (product_name, service_start_date, service_end_date);

-- The opportunity <-> project join. Only 34 rows on DEV, but it is the only
-- path from a project to the product lines that fund it.
CREATE TABLE IF NOT EXISTS sf_opportunity_link (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    -- u_id: the Salesforce record id for the link itself (a3UE…).
    link_sf_id VARCHAR(40),
    -- u_name is a human-readable code (LO-26-09-N-00034824), not a title.
    number VARCHAR(40),
    opportunity_id UUID REFERENCES sf_opportunity(id) ON DELETE CASCADE,
    project_id UUID REFERENCES project(id) ON DELETE CASCADE,
    sync_time_stamp TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_sf_opportunity_link_opportunity_id
    ON sf_opportunity_link (opportunity_id);
CREATE INDEX IF NOT EXISTS idx_sf_opportunity_link_project_id
    ON sf_opportunity_link (project_id);
