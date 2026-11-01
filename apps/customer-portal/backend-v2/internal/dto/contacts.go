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
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package dto

import (
	"regexp"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/usermanagement"
)

// contactEmailRE matches this API's own constraint on
// ContactOnboardPayload.ContactEmail exactly.
var contactEmailRE = regexp.MustCompile(`^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$`)

// ValidContactEmail reports whether email satisfies the project-contact
// onboarding service's email-format constraint.
func ValidContactEmail(email string) bool {
	return contactEmailRE.MatchString(email)
}

// ContactAccount is the account summary embedded in a Contact.
type ContactAccount struct {
	ID             *string `json:"id,omitempty"`
	DomainList     string  `json:"domainList,omitempty"`
	Classification string  `json:"classification,omitempty"`
	IsPartner      *bool   `json:"isPartner,omitempty"`
}

// Contact is a project contact.
type Contact struct {
	ID                  string          `json:"id"`
	Email               string          `json:"email"`
	FirstName           *string         `json:"firstName,omitempty"`
	LastName            string          `json:"lastName"`
	IsCsAdmin           bool            `json:"isCsAdmin"`
	IsLead              bool            `json:"isLead"`
	IsCsIntegrationUser bool            `json:"isCsIntegrationUser"`
	IsPortalUser        bool            `json:"isPortalUser"`
	IsSecurityContact   bool            `json:"isSecurityContact"`
	MembershipStatus    *string         `json:"membershipStatus,omitempty"`
	Account             *ContactAccount `json:"account,omitempty"`
	// CanResendInvitation is set only on database-backed rows
	// (CSM_MIGRATION_PORTAL_CONTACTS_ENABLED) whose invitation is still
	// outstanding, so the webapp shows the resend action exactly when
	// POST .../resend-invitation can succeed. It is never set before cutover.
	CanResendInvitation bool `json:"canResendInvitation,omitempty"`
	// IsAccountAdmin is read-only: true when the contact holds an account-level
	// admin role. It is set only on database-backed rows and is never read on
	// the write path, so it cannot turn into a per-project role. IsCsAdmin stays
	// the project-level Admin role, which is the editable one.
	IsAccountAdmin bool `json:"isAccountAdmin,omitempty"`
}

// MapContact builds the portal response from a usermanagement.Contact.
func MapContact(c usermanagement.Contact) Contact {
	out := Contact{
		ID:                  c.ID,
		Email:               c.Email,
		FirstName:           c.FirstName,
		LastName:            c.LastName,
		IsCsAdmin:           c.IsCsAdmin,
		IsLead:              c.IsLead,
		IsCsIntegrationUser: c.IsCsIntegrationUser,
		IsPortalUser:        c.IsPortalUser,
		IsSecurityContact:   c.IsSecurityContact,
		MembershipStatus:    c.MembershipStatus,
	}
	if c.Account != nil {
		out.Account = &ContactAccount{
			ID:             c.Account.ID,
			DomainList:     c.Account.DomainList,
			Classification: c.Account.Classification,
			IsPartner:      c.Account.IsPartner,
		}
	}
	return out
}

// MapContacts builds the portal response list from usermanagement.Contacts.
func MapContacts(contacts []usermanagement.Contact) []Contact {
	out := make([]Contact, 0, len(contacts))
	for _, c := range contacts {
		out = append(out, MapContact(c))
	}
	return out
}

// ContactOnboardRequest is the portal's request body for
// POST /projects/{id}/contacts. AdminEmail is never client-supplied — the
// calling handler always injects the caller's own email.
type ContactOnboardRequest struct {
	ContactEmail        string `json:"contactEmail"`
	ContactFirstName    string `json:"contactFirstName"`
	ContactLastName     string `json:"contactLastName"`
	IsCsIntegrationUser bool   `json:"isCsIntegrationUser"`
	IsCsAdmin           bool   `json:"isCsAdmin,omitempty"`
	IsLead              bool   `json:"isLead,omitempty"`
	IsPortalUser        bool   `json:"isPortalUser,omitempty"`
	IsSecurityContact   bool   `json:"isSecurityContact,omitempty"`
}

