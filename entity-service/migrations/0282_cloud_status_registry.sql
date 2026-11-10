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

-- Cloud status registry: the clouds the public status dashboard serves, their
-- regions and their availability weights, as DATA.
--
-- Before this migration a cloud was a closed set baked into the service: a
-- Postgres enum on cloud_monitor.cloud_offering, a Go map of slugs, a Go map
-- of weights and parent service ids, and five copies of the slug list in the
-- API spec. Onboarding a cloud meant changing all of them and deploying. After
-- it, onboarding is INSERTs (example at the end of this header) plus a webhook
-- URL and secret for the slug in the notification service's environment
-- (CLOUD_STATUS_WEBHOOK_URLS / CLOUD_STATUS_WEBHOOK_SECRETS), which stay
-- configuration and never live in the database. Webhooks for the cloud's own
-- outages also need its parent service id in CLOUD_STATUS_SERVICE_IDS (the
-- trigger scope, also configuration); the read-only dashboard endpoints do not.
--
-- TABLES
--   cloud_registry                  one row per cloud: slug (what the endpoints
--                                   take), display name, offering_key (the value
--                                   stored in cloud_monitor.cloud_offering),
--                                   incident_prefix, weighted flag, is_active.
--   cloud_registry_region           one row per region: the dashboard's literal
--                                   key ("us - dp") and the parent service whose
--                                   offerings make the region up.
--   cloud_registry_offering_weight  one row per weighted offering: its share of
--                                   its region as a percentage. The weights of a
--                                   region's offerings sum to exactly 100. A
--                                   cloud with weighted = FALSE takes a plain
--                                   mean and has no weight rows.
--
-- cloud_monitor.cloud_offering BECOMES VARCHAR(40), NOT A NEW ENUM, and the old
-- enum TYPE IS KEPT. Reasons, in order of how much they matter:
--   * The monitor data is written by csm-sync-service during the data-source
--     migration, outside this repository, still using the old upper-case
--     strings ('CHOREO_EU'). A string column accepts those unchanged. A
--     statement that casts to the old type (x::cloud_monitor_cloud_offering_enum)
--     keeps compiling because the type still exists, and the enum-to-varchar
--     assignment is an allowed I/O conversion. Dropping the type would break
--     such a statement.
--   * Adding a value to the enum instead (ALTER TYPE ... ADD VALUE) would still
--     need one migration per cloud, which is the problem being removed.
--   * No index, view, policy or default depends on the column (checked across
--     the migrations), and cloud_monitor holds a few hundred rows, so the
--     rewrite is instant. lock_timeout guards the lock anyway.
-- The value stays the registry's offering_key, upper-case by convention
-- (CHOREO_EU), not enforced: a monitor whose value matches no active cloud is
-- simply unroutable, exactly as an unknown enum value was.
--
-- NO FOREIGN KEYS TO service OR service_offering, on purpose. Both are mirrored
-- by the sync. A RESTRICT key could make the sync's delete fail; a CASCADE one
-- would silently erase a cloud's plan when it recreates a row. The same
-- reasoning cloud_status_events applies to outage. Referential integrity is
-- checked instead by the query below, which should return no rows after an
-- onboarding:
--
--   SELECT r.cloud_slug, r.parent_service_id FROM cloud_registry_region r
--    LEFT JOIN service s ON s.id = r.parent_service_id WHERE s.id IS NULL
--   UNION ALL
--   SELECT w.cloud_slug, w.service_offering_id FROM cloud_registry_offering_weight w
--    LEFT JOIN service_offering so ON so.id = w.service_offering_id WHERE so.id IS NULL;
--
-- INCIDENT PREFIX. Incident history and detail match a cloud's incidents by the
-- parent service's NAME starting with a prefix (ILIKE prefix || '%'). That was
-- the cloud slug itself. It is now its own column so a cloud whose services are
-- not named after its slug can still be routed; the seven seeded clouds use
-- their slug, so behaviour is unchanged. Prefixes that start with another
-- cloud's prefix ("choreo" vs "choreo-eu") overlap, as they always did.
--
-- NO SEED DATA IN THIS MIGRATION. It creates the tables and nothing else. The
-- seven existing clouds, their regions and their availability weights are
-- environment data: the parent and offering ids are the mirrored catalog's own
-- ids and are not guaranteed identical across environments, so they are loaded
-- per environment by a separate seed step, not baked into a migration.
--
-- *** UNTIL THAT SEED HAS RUN, EVERY CLOUD IS UNKNOWN. *** An empty registry
-- answers "cloud must be one of ..." with no slugs, and an environment that
-- ran this migration without seeding serves no cloud status until it does.
-- Seed an environment before it serves status dashboard traffic.
--
-- ONBOARDING A CLOUD ("acme-cloud", one region "us", two offerings):
--
--   INSERT INTO cloud_registry (slug, display_name, offering_key, incident_prefix, weighted, sort_order)
--   VALUES ('acme-cloud', 'Acme Cloud', 'ACME_CLOUD', 'acme-cloud', TRUE, 80);
--   INSERT INTO cloud_registry_region (cloud_slug, region_key, parent_service_id, sort_order)
--   VALUES ('acme-cloud', 'us', '00000000-0000-0000-0000-0000000000a1', 10);
--   INSERT INTO cloud_registry_offering_weight (cloud_slug, service_offering_id, weight)
--   VALUES ('acme-cloud', '00000000-0000-0000-0000-0000000000b1', 60),
--          ('acme-cloud', '00000000-0000-0000-0000-0000000000b2', 40);
--   -- then point cloud_monitor rows at it: cloud_offering = 'ACME_CLOUD'
--
-- A change is picked up within the registry cache's TTL (30s). Setting
-- is_active = FALSE retires a cloud: it is rejected like an unknown one.
--
-- Idempotent: IF NOT EXISTS and a conditional type change.
-- One transaction with a short lock timeout, as the sibling migrations do.

