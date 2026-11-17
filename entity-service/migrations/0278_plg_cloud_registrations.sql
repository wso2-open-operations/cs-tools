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

-- PLG: cloud-platform registrations, keyed by company id.
--
-- A cloud platform sends only the company id; its name and email are never
-- stored but read from the Cloud API on reveal. So organization_name and
-- registered_user become nullable (a CHECK keeps a name or a company id),
-- plg_product.identity_source marks cloud products, and the views keep
-- organisations with no registrant. Safe to re-run.

DO $$ BEGIN
    CREATE TYPE plg_identity_source_enum AS ENUM ('MOESIF', 'CLOUD_PLATFORM');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

ALTER TABLE plg_product
    ADD COLUMN IF NOT EXISTS identity_source plg_identity_source_enum NOT NULL DEFAULT 'MOESIF';

COMMENT ON COLUMN plg_product.identity_source IS
    'Where this product''s organisation names and emails come from. MOESIF: stored at ingest. '
    'CLOUD_PLATFORM: never stored; resolved from the Cloud API by company id on request.';

UPDATE plg_product SET identity_source = 'CLOUD_PLATFORM'
WHERE  code = 'AGENT_PLATFORM' AND identity_source <> 'CLOUD_PLATFORM';

ALTER TABLE plg_organization ALTER COLUMN organization_name DROP NOT NULL;
ALTER TABLE plg_organization ALTER COLUMN registered_user   DROP NOT NULL;

-- Dropped and re-added: ADD CONSTRAINT has no IF NOT EXISTS.
ALTER TABLE plg_organization DROP CONSTRAINT IF EXISTS ck_plg_organization_identifiable;
ALTER TABLE plg_organization ADD CONSTRAINT ck_plg_organization_identifiable
    CHECK (organization_name IS NOT NULL OR moesif_company_id IS NOT NULL);

-- Views restated from 0129; plg_organization_v keeps its column list.

CREATE OR REPLACE VIEW plg_organization_v AS
SELECT o.id                    AS organization_id,
       o.organization_name,
       o.created_on,
       o.registered_user,
       p.email                 AS registered_email,
       NULLIF(TRIM(BOTH ' ' FROM COALESCE(p.first_name, '') || ' ' ||
                                 COALESCE(p.last_name, '')), '') AS registered_name,

       o.plg_cs_owner,
       u.email                 AS plg_cs_owner_email,
       u.display_name          AS plg_cs_owner_name,
       u.is_active                AS plg_cs_owner_active,

       o.moesif_company_id,

       COALESCE(o.country_name, (
           SELECT o2.country_name FROM plg_organization o2
           WHERE o2.registered_user = o.registered_user AND o2.country_name IS NOT NULL
           ORDER BY o2.created_on LIMIT 1))                      AS country_name,

       COALESCE(o.company_name_from_domain, (
           SELECT o2.company_name_from_domain FROM plg_organization o2
           WHERE o2.registered_user = o.registered_user AND o2.company_name_from_domain IS NOT NULL
           ORDER BY o2.created_on LIMIT 1))                      AS company_name_from_domain,

       (o.country_name IS NULL)                                  AS fields_inherited
FROM   plg_organization o
-- LEFT: a cloud organisation has no registrant; its email and name are NULL.
LEFT   JOIN plg_person p  ON p.id = o.registered_user
LEFT   JOIN plg_user_v u ON u.id = o.plg_cs_owner;

CREATE OR REPLACE VIEW plg_org_platform_v AS
SELECT op.id                     AS org_platform_id,
       op.organization_id,
       v.organization_name,
       v.registered_email,
       v.registered_name,
       v.plg_cs_owner,
       v.plg_cs_owner_email,
       v.plg_cs_owner_name,
       op.product_id,
       pr.code                   AS product_code,
       pr.name                   AS product_name,
       pr.display_order          AS product_display_order,
       op.registered_on,
       op.lifecycle_stage,
       ls.name                   AS lifecycle_stage_name,
       ls.display_order          AS lifecycle_stage_order,
       op.stage_entered_on,
       op.acknowledged_on,
       op.acknowledged_by,
       ack.email                 AS acknowledged_by_email,
       ack.display_name          AS acknowledged_by_name,

       op.health_state,
       op.health_entered_on,
       op.health_updated_by,
       hu.email                  AS health_updated_by_email,
       hu.display_name           AS health_updated_by_name,

       op.subscription_tier,
       op.subscription_entered_on,
       op.subscription_updated_by,
       su.email                  AS subscription_updated_by_email,
       su.display_name           AS subscription_updated_by_name,

       op.trial_end_date,
       op.trial_extended_date,
       plg_current_period_end_date(op.subscription_tier, op.trial_end_date,
                                   op.trial_extended_date) AS current_period_end_date,
       (op.acknowledged_on IS NULL)                                  AS is_new,

       plg_applicable_playbook_types(op.health_state)::TEXT[]        AS applicable_playbook_types,

       EXISTS (SELECT 1 FROM plg_playbook pb
               WHERE pb.product_id      = op.product_id
                 AND pb.lifecycle_stage = op.lifecycle_stage
                 AND pb.active
                 AND pb.playbook_type   = ANY(
                         plg_applicable_playbook_types(op.health_state)))
                                                                     AS stage_carries_playbooks,
       COALESCE(rs.run_total, 0)                                     AS run_total,
       COALESCE(rs.run_active, 0)                                    AS run_active,
       COALESCE(rs.run_closed, 0)                                    AS run_closed,
       COALESCE(rs.task_total, 0)                                    AS task_total,
       COALESCE(rs.task_completed, 0)                                AS task_completed,

       -- Last: CREATE OR REPLACE VIEW only appends columns.
       pr.identity_source                                            AS product_identity_source
FROM   plg_org_platform op
JOIN   plg_organization_v v ON v.organization_id = op.organization_id
JOIN   plg_product pr       ON pr.id = op.product_id
JOIN   plg_lifecycle_stage ls ON ls.stage = op.lifecycle_stage
LEFT   JOIN plg_user_v ack ON ack.id = op.acknowledged_by
LEFT   JOIN plg_user_v hu  ON hu.id  = op.health_updated_by
LEFT   JOIN plg_user_v su  ON su.id  = op.subscription_updated_by
LEFT   JOIN LATERAL (
           SELECT COUNT(*)                                              AS run_total,
                  COUNT(*) FILTER (WHERE rv.run_status = 'ACTIVE')      AS run_active,
                  COUNT(*) FILTER (WHERE rv.run_status = 'CLOSED')      AS run_closed,
                  COALESCE(SUM(rv.task_total), 0)                       AS task_total,
                  COALESCE(SUM(rv.task_completed), 0)                   AS task_completed
           FROM   plg_playbook_run_v rv
           WHERE  rv.org_platform_id = op.id
       ) rs ON TRUE;
