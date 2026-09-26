// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// Salesforce Project_Contact__c roles (Role__c multi-picklist labels) the
// ingest understands. Anything else is reported back as ignored.
const (
	sfRolePortalUser      = "portal user"
	sfRoleSecurityContact = "security contact"
	sfRoleLead            = "lead"
	sfRoleAdmin           = "admin"
)

// The same four roles in Salesforce's own spelling. The portal writes send
// these back to Salesforce verbatim (Role__c is a picklist — a label it does
// not know is rejected), so they are exact, not lower-cased like the match
// constants above.
const (
	sfRoleLabelPortalUser      = "Portal user"
	sfRoleLabelSecurityContact = "Security Contact"
	sfRoleLabelLead            = "Lead"
	sfRoleLabelAdmin           = "Admin"
)

// validSalesforceRoles is the set a portal write may ask for, keyed by the
// lower-cased label. A role Salesforce would reject is a 400 here rather than
// a Salesforce error mid-transaction — the ingest's own posture of ignoring
// unknown roles is right for a record Salesforce originated, and wrong for
// one a caller is asking us to create.
var validSalesforceRoles = map[string]string{
	sfRolePortalUser:      sfRoleLabelPortalUser,
	sfRoleSecurityContact: sfRoleLabelSecurityContact,
	sfRoleLead:            sfRoleLabelLead,
	sfRoleAdmin:           sfRoleLabelAdmin,
}

// Global role.name values (seeded by the ServiceNow sync) the ingest grants.
const (
	globalRoleExternal      = "external"
	globalRoleCustomer      = "customer"
	globalRolePartner       = "partner"
	globalRoleCustomerAdmin = "customer_admin"
	globalRolePartnerAdmin  = "partner_admin"
)

// project_group."group" values the ingest maps memberships to.
const (
	projectGroupFullAccess    = "Full Access"
	projectGroupGeneralAccess = "General Access"
	projectGroupSecurityOnly  = "Security Only"
	projectGroupLeadUserGroup = "Lead User Group"
	// projectGroupAdmin carries the ADMIN project role (migration 000084).
	// Admin is a per-project fact now; the account-level customer_admin /
	// partner_admin role is derived from every membership a user holds, not
	// stored independently.
	projectGroupAdmin = "Admin"
)

// managedAdminRoles are the global roles the membership write owns
// exclusively: exactly one of them is granted when the user turns out to be
// an admin on any of their projects, and the rest are revoked. Every other
// role a user holds is left alone.
var managedAdminRoles = []string{globalRoleCustomerAdmin, globalRolePartnerAdmin}

// salesforceLastModifiedLayout is how sales-entity-service renders Salesforce
// datetimes, e.g. 2026-09-18T06:37:07.000+0000.
const salesforceLastModifiedLayout = "2006-01-02T15:04:05.000-0700"

// membershipStateAliases folds the spellings Salesforce may use for the
// re-invited state onto project_contact_state_enum's own label.
var membershipStateAliases = map[string]string{
	"RE_INVITED": domain.MembershipStateReInvited,
	"REINVITED":  domain.MembershipStateReInvited,
}

var validMembershipState = map[string]bool{
	domain.MembershipStateInvited:     true,
	domain.MembershipStateRegistered:  true,
	domain.MembershipStateReInvited:   true,
	domain.MembershipStateDeactivated: true,
}

// normalizeMembershipState maps a Salesforce State__c value onto
// project_contact_state_enum. An unknown value is a ValidationError: writing
// it would fail the enum cast anyway, and a 400 tells the replayer exactly
// which record is malformed instead of a generic 500.
func normalizeMembershipState(raw string) (string, error) {
	state := strings.ToUpper(strings.TrimSpace(raw))
	if alias, ok := membershipStateAliases[state]; ok {
		state = alias
	}
	if state == "" {
		return "", &apierror.ValidationError{Msg: "project contact state is required"}
	}
	if !validMembershipState[state] {
		return "", &apierror.ValidationError{Msg: "project contact state " + state + " is not one of INVITED, REGISTERED, RE-INVITED, DEACTIVATED"}
	}
	return state, nil
}

// splitSalesforceRoles turns a Role__c multi-picklist string ("Admin;Portal
// user") into its trimmed parts. Both ';' (Salesforce's own separator) and
// ',' are accepted.
func splitSalesforceRoles(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == ';' || r == ',' })
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// hasSalesforceRole reports whether roles contains want (case-insensitive,
// whitespace-insensitive).
func hasSalesforceRole(roles []string, want string) bool {
	for _, r := range roles {
		if strings.EqualFold(strings.TrimSpace(r), want) {
			return true
		}
	}
	return false
}

