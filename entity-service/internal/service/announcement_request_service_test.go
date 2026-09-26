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
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
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

	gotApproveID         string
	gotApproveActorID    string
	gotApproveActorEmail string

	gotPublishID         string
	gotPublishActorID    string
	gotPublishActorEmail string
	gotPublishCaseIDs    []string

	gotScheduleID           string
	gotScheduleScheduledFor *time.Time
	scheduleCalled          bool

	gotCreateUpdateReqID          string
	gotCreateUpdateContent        string
	gotCreateUpdateCreatedBy      string
	gotCreateUpdateCreatedByEmail string
	createUpdateResult            domain.AnnouncementRequestUpdate

	gotListUpdatesID  string
	listUpdatesResult []domain.AnnouncementRequestUpdate

	gotUpsertDeliveriesID  string
	gotUpsertDeliveriesReq []domain.RecordAnnouncementRequestDeliveryInput
	upsertDeliveriesCalls  int
	upsertDeliveriesResult []domain.AnnouncementRequestDelivery
	upsertDeliveriesErr    error

	gotListDeliveriesID  string
	listDeliveriesResult []domain.AnnouncementRequestDelivery

	gotClaimID    string
	claimCalled   bool
	claimResult   domain.AnnouncementRequest
	claimErr      error
	gotReleaseID  string
	releaseCalled int
	releaseErr    error

	// mu guards every field UpsertDeliveries touches -- AutoPublish now
	// calls it concurrently, one goroutine per project (see its own doc
	// comment), so this fake needs to be as safe for concurrent use as the
	// real repository already is.
	mu sync.Mutex
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

func (f *fakeAnnouncementRequestRepo) Approve(_ context.Context, id, actorID, actorEmail string) (domain.AnnouncementRequest, error) {
	f.gotApproveID = id
	f.gotApproveActorID = actorID
	f.gotApproveActorEmail = actorEmail
	return domain.AnnouncementRequest{ID: id, State: domain.AnnouncementRequestStateApproved}, nil
}

func (f *fakeAnnouncementRequestRepo) RevertToDraft(_ context.Context, id string, req domain.UpdateAnnouncementRequestRequest) (domain.AnnouncementRequest, error) {
	f.revertCalled = true
	f.gotRevertID = id
	f.gotRevertReq = req
	return domain.AnnouncementRequest{ID: id, State: domain.AnnouncementRequestStateDraft}, nil
}

func (f *fakeAnnouncementRequestRepo) MarkPublished(_ context.Context, id, actorID, actorEmail string, caseIDs []string) (domain.AnnouncementRequest, error) {
	f.gotPublishID = id
	f.gotPublishActorID = actorID
	f.gotPublishActorEmail = actorEmail
	f.gotPublishCaseIDs = caseIDs
	return domain.AnnouncementRequest{ID: id, State: domain.AnnouncementRequestStatePublished, PublishedCaseIDs: caseIDs}, nil
}

func (f *fakeAnnouncementRequestRepo) SetSchedule(_ context.Context, id string, scheduledFor *time.Time) (domain.AnnouncementRequest, error) {
	f.gotScheduleID = id
	f.gotScheduleScheduledFor = scheduledFor
	f.scheduleCalled = true
	return domain.AnnouncementRequest{ID: id, State: domain.AnnouncementRequestStateApproved, ScheduledFor: scheduledFor}, nil
}

func (f *fakeAnnouncementRequestRepo) ClaimForAutoPublish(_ context.Context, id string, _ time.Duration) (domain.AnnouncementRequest, error) {
	f.gotClaimID = id
	f.claimCalled = true
	if f.claimErr != nil {
		return domain.AnnouncementRequest{}, f.claimErr
	}
	if f.claimResult.State != "" {
		return f.claimResult, nil
	}
	return f.getResult, nil
}

func (f *fakeAnnouncementRequestRepo) ReleaseAutoPublishClaim(_ context.Context, id string) error {
	f.gotReleaseID = id
	f.releaseCalled++
	return f.releaseErr
}

func (f *fakeAnnouncementRequestRepo) CreateUpdate(_ context.Context, announcementRequestID, content, createdBy, createdByEmail string) (domain.AnnouncementRequestUpdate, error) {
	f.gotCreateUpdateReqID = announcementRequestID
	f.gotCreateUpdateContent = content
	f.gotCreateUpdateCreatedBy = createdBy
	f.gotCreateUpdateCreatedByEmail = createdByEmail
	if f.createUpdateResult.ID != "" {
		return f.createUpdateResult, nil
	}
	return domain.AnnouncementRequestUpdate{ID: "update-1", AnnouncementRequestID: announcementRequestID, Content: content, CreatedBy: createdBy}, nil
}

func (f *fakeAnnouncementRequestRepo) ListUpdates(_ context.Context, announcementRequestID string) ([]domain.AnnouncementRequestUpdate, error) {
	f.gotListUpdatesID = announcementRequestID
	return f.listUpdatesResult, nil
}

func (f *fakeAnnouncementRequestRepo) UpsertDeliveries(_ context.Context, announcementRequestID string, deliveries []domain.RecordAnnouncementRequestDeliveryInput) ([]domain.AnnouncementRequestDelivery, error) {
	f.mu.Lock()
	f.gotUpsertDeliveriesID = announcementRequestID
	f.upsertDeliveriesCalls++
	// AutoPublish now records one project's outcome per call (see its own
	// doc comment for why), so this accumulates across every call within a
	// test rather than keeping only the most recent one.
	f.gotUpsertDeliveriesReq = append(f.gotUpsertDeliveriesReq, deliveries...)
	upsertDeliveriesErr := f.upsertDeliveriesErr
	upsertDeliveriesResult := f.upsertDeliveriesResult
	f.mu.Unlock()
	if upsertDeliveriesErr != nil {
		return nil, upsertDeliveriesErr
	}
	if upsertDeliveriesResult != nil {
		return upsertDeliveriesResult, nil
	}
	result := make([]domain.AnnouncementRequestDelivery, len(deliveries))
	for i, d := range deliveries {
		result[i] = domain.AnnouncementRequestDelivery{
			AnnouncementRequestID: announcementRequestID,
			ProjectID:             d.ProjectID,
			CaseID:                d.CaseID,
			Status:                d.Status,
			ErrorMessage:          d.ErrorMessage,
		}
	}
	return result, nil
}

