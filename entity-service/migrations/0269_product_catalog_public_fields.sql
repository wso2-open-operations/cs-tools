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

-- Product catalogue, public fields. CSM becomes the master of the product
-- EOL data that the legacy Product API (wso2-product-service) publishes to
-- the wso2.com Support Matrix and Release Matrix. These columns hold what
-- that API serves and CSM did not have.
--
-- product_version keeps one row per (product, version, deployment profile).
-- The public feeds list one row per published version, so the profile rows of
-- one listing share public_id, and the feed groups by it. public_id carries
-- the legacy numeric id; versions created after the import take the next
-- value of product_version_public_id_seq.
--
-- public_name / public_abbreviation are the exact name and abbreviation the
-- feeds publish. They are per version because the published name changes
-- between versions of one product. NULL means fall back to product.name /
-- product.code.
--
-- is_public / show_in_support_matrix default to false so versions that only
-- CSM knows about stay off the public feeds until someone opts them in.
-- legacy_status is the legacy "status" bit, carried as is (its meaning is
-- unknown and it cannot be derived from current_support_status). It defaults
-- to true, as the legacy API published a missing value.
--
-- retired_on replaces deletion. A version is referenced by deployments,
-- work items and change requests, and deleting it cascades into some of
-- them, so the catalogue never deletes a version.
--
-- product_version is also written by operations/csm-sync-service. Every
-- column added here is nullable or has a default, so its writes are
-- unaffected. No row-level security, like the rest of the catalogue.

BEGIN;
SET LOCAL lock_timeout = '5s';

-- Legacy ids end at 1085. New listings start at 1100, leaving room for rows
-- created in the legacy database during the parallel run.
CREATE SEQUENCE IF NOT EXISTS product_version_public_id_seq START WITH 1100;

ALTER TABLE product_version
    ADD COLUMN IF NOT EXISTS public_id                     BIGINT,
    ADD COLUMN IF NOT EXISTS public_name                   VARCHAR(150),
    ADD COLUMN IF NOT EXISTS public_abbreviation           VARCHAR(64),
    ADD COLUMN IF NOT EXISTS description                   TEXT,
    ADD COLUMN IF NOT EXISTS support_eol_announcement_date DATE,
    ADD COLUMN IF NOT EXISTS is_public                     BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS show_in_support_matrix        BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS tags                          TEXT[]  NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS release_links                 VARCHAR(255),
    ADD COLUMN IF NOT EXISTS legacy_status                 BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS retired_on                    TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS retired_by                    VARCHAR(255);

CREATE INDEX IF NOT EXISTS idx_product_version_public_id
    ON product_version (public_id);

COMMENT ON COLUMN product_version.public_id IS
    'Numeric id the public feeds return. Shared by the profile rows of one published version. Legacy Product API ids are kept; new versions take product_version_public_id_seq.';
COMMENT ON COLUMN product_version.public_name IS
    'Product name the public feeds publish for this version. NULL falls back to product.name.';
COMMENT ON COLUMN product_version.public_abbreviation IS
    'Abbreviation the public feeds publish for this version. NULL falls back to product.code.';
COMMENT ON COLUMN product_version.legacy_status IS
    'The legacy Product API status bit, imported as is. Defaults to true.';
COMMENT ON COLUMN product_version.retired_on IS
    'Set when the version is retired from the catalogue. Retired versions leave the public feeds but stay referenceable.';

-- Carbon platform details of a Carbon-based version (Release Matrix). 1:1 with
-- product_version; only versions that have Carbon data get a row.
CREATE TABLE IF NOT EXISTS product_version_carbon (
    product_version_id    UUID        PRIMARY KEY REFERENCES product_version (id) ON DELETE CASCADE,
    platform_release      VARCHAR(50),
    platform_version      VARCHAR(50),
    carbon_kernel_version VARCHAR(50),
    release_chunk         VARCHAR(50),
    kernel_patch_no       VARCHAR(50),
    created_on            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by            VARCHAR(255) NOT NULL,
    updated_on            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by            VARCHAR(255) NOT NULL
);

COMMENT ON TABLE product_version_carbon IS
    'Carbon platform of a product version, published as carbonPlatform on the Release Matrix feed.';

COMMIT;
