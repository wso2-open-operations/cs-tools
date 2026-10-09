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

-- Seeds the five project_group rows the Salesforce membership ingest's
-- mapProjectGroups and the announcement-visibility RLS policy (migration
-- 000085) both depend on by NAME: "Full Access", "General Access",
-- "Security Only", "Lead User Group", and "Business Contact  Group" (two
-- spaces -- the name ServiceNow itself gave it; matched exactly by
-- salesforceRolesForGroups and project-contacts reads). Migration 0128
-- seeded "Admin" the same way, for the same reason (admin became a project
-- role rather than a global one); these five are its siblings, never
-- seeded anywhere else in this directory.
--
-- Why this matters beyond onboarding: entity-service/CLAUDE.md's own
-- "Portal-driven membership writes" / "Salesforce membership ingest"
-- sections document these five project_group rows as something the
-- ServiceNow sync is expected to have already created in every real
-- environment -- but nothing in migrations/ ever actually created them, so
-- a database that reaches this point without that sync having run (a fresh
-- environment, a disaster-recovery rebuild, a test database built purely
-- from this repo's own migrations) silently has none of them. The concrete,
-- user-facing symptom: project_group_role has no ADMIN-style row to attach
-- a membership to for ordinary PORTAL_USER/SECURITY_CONTACT/LEAD_USER/
-- BUSINESS_CONTACT contacts, so the Salesforce membership ingest 503s (the
-- group lookup fails) and the 000085 announcement-visibility policy's own
-- EXISTS join matches nothing for ANY contact -- external users would see
-- zero announcements, not because they are correctly excluded, but because
-- the groups that would admit them were never created.
--
-- Idempotent and deliberately so, following 0128's own precedent exactly:
-- WHERE NOT EXISTS on project_role (match by role alone -- it is not
-- expected to change once ServiceNow has seeded it, so no update path is
-- needed here), on project_group (match by "group" name), and on
-- project_group_role (match by the (group, role) pair) -- safe to run
-- against a database the ServiceNow sync already populated: every insert
-- below is then a no-op, not a duplicate.
--
-- Unlike 0128 (which only ever needed to seed the ADMIN project_role row,
-- since 0028 already declares the enum label and ServiceNow's own sync was
-- assumed to have inserted the other four project_role rows), this also
-- seeds PORTAL_USER/SECURITY_CONTACT/LEAD_USER/BUSINESS_CONTACT's own
-- project_role rows defensively: the enum value existing (0028) says
-- nothing about whether a project_role ROW for it has actually been
-- inserted anywhere, and this migration must not depend on that sync
-- having already run.
INSERT INTO project_role (id, created_on, updated_on, created_by, updated_by, role)
SELECT gen_random_uuid(), NOW(), NOW(), 'migration-0208', 'migration-0208', r.role::project_role_enum
FROM (VALUES ('PORTAL_USER'), ('SECURITY_CONTACT'), ('LEAD_USER'), ('BUSINESS_CONTACT')) AS r(role)
WHERE NOT EXISTS (SELECT 1 FROM project_role pr WHERE pr.role = r.role::project_role_enum);

INSERT INTO project_group (id, created_on, updated_on, created_by, updated_by, "group")
SELECT gen_random_uuid(), NOW(), NOW(), 'migration-0208', 'migration-0208', g.name
FROM (VALUES
  ('Full Access'),
  ('General Access'),
  ('Security Only'),
  ('Lead User Group'),
  ('Business Contact  Group')
) AS g(name)
WHERE NOT EXISTS (SELECT 1 FROM project_group pg WHERE pg."group" = g.name);

-- Each group's role set, per entity-service/CLAUDE.md's own "Project
-- groups" mapping (§6.4 of the onboarding design): Full Access carries BOTH
-- PORTAL_USER and SECURITY_CONTACT (a contact with both labels in
-- Salesforce), General Access carries PORTAL_USER alone, Security Only
-- carries SECURITY_CONTACT alone, Lead User Group carries LEAD_USER alone
-- (additive to whichever of the first two a Lead also holds), and
-- Business Contact  Group carries BUSINESS_CONTACT alone.
INSERT INTO project_group_role (id, created_on, updated_on, created_by, updated_by, project_group_id, project_role_id)
SELECT gen_random_uuid(), NOW(), NOW(), 'migration-0208', 'migration-0208', pg.id, pr.id
FROM (VALUES
  ('Full Access', 'PORTAL_USER'),
  ('Full Access', 'SECURITY_CONTACT'),
  ('General Access', 'PORTAL_USER'),
  ('Security Only', 'SECURITY_CONTACT'),
  ('Lead User Group', 'LEAD_USER'),
  ('Business Contact  Group', 'BUSINESS_CONTACT')
) AS mapping(group_name, role_name)
JOIN project_group pg ON pg."group" = mapping.group_name
JOIN project_role pr ON pr.role = mapping.role_name::project_role_enum
WHERE NOT EXISTS (
    SELECT 1 FROM project_group_role x
    WHERE x.project_group_id = pg.id AND x.project_role_id = pr.id
);
