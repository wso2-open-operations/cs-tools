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

package sweep

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp/salesentity"
)

func sp(s string) *string { return &s }

// fakeSalesforce stands in for the sales-entity-service client.
type fakeSalesforce struct {
	projects      map[string]salesentity.Project
	customers     map[string]salesentity.Customer
	members       map[string][]salesentity.ProjectContact
	contacts      map[string][]salesentity.Contact
	links         map[string][]salesentity.LinkedOpportunity
	opportunities map[string]salesentity.Opportunity
	invoices      map[string][]salesentity.Invoice
	calls         []string
}

func (f *fakeSalesforce) GetProject(_ context.Context, id string) (salesentity.Project, error) {
	f.calls = append(f.calls, "project "+id)
	p, ok := f.projects[id]
	if !ok {
		return salesentity.Project{}, salesentity.ErrNotFound
	}
	return p, nil
}
func (f *fakeSalesforce) GetCustomer(_ context.Context, id string) (salesentity.Customer, error) {
	f.calls = append(f.calls, "customer "+id)
	c, ok := f.customers[id]
	if !ok {
		return salesentity.Customer{}, salesentity.ErrNotFound
	}
	return c, nil
}
func (f *fakeSalesforce) ProjectContacts(_ context.Context, id string) ([]salesentity.ProjectContact, error) {
	f.calls = append(f.calls, "members "+id)
	return f.members[id], nil
}
func (f *fakeSalesforce) AccountContacts(_ context.Context, id string) ([]salesentity.Contact, error) {
	f.calls = append(f.calls, "contacts "+id)
	return f.contacts[id], nil
}
func (f *fakeSalesforce) LinkedOpportunities(_ context.Context, id string) ([]salesentity.LinkedOpportunity, error) {
	f.calls = append(f.calls, "links "+id)
	return f.links[id], nil
}
func (f *fakeSalesforce) Opportunities(_ context.Context, ids []string) ([]salesentity.Opportunity, error) {
	f.calls = append(f.calls, "opportunities")
	var out []salesentity.Opportunity
	for _, id := range ids {
		if o, ok := f.opportunities[id]; ok {
			out = append(out, o)
		}
	}
	return out, nil
}
func (f *fakeSalesforce) Invoices(_ context.Context, id string) ([]salesentity.Invoice, error) {
	f.calls = append(f.calls, "invoices "+id)
	return f.invoices[id], nil
}

// partnerFixture is the staging partner project as Salesforce has it.
func partnerFixture() *fakeSalesforce {
	return &fakeSalesforce{
		projects: map[string]salesentity.Project{
			"a0dP": {ID: "a0dP", StartDate: sp("2025-07-01"), EndDate: sp("2026-10-13"), CustomerID: sp("001P")},
		},
		customers: map[string]salesentity.Customer{
			"001P": {ID: "001P", Owner: &salesentity.User{Name: sp("Kavith R"), Email: sp("kavithr@wso2.com")},
				TechnicalOwner: sp("to@wso2.com"), RenewalManager: &salesentity.User{Name: sp("Abinash A"), Email: sp("abinash@wso2.com")},
				AccountClassification: sp("Partner")},
		},
		members: map[string][]salesentity.ProjectContact{
			"a0dP": {
				{Email: "invited@acme.com", State: sp("REGISTERED"), Roles: []string{"Portal user", "Business Contact"},
					Contact: &salesentity.ProjectContactPerson{Name: sp("Jane B"), Email: sp("jane@acme.com")}},
				{Email: "gone@acme.com", State: sp("DEACTIVATED"), Roles: []string{"Business Contact"}},
				{Email: "dev@acme.com", State: sp("INVITED"), Roles: []string{"Portal user"}},
			},
		},
		contacts: map[string][]salesentity.Contact{
			"001P": {{Name: sp("Primary P"), Email: sp("primary@acme.com"), IsPrimaryContact: true}, {Name: sp("Other"), Email: sp("other@acme.com")}},
		},
		links: map[string][]salesentity.LinkedOpportunity{"a0dP": {{ID: "l1", OpportunityID: sp("006A")}}},
		opportunities: map[string]salesentity.Opportunity{
			"006A": {ID: "006A", Name: sp("ACP Partner Opportunity"), StageName: sp("50 - Closed Won"), EulaVersion: sp("EULA 3.4")},
		},
		invoices: map[string][]salesentity.Invoice{
			"006A": {{ID: sp("a0IA"), Name: sp("ACP Partner Invoice"), OpportunityID: sp("006A"), DueDate: sp("2026-09-28"),
				InvoiceDate: sp("2026-09-01"), Classification: sp("SUB")}},
		},
	}
}