func (f *fakeAnnouncementRequestRepo) ListDeliveries(_ context.Context, announcementRequestID string) ([]domain.AnnouncementRequestDelivery, error) {
	f.gotListDeliveriesID = announcementRequestID
	return f.listDeliveriesResult, nil
}

// fakeCaseFanOutClient stubs the narrow CreateCase/AddCaseTagAs subset
// AutoPublish uses — every created case gets a sequential id ("case-1",
// "case-2", ...) unless createCaseFn overrides that.
type fakeCaseFanOutClient struct {
	createCaseFn func(ctx context.Context, req domain.CreateCaseRequest) (domain.CreateCaseResponse, error)
	addTagFn     func(ctx context.Context, caseID, label string) (domain.Tag, error)

	createdCases      []domain.CreateCaseRequest
	taggedCases       []string
	taggedActorEmails []string
	nextCaseNum       int

	// mu guards every field above -- AutoPublish now calls CreateCase/
	// AddCaseTagAs concurrently, one goroutine per project (see its own doc
	// comment), so this fake needs to be as safe for concurrent use as the
	// real client already is.
	mu sync.Mutex
}

func (f *fakeCaseFanOutClient) CreateCase(ctx context.Context, req domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
	f.mu.Lock()
	f.createdCases = append(f.createdCases, req)
	f.nextCaseNum++
	caseNum := f.nextCaseNum
	fn := f.createCaseFn
	f.mu.Unlock()
	if fn != nil {
		return fn(ctx, req)
	}
	return domain.CreateCaseResponse{Case: domain.CreateCaseDetails{ID: fmt.Sprintf("case-%d", caseNum)}}, nil
}

func (f *fakeCaseFanOutClient) AddCaseTagAs(ctx context.Context, caseID, label, actorEmail string) (domain.Tag, error) {
	f.mu.Lock()
	f.taggedCases = append(f.taggedCases, caseID)
	f.taggedActorEmails = append(f.taggedActorEmails, actorEmail)
	fn := f.addTagFn
	f.mu.Unlock()
	if fn != nil {
		return fn(ctx, caseID, label)
	}
	return domain.Tag{}, nil
}

func TestAnnouncementRequestService_CreateDraft(t *testing.T) {
	t.Run("forwards a valid request", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{}
		svc := NewAnnouncementRequestService(repo, nil, nil)

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
		svc := NewAnnouncementRequestService(repo, nil, nil)
		_, err := svc.CreateDraft(context.Background(), domain.CreateAnnouncementRequestRequest{
			Kind: "bogus", CreatedBy: "user-1",
		})
		if err == nil {
			t.Fatal("expected validation error for invalid kind, got nil")
		}
	})

	t.Run("rejects a missing createdBy", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{}
		svc := NewAnnouncementRequestService(repo, nil, nil)
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
		svc := NewAnnouncementRequestService(repo, nil, nil)

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
		svc := NewAnnouncementRequestService(repo, nil, nil)

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
				svc := NewAnnouncementRequestService(repo, nil, nil)
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
		svc := NewAnnouncementRequestService(repo, nil, nil)
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
		svc := NewAnnouncementRequestService(repo, nil, nil)
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
		svc := NewAnnouncementRequestService(repo, nil, nil)
		_, err := svc.Approve(context.Background(), "req-1", "user-2", "user-2@example.com")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.gotApproveActorID != "user-2" {
			t.Fatalf("expected actorId forwarded, got %q", repo.gotApproveActorID)
		}
		if repo.gotApproveActorEmail != "user-2@example.com" {
			t.Fatalf("expected actorEmail forwarded, got %q", repo.gotApproveActorEmail)
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
				svc := NewAnnouncementRequestService(repo, nil, nil)
				if _, err := svc.Approve(context.Background(), "req-1", "user-2", "user-2@example.com"); err == nil {
					t.Fatalf("expected a conflict error approving from state %q, got nil", state)
				}
			})
		}
	})
}

func TestAnnouncementRequestService_MarkPublished(t *testing.T) {
	t.Run("accepts from approved when the actor is the creator, forwarding caseIds", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{
			State:     domain.AnnouncementRequestStateApproved,
			CreatedBy: "user-3",
		}}
		svc := NewAnnouncementRequestService(repo, nil, nil)
		got, err := svc.MarkPublished(context.Background(), "req-1", "user-3", "user-3@example.com", []string{"case-1", "case-2"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.gotPublishActorID != "user-3" {
			t.Fatalf("expected actorId forwarded, got %q", repo.gotPublishActorID)
		}
		if repo.gotPublishActorEmail != "user-3@example.com" {
			t.Fatalf("expected actorEmail forwarded, got %q", repo.gotPublishActorEmail)
		}
		if len(repo.gotPublishCaseIDs) != 2 {
			t.Fatalf("expected caseIds forwarded, got %v", repo.gotPublishCaseIDs)
		}
		if len(got.PublishedCaseIDs) != 2 {
			t.Fatalf("expected PublishedCaseIDs on the result, got %+v", got)
		}
	})

	t.Run("rejects an empty caseIds", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{
			State:     domain.AnnouncementRequestStateApproved,
			CreatedBy: "user-3",
		}}
		svc := NewAnnouncementRequestService(repo, nil, nil)
		_, err := svc.MarkPublished(context.Background(), "req-1", "user-3", "user-3@example.com", nil)
		var ve *apierror.ValidationError
		if !isValidationError(err, &ve) {
			t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
		}
	})

	// TestAnnouncementRequestService_MarkPublished/rejects_a_non-creator_actor
	// locks in that publish -- unlike approve, which anyone can still do for
	// now (see Approve's own tests) -- is restricted to the request's own
	// creator: an approver's job is only to approve, per Danidu's explicit
	// instruction that the person accepting an announcement must not also be
	// the one who can send it.
	t.Run("rejects a non-creator actor", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{
			State:     domain.AnnouncementRequestStateApproved,
			CreatedBy: "user-1",
		}}
		svc := NewAnnouncementRequestService(repo, nil, nil)
		_, err := svc.MarkPublished(context.Background(), "req-1", "user-3", "user-3@example.com", []string{"case-1"})
		if _, ok := err.(*apierror.ForbiddenError); !ok {
			t.Fatalf("expected *apierror.ForbiddenError, got %T: %v", err, err)
		}
	})

	t.Run("rejects from any state other than approved", func(t *testing.T) {
		for _, state := range []domain.AnnouncementRequestState{
			domain.AnnouncementRequestStateDraft,
			domain.AnnouncementRequestStatePendingApproval,
			domain.AnnouncementRequestStatePublished,
		} {
			t.Run(string(state), func(t *testing.T) {
				repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{State: state, CreatedBy: "user-3"}}
				svc := NewAnnouncementRequestService(repo, nil, nil)
				if _, err := svc.MarkPublished(context.Background(), "req-1", "user-3", "user-3@example.com", []string{"case-1"}); err == nil {
					t.Fatalf("expected a conflict error publishing from state %q, got nil", state)
				}
			})
		}
	})
}

