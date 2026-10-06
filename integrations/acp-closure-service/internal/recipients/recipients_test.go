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

package recipients

import (
	"slices"
	"testing"
)

func TestAccountManagerEmail_ReturnsEmailWhenPopulated(t *testing.T) {
	email := "am@wso2.example"
	am := &PersonRef{ID: "am-1", Name: "Jordan Perera", Email: &email}

	if got := AccountManagerEmail(am); got != email {
		t.Errorf("AccountManagerEmail() = %q, want %q", got, email)
	}
}

func TestAccountManagerEmail_ReturnsEmptyWhenNoAccountManager(t *testing.T) {
	if got := AccountManagerEmail(nil); got != "" {
		t.Errorf("AccountManagerEmail(nil) = %q, want \"\"", got)
	}
}

func TestAccountManagerEmail_ReturnsEmptyWhenAccountManagerHasNoEmail(t *testing.T) {
	am := &PersonRef{ID: "am-1", Name: "Jordan Perera", Email: nil}

	if got := AccountManagerEmail(am); got != "" {
		t.Errorf("AccountManagerEmail() = %q, want \"\"", got)
	}
}

func TestResolveCustomerContacts_PrefersBusinessContact(t *testing.T) {
	projectContacts := []ProjectContact{
		{Name: "Alice", Email: "alice@customer.example", Roles: []string{"developer"}},
		{Name: "Bob", Email: "bob@customer.example", Roles: []string{businessContactRole, "developer"}},
	}
	accountContacts := []AccountContact{
		{Name: "Carol", Email: "carol@customer.example", IsPrimary: true},
	}

	got := ResolveCustomerContacts(projectContacts, accountContacts)

	if got.NeedsAMNudge {
		t.Fatalf("NeedsAMNudge = true, want false")
	}
	if got.ResolvedVia != ResolvedViaBusinessContact {
		t.Errorf("ResolvedVia = %v, want %v", got.ResolvedVia, ResolvedViaBusinessContact)
	}
	if len(got.CustomerContacts) != 1 || got.CustomerContacts[0].Email != "bob@customer.example" {
		t.Errorf("CustomerContacts = %+v, want Bob", got.CustomerContacts)
	}
}

func TestResolveCustomerContacts_FallsBackToPrimaryContact(t *testing.T) {
	projectContacts := []ProjectContact{
		{Name: "Alice", Email: "alice@customer.example", Roles: []string{"developer"}},
	}
	accountContacts := []AccountContact{
		{Name: "Dana", Email: "dana@customer.example", IsPrimary: false},
		{Name: "Carol", Email: "carol@customer.example", IsPrimary: true},
	}

	got := ResolveCustomerContacts(projectContacts, accountContacts)

	if got.NeedsAMNudge {
		t.Fatalf("NeedsAMNudge = true, want false")
	}
	if got.ResolvedVia != ResolvedViaPrimaryContact {
		t.Errorf("ResolvedVia = %v, want %v", got.ResolvedVia, ResolvedViaPrimaryContact)
	}
	if len(got.CustomerContacts) != 1 || got.CustomerContacts[0].Email != "carol@customer.example" {
		t.Errorf("CustomerContacts = %+v, want Carol", got.CustomerContacts)
	}
}

// TestResolveCustomerContacts_SkipsBusinessContactWithEmptyEmail covers a
// real, unremarkable data state (mirrors AccountManagerEmail's treatment of
// "assigned but no email" elsewhere in this package): a Project Contact has
// the business-contact role but no email on file. Accepting it anyway would
// resolve to Recipient: "" instead of falling through to a usable tier.
func TestResolveCustomerContacts_SkipsBusinessContactWithEmptyEmail(t *testing.T) {
	projectContacts := []ProjectContact{
		{Name: "Bob", Email: "", Roles: []string{businessContactRole}},
	}
	accountContacts := []AccountContact{
		{Name: "Carol", Email: "carol@customer.example", IsPrimary: true},
	}

	got := ResolveCustomerContacts(projectContacts, accountContacts)

	if got.NeedsAMNudge {
		t.Fatalf("NeedsAMNudge = true, want false")
	}
	if got.ResolvedVia != ResolvedViaPrimaryContact {
		t.Errorf("ResolvedVia = %v, want %v", got.ResolvedVia, ResolvedViaPrimaryContact)
	}
	if len(got.CustomerContacts) != 1 || got.CustomerContacts[0].Email != "carol@customer.example" {
		t.Errorf("CustomerContacts = %+v, want Carol", got.CustomerContacts)
	}
}

