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

-- portal_banner: one HTML banner per row, shown above the header in sort_order.
-- The admin edits these from CSM Settings. status is draft, published, or
-- unpublished. Only a published row is eligible to show, and only on a portal
-- whose visibility flag is true. Draft and unpublished stay on the Settings list
-- and never reach either portal. Both flags false means published but shown
-- nowhere.
--
-- The banner is the html column. A close button lives in that HTML
-- (data-close-banner) and is edited with the rest of the HTML. Dismissal in
-- the browser uses the row id. Title, message, severity, and an action link
-- are not stored.
--
-- Entity-service owns this table. It is not a ServiceNow sync table, so
-- operations/csm-sync-service does not carry this file. Access is Asgardeo
-- plus the admin role on writes. This table has no row-level security.
--
-- starts_on / expires_on follow the schema's _on timestamp names. The API
-- can still call them startsAt / expiresAt.

BEGIN;
SET LOCAL lock_timeout = '5s';

CREATE TABLE IF NOT EXISTS portal_banner (
    id                            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    status                        TEXT        NOT NULL DEFAULT 'draft'
                                              CHECK (status IN ('draft', 'published', 'unpublished')),
    sort_order                    INTEGER     NOT NULL DEFAULT 0 CHECK (sort_order >= 0),
    visible_in_customer_portal    BOOLEAN     NOT NULL DEFAULT FALSE,
    visible_in_csm_portal         BOOLEAN     NOT NULL DEFAULT FALSE,
    html                          TEXT        NOT NULL DEFAULT '',
    starts_on                     TIMESTAMPTZ NULL,
    expires_on                    TIMESTAMPTZ NULL,
    created_on                    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by                    TEXT        NOT NULL,
    updated_on                    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by                    TEXT        NOT NULL
);

COMMENT ON TABLE portal_banner IS
    'HTML portal banners. status published plus the matching visibility flag is what a portal shows. draft and unpublished stay hidden. A close button is part of the HTML.';

COMMIT;
