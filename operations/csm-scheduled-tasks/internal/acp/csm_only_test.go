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

package acp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp/entity"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp/salesentity"
)

// fakeSalesEntity records every call: reads and the State__c write apart.
type fakeSalesEntity struct {
	mu     sync.Mutex
	reads  []string
	writes []string
}

func (f *fakeSalesEntity) read(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads = append(f.reads, name)
}

func (f *fakeSalesEntity) GetProject(_ context.Context, id string) (salesentity.Project, error) {
	f.read("GetProject")
	end := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	return salesentity.Project{ID: id, StartDate: strPtr("2025-01-01"), EndDate: &end, CustomerID: strPtr("001X")}, nil
}
func (f *fakeSalesEntity) GetCustomer(context.Context, string) (salesentity.Customer, error) {
	f.read("GetCustomer")
	return salesentity.Customer{ID: "001X", Status: strPtr("Customer"), AccountClassification: strPtr("Enterprise")}, nil
}
func (f *fakeSalesEntity) ProjectContacts(context.Context, string) ([]salesentity.ProjectContact, error) {
	f.read("ProjectContacts")
	return nil, nil
}
func (f *fakeSalesEntity) AccountContacts(context.Context, string) ([]salesentity.Contact, error) {
	f.read("AccountContacts")
	return nil, nil
}
func (f *fakeSalesEntity) LinkedOpportunities(context.Context, string) ([]salesentity.LinkedOpportunity, error) {
	f.read("LinkedOpportunities")
	return nil, nil
}
func (f *fakeSalesEntity) Opportunities(context.Context, []string) ([]salesentity.Opportunity, error) {
	f.read("Opportunities")
	return nil, nil
}
func (f *fakeSalesEntity) Invoices(context.Context, string) ([]salesentity.Invoice, error) {
	f.read("Invoices")
	return nil, nil
}
func (f *fakeSalesEntity) UpdateProjectStatus(_ context.Context, id, status string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writes = append(f.writes, id+":"+status)
	return nil
}

func strPtr(s string) *string { return &s }

type sentMail struct{ to []string }

type fakeEmail struct {
	mu   sync.Mutex
	sent []sentMail
}

func (f *fakeEmail) SendEmail(_ context.Context, to, _ []string, _, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, sentMail{to: to})
	return nil
}