// TestResolveCustomerContacts_NudgesAccountManagerWhenOnlyContactsHaveEmptyEmail
// covers both tiers resolving to a real contact, but neither having a usable
// email — must fall through to NeedsAMNudge rather than resolving to an
// empty Recipient.
func TestResolveCustomerContacts_NudgesAccountManagerWhenOnlyContactsHaveEmptyEmail(t *testing.T) {
	projectContacts := []ProjectContact{
		{Name: "Bob", Email: "", Roles: []string{businessContactRole}},
	}
	accountContacts := []AccountContact{
		{Name: "Carol", Email: "", IsPrimary: true},
	}

	got := ResolveCustomerContacts(projectContacts, accountContacts)

	if !got.NeedsAMNudge {
		t.Fatalf("NeedsAMNudge = false, want true")
	}
	if len(got.CustomerContacts) != 0 {
		t.Errorf("CustomerContacts = %+v, want none", got.CustomerContacts)
	}
	if got.ResolvedVia != ResolvedViaNone {
		t.Errorf("ResolvedVia = %v, want %v", got.ResolvedVia, ResolvedViaNone)
	}
}

func TestResolveCustomerContacts_NudgesAccountManagerWhenNoContactFound(t *testing.T) {
	projectContacts := []ProjectContact{
		{Name: "Alice", Email: "alice@customer.example", Roles: []string{"developer"}},
	}
	accountContacts := []AccountContact{
		{Name: "Dana", Email: "dana@customer.example", IsPrimary: false},
	}

	got := ResolveCustomerContacts(projectContacts, accountContacts)

	if !got.NeedsAMNudge {
		t.Fatalf("NeedsAMNudge = false, want true")
	}
	if len(got.CustomerContacts) != 0 {
		t.Errorf("CustomerContacts = %+v, want none", got.CustomerContacts)
	}
	if got.ResolvedVia != ResolvedViaNone {
		t.Errorf("ResolvedVia = %v, want %v", got.ResolvedVia, ResolvedViaNone)
	}
}

func TestResolveCustomerContacts_NudgesAccountManagerWhenNoContactsAtAll(t *testing.T) {
	got := ResolveCustomerContacts(nil, nil)

	if !got.NeedsAMNudge {
		t.Fatalf("NeedsAMNudge = false, want true")
	}
	if len(got.CustomerContacts) != 0 {
		t.Errorf("CustomerContacts = %+v, want none", got.CustomerContacts)
	}
	if got.ResolvedVia != ResolvedViaNone {
		t.Errorf("ResolvedVia = %v, want %v", got.ResolvedVia, ResolvedViaNone)
	}
}

// TestResolveCustomerContacts_ReturnsEveryBusinessContact: the customer
// notice goes to every business contact on the project, as the ServiceNow
// system does (confirmed by the user from real legacy emails with several
// customer addresses), not just the first one found. Contacts without an
// email, or without the role, are left out; order is kept.
func TestResolveCustomerContacts_ReturnsEveryBusinessContact(t *testing.T) {
	projectContacts := []ProjectContact{
		{Name: "Bob", Email: "bob@customer.example", Roles: []string{businessContactRole}},
		{Name: "Alice", Email: "alice@customer.example", Roles: []string{"developer"}},
		{Name: "Eve", Email: "", Roles: []string{businessContactRole}},
		{Name: "Frank", Email: "frank@customer.example", Roles: []string{"developer", businessContactRole}},
	}
	accountContacts := []AccountContact{
		{Name: "Carol", Email: "carol@customer.example", IsPrimary: true},
	}

	got := ResolveCustomerContacts(projectContacts, accountContacts)

	if got.ResolvedVia != ResolvedViaBusinessContact {
		t.Errorf("ResolvedVia = %v, want %v", got.ResolvedVia, ResolvedViaBusinessContact)
	}
	want := []Contact{
		{Name: "Bob", Email: "bob@customer.example"},
		{Name: "Frank", Email: "frank@customer.example"},
	}
	if !slices.Equal(got.CustomerContacts, want) {
		t.Errorf("CustomerContacts = %+v, want %+v", got.CustomerContacts, want)
	}
}