func TestAnnouncementRequestService_Schedule(t *testing.T) {
	future := time.Now().Add(24 * time.Hour)

	t.Run("accepts from approved when the actor is the creator, forwarding scheduledFor", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{
			State:     domain.AnnouncementRequestStateApproved,
			CreatedBy: "user-3",
		}}
		svc := NewAnnouncementRequestService(repo, nil, nil)
		got, err := svc.Schedule(context.Background(), "req-1", "user-3", "user-3@example.com", &future)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !repo.scheduleCalled || repo.gotScheduleID != "req-1" {
			t.Fatalf("expected SetSchedule called for req-1, got %+v", repo)
		}
		if repo.gotScheduleScheduledFor == nil || !repo.gotScheduleScheduledFor.Equal(future) {
			t.Fatalf("expected scheduledFor forwarded, got %v", repo.gotScheduleScheduledFor)
		}
		if got.ScheduledFor == nil {
			t.Fatalf("expected ScheduledFor on the result, got %+v", got)
		}
	})

	t.Run("accepts a nil scheduledFor to clear the schedule", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{
			State:     domain.AnnouncementRequestStateApproved,
			CreatedBy: "user-3",
		}}
		svc := NewAnnouncementRequestService(repo, nil, nil)
		if _, err := svc.Schedule(context.Background(), "req-1", "user-3", "user-3@example.com", nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.gotScheduleScheduledFor != nil {
			t.Fatalf("expected nil scheduledFor forwarded (clearing), got %v", repo.gotScheduleScheduledFor)
		}
	})

	t.Run("rejects a scheduledFor that isn't strictly in the future", func(t *testing.T) {
		now := time.Now()
		past := now.Add(-time.Hour)
		for name, ts := range map[string]time.Time{"in the past": past, "right now": now} {
			t.Run(name, func(t *testing.T) {
				repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{
					State:     domain.AnnouncementRequestStateApproved,
					CreatedBy: "user-3",
				}}
				svc := NewAnnouncementRequestService(repo, nil, nil)
				_, err := svc.Schedule(context.Background(), "req-1", "user-3", "user-3@example.com", &ts)
				var ve *apierror.ValidationError
				if !isValidationError(err, &ve) {
					t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
				}
			})
		}
	})

	t.Run("rejects a non-creator actor", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{
			State:     domain.AnnouncementRequestStateApproved,
			CreatedBy: "user-1",
		}}
		svc := NewAnnouncementRequestService(repo, nil, nil)
		_, err := svc.Schedule(context.Background(), "req-1", "user-3", "user-3@example.com", &future)
		if _, ok := err.(*apierror.ForbiddenError); !ok {
			t.Fatalf("expected *apierror.ForbiddenError, got %T: %v", err, err)
		}
	})

	t.Run("rejects from any state other than approved", func(t *testing.T) {
		for _, state := range []domain.AnnouncementRequestState{
			domain.AnnouncementRequestStateDraft,
			domain.AnnouncementRequestStatePendingApproval,
			domain.AnnouncementRequestStatePublished,
		} {
			t.Run(string(state), func(t *testing.T) {
				repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{State: state, CreatedBy: "user-3"}}
				svc := NewAnnouncementRequestService(repo, nil, nil)
				if _, err := svc.Schedule(context.Background(), "req-1", "user-3", "user-3@example.com", &future); err == nil {
					t.Fatalf("expected a conflict error scheduling from state %q, got nil", state)
				}
			})
		}
	})
}

