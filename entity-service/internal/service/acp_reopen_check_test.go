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
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesentity"
)

// A closure state set by hand (it differs from what the sub-states give) is
// a person's decision: the reopen leaves it alone, before any Salesforce
// call and again under the row lock.
func TestACPReopen_ClosureStateSetByHandIsLeftAlone(t *testing.T) {
	p := acpSuspendedProject()
	p.ClosureState = acpStr("OPEN") // an admin set Open while the end-date track is still CLOSED
	repo := &acpFakeRepo{projects: map[string]*domain.ACPReopenProject{acpTestSfID: p}}
	sf := acpRenewedSF()
	if err := newACPTestService(repo, sf, &acpFakePublisher{}, nil).HandleEvent(context.Background(), acpProjectEvent(domain.SalesforceEventUpdated)); err != nil {
		t.Fatal(err)
	}
	if repo.reopens != 0 || sf.calls != 0 || len(repo.notices) != 0 {
		t.Errorf("reopens=%d salesforce calls=%d notices=%d, want none", repo.reopens, sf.calls, len(repo.notices))
	}

	// Set by hand while the check was reading Salesforce: the re-check under
	// the lock stops it.
	repo = &acpFakeRepo{projects: map[string]*domain.ACPReopenProject{acpTestSfID: acpSuspendedProject()}}
	overridden := acpSuspendedProject()
	overridden.ClosureState = acpStr("RESTRICTED")
	repo.underLock = overridden
	sf = acpRenewedSF()
	if err := newACPTestService(repo, sf, &acpFakePublisher{}, nil).HandleEvent(context.Background(), acpProjectEvent(domain.SalesforceEventUpdated)); err != nil {
		t.Fatal(err)
	}
	if len(sf.statusCalls) != 0 || len(repo.notices) != 0 || acpLabel(repo.projects[acpTestSfID].EndDateClosureState) != "CLOSED" {
		t.Errorf("status=%v notices=%d end=%s, want nothing written", sf.statusCalls, len(repo.notices), acpLabel(repo.projects[acpTestSfID].EndDateClosureState))
	}
}

func TestACPDerivedClosureState(t *testing.T) {
	tests := []struct {
		end, invoice, compliance, want string
	}{
		{"OPEN", "OPEN", "OPEN", "OPEN"},
		{"CLOSED", "OPEN", "OPEN", "SUSPENDED"},
		{"SUSPENDED", "OPEN", "OPEN", "SUSPENDED"},
		{"OPEN", "SUSPENDED_PENDING", "OPEN", "SUSPENDED"},
		{"OPEN", "OPEN", "SUSPENDED", "SUSPENDED"},
		{"RESTRICTED", "OPEN", "OPEN", "RESTRICTED"},
		{"OPEN", "RESTRICTED", "OPEN", "RESTRICTED"},
		{"", "", "", "OPEN"},
	}
	for _, tt := range tests {
		p := domain.ACPReopenProject{EndDateClosureState: acpStr(tt.end), InvoiceDueDateClosureState: acpStr(tt.invoice),
			ComplianceViolationClosureState: acpStr(tt.compliance)}
		if got := acpDerivedClosureState(p); got != tt.want {
			t.Errorf("%s/%s/%s = %s, want %s", tt.end, tt.invoice, tt.compliance, got, tt.want)
		}
	}
}

