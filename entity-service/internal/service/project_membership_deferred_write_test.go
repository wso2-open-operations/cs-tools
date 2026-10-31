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

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesentity"
)

// ---- fakes ---------------------------------------------------------------

// fakeWriteRequestRepo is the salesforce_write_request ledger in memory.
type fakeWriteRequestRepo struct {
	requests  map[string]domain.SalesforceWriteRequest
	published []string
	attempts  []string
	failed    map[string]string
	retryable map[string]bool
	getErr    error
}

func newFakeWriteRequestRepo() *fakeWriteRequestRepo {
	return &fakeWriteRequestRepo{requests: map[string]domain.SalesforceWriteRequest{}, failed: map[string]string{}, retryable: map[string]bool{}}
}

func (f *fakeWriteRequestRepo) Get(_ context.Context, id string) (domain.SalesforceWriteRequest, error) {
	if f.getErr != nil {
		return domain.SalesforceWriteRequest{}, f.getErr
	}
	r, ok := f.requests[id]
	if !ok {
		return domain.SalesforceWriteRequest{}, &apierror.NotFoundError{Msg: "salesforce write request not found"}
	}
	return r, nil
}

func (f *fakeWriteRequestRepo) PendingForProjectContact(_ context.Context, projectContactID string) (*domain.SalesforceWriteRequest, error) {
	for _, r := range f.requests {
		if r.ProjectContactID == projectContactID && r.Status == domain.SalesforceWriteRequestPending {
			rr := r
			return &rr, nil
		}
	}
	return nil, nil
}

func (f *fakeWriteRequestRepo) LatestForProjectContact(_ context.Context, projectContactID string) (*domain.SalesforceWriteRequest, error) {
	var latest *domain.SalesforceWriteRequest
	for _, r := range f.requests {
		if r.ProjectContactID == projectContactID && (latest == nil || r.CreatedOn.After(latest.CreatedOn)) {
			rr := r
			latest = &rr
		}
	}
	return latest, nil
}

func (f *fakeWriteRequestRepo) SyncStatusForProjectContacts(context.Context, []string) (map[string]string, error) {
	return map[string]string{}, nil
}

func (f *fakeWriteRequestRepo) MarkPublished(_ context.Context, id string) error {
	f.published = append(f.published, id)
	return nil
}

func (f *fakeWriteRequestRepo) RecordAttempt(_ context.Context, id string, _ error) error {
	f.attempts = append(f.attempts, id)
	return nil
}

func (f *fakeWriteRequestRepo) MarkFailed(_ context.Context, id string, cause error, retryable bool) error {
	f.failed[id] = cause.Error()
	f.retryable[id] = retryable
	if r, ok := f.requests[id]; ok {
		r.Status = domain.SalesforceWriteRequestFailed
		f.requests[id] = r
	}
	return nil
}

// fakeInvitationValidator refuses when err is set, and records the calls.
type fakeInvitationValidator struct {
	err   error
	calls int
}

func (f *fakeInvitationValidator) Validate(context.Context, domain.MembershipWriteTarget, string, string) error {
	f.calls++
	return f.err
}

// fakePartners answers the partner accounts of the project's account.
type fakePartners struct {
	partners []string
	err      error
}

func (f *fakePartners) PartnerAccountSfIDs(context.Context, string) ([]string, error) {
	return f.partners, f.err
}

// deferredHarness is the write harness with the deferred write wired: a
// request ledger, a publisher for the command topic and the partner lookup.
type deferredHarness struct {
	*writeHarness
	requests *fakeWriteRequestRepo
	commands *fakeInvitePublisher
	partners *fakePartners
}

func newDeferredHarness(t *testing.T) *deferredHarness {
	t.Helper()
	h := newInternalWriteHarness(t)
	d := &deferredHarness{writeHarness: h, requests: newFakeWriteRequestRepo(), commands: &fakeInvitePublisher{},
		partners: &fakePartners{partners: []string{"001000000000002PAR"}}}
	d.svc = NewProjectMembershipWriteService(MembershipWriteDeps{
		Memberships:              h.repo,
		Steps:                    h.steps,
		SalesEntity:              h.se,
		Publisher:                h.pub,
		Failures:                 h.failures,
		Access:                   alwaysUnrestrictedAccess{},
		UserCache:                h.cache,
		WriteRequests:            d.requests,
		SalesforceWritePublisher: d.commands,
		Partners:                 d.partners,
	})
	return d
}

