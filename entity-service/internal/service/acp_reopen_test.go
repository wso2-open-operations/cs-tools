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
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesentity"
)

const (
	acpTestSfID      = "a0dE200000CZ9ZNIA1"
	acpTestProjectID = "e3e87599-1bc7-6650-182c-0dc5604bcb68"
	acpOtherSfID     = "a0dE200000CZ9ZOIA1"
)

var acpTestNow = time.Date(2026, 10, 29, 9, 30, 0, 0, time.UTC)

// acpFakeRepo is ACPReopenRepository over one in-memory project. Reopen
// mirrors the real transaction: plan on the current row, write, recompute,
// beforeCommit, and nothing kept when beforeCommit fails.
type acpFakeRepo struct {
	projects map[string]*domain.ACPReopenProject // by sf id
	// underLock replaces the row Reopen hands plan, to test the re-check.
	underLock *domain.ACPReopenProject
	reopens   int
	getErr    error
	// notices are the "Back to Open state" notices queued with a reopen.
	notices []events.ProjectReactivatedPayload
	// invoiceTrackChecks counts AnyACPSuspendedInvoiceTrack calls.
	invoiceTrackChecks int
}

func (r *acpFakeRepo) GetProjectBySfID(_ context.Context, sfID string) (*domain.ACPReopenProject, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	p, ok := r.projects[sfID]
	if !ok {
		return nil, nil
	}
	cp := *p
	return &cp, nil
}

func (r *acpFakeRepo) Reopen(ctx context.Context, projectID string,
	plan func(domain.ACPReopenProject) (domain.ACPReopenWrite, bool, error),
	beforeCommit func(context.Context, domain.ACPReopenOutcome) error,
	notice func(domain.ACPReopenOutcome) (json.RawMessage, error),
) (domain.ACPReopenOutcome, bool, error) {
	r.reopens++
	var stored *domain.ACPReopenProject
	for _, p := range r.projects {
		if p.ID == projectID {
			stored = p
		}
	}
	if stored == nil {
		return domain.ACPReopenOutcome{}, false, nil
	}
	current := *stored
	if r.underLock != nil {
		current = *r.underLock
	}
	w, ok, err := plan(current)
	if err != nil || !ok {
		return domain.ACPReopenOutcome{}, false, err
	}
	next := current
	if w.ReopenEndDate {
		next.EndDateClosureState = acpStr("OPEN")
	}
	if w.ReopenInvoice {
		next.InvoiceDueDateClosureState = acpStr("OPEN")
	}
	next.SuspensionProcessState = w.SuspensionProcessState
	open := acpLabel(next.EndDateClosureState) != "CLOSED" && acpLabel(next.EndDateClosureState) != "SUSPENDED" &&
		!strings.HasPrefix(acpLabel(next.InvoiceDueDateClosureState), "SUSPENDED") &&
		acpLabel(next.ComplianceViolationClosureState) != "SUSPENDED"
	state := "SUSPENDED"
	if open {
		state = "OPEN"
	}
	next.ClosureState = &state
	outcome := domain.ACPReopenOutcome{ClosureState: &state, Open: open}
	if beforeCommit != nil {
		if err := beforeCommit(ctx, outcome); err != nil {
			return domain.ACPReopenOutcome{}, false, err
		}
	}
	if notice != nil {
		raw, err := notice(outcome)
		if err != nil {
			return domain.ACPReopenOutcome{}, false, err
		}
		if raw != nil {
			var pl events.ProjectReactivatedPayload
			if err := json.Unmarshal(raw, &pl); err != nil {
				return domain.ACPReopenOutcome{}, false, err
			}
			r.notices = append(r.notices, pl)
		}
	}
	*stored = next
	return outcome, true, nil
}

func (r *acpFakeRepo) AnyACPSuspendedInvoiceTrack(context.Context) (bool, error) {
	r.invoiceTrackChecks++
	for _, p := range r.projects {
		ev, _ := acpSectionEventType(p.SuspensionProcessState, acpInvoiceSection)
		if strings.HasPrefix(acpLabel(p.InvoiceDueDateClosureState), "SUSPENDED") && ev == acpSuspendEventType {
			return true, nil
		}
	}
	return false, nil
}