// An invoice event first asks the database whether any invoice track is
// ACP-suspended; with none it makes no Salesforce call at all.
func TestACPReopen_InvoiceEventWithNothingSuspendedCallsNoSalesforce(t *testing.T) {
	repo := &acpFakeRepo{projects: map[string]*domain.ACPReopenProject{acpTestSfID: acpSuspendedProject()}} // end-date track only
	sf := acpRenewedSF()
	req := domain.SalesforceEventRequest{EventType: domain.SalesforceEventUpdated, Entity: domain.SalesforceEntityInvoice, ReferenceID: "INV1"}
	if err := newACPTestService(repo, sf, nil, nil).HandleEvent(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if repo.invoiceTrackChecks != 1 || sf.calls != 0 {
		t.Errorf("database checks=%d salesforce calls=%d, want 1 and 0", repo.invoiceTrackChecks, sf.calls)
	}
}

// The notice goes only to well-formed internal addresses: a customer's
// person recorded as technical owner and a broken address are dropped.
func TestBuildProjectReactivatedPayload_OnlyInternalWellFormedRecipients(t *testing.T) {
	c := &salesentity.Customer{
		Owner:          &salesentity.CustomerUser{Name: acpStr("Alex Owner"), Email: acpStr("alex@example.com")},
		RenewalManager: &salesentity.CustomerUser{Email: acpStr("broken@@example.com")},
		TechnicalOwner: acpStr("tech@customer.example"),
	}
	got := buildProjectReactivatedPayload(domain.ACPReopenProject{ID: "pid"}, salesentity.Project{}, c,
		domain.ACPReopenWrite{ReopenEndDate: true}, acpTestNow, "@example.com")
	if !reflect.DeepEqual(got.Recipients, []string{"alex@example.com"}) {
		t.Errorf("recipients = %v, want only the internal, well-formed address", got.Recipients)
	}
	// An account owner outside the domain is not named as the owner either.
	c.Owner.Email = acpStr("owner@customer.example")
	got = buildProjectReactivatedPayload(domain.ACPReopenProject{ID: "pid"}, salesentity.Project{}, c,
		domain.ACPReopenWrite{ReopenEndDate: true}, acpTestNow, "@example.com")
	if got.AccountOwnerEmail != "" || len(got.Recipients) != 0 {
		t.Errorf("owner=%q recipients=%v, want none", got.AccountOwnerEmail, got.Recipients)
	}
}

func TestACPInternalAddress(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"alice@example.com", true},
		{"Alice@EXAMPLE.com", true},
		{" alice@example.com ", true},
		{"alice@customer.example", false},
		{"alice@notexample.com", false},
		{"Alice <alice@example.com>", false},
		{"broken@@example.com", false},
		{"alice@", false},
		{"", false},
	}
	for _, tt := range tests {
		if _, got := acpInternalAddress(tt.in, "@example.com"); got != tt.want {
			t.Errorf("acpInternalAddress(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

// In dual-write, a reopen is copied to the previous system with the same
// fields PATCH /projects/{id} carries.
func TestACPReopen_ReopenIsMirrored(t *testing.T) {
	repo := &acpFakeRepo{projects: map[string]*domain.ACPReopenProject{acpTestSfID: acpSuspendedProject()}}
	var mirrored []domain.ProjectUpdateRequest
	svc := newACPTestService(repo, acpRenewedSF(), &acpFakePublisher{}, func(r *ACPReopen) {
		r.Mirror = func(_ context.Context, projectID string, req domain.ProjectUpdateRequest) {
			if projectID != acpTestProjectID {
				t.Errorf("mirrored project %s", projectID)
			}
			mirrored = append(mirrored, req)
		}
	})
	if err := svc.HandleEvent(context.Background(), acpProjectEvent(domain.SalesforceEventUpdated)); err != nil {
		t.Fatal(err)
	}
	if len(mirrored) != 1 {
		t.Fatalf("mirror writes = %d, want 1", len(mirrored))
	}
	m := mirrored[0]
	if derefString(m.EndDateClosureState) != "Open" || m.InvoiceDueDateClosureState != nil {
		t.Errorf("mirror = end %v invoice %v, want end Open only", m.EndDateClosureState, m.InvoiceDueDateClosureState)
	}
	if ev, _ := acpSectionEventType(m.SuspensionProcessState, acpEndDateSection); ev != acpOpenEventType {
		t.Errorf("mirrored record = %s", m.SuspensionProcessState)
	}

	// Nothing is mirrored in a dry run or when nothing reopens.
	mirrored = nil
	repo = &acpFakeRepo{projects: map[string]*domain.ACPReopenProject{acpTestSfID: acpSuspendedProject()}}
	svc = newACPTestService(repo, acpRenewedSF(), nil, func(r *ACPReopen) {
		r.DryRun = true
		r.Mirror = func(context.Context, string, domain.ProjectUpdateRequest) {
			mirrored = append(mirrored, domain.ProjectUpdateRequest{})
		}
	})
	if err := svc.HandleEvent(context.Background(), acpProjectEvent(domain.SalesforceEventUpdated)); err != nil {
		t.Fatal(err)
	}
	if len(mirrored) != 0 {
		t.Errorf("a dry run mirrored %d writes", len(mirrored))
	}
}

// The Salesforce write made inside the transaction carries its own deadline.
func TestACPReopen_SalesforceWriteHasItsOwnDeadline(t *testing.T) {
	repo := &acpFakeRepo{projects: map[string]*domain.ACPReopenProject{acpTestSfID: acpSuspendedProject()}}
	sf := &acpDeadlineSF{acpFakeSF: acpRenewedSF()}
	r := ACPReopen{Projects: repo, SalesEntity: sf, Now: func() time.Time { return acpTestNow }}
	if _, err := r.reopenProject(context.Background(), acpTestSfID); err != nil {
		t.Fatal(err)
	}
	if !sf.hadDeadline || sf.remaining > acpReopenSalesforceWriteTimeout {
		t.Errorf("deadline=%v remaining=%s, want at most %s", sf.hadDeadline, sf.remaining, acpReopenSalesforceWriteTimeout)
	}
}

type acpDeadlineSF struct {
	*acpFakeSF
	hadDeadline bool
	remaining   time.Duration
}

func (f *acpDeadlineSF) UpdateProjectStatus(ctx context.Context, id, status string) error {
	if d, ok := ctx.Deadline(); ok {
		f.hadDeadline, f.remaining = true, time.Until(d)
	}
	return f.acpFakeSF.UpdateProjectStatus(ctx, id, status)
}

// acpOpenedByHandProject is a project ACP suspended on the end-date track
// that a person then set back to Open: Open everywhere, the record still says
// "suspend".
func acpOpenedByHandProject() *domain.ACPReopenProject {
	p := acpSuspendedProject()
	p.EndDateClosureState = acpStr("OPEN")
	p.ClosureState = acpStr("OPEN")
	return p
}

func newACPChecker(repo *acpFakeRepo, sf ACPReopenSalesforce, mod func(*ACPReopen)) *ACPReopen {
	r := &ACPReopen{Projects: repo, SalesEntity: sf, Notify: true, InternalDomain: "@example.com",
		Now: func() time.Time { return acpTestNow }}
	if mod != nil {
		mod(r)
	}
	return r
}

// The daily check reopens an ACP-suspended project whose renewal was never
// seen as an event, finishes one a person set back to Open, and leaves the
// rest.
func TestACPReopenCheck_ReopensAndFinishes(t *testing.T) {
	missed := acpSuspendedProject() // renewed in Salesforce, the event was lost
	opened := acpOpenedByHandProject()
	opened.ID, opened.SfID = "e3e87599-1bc7-6650-182c-0dc5604bcb69", acpOtherSfID
	repo := &acpFakeRepo{projects: map[string]*domain.ACPReopenProject{acpTestSfID: missed, acpOtherSfID: opened}}
	sf := acpRenewedSF()

	resp, err := newACPChecker(repo, sf, nil).CheckSuspendedProjects(context.Background(), domain.ACPReopenCheckRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Checked != 2 || resp.Reopened != 1 || resp.Finished != 1 || resp.Failed != 0 || resp.More {
		t.Errorf("response = %+v", resp)
	}
	if acpLabel(missed.ClosureState) != "OPEN" {
		t.Errorf("the missed renewal was not reopened: %s", acpLabel(missed.ClosureState))
	}
	if ev, _ := acpSectionEventType(opened.SuspensionProcessState, acpEndDateSection); ev != acpOpenEventType {
		t.Errorf("the record of the project opened by hand still says %q", ev)
	}
	if len(sf.statusCalls) != 2 {
		t.Errorf("Salesforce status writes = %v, want both projects Open", sf.statusCalls)
	}
	if len(repo.notices) != 2 {
		t.Fatalf("notices = %d, want 2", len(repo.notices))
	}
	var finishedNotice events.ProjectReactivatedPayload
	for _, n := range repo.notices {
		if n.ProjectID == opened.ID {
			finishedNotice = n
		}
	}
	if len(finishedNotice.Reasons) != 0 {
		t.Errorf("the notice of a project opened by hand gives reasons %v, want none", finishedNotice.Reasons)
	}

	// Run again: nothing left to do.
	sf.statusCalls = nil
	resp, err = newACPChecker(repo, sf, nil).CheckSuspendedProjects(context.Background(), domain.ACPReopenCheckRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Checked != 0 || len(sf.statusCalls) != 0 || len(repo.notices) != 2 {
		t.Errorf("second run: %+v status=%v notices=%d", resp, sf.statusCalls, len(repo.notices))
	}
}

// A dry run logs and changes nothing, also for a project opened by hand; a
// failure is counted and named, and the rest are still checked.
func TestACPReopenCheck_DryRunAndFailures(t *testing.T) {
	opened := acpOpenedByHandProject()
	repo := &acpFakeRepo{projects: map[string]*domain.ACPReopenProject{acpTestSfID: opened}}
	sf := acpRenewedSF()
	resp, err := newACPChecker(repo, sf, func(r *ACPReopen) { r.DryRun = true }).CheckSuspendedProjects(context.Background(), domain.ACPReopenCheckRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Checked != 1 || resp.Finished != 0 || len(sf.statusCalls) != 0 || len(repo.notices) != 0 {
		t.Errorf("dry run: %+v status=%v notices=%d", resp, sf.statusCalls, len(repo.notices))
	}

	failing := acpSuspendedProject()
	ok := acpSuspendedProject()
	ok.ID, ok.SfID = "e3e87599-1bc7-6650-182c-0dc5604bcb69", acpOtherSfID
	repo = &acpFakeRepo{projects: map[string]*domain.ACPReopenProject{acpTestSfID: failing, acpOtherSfID: ok}}
	sf2 := &acpFailingFor{acpFakeSF: acpRenewedSF(), sfID: acpTestSfID}
	resp, err = newACPChecker(repo, sf2, nil).CheckSuspendedProjects(context.Background(), domain.ACPReopenCheckRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Failed != 1 || !reflect.DeepEqual(resp.FailedProjectIDs, []string{acpTestProjectID}) || resp.Reopened != 1 {
		t.Errorf("response = %+v", resp)
	}
}

type acpFailingFor struct {
	*acpFakeSF
	sfID string
}

func (f *acpFailingFor) GetProject(ctx context.Context, id string) (salesentity.Project, error) {
	if id == f.sfID {
		return salesentity.Project{}, errors.New("salesforce down")
	}
	return f.acpFakeSF.GetProject(ctx, id)
}

// Pages follow the cursor; a malformed cursor is refused.
func TestACPReopenCheck_Pages(t *testing.T) {
	repo := &acpFakeRepo{projects: map[string]*domain.ACPReopenProject{}}
	for i, sfID := range []string{"a0dE200000CZ9Z1IA1", "a0dE200000CZ9Z2IA1", "a0dE200000CZ9Z3IA1"} {
		p := acpSuspendedProject()
		p.ID = "e3e87599-1bc7-6650-182c-0dc5604bcb6" + string(rune('0'+i))
		p.SfID = sfID
		repo.projects[sfID] = p
	}
	checker := newACPChecker(repo, acpRenewedSF(), func(r *ACPReopen) { r.DryRun = true })
	seen := 0
	cursor := ""
	for page := 0; page < 5; page++ {
		resp, err := checker.CheckSuspendedProjects(context.Background(), domain.ACPReopenCheckRequest{Cursor: cursor, Limit: 2})
		if err != nil {
			t.Fatal(err)
		}
		seen += resp.Checked
		if !resp.More {
			break
		}
		cursor = resp.NextCursor
	}
	if seen != 3 {
		t.Errorf("checked %d projects over the pages, want 3", seen)
	}
	if _, err := checker.CheckSuspendedProjects(context.Background(), domain.ACPReopenCheckRequest{Cursor: "not-a-uuid"}); err == nil {
		t.Error("a malformed cursor was accepted")
	}
}

// A page that runs short of time still checks its first project (so every
// call makes progress), then stops and says there is more, with a cursor
// that resumes at the next project. A stopped page is never reported as the
// end.
func TestACPReopenCheck_StopsBeforeTheDeadlineButAlwaysMakesProgress(t *testing.T) {
	first := acpSuspendedProject()
	second := acpSuspendedProject()
	second.ID, second.SfID = "e3e87599-1bc7-6650-182c-0dc5604bcb69", acpOtherSfID
	repo := &acpFakeRepo{projects: map[string]*domain.ACPReopenProject{acpTestSfID: first, acpOtherSfID: second}}
	checker := newACPChecker(repo, acpRenewedSF(), func(r *ACPReopen) { r.DryRun = true })

	ctx, cancel := context.WithTimeout(context.Background(), acpCheckTimeReserve/2)
	defer cancel()
	resp, err := checker.CheckSuspendedProjects(ctx, domain.ACPReopenCheckRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Checked != 1 || !resp.More || resp.NextCursor != first.ID {
		t.Fatalf("response = %+v, want one checked, more, resume after %s", resp, first.ID)
	}
	resp, err = checker.CheckSuspendedProjects(ctx, domain.ACPReopenCheckRequest{Cursor: resp.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Checked != 1 || resp.More {
		t.Errorf("second call = %+v, want the last project and no more", resp)
	}
}

// acpFakeOutbox is the notice queue over memory.
type acpFakeOutbox struct {
	pending  []int64
	payloads map[int64]json.RawMessage
	failures map[int64]int
	done     map[int64]bool
}

func (o *acpFakeOutbox) Pending(context.Context, int) ([]int64, error) {
	var ids []int64
	for _, id := range o.pending {
		if !o.done[id] {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (o *acpFakeOutbox) Process(ctx context.Context, id int64, publish func(context.Context, string, json.RawMessage) error) (bool, error) {
	if err := publish(ctx, "project-"+string(rune('0'+id)), o.payloads[id]); err != nil {
		return false, err
	}
	o.done[id] = true
	return true, nil
}

func (o *acpFakeOutbox) RecordFailure(_ context.Context, id int64, _ string, max int) (bool, error) {
	o.failures[id]++
	if o.failures[id] >= max {
		o.done[id] = true
		return true, nil
	}
	return false, nil
}

type acpFlakyPublisher struct {
	fail      bool
	published []string
}

func (p *acpFlakyPublisher) Publish(_ context.Context, t events.Type, id string, _ json.RawMessage) error {
	if p.fail {
		return errors.New("event hub down")
	}
	if t != events.TypeProjectReactivated {
		return errors.New("wrong event type " + string(t))
	}
	p.published = append(p.published, id)
	return nil
}
func (p *acpFlakyPublisher) Close() {}

// A shutdown during a publish leaves the notice queued, with no failure
// recorded; a non-positive interval falls back to the default.
func TestProjectReactivationDrainer_ShutdownRecordsNoFailure(t *testing.T) {
	outbox := &acpFakeOutbox{pending: []int64{1}, payloads: map[int64]json.RawMessage{1: json.RawMessage(`{}`)},
		failures: map[int64]int{}, done: map[int64]bool{}}
	ctx, cancel := context.WithCancel(context.Background())
	pub := &acpCancellingPublisher{cancel: cancel}
	NewProjectReactivationDrainer(outbox, pub, time.Second, 3).DrainOnce(ctx)
	if outbox.failures[1] != 0 || outbox.done[1] {
		t.Errorf("failures=%d done=%v, want the notice left queued as it was", outbox.failures[1], outbox.done[1])
	}
	if d := NewProjectReactivationDrainer(outbox, pub, 0, 3); d.interval != projectReactivationDefaultInterval {
		t.Errorf("a zero interval was kept: %s", d.interval)
	}
}

type acpCancellingPublisher struct{ cancel context.CancelFunc }

func (p *acpCancellingPublisher) Publish(ctx context.Context, _ events.Type, _ string, _ json.RawMessage) error {
	p.cancel()
	return ctx.Err()
}
func (p *acpCancellingPublisher) Close() {}

// A queued notice is published once Event Hub takes it; until then it is
// retried, and parked after maxAttempts.
func TestProjectReactivationDrainer(t *testing.T) {
	outbox := &acpFakeOutbox{pending: []int64{1}, payloads: map[int64]json.RawMessage{1: json.RawMessage(`{}`)},
		failures: map[int64]int{}, done: map[int64]bool{}}
	pub := &acpFlakyPublisher{fail: true}
	d := NewProjectReactivationDrainer(outbox, pub, time.Second, 3)

	d.DrainOnce(context.Background())
	if outbox.done[1] || outbox.failures[1] != 1 {
		t.Fatalf("after a failure: done=%v failures=%d", outbox.done[1], outbox.failures[1])
	}
	pub.fail = false
	d.DrainOnce(context.Background())
	if !outbox.done[1] || !reflect.DeepEqual(pub.published, []string{"project-1"}) {
		t.Errorf("after recovery: done=%v published=%v", outbox.done[1], pub.published)
	}

	parked := &acpFakeOutbox{pending: []int64{2}, payloads: map[int64]json.RawMessage{2: json.RawMessage(`{}`)},
		failures: map[int64]int{}, done: map[int64]bool{}}
	d = NewProjectReactivationDrainer(parked, &acpFlakyPublisher{fail: true}, time.Second, 2)
	d.DrainOnce(context.Background())
	d.DrainOnce(context.Background())
	if !parked.done[2] || parked.failures[2] != 2 {
		t.Errorf("not parked after max attempts: done=%v failures=%d", parked.done[2], parked.failures[2])
	}
}
