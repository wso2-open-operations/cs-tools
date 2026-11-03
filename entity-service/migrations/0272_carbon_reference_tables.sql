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

-- Carbon platform history and Carbon kernel releases: static reference data
-- the wso2.com Release Matrix reads. Copied once from the legacy Product API
-- (last changed 2015) and read-only through the API, so there are no audit
-- columns. ids are the legacy integer ids, which the feeds return.
--
-- release_date_text keeps the legacy text exactly as stored (M/d/yyyy). The
-- legacy API sorts these rows by that text, so the compatible feed needs it
-- to keep the same order. release_date is the same value as a date.

BEGIN;
SET LOCAL lock_timeout = '5s';

CREATE TABLE IF NOT EXISTS carbon_platform_history (
    id                                 INTEGER      PRIMARY KEY,
    carbon_version                     VARCHAR(100),
    release_date                       DATE,
    release_date_text                  VARCHAR(100),
    esb_version                        VARCHAR(100),
    governance_registry_version        VARCHAR(100),
    application_server_version         VARCHAR(100),
    data_services_server_version       VARCHAR(100),
    identity_server_version            VARCHAR(100),
    business_process_server_version    VARCHAR(100),
    mashup_server_version              VARCHAR(100),
    gadget_server_version              VARCHAR(100),
    business_activity_monitor_version  VARCHAR(100),
    api_manager_version                VARCHAR(100),
    business_rules_server_version      VARCHAR(100),
    event_server_version               VARCHAR(100),
    message_broker_version             VARCHAR(100),
    complex_event_processor_version    VARCHAR(100),
    elastic_load_balancer_version      VARCHAR(100),
    storage_server_version             VARCHAR(100),
    stratos_version                    VARCHAR(100),
    cloud_gateway_version              VARCHAR(100),
    wsas_version                       VARCHAR(100),
    app_factory_version                VARCHAR(100),
    user_engagement_server_version     VARCHAR(100)
);

COMMENT ON TABLE carbon_platform_history IS
    'Carbon platform releases and the product versions built on each. Static reference data for the Release Matrix.';

CREATE TABLE IF NOT EXISTS carbon_kernel_release (
    id               INTEGER     PRIMARY KEY,
    kernel_release   VARCHAR(50),
    release_date     DATE,
    carbon_platform  VARCHAR(50)
);

COMMENT ON TABLE carbon_kernel_release IS
    'Carbon kernel releases by platform. Static reference data for the Release Matrix.';

COMMIT;