func salesforceDown() error {
	return &apierror.ServiceUnavailableError{Msg: "salesentity: POST /contacts/search request: dial tcp: connection refused"}
}

// pendingRequest seeds the ledger with the request an earlier deferred
// invitation would have left, and the membership row that goes with it.
func (d *deferredHarness) pendingRequest(t *testing.T, intent domain.SalesforceWriteIntent) domain.SalesforceWriteRequest {
	t.Helper()
	raw, err := json.Marshal(intent)
	if err != nil {
		t.Fatal(err)
	}
	published := time.Now()
	req := domain.SalesforceWriteRequest{
		ID: writeRequestID, Operation: domain.SalesforceWriteOperationInvite,
		ProjectID: writeProjectID, ProjectContactID: writeProjectContactID,
		Email: writeEmail, Intent: raw, Status: domain.SalesforceWriteRequestPending, PublishedOn: &published,
	}
	d.requests.requests[req.ID] = req
	d.repo.existing = &domain.ProjectMembershipRow{
		ProjectContactID: writeProjectContactID, Email: writeEmail, State: intent.State,
		FirstName: intent.FirstName, LastName: intent.LastName, ProjectKey: "ACMEPROD", ProjectName: "Acme Prod",
	}
	return req
}

func inviteIntent() domain.SalesforceWriteIntent {
	return domain.SalesforceWriteIntent{Email: writeEmail, FirstName: "Jane", LastName: "Doe", Roles: []string{"Portal user"}, State: domain.MembershipStateInvited}
}

// ---- the invitation defers -----------------------------------------------

func TestDeferredWrite_InviteCommitsLocallyWhenSalesforceIsDown(t *testing.T) {
	d := newDeferredHarness(t)
	d.se.searchErr = salesforceDown()

	got, err := d.svc.Invite(context.Background(), writeProjectID, inviteReq("Portal user"))
	if err != nil {
		t.Fatalf("an outage must not fail the invitation: %v", err)
	}
	if got.SalesforceSync != domain.SalesforceSyncPending || got.MembershipSfID != "" || got.ContactSfID != "" {
		t.Errorf("response = %+v, want salesforceSync PENDING and no ids", got)
	}
	if got.State != domain.MembershipStateInvited || got.ProjectContactID != writeProjectContactID {
		t.Errorf("response = %+v", got)
	}
	if len(d.repo.upserts) != 1 {
		t.Fatalf("want one local upsert, got %d", len(d.repo.upserts))
	}
	in := d.repo.upserts[0]
	if in.MembershipSfID != "" || in.ContactSfID != "" || in.DeferredSalesforceWrite == nil {
		t.Errorf("upsert = %+v, want blank ids and a deferred write", in)
	}
	if in.DeferredSalesforceWrite.Intent.Email != writeEmail || in.DeferredSalesforceWrite.Intent.State != domain.MembershipStateInvited ||
		len(in.DeferredSalesforceWrite.Intent.Roles) != 1 || in.DeferredSalesforceWrite.Intent.Roles[0] != "Portal user" {
		t.Errorf("stored intent = %+v", in.DeferredSalesforceWrite.Intent)
	}
	if in.ContactAccountSfID != writeAccountSfID || in.Type != domain.MembershipTypeOwnContact || len(in.ProjectGroups) == 0 {
		t.Errorf("deferred rows must be an own contact of the project's account with its groups: %+v", in)
	}
	if len(d.repo.steps) != 0 {
		t.Errorf("no onboarding step may be recorded without a membership id: %+v", d.repo.steps)
	}
	// The command went out, keyed by the request, and the row was stamped.
	if len(d.commands.published) != 1 || d.commands.published[0].Type != events.TypeSalesforceMembershipWriteRequested || d.commands.published[0].EntityID != writeRequestID {
		t.Fatalf("command publish = %+v", d.commands.published)
	}
	var p events.SalesforceMembershipWriteRequestedPayload
	if err := json.Unmarshal(d.commands.published[0].Payload, &p); err != nil {
		t.Fatal(err)
	}
	if p.RequestID != writeRequestID || p.ProjectContactID != writeProjectContactID || p.Operation != domain.SalesforceWriteOperationInvite {
		t.Errorf("payload = %+v", p)
	}
	// Sales Entity cannot read this database: the command must carry the
	// whole intent and the Salesforce ids of the project and account.
	if p.Email != writeEmail || p.FirstName != "Jane" || p.LastName != "Doe" || len(p.Roles) != 1 || p.Roles[0] != "Portal user" ||
		p.State != domain.MembershipStateInvited || p.ProjectSfID != writeProjectSfID || p.AccountSfID != writeAccountSfID {
		t.Errorf("command intent = %+v", p)
	}
	// The accounts an existing contact may be adopted from: the project's
	// own and its partners, read here because the consumer cannot.
	if len(p.AllowedAccountSfIDs) != 2 || p.AllowedAccountSfIDs[0] != writeAccountSfID || p.AllowedAccountSfIDs[1] != "001000000000002PAR" {
		t.Errorf("allowedAccountSfIds = %v", p.AllowedAccountSfIDs)
	}
	if len(d.requests.published) != 1 || d.requests.published[0] != writeRequestID {
		t.Errorf("published_on not stamped: %v", d.requests.published)
	}
	// No invitation email yet: nothing on the project topic.
	if len(d.pub.published) != 0 {
		t.Errorf("project_contact.invited must wait for the Salesforce id: %+v", d.pub.published)
	}
	if len(d.se.createdContact) != 0 || len(d.se.createdPC) != 0 {
		t.Error("nothing may be created in Salesforce while it is down")
	}
}

