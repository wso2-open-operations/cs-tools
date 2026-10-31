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
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/usermanagement"
)

// TestSalesforceRoleLabelsArePinned guards the literal strings. Salesforce
// matches the Role__c picklist by exact label, so a "tidy-up" to
// "Portal User" would silently grant nothing. These must stay equal to
// entity-service's sfRoleLabel* constants.
func TestSalesforceRoleLabelsArePinned(t *testing.T) {
	got := []string{sfRolePortalUser, sfRoleSecurityContact, sfRoleLead, sfRoleAdmin}
	want := []string{"Portal user", "Security Contact", "Lead", "Admin"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("role labels = %q, want %q", got, want)
	}
}

func TestRolesFromFlags(t *testing.T) {
	tests := []struct {
		name  string
		flags membershipRoleFlags
		want  []string
	}{
		{"none", membershipRoleFlags{}, []string{}},
		{"portal user", membershipRoleFlags{IsPortalUser: true}, []string{"Portal user"}},
		{"security contact", membershipRoleFlags{IsSecurityContact: true}, []string{"Security Contact"}},
		{"lead", membershipRoleFlags{IsLead: true}, []string{"Lead"}},
		{"admin", membershipRoleFlags{IsCsAdmin: true}, []string{"Admin"}},
		{
			"all four in a fixed order",
			membershipRoleFlags{IsCsAdmin: true, IsLead: true, IsSecurityContact: true, IsPortalUser: true},
			[]string{"Portal user", "Security Contact", "Lead", "Admin"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rolesFromFlags(tt.flags)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("rolesFromFlags(%+v) = %q, want %q", tt.flags, got, tt.want)
			}
		})
	}
}

// TestRolesFromFlags_EmptySerialisesAsArray pins the wholesale-replace
// contract: "hold no roles" must reach entity-service as [], never null.
func TestRolesFromFlags_EmptySerialisesAsArray(t *testing.T) {
	body, err := json.Marshal(entity.UpdateProjectMembershipRolesRequest{Roles: RolesFromRoleUpdateRequest(MembershipRoleUpdateRequest{})})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(body) != `{"roles":[]}` {
		t.Errorf("body = %s, want {\"roles\":[]}", body)
	}
}

func TestFlagsFromRoles(t *testing.T) {
	tests := []struct {
		name  string
		roles []string
		want  membershipRoleFlags
	}{
		{"nil", nil, membershipRoleFlags{}},
		{"exact labels", []string{"Portal user", "Security Contact", "Lead", "Admin"},
			membershipRoleFlags{IsPortalUser: true, IsSecurityContact: true, IsLead: true, IsCsAdmin: true}},
		{"casing differs", []string{"PORTAL USER", "security contact", "lead", "ADMIN"},
			membershipRoleFlags{IsPortalUser: true, IsSecurityContact: true, IsLead: true, IsCsAdmin: true}},
		{"surrounding whitespace", []string{"  Portal user ", "\tAdmin\n"},
			membershipRoleFlags{IsPortalUser: true, IsCsAdmin: true}},
		{"unknown label ignored", []string{"Billing Contact", "Lead"},
			membershipRoleFlags{IsLead: true}},
		{"duplicates harmless", []string{"Lead", "lead"},
			membershipRoleFlags{IsLead: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := flagsFromRoles(tt.roles); got != tt.want {
				t.Errorf("flagsFromRoles(%q) = %+v, want %+v", tt.roles, got, tt.want)
			}
		})
	}
}

// TestRoleMapping_RoundTrip checks every one of the 16 flag combinations
// survives flags -> labels -> flags, both verbatim and after Salesforce
// hands the labels back in a different case.
func TestRoleMapping_RoundTrip(t *testing.T) {
	casings := map[string]func(string) string{
		"verbatim": func(s string) string { return s },
		"upper":    strings.ToUpper,
		"lower":    strings.ToLower,
	}
	for mask := 0; mask < 16; mask++ {
		in := membershipRoleFlags{
			IsPortalUser:      mask&1 != 0,
			IsSecurityContact: mask&2 != 0,
			IsLead:            mask&4 != 0,
			IsCsAdmin:         mask&8 != 0,
		}
		for name, recase := range casings {
			labels := rolesFromFlags(in)
			for i := range labels {
				labels[i] = recase(labels[i])
			}
			if got := flagsFromRoles(labels); got != in {
				t.Errorf("%s round trip of %+v = %+v", name, in, got)
			}
		}
	}
}