func TestAnnouncementRequestService_AutoPublish(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(time.Hour)
	internal := stubAccess{scope: AccessScope{Unrestricted: true}}

	dueApproved := func() domain.AnnouncementRequest {
		return domain.AnnouncementRequest{
			State:              domain.AnnouncementRequestStateApproved,
			CreatedBy:          "user-3",
			CreatedByEmail:     strPtr("user-3@example.com"),
			Subject:            "Maintenance",
			Description:        "Details",
			ResolvedProjectIDs: []string{"proj-1", "proj-2"},
			ScheduledFor:       &past,
		}
	}

	t.Run("rejects a non-internal caller", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: dueApproved()}
		svc := NewAnnouncementRequestService(repo, &fakeCaseFanOutClient{}, stubAccess{scope: AccessScope{Unrestricted: false}})
		_, err := svc.AutoPublish(context.Background(), "req-1")
		if _, ok := err.(*apierror.ForbiddenError); !ok {
			t.Fatalf("expected *apierror.ForbiddenError, got %T: %v", err, err)
		}
	})

	t.Run("rejects from any state other than approved", func(t *testing.T) {
		for _, state := range []domain.AnnouncementRequestState{
			domain.AnnouncementRequestStateDraft,
			domain.AnnouncementRequestStatePendingApproval,
			domain.AnnouncementRequestStatePublished,
		} {
			t.Run(string(state), func(t *testing.T) {
				req := dueApproved()
				req.State = state
				repo := &fakeAnnouncementRequestRepo{getResult: req}
				svc := NewAnnouncementRequestService(repo, &fakeCaseFanOutClient{}, internal)
				if _, err := svc.AutoPublish(context.Background(), "req-1"); err == nil {
					t.Fatalf("expected a conflict error auto-publishing from state %q, got nil", state)
				}
			})
		}
	})

	t.Run("rejects when scheduledFor is nil or still in the future", func(t *testing.T) {
		for name, sched := range map[string]*time.Time{"nil": nil, "in the future": &future} {
			t.Run(name, func(t *testing.T) {
				req := dueApproved()
				req.ScheduledFor = sched
				repo := &fakeAnnouncementRequestRepo{getResult: req}
				svc := NewAnnouncementRequestService(repo, &fakeCaseFanOutClient{}, internal)
				if _, err := svc.AutoPublish(context.Background(), "req-1"); err == nil {
					t.Fatal("expected a conflict error when not yet due, got nil")
				}
			})
		}
	})

	t.Run("rejects when there is no resolved audience", func(t *testing.T) {
		req := dueApproved()
		req.ResolvedProjectIDs = nil
		repo := &fakeAnnouncementRequestRepo{getResult: req}
		svc := NewAnnouncementRequestService(repo, &fakeCaseFanOutClient{}, internal)
		if _, err := svc.AutoPublish(context.Background(), "req-1"); err == nil {
			t.Fatal("expected a conflict error for an empty resolved audience, got nil")
		}
	})

	t.Run("creates a case per unresolved project, records deliveries, and marks published on full success", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: dueApproved()}
		cases := &fakeCaseFanOutClient{}
		svc := NewAnnouncementRequestService(repo, cases, internal)

		got, err := svc.AutoPublish(context.Background(), "req-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cases.createdCases) != 2 {
			t.Fatalf("expected one case created per resolved project, got %d: %+v", len(cases.createdCases), cases.createdCases)
		}
		for _, c := range cases.createdCases {
			if c.Type != "announcement" || c.CreatedBy != "user-3" {
				t.Fatalf("expected type=announcement and createdBy from the request's own creator, got %+v", c)
			}
		}
		if len(cases.taggedCases) != 0 {
			t.Fatalf("expected no tag attach for a non-security announcement, got %v", cases.taggedCases)
		}
		if len(repo.gotUpsertDeliveriesReq) != 2 {
			t.Fatalf("expected 2 delivery entries recorded, got %+v", repo.gotUpsertDeliveriesReq)
		}
		if repo.gotPublishID != "req-1" || len(repo.gotPublishCaseIDs) != 2 {
			t.Fatalf("expected MarkPublished called with both case ids, got id=%q caseIds=%v", repo.gotPublishID, repo.gotPublishCaseIDs)
		}
		if got.State != domain.AnnouncementRequestStatePublished {
			t.Fatalf("expected the result to report published, got %+v", got)
		}
	})

	t.Run("marks the request as security by attaching the tag to every created case", func(t *testing.T) {
		req := dueApproved()
		req.IsSecurityAnnouncement = true
		repo := &fakeAnnouncementRequestRepo{getResult: req}
		cases := &fakeCaseFanOutClient{}
		svc := NewAnnouncementRequestService(repo, cases, internal)

		if _, err := svc.AutoPublish(context.Background(), "req-1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cases.taggedCases) != 2 {
			t.Fatalf("expected both created cases tagged, got %v", cases.taggedCases)
		}
	})

	t.Run("attaches the security tag as the request's own creator, not a resolved x-user-id-token", func(t *testing.T) {
		// Regression test: AutoPublish runs with no end-user token on its
		// ctx at all (it's an internal, machine-to-machine caller — see
		// caseFanOutClient's own doc comment) — AddCaseTag would 401 with
		// "x-user-id-token header is required" every single time from this
		// caller, which is exactly what happened live before AddCaseTagAs
		// existed: every scheduled security announcement's tag attach
		// failed, though its cases were created fine (CreateCase already
		// had its own CreatedBy workaround). Asserts on the actor email
		// actually forwarded, not just that a tag call happened.
		req := dueApproved()
		req.IsSecurityAnnouncement = true
		repo := &fakeAnnouncementRequestRepo{getResult: req}
		cases := &fakeCaseFanOutClient{}
		svc := NewAnnouncementRequestService(repo, cases, internal)

		if _, err := svc.AutoPublish(context.Background(), "req-1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, email := range cases.taggedActorEmails {
			if email != "user-3@example.com" {
				t.Fatalf("expected every tag attach to use the request's own CreatedByEmail, got %v", cases.taggedActorEmails)
			}
		}
	})

	t.Run("rejects a security announcement with no recorded creator email, rather than attaching the tag as no one", func(t *testing.T) {
		req := dueApproved()
		req.IsSecurityAnnouncement = true
		req.CreatedByEmail = nil
		repo := &fakeAnnouncementRequestRepo{getResult: req}
		cases := &fakeCaseFanOutClient{}
		svc := NewAnnouncementRequestService(repo, cases, internal)

		if _, err := svc.AutoPublish(context.Background(), "req-1"); err == nil {
			t.Fatal("expected a conflict error for a security announcement with no CreatedByEmail, got nil")
		}
		if len(cases.createdCases) != 0 {
			t.Fatalf("expected no case creation attempted before this guard, got %+v", cases.createdCases)
		}
	})

	t.Run("resumes from existing successful deliveries without recreating their cases", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{
			getResult: dueApproved(),
			listDeliveriesResult: []domain.AnnouncementRequestDelivery{
				{ProjectID: "proj-1", CaseID: strPtr("case-existing"), Status: domain.AnnouncementRequestDeliveryStatusSucceeded},
			},
		}
		cases := &fakeCaseFanOutClient{}
		svc := NewAnnouncementRequestService(repo, cases, internal)

		_, err := svc.AutoPublish(context.Background(), "req-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cases.createdCases) != 1 || cases.createdCases[0].ProjectID != "proj-2" {
			t.Fatalf("expected only the still-pending project (proj-2) to get a new case, got %+v", cases.createdCases)
		}
		found := false
		for _, cid := range repo.gotPublishCaseIDs {
			if cid == "case-existing" {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected the already-succeeded project's existing case id forwarded to publish, got %v", repo.gotPublishCaseIDs)
		}
	})

	t.Run("retries a tag_failed delivery by reattaching to the existing case, never creating a second one", func(t *testing.T) {
		req := dueApproved()
		req.IsSecurityAnnouncement = true
		repo := &fakeAnnouncementRequestRepo{
			getResult: req,
			listDeliveriesResult: []domain.AnnouncementRequestDelivery{
				{ProjectID: "proj-1", CaseID: strPtr("case-existing"), Status: domain.AnnouncementRequestDeliveryStatusTagFailed},
				{ProjectID: "proj-2", CaseID: strPtr("case-existing-2"), Status: domain.AnnouncementRequestDeliveryStatusSucceeded},
			},
		}
		cases := &fakeCaseFanOutClient{}
		svc := NewAnnouncementRequestService(repo, cases, internal)

		_, err := svc.AutoPublish(context.Background(), "req-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cases.createdCases) != 0 {
			t.Fatalf("expected no new case created — both projects already have one, got %+v", cases.createdCases)
		}
		if len(cases.taggedCases) != 1 || cases.taggedCases[0] != "case-existing" {
			t.Fatalf("expected only the tag_failed project's existing case retagged, got %v", cases.taggedCases)
		}
		if len(cases.taggedActorEmails) != 1 || cases.taggedActorEmails[0] != "user-3@example.com" {
			t.Fatalf("expected the retry to use the request's CreatedByEmail, got %v", cases.taggedActorEmails)
		}
	})

	t.Run("returns a conflict without touching cases when the claim is already held (overlapping attempt)", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{
			getResult: dueApproved(),
			claimErr:  &apierror.ConflictError{Msg: "already claimed"},
		}
		cases := &fakeCaseFanOutClient{}
		svc := NewAnnouncementRequestService(repo, cases, internal)

		_, err := svc.AutoPublish(context.Background(), "req-1")
		if _, ok := err.(*apierror.ConflictError); !ok {
			t.Fatalf("expected *apierror.ConflictError, got %T: %v", err, err)
		}
		if len(cases.createdCases) != 0 {
			t.Fatalf("expected no case creation attempted when the claim itself failed, got %+v", cases.createdCases)
		}
		if repo.releaseCalled != 0 {
			t.Fatalf("expected no release when the claim was never actually taken, got %d", repo.releaseCalled)
		}
	})

	t.Run("releases the claim on both full success and partial failure", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			repo := &fakeAnnouncementRequestRepo{getResult: dueApproved()}
			svc := NewAnnouncementRequestService(repo, &fakeCaseFanOutClient{}, internal)
			if _, err := svc.AutoPublish(context.Background(), "req-1"); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if repo.releaseCalled != 1 {
				t.Fatalf("expected the claim released exactly once, got %d", repo.releaseCalled)
			}
		})

		t.Run("partial failure", func(t *testing.T) {
			repo := &fakeAnnouncementRequestRepo{getResult: dueApproved()}
			cases := &fakeCaseFanOutClient{
				createCaseFn: func(_ context.Context, req domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
					if req.ProjectID == "proj-2" {
						return domain.CreateCaseResponse{}, fmt.Errorf("boom")
					}
					return domain.CreateCaseResponse{Case: domain.CreateCaseDetails{ID: "case-1"}}, nil
				},
			}
			svc := NewAnnouncementRequestService(repo, cases, internal)
			if _, err := svc.AutoPublish(context.Background(), "req-1"); err == nil {
				t.Fatal("expected an error, got nil")
			}
			if repo.releaseCalled != 1 {
				t.Fatalf("expected the claim released exactly once even on failure, got %d", repo.releaseCalled)
			}
		})
	})

	t.Run("a security tag failure on a newly created case blocks publish, same as a retried tag failure", func(t *testing.T) {
		req := dueApproved()
		req.IsSecurityAnnouncement = true
		repo := &fakeAnnouncementRequestRepo{getResult: req}
		cases := &fakeCaseFanOutClient{
			addTagFn: func(_ context.Context, caseID, _ string) (domain.Tag, error) {
				if caseID == "case-proj-2" {
					return domain.Tag{}, fmt.Errorf("tag service unavailable")
				}
				return domain.Tag{}, nil
			},
			createCaseFn: func(_ context.Context, req domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
				return domain.CreateCaseResponse{Case: domain.CreateCaseDetails{ID: "case-" + req.ProjectID}}, nil
			},
		}
		svc := NewAnnouncementRequestService(repo, cases, internal)

		_, err := svc.AutoPublish(context.Background(), "req-1")
		if _, ok := err.(*apierror.ConflictError); !ok {
			t.Fatalf("expected a conflict (not yet fully delivered) when a new case's tag attach fails, got %T: %v", err, err)
		}
		if repo.gotPublishID != "" {
			t.Fatalf("expected MarkPublished never called while a security tag is still missing, got id=%q", repo.gotPublishID)
		}
	})

	t.Run("records each project's outcome as it happens, not batched at the end", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: dueApproved()}
		cases := &fakeCaseFanOutClient{
			createCaseFn: func(_ context.Context, req domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
				return domain.CreateCaseResponse{Case: domain.CreateCaseDetails{ID: "case-" + req.ProjectID}}, nil
			},
		}
		svc := NewAnnouncementRequestService(repo, cases, internal)

		if _, err := svc.AutoPublish(context.Background(), "req-1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Proves deliveries are persisted incrementally, one call per
		// project, rather than accumulated and flushed once in a single
		// batched call at the end — the property this test is actually
		// about. It no longer asserts a specific project ever saw fewer
		// than N deliveries already recorded: now that the fan-out is
		// bounded-concurrent (see AutoPublish's own doc comment), the
		// order in which projects finish is no longer deterministic.
		if repo.upsertDeliveriesCalls != 2 {
			t.Fatalf("expected 2 separate UpsertDeliveries calls (one per project), got %d", repo.upsertDeliveriesCalls)
		}
		if len(repo.gotUpsertDeliveriesReq) != 2 {
			t.Fatalf("expected both projects recorded by the end, got %+v", repo.gotUpsertDeliveriesReq)
		}
	})

	t.Run("leaves the request approved and returns a conflict when a case creation fails", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: dueApproved()}
		cases := &fakeCaseFanOutClient{
			createCaseFn: func(_ context.Context, req domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
				if req.ProjectID == "proj-2" {
					return domain.CreateCaseResponse{}, fmt.Errorf("boom")
				}
				return domain.CreateCaseResponse{Case: domain.CreateCaseDetails{ID: "case-1"}}, nil
			},
		}
		svc := NewAnnouncementRequestService(repo, cases, internal)

		_, err := svc.AutoPublish(context.Background(), "req-1")
		if err == nil {
			t.Fatal("expected an error when one project's case creation fails, got nil")
		}
		if repo.gotPublishID != "" {
			t.Fatalf("expected MarkPublished never called on partial failure, got id=%q", repo.gotPublishID)
		}
		if len(repo.gotUpsertDeliveriesReq) != 2 {
			t.Fatalf("expected both outcomes (succeeded + failed) recorded for this pass, got %+v", repo.gotUpsertDeliveriesReq)
		}
	})
}