func TestDeferredWrite_InviteDefersWhenTheValidatorCannotReachSalesforce(t *testing.T) {
	d := newDeferredHarness(t)
	v := &fakeInvitationValidator{err: salesforceDown()}
	d.svc = NewProjectMembershipWriteService(MembershipWriteDeps{
		Memberships: d.repo, Steps: d.steps, SalesEntity: d.se, Publisher: d.pub, Access: alwaysUnrestrictedAccess{},
		Invitations: v, WriteRequests: d.requests, SalesforceWritePublisher: d.commands, Partners: d.partners,
	})
	got, err := d.svc.Invite(context.Background(), writeProjectID, inviteReq("Portal user"))
	if err != nil {
		t.Fatal(err)
	}
	if got.SalesforceSync != domain.SalesforceSyncPending || len(d.commands.published) != 1 {
		t.Errorf("want a deferred invitation, got %+v / %d commands", got, len(d.commands.published))
	}
}

// What entity-service actually sees during a Salesforce outage is a timeout
// of the Sales Entity call (sales-entity-service retries Salesforce with
// backoff for longer than the client timeout). That must defer too, as long
// as the admin's own request is still alive.
func TestDeferredWrite_InviteDefersOnADownstreamTimeout(t *testing.T) {
	d := newDeferredHarness(t)
	d.se.searchErr = fmt.Errorf("salesentity: POST /contacts/search: %w", context.DeadlineExceeded)
	got, err := d.svc.Invite(context.Background(), writeProjectID, inviteReq("Portal user"))
	if err != nil {
		t.Fatalf("a downstream timeout with a live request must defer: %v", err)
	}
	if got.SalesforceSync != domain.SalesforceSyncPending || len(d.commands.published) != 1 {
		t.Errorf("want a deferred invitation, got %+v / %d commands", got, len(d.commands.published))
	}
	// But not when the admin's own request is gone: there is nobody to
	// answer, and the error is the caller's, not Salesforce's.
	d2 := newDeferredHarness(t)
	d2.se.searchErr = fmt.Errorf("salesentity: POST /contacts/search: %w", context.DeadlineExceeded)
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if _, err := d2.svc.Invite(ctx, writeProjectID, inviteReq("Portal user")); err == nil || len(d2.commands.published) != 0 {
		t.Errorf("a dead request must not be deferred: err=%v commands=%d", err, len(d2.commands.published))
	}
}

