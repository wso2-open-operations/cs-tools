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

-- Product usage tracking, mirrored from ServiceNow.
--
-- Six custom u_ tables form one cluster around a "node" -- a single running
-- deployment instance that reports what it is and how much it is used:
--
--   deployment_node        the dimension: node -> product version + subscription
--   deployment_information what a node reported about itself (JDK, cores, raw JSON)
--   hourly_usage_summary   individual counts per node
--   daily_usage_summary    per node per day
--   monthly_usage_summary  per node per month
--   product_usage_map      product code -> the unit its usage is counted in
--
-- NODE IDENTITY IS NOT CONSISTENT UPSTREAM, and this schema preserves that
-- rather than papering over it. u_daily_usage_summary and u_usage_count carry
-- u_node_id as a REFERENCE to u_deployment_meta_information, so those become
-- real foreign keys. u_monthly_usage_count and u_deployment_information carry
-- it as a plain string(128) -- the node's own identifier, not a sys_id -- so
-- those stay text and join on deployment_node.node_id instead. Inventing a
-- foreign key where the source has a string would mean dropping every row
-- whose node has not been seen yet.
--
-- ONLY ONE ENUM. u_data_source is a real ServiceNow choice list, which is why
-- its stored values are numeric ("1", "2") and the mapping needs a value_map at
-- all. The other columns that look like vocabularies -- count_type, usage_unit --
-- are plain strings upstream with no choice list behind them, so they stay text:
-- the loader maps unlisted values with on_unmapped: null_and_warn, and a usage
-- figure whose unit has been nulled is unreadable data.

-- File Upload / API Call, from u_daily_usage_summary.u_data_source.
DO $$ BEGIN
    CREATE TYPE usage_data_source_enum AS ENUM ('API_CALL', 'FILE_UPLOAD');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- ---------- the dimension ----------
CREATE TABLE IF NOT EXISTS deployment_node (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    node_id VARCHAR(128) NOT NULL,
    subscription_key VARCHAR(128) NOT NULL,
    deployment_ref VARCHAR(128),
    product_version_id UUID REFERENCES product_version(id) ON DELETE SET NULL
);

-- node_id is how the string-keyed tables find their node, so it is the hot
-- lookup here even though sys_id is the primary key.
CREATE INDEX IF NOT EXISTS idx_deployment_node_node_id ON deployment_node (node_id);
CREATE INDEX IF NOT EXISTS idx_deployment_node_product_version_id ON deployment_node (product_version_id);
CREATE INDEX IF NOT EXISTS idx_deployment_node_subscription_key ON deployment_node (subscription_key);

-- ---------- what a node reported about itself ----------
CREATE TABLE IF NOT EXISTS deployment_information (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    node_id VARCHAR(128) NOT NULL,
    -- u_deployment_info is json(65000) upstream. jsonb rather than text: it is
    -- queried by content, and storing it as text would push every reader into
    -- parsing it themselves.
    deployment_info JSONB NOT NULL,
    deployment_info_hash VARCHAR(128) NOT NULL,
    jdk_version VARCHAR(64),
    -- VARCHAR, not INTEGER: upstream is string(64) and may hold something like
    -- "8 (4 physical)". Narrowing it here would drop rows the sync cannot cast.
    number_of_cores VARCHAR(64),
    reported_created_on TIMESTAMPTZ NOT NULL,
    reported_updated_on TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_deployment_information_node_id ON deployment_information (node_id);
CREATE INDEX IF NOT EXISTS idx_deployment_information_hash ON deployment_information (deployment_info_hash);

-- ---------- the facts ----------
CREATE TABLE IF NOT EXISTS hourly_usage_summary (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    deployment_node_id UUID REFERENCES deployment_node(id) ON DELETE CASCADE,
    -- count_type is a free string upstream (string(50)), not a choice list, so
    -- it stays text. In practice it holds CORES / TPS / MTX / MAU, but nothing
    -- upstream enforces that: ServiceNow stores these as plain strings with no
    -- choice list behind them, so an enum here would silently null any value
    -- outside the set -- and a usage count with no unit is unreadable data.
    count_type VARCHAR(50) NOT NULL,
    count INTEGER NOT NULL,
    counted_on TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_hourly_usage_summary_deployment_node_id ON hourly_usage_summary (deployment_node_id);
CREATE INDEX IF NOT EXISTS idx_hourly_usage_summary_counted_on ON hourly_usage_summary (counted_on);
CREATE INDEX IF NOT EXISTS idx_hourly_usage_summary_count_type ON hourly_usage_summary (count_type);

CREATE TABLE IF NOT EXISTS daily_usage_summary (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    deployment_node_id UUID REFERENCES deployment_node(id) ON DELETE CASCADE,
    deployment_ref VARCHAR(1000),
    summary_date DATE,
    count_type VARCHAR(1000),
    value INTEGER,
    data_source usage_data_source_enum
);

CREATE INDEX IF NOT EXISTS idx_daily_usage_summary_deployment_node_id ON daily_usage_summary (deployment_node_id);
CREATE INDEX IF NOT EXISTS idx_daily_usage_summary_summary_date ON daily_usage_summary (summary_date);

CREATE TABLE IF NOT EXISTS monthly_usage_summary (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    -- Text, not a foreign key: upstream carries the node's own identifier here
    -- rather than a sys_id. Joins to deployment_node.node_id.
    node_id VARCHAR(128) NOT NULL,
    count_type VARCHAR(50) NOT NULL,
    count INTEGER NOT NULL,
    year INTEGER NOT NULL,
    month INTEGER NOT NULL,

    CONSTRAINT chk_monthly_usage_summary_month_range CHECK (month BETWEEN 1 AND 12)
);

CREATE INDEX IF NOT EXISTS idx_monthly_usage_summary_node_id ON monthly_usage_summary (node_id);
CREATE INDEX IF NOT EXISTS idx_monthly_usage_summary_period ON monthly_usage_summary (year, month);

-- ---------- the lookup ----------
CREATE TABLE IF NOT EXISTS product_usage_map (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    product_code VARCHAR(40),
    -- Free string upstream, not a choice list -- same reasoning as
    -- hourly_usage_summary.count_type above.
    usage_unit VARCHAR(50),
    value VARCHAR(40)
);

CREATE INDEX IF NOT EXISTS idx_product_usage_map_product_code ON product_usage_map (product_code);