// mapGlobalRoles derives the role.name values a membership grants (§6.4):
// every contact is `external`, and a PARTNER CONTACT is `partner` while
// anything else is `customer`. An integration user gets no global roles at
// all (it never signs in), and none are revoked either.
//
// adminRole is only WHICH admin role this contact would hold — never whether
// they hold it. That is decided by the repository, after the membership has
// been written, from every membership the user has (see
// domain.SalesforceMembershipUpsert.AdminRoleName). Deciding it here, from
// the one membership being processed, is exactly the bug this replaces: a
// single non-admin membership used to revoke a user's admin on every project
// they had, because `managed - wanted` was computed against that one record.
func mapGlobalRoles(membershipType string, isIntegrationUser bool) (grant, managed []string, adminRole string) {
	if isIntegrationUser {
		return nil, nil, ""
	}
	partner := strings.EqualFold(strings.TrimSpace(membershipType), domain.MembershipTypePartnerContact)
	grant = []string{globalRoleExternal}
	if partner {
		grant = append(grant, globalRolePartner)
		adminRole = globalRolePartnerAdmin
	} else {
		grant = append(grant, globalRoleCustomer)
		adminRole = globalRoleCustomerAdmin
	}
	return grant, managedAdminRoles, adminRole
}

// mapProjectGroups derives the project_group."group" set for a membership's
// Salesforce roles (§6.4). Portal user + Security Contact is Full Access,
// Portal user alone General Access, Security Contact alone Security Only; a
// Lead additionally joins Lead User Group; an Admin additionally joins Admin,
// the group carrying the ADMIN project role (migration 000084) — Admin used
// to be a global-only role, with nothing recorded per project at all.
// Roles the mapping does not know are returned in ignored so the caller can
// log them; they never fail the ingest.
func mapProjectGroups(roles []string) (groups, ignored []string) {
	var portal, security, lead, admin bool
	for _, raw := range roles {
		switch strings.ToLower(strings.TrimSpace(raw)) {
		case sfRolePortalUser:
			portal = true
		case sfRoleSecurityContact:
			security = true
		case sfRoleLead:
			lead = true
		case sfRoleAdmin:
			admin = true
		case "":
			// blank picklist entry
		default:
			ignored = append(ignored, strings.TrimSpace(raw))
		}
	}
	switch {
	case portal && security:
		groups = append(groups, projectGroupFullAccess)
	case portal:
		groups = append(groups, projectGroupGeneralAccess)
	case security:
		groups = append(groups, projectGroupSecurityOnly)
	}
	if lead {
		groups = append(groups, projectGroupLeadUserGroup)
	}
	if admin {
		groups = append(groups, projectGroupAdmin)
	}
	return groups, ignored
}

// salesforceRolesForGroups is mapProjectGroups' inverse: the raw Salesforce
// Role__c labels a membership's stored project groups represent. The portal
// write path needs it to answer with the roles a membership now carries, and
// the ingest's own echo path never uses it — Salesforce remains the spelling
// authority for a record it originated.
func salesforceRolesForGroups(groups []string) []string {
	var portal, security, lead, admin bool
	for _, g := range groups {
		switch strings.TrimSpace(g) {
		case projectGroupFullAccess:
			portal, security = true, true
		case projectGroupGeneralAccess:
			portal = true
		case projectGroupSecurityOnly:
			security = true
		case projectGroupLeadUserGroup:
			lead = true
		case projectGroupAdmin:
			admin = true
		}
	}
	roles := []string{}
	if portal {
		roles = append(roles, sfRoleLabelPortalUser)
	}
	if security {
		roles = append(roles, sfRoleLabelSecurityContact)
	}
	if lead {
		roles = append(roles, sfRoleLabelLead)
	}
	if admin {
		roles = append(roles, sfRoleLabelAdmin)
	}
	return roles
}

// parseSalesforceLastModified parses sales-entity-service's rendering of
// LastModifiedDate. RFC3339 is accepted as well. ok is false when the value
// is absent or unparseable; the caller then skips the duplicate-event guard
// rather than failing the ingest.
func parseSalesforceLastModified(raw *string) (time.Time, bool) {
	if raw == nil {
		return time.Time{}, false
	}
	v := strings.TrimSpace(*raw)
	if v == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{salesforceLastModifiedLayout, time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, v); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}
