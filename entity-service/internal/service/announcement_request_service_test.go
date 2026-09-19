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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// fakeAnnouncementRequestRepo captures the requests it was called with, and
// returns getResult/getErr from Get — every state-machine test in this file
// configures getResult.State to whatever the transition under test needs to
// see as "current."
type fakeAnnouncementRequestRepo struct {
	getResult domain.AnnouncementRequest
	getErr    error

	gotCreateReq domain.CreateAnnouncementRequestRequest
	gotSearchReq domain.SearchAnnouncementRequestsRequest
	searchResult []domain.AnnouncementRequest
	searchTotal  int

	gotUpdateID            string
	gotUpdateExpectedState domain.AnnouncementRequestState
	gotUpdateReq           domain.UpdateAnnouncementRequestRequest

	gotRevertID  string
	gotRevertReq domain.UpdateAnnouncementRequestRequest
	revertCalled bool

	gotDryRunID  string
	gotDryRunReq domain.RecordAnnouncementDryRunRequest

	gotSubmitID  string
	gotSubmitReq domain.SubmitAnnouncementRequestRequest

	gotApproveID      string
	gotApproveActorID string

	gotPublishID      string
	gotPublishActorID string
}

func (f *fakeAnnouncementRequestRepo) Create(_ context.Context, req domain.CreateAnnouncementRequestRequest) (domain.AnnouncementRequest, error) {
	f.gotCreateReq = req
	return domain.AnnouncementRequest{State: domain.AnnouncementRequestStateDraft, Kind: req.Kind, CreatedBy: req.CreatedBy}, nil
}

func (f *fakeAnnouncementRequestRepo) Get(_ context.Context, _ string) (domain.AnnouncementRequest, error) {
	return f.getResult, f.getErr
}

func (f *fakeAnnouncementRequestRepo) Search(_ context.Context, req domain.SearchAnnouncementRequestsRequest) ([]domain.AnnouncementRequest, int, error) {
	f.gotSearchReq = req
	return f.searchResult, f.searchTotal, nil
}

func (f *fakeAnnouncementRequestRepo) Update(_ context.Context, id string, expectedState domain.AnnouncementRequestState, req domain.UpdateAnnouncementRequestRequest) (domain.AnnouncementRequest, error) {
	f.gotUpdateID = id
	f.gotUpdateExpectedState = expectedState
	f.gotUpdateReq = req
	return domain.AnnouncementRequest{ID: id, State: f.getResult.State}, nil
}

func (f *fakeAnnouncementRequestRepo) RecordDryRun(_ context.Context, id string, req domain.RecordAnnouncementDryRunRequest) (domain.AnnouncementRequest, error) {
	f.gotDryRunID = id
	f.gotDryRunReq = req
	return domain.AnnouncementRequest{ID: id, State: domain.AnnouncementRequestStateDraft, DryRunCaseID: &req.CaseID}, nil
}

func (f *fakeAnnouncementRequestRepo) Submit(_ context.Context, id string, req domain.SubmitAnnouncementRequestRequest) (domain.AnnouncementRequest, error) {
	f.gotSubmitID = id
	f.gotSubmitReq = req
	return domain.AnnouncementRequest{ID: id, State: domain.AnnouncementRequestStatePendingApproval, ResolvedProjectIDs: req.ResolvedProjectIDs}, nil
}

func (f *fakeAnnouncementRequestRepo) Approve(_ context.Context, id, actorID string) (domain.AnnouncementRequest, error) {
	f.gotApproveID = id
	f.gotApproveActorID = actorID
	return domain.AnnouncementRequest{ID: id, State: domain.AnnouncementRequestStateApproved}, nil
}

func (f *fakeAnnouncementRequestRepo) RevertToDraft(_ context.Context, id string, req domain.UpdateAnnouncementRequestRequest) (domain.AnnouncementRequest, error) {
	f.revertCalled = true
	f.gotRevertID = id
	f.gotRevertReq = req
	return domain.AnnouncementRequest{ID: id, State: domain.AnnouncementRequestStateDraft}, nil
}

func (f *fakeAnnouncementRequestRepo) MarkPublished(_ context.Context, id, actorID string) (domain.AnnouncementRequest, error) {
	f.gotPublishID = id
	f.gotPublishActorID = actorID
	return domain.AnnouncementRequest{ID: id, State: domain.AnnouncementRequestStatePublished}, nil
}