func csmPartnerProject() project {
	return project{ID: "e3e87599-csm", Name: "ACP Partner Project - Subscription", ProjectKey: "APPSUB",
		Account: &projectAccountRef{ID: "f213fdd1-csm", Name: "ACP Test Partner Account"}, SfID: sp("a0dP")}
}

// TestSalesforceDetails_ProjectFieldsComeFromSalesforce: dates, the account
// and the partner flag are Salesforce's; the CSM id, key and name (used for
// writes and the notices) stay.
func TestSalesforceDetails_ProjectFieldsComeFromSalesforce(t *testing.T) {
	d := NewSalesforceDetails(partnerFixture())
	stale := csmPartnerProject()
	stale.EndDate = timePtr(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC))

	got, _, err := d.forProject(context.Background(), stale)
	if err != nil {
		t.Fatalf("forProject: %v", err)
	}
	if got.ID != "e3e87599-csm" || got.ProjectKey != "APPSUB" || got.Name != "ACP Partner Project - Subscription" {
		t.Errorf("CSM identity changed: %+v", got)
	}
	if got.EndDate == nil || !got.EndDate.Equal(time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)) ||
		got.StartDate == nil || !got.StartDate.Equal(time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("dates = %v / %v, want Salesforce's", got.StartDate, got.EndDate)
	}
	if got.Account == nil || got.Account.ID != "001P" || got.Account.Name != "ACP Test Partner Account" ||
		got.Account.IsPartner == nil || !*got.Account.IsPartner {
		t.Errorf("account = %+v, want the Salesforce account, Partner by classification", got.Account)
	}
}

// TestSalesforceDetails_IsPartnerFollowsClassification: only the
// classification "Partner" makes a partner account, as entity-service and
// legacy decide; Salesforce's own IsPartner checkbox is not read.
func TestSalesforceDetails_IsPartnerFollowsClassification(t *testing.T) {
	for classification, want := range map[string]bool{"Partner": true, " partner ": true, "Customer": false, "": false} {
		f := partnerFixture()
		c := f.customers["001P"]
		c.AccountClassification = sp(classification)
		f.customers["001P"] = c
		got, _, err := NewSalesforceDetails(f).forProject(context.Background(), csmPartnerProject())
		if err != nil {
			t.Fatalf("%q: %v", classification, err)
		}
		if *got.Account.IsPartner != want {
			t.Errorf("classification %q: isPartner = %v, want %v", classification, *got.Account.IsPartner, want)
		}
	}
}

func TestSalesforceDetails_FailsVisiblyWithoutTheProject(t *testing.T) {
	noSfID := csmPartnerProject()
	noSfID.SfID = sp("  ")
	if _, _, err := NewSalesforceDetails(partnerFixture()).forProject(context.Background(), noSfID); err == nil {
		t.Error("project with no Salesforce id: err = nil, want an error (never a guessed project)")
	}
	missing := csmPartnerProject()
	missing.SfID = sp("a0dmissing")
	if _, _, err := NewSalesforceDetails(partnerFixture()).forProject(context.Background(), missing); !errors.Is(err, salesentity.ErrNotFound) {
		t.Errorf("project not in Salesforce: err = %v, want ErrNotFound", err)
	}
}

// TestSalesforceDetails_ProjectWithoutAccount: no Account__c means no account
// people and no partner flags, the same as a CSM project with no account.
func TestSalesforceDetails_ProjectWithoutAccount(t *testing.T) {
	f := partnerFixture()
	p := f.projects["a0dP"]
	p.CustomerID = nil
	f.projects["a0dP"] = p
	got, _, err := NewSalesforceDetails(f).forProject(context.Background(), csmPartnerProject())
	if err != nil {
		t.Fatalf("forProject: %v", err)
	}
	if got.Account != nil {
		t.Errorf("account = %+v, want nil", got.Account)
	}
}