// TestResolveCustomerContacts_ReturnsEveryPrimaryContact: with no business
// contact, every Primary Contact on the account gets the notice. Real
// accounts can have more than one (the staging ACP Test Partner Account
// has two).
func TestResolveCustomerContacts_ReturnsEveryPrimaryContact(t *testing.T) {
	accountContacts := []AccountContact{
		{Name: "Carol", Email: "carol@customer.example", IsPrimary: true},
		{Name: "Dana", Email: "dana@customer.example", IsPrimary: false},
		{Name: "Gina", Email: "gina@customer.example", IsPrimary: true},
	}

	got := ResolveCustomerContacts(nil, accountContacts)

	if got.ResolvedVia != ResolvedViaPrimaryContact {
		t.Errorf("ResolvedVia = %v, want %v", got.ResolvedVia, ResolvedViaPrimaryContact)
	}
	want := []Contact{
		{Name: "Carol", Email: "carol@customer.example"},
		{Name: "Gina", Email: "gina@customer.example"},
	}
	if !slices.Equal(got.CustomerContacts, want) {
		t.Errorf("CustomerContacts = %+v, want %+v", got.CustomerContacts, want)
	}
}

// TestResolveCustomerContacts_ListsEachEmailOnce: the same address appearing
// twice (differing only in case) is one recipient, not two copies of the
// same email.
func TestResolveCustomerContacts_ListsEachEmailOnce(t *testing.T) {
	projectContacts := []ProjectContact{
		{Name: "Bob", Email: "bob@customer.example", Roles: []string{businessContactRole}},
		{Name: "Bob Again", Email: "Bob@Customer.example", Roles: []string{businessContactRole}},
	}

	got := ResolveCustomerContacts(projectContacts, nil)

	want := []Contact{{Name: "Bob", Email: "bob@customer.example"}}
	if !slices.Equal(got.CustomerContacts, want) {
		t.Errorf("CustomerContacts = %+v, want %+v", got.CustomerContacts, want)
	}
}

// TestResolveCustomerContacts_MatchesTheRealBusinessContactRole pins the
// role value to what the API actually returns. csm-integration-service v1.1
// (CSM Postgres) puts project roles in "roles" as upper-case enum labels
// (BUSINESS_CONTACT, PORTAL_USER, SECURITY_CONTACT, LEAD_USER, ADMIN),
// confirmed against a live response on 2026-10-01. ServiceNow access roles
// such as sn_customerservice.customer, which is all v1.0 returns, never
// count as a business contact.
func TestResolveCustomerContacts_MatchesTheRealBusinessContactRole(t *testing.T) {
	projectContacts := []ProjectContact{
		{Name: "Bea Business", Email: "bea.business@customer.example", Roles: []string{"BUSINESS_CONTACT"}},
		{Name: "Paul Portal", Email: "paul.portal@customer.example", Roles: []string{"PORTAL_USER", "SECURITY_CONTACT"}},
		{Name: "Pat Partner", Email: "pat.partner@customer.example", Roles: []string{"sn_customerservice.partner", "sn_customerservice.customer"}},
	}

	got := ResolveCustomerContacts(projectContacts, nil)

	if got.ResolvedVia != ResolvedViaBusinessContact {
		t.Errorf("ResolvedVia = %v, want %v", got.ResolvedVia, ResolvedViaBusinessContact)
	}
	want := []Contact{{Name: "Bea Business", Email: "bea.business@customer.example"}}
	if !slices.Equal(got.CustomerContacts, want) {
		t.Errorf("CustomerContacts = %+v, want %+v", got.CustomerContacts, want)
	}
}

func TestHasBusinessContacts(t *testing.T) {
	tests := []struct {
		name     string
		contacts []ProjectContact
		want     bool
	}{
		{"business contact with email", []ProjectContact{{Email: "a@customer.example", Roles: []string{"PORTAL_USER", businessContactRole}}}, true},
		{"business contact without email", []ProjectContact{{Email: "", Roles: []string{businessContactRole}}}, false},
		{"only other roles", []ProjectContact{{Email: "a@customer.example", Roles: []string{"PORTAL_USER", "sn_customerservice.customer"}}}, false},
		{"no contacts", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasBusinessContacts(tt.contacts); got != tt.want {
				t.Errorf("HasBusinessContacts() = %v, want %v", got, tt.want)
			}
		})
	}
}