// csmEntityService is an entity-service holding one Customer project past its
// end date, and the PATCHes it received.
func csmEntityService(t *testing.T, customer bool) (*entity.Client, *[]string) {
	t.Helper()
	var mu sync.Mutex
	var patches []string
	end := time.Now().AddDate(0, 0, -1).Format(time.RFC3339)
	project := `{"id":"p1","name":"Example Project","key":"EXAMPLE","sfId":"a0dP","subscriptionType":"subscription","suspensionProcessOverridden":false,` +
		`"startDate":"2025-01-01T00:00:00Z","endDate":"` + end + `","closureState":"Open","endDateClosureState":"Open","invoiceDueDateClosureState":"Open",` +
		`"account":{"id":"acc1","name":"Example Corp","isPartner":false}}`
	accounts := `{"accounts":[],"total":0,"hasMore":false}`
	if customer {
		accounts = `{"accounts":[{"id":"acc1"}],"total":1,"hasMore":false}`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/token":
			_, _ = w.Write([]byte(`{"access_token":"t","token_type":"bearer","expires_in":3600}`))
		case r.Method == http.MethodPost && r.URL.Path == "/projects/search":
			_, _ = w.Write([]byte(`{"projects":[` + project + `],"total":1,"hasMore":false}`))
		case r.Method == http.MethodGet && r.URL.Path == "/projects/p1":
			_, _ = w.Write([]byte(project))
		case r.Method == http.MethodPatch && r.URL.Path == "/projects/p1":
			b, _ := io.ReadAll(r.Body)
			mu.Lock()
			patches = append(patches, string(b))
			mu.Unlock()
			_, _ = w.Write([]byte(`{}`))
		case r.URL.Path == "/accounts/search":
			_, _ = w.Write([]byte(accounts))
		case r.URL.Path == "/accounts/acc1":
			_, _ = w.Write([]byte(`{"accountManager":{"id":"u1","name":"Jane Doe","email":"jane.doe@wso2.com"},"hasPrimaryPartner":false}`))
		case r.URL.Path == "/projects/p1/contacts/search":
			_, _ = w.Write([]byte(`{"contacts":[{"name":"John Roe","email":"john.roe@wso2.com","roles":["BUSINESS_CONTACT"]}],"total":1}`))
		case r.URL.Path == "/accounts/acc1/contacts/search":
			_, _ = w.Write([]byte(`{"contacts":[],"total":0}`))
		case r.URL.Path == "/project-opportunity-links/search":
			_, _ = w.Write([]byte(`{"links":[],"total":0}`))
		default:
			t.Errorf("unexpected entity-service call %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	c, err := entity.NewClient(entity.Config{BaseURL: srv.URL, TokenURL: srv.URL + "/token", ClientID: "id", ClientSecret: "secret", Scopes: []string{"s"}})
	if err != nil {
		t.Fatal(err)
	}
	return c, &patches
}

func realRunConfig() Config {
	return Config{EmailEnabled: true, RunOnWeekends: true, Screening: true, FullSweepWeekday: time.Monday, Repair: true, RepairMaxEvents: 10, AllowNonWSO2Recipients: false}
}

// With reads off, a whole real run makes no sales-entity-service read, not for
// the project, the customer, the contacts, the opportunities or the invoices;
// the only call is the State__c write of the suspension.
func TestRun_NoSalesEntityReadsMakesNoRead(t *testing.T) {
	csm, patches := csmEntityService(t, true)
	sales, mail := &fakeSalesEntity{}, &fakeEmail{}
	cfg := realRunConfig()
	cfg.NoSalesEntityReads = true
	if err := Run(Clients{CSM: csm, SalesEntity: sales, Email: mail}, cfg)(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(sales.reads) != 0 {
		t.Errorf("sales-entity-service reads = %v, want none", sales.reads)
	}
	if len(sales.writes) != 1 || sales.writes[0] != "a0dP:Suspended" {
		t.Errorf("sales-entity-service writes = %v, want the one State__c PATCH", sales.writes)
	}
	if len(mail.sent) == 0 {
		t.Error("no notice sent: the decision should have come from CSM's copy")
	}
	joined := strings.Join(*patches, " ")
	if !strings.Contains(joined, `"endDateClosureState":"Closed"`) {
		t.Errorf("CSM writes = %v, want the suspension recorded", *patches)
	}
}

// The control: with reads on the same run does read from sales-entity-service,
// so the test above would notice a read slipping through.
func TestRun_SalesEntityReadsOnDoesRead(t *testing.T) {
	csm, _ := csmEntityService(t, true)
	sales := &fakeSalesEntity{}
	cfg := realRunConfig()
	cfg.Screening, cfg.Repair = false, false
	_ = Run(Clients{CSM: csm, SalesEntity: sales, Email: &fakeEmail{}}, cfg)(context.Background())
	if len(sales.reads) == 0 {
		t.Error("no sales-entity-service read with reads on")
	}
}

// An account that is not in the Customer list is not evaluated: nothing is
// sent or suspended.
func TestRun_NoSalesEntityReadsSkipsANonCustomer(t *testing.T) {
	csm, patches := csmEntityService(t, false)
	sales, mail := &fakeSalesEntity{}, &fakeEmail{}
	cfg := realRunConfig()
	cfg.NoSalesEntityReads = true
	if err := Run(Clients{CSM: csm, SalesEntity: sales, Email: mail}, cfg)(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(mail.sent) != 0 || len(sales.writes) != 0 || len(*patches) != 0 {
		t.Errorf("sent=%d sfWrites=%v csmWrites=%v, want nothing for a non-Customer account", len(mail.sent), sales.writes, *patches)
	}
}

// A dry run in this mode needs no sales-entity-service client at all; a real
// one refuses to run without it (it has the State__c write to make).
func TestRun_NoSalesEntityReadsClientRequirements(t *testing.T) {
	csm, _ := csmEntityService(t, true)
	cfg := realRunConfig()
	cfg.NoSalesEntityReads, cfg.DryRun, cfg.EmailEnabled = true, true, false
	if err := Run(Clients{CSM: csm}, cfg)(context.Background()); err != nil {
		t.Errorf("dry run without a client: %v, want it to run", err)
	}
	real := realRunConfig()
	real.NoSalesEntityReads = true
	if err := Run(Clients{CSM: csm, Email: &fakeEmail{}}, real)(context.Background()); err == nil || !strings.Contains(err.Error(), "State__c") {
		t.Errorf("real run without a client: err = %v, want a refusal", err)
	}
	reads := realRunConfig() // reads on, no client
	if err := Run(Clients{CSM: csm, Email: &fakeEmail{}}, reads)(context.Background()); err == nil || !strings.Contains(err.Error(), "no sales-entity-service client") {
		t.Errorf("reads on without a client: err = %v, want a refusal", err)
	}
}
