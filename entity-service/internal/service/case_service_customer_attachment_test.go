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
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// These tests pin caseService.applyCustomerAttachmentStateTransition, the
// attachment counterpart of the customer-reply rule pinned in
// case_service_customer_reply_test.go: a customer's attachment on a record
// that is Awaiting Info or Solution Proposed moves it to Waiting on WSO2
// through the service's own UpdateCase. The transition fires only when the
// attachment is truly stored: on a complete create, and on confirm for the
// two-step (pending) upload, never for the pending placeholder itself.

const customerAttachmentStorageKey = "uploads/log.txt"

func customerAttachmentRequest(status domain.AttachmentStatus) domain.CreateAttachmentRequest {
	key := customerAttachmentStorageKey
	return domain.CreateAttachmentRequest{
		ReferenceID:   testDeploymentUUID,
		ReferenceType: domain.ReferenceTypeCase,
		Name:          "log.txt",
		Type:          "text/plain",
		StorageKey:    &key,
		SizeBytes:     5,
		Status:        status,
	}
}

// withAttachmentStubs wires the attachment repository methods onto the
// customer-reply fixture. The stored status is whatever the request asked
// for; confirm always yields complete.
func withAttachmentStubs(f *customerReplyFixture) {
	uploader := "user-customer"
	f.repo.createCaseAttachment = func(_ context.Context, req domain.CreateAttachmentRequest) (domain.Attachment, error) {
		return domain.Attachment{ID: "att-1", ReferenceID: req.ReferenceID, ReferenceType: req.ReferenceType, SizeBytes: req.SizeBytes, Status: req.Status, StorageKey: req.StorageKey, CreatedOn: time.Now()}, nil
	}
	f.repo.getCaseAttachmentByID = func(context.Context, string) (domain.Attachment, error) {
		return domain.Attachment{ID: "att-1", ReferenceID: testDeploymentUUID, ReferenceType: domain.ReferenceTypeCase, Status: domain.AttachmentStatusPending, CreatedBy: &domain.UserReference{ID: &uploader}}, nil
	}
	f.repo.confirmCaseAttachment = func(context.Context, string) (domain.Attachment, error) {
		return domain.Attachment{ID: "att-1", ReferenceID: testDeploymentUUID, ReferenceType: domain.ReferenceTypeCase, Status: domain.AttachmentStatusComplete, CreatedOn: time.Now()}, nil
	}
}

func TestCaseService_CustomerAttachmentMovesToWaitingOnWSO2(t *testing.T) {
	states := []struct {
		state domain.CaseState
		moves bool
	}{
		{domain.CaseStateAwaitingInfo, true},
		{domain.CaseStateSolutionProposed, true},
		{domain.CaseStateOpen, false},
		{domain.CaseStateWorkInProgress, false},
		{domain.CaseStateWaitingOnWSO2, false},
		{domain.CaseStateClosed, false},
	}
	flows := []struct {
		name string
		run  func(svc CaseService, ctx context.Context) error
	}{
		{"complete create", func(svc CaseService, ctx context.Context) error {
			_, err := svc.CreateCaseAttachment(ctx, customerAttachmentRequest(""))
			return err
		}},
		{"confirm of a pending upload", func(svc CaseService, ctx context.Context) error {
			_, err := svc.ConfirmCaseAttachment(ctx, "33333333-3333-3333-3333-333333333333")
			return err
		}},
	}
	for _, flow := range flows {
		for _, tc := range states {
			t.Run(flow.name+"/"+string(tc.state), func(t *testing.T) {
				f := newCustomerReplyFixture("case", tc.state)
				withAttachmentStubs(f)
				svc := NewCaseService(f.repo, f.userRepo, nil, customerReplyExternalAccess, nil)
				ctx := contextWithUserIDToken(fakeJWTWithEmail(t, customerReplyCustomerEmail))

				if err := flow.run(svc, ctx); err != nil {
					t.Fatalf("%s: %v", flow.name, err)
				}
				moves := f.movesTo()
				if !tc.moves {
					if len(moves) != 0 {
						t.Fatalf("UpdateCase asked for %v from %s, want no move", moves, tc.state)
					}
					return
				}
				if len(moves) != 1 || moves[0] != domain.CaseStateWaitingOnWSO2 {
					t.Fatalf("UpdateCase asked for %v, want exactly one move to waiting_on_wso2", moves)
				}
			})
		}
	}
}

// TestCaseService_PendingAttachmentDoesNotMove: a pending row is a
// placeholder for an upload that may never finish, so creating one leaves the
// case alone; only confirming it moves the case.
func TestCaseService_PendingAttachmentDoesNotMove(t *testing.T) {
	f := newCustomerReplyFixture("case", domain.CaseStateAwaitingInfo)
	withAttachmentStubs(f)
	svc := NewCaseService(f.repo, f.userRepo, nil, customerReplyExternalAccess, nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, customerReplyCustomerEmail))

	if _, err := svc.CreateCaseAttachment(ctx, customerAttachmentRequest(domain.AttachmentStatusPending)); err != nil {
		t.Fatalf("CreateCaseAttachment: %v", err)
	}
	if moves := f.movesTo(); len(moves) != 0 {
		t.Fatalf("UpdateCase asked for %v on a pending create, want no move", moves)
	}
}