// TestSalesforceDetails_AccountAndPrimaryPartner: the account's people, and
// hasPrimaryPartner from the primary partner field (legacy's rule).
func TestSalesforceDetails_AccountAndPrimaryPartner(t *testing.T) {
	for _, tc := range []struct {
		name    string
		partner *string
		want    bool
	}{{"none", nil, false}, {"blank", sp(" "), false}, {"set", sp("001TechResell"), true}} {
		t.Run(tc.name, func(t *testing.T) {
			f := partnerFixture()
			c := f.customers["001P"]
			c.PrimaryPartnerAccountID = tc.partner
			f.customers["001P"] = c
			proj, reader, err := NewSalesforceDetails(f).forProject(context.Background(), csmPartnerProject())
			if err != nil {
				t.Fatalf("forProject: %v", err)
			}
			raw, err := reader.GetAccount(context.Background(), proj.accountID())
			if err != nil {
				t.Fatalf("GetAccount: %v", err)
			}
			var acc accountDTO
			if err := json.Unmarshal(raw, &acc); err != nil {
				t.Fatalf("parse: %v", err)
			}
			if acc.AccountManager == nil || acc.AccountManager.Name != "Kavith R" || stringValue(acc.AccountManager.Email) != "kavithr@wso2.com" ||
				acc.TechnicalOwner == nil || stringValue(acc.TechnicalOwner.Email) != "to@wso2.com" ||
				acc.RenewalAccountManager == nil || stringValue(acc.RenewalAccountManager.Email) != "abinash@wso2.com" {
				t.Errorf("account people = %+v", acc)
			}
			if acc.HasPrimaryPartner == nil || *acc.HasPrimaryPartner != tc.want {
				t.Errorf("hasPrimaryPartner = %v, want %v", acc.HasPrimaryPartner, tc.want)
			}
		})
	}
}

// TestSalesforceDetails_Contacts: the Business Contact label maps to the role
// the recipients rule reads, a deactivated membership is left out (as CSM's
// contact list does), and account contacts carry the primary flag.
func TestSalesforceDetails_Contacts(t *testing.T) {
	proj, reader, err := NewSalesforceDetails(partnerFixture()).forProject(context.Background(), csmPartnerProject())
	if err != nil {
		t.Fatalf("forProject: %v", err)
	}
	projectContacts, accountContacts, err := fetchContacts(context.Background(), reader, proj)
	if err != nil {
		t.Fatalf("fetchContacts: %v", err)
	}
	if len(projectContacts) != 2 {
		t.Fatalf("project contacts = %+v, want the two live memberships", projectContacts)
	}
	jane := projectContacts[0]
	if jane.Name != "Jane B" || jane.Email != "jane@acme.com" || len(jane.Roles) != 2 || jane.Roles[1] != "BUSINESS_CONTACT" {
		t.Errorf("business contact = %+v", jane)
	}
	if projectContacts[1].Email != "dev@acme.com" || projectContacts[1].Name != "dev@acme.com" {
		t.Errorf("membership without a contact record = %+v, want the invited email as name and email", projectContacts[1])
	}
	if accountContacts != nil {
		t.Errorf("account contacts read although a business contact exists: %+v", accountContacts)
	}

	// Without a business contact, the account's contacts are read.
	f := partnerFixture()
	f.members["a0dP"] = f.members["a0dP"][2:]
	proj, reader, _ = NewSalesforceDetails(f).forProject(context.Background(), csmPartnerProject())
	_, accountContacts, err = fetchContacts(context.Background(), reader, proj)
	if err != nil {
		t.Fatalf("fetchContacts: %v", err)
	}
	if len(accountContacts) != 2 || !accountContacts[0].IsPrimary || accountContacts[0].Email != "primary@acme.com" || accountContacts[1].IsPrimary {
		t.Errorf("account contacts = %+v", accountContacts)
	}
}

