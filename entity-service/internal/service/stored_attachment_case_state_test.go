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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// These tests pin the stored attachment confirm path's use of the case
// service's customer-attachment rule: a customer's work item attachment on a
// case that is Awaiting Info or Solution Proposed moves it to Waiting on WSO2
// once the upload is COMPLETE. They run the real case service over the
// recording repository of the customer-reply tests, so what is asserted is
// what the rule asks UpdateCase for.

// storedConfirmWithCase confirms a pending upload of kind, as the customer,
// against a real case service whose record is in state.
func storedConfirmWithCase(t *testing.T, kind domain.StoredAttachmentKind, state domain.CaseState, access AccessService) (*customerReplyFixture, error) {
	t.Helper()
	f := newCustomerReplyFixture("case", state)
	repo := newFakeStoredRepo()
	pendingUpload(repo, 500)
	svc := newStoredSvc(repo)
	WithStoredAttachmentCaseStates(svc, NewCaseService(f.repo, f.userRepo, nil, access, nil))
	_, err := svc.ConfirmUpload(saCtx(t, saEmail), domain.ConfirmAttachmentUploadRequest{Kind: kind, ID: saAttID, SizeBytes: int64Ptr(500)})
	return f, err
}

func TestBackend_CustomerWorkItemUploadMovesCaseToWaitingOnWSO2(t *testing.T) {
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
	for _, tc := range states {
		t.Run(string(tc.state), func(t *testing.T) {
			f, err := storedConfirmWithCase(t, domain.StoredAttachmentWorkItem, tc.state, customerReplyExternalAccess)
			if err != nil {
				t.Fatalf("ConfirmUpload: %v", err)
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

func TestBackend_StaffWorkItemUploadDoesNotMoveCase(t *testing.T) {
	for name, access := range map[string]AccessService{
		"unrestricted":    alwaysUnrestrictedAccess{},
		"internal access": stubAccess{scope: AccessScope{HasInternalAccess: true, ViewerEmail: saEmail}},
		"unresolved":      erroringAccess{err: errors.New("no verified identity")},
	} {
		t.Run(name, func(t *testing.T) {
			f, err := storedConfirmWithCase(t, domain.StoredAttachmentWorkItem, domain.CaseStateAwaitingInfo, access)
			if err != nil {
				t.Fatalf("ConfirmUpload: %v", err)
			}
			if moves := f.movesTo(); len(moves) != 0 {
				t.Fatalf("UpdateCase asked for %v for a non-customer upload, want no move", moves)
			}
		})
	}
}

// A comment image or a deployment attachment is not an answer to WSO2's
// request on the case.
func TestBackend_CommentAndDeploymentUploadsDoNotMoveCase(t *testing.T) {
	for _, kind := range []domain.StoredAttachmentKind{domain.StoredAttachmentComment, domain.StoredAttachmentDeployment} {
		t.Run(string(kind), func(t *testing.T) {
			f := newCustomerReplyFixture("case", domain.CaseStateAwaitingInfo)
			repo := newFakeStoredRepo()
			ct := "image/png"
			size := int64(50)
			repo.rows[saAttID] = domain.StoredAttachmentRow{
				ID: saAttID, Number: "WIA0000042", Name: "a.png", ContentType: &ct, SizeBytes: &size,
				Status: "PENDING", CreatedBy: saEmail, ParentID: saParentID,
			}
			svc := newStoredSvc(repo)
			WithStoredAttachmentCaseStates(svc, NewCaseService(f.repo, f.userRepo, nil, customerReplyExternalAccess, nil))
			_, err := svc.ConfirmUpload(saCtx(t, saEmail), domain.ConfirmAttachmentUploadRequest{Kind: kind, ID: saAttID, SizeBytes: &size, ContentType: ct})
			if err != nil {
				t.Fatalf("ConfirmUpload: %v", err)
			}
			if moves := f.movesTo(); len(moves) != 0 {
				t.Fatalf("UpdateCase asked for %v for a %s upload, want no move", moves, kind)
			}
		})
	}
}

// A repeated confirm answers from the COMPLETE row and must not replay the move.
func TestBackend_RepeatedConfirmDoesNotMoveCase(t *testing.T) {
	f := newCustomerReplyFixture("case", domain.CaseStateAwaitingInfo)
	repo := newFakeStoredRepo()
	key := workItemKey
	repo.rows[saAttID] = completeRow("COMPLETE", &key)
	svc := newStoredSvc(repo)
	WithStoredAttachmentCaseStates(svc, NewCaseService(f.repo, f.userRepo, nil, customerReplyExternalAccess, nil))
	if _, err := svc.ConfirmUpload(saCtx(t, saEmail), domain.ConfirmAttachmentUploadRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID}); err != nil {
		t.Fatalf("ConfirmUpload: %v", err)
	}
	if moves := f.movesTo(); len(moves) != 0 {
		t.Fatalf("UpdateCase asked for %v on a repeated confirm, want no move", moves)
	}
}

// The file is stored before the rule runs, so a failed read or move is logged
// and the confirm still succeeds.
func TestBackend_FailedMoveNeverFailsTheConfirm(t *testing.T) {
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
			tc.arrange(f)
			logs := captureSlog(t)
			repo := newFakeStoredRepo()
			pendingUpload(repo, 500)
			svc := newStoredSvc(repo)
			WithStoredAttachmentCaseStates(svc, NewCaseService(f.repo, f.userRepo, nil, customerReplyExternalAccess, nil))
			resp, err := svc.ConfirmUpload(saCtx(t, saEmail), domain.ConfirmAttachmentUploadRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID, SizeBytes: int64Ptr(500)})
			if err != nil {
				t.Fatalf("ConfirmUpload returned %v, want success regardless of the move", err)
			}
			if resp.ID != saAttID {
				t.Errorf("attachment id = %q, want %q", resp.ID, saAttID)
			}
			if _, ok := repo.completed[saAttID]; !ok {
				t.Error("upload was not marked complete")
			}
			out := logs.String()
			if !strings.Contains(out, "level=ERROR") || !strings.Contains(out, tc.wantLog) || !strings.Contains(out, "caseId="+saParentID) {
				t.Errorf("want an ERROR log line %q carrying caseId=%s, got:\n%s", tc.wantLog, saParentID, out)
			}
		})
	}
}