func TestAnnouncementRequestService_AddUpdate(t *testing.T) {
	t.Run("accepts from published when the actor is the creator", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{
			State:     domain.AnnouncementRequestStatePublished,
			CreatedBy: "user-3",
		}}
		svc := NewAnnouncementRequestService(repo, nil, nil)
		got, err := svc.AddUpdate(context.Background(), "req-1", "user-3", "user-3@example.com", "A correction to the above.")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.gotCreateUpdateReqID != "req-1" || repo.gotCreateUpdateContent != "A correction to the above." || repo.gotCreateUpdateCreatedBy != "user-3" {
			t.Fatalf("expected the update forwarded to the repo, got id=%q content=%q createdBy=%q",
				repo.gotCreateUpdateReqID, repo.gotCreateUpdateContent, repo.gotCreateUpdateCreatedBy)
		}
		if repo.gotCreateUpdateCreatedByEmail != "user-3@example.com" {
			t.Fatalf("expected createdByEmail forwarded, got %q", repo.gotCreateUpdateCreatedByEmail)
		}
		if got.Content != "A correction to the above." {
			t.Fatalf("expected content on the result, got %+v", got)
		}
	})

	t.Run("rejects empty content", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{
			State: domain.AnnouncementRequestStatePublished, CreatedBy: "user-3",
		}}
		svc := NewAnnouncementRequestService(repo, nil, nil)
		_, err := svc.AddUpdate(context.Background(), "req-1", "user-3", "user-3@example.com", "   ")
		var ve *apierror.ValidationError
		if !isValidationError(err, &ve) {
			t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
		}
	})

	// Same creator-only restriction as MarkPublished, and for the same
	// reason: an approver's job is only to approve, not to also decide what
	// gets appended to real customer-facing cases after the fact.
	t.Run("rejects a non-creator actor", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{
			State: domain.AnnouncementRequestStatePublished, CreatedBy: "user-1",
		}}
		svc := NewAnnouncementRequestService(repo, nil, nil)
		_, err := svc.AddUpdate(context.Background(), "req-1", "user-3", "user-3@example.com", "content")
		if _, ok := err.(*apierror.ForbiddenError); !ok {
			t.Fatalf("expected *apierror.ForbiddenError, got %T: %v", err, err)
		}
	})

	t.Run("rejects from any state other than published", func(t *testing.T) {
		for _, state := range []domain.AnnouncementRequestState{
			domain.AnnouncementRequestStateDraft,
			domain.AnnouncementRequestStatePendingApproval,
			domain.AnnouncementRequestStateApproved,
		} {
			t.Run(string(state), func(t *testing.T) {
				repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{State: state, CreatedBy: "user-3"}}
				svc := NewAnnouncementRequestService(repo, nil, nil)
				if _, err := svc.AddUpdate(context.Background(), "req-1", "user-3", "user-3@example.com", "content"); err == nil {
					t.Fatalf("expected a conflict error adding an update from state %q, got nil", state)
				}
			})
		}
	})
}