// TestCaseService_StaffAttachmentDoesNotMove: an internal caller is never a
// customer, so a staff upload leaves an Awaiting Info case alone.
func TestCaseService_StaffAttachmentDoesNotMove(t *testing.T) {
	f := newCustomerReplyFixture("case", domain.CaseStateAwaitingInfo)
	withAttachmentStubs(f)
	for name, access := range map[string]AccessService{
		"unrestricted":    alwaysUnrestrictedAccess{},
		"internal access": stubAccess{scope: AccessScope{HasInternalAccess: true, ViewerEmail: "agent.smith@example.com"}},
		"unresolved":      erroringAccess{err: errors.New("no verified identity")},
	} {
		t.Run(name, func(t *testing.T) {
			svc := NewCaseService(f.repo, f.userRepo, nil, access, nil)
			ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "agent.smith@example.com"))
			if _, err := svc.CreateCaseAttachment(ctx, customerAttachmentRequest("")); err != nil {
				t.Fatalf("CreateCaseAttachment: %v", err)
			}
			if moves := f.movesTo(); len(moves) != 0 {
				t.Fatalf("UpdateCase asked for %v for a non-customer upload, want no move", moves)
			}
		})
	}
}

// TestCaseService_CustomerAttachmentMoveFailureNeverFailsTheUpload: the
// attachment is committed before the rule runs, so a failed state read or move
// is logged at ERROR with the case id and the upload still succeeds.
func TestCaseService_CustomerAttachmentMoveFailureNeverFailsTheUpload(t *testing.T) {
	cases := []struct {
		name    string
		arrange func(f *customerReplyFixture)
		wantLog string
	}{
		{"UpdateCase fails", func(f *customerReplyFixture) {
			f.repo.updateCase = func(context.Context, domain.UpdateCaseRequest) (domain.Case, *domain.CaseSeverity, error) {
				return domain.Case{}, nil, errors.New("boom")
			}
		}, "move case to waiting on wso2 after customer attachment failed"},
		{"reading the current state fails", func(f *customerReplyFixture) {
			f.repo.getCaseByID = func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
				return domain.CaseView{}, errors.New("boom")
			}
		}, "customer attachment state transition not evaluated, get case failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newCustomerReplyFixture("case", domain.CaseStateAwaitingInfo)
			withAttachmentStubs(f)
			tc.arrange(f)
			logs := captureSlog(t)
			svc := NewCaseService(f.repo, f.userRepo, nil, customerReplyExternalAccess, nil)
			ctx := contextWithUserIDToken(fakeJWTWithEmail(t, customerReplyCustomerEmail))

			resp, err := svc.CreateCaseAttachment(ctx, customerAttachmentRequest(""))
			if err != nil {
				t.Fatalf("CreateCaseAttachment returned %v, want the upload to succeed regardless of the move", err)
			}
			if resp.Attachment.ID != "att-1" {
				t.Errorf("attachment id = %q, want att-1", resp.Attachment.ID)
			}
			out := logs.String()
			if !strings.Contains(out, "level=ERROR") || !strings.Contains(out, tc.wantLog) || !strings.Contains(out, "caseId="+testDeploymentUUID) {
				t.Errorf("want an ERROR log line %q carrying caseId=%s, got:\n%s", tc.wantLog, testDeploymentUUID, out)
			}
		})
	}
}

// TestCaseAttachmentDualWriteService_CustomerAttachmentMovesViaPostgres: under
// dual-write the mirror is the bare upload (no ServiceNow-side move), and
// Postgres decides and performs the one state move.
func TestCaseAttachmentDualWriteService_CustomerAttachmentMovesViaPostgres(t *testing.T) {
	f := newCustomerReplyFixture("case", domain.CaseStateAwaitingInfo)
	f.repo.createCaseAttachmentFromSN = func(_ context.Context, req domain.CreateAttachmentRequest, id string, sizeBytes int, uploadedBy string) (domain.Attachment, error) {
		return domain.Attachment{ID: id, SizeBytes: sizeBytes, Status: domain.AttachmentStatusComplete, CreatedOn: time.Now()}, nil
	}
	mirror := &stubCaseAttachmentSNMirror{createCaseAttachment: func(context.Context, domain.CreateAttachmentRequest) (domain.CreateAttachmentResponse, error) {
		return domain.CreateAttachmentResponse{Message: "ok", Attachment: domain.AttachmentDetail{ID: "44444444-4444-4444-4444-444444444444", SizeBytes: 5}}, nil
	}}
	base := NewCaseService(f.repo, f.userRepo, nil, customerReplyExternalAccess, nil)
	svc := NewCaseAttachmentDualWriteService(base, mirror, nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, customerReplyCustomerEmail))

	req := customerAttachmentRequest("")
	req.File = "data:text/plain;base64,aGVsbG8="
	if _, err := svc.CreateCaseAttachment(ctx, req); err != nil {
		t.Fatalf("CreateCaseAttachment: %v", err)
	}
	if moves := f.movesTo(); len(moves) != 1 || moves[0] != domain.CaseStateWaitingOnWSO2 {
		t.Fatalf("UpdateCase asked for %v, want exactly one move to waiting_on_wso2", moves)
	}
}