func TestBuildCreateProjectMembershipRequest(t *testing.T) {
	got := BuildCreateProjectMembershipRequest(ContactOnboardRequest{
		ContactEmail:        "  jane@acme.com ",
		ContactFirstName:    " Jane",
		ContactLastName:     "Doe ",
		IsCsIntegrationUser: true,
		IsPortalUser:        true,
		IsCsAdmin:           true,
	}, " admin@acme.com ")
	want := entity.CreateProjectMembershipRequest{
		Email:               "jane@acme.com",
		FirstName:           "Jane",
		LastName:            "Doe",
		Roles:               []string{"Portal user", "Admin"},
		IsCsIntegrationUser: true,
		InviterEmail:        "admin@acme.com",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestMapEntityMembership(t *testing.T) {
	got := MapEntityMembership(entity.ProjectMembership{
		ProjectContactID: "pc-1",
		ContactSfID:      "003xx",
		Email:            "jane@acme.com",
		State:            "INVITED",
		Roles:            []string{"portal user", "Security Contact"},
	})
	if got.ID != "pc-1" || got.State != "INVITED" {
		t.Errorf("ID/State = %q/%q, want pc-1/INVITED", got.ID, got.State)
	}
	if !got.IsPortalUser || !got.IsSecurityContact || got.IsLead || got.IsCsAdmin {
		t.Errorf("flags = portal:%v security:%v lead:%v admin:%v, want true/true/false/false",
			got.IsPortalUser, got.IsSecurityContact, got.IsLead, got.IsCsAdmin)
	}
	if got.Contact == nil || got.Contact.Email == nil || *got.Contact.Email != "jane@acme.com" ||
		got.Contact.ID == nil || *got.Contact.ID != "003xx" {
		t.Errorf("contact = %+v, want email jane@acme.com and id 003xx", got.Contact)
	}
}

func TestMapEntityMembership_NoEmailLeavesContactNil(t *testing.T) {
	if got := MapEntityMembership(entity.ProjectMembership{ProjectContactID: "pc-1", ContactSfID: "003xx"}); got.Contact != nil {
		t.Errorf("contact = %+v, want nil", got.Contact)
	}
}

func TestMapEntityMembership_NoSfIDLeavesContactIDNil(t *testing.T) {
	got := MapEntityMembership(entity.ProjectMembership{Email: "jane@acme.com"})
	if got.Contact == nil || got.Contact.ID != nil {
		t.Errorf("contact = %+v, want email set and nil id", got.Contact)
	}
}

func TestFlagsFromProjectRoles(t *testing.T) {
	tests := []struct {
		name  string
		roles []string
		want  membershipRoleFlags
	}{
		{"none", nil, membershipRoleFlags{}},
		{"all four", []string{"PORTAL_USER", "SECURITY_CONTACT", "LEAD_USER", "ADMIN"},
			membershipRoleFlags{IsPortalUser: true, IsSecurityContact: true, IsLead: true, IsCsAdmin: true}},
		{"lower case and spaces", []string{" admin ", "portal_user"},
			membershipRoleFlags{IsPortalUser: true, IsCsAdmin: true}},
		{"business contact has no checkbox", []string{"BUSINESS_CONTACT"}, membershipRoleFlags{}},
		{"Salesforce labels are not database roles", []string{"Portal user", "Admin"}, membershipRoleFlags{IsCsAdmin: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := flagsFromProjectRoles(tt.roles); got != tt.want {
				t.Errorf("flagsFromProjectRoles(%q) = %+v, want %+v", tt.roles, got, tt.want)
			}
		})
	}
}

func TestMapEntityProjectContact_OneWordName(t *testing.T) {
	name := "Cher"
	got := MapEntityProjectContact(entity.ProjectContact{Name: &name, Email: "cher@acme.com"})
	if got.FirstName != nil || got.LastName != "Cher" {
		t.Errorf("first/last = %v/%q, want nil/Cher", got.FirstName, got.LastName)
	}
}

func TestMapEntityProjectContact_CanResendInvitation(t *testing.T) {
	for _, tt := range []struct {
		state string
		want  bool
	}{
		{"INVITED", true},
		{"RE-INVITED", true},
		{" invited ", true},
		{"REGISTERED", false},
		{"DEACTIVATED", false},
		{"", false},
	} {
		t.Run(tt.state, func(t *testing.T) {
			got := MapEntityProjectContact(entity.ProjectContact{Email: "a@acme.com", RegistrationState: tt.state})
			if got.CanResendInvitation != tt.want {
				t.Errorf("CanResendInvitation for %q = %v, want %v", tt.state, got.CanResendInvitation, tt.want)
			}
		})
	}
}

// The pre-cutover list never offers a resend: the route answers 404 there.
func TestMapContact_NeverOffersResend(t *testing.T) {
	state := "INVITED"
	got := MapContact(usermanagement.Contact{Email: "a@acme.com", MembershipStatus: &state})
	if got.CanResendInvitation {
		t.Error("a pre-cutover contact must not offer a resend")
	}
}

func TestMapEntityProjectContact_AdminFlags(t *testing.T) {
	tests := []struct {
		name             string
		roles            []string
		accountRoles     []string
		wantCsAdmin      bool
		wantAccountAdmin bool
	}{
		{"project ADMIN only", []string{"ADMIN"}, nil, true, false},
		{"customer_admin only", []string{"PORTAL_USER"}, []string{"external", "customer_admin"}, false, true},
		{"partner_admin only", nil, []string{"partner_admin"}, false, true},
		{"both signals", []string{"ADMIN"}, []string{"customer_admin"}, true, true},
		{"neither", []string{"PORTAL_USER", "LEAD_USER"}, []string{"external", "customer", "partner"}, false, false},
		{"mixed case and whitespace project role", []string{" admin "}, nil, true, false},
		{"mixed case and whitespace account role", nil, []string{"  Customer_Admin "}, false, true},
		{"nil account roles", []string{"PORTAL_USER"}, nil, false, false},
		{"empty everything", nil, nil, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MapEntityProjectContact(entity.ProjectContact{
				Email: "jane.doe@example.com", Roles: tt.roles, AccountRoles: tt.accountRoles,
			})
			if got.IsCsAdmin != tt.wantCsAdmin {
				t.Errorf("IsCsAdmin = %v, want %v", got.IsCsAdmin, tt.wantCsAdmin)
			}
			if got.IsAccountAdmin != tt.wantAccountAdmin {
				t.Errorf("IsAccountAdmin = %v, want %v", got.IsAccountAdmin, tt.wantAccountAdmin)
			}
		})
	}
}

// The account admin signal must not leak into the other flags, including the
// editable project admin flag.
func TestMapEntityProjectContact_AccountAdminLeavesOtherFlagsAlone(t *testing.T) {
	got := MapEntityProjectContact(entity.ProjectContact{
		Email: "jane.doe@example.com", Roles: []string{"LEAD_USER"}, AccountRoles: []string{"partner_admin"},
	})
	if !got.IsAccountAdmin || got.IsCsAdmin || !got.IsLead || got.IsPortalUser || got.IsSecurityContact {
		t.Errorf("flags = accountAdmin %v admin %v lead %v portal %v security %v",
			got.IsAccountAdmin, got.IsCsAdmin, got.IsLead, got.IsPortalUser, got.IsSecurityContact)
	}
}

// isAccountAdmin is omitted when false so the pre-cutover list stays
// byte-identical, and present when true.
func TestContact_IsAccountAdminJSON(t *testing.T) {
	off, err := json.Marshal(Contact{ID: "1", Email: "jane.doe@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(off), "isAccountAdmin") {
		t.Errorf("false value serialised: %s", off)
	}
	on, err := json.Marshal(Contact{ID: "1", Email: "jane.doe@example.com", IsAccountAdmin: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(on), `"isAccountAdmin":true`) {
		t.Errorf("true value missing: %s", on)
	}
}