func TestDeferredWrite_InviteStillFailsWhenDeferralIsNotWired(t *testing.T) {
	h := newInternalWriteHarness(t)
	h.se.searchErr = salesforceDown()
	if _, err := h.svc.Invite(context.Background(), writeProjectID, inviteReq("Portal user")); err == nil {
		t.Fatal("without the ledger and publisher an outage must fail the invitation as before")
	}
	if len(h.repo.upserts) != 0 {
		t.Error("nothing may be written")
	}
}

// Only an outage is deferred. A refusal from Salesforce is final and a
// retry would not change it, so it is still the caller's error.
func TestDeferredWrite_InviteDoesNotDeferARefusal(t *testing.T) {
	cases := map[string]func(*deferredHarness){
		"validation refused by Salesforce": func(d *deferredHarness) {
			d.se.createErr = &apierror.DownstreamError{Msg: "salesforce rejected the contact"}
		},
		"invitation validator refuses": func(d *deferredHarness) {
			d.svc = NewProjectMembershipWriteService(MembershipWriteDeps{
				Memberships: d.repo, Steps: d.steps, SalesEntity: d.se, Publisher: d.pub, Access: alwaysUnrestrictedAccess{},
				Invitations:   &fakeInvitationValidator{err: &apierror.ValidationError{Msg: "public e-mail domains cannot be invited"}},
				WriteRequests: d.requests, SalesforceWritePublisher: d.commands, Partners: d.partners,
			})
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			d := newDeferredHarness(t)
			setup(d)
			if _, err := d.svc.Invite(context.Background(), writeProjectID, inviteReq("Portal user")); err == nil {
				t.Fatal("want the refusal")
			}
			if len(d.repo.upserts) != 0 || len(d.commands.published) != 0 {
				t.Errorf("a refusal must write nothing: upserts=%d commands=%d", len(d.repo.upserts), len(d.commands.published))
			}
		})
	}
}

// A re-invitation has Salesforce ids already; deferring it would put CSM
// ahead of Salesforce for a row the registration flow can act on.
func TestDeferredWrite_ReinvitationIsNotDeferred(t *testing.T) {
	d := newDeferredHarness(t)
	d.repo.existing = &domain.ProjectMembershipRow{
		ProjectContactID: writeProjectContactID, MembershipSfID: writeMembershipID, ContactSfID: writeContactSfID,
		Email: writeEmail, State: domain.MembershipStateDeactivated,
	}
	d.se.getContactErr = salesforceDown()
	d.se.searchErr = salesforceDown()
	if _, err := d.svc.Invite(context.Background(), writeProjectID, inviteReq("Portal user")); !isSalesforceUnavailable(context.Background(), err) {
		t.Fatalf("want the outage error, got %v", err)
	}
	if len(d.repo.upserts) != 0 || len(d.commands.published) != 0 {
		t.Error("a re-invitation must not be deferred")
	}
}

// ---- sales-entity-service answers ----------------------------------------

// completedResult is what Sales Entity publishes after creating both records
// for inviteIntent; the fake Sales Entity is given the same records so the
// completion can read them back.
func (d *deferredHarness) completedResult(t *testing.T, state string) events.SalesforceMembershipWriteCompletedPayload {
	t.Helper()
	d.se.contact = existingSalesforceContact()
	d.se.membership = &salesentity.ProjectContact{
		ID: writeMembershipID, Email: writeEmail, State: sampleStr(state), Roles: []string{"Portal user"},
		Contact:          &salesentity.ProjectContactContact{ID: sampleStr(writeContactSfID), Email: sampleStr(writeEmail), Name: sampleStr("Jane Doe")},
		Subscription:     &salesentity.ProjectContactSubscription{ID: sampleStr(writeProjectSfID), Key: sampleStr("ACMEPROD"), Name: sampleStr("Acme Prod")},
		LastModifiedDate: sampleStr("2026-10-23T10:00:00.000+0000"),
	}
	return events.SalesforceMembershipWriteCompletedPayload{
		RequestID: writeRequestID, ProjectContactID: writeProjectContactID,
		ContactSfID: writeContactSfID, MembershipSfID: writeMembershipID, State: state, Roles: []string{"Portal user"},
		ContactCreated: true, MembershipCreated: true, LastModifiedDate: "2026-10-23T10:00:00.000+0000",
	}
}