func TestAnnouncementRequestService_ListUpdates(t *testing.T) {
	t.Run("returns the repo's updates for an existing request", func(t *testing.T) {
		want := []domain.AnnouncementRequestUpdate{
			{ID: "u-2", Content: "second"}, {ID: "u-1", Content: "first"},
		}
		repo := &fakeAnnouncementRequestRepo{
			getResult:         domain.AnnouncementRequest{ID: "req-1", State: domain.AnnouncementRequestStatePublished},
			listUpdatesResult: want,
		}
		svc := NewAnnouncementRequestService(repo, nil, nil)
		got, err := svc.ListUpdates(context.Background(), "req-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.gotListUpdatesID != "req-1" {
			t.Fatalf("expected the id forwarded to the repo, got %q", repo.gotListUpdatesID)
		}
		if len(got.Updates) != 2 {
			t.Fatalf("expected both updates, got %+v", got.Updates)
		}
	})

	t.Run("propagates a NotFoundError for a nonexistent request without calling ListUpdates", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getErr: &apierror.NotFoundError{Msg: "announcement request not found"}}
		svc := NewAnnouncementRequestService(repo, nil, nil)
		_, err := svc.ListUpdates(context.Background(), "missing")
		if _, ok := err.(*apierror.NotFoundError); !ok {
			t.Fatalf("expected *apierror.NotFoundError, got %T: %v", err, err)
		}
		if repo.gotListUpdatesID != "" {
			t.Fatalf("expected ListUpdates not to be called, got id %q", repo.gotListUpdatesID)
		}
	})
}