BEGIN;
SET LOCAL lock_timeout = '5s';

DO $$ BEGIN
    IF EXISTS (
        SELECT 1
          FROM information_schema.columns
         WHERE table_schema = current_schema()
           AND table_name = 'cloud_monitor'
           AND column_name = 'cloud_offering'
           AND data_type = 'USER-DEFINED'
    ) THEN
        ALTER TABLE cloud_monitor
            ALTER COLUMN cloud_offering TYPE VARCHAR(40) USING cloud_offering::text;
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS cloud_registry (
    slug            VARCHAR(40)  PRIMARY KEY,
    display_name    VARCHAR(100) NOT NULL,
    offering_key    VARCHAR(40)  NOT NULL UNIQUE,
    incident_prefix VARCHAR(255) NOT NULL,
    weighted        BOOLEAN      NOT NULL DEFAULT TRUE,
    is_active       BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order      INTEGER      NOT NULL DEFAULT 0,
    created_on      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_on      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT cloud_registry_slug_format CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$')
);

CREATE TABLE IF NOT EXISTS cloud_registry_region (
    cloud_slug        VARCHAR(40)  NOT NULL REFERENCES cloud_registry(slug) ON DELETE CASCADE,
    region_key        VARCHAR(100) NOT NULL,
    parent_service_id UUID         NOT NULL,
    sort_order        INTEGER      NOT NULL DEFAULT 0,
    PRIMARY KEY (cloud_slug, region_key)
);

CREATE TABLE IF NOT EXISTS cloud_registry_offering_weight (
    cloud_slug          VARCHAR(40)  NOT NULL REFERENCES cloud_registry(slug) ON DELETE CASCADE,
    service_offering_id UUID         NOT NULL,
    weight              NUMERIC(7,3) NOT NULL CHECK (weight >= 0 AND weight <= 100),
    PRIMARY KEY (cloud_slug, service_offering_id)
);

COMMIT;
