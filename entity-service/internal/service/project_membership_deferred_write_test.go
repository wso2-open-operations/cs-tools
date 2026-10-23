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
	"testing"

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
	getErr    error
}

func newFakeWriteRequestRepo() *fakeWriteRequestRepo {
	return &fakeWriteRequestRepo{requests: map[string]domain.SalesforceWriteRequest{}, failed: map[string]string{}}
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

func (f *fakeWriteRequestRepo) MarkFailed(_ context.Context, id string, cause error) error {
	f.failed[id] = cause.Error()
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

// deferredHarness is the write harness with the deferred write wired: a
// request ledger and a publisher for the command topic.
type deferredHarness struct {
	*writeHarness
	requests *fakeWriteRequestRepo
	commands *fakeInvitePublisher
}

func newDeferredHarness(t *testing.T) *deferredHarness {
	t.Helper()
	h := newInternalWriteHarness(t)
	d := &deferredHarness{writeHarness: h, requests: newFakeWriteRequestRepo(), commands: &fakeInvitePublisher{}}
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
	req := domain.SalesforceWriteRequest{
		ID: writeRequestID, Operation: domain.SalesforceWriteOperationInvite,
		ProjectID: writeProjectID, ProjectContactID: writeProjectContactID,
		Email: writeEmail, Intent: raw, Status: domain.SalesforceWriteRequestPending,
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
		Invitations: v, WriteRequests: d.requests, SalesforceWritePublisher: d.commands,
	})
	got, err := d.svc.Invite(context.Background(), writeProjectID, inviteReq("Portal user"))
	if err != nil {
		t.Fatal(err)
	}
	if got.SalesforceSync != domain.SalesforceSyncPending || len(d.commands.published) != 1 {
		t.Errorf("want a deferred invitation, got %+v / %d commands", got, len(d.commands.published))
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
				WriteRequests: d.requests, SalesforceWritePublisher: d.commands,
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
	if _, err := d.svc.Invite(context.Background(), writeProjectID, inviteReq("Portal user")); !isSalesforceUnavailable(err) {
		t.Fatalf("want the outage error, got %v", err)
	}
	if len(d.repo.upserts) != 0 || len(d.commands.published) != 0 {
		t.Error("a re-invitation must not be deferred")
	}
}

// ---- the consumer finishes it --------------------------------------------

func TestDeferredWrite_ProcessWritesSalesforceAndPublishesTheInvitation(t *testing.T) {
	d := newDeferredHarness(t)
	req := d.pendingRequest(t, inviteIntent())

	if err := d.svc.(DeferredSalesforceWriter).ProcessDeferredSalesforceWrite(context.Background(), req.ID, false); err != nil {
		t.Fatalf("Salesforce is back; the write must succeed: %v", err)
	}
	// Search first, then create both records.
	if len(d.se.contactSearchs) != 1 || len(d.se.createdContact) != 1 || len(d.se.createdPC) != 1 {
		t.Errorf("salesforce calls: searches=%d contacts=%d memberships=%d", len(d.se.contactSearchs), len(d.se.createdContact), len(d.se.createdPC))
	}
	if len(d.repo.upserts) != 1 {
		t.Fatalf("want one upsert, got %d", len(d.repo.upserts))
	}
	in := d.repo.upserts[0]
	if in.MembershipSfID != writeMembershipID || in.ContactSfID != writeContactSfID || in.State != domain.MembershipStateInvited {
		t.Errorf("upsert = %+v, want the Salesforce ids stamped", in)
	}
	if in.CompletesSalesforceWriteRequestID != req.ID || len(d.repo.completedRequests) != 1 {
		t.Errorf("the request must be completed in the same transaction: %+v / %v", in.CompletesSalesforceWriteRequestID, d.repo.completedRequests)
	}
	if len(d.repo.steps) != 1 || d.repo.steps[0].Step != domain.OnboardingStepDatabase || d.repo.steps[0].MembershipSfID != writeMembershipID {
		t.Errorf("DATABASE step = %+v", d.repo.steps)
	}
	if len(d.pub.published) != 1 || d.pub.published[0].Type != events.TypeProjectContactInvited || d.pub.published[0].EntityID != writeMembershipID {
		t.Fatalf("project_contact.invited = %+v", d.pub.published)
	}
	var p events.ProjectContactInvitedPayload
	if err := json.Unmarshal(d.pub.published[0].Payload, &p); err != nil {
		t.Fatal(err)
	}
	if p.Email != writeEmail || p.ContactSfID != writeContactSfID || p.Resend || p.ProjectKey != "ACMEPROD" {
		t.Errorf("invited payload = %+v", p)
	}
	if len(d.requests.attempts) != 0 || len(d.requests.failed) != 0 {
		t.Errorf("a success records no failed attempt: %v %v", d.requests.attempts, d.requests.failed)
	}
}

func TestDeferredWrite_ProcessAdoptsARecordSalesforceAlreadyHas(t *testing.T) {
	d := newDeferredHarness(t)
	req := d.pendingRequest(t, inviteIntent())
	d.se.contact = existingSalesforceContact()
	d.se.membership = &salesentity.ProjectContact{ID: writeMembershipID, State: sampleStr(domain.MembershipStateInvited), Roles: []string{"Portal user"}}

	if err := d.svc.(DeferredSalesforceWriter).ProcessDeferredSalesforceWrite(context.Background(), req.ID, false); err != nil {
		t.Fatal(err)
	}
	if len(d.se.createdContact) != 0 || len(d.se.createdPC) != 0 {
		t.Error("an existing Salesforce record must be adopted, not duplicated")
	}
	if len(d.se.updates) != 1 {
		t.Errorf("the adopted membership gets the requested state and roles: %+v", d.se.updates)
	}
	if len(d.repo.completedRequests) != 1 || len(d.pub.published) != 1 {
		t.Errorf("completed=%v published=%d", d.repo.completedRequests, len(d.pub.published))
	}
}

func TestDeferredWrite_ProcessRetriesWhileSalesforceIsStillDown(t *testing.T) {
	d := newDeferredHarness(t)
	req := d.pendingRequest(t, inviteIntent())
	d.se.searchErr = salesforceDown()

	err := d.svc.(DeferredSalesforceWriter).ProcessDeferredSalesforceWrite(context.Background(), req.ID, false)
	if !isSalesforceUnavailable(err) {
		t.Fatalf("want the outage returned so the consumer retries, got %v", err)
	}
	if len(d.requests.attempts) != 1 || d.requests.attempts[0] != req.ID {
		t.Errorf("attempt not recorded: %v", d.requests.attempts)
	}
	if len(d.requests.failed) != 0 || d.requests.requests[req.ID].Status != domain.SalesforceWriteRequestPending {
		t.Error("the request must stay PENDING while retries remain")
	}
	if len(d.repo.upserts) != 0 || len(d.pub.published) != 0 {
		t.Error("nothing is written or published on a failed attempt")
	}
}

func TestDeferredWrite_ProcessMarksFailedOnTheLastAttempt(t *testing.T) {
	d := newDeferredHarness(t)
	req := d.pendingRequest(t, inviteIntent())
	d.se.searchErr = salesforceDown()

	err := d.svc.(DeferredSalesforceWriter).ProcessDeferredSalesforceWrite(context.Background(), req.ID, true)
	if err == nil {
		t.Fatal("the exhaustion is still reported to the consumer")
	}
	if d.requests.requests[req.ID].Status != domain.SalesforceWriteRequestFailed {
		t.Error("the final attempt must leave the request FAILED, not PENDING forever")
	}
	if len(d.requests.attempts) != 0 {
		t.Error("MarkFailed counts the attempt; RecordAttempt must not double it")
	}
}

func TestDeferredWrite_ProcessFailsForGoodOnARefusal(t *testing.T) {
	cases := map[string]func(*deferredHarness){
		"salesforce refuses the contact": func(d *deferredHarness) {
			d.se.createErr = &apierror.DownstreamError{Msg: "salesforce rejected the contact"}
		},
		"invitation validator refuses": func(d *deferredHarness) {
			d.svc = NewProjectMembershipWriteService(MembershipWriteDeps{
				Memberships: d.repo, Steps: d.steps, SalesEntity: d.se, Publisher: d.pub, Access: alwaysUnrestrictedAccess{},
				Invitations:   &fakeInvitationValidator{err: &apierror.ValidationError{Msg: "contact belongs to another account"}},
				WriteRequests: d.requests, SalesforceWritePublisher: d.commands,
			})
		},
		"membership row is gone": func(d *deferredHarness) {
			d.repo.existing = nil
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			d := newDeferredHarness(t)
			req := d.pendingRequest(t, inviteIntent())
			setup(d)
			if err := d.svc.(DeferredSalesforceWriter).ProcessDeferredSalesforceWrite(context.Background(), req.ID, false); err != nil {
				t.Fatalf("a refusal is acknowledged (nil), got %v", err)
			}
			if d.requests.requests[req.ID].Status != domain.SalesforceWriteRequestFailed {
				t.Error("the request must be FAILED")
			}
			if len(d.pub.published) != 0 || len(d.repo.completedRequests) != 0 {
				t.Error("nothing is published or completed on a refusal")
			}
		})
	}
}

func TestDeferredWrite_ProcessIsIdempotent(t *testing.T) {
	d := newDeferredHarness(t)
	req := d.pendingRequest(t, inviteIntent())
	req.Status = domain.SalesforceWriteRequestSucceeded
	d.requests.requests[req.ID] = req

	if err := d.svc.(DeferredSalesforceWriter).ProcessDeferredSalesforceWrite(context.Background(), req.ID, false); err != nil {
		t.Fatal(err)
	}
	if len(d.se.contactSearchs) != 0 || len(d.repo.upserts) != 0 || len(d.pub.published) != 0 {
		t.Error("a settled request must do nothing: a redelivered record would otherwise send a second invitation")
	}
	// A request that no longer exists is acknowledged too.
	if err := d.svc.(DeferredSalesforceWriter).ProcessDeferredSalesforceWrite(context.Background(), "missing", false); err != nil {
		t.Errorf("missing request: want nil, got %v", err)
	}
}

func TestDeferredWrite_ProcessRetriesWhenTheLedgerIsUnreadable(t *testing.T) {
	d := newDeferredHarness(t)
	d.requests.getErr = errors.New("database unavailable")
	if err := d.svc.(DeferredSalesforceWriter).ProcessDeferredSalesforceWrite(context.Background(), writeRequestID, false); err == nil {
		t.Fatal("a ledger read failure must be retried, not acknowledged")
	}
}

// ---- while the write is pending ------------------------------------------

func TestDeferredWrite_OtherWritesAreRefusedWhilePending(t *testing.T) {
	d := newDeferredHarness(t)
	d.pendingRequest(t, inviteIntent())
	ctx := context.Background()

	assertPending := func(t *testing.T, err error) {
		t.Helper()
		var ce *apierror.ConflictError
		if !errors.As(err, &ce) || ce.Code != codeSalesforceSyncPending {
			t.Fatalf("want 409 %s, got %v", codeSalesforceSyncPending, err)
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
	if !errors.As(err, &ce) || ce.Code == codeSalesforceSyncPending {
		t.Fatalf("want the generic missing-id 409, got %v", err)
	}
}