func completer(d *deferredHarness) DeferredSalesforceWriteCompleter {
	return d.svc.(DeferredSalesforceWriteCompleter)
}

func TestDeferredWrite_CompleteStampsIdsAndPublishesTheInvitation(t *testing.T) {
	d := newDeferredHarness(t)
	req := d.pendingRequest(t, inviteIntent())
	result := d.completedResult(t, domain.MembershipStateInvited)

	if err := completer(d).CompleteDeferredSalesforceWrite(context.Background(), result, false); err != nil {
		t.Fatalf("a completed result must finish the request: %v", err)
	}
	// Read back through Sales Entity, never written there from here.
	if len(d.se.contactGets) != 1 || len(d.se.createdContact) != 0 || len(d.se.createdPC) != 0 || len(d.se.updates) != 0 {
		t.Errorf("completion must only read Sales Entity: gets=%d creates=%d/%d updates=%d", len(d.se.contactGets), len(d.se.createdContact), len(d.se.createdPC), len(d.se.updates))
	}
	if len(d.repo.upserts) != 1 {
		t.Fatalf("want one upsert, got %d", len(d.repo.upserts))
	}
	in := d.repo.upserts[0]
	if in.MembershipSfID != writeMembershipID || in.ContactSfID != writeContactSfID || in.State != domain.MembershipStateInvited || in.ProjectID != writeProjectID {
		t.Errorf("upsert = %+v, want the Salesforce ids stamped on the request's project", in)
	}
	if in.CompletesSalesforceWriteRequestID != req.ID || len(d.repo.completedRequests) != 1 {
		t.Errorf("the request must be completed in the same transaction: %+v / %v", in.CompletesSalesforceWriteRequestID, d.repo.completedRequests)
	}
	if in.AdoptProjectContactID != writeProjectContactID {
		t.Errorf("the upsert must be bound to the deferred row %s, got %q", writeProjectContactID, in.AdoptProjectContactID)
	}
	if len(d.repo.steps) != 1 || d.repo.steps[0].Step != domain.OnboardingStepDatabase || d.repo.steps[0].MembershipSfID != writeMembershipID || d.repo.steps[0].EventType != domain.PortalMembershipWriteEventType {
		t.Errorf("DATABASE step = %+v", d.repo.steps)
	}
	if len(d.pub.published) != 1 || d.pub.published[0].Type != events.TypeProjectContactInvited || d.pub.published[0].EntityID != writeMembershipID {
		t.Fatalf("project_contact.invited = %+v", d.pub.published)
	}
	var p events.ProjectContactInvitedPayload
	if err := json.Unmarshal(d.pub.published[0].Payload, &p); err != nil {
		t.Fatal(err)
	}
	if p.Email != writeEmail || p.ContactSfID != writeContactSfID || p.Resend || p.ProjectKey != "ACMEPROD" || p.GivenName != "Jane" {
		t.Errorf("invited payload = %+v", p)
	}
	if len(d.requests.attempts) != 0 || len(d.requests.failed) != 0 {
		t.Errorf("a completion records no failed attempt: %v %v", d.requests.attempts, d.requests.failed)
	}
}

func TestDeferredWrite_CompleteKeepsTheStateSalesforceStored(t *testing.T) {
	d := newDeferredHarness(t)
	d.pendingRequest(t, inviteIntent())
	result := d.completedResult(t, domain.MembershipStateRegistered)
	if err := completer(d).CompleteDeferredSalesforceWrite(context.Background(), result, false); err != nil {
		t.Fatal(err)
	}
	if d.repo.upserts[0].State != domain.MembershipStateRegistered {
		t.Errorf("state = %q, want REGISTERED as Salesforce stored it", d.repo.upserts[0].State)
	}
}

