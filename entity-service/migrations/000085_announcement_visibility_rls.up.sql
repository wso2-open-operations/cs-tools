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

-- Reimplements, at the database layer, the announcement-visibility rule
-- ServiceNow's own sys_security_acl scripts enforce (three ACL rows on
-- sn_customerservice_case, keyed on the caller's project_contact group
-- role) -- so that a caller reading `announcement` directly from Postgres
-- gets the same restriction ServiceNow already applies, rather than relying
-- solely on entity-service's own query-building code to always remember to
-- filter correctly.
--
-- Deliberately NOT a plain application-level WHERE clause / callable SQL
-- function: that would only protect callers who remember to use it. RLS
-- enforces this for every SELECT against the table -- present call sites
-- and any added later -- with a fail-closed default: a caller that never
-- sets app.viewer_email sees zero rows, not every row.
--
-- The rule (matches the doc's intended Visibility Matrix, not ServiceNow's
-- current script bugs -- see PR description for the two confirmed
-- ServiceNow ACL bugs this deliberately does not reproduce):
--   general announcement  -> visible to PORTAL_USER or LEAD_USER
--   security announcement -> visible to SECURITY_CONTACT
--   BUSINESS_CONTACT alone contributes nothing (matches the doc's "Business
--     Contact Group is orthogonal" note -- a contact holding it alongside
--     Full Access still sees via Full Access, not via Business Contact)
--   LEAD_USER (project_group "Lead User Group") is undocumented in every
--     ServiceNow ACL script and the doc's own matrix -- treated as general
--     access only, since "can escalate a case" implies nothing about
--     security-bulletin eligibility
--
-- Identity arrives via two session-local GUCs the caller must set inside
-- the SAME transaction as the SELECT (SELECT set_config('app.viewer_email',
-- <email>, true) / set_config('app.is_internal', 'true'|'false', true)),
-- never as a plain SET: local scoping is what makes this safe under a
-- pooled connection -- the value reverts automatically at COMMIT/ROLLBACK,
-- so a later, unrelated request reusing the same physical connection can
-- never inherit a previous caller's identity. Tested empirically (not just
-- reasoned about): the equivalent one-line "smuggle set_config into the
-- protected query's own WHERE clause" shortcut was proven unsafe -- the
-- planner is free to evaluate the RLS qual before a non-leakproof function
-- call like set_config(), and did so under an index scan in testing,
-- returning wrong results. set_config must be its own statement, ahead of
-- the query that depends on it, inside one explicit transaction.
--
-- current_setting(..., true) returns NULL (not an error, not empty string)
-- when the caller never set the GUC at all -- this is what makes the
-- fail-closed default work with no special-case handling: NULL never
-- equals 'true', and NULL never equals an email, so both the internal
-- bypass and the EXISTS lookup naturally evaluate to "no access" rather
-- than needing an explicit "is this set" check.
--
-- announcement_is_security decides general-vs-security from TWO signals,
-- not just announcement_type (migration 000084_announcement_add_type):
-- checked live against the real ServiceNow-synced data, announcement_type
-- is correctly set on some real security bulletins but NOT on others (a
-- currently-open, CVSS 10.0 "account takeover" bulletin fanned out to many
-- real customers had announcement_type = 'GENERAL') -- and there is no
-- guarantee historical rows will ever be corrected. The "Security
-- Announcement" work_item_tag (already attached by the publish flow's
-- AddCaseTagAs, see announcement_request_service.go's
-- autoPublishSecurityTagLabel) is checked too, as a second, independent
-- signal: a row is treated as security if EITHER says so, so a gap in one
-- signal doesn't silently under-protect a real security announcement.
--
-- KNOWN GAP, accepted rather than worked around here (checked live, not
-- assumed): a whole class of historical/ongoing ServiceNow-originated
-- bulletins -- every "[Special Security Announcement]"-prefixed case,
-- including the entire real Log4Shell (CVE-2021-44228) campaign, 31,741
-- cases -- has BOTH signals wrong: announcement_type = 'GENERAL' and no
-- work_item_tag at all (confirmed against ServiceNow's own label_entry
-- table). Whatever process creates these bulletins sets neither signal
-- correctly, for years running -- a source-data quality issue in that
-- external process, not something a Postgres policy can safely infer from
-- (e.g. matching on subject-line keywords would be fragile and easy to get
-- wrong in both directions, trading a known gap for a new one). This is
-- still strictly an improvement over the pre-migration baseline, which
-- enforced no restriction on any announcement at all -- new announcements
-- and portal-published ones are correctly protected; this historical
-- ServiceNow-batch class is not, until that upstream process is fixed.
CREATE OR REPLACE FUNCTION announcement_is_security(ann_id UUID, ann_type announcement_type_enum)
RETURNS BOOLEAN
LANGUAGE sql
STABLE
AS $$
  SELECT ann_type = 'SECURITY'
    OR EXISTS (
      SELECT 1
      FROM work_item_tag wit
      JOIN tag t ON t.id = wit.tag_id
      WHERE wit.work_item_id = ann_id
        AND LOWER(t.name) = LOWER('Security Announcement')
    )
