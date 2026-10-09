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

-- Announcement visibility, restated as the product rule it is meant to be:
--
--   internal staff           -> general AND security announcements
--   SECURITY_CONTACT holders -> general AND security announcements
--   PORTAL_USER / LEAD_USER  -> general announcements, plus security
--                               announcements ONLY in a project that has no
--                               security contact at all
--   anyone else              -> nothing
--
-- One change against the policy migration 100010 left behind: a security
-- contact now also sees general announcements. Before, the policy matched on
-- a single role per announcement, so a contact holding only SECURITY_CONTACT
-- (the "Security Only" project group) saw security announcements and nothing
-- else, which cut a security contact off from ordinary service notices sent
-- to their own project. A contact holding several roles (e.g. "Full Access" =
-- Portal user + Security Contact) already saw both and is unaffected.
--
-- 0149's fallback is kept exactly as it was: when a project has NO security
-- contact (project_has_security_contact(), which ignores DEACTIVATED
-- contacts), its ordinary portal users see the security announcements too,
-- rather than nobody but staff seeing them. As soon as the project has a
-- security contact, ordinary portal users stop seeing them.
--
-- LEAD_USER keeps the same access as PORTAL_USER (migration 100001's
-- reasoning stands: "can escalate a case" implies nothing extra about
-- security-bulletin eligibility), and BUSINESS_CONTACT alone still
-- contributes nothing.
--
-- announcement_is_security() is untouched: it still decides general-vs-
-- security from announcement_type OR the "Security Announcement" tag, so
-- historical rows identified only by the tag stay protected. The one known
-- gap is unchanged too: old ServiceNow "[Special Security Announcement]"
-- cases carry neither signal and are treated as general (see 000085).
--
-- The role check comes first in the OR so a security contact never pays for
-- the announcement_is_security() lookup. The internal-caller check keeps the
-- InitPlan form migration 100015 introduced, since 0154 rewrote only the
-- policies that existed when it ran.
--
-- Comments and watchers on an announcement follow this automatically: their
-- policies (migration 100016) require the announcement row itself to be
-- visible, so no change is needed there.
--
-- ALTER POLICY replaces the expression in place: there is no moment at which
-- the table has no policy (which, under FORCE ROW LEVEL SECURITY, would hide
-- every row), and the file is safe to re-run.
ALTER POLICY announcement_visibility ON announcement
  USING (
    (SELECT current_setting('app.is_internal', true) = 'true')
    OR EXISTS (
      SELECT 1
      FROM work_item wi
      JOIN project_contact pc ON pc.project_id = wi.project_id
      JOIN project_contact_group pcg ON pcg.project_contact_id = pc.id
      JOIN project_group_role pgr ON pgr.project_group_id = pcg.project_group_id
      JOIN project_role pr ON pr.id = pgr.project_role_id
      WHERE wi.id = announcement.id
        AND LOWER(pc.email) = LOWER(NULLIF(current_setting('app.viewer_email', true), ''))
        AND (
          pr.role = 'SECURITY_CONTACT'
          OR (
            pr.role IN ('PORTAL_USER', 'LEAD_USER')
            AND (
              NOT announcement_is_security(announcement.id, announcement.announcement_type)
              OR NOT project_has_security_contact(wi.project_id)
            )
          )
        )
    )
  );