func TestAnnouncementRequestService_CreateDraft(t *testing.T) {
	t.Run("forwards a valid request", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{}
		svc := NewAnnouncementRequestService(repo)

		_, err := svc.CreateDraft(context.Background(), domain.CreateAnnouncementRequestRequest{
			Kind: domain.AnnouncementRequestKindCustomer, CreatedBy: "user-1",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.gotCreateReq.CreatedBy != "user-1" {
			t.Fatalf("expected createdBy forwarded, got %+v", repo.gotCreateReq)
		}
	})

	t.Run("rejects an invalid kind", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{}
		svc := NewAnnouncementRequestService(repo)
		_, err := svc.CreateDraft(context.Background(), domain.CreateAnnouncementRequestRequest{
			Kind: "bogus", CreatedBy: "user-1",
		})
		if err == nil {
			t.Fatal("expected validation error for invalid kind, got nil")
		}
	})

	t.Run("rejects a missing createdBy", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{}
		svc := NewAnnouncementRequestService(repo)
		_, err := svc.CreateDraft(context.Background(), domain.CreateAnnouncementRequestRequest{
			Kind: domain.AnnouncementRequestKindEOL,
		})
		if err == nil {
			t.Fatal("expected validation error for missing createdBy, got nil")
		}
	})
}

func TestAnnouncementRequestService_Submit(t *testing.T) {
	t.Run("rejects when no dry run has been recorded", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{
			State: domain.AnnouncementRequestStateDraft, DryRunCaseID: nil,
		}}
		svc := NewAnnouncementRequestService(repo)

		_, err := svc.Submit(context.Background(), "req-1", domain.SubmitAnnouncementRequestRequest{
			ResolvedProjectIDs: []string{"proj-1"}, ActorID: "user-1",
		})
		if err == nil {
			t.Fatal("expected a conflict error when no dry run has been recorded, got nil")
		}
	})

	t.Run("accepts once a dry run has been recorded", func(t *testing.T) {
		caseID := "case-123"
		repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{
			State: domain.AnnouncementRequestStateDraft, DryRunCaseID: &caseID,
		}}
		svc := NewAnnouncementRequestService(repo)

		_, err := svc.Submit(context.Background(), "req-1", domain.SubmitAnnouncementRequestRequest{
			ResolvedProjectIDs: []string{"proj-1", "proj-2"}, ActorID: "user-1",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(repo.gotSubmitReq.ResolvedProjectIDs) != 2 {
			t.Fatalf("expected the resolved project ids forwarded unchanged, got %+v", repo.gotSubmitReq.ResolvedProjectIDs)
		}
	})

	t.Run("rejects submitting from any state other than draft", func(t *testing.T) {
		caseID := "case-123"
		for _, state := range []domain.AnnouncementRequestState{
			domain.AnnouncementRequestStatePendingApproval,
			domain.AnnouncementRequestStateApproved,
			domain.AnnouncementRequestStatePublished,
		} {
			t.Run(string(state), func(t *testing.T) {
				repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{
					State: state, DryRunCaseID: &caseID,
				}}
				svc := NewAnnouncementRequestService(repo)
				_, err := svc.Submit(context.Background(), "req-1", domain.SubmitAnnouncementRequestRequest{
					ResolvedProjectIDs: []string{"proj-1"}, ActorID: "user-1",
				})
				if err == nil {
					t.Fatalf("expected a conflict error submitting from state %q, got nil", state)
				}
			})
		}
	})

	t.Run("rejects an empty resolved project id list", func(t *testing.T) {
		caseID := "case-123"
		repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{
			State: domain.AnnouncementRequestStateDraft, DryRunCaseID: &caseID,
		}}
		svc := NewAnnouncementRequestService(repo)
		_, err := svc.Submit(context.Background(), "req-1", domain.SubmitAnnouncementRequestRequest{
			ResolvedProjectIDs: nil, ActorID: "user-1",
		})
		if err == nil {
			t.Fatal("expected a validation error for an empty resolved project id list, got nil")
		}
	})

	t.Run("rejects a missing actorId", func(t *testing.T) {
		caseID := "case-123"
		repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{
			State: domain.AnnouncementRequestStateDraft, DryRunCaseID: &caseID,
		}}
		svc := NewAnnouncementRequestService(repo)
		_, err := svc.Submit(context.Background(), "req-1", domain.SubmitAnnouncementRequestRequest{
			ResolvedProjectIDs: []string{"proj-1"},
		})
		if err == nil {
			t.Fatal("expected a validation error for a missing actorId, got nil")
		}
	})
}