// TestSalesforceDetails_DueInvoices: links, opportunities and invoices come
// from Salesforce and go through the unchanged eligibility rules.
func TestSalesforceDetails_DueInvoices(t *testing.T) {
	proj, reader, err := NewSalesforceDetails(partnerFixture()).forProject(context.Background(), csmPartnerProject())
	if err != nil {
		t.Fatalf("forProject: %v", err)
	}
	got, err := resolveDueInvoices(context.Background(), reader, proj)
	if err != nil {
		t.Fatalf("resolveDueInvoices: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("due invoices = %+v, want one", got)
	}
	inv := got[0]
	if inv.ID != "a0IA" || inv.SfID != "a0IA" || inv.Number != "ACP Partner Invoice" || inv.Opportunity != "ACP Partner Opportunity" ||
		!inv.DueDate.Equal(time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)) || !inv.InvoiceDate.Equal(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)) ||
		inv.EULAVersionDecimal != 3.4 {
		t.Errorf("invoice = %+v", inv)
	}

	// A paid invoice and a not-yet-won opportunity are not due.
	f := partnerFixture()
	f.invoices["006A"][0].PaidDate = sp("2026-09-20")
	proj, reader, _ = NewSalesforceDetails(f).forProject(context.Background(), csmPartnerProject())
	if got, _ := resolveDueInvoices(context.Background(), reader, proj); len(got) != 0 {
		t.Errorf("paid invoice is due: %+v", got)
	}
	f = partnerFixture()
	f.opportunities["006A"] = salesentity.Opportunity{ID: "006A", StageName: sp("45 - Proposal"), EulaVersion: sp("EULA 3.4")}
	proj, reader, _ = NewSalesforceDetails(f).forProject(context.Background(), csmPartnerProject())
	if got, _ := resolveDueInvoices(context.Background(), reader, proj); len(got) != 0 {
		t.Errorf("invoice of a Proposal opportunity is due: %+v", got)
	}
}

// TestEulaVersionDecimal pins entity-service's rule (deriveEulaVersion): the
// first x.y in the text, 3.4 for an empty version, none otherwise.
func TestEulaVersionDecimal(t *testing.T) {
	for in, want := range map[string]string{"EULA 3.4": "3.4", "Version 3.3 (legacy)": "3.3", "": "3.4", "  ": "3.4", "Customer contract": ""} {
		got := eulaVersionDecimal(sp(in))
		if stringValue(got) != want || (want == "" && got != nil) {
			t.Errorf("eulaVersionDecimal(%q) = %v, want %q", in, stringValue(got), want)
		}
	}
	if got := eulaVersionDecimal(nil); stringValue(got) != "3.4" {
		t.Errorf("eulaVersionDecimal(nil) = %v, want 3.4", stringValue(got))
	}
}

// TestCSMDetails_PassesThrough: the CSM source leaves the project and reader
// unchanged.
func TestCSMDetails_PassesThrough(t *testing.T) {
	reader := &mockEntityReader{}
	proj := csmPartnerProject()
	got, gotReader, err := CSMDetails(reader).forProject(context.Background(), proj)
	if err != nil || got.ID != proj.ID || got.SfID != proj.SfID || gotReader != reader {
		t.Errorf("forProject = %+v, %v, %v", got, gotReader, err)
	}
}

func timePtr(t time.Time) *time.Time { return &t }

// csmListing is a CSM reader whose open-project list holds the given
// projects (as CSM has them) and which fails any per-project detail read, so
// a test proves those reads went to Salesforce.
func csmListing(t *testing.T, projects ...project) *mockEntityReader {
	t.Helper()
	page, err := json.Marshal(searchProjectsResponse{Projects: projects, Total: len(projects)})
	if err != nil {
		t.Fatal(err)
	}
	fail := func() ([]byte, error) {
		t.Error("a per-project detail was read from CSM")
		return nil, errors.New("not from CSM")
	}
	return &mockEntityReader{
		searchProjectsFn:                func(context.Context, []byte) ([]byte, error) { return page, nil },
		getProjectFn:                    func(_ context.Context, id string) ([]byte, error) { return json.Marshal(projects[0]) },
		getAccountFn:                    func(context.Context, string) ([]byte, error) { return fail() },
		searchAccountContactsFn:         func(context.Context, string, []byte) ([]byte, error) { return fail() },
		searchProjectContactsFn:         func(context.Context, string, []byte) ([]byte, error) { return fail() },
		searchProjectOpportunityLinksFn: func(context.Context, []byte) ([]byte, error) { return fail() },
		searchInvoicesFn:                func(context.Context, []byte) ([]byte, error) { return fail() },
		getOpportunityFn:                func(context.Context, string) ([]byte, error) { return fail() },
	}
}