$$;

ALTER TABLE announcement ENABLE ROW LEVEL SECURITY;

-- Without FORCE, a non-superuser table owner is still exempt from its own
-- table's RLS policies -- and the connecting application role IS this
-- table's owner. Confirmed by testing: the policy silently did nothing
-- until this was added.
ALTER TABLE announcement FORCE ROW LEVEL SECURITY;

-- The policy's EXISTS subquery filters project_contact by (project_id,
-- LOWER(email)) on every row check. There was no index covering that before
-- this migration -- confirmed by testing that the query planner falls back
-- to a full scan of project_contact for every announcement row without it.
-- Expression index on LOWER(email), not a plain column index, since the
-- policy itself compares LOWER(pc.email) -- a plain (project_id, email)
-- index cannot satisfy that predicate and the planner would ignore it.
CREATE INDEX IF NOT EXISTS idx_project_contact_project_id_email
  ON project_contact (project_id, LOWER(email));

CREATE POLICY announcement_visibility ON announcement
  FOR SELECT
  USING (
    current_setting('app.is_internal', true) = 'true'
    OR EXISTS (
      SELECT 1
      FROM work_item wi
      JOIN project_contact pc ON pc.project_id = wi.project_id
      JOIN project_contact_group pcg ON pcg.project_contact_id = pc.id
      JOIN project_group_role pgr ON pgr.project_group_id = pcg.project_group_id
      JOIN project_role pr ON pr.id = pgr.project_role_id
      WHERE wi.id = announcement.id
        AND LOWER(pc.email) = LOWER(current_setting('app.viewer_email', true))
        AND (
          (NOT announcement_is_security(announcement.id, announcement.announcement_type) AND pr.role IN ('PORTAL_USER', 'LEAD_USER'))
          OR (announcement_is_security(announcement.id, announcement.announcement_type) AND pr.role = 'SECURITY_CONTACT')
        )
    )
  );

-- Writes are deliberately left unrestricted by this migration. The only
-- writer of `announcement` today is the external ServiceNow sync job, not
-- customer-facing traffic -- every ServiceNow ACL this mirrors is
-- Operation: read, and there is no equivalent write-side rule to
-- reimplement. This is not optional boilerplate: confirmed by testing that
-- ENABLE + FORCE ROW LEVEL SECURITY with no policy at all for a given
-- command blocks 100% of that command by default, for every role,
-- including the table owner. Without these three policies, the sync job's
-- INSERTs would have started failing outright the moment this shipped.
CREATE POLICY announcement_write_unrestricted ON announcement
  FOR INSERT WITH CHECK (true);
CREATE POLICY announcement_update_unrestricted ON announcement
  FOR UPDATE USING (true) WITH CHECK (true);
CREATE POLICY announcement_delete_unrestricted ON announcement
  FOR DELETE USING (true);