func TestAnnouncementRequestService_Update(t *testing.T) {
	subject := "Updated subject"

	t.Run("draft: plain update, no revert", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{State: domain.AnnouncementRequestStateDraft}}
		svc := NewAnnouncementRequestService(repo, nil, nil)

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
		svc := NewAnnouncementRequestService(repo, nil, nil)

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
		svc := NewAnnouncementRequestService(repo, nil, nil)

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
		svc := NewAnnouncementRequestService(repo, nil, nil)

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
		svc := NewAnnouncementRequestService(repo, nil, nil)

		_, err := svc.Update(context.Background(), "req-1", domain.UpdateAnnouncementRequestRequest{Subject: &subject, ActorID: "user-1"})
		if err == nil {
			t.Fatal("expected an error editing a published request, got nil")
		}
	})

	t.Run("rejects a missing actorId", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{State: domain.AnnouncementRequestStateDraft}}
		svc := NewAnnouncementRequestService(repo, nil, nil)
		_, err := svc.Update(context.Background(), "req-1", domain.UpdateAnnouncementRequestRequest{Subject: &subject})
		if err == nil {
			t.Fatal("expected a validation error for a missing actorId, got nil")
		}
	})
}

func TestAnnouncementRequestService_RecordDryRun(t *testing.T) {
	t.Run("accepts from draft", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{State: domain.AnnouncementRequestStateDraft}}
		svc := NewAnnouncementRequestService(repo, nil, nil)

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
				svc := NewAnnouncementRequestService(repo, nil, nil)
				_, err := svc.RecordDryRun(context.Background(), "req-1", domain.RecordAnnouncementDryRunRequest{CaseID: "case-1", ActorID: "user-1"})
				if err == nil {
					t.Fatalf("expected a conflict error recording a dry run from state %q, got nil", state)
				}
			})
		}
	})

	t.Run("rejects a missing caseId", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{State: domain.AnnouncementRequestStateDraft}}
		svc := NewAnnouncementRequestService(repo, nil, nil)
		_, err := svc.RecordDryRun(context.Background(), "req-1", domain.RecordAnnouncementDryRunRequest{ActorID: "user-1"})
		if err == nil {
			t.Fatal("expected a validation error for a missing caseId, got nil")
		}
	})
}