func TestDeferredWrite_CompleteRetriesWhileSalesEntityIsUnreadable(t *testing.T) {
	d := newDeferredHarness(t)
	req := d.pendingRequest(t, inviteIntent())
	result := d.completedResult(t, domain.MembershipStateInvited)
	d.se.searchErr = salesforceDown()

	err := completer(d).CompleteDeferredSalesforceWrite(context.Background(), result, false)
	if !isSalesforceUnavailable(context.Background(), err) {
		t.Fatalf("want the outage returned so the consumer retries, got %v", err)
	}
	if len(d.requests.attempts) != 1 || d.requests.attempts[0] != req.ID {
		t.Errorf("attempt not recorded: %v", d.requests.attempts)
	}
	if d.requests.requests[req.ID].Status != domain.SalesforceWriteRequestPending || len(d.repo.upserts) != 0 || len(d.pub.published) != 0 {
		t.Error("nothing settles, is written or published on a retryable failure")
	}
}

func TestDeferredWrite_CompleteMarksFailedOnTheLastAttempt(t *testing.T) {
	d := newDeferredHarness(t)
	req := d.pendingRequest(t, inviteIntent())
	result := d.completedResult(t, domain.MembershipStateInvited)
	d.se.searchErr = salesforceDown()

	if err := completer(d).CompleteDeferredSalesforceWrite(context.Background(), result, true); err == nil {
		t.Fatal("the exhaustion is still reported to the consumer")
	}
	if d.requests.requests[req.ID].Status != domain.SalesforceWriteRequestFailed {
		t.Error("the final attempt must leave the request FAILED, not PENDING forever")
	}
	if reason := d.requests.failed[req.ID]; !strings.Contains(reason, writeMembershipID) {
		t.Errorf("the FAILED reason must name the Salesforce ids an operator needs: %q", reason)
	}
}

func TestDeferredWrite_CompleteFailsForGoodWhenTheRowIsGone(t *testing.T) {
	d := newDeferredHarness(t)
	req := d.pendingRequest(t, inviteIntent())
	result := d.completedResult(t, domain.MembershipStateInvited)
	d.repo.existing = nil
	if err := completer(d).CompleteDeferredSalesforceWrite(context.Background(), result, false); err != nil {
		t.Fatalf("a terminal problem is acknowledged (nil), got %v", err)
	}
	if d.requests.requests[req.ID].Status != domain.SalesforceWriteRequestFailed || len(d.pub.published) != 0 {
		t.Error("the request must be FAILED and nothing published")
	}
}