// BuildOnBoardContactPayload builds the user-management client's request
// from the portal request plus the caller's own email as AdminEmail.
func BuildOnBoardContactPayload(req ContactOnboardRequest, adminEmail string) usermanagement.OnBoardContactPayload {
	return usermanagement.OnBoardContactPayload{
		ContactEmail:        req.ContactEmail,
		AdminEmail:          adminEmail,
		ContactFirstName:    req.ContactFirstName,
		ContactLastName:     req.ContactLastName,
		IsCsIntegrationUser: req.IsCsIntegrationUser,
		IsCsAdmin:           req.IsCsAdmin,
		IsLead:              req.IsLead,
		IsPortalUser:        req.IsPortalUser,
		IsSecurityContact:   req.IsSecurityContact,
	}
}

// ContactRef is the minimal contact reference embedded in a Membership.
type ContactRef struct {
	ID    *string `json:"id,omitempty"`
	Email *string `json:"email,omitempty"`
}

// Membership is a contact's membership on a project.
type Membership struct {
	ID                string      `json:"id"`
	State             string      `json:"state"`
	IsCsAdmin         bool        `json:"isCsAdmin"`
	IsLead            bool        `json:"isLead"`
	IsPortalUser      bool        `json:"isPortalUser"`
	IsSecurityContact bool        `json:"isSecurityContact"`
	Contact           *ContactRef `json:"contact,omitempty"`
}

// MapMembership builds the portal response from a usermanagement.Membership.
func MapMembership(m usermanagement.Membership) Membership {
	out := Membership{
		ID:                m.ID,
		State:             m.State,
		IsCsAdmin:         m.IsCsAdmin,
		IsLead:            m.IsLead,
		IsPortalUser:      m.IsPortalUser,
		IsSecurityContact: m.IsSecurityContact,
	}
	if m.Contact != nil {
		out.Contact = &ContactRef{ID: m.Contact.ID, Email: m.Contact.Email}
	}
	return out
}

// MembershipRoleUpdateRequest is the portal's request body for
// PATCH /projects/{id}/contacts/{email}. AdminEmail is never client-supplied.
type MembershipRoleUpdateRequest struct {
	IsCsAdmin         bool `json:"isCsAdmin,omitempty"`
	IsLead            bool `json:"isLead,omitempty"`
	IsPortalUser      bool `json:"isPortalUser,omitempty"`
	IsSecurityContact bool `json:"isSecurityContact,omitempty"`
}

// BuildMembershipRolePayload builds the user-management client's request
// from the portal request plus the caller's own email as AdminEmail.
func BuildMembershipRolePayload(req MembershipRoleUpdateRequest, adminEmail string) usermanagement.MembershipRolePayload {
	return usermanagement.MembershipRolePayload{
		AdminEmail:        adminEmail,
		IsCsAdmin:         req.IsCsAdmin,
		IsLead:            req.IsLead,
		IsPortalUser:      req.IsPortalUser,
		IsSecurityContact: req.IsSecurityContact,
	}
}

// ContactValidationRequest is the portal's request body for
// POST /projects/{id}/contacts/validate.
type ContactValidationRequest struct {
	ContactEmail string `json:"contactEmail"`
}

// ContactValidationResponse is the portal's response for
// POST /projects/{id}/contacts/validate.
type ContactValidationResponse struct {
	IsContactValid bool     `json:"isContactValid"`
	Message        string   `json:"message"`
	ContactDetails *Contact `json:"contactDetails,omitempty"`
}

// The messages POST /projects/{id}/contacts/validate answers with. They are
// part of the webapp's contract: it shows them as they are, so both the
// pre-cutover and the entity-service path use exactly these.
const (
	// ContactValidationMsgExistingContact accompanies a 200 whose
	// contactDetails carry the Salesforce contact the invitation would reuse.
	ContactValidationMsgExistingContact = "Contact is valid but already exists in the project!"
	// ContactValidationMsgNewContact accompanies a 200 for a new address.
	ContactValidationMsgNewContact = "Project contact is valid and can be added to the project!"
	// ContactValidationMsgConflict is the 409 message.
	ContactValidationMsgConflict = "Contact with the provided email already exists in the project!"
	// ContactValidationMsgInvalidEmail is the pre-cutover onboarding
	// service's own wording for an address that is not one.
	ContactValidationMsgInvalidEmail = "We're sorry, but the email address you entered for your contact information is invalid. " +
		"Please provide a valid email address to proceed."
)
