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

// Package recipients implements the ACP Phase 1 customer-contact
// resolution: given a project's already-fetched Project Contacts and an
// account's Account Contacts, decide who should receive the customer-facing
// closure notice. Pure — no I/O, same philosophy as the closure package.
package recipients

import "strings"

// ProjectContact mirrors entity-service's project-contact shape (Supported
// by the ServiceNow data source only).
type ProjectContact struct {
	Name  string
	Email string
	Roles []string
}

// AccountContact mirrors entity-service's account-contact shape (Supported
// by the ServiceNow data source only).
type AccountContact struct {
	Name      string
	Email     string
	IsPrimary bool
}

// Contact is a resolved recipient for the closure notice.
type Contact struct {
	Name  string
	Email string
}

// ResolvedVia records which tier of the fallback chain produced the
// resolution, for observability into how often each tier is actually used.
type ResolvedVia string

const (
	ResolvedViaBusinessContact ResolvedVia = "business_contact"
	ResolvedViaPrimaryContact  ResolvedVia = "primary_contact"
	ResolvedViaNone            ResolvedVia = "none"
)

// businessContactRole is the Roles value that marks a project's Business
// Contact. Project roles come from the CSM Postgres database (copied from
// Salesforce), as upper-case enum labels in the contacts search's "roles"
// field: confirmed against a live csm-integration-service v1.1 response on
// 2026-10-01. csm-integration-service v1.0 (ServiceNow) doesn't return
// project roles at all, only ServiceNow access roles such as
// sn_customerservice.customer, so on v1.0 no contact matches and the
// Primary Contact fallback is used.
const businessContactRole = "BUSINESS_CONTACT"

// Resolution is the outcome of resolving who should receive a project's
// customer-facing ACP notice. NeedsAMNudge signals a *different* email (a
// "please configure a business contact" nudge) than the closure notice
// itself — it is not additive with CustomerContacts.
type Resolution struct {
	CustomerContacts []Contact
	NeedsAMNudge     bool
	ResolvedVia      ResolvedVia
}

// ResolveCustomerContacts implements the three-tier fallback: every Project
// Contact with the business-contact role first, then every Primary Contact
// on the account, then a signal to nudge the Account Manager instead. Each
// tier returns all of its matches, not just the first: the ServiceNow system
// sends the customer notice to several customer addresses (confirmed by the
// user from real legacy emails), and real accounts can have more than one
// Primary Contact. A contact only counts if it has a usable (non-empty)
// email — a real contact record with no email on file (confirmed elsewhere
// in this package, see AccountManagerEmail's doc comment, to be a
// legitimate, unremarkable data state) is not a usable recipient, and a tier
// with no usable contact falls through to the next. Each address is listed
// once (compared case-insensitively), in the order the contacts came.
func ResolveCustomerContacts(projectContacts []ProjectContact, accountContacts []AccountContact) Resolution {
	if business := businessContacts(projectContacts); len(business) > 0 {
		return Resolution{CustomerContacts: business, ResolvedVia: ResolvedViaBusinessContact}
	}

	var primary []Contact
	for _, c := range accountContacts {
		if c.IsPrimary {
			primary = appendUniqueEmail(primary, Contact{Name: c.Name, Email: c.Email})
		}
	}
	if len(primary) > 0 {
		return Resolution{CustomerContacts: primary, ResolvedVia: ResolvedViaPrimaryContact}
	}

	return Resolution{NeedsAMNudge: true, ResolvedVia: ResolvedViaNone}
}

// appendUniqueEmail appends c unless its email is empty or already in list.
func appendUniqueEmail(list []Contact, c Contact) []Contact {
	if c.Email == "" {
		return list
	}
	for _, existing := range list {
		if strings.EqualFold(existing.Email, c.Email) {
			return list
		}
	}
	return append(list, c)
}

// PersonRef mirrors entity-service's person-reference shape (technicalOwner/
// accountManager/renewalAccountManager on an Account, per entity-service's
// domain.PersonRef). Email is nullable — an assigned person doesn't always
// have a recorded email. Supported by the ServiceNow data source only.
type PersonRef struct {
	ID    string
	Name  string
	Email *string
}

// AccountManagerEmail returns an account's Account Manager email, given its
// already-fetched accountManager reference (nil if the account has none
// assigned). Returns "" both when there's no Account Manager and when one is
// assigned but has no recorded email — both are legitimate, unremarkable
// states for a real account (confirmed: many accounts have incomplete role
// assignments), not error conditions the caller needs to distinguish.
func AccountManagerEmail(accountManager *PersonRef) string {
	if accountManager == nil || accountManager.Email == nil {
		return ""
	}
	return *accountManager.Email
}

// HasBusinessContacts reports whether ResolveCustomerContacts will resolve
// from projectContacts alone and never look at account contacts. Callers use
// it to skip fetching account contacts that wouldn't be used. It shares
// businessContacts with the resolver, so the two can't disagree about what a
// usable business contact is.
func HasBusinessContacts(projectContacts []ProjectContact) bool {
	return len(businessContacts(projectContacts)) > 0
}

// businessContacts returns the usable business contacts: the role and an
// email, each address once, in order.
func businessContacts(projectContacts []ProjectContact) []Contact {
	var business []Contact
	for _, c := range projectContacts {
		if hasBusinessContactRole(c) {
			business = appendUniqueEmail(business, Contact{Name: c.Name, Email: c.Email})
		}
	}
	return business
}

func hasBusinessContactRole(c ProjectContact) bool {
	for _, r := range c.Roles {
		if r == businessContactRole {
			return true
		}
	}
	return false
}