func TestDeferredWrite_FailedResultMarksTheRequestFailed(t *testing.T) {
	d := newDeferredHarness(t)
	req := d.pendingRequest(t, inviteIntent())
	err := completer(d).FailDeferredSalesforceWrite(context.Background(), events.SalesforceMembershipWriteFailedPayload{
		RequestID: req.ID, ProjectContactID: writeProjectContactID, Reason: "Account does not accept this domain", Terminal: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if d.requests.requests[req.ID].Status != domain.SalesforceWriteRequestFailed || d.requests.failed[req.ID] != "Account does not accept this domain" {
		t.Errorf("request = %+v, failed = %v", d.requests.requests[req.ID], d.requests.failed)
	}
	if len(d.repo.upserts) != 0 || len(d.pub.published) != 0 {
		t.Error("a failed result writes and publishes nothing")
	}
}

// Results are at-least-once. A settled request acknowledges any later result
// without touching Salesforce, the rows or the project topic.
func TestDeferredWrite_ResultsAreIdempotent(t *testing.T) {
	d := newDeferredHarness(t)
	req := d.pendingRequest(t, inviteIntent())
	result := d.completedResult(t, domain.MembershipStateInvited)
	req.Status = domain.SalesforceWriteRequestSucceeded
	d.requests.requests[req.ID] = req

	if err := completer(d).CompleteDeferredSalesforceWrite(context.Background(), result, false); err != nil {
		t.Fatal(err)
	}
	if err := completer(d).FailDeferredSalesforceWrite(context.Background(), events.SalesforceMembershipWriteFailedPayload{RequestID: req.ID, Reason: "late"}); err != nil {
		t.Fatal(err)
	}
	if len(d.se.contactGets) != 0 || len(d.repo.upserts) != 0 || len(d.pub.published) != 0 || d.requests.requests[req.ID].Status != domain.SalesforceWriteRequestSucceeded {
		t.Error("a settled request must do nothing: a redelivered result would otherwise send a second invitation or flip SUCCEEDED to FAILED")
	}
	// A request that no longer exists is acknowledged too.
	if err := completer(d).CompleteDeferredSalesforceWrite(context.Background(), events.SalesforceMembershipWriteCompletedPayload{RequestID: "missing", ContactSfID: "x", MembershipSfID: "y"}, false); err != nil {
		t.Errorf("missing request: want nil, got %v", err)
	}
}

func TestDeferredWrite_CompleteRetriesWhenTheLedgerIsUnreadable(t *testing.T) {
	d := newDeferredHarness(t)
	d.requests.getErr = errors.New("database unavailable")
	if err := completer(d).CompleteDeferredSalesforceWrite(context.Background(), events.SalesforceMembershipWriteCompletedPayload{RequestID: writeRequestID, ContactSfID: "x", MembershipSfID: "y"}, false); err == nil {
		t.Fatal("a ledger read failure must be retried, not acknowledged")
	}
}

// The consumer runs the invitation checks from the allowed accounts; if they
// cannot be read, the invitation is not deferred unchecked.
func TestDeferredWrite_InviteFailsWhenPartnersCannotBeRead(t *testing.T) {
	d := newDeferredHarness(t)
	d.se.searchErr = salesforceDown()
	d.partners.err = errors.New("database unavailable")
	if _, err := d.svc.Invite(context.Background(), writeProjectID, inviteReq("Portal user")); err == nil {
		t.Fatal("want an error")
	}
	if len(d.repo.upserts) != 0 || len(d.commands.published) != 0 {
		t.Error("nothing may be committed or published when the allowed accounts are unknown")
	}
}

func TestDeferredWrite_FailedResultKeepsWhetherItIsRetryable(t *testing.T) {
	for _, terminal := range []bool{true, false} {
		d := newDeferredHarness(t)
		req := d.pendingRequest(t, inviteIntent())
		if err := completer(d).FailDeferredSalesforceWrite(context.Background(), events.SalesforceMembershipWriteFailedPayload{RequestID: req.ID, Reason: "x", Terminal: terminal}); err != nil {
			t.Fatal(err)
		}
		if d.requests.retryable[req.ID] != !terminal {
			t.Errorf("terminal=%v: retryable recorded as %v", terminal, d.requests.retryable[req.ID])
		}
	}
	// Completion that could not finish on the last attempt is retryable; a
	// refusal is not.
	d := newDeferredHarness(t)
	req := d.pendingRequest(t, inviteIntent())
	result := d.completedResult(t, domain.MembershipStateInvited)
	d.se.searchErr = salesforceDown()
	_ = completer(d).CompleteDeferredSalesforceWrite(context.Background(), result, true)
	if !d.requests.retryable[req.ID] {
		t.Error("an exhausted completion must be retryable")
	}
}

// ---- a stuck deferred write (FAILED or never published) -------------------

func (d *deferredHarness) stuckRequest(t *testing.T, status domain.SalesforceWriteRequestStatus, published bool) domain.SalesforceWriteRequest {
	t.Helper()
	req := d.pendingRequest(t, inviteIntent())
	req.Status = status
	req.PublishedOn = nil
	if published {
		now := time.Now()
		req.PublishedOn = &now
	}
	d.requests.requests[req.ID] = req
	d.repo.existing.ProjectGroups = []string{"PORTAL_USER"}
	return req
}

func TestDeferredWrite_DeactivateRunsLocallyWhenTheWriteIsStuck(t *testing.T) {
	cases := map[string]struct {
		status    domain.SalesforceWriteRequestStatus
		published bool
	}{
		"request FAILED":                   {domain.SalesforceWriteRequestFailed, true},
		"request PENDING, never published": {domain.SalesforceWriteRequestPending, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			d := newDeferredHarness(t)
			req := d.stuckRequest(t, tc.status, tc.published)
			if err := d.svc.Deactivate(context.Background(), writeProjectID, writeEmail); err != nil {
				t.Fatalf("a stuck row must be removable: %v", err)
			}
			if len(d.se.contactSearchs)+len(d.se.updates)+len(d.se.createdContact) != 0 {
				t.Error("a local deactivation must not touch Salesforce")
			}
			if len(d.repo.upserts) != 1 {
				t.Fatalf("want one upsert, got %d", len(d.repo.upserts))
			}
			in := d.repo.upserts[0]
			if in.State != domain.MembershipStateDeactivated || in.MembershipSfID != "" || in.CancelsSalesforceWriteRequestID != req.ID || len(in.ProjectGroups) != 1 {
				t.Errorf("upsert = %+v, want DEACTIVATED, no ids, groups kept, request %s cancelled", in, req.ID)
			}
			if len(d.repo.steps) != 0 || len(d.pub.published) != 0 {
				t.Error("no onboarding step and no event for a local deactivation")
			}
		})
	}
}

func TestDeferredWrite_StuckWriteRefusesRoleChangeAndResendWithItsOwnCode(t *testing.T) {
	d := newDeferredHarness(t)
	d.stuckRequest(t, domain.SalesforceWriteRequestFailed, true)
	assertCode := func(t *testing.T, err error, code string) {
		t.Helper()
		var ce *apierror.ConflictError
		if !errors.As(err, &ce) || ce.Code != code {
			t.Fatalf("want 409 %s, got %v", code, err)
		}
	}
	_, err := d.svc.UpdateRoles(context.Background(), writeProjectID, writeEmail, domain.UpdateProjectMembershipRolesRequest{Roles: []string{"Admin"}})
	assertCode(t, err, apierror.CodeSalesforceSyncFailed)
	assertCode(t, d.svc.ResendInvitation(context.Background(), writeProjectID, writeEmail), apierror.CodeSalesforceSyncFailed)
}

// ---- while the write is pending ------------------------------------------

func TestDeferredWrite_OtherWritesAreRefusedWhilePending(t *testing.T) {
	d := newDeferredHarness(t)
	d.pendingRequest(t, inviteIntent())
	ctx := context.Background()

	assertPending := func(t *testing.T, err error) {
		t.Helper()
		var ce *apierror.ConflictError
		if !errors.As(err, &ce) || ce.Code != apierror.CodeSalesforceSyncPending {
			t.Fatalf("want 409 %s, got %v", apierror.CodeSalesforceSyncPending, err)
		}
	}
	_, err := d.svc.UpdateRoles(ctx, writeProjectID, writeEmail, domain.UpdateProjectMembershipRolesRequest{Roles: []string{"Admin"}})
	assertPending(t, err)
	assertPending(t, d.svc.Deactivate(ctx, writeProjectID, writeEmail))
	assertPending(t, d.svc.ResendInvitation(ctx, writeProjectID, writeEmail))
	if len(d.se.updates) != 0 || len(d.pub.published) != 0 {
		t.Error("a refused write touches nothing")
	}
	// A second invitation of the same address is the ordinary conflict.
	if _, err := d.svc.Invite(ctx, writeProjectID, inviteReq("Portal user")); err == nil {
		t.Error("want a conflict for an address already on the project")
	}
}

// A row with no id and NO pending request is the data-migration gap, not a
// pending sync: the generic refusal stays.
func TestDeferredWrite_MissingIdWithoutARequestIsTheOldRefusal(t *testing.T) {
	d := newDeferredHarness(t)
	d.repo.existing = &domain.ProjectMembershipRow{ProjectContactID: writeProjectContactID, Email: writeEmail, State: domain.MembershipStateInvited}
	_, err := d.svc.UpdateRoles(context.Background(), writeProjectID, writeEmail, domain.UpdateProjectMembershipRolesRequest{Roles: []string{"Admin"}})
	var ce *apierror.ConflictError
	if !errors.As(err, &ce) || ce.Code == apierror.CodeSalesforceSyncPending {
		t.Fatalf("want the generic missing-id 409, got %v", err)
	}
}