func TestAnnouncementRequestService_Search(t *testing.T) {
	t.Run("normalizes pagination defaults", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{}
		svc := NewAnnouncementRequestService(repo, nil, nil)

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
		svc := NewAnnouncementRequestService(repo, nil, nil)
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
		svc := NewAnnouncementRequestService(repo, nil, nil)
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

	t.Run("accepts readyForScheduledPublish alone, forwarding it to the repo", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{}
		svc := NewAnnouncementRequestService(repo, nil, nil)
		_, err := svc.Search(context.Background(), domain.SearchAnnouncementRequestsRequest{
			ReadyForScheduledPublish: true,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !repo.gotSearchReq.ReadyForScheduledPublish {
			t.Fatal("expected readyForScheduledPublish forwarded to the repo")
		}
	})

	t.Run("rejects readyForScheduledPublish combined with an explicit state", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{}
		svc := NewAnnouncementRequestService(repo, nil, nil)
		approved := domain.AnnouncementRequestStateApproved
		_, err := svc.Search(context.Background(), domain.SearchAnnouncementRequestsRequest{
			ReadyForScheduledPublish: true,
			State:                    &approved,
		})
		var ve *apierror.ValidationError
		if !isValidationError(err, &ve) {
			t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
		}
	})
}

func TestAnnouncementRequestService_RecordDeliveries(t *testing.T) {
	t.Run("accepts from approved when the actor is the creator", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{
			State:              domain.AnnouncementRequestStateApproved,
			CreatedBy:          "user-3",
			ResolvedProjectIDs: []string{"proj-1", "proj-2"},
		}}
		svc := NewAnnouncementRequestService(repo, nil, nil)
		caseID := "case-1"
		resp, err := svc.RecordDeliveries(context.Background(), "req-1", "user-3", []domain.RecordAnnouncementRequestDeliveryInput{
			{ProjectID: "proj-1", CaseID: &caseID, Status: domain.AnnouncementRequestDeliveryStatusSucceeded},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.gotUpsertDeliveriesID != "req-1" {
			t.Fatalf("expected the id forwarded to the repo, got %q", repo.gotUpsertDeliveriesID)
		}
		if len(repo.gotUpsertDeliveriesReq) != 1 || repo.gotUpsertDeliveriesReq[0].ProjectID != "proj-1" {
			t.Fatalf("expected the deliveries forwarded to the repo, got %+v", repo.gotUpsertDeliveriesReq)
		}
		if len(resp.Deliveries) != 1 {
			t.Fatalf("expected the saved deliveries on the result, got %+v", resp)
		}
	})

	t.Run("rejects an empty deliveries list", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{
			State: domain.AnnouncementRequestStateApproved, CreatedBy: "user-3",
		}}
		svc := NewAnnouncementRequestService(repo, nil, nil)
		_, err := svc.RecordDeliveries(context.Background(), "req-1", "user-3", nil)
		var ve *apierror.ValidationError
		if !isValidationError(err, &ve) {
			t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
		}
	})

	t.Run("rejects a delivery for a project outside the resolved audience", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{
			State:              domain.AnnouncementRequestStateApproved,
			CreatedBy:          "user-3",
			ResolvedProjectIDs: []string{"proj-1"},
		}}
		svc := NewAnnouncementRequestService(repo, nil, nil)
		caseID := "case-1"
		_, err := svc.RecordDeliveries(context.Background(), "req-1", "user-3", []domain.RecordAnnouncementRequestDeliveryInput{
			{ProjectID: "proj-not-resolved", CaseID: &caseID, Status: domain.AnnouncementRequestDeliveryStatusSucceeded},
		})
		var ve *apierror.ValidationError
		if !isValidationError(err, &ve) {
			t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
		}
	})

	t.Run("rejects succeeded/tag_failed with no caseId", func(t *testing.T) {
		for _, status := range []domain.AnnouncementRequestDeliveryStatus{
			domain.AnnouncementRequestDeliveryStatusSucceeded,
			domain.AnnouncementRequestDeliveryStatusTagFailed,
		} {
			t.Run(string(status), func(t *testing.T) {
				repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{
					State:              domain.AnnouncementRequestStateApproved,
					CreatedBy:          "user-3",
					ResolvedProjectIDs: []string{"proj-1"},
				}}
				svc := NewAnnouncementRequestService(repo, nil, nil)
				_, err := svc.RecordDeliveries(context.Background(), "req-1", "user-3", []domain.RecordAnnouncementRequestDeliveryInput{
					{ProjectID: "proj-1", Status: status},
				})
				var ve *apierror.ValidationError
				if !isValidationError(err, &ve) {
					t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
				}
			})
		}
	})

	// A delivery with status "failed" and no caseId is the normal shape for a
	// case-create that never succeeded — must not be rejected the way
	// succeeded/tag_failed are above.
	t.Run("accepts failed with no caseId", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{
			State:              domain.AnnouncementRequestStateApproved,
			CreatedBy:          "user-3",
			ResolvedProjectIDs: []string{"proj-1"},
		}}
		svc := NewAnnouncementRequestService(repo, nil, nil)
		errMsg := "downstream timeout"
		_, err := svc.RecordDeliveries(context.Background(), "req-1", "user-3", []domain.RecordAnnouncementRequestDeliveryInput{
			{ProjectID: "proj-1", Status: domain.AnnouncementRequestDeliveryStatusFailed, ErrorMessage: &errMsg},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// A failed delivery never has a real case — accepting a caseId here would
	// let a row claim both "the create failed" and "a case exists for it" at
	// once, which every reader of this ledger (including
	// usePublishAnnouncementRequest's own hydration) assumes can't happen.
	t.Run("rejects failed with a caseId", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{
			State:              domain.AnnouncementRequestStateApproved,
			CreatedBy:          "user-3",
			ResolvedProjectIDs: []string{"proj-1"},
		}}
		svc := NewAnnouncementRequestService(repo, nil, nil)
		caseID := "case-1"
		_, err := svc.RecordDeliveries(context.Background(), "req-1", "user-3", []domain.RecordAnnouncementRequestDeliveryInput{
			{ProjectID: "proj-1", CaseID: &caseID, Status: domain.AnnouncementRequestDeliveryStatusFailed},
		})
		var ve *apierror.ValidationError
		if !isValidationError(err, &ve) {
			t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
		}
	})

	t.Run("rejects a non-creator actor", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{
			State:              domain.AnnouncementRequestStateApproved,
			CreatedBy:          "user-1",
			ResolvedProjectIDs: []string{"proj-1"},
		}}
		svc := NewAnnouncementRequestService(repo, nil, nil)
		caseID := "case-1"
		_, err := svc.RecordDeliveries(context.Background(), "req-1", "user-3", []domain.RecordAnnouncementRequestDeliveryInput{
			{ProjectID: "proj-1", CaseID: &caseID, Status: domain.AnnouncementRequestDeliveryStatusSucceeded},
		})
		if _, ok := err.(*apierror.ForbiddenError); !ok {
			t.Fatalf("expected *apierror.ForbiddenError, got %T: %v", err, err)
		}
	})

	t.Run("rejects from any state other than approved", func(t *testing.T) {
		for _, state := range []domain.AnnouncementRequestState{
			domain.AnnouncementRequestStateDraft,
			domain.AnnouncementRequestStatePendingApproval,
			domain.AnnouncementRequestStatePublished,
		} {
			t.Run(string(state), func(t *testing.T) {
				repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{
					State: state, CreatedBy: "user-3", ResolvedProjectIDs: []string{"proj-1"},
				}}
				svc := NewAnnouncementRequestService(repo, nil, nil)
				caseID := "case-1"
				_, err := svc.RecordDeliveries(context.Background(), "req-1", "user-3", []domain.RecordAnnouncementRequestDeliveryInput{
					{ProjectID: "proj-1", CaseID: &caseID, Status: domain.AnnouncementRequestDeliveryStatusSucceeded},
				})
				if err == nil {
					t.Fatalf("expected a conflict error recording deliveries from state %q, got nil", state)
				}
			})
		}
	})
}

func TestAnnouncementRequestService_ListDeliveries(t *testing.T) {
	t.Run("returns the repo's deliveries for an existing request", func(t *testing.T) {
		want := []domain.AnnouncementRequestDelivery{
			{ID: "d-1", ProjectID: "proj-1", Status: domain.AnnouncementRequestDeliveryStatusSucceeded},
		}
		repo := &fakeAnnouncementRequestRepo{
			getResult:            domain.AnnouncementRequest{ID: "req-1", State: domain.AnnouncementRequestStateApproved},
			listDeliveriesResult: want,
		}
		svc := NewAnnouncementRequestService(repo, nil, nil)
		resp, err := svc.ListDeliveries(context.Background(), "req-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.gotListDeliveriesID != "req-1" {
			t.Fatalf("expected the id forwarded to the repo, got %q", repo.gotListDeliveriesID)
		}
		if len(resp.Deliveries) != 1 || resp.Deliveries[0].ID != "d-1" {
			t.Fatalf("expected the repo's deliveries on the result, got %+v", resp.Deliveries)
		}
	})

	t.Run("returns NotFoundError for a nonexistent request without calling ListDeliveries", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getErr: &apierror.NotFoundError{Msg: "announcement request not found: req-1"}}
		svc := NewAnnouncementRequestService(repo, nil, nil)
		_, err := svc.ListDeliveries(context.Background(), "req-1")
		if _, ok := err.(*apierror.NotFoundError); !ok {
			t.Fatalf("expected *apierror.NotFoundError, got %T: %v", err, err)
		}
		if repo.gotListDeliveriesID != "" {
			t.Fatal("expected ListDeliveries not to be called for a nonexistent request")
		}
	})
}