func TestAnnouncementRequestService_Approve(t *testing.T) {
	t.Run("accepts from pending_approval", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{
			State: domain.AnnouncementRequestStatePendingApproval,
		}}
		svc := NewAnnouncementRequestService(repo)
		_, err := svc.Approve(context.Background(), "req-1", "user-2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.gotApproveActorID != "user-2" {
			t.Fatalf("expected actorId forwarded, got %q", repo.gotApproveActorID)
		}
	})

	t.Run("rejects from any state other than pending_approval", func(t *testing.T) {
		for _, state := range []domain.AnnouncementRequestState{
			domain.AnnouncementRequestStateDraft,
			domain.AnnouncementRequestStateApproved,
			domain.AnnouncementRequestStatePublished,
		} {
			t.Run(string(state), func(t *testing.T) {
				repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{State: state}}
				svc := NewAnnouncementRequestService(repo)
				if _, err := svc.Approve(context.Background(), "req-1", "user-2"); err == nil {
					t.Fatalf("expected a conflict error approving from state %q, got nil", state)
				}
			})
		}
	})
}

func TestAnnouncementRequestService_MarkPublished(t *testing.T) {
	t.Run("accepts from approved", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{
			State: domain.AnnouncementRequestStateApproved,
		}}
		svc := NewAnnouncementRequestService(repo)
		_, err := svc.MarkPublished(context.Background(), "req-1", "user-3")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.gotPublishActorID != "user-3" {
			t.Fatalf("expected actorId forwarded, got %q", repo.gotPublishActorID)
		}
	})

	t.Run("rejects from any state other than approved", func(t *testing.T) {
		for _, state := range []domain.AnnouncementRequestState{
			domain.AnnouncementRequestStateDraft,
			domain.AnnouncementRequestStatePendingApproval,
			domain.AnnouncementRequestStatePublished,
		} {
			t.Run(string(state), func(t *testing.T) {
				repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{State: state}}
				svc := NewAnnouncementRequestService(repo)
				if _, err := svc.MarkPublished(context.Background(), "req-1", "user-3"); err == nil {
					t.Fatalf("expected a conflict error publishing from state %q, got nil", state)
				}
			})
		}
	})
}

func TestAnnouncementRequestService_Update(t *testing.T) {
	subject := "Updated subject"

	t.Run("draft: plain update, no revert", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{State: domain.AnnouncementRequestStateDraft}}
		svc := NewAnnouncementRequestService(repo)

		got, err := svc.Update(context.Background(), "req-1", domain.UpdateAnnouncementRequestRequest{Subject: &subject, ActorID: "user-1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.revertCalled {
			t.Fatal("expected a plain Update, not RevertToDraft, when editing a draft")
		}
		if repo.gotUpdateID != "req-1" {
			t.Fatalf("expected Update called with the request id, got %q", repo.gotUpdateID)
		}
		if repo.gotUpdateExpectedState != domain.AnnouncementRequestStateDraft {
			t.Fatalf("expected Update's atomic precondition to be draft, got %q", repo.gotUpdateExpectedState)
		}
		if got.State != domain.AnnouncementRequestStateDraft {
			t.Fatalf("expected state to stay draft, got %q", got.State)
		}
	})

	t.Run("pending_approval: edit reverts to draft as one atomic call", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{State: domain.AnnouncementRequestStatePendingApproval}}
		svc := NewAnnouncementRequestService(repo)

		got, err := svc.Update(context.Background(), "req-1", domain.UpdateAnnouncementRequestRequest{Subject: &subject, ActorID: "user-1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !repo.revertCalled {
			t.Fatal("expected RevertToDraft to be called when editing a pending_approval request")
		}
		if repo.gotRevertReq.Subject == nil || *repo.gotRevertReq.Subject != subject {
			t.Fatalf("expected the edited subject forwarded to RevertToDraft, got %+v", repo.gotRevertReq)
		}
		if got.State != domain.AnnouncementRequestStateDraft {
			t.Fatalf("expected state to become draft, got %q", got.State)
		}
	})

	t.Run("approved: in-place update, no revert, no state change", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{State: domain.AnnouncementRequestStateApproved}}
		svc := NewAnnouncementRequestService(repo)

		got, err := svc.Update(context.Background(), "req-1", domain.UpdateAnnouncementRequestRequest{Subject: &subject, ActorID: "user-1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.revertCalled {
			t.Fatal("expected an in-place Update, not RevertToDraft, when editing an approved request")
		}
		if repo.gotUpdateExpectedState != domain.AnnouncementRequestStateApproved {
			t.Fatalf("expected Update's atomic precondition to be approved, got %q", repo.gotUpdateExpectedState)
		}
		if got.State != domain.AnnouncementRequestStateApproved {
			t.Fatalf("expected state to stay approved, got %q", got.State)
		}
	})

	t.Run("approved: rejects an audience change — the approved snapshot is frozen", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{State: domain.AnnouncementRequestStateApproved}}
		svc := NewAnnouncementRequestService(repo)

		_, err := svc.Update(context.Background(), "req-1", domain.UpdateAnnouncementRequestRequest{
			AudienceDefinition: []byte(`{"scope":"all"}`), ActorID: "user-1",
		})
		if err == nil {
			t.Fatal("expected an error rejecting an audience change on an approved request, got nil")
		}
		if repo.revertCalled || repo.gotUpdateID != "" {
			t.Fatal("expected the repository never called when the audience-change rejection fires")
		}
	})

	t.Run("published: rejected outright", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{State: domain.AnnouncementRequestStatePublished}}
		svc := NewAnnouncementRequestService(repo)

		_, err := svc.Update(context.Background(), "req-1", domain.UpdateAnnouncementRequestRequest{Subject: &subject, ActorID: "user-1"})
		if err == nil {
			t.Fatal("expected an error editing a published request, got nil")
		}
	})

	t.Run("rejects a missing actorId", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{State: domain.AnnouncementRequestStateDraft}}
		svc := NewAnnouncementRequestService(repo)
		_, err := svc.Update(context.Background(), "req-1", domain.UpdateAnnouncementRequestRequest{Subject: &subject})
		if err == nil {
			t.Fatal("expected a validation error for a missing actorId, got nil")
		}
	})
}