// TestRun_SalesforceDetailsDecideAndCSMIsWritten: the partner project ended
// on 13 Oct in Salesforce, though CSM's copy says next year. The run suspends
// it from Salesforce's date, emails Salesforce's account people and business
// contact, and writes the notice and the suspension to the CSM project id.
// The invoice cascade is off for a partner account with no primary partner.
func TestRun_SalesforceDetailsDecideAndCSMIsWritten(t *testing.T) {
	proj := csmPartnerProject()
	proj.EndDate = timePtr(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC))
	open := "Open"
	proj.EndDateClosureState, proj.InvoiceDueDateClosureState, proj.ClosureState = &open, &open, &open
	proj.SuspensionProcessState = json.RawMessage(`{"based_on_subscription_end_date":{"event_type":"7_days_notice"}}`)
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}
	now := time.Date(2026, 10, 14, 9, 30, 0, 0, time.UTC)

	for _, scoped := range []string{"", proj.ID} {
		updater.calls, ntf.sent = nil, nil
		result, err := Run(context.Background(), csmListing(t, proj), NewSalesforceDetails(partnerFixture()), updater, ntf, now, scoped, nil)
		if err != nil || len(result.Failures) != 0 || result.ProjectsEvaluated != 1 {
			t.Fatalf("scoped=%q: Run = %+v, %v", scoped, result, err)
		}
		if len(ntf.sent) != 2 {
			t.Fatalf("scoped=%q: notices = %d, want the internal and the customer suspension notice", scoped, len(ntf.sent))
		}
		internal := ntf.sent[0].Recipients
		if internal.AccountOwner.Email != "kavithr@wso2.com" || internal.RenewalManager.Email != "abinash@wso2.com" || internal.TechnicalOwner.Email != "to@wso2.com" {
			t.Errorf("internal recipients = %+v, want Salesforce's account people", internal)
		}
		customer := ntf.sent[1].Recipients.Customers
		if len(customer) != 1 || customer[0].Email != "jane@acme.com" {
			t.Errorf("customer recipients = %+v, want the Salesforce business contact", customer)
		}
		if ntf.sent[0].ProjectSfID != "a0dP" || ntf.sent[0].ProjectID != proj.ID {
			t.Errorf("notice ids = %s / %s", ntf.sent[0].ProjectID, ntf.sent[0].ProjectSfID)
		}
		suspended := false
		for _, c := range updater.calls {
			if c.id != proj.ID {
				t.Errorf("write to %q, want the CSM project id %q", c.id, proj.ID)
			}
			var body map[string]any
			_ = json.Unmarshal(c.body, &body)
			if body["endDateClosureState"] == "Closed" {
				suspended = true
			}
			if _, ok := body["invoiceDueDateClosureState"]; ok {
				t.Errorf("invoice suspension written for a partner account with no primary partner: %s", c.body)
			}
		}
		if !suspended {
			t.Errorf("scoped=%q: writes = %+v, want endDateClosureState Closed", scoped, updater.calls)
		}
	}
}

// TestRun_DetailFailureIsAProjectFailure: a project Salesforce does not know
// (or with no sfId) is a visible failure for that project only; the run goes
// on to the next one.
func TestRun_DetailFailureIsAProjectFailure(t *testing.T) {
	missing := csmPartnerProject()
	missing.ID, missing.SfID = "csm-missing", sp("a0dmissing")
	ok := csmPartnerProject()
	ok.EndDate = nil
	f := partnerFixture()
	p := f.projects["a0dP"]
	p.EndDate = nil
	f.projects["a0dP"] = p

	result, err := Run(context.Background(), csmListing(t, missing, ok), NewSalesforceDetails(f), &mockProjectUpdater{}, &mockNotifier{}, time.Now(), "", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.ProjectsEvaluated != 2 || len(result.Failures) != 1 || result.Failures[0].ProjectID != "csm-missing" {
		t.Errorf("result = %+v, want two evaluated and one failure for csm-missing", result)
	}
}