func (r *acpFakeRepo) ListACPNotedProjects(_ context.Context, afterID string, limit int) ([]domain.ACPReopenProject, error) {
	var out []domain.ACPReopenProject
	for _, p := range r.projects {
		end, _ := acpSectionEventType(p.SuspensionProcessState, acpEndDateSection)
		inv, _ := acpSectionEventType(p.SuspensionProcessState, acpInvoiceSection)
		if p.ID > afterID && p.SfID != "" && (end == acpSuspendEventType || inv == acpSuspendEventType) {
			out = append(out, *p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func acpLabel(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

// acpFakeSF is ACPReopenSalesforce over fixed records.
type acpFakeSF struct {
	project       salesentity.Project
	customer      salesentity.Customer
	invoice       salesentity.Invoice
	linksByProj   []salesentity.LinkedOpportunity
	linksByOpp    map[string][]salesentity.LinkedOpportunity
	opportunities map[string]salesentity.Opportunity
	invoices      map[string][]salesentity.Invoice

	getProjectErr error
	statusErr     error
	statusCalls   []string
	calls         int
}

func (f *acpFakeSF) GetProject(_ context.Context, id string) (salesentity.Project, error) {
	f.calls++
	if f.getProjectErr != nil {
		return salesentity.Project{}, f.getProjectErr
	}
	p := f.project
	p.ID = id
	return p, nil
}
func (f *acpFakeSF) GetCustomer(context.Context, string) (salesentity.Customer, error) {
	f.calls++
	return f.customer, nil
}
func (f *acpFakeSF) GetInvoice(context.Context, string) (salesentity.Invoice, error) {
	f.calls++
	return f.invoice, nil
}
func (f *acpFakeSF) GetOpportunity(_ context.Context, id string) (salesentity.Opportunity, error) {
	f.calls++
	return f.opportunities[id], nil
}
func (f *acpFakeSF) LinkedOpportunitiesByProject(context.Context, string) ([]salesentity.LinkedOpportunity, error) {
	f.calls++
	return f.linksByProj, nil
}
func (f *acpFakeSF) LinkedOpportunitiesByOpportunity(_ context.Context, id string) ([]salesentity.LinkedOpportunity, error) {
	f.calls++
	return f.linksByOpp[id], nil
}
func (f *acpFakeSF) InvoicesByOpportunity(_ context.Context, id string) ([]salesentity.Invoice, error) {
	f.calls++
	return f.invoices[id], nil
}
func (f *acpFakeSF) UpdateProjectStatus(_ context.Context, id, status string) error {
	f.calls++
	if f.statusErr != nil {
		return f.statusErr
	}
	f.statusCalls = append(f.statusCalls, id+"="+status)
	return nil
}

type acpFakePublisher struct {
	types    []events.Type
	ids      []string
	payloads []events.ProjectReactivatedPayload
}

func (p *acpFakePublisher) Publish(_ context.Context, t events.Type, id string, payload json.RawMessage) error {
	var pl events.ProjectReactivatedPayload
	if err := json.Unmarshal(payload, &pl); err != nil {
		return err
	}
	p.types = append(p.types, t)
	p.ids = append(p.ids, id)
	p.payloads = append(p.payloads, pl)
	return nil
}
func (p *acpFakePublisher) Close() {}

// acpSuspendedProject is the test project as ACP leaves it after suspending
// it on the end-date track.
func acpSuspendedProject() *domain.ACPReopenProject {
	return &domain.ACPReopenProject{
		ID: acpTestProjectID, SfID: acpTestSfID, Name: "[S] ACP Test Project", Key: "APPSUB",
		EndDateClosureState:        acpStr("CLOSED"),
		InvoiceDueDateClosureState: acpStr("OPEN"),
		ClosureState:               acpStr("SUSPENDED"),
		SuspensionProcessState: json.RawMessage(`{"based_on_subscription_end_date":{"event_type":"suspend","actionSendEmailNotification":"SUCCESSFUL"},` +
			`"based_on_due_invoices":{"event_type":"open"}}`),
	}
}

func acpRenewedSF() *acpFakeSF {
	return &acpFakeSF{
		project: salesentity.Project{
			Name: acpStr("[S] ACP Test Project"), Key: acpStr("APPSUB"),
			StartDate: acpStr("2026-10-28"), EndDate: acpStr("2027-10-27"), CustomerID: acpStr("001ACC"),
		},
		customer: salesentity.Customer{
			ID:             "001ACC",
			Owner:          &salesentity.CustomerUser{Name: acpStr("Alex Owner"), Email: acpStr("owner@example.com")},
			RenewalManager: &salesentity.CustomerUser{Email: acpStr("OWNER@example.com")}, // same person: listed once
			TechnicalOwner: acpStr("tech.owner@example.com"),
		},
	}
}

// newACPTestService builds the reopen check with notices on when pub is set
// (pub itself is not called: the notice is queued in the repository).
func newACPTestService(repo *acpFakeRepo, sf *acpFakeSF, pub EventPublisherService, mod func(*ACPReopen)) *salesforceEventService {
	r := ACPReopen{Projects: repo, SalesEntity: sf, Notify: pub != nil, InternalDomain: "@example.com",
		Now: func() time.Time { return acpTestNow }}
	if mod != nil {
		mod(&r)
	}
	svc := NewSalesforceEventService(nil, nil, SalesforceIngestSupport{}).(*salesforceEventService)
	return WithACPReopen(svc, r).(*salesforceEventService)
}

func acpProjectEvent(eventType string) domain.SalesforceEventRequest {
	return domain.SalesforceEventRequest{EventType: eventType, Entity: domain.SalesforceEntityProject, ReferenceID: acpTestSfID}
}

func TestACPReopen_RenewalReopensTheProject(t *testing.T) {
	repo := &acpFakeRepo{projects: map[string]*domain.ACPReopenProject{acpTestSfID: acpSuspendedProject()}}
	sf := acpRenewedSF()
	pub := &acpFakePublisher{}
	svc := newACPTestService(repo, sf, pub, nil)

	if err := svc.HandleEvent(context.Background(), acpProjectEvent(domain.SalesforceEventUpdated)); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	stored := repo.projects[acpTestSfID]
	if acpLabel(stored.EndDateClosureState) != "OPEN" || acpLabel(stored.ClosureState) != "OPEN" {
		t.Errorf("project = end %s overall %s, want OPEN/OPEN", acpLabel(stored.EndDateClosureState), acpLabel(stored.ClosureState))
	}
	if ev, _ := acpSectionEventType(stored.SuspensionProcessState, acpEndDateSection); ev != "open" {
		t.Errorf("end-date section event_type = %q, want open", ev)
	}
	if !reflect.DeepEqual(sf.statusCalls, []string{acpTestSfID + "=Open"}) {
		t.Errorf("Salesforce status writes = %v", sf.statusCalls)
	}
	if len(repo.notices) != 1 || repo.notices[0].ProjectID != acpTestProjectID {
		t.Fatalf("queued notices = %+v", repo.notices)
	}
	got := repo.notices[0]
	want := events.ProjectReactivatedPayload{
		ProjectID: acpTestProjectID, ProjectSfID: acpTestSfID, ProjectName: "[S] ACP Test Project", ProjectKey: "APPSUB",
		AccountOwnerName: "Alex Owner", AccountOwnerEmail: "owner@example.com",
		StartDate: "2026-10-28", EndDate: "2027-10-27",
		Reasons:       []string{events.ProjectReactivatedReasonRenewal},
		Recipients:    []string{"owner@example.com", "tech.owner@example.com"},
		ReactivatedOn: "2026-10-29T09:30:00Z",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("payload\n got %+v\nwant %+v", got, want)
	}
}

func TestACPReopen_EndDateStillPastLeavesItSuspended(t *testing.T) {
	repo := &acpFakeRepo{projects: map[string]*domain.ACPReopenProject{acpTestSfID: acpSuspendedProject()}}
	sf := acpRenewedSF()
	sf.project.EndDate = acpStr("2026-10-29") // today: ACP would suspend it again
	pub := &acpFakePublisher{}
	if err := newACPTestService(repo, sf, pub, nil).HandleEvent(context.Background(), acpProjectEvent(domain.SalesforceEventUpdated)); err != nil {
		t.Fatal(err)
	}
	if repo.reopens != 0 || len(sf.statusCalls) != 0 || len(repo.notices) != 0 {
		t.Errorf("reopens=%d status=%v published=%d, want nothing", repo.reopens, sf.statusCalls, len(repo.notices))
	}
}

func TestACPReopen_ManualSuspensionIsLeftAlone(t *testing.T) {
	p := acpSuspendedProject()
	// Closed by hand: no "suspend" record from ACP.
	p.SuspensionProcessState = json.RawMessage(`{"based_on_subscription_end_date":{"event_type":"7_days_notice"}}`)
	repo := &acpFakeRepo{projects: map[string]*domain.ACPReopenProject{acpTestSfID: p}}
	sf := acpRenewedSF()
	if err := newACPTestService(repo, sf, nil, nil).HandleEvent(context.Background(), acpProjectEvent(domain.SalesforceEventUpdated)); err != nil {
		t.Fatal(err)
	}
	if repo.reopens != 0 || sf.calls != 0 {
		t.Errorf("reopens=%d salesforce calls=%d, want none", repo.reopens, sf.calls)
	}
}

func TestACPReopen_NotSuspendedMakesNoSalesforceCall(t *testing.T) {
	p := acpSuspendedProject()
	p.EndDateClosureState = acpStr("OPEN")
	repo := &acpFakeRepo{projects: map[string]*domain.ACPReopenProject{acpTestSfID: p}}
	sf := acpRenewedSF()
	if err := newACPTestService(repo, sf, nil, nil).HandleEvent(context.Background(), acpProjectEvent(domain.SalesforceEventUpdated)); err != nil {
		t.Fatal(err)
	}
	if sf.calls != 0 || repo.reopens != 0 {
		t.Errorf("salesforce calls=%d reopens=%d", sf.calls, repo.reopens)
	}
}

func TestACPReopen_DryRunWritesNothing(t *testing.T) {
	repo := &acpFakeRepo{projects: map[string]*domain.ACPReopenProject{acpTestSfID: acpSuspendedProject()}}
	sf := acpRenewedSF()
	pub := &acpFakePublisher{}
	svc := newACPTestService(repo, sf, pub, func(r *ACPReopen) { r.DryRun = true })
	if err := svc.HandleEvent(context.Background(), acpProjectEvent(domain.SalesforceEventUpdated)); err != nil {
		t.Fatal(err)
	}
	if repo.reopens != 0 || len(sf.statusCalls) != 0 || len(repo.notices) != 0 {
		t.Errorf("dry run wrote: reopens=%d status=%v published=%d", repo.reopens, sf.statusCalls, len(repo.notices))
	}
}

func TestACPReopen_TestProjectFilter(t *testing.T) {
	repo := &acpFakeRepo{projects: map[string]*domain.ACPReopenProject{acpTestSfID: acpSuspendedProject()}}
	sf := acpRenewedSF()
	svc := newACPTestService(repo, sf, nil, func(r *ACPReopen) { r.TestProjectSfID = acpOtherSfID })
	if err := svc.HandleEvent(context.Background(), acpProjectEvent(domain.SalesforceEventUpdated)); err != nil {
		t.Fatal(err)
	}
	if repo.reopens != 0 || sf.calls != 0 {
		t.Errorf("a project other than the test project was checked: reopens=%d calls=%d", repo.reopens, sf.calls)
	}
	// The 15-character form of the test project's own id still matches.
	svc = newACPTestService(repo, sf, nil, func(r *ACPReopen) { r.TestProjectSfID = acpTestSfID[:15] })
	if err := svc.HandleEvent(context.Background(), acpProjectEvent(domain.SalesforceEventUpdated)); err != nil {
		t.Fatal(err)
	}
	if repo.reopens != 1 {
		t.Errorf("the test project was not checked: reopens=%d", repo.reopens)
	}
}

func TestACPReopen_IgnoresDeletedEventsAndAnUnknownProject(t *testing.T) {
	repo := &acpFakeRepo{projects: map[string]*domain.ACPReopenProject{}}
	sf := acpRenewedSF()
	svc := newACPTestService(repo, sf, nil, nil)
	if err := svc.HandleEvent(context.Background(), acpProjectEvent(domain.SalesforceEventDeleted)); err != nil {
		t.Fatal(err)
	}
	if err := svc.HandleEvent(context.Background(), acpProjectEvent(domain.SalesforceEventUpdated)); err != nil {
		t.Fatal(err)
	}
	if sf.calls != 0 || repo.reopens != 0 {
		t.Errorf("calls=%d reopens=%d", sf.calls, repo.reopens)
	}
}

func TestACPReopen_SalesforceRefusalRollsBackAndIsRetried(t *testing.T) {
	repo := &acpFakeRepo{projects: map[string]*domain.ACPReopenProject{acpTestSfID: acpSuspendedProject()}}
	sf := acpRenewedSF()
	sf.statusErr = errors.New("sales-entity-service unavailable")
	pub := &acpFakePublisher{}
	svc := newACPTestService(repo, sf, pub, nil)
	// The failure is logged and left to the daily check: the event itself,
	// whose ingest succeeded, is not failed.
	if err := svc.HandleEvent(context.Background(), acpProjectEvent(domain.SalesforceEventUpdated)); err != nil {
		t.Fatalf("a reopen failure failed the event: %v", err)
	}
	if acpLabel(repo.projects[acpTestSfID].EndDateClosureState) != "CLOSED" || len(repo.notices) != 0 {
		t.Errorf("the reopen was kept: end=%s published=%d", acpLabel(repo.projects[acpTestSfID].EndDateClosureState), len(repo.notices))
	}
	// The retry (Salesforce back) reopens it.
	sf.statusErr = nil
	if err := svc.HandleEvent(context.Background(), acpProjectEvent(domain.SalesforceEventUpdated)); err != nil {
		t.Fatal(err)
	}
	if acpLabel(repo.projects[acpTestSfID].EndDateClosureState) != "OPEN" || len(repo.notices) != 1 {
		t.Errorf("retry: end=%s published=%d", acpLabel(repo.projects[acpTestSfID].EndDateClosureState), len(repo.notices))
	}
	// A duplicate event afterwards does nothing more.
	if err := svc.HandleEvent(context.Background(), acpProjectEvent(domain.SalesforceEventUpdated)); err != nil {
		t.Fatal(err)
	}
	if len(repo.notices) != 1 || len(sf.statusCalls) != 1 {
		t.Errorf("duplicate event: published=%d status=%v", len(repo.notices), sf.statusCalls)
	}
}

func TestACPReopen_ChangedUnderTheLockIsNotReopened(t *testing.T) {
	repo := &acpFakeRepo{projects: map[string]*domain.ACPReopenProject{acpTestSfID: acpSuspendedProject()}}
	moved := acpSuspendedProject()
	moved.EndDateClosureState = acpStr("OPEN") // somebody reopened it meanwhile
	repo.underLock = moved
	sf := acpRenewedSF()
	pub := &acpFakePublisher{}
	if err := newACPTestService(repo, sf, pub, nil).HandleEvent(context.Background(), acpProjectEvent(domain.SalesforceEventUpdated)); err != nil {
		t.Fatal(err)
	}
	if len(sf.statusCalls) != 0 || len(repo.notices) != 0 {
		t.Errorf("status=%v published=%d, want nothing", sf.statusCalls, len(repo.notices))
	}
}

func TestACPReopen_OneTrackFixedKeepsTheProjectSuspended(t *testing.T) {
	p := acpSuspendedProject()
	p.InvoiceDueDateClosureState = acpStr("SUSPENDED")
	p.SuspensionProcessState = json.RawMessage(`{"based_on_subscription_end_date":{"event_type":"suspend"},"based_on_due_invoices":{"event_type":"suspend"}}`)
	repo := &acpFakeRepo{projects: map[string]*domain.ACPReopenProject{acpTestSfID: p}}
	sf := acpRenewedSF()
	sf.project.StartDate = acpStr("2026-01-01")
	// One Closed Won opportunity with an invoice that is still overdue.
	sf.linksByProj = []salesentity.LinkedOpportunity{{ID: "L1", OpportunityID: acpStr("006A")}}
	sf.opportunities = map[string]salesentity.Opportunity{"006A": {ID: "006A", StageName: acpStr("50 - Closed Won"),
		EulaVersion: salesentity.OptionalString{Present: true, Value: acpStr("EULA 3.4")}}}
	sf.invoices = map[string][]salesentity.Invoice{"006A": {{ID: acpStr("INV1"), InvoiceDate: acpStr("2026-08-01"), DueDate: acpStr("2026-09-01")}}}
	pub := &acpFakePublisher{}
	if err := newACPTestService(repo, sf, pub, nil).HandleEvent(context.Background(), acpProjectEvent(domain.SalesforceEventUpdated)); err != nil {
		t.Fatal(err)
	}
	stored := repo.projects[acpTestSfID]
	if acpLabel(stored.EndDateClosureState) != "OPEN" || acpLabel(stored.InvoiceDueDateClosureState) != "SUSPENDED" {
		t.Errorf("tracks = end %s invoice %s, want OPEN / SUSPENDED", acpLabel(stored.EndDateClosureState), acpLabel(stored.InvoiceDueDateClosureState))
	}
	if acpLabel(stored.ClosureState) != "SUSPENDED" || len(sf.statusCalls) != 0 || len(repo.notices) != 0 {
		t.Errorf("overall=%s status=%v published=%d: the project must stay suspended with no email",
			acpLabel(stored.ClosureState), sf.statusCalls, len(repo.notices))
	}
}

func TestACPReopen_InvoicePaidReopensForEveryLinkedProject(t *testing.T) {
	p := acpSuspendedProject()
	p.EndDateClosureState = acpStr("OPEN")
	p.InvoiceDueDateClosureState = acpStr("SUSPENDED")
	p.SuspensionProcessState = json.RawMessage(`{"based_on_subscription_end_date":{"event_type":"open"},"based_on_due_invoices":{"event_type":"suspend"}}`)
	repo := &acpFakeRepo{projects: map[string]*domain.ACPReopenProject{acpTestSfID: p}}
	sf := acpRenewedSF()
	sf.invoice = salesentity.Invoice{ID: acpStr("INV1"), OpportunityID: acpStr("006A")}
	// The opportunity is linked to the test project and to one CSM does not have.
	sf.linksByOpp = map[string][]salesentity.LinkedOpportunity{"006A": {
		{ID: "L1", ProjectID: acpStr(acpTestSfID)}, {ID: "L2", ProjectID: acpStr(acpOtherSfID)}, {ID: "L3", ProjectID: acpStr(acpTestSfID)},
	}}
	sf.linksByProj = []salesentity.LinkedOpportunity{{ID: "L1", OpportunityID: acpStr("006A")}}
	sf.opportunities = map[string]salesentity.Opportunity{"006A": {ID: "006A", StageName: acpStr("50 - Closed Won"),
		EulaVersion: salesentity.OptionalString{Present: true, Value: acpStr("EULA 3.4")}}}
	sf.invoices = map[string][]salesentity.Invoice{"006A": {{ID: acpStr("INV1"), InvoiceDate: acpStr("2026-08-01"),
		DueDate: acpStr("2026-09-01"), PaidDate: acpStr("2026-10-28")}}}
	pub := &acpFakePublisher{}
	svc := newACPTestService(repo, sf, pub, nil)
	req := domain.SalesforceEventRequest{EventType: domain.SalesforceEventUpdated, Entity: domain.SalesforceEntityInvoice, ReferenceID: "INV1"}
	if err := svc.HandleEvent(context.Background(), req); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if repo.reopens != 1 {
		t.Errorf("reopens = %d, want 1 (the test project, once)", repo.reopens)
	}
	if acpLabel(repo.projects[acpTestSfID].InvoiceDueDateClosureState) != "OPEN" || len(repo.notices) != 1 {
		t.Fatalf("invoice track = %s published = %d", acpLabel(repo.projects[acpTestSfID].InvoiceDueDateClosureState), len(repo.notices))
	}
	if !reflect.DeepEqual(repo.notices[0].Reasons, []string{events.ProjectReactivatedReasonInvoicePaid}) {
		t.Errorf("reasons = %v", repo.notices[0].Reasons)
	}
}

func TestACPReopen_PartnerWithoutPrimaryPartnerHasNoInvoiceTrack(t *testing.T) {
	p := acpSuspendedProject()
	p.EndDateClosureState = acpStr("OPEN")
	p.InvoiceDueDateClosureState = acpStr("SUSPENDED")
	p.SuspensionProcessState = json.RawMessage(`{"based_on_due_invoices":{"event_type":"suspend"}}`)
	repo := &acpFakeRepo{projects: map[string]*domain.ACPReopenProject{acpTestSfID: p}}
	sf := acpRenewedSF()
	sf.customer.AccountClassification = acpStr("Partner")
	if err := newACPTestService(repo, sf, nil, nil).HandleEvent(context.Background(), acpProjectEvent(domain.SalesforceEventUpdated)); err != nil {
		t.Fatal(err)
	}
	if acpLabel(repo.projects[acpTestSfID].InvoiceDueDateClosureState) != "OPEN" {
		t.Errorf("invoice track = %s, want OPEN", acpLabel(repo.projects[acpTestSfID].InvoiceDueDateClosureState))
	}
}

// A failure a retry can fix is returned by the check itself (the daily check
// counts it), but never fails the Salesforce event: its ingest succeeded.
func TestACPReopen_FailuresAreReportedButNeverFailTheEvent(t *testing.T) {
	repo := &acpFakeRepo{projects: map[string]*domain.ACPReopenProject{acpTestSfID: acpSuspendedProject()}}
	sf := acpRenewedSF()
	sf.getProjectErr = errors.New("salesforce down")
	svc := newACPTestService(repo, sf, nil, nil)
	if err := svc.HandleEvent(context.Background(), acpProjectEvent(domain.SalesforceEventUpdated)); err != nil {
		t.Errorf("a Salesforce read failure failed the event: %v", err)
	}
	if _, err := svc.acpReopen.reopenProject(context.Background(), acpTestSfID); err == nil {
		t.Error("a Salesforce read failure was not reported by the check")
	}
	repo.getErr = errors.New("database down")
	svc = newACPTestService(repo, acpRenewedSF(), nil, nil)
	if err := svc.HandleEvent(context.Background(), acpProjectEvent(domain.SalesforceEventUpdated)); err != nil {
		t.Errorf("a database failure failed the event: %v", err)
	}
	if _, err := svc.acpReopen.reopenProject(context.Background(), acpTestSfID); err == nil {
		t.Error("a database failure was not reported by the check")
	}
}

func TestACPReopen_CorruptStateIsAcknowledged(t *testing.T) {
	p := acpSuspendedProject()
	p.SuspensionProcessState = json.RawMessage(`{broken`)
	repo := &acpFakeRepo{projects: map[string]*domain.ACPReopenProject{acpTestSfID: p}}
	sf := acpRenewedSF()
	if err := newACPTestService(repo, sf, nil, nil).HandleEvent(context.Background(), acpProjectEvent(domain.SalesforceEventUpdated)); err != nil {
		t.Errorf("err = %v: bad data must not be retried for ever", err)
	}
	if repo.reopens != 0 || sf.calls != 0 {
		t.Errorf("reopens=%d calls=%d", repo.reopens, sf.calls)
	}
}

func TestACPReopen_NoPublisherStillReopens(t *testing.T) {
	repo := &acpFakeRepo{projects: map[string]*domain.ACPReopenProject{acpTestSfID: acpSuspendedProject()}}
	if err := newACPTestService(repo, acpRenewedSF(), nil, nil).HandleEvent(context.Background(), acpProjectEvent(domain.SalesforceEventUpdated)); err != nil {
		t.Fatal(err)
	}
	if acpLabel(repo.projects[acpTestSfID].ClosureState) != "OPEN" {
		t.Error("not reopened without a publisher")
	}
}

func TestACPReopen_OffMeansNothing(t *testing.T) {
	svc := NewSalesforceEventService(nil, nil, SalesforceIngestSupport{}).(*salesforceEventService)
	if err := svc.HandleEvent(context.Background(), acpProjectEvent(domain.SalesforceEventUpdated)); err != nil {
		t.Errorf("err = %v", err)
	}
}

func TestBuildProjectReactivatedPayload_FallsBackToCSM(t *testing.T) {
	p := domain.ACPReopenProject{ID: "pid", SfID: "sfid", Name: "CSM Name -[S]", Key: "KEY"}
	got := buildProjectReactivatedPayload(p, salesentity.Project{}, nil,
		domain.ACPReopenWrite{ReopenEndDate: true, ReopenInvoice: true}, acpTestNow, "@example.com")
	if got.ProjectName != "CSM Name -[S]" || got.ProjectKey != "KEY" || got.ProjectSfID != "sfid" {
		t.Errorf("payload = %+v", got)
	}
	if !reflect.DeepEqual(got.Reasons, []string{events.ProjectReactivatedReasonRenewal, events.ProjectReactivatedReasonInvoicePaid}) {
		t.Errorf("reasons = %v", got.Reasons)
	}
	if got.Recipients == nil || len(got.Recipients) != 0 {
		t.Errorf("recipients = %#v, want an empty list (never null)", got.Recipients)
	}
}

// An invoice due before the project's start date never counts, in ACP and
// here alike, so an overdue invoice of an earlier term does not keep a
// project whose start moved (a renewal) suspended.
func TestACPReopen_InvoiceDueBeforeTheProjectStartDoesNotCount(t *testing.T) {
	p := acpSuspendedProject()
	p.EndDateClosureState = acpStr("OPEN")
	p.InvoiceDueDateClosureState = acpStr("SUSPENDED")
	p.SuspensionProcessState = json.RawMessage(`{"based_on_due_invoices":{"event_type":"suspend"}}`)
	repo := &acpFakeRepo{projects: map[string]*domain.ACPReopenProject{acpTestSfID: p}}
	sf := acpRenewedSF() // starts 2026-10-28
	sf.linksByProj = []salesentity.LinkedOpportunity{{ID: "L1", OpportunityID: acpStr("006A")}}
	sf.opportunities = map[string]salesentity.Opportunity{"006A": {ID: "006A", StageName: acpStr("50 - Closed Won"),
		EulaVersion: salesentity.OptionalString{Present: true, Value: acpStr("EULA 3.4")}}}
	sf.invoices = map[string][]salesentity.Invoice{"006A": {{ID: acpStr("INV1"), InvoiceDate: acpStr("2026-08-01"), DueDate: acpStr("2026-09-01")}}}
	if err := newACPTestService(repo, sf, nil, nil).HandleEvent(context.Background(), acpProjectEvent(domain.SalesforceEventUpdated)); err != nil {
		t.Fatal(err)
	}
	if acpLabel(repo.projects[acpTestSfID].InvoiceDueDateClosureState) != "OPEN" {
		t.Errorf("invoice track = %s, want OPEN", acpLabel(repo.projects[acpTestSfID].InvoiceDueDateClosureState))
	}
}

// An invoice date that is not a date keeps the track suspended: never reopen
// on data that cannot be checked.
func TestACPReopen_UnreadableInvoiceKeepsTheTrackSuspended(t *testing.T) {
	p := acpSuspendedProject()
	p.EndDateClosureState = acpStr("OPEN")
	p.InvoiceDueDateClosureState = acpStr("SUSPENDED")
	p.SuspensionProcessState = json.RawMessage(`{"based_on_due_invoices":{"event_type":"suspend"}}`)
	repo := &acpFakeRepo{projects: map[string]*domain.ACPReopenProject{acpTestSfID: p}}
	sf := acpRenewedSF()
	sf.linksByProj = []salesentity.LinkedOpportunity{{ID: "L1", OpportunityID: acpStr("006A")}}
	sf.opportunities = map[string]salesentity.Opportunity{"006A": {ID: "006A", StageName: acpStr("50 - Closed Won"),
		EulaVersion: salesentity.OptionalString{Present: true, Value: acpStr("EULA 3.4")}}}
	sf.invoices = map[string][]salesentity.Invoice{"006A": {{ID: acpStr("INV1"), DueDate: acpStr("not a date")}}}
	if err := newACPTestService(repo, sf, nil, nil).HandleEvent(context.Background(), acpProjectEvent(domain.SalesforceEventUpdated)); err != nil {
		t.Fatal(err)
	}
	if repo.reopens != 0 {
		t.Errorf("reopened on an unreadable invoice")
	}
}

// The reopen check runs after a successful Project ingest, and not at all
// when the ingest fails: that event is retried and checked then.
func TestACPReopen_RunsOnlyAfterTheIngestSucceeded(t *testing.T) {
	p := acpSuspendedProject()
	p.SfID = testProjectSfID
	repo := &acpFakeRepo{projects: map[string]*domain.ACPReopenProject{testProjectSfID: p}}
	sf := acpRenewedSF()

	failing := &fakeProjectSalesEntity{err: errors.New("sales-entity down")}
	svc := newProjectService(failing, newProjectRepo(map[string]string{testProjectSfID: testProjectRowID}), &fakeIngestStateRepo{}, false)
	svc = WithACPReopen(svc, ACPReopen{Projects: repo, SalesEntity: sf, Now: func() time.Time { return acpTestNow }}).(*salesforceEventService)
	if err := svc.HandleEvent(context.Background(), projectEvent(domain.SalesforceEventUpdated)); err == nil {
		t.Fatal("the ingest failure was not returned")
	}
	if repo.reopens != 0 || sf.calls != 0 {
		t.Errorf("reopen ran after a failed ingest: reopens=%d calls=%d", repo.reopens, sf.calls)
	}

	ok := &fakeProjectSalesEntity{project: sampleProject()}
	svc = newProjectService(ok, newProjectRepo(map[string]string{testProjectSfID: testProjectRowID}), &fakeIngestStateRepo{}, false)
	svc = WithACPReopen(svc, ACPReopen{Projects: repo, SalesEntity: sf, Now: func() time.Time { return acpTestNow }}).(*salesforceEventService)
	if err := svc.HandleEvent(context.Background(), projectEvent(domain.SalesforceEventUpdated)); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if repo.reopens != 1 || acpLabel(repo.projects[testProjectSfID].ClosureState) != "OPEN" {
		t.Errorf("reopen did not run after the ingest: reopens=%d", repo.reopens)
	}
}