func TestAnnouncementRequestService_RecordDryRun(t *testing.T) {
	t.Run("accepts from draft", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{State: domain.AnnouncementRequestStateDraft}}
		svc := NewAnnouncementRequestService(repo)

		_, err := svc.RecordDryRun(context.Background(), "req-1", domain.RecordAnnouncementDryRunRequest{CaseID: "case-1", ActorID: "user-1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.gotDryRunReq.CaseID != "case-1" {
			t.Fatalf("expected caseId forwarded, got %+v", repo.gotDryRunReq)
		}
	})

	t.Run("rejects from any state other than draft", func(t *testing.T) {
		for _, state := range []domain.AnnouncementRequestState{
			domain.AnnouncementRequestStatePendingApproval,
			domain.AnnouncementRequestStateApproved,
			domain.AnnouncementRequestStatePublished,
		} {
			t.Run(string(state), func(t *testing.T) {
				repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{State: state}}
				svc := NewAnnouncementRequestService(repo)
				_, err := svc.RecordDryRun(context.Background(), "req-1", domain.RecordAnnouncementDryRunRequest{CaseID: "case-1", ActorID: "user-1"})
				if err == nil {
					t.Fatalf("expected a conflict error recording a dry run from state %q, got nil", state)
				}
			})
		}
	})

	t.Run("rejects a missing caseId", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{State: domain.AnnouncementRequestStateDraft}}
		svc := NewAnnouncementRequestService(repo)
		_, err := svc.RecordDryRun(context.Background(), "req-1", domain.RecordAnnouncementDryRunRequest{ActorID: "user-1"})
		if err == nil {
			t.Fatal("expected a validation error for a missing caseId, got nil")
		}
	})
}

func TestAnnouncementRequestService_Search(t *testing.T) {
	t.Run("normalizes pagination defaults", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{}
		svc := NewAnnouncementRequestService(repo)

		_, err := svc.Search(context.Background(), domain.SearchAnnouncementRequestsRequest{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.gotSearchReq.Pagination.Limit != defaultLimit {
			t.Fatalf("expected default limit %d applied, got %d", defaultLimit, repo.gotSearchReq.Pagination.Limit)
		}
	})

	t.Run("rejects an invalid state filter", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{}
		svc := NewAnnouncementRequestService(repo)
		bogus := domain.AnnouncementRequestState("bogus")
		_, err := svc.Search(context.Background(), domain.SearchAnnouncementRequestsRequest{State: &bogus})
		if err == nil {
			t.Fatal("expected a validation error for an invalid state filter, got nil")
		}
	})

	t.Run("computes hasMore from total/offset/returned count", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{
			searchResult: []domain.AnnouncementRequest{{ID: "a"}, {ID: "b"}},
			searchTotal:  5,
		}
		svc := NewAnnouncementRequestService(repo)
		resp, err := svc.Search(context.Background(), domain.SearchAnnouncementRequestsRequest{
			Pagination: domain.Pagination{Limit: 2, Offset: 0},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.HasMore {
			t.Fatal("expected hasMore=true (2 returned + 0 offset < 5 total)")
		}
		if resp.Total != 5 {
			t.Fatalf("expected total forwarded unchanged, got %d", resp.Total)
		}
	})
}
