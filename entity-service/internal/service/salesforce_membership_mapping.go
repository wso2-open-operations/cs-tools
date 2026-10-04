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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesentity"
)

// Salesforce Project_Contact__c roles (Role__c multi-picklist labels) the
// ingest understands. Anything else is reported back as ignored.
const (
	sfRolePortalUser      = "portal user"
	sfRoleSecurityContact = "security contact"
	sfRoleLead            = "lead"
	sfRoleAdmin           = "admin"
	// sfRoleBusinessContact is ingested but, unlike the four above, not
	// accepted from a portal write (validSalesforceRoles): no portal screen
	// offers it, so it is set in Salesforce only.
	sfRoleBusinessContact = "business contact"
)

// sfRoleLabelBusinessContact is the Business Contact label in Salesforce's
// own spelling, for mapping the stored group back to Role__c.
const sfRoleLabelBusinessContact = "Business Contact"

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
	// projectGroupAdmin carries the ADMIN project role (migration 0128).
	// Admin is a per-project fact now; the account-level customer_admin /
	// partner_admin role is derived from every membership a user holds, not
	// stored independently.
	projectGroupAdmin = "Admin"
	// projectGroupBusinessContact carries the BUSINESS_CONTACT project role.
	// The two spaces are deliberate: it is the name ServiceNow gave the group
	// and the name stored in project_group, matched exactly.
	projectGroupBusinessContact = "Business Contact  Group"
)

// salesforceAccountClassificationPartner is the Salesforce account
// classification that makes a contact a partner rather than a customer.
const salesforceAccountClassificationPartner = "Partner"

// managedAdminRoles are the global roles the membership write owns
// exclusively: exactly one of them is granted when the user turns out to be
// an admin on any of their projects, and the rest are revoked. Every other
// role a user holds is left alone.
var managedAdminRoles = []string{globalRoleCustomerAdmin, globalRolePartnerAdmin}

// managedOrgRoles is the {customer, partner} pair: exactly one is held, the
// one the contact's account classification picks, and the other is revoked,
// so a reclassified account flips the role instead of accumulating both.
var managedOrgRoles = []string{globalRoleCustomer, globalRolePartner}

// ignoredSalesforceRoles are the Role__c labels that have no CSM project group
// and no reader (decision D2): they are dropped without a warning of their
// own, and reported once per membership with the rest of the ignored labels.
var ignoredSalesforceRoles = map[string]bool{
	"business owner":      true,
	"business promoter":   true,
	"business detractor":  true,
	"technical owner":     true,
	"technical champion":  true,
	"technical detractor": true,
}

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

// splitIgnoredRoles separates the ignored Role__c labels into the ones the
// ingest drops by decision (ignoredSalesforceRoles) and the ones it has
// never heard of, so the caller can log a vocabulary gap louder than a
// known, deliberate omission.
func splitIgnoredRoles(ignored []string) (byDecision, unknown []string) {
	for _, r := range ignored {
		if ignoredSalesforceRoles[strings.ToLower(strings.TrimSpace(r))] {
			byDecision = append(byDecision, r)
		} else {
			unknown = append(unknown, r)
		}
	}
	return byDecision, unknown
}

// contactRoleMapping is the global-role side of a contact: what to grant,
// which managed pairs to enforce, and which admin role applies.
type contactRoleMapping struct {
	// Grant is external plus customer or partner.
	Grant []string
	// ManagedGlobal is managedOrgRoles when the account classification is
	// known, nil when it is not (nothing is then revoked).
	ManagedGlobal []string
	// ManagedAdmin is managedAdminRoles, nil for an integration user.
	ManagedAdmin []string
	// AdminRole is which admin role this contact would hold -- never whether
	// they hold it. That is decided by the repository, after the write, from
	// every membership the user has (see
	// domain.SalesforceMembershipUpsert.AdminRoleName). Deciding it from the
	// one membership being processed is exactly the bug that design
	// replaced: a single non-admin membership used to revoke a user's admin
	// on every project they had.
	AdminRole string
}

// mapGlobalRoles derives the global roles of a contact from its account
// classification (decision D1, the ServiceNow script's basis): every contact
// is `external`, a contact whose account is classified Partner is `partner`
// with partner_admin as its admin role, and anything else is `customer` with
// customer_admin. The Contact writer, the membership ingest and the portal
// writes all call this, so they agree.
//
// account nil, or an account with a blank classification, means the
// classification is unknown (a portal write that has just created the
// contact, whose create response carries no account; an account Sales Entity
// sent without a classification): the contact is treated as a customer, as
// before, but ManagedGlobal is nil so a partner role the user already holds
// is not revoked on missing evidence.
//
// An integration user gets no global roles at all (it never signs in), and
// none are revoked either.
func mapGlobalRoles(account *salesentity.ContactAccount, isIntegrationUser bool) contactRoleMapping {
	if isIntegrationUser {
		return contactRoleMapping{}
	}
	m := contactRoleMapping{Grant: []string{globalRoleExternal}, ManagedAdmin: managedAdminRoles}
	if account != nil && strings.TrimSpace(derefString(account.Classification)) != "" {
		m.ManagedGlobal = managedOrgRoles
	}
	if isPartnerAccount(account) {
		m.Grant = append(m.Grant, globalRolePartner)
		m.AdminRole = globalRolePartnerAdmin
	} else {
		m.Grant = append(m.Grant, globalRoleCustomer)
		m.AdminRole = globalRoleCustomerAdmin
	}
	return m
}

// isPartnerAccount reports whether a contact's account is classified Partner.
func isPartnerAccount(account *salesentity.ContactAccount) bool {
	return account != nil && strings.EqualFold(strings.TrimSpace(derefString(account.Classification)), salesforceAccountClassificationPartner)
}

// mapProjectGroups derives the project_group."group" set for a membership's
// Salesforce roles (§6.4). Portal user + Security Contact is Full Access,
// Portal user alone General Access, Security Contact alone Security Only; a
// Lead additionally joins Lead User Group; an Admin additionally joins Admin,
// the group carrying the ADMIN project role (migration 0128) — Admin used
// to be a global-only role, with nothing recorded per project at all; a
// Business Contact additionally joins "Business Contact  Group", the group
// carrying the BUSINESS_CONTACT project role that the ServiceNow sync used
// to write.
// Roles the mapping does not know are returned in ignored so the caller can
// log them (once per membership); they never fail the ingest. That includes
// the six labels with no CSM group (ignoredSalesforceRoles, decision D2).
func mapProjectGroups(roles []string) (groups, ignored []string) {
	var portal, security, lead, admin, businessContact bool
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
		case sfRoleBusinessContact:
			businessContact = true
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
	if businessContact {
		groups = append(groups, projectGroupBusinessContact)
	}
	return groups, ignored
}

// salesforceRolesForGroups is mapProjectGroups' inverse: the raw Salesforce
// Role__c labels a membership's stored project groups represent. The portal
// write path needs it to answer with the roles a membership now carries, and
// the ingest's own echo path never uses it — Salesforce remains the spelling
// authority for a record it originated.
func salesforceRolesForGroups(groups []string) []string {
	var portal, security, lead, admin, businessContact bool
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
		case projectGroupBusinessContact:
			businessContact = true
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
	if businessContact {
		roles = append(roles, sfRoleLabelBusinessContact)
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
