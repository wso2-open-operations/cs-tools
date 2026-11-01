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
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// stubTimeCardRepo is a minimal repository.TimeCardRepository whose
// unconfigured methods panic if called -- same convention as
// stubCallRequestRepo (call_request_service_test.go).
type stubTimeCardRepo struct {
	createTimeCard          func(ctx context.Context, req domain.CreateTimeCardRequest, userID string) (domain.TimeCardView, error)
	updateTimeCardFields    func(ctx context.Context, req domain.UpdateTimeCardRequest, actorID string) (domain.TimeCardView, error)
	transitionTimeCardState func(ctx context.Context, id string, state domain.TimeCardState, leadComment *string, actorID string) (domain.TimeCardView, error)
	deleteTimeCard          func(ctx context.Context, id, submitterID string) error
	setTimeCardSNSysID      func(ctx context.Context, id, snSysID string) error
	getTimeCardSNSysID      func(ctx context.Context, id string) (*string, error)
}

func (s *stubTimeCardRepo) SearchTimeCards(context.Context, domain.SearchTimeCardsRequest, string) ([]domain.TimeCardView, int, error) {
	panic("not implemented")
}
func (s *stubTimeCardRepo) SearchCaseTimeCards(context.Context, domain.SearchTimeCardsRequest, string) ([]domain.CaseTimeCardSummary, int, error) {
	panic("not implemented")
}
func (s *stubTimeCardRepo) CreateTimeCard(ctx context.Context, req domain.CreateTimeCardRequest, userID string) (domain.TimeCardView, error) {
	if s.createTimeCard != nil {
		return s.createTimeCard(ctx, req, userID)
	}
	panic("not implemented")
}
func (s *stubTimeCardRepo) UpdateTimeCardFields(ctx context.Context, req domain.UpdateTimeCardRequest, actorID string) (domain.TimeCardView, error) {
	if s.updateTimeCardFields != nil {
		return s.updateTimeCardFields(ctx, req, actorID)
	}
	panic("not implemented")
}
func (s *stubTimeCardRepo) TransitionTimeCardState(ctx context.Context, id string, state domain.TimeCardState, leadComment *string, actorID string) (domain.TimeCardView, error) {
	if s.transitionTimeCardState != nil {
		return s.transitionTimeCardState(ctx, id, state, leadComment, actorID)
	}
	panic("not implemented")
}
func (s *stubTimeCardRepo) DeleteTimeCard(ctx context.Context, id, submitterID string) error {
	if s.deleteTimeCard != nil {
		return s.deleteTimeCard(ctx, id, submitterID)
	}
	panic("not implemented")
}
func (s *stubTimeCardRepo) SetTimeCardSNSysID(ctx context.Context, id, snSysID string) error {
	if s.setTimeCardSNSysID != nil {
		return s.setTimeCardSNSysID(ctx, id, snSysID)
	}
	return nil
}
func (s *stubTimeCardRepo) GetTimeCardSNSysID(ctx context.Context, id string) (*string, error) {
	if s.getTimeCardSNSysID != nil {
		return s.getTimeCardSNSysID(ctx, id)
	}
	return nil, nil
}

// stubMirrorTimeCardService embeds TimeCardService (nil) and overrides only
// CreateTimeCard -- same convention as stubMirrorCallRequestService.
type stubMirrorTimeCardService struct {
	TimeCardService
	createTimeCard func(ctx context.Context, req domain.CreateTimeCardRequest) (domain.TimeCardMutationResponse, error)
	updateTimeCard func(ctx context.Context, req domain.UpdateTimeCardRequest) (domain.TimeCardMutationResponse, error)
	deleteTimeCard func(ctx context.Context, req domain.DeleteTimeCardRequest) (domain.DeleteTimeCardResponse, error)
}

func (s *stubMirrorTimeCardService) CreateTimeCard(ctx context.Context, req domain.CreateTimeCardRequest) (domain.TimeCardMutationResponse, error) {
	return s.createTimeCard(ctx, req)
}

func (s *stubMirrorTimeCardService) UpdateTimeCard(ctx context.Context, req domain.UpdateTimeCardRequest) (domain.TimeCardMutationResponse, error) {
	return s.updateTimeCard(ctx, req)
}

func (s *stubMirrorTimeCardService) DeleteTimeCard(ctx context.Context, req domain.DeleteTimeCardRequest) (domain.DeleteTimeCardResponse, error) {
	return s.deleteTimeCard(ctx, req)
}

// testApproverUUID is a distinct id from testUUID (the submitting caller's
// own resolved user id in these tests) specifically so ApproverIDs never
// accidentally names the submitter as their own approver -- validateApproverIDsExcludeSubmitter
// would otherwise reject every one of these requests.
const testApproverUUID = "77777777-0000-4000-8000-000000000003"

func validCreateTimeCardRequest() domain.CreateTimeCardRequest {
	return domain.CreateTimeCardRequest{
		CaseID: testUUID, ProjectID: testUUID, Date: "2026-09-01",
		ApproverIDs: []string{testApproverUUID}, TimeAnalyzing: 10,
	}
}

// TestTimeCardService_CreateTimeCard_MirrorsToServiceNow covers the
// writeback wiring: on a successful Postgres create, the mirror's
// CreateTimeCard is dispatched asynchronously and does not block or affect
// the response.
func TestTimeCardService_CreateTimeCard_MirrorsToServiceNow(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := validCreateTimeCardRequest()

	called := make(chan domain.CreateTimeCardRequest, 1)
	mirror := &stubMirrorTimeCardService{
		createTimeCard: func(_ context.Context, mirrorReq domain.CreateTimeCardRequest) (domain.TimeCardMutationResponse, error) {
			called <- mirrorReq
			return domain.TimeCardMutationResponse{}, nil
		},
	}
	repo := &stubTimeCardRepo{
		createTimeCard: func(context.Context, domain.CreateTimeCardRequest, string) (domain.TimeCardView, error) {
			return domain.TimeCardView{ID: testUUID}, nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	svc := NewTimeCardServiceWithSNWriteback(repo, stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: testUUID, Email: "jane.doe@example.com"}, nil
		},
	}, dispatcher, mirror)

	if _, err := svc.CreateTimeCard(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case got := <-called:
		if got.CaseID != req.CaseID || got.Date != req.Date {
			t.Errorf("mirror got %+v, want caseId/date to match %+v", got, req)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("mirror.CreateTimeCard was never called")
	}
	if got := failures.count(); got != 0 {
		t.Errorf("expected 0 sn_writeback_failures records for a successful mirror, got %d", got)
	}
}

// TestTimeCardService_CreateTimeCard_MirrorFailureRecordsWritebackFailure
// covers the failure half: Postgres already succeeded, so the call must
// still report success, but the mirror error lands in sn_writeback_failures
// for manual backfill.
func TestTimeCardService_CreateTimeCard_MirrorFailureRecordsWritebackFailure(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := validCreateTimeCardRequest()

	mirror := &stubMirrorTimeCardService{
		createTimeCard: func(context.Context, domain.CreateTimeCardRequest) (domain.TimeCardMutationResponse, error) {
			return domain.TimeCardMutationResponse{}, errors.New("sn downstream unreachable")
		},
	}
	repo := &stubTimeCardRepo{
		createTimeCard: func(context.Context, domain.CreateTimeCardRequest, string) (domain.TimeCardView, error) {
			return domain.TimeCardView{ID: testUUID}, nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	svc := NewTimeCardServiceWithSNWriteback(repo, stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: testUUID, Email: "jane.doe@example.com"}, nil
		},
	}, dispatcher, mirror)

	if _, err := svc.CreateTimeCard(ctx, req); err != nil {
		t.Fatalf("expected the Postgres-side success to be reported despite the mirror failure, got %v", err)
	}

	waitFor(t, func() bool { return failures.count() == 1 })
}

// TestTimeCardService_CreateTimeCard_MirrorSuccessPersistsSNSysID covers the
// id-mapping half of the CREATE mirror: on success, the ServiceNow-returned
// time card id (converted to its raw sys_id form) is persisted back onto the
// Postgres row via SetTimeCardSNSysID.
func TestTimeCardService_CreateTimeCard_MirrorSuccessPersistsSNSysID(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := validCreateTimeCardRequest()

	mirror := &stubMirrorTimeCardService{
		createTimeCard: func(context.Context, domain.CreateTimeCardRequest) (domain.TimeCardMutationResponse, error) {
			return domain.TimeCardMutationResponse{TimeCard: &domain.TimeCardView{ID: testSNSysID2}}, nil
		},
	}
	type setCall struct{ id, snSysID string }
	setCalls := make(chan setCall, 1)
	repo := &stubTimeCardRepo{
		createTimeCard: func(context.Context, domain.CreateTimeCardRequest, string) (domain.TimeCardView, error) {
			return domain.TimeCardView{ID: testUUID}, nil
		},
		setTimeCardSNSysID: func(_ context.Context, id, snSysID string) error {
			setCalls <- setCall{id: id, snSysID: snSysID}
			return nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	svc := NewTimeCardServiceWithSNWriteback(repo, stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: testUUID, Email: "jane.doe@example.com"}, nil
		},
	}, dispatcher, mirror)

	if _, err := svc.CreateTimeCard(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case got := <-setCalls:
		if got.id != testUUID {
			t.Errorf("SetTimeCardSNSysID id = %q, want %q (the Postgres row id)", got.id, testUUID)
		}
		wantSysID := uuidToSysid(testSNSysID2)
		if got.snSysID != wantSysID {
			t.Errorf("SetTimeCardSNSysID snSysID = %q, want %q", got.snSysID, wantSysID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SetTimeCardSNSysID was never called")
	}
}

// TestTimeCardService_UpdateTimeCard_MirrorsWithStoredSNSysID covers the
// UPDATE mirror's happy path (field-edit branch): when a ServiceNow sys_id
// is already stored, the mirror fires against it.
func TestTimeCardService_UpdateTimeCard_MirrorsWithStoredSNSysID(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := domain.UpdateTimeCardRequest{ID: testUUID, WorkLogComment: strPtrTC("updated")}

	storedSysID := uuidToSysid(testSNSysID2)
	repo := &stubTimeCardRepo{
		updateTimeCardFields: func(context.Context, domain.UpdateTimeCardRequest, string) (domain.TimeCardView, error) {
			return domain.TimeCardView{ID: testUUID}, nil
		},
		getTimeCardSNSysID: func(context.Context, string) (*string, error) {
			return &storedSysID, nil
		},
	}
	called := make(chan domain.UpdateTimeCardRequest, 1)
	mirror := &stubMirrorTimeCardService{
		updateTimeCard: func(_ context.Context, mirrorReq domain.UpdateTimeCardRequest) (domain.TimeCardMutationResponse, error) {
			called <- mirrorReq
			return domain.TimeCardMutationResponse{}, nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	svc := NewTimeCardServiceWithSNWriteback(repo, stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: testUUID, Email: "jane.doe@example.com"}, nil
		},
	}, dispatcher, mirror)

	if _, err := svc.UpdateTimeCard(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case got := <-called:
		if got.ID != testSNSysID2 {
			t.Errorf("mirror UpdateTimeCard ID = %q, want %q (sysidToUUID of the stored sys_id)", got.ID, testSNSysID2)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("mirror.UpdateTimeCard was never called")
	}
	if got := failures.count(); got != 0 {
		t.Errorf("expected 0 sn_writeback_failures records for a successful mirror, got %d", got)
	}
}

// TestTimeCardService_UpdateTimeCard_SkipsMirrorWhenNoSNSysIDStored covers
// the pre-migration-row case: a NULL sn_sys_id must make the UPDATE mirror
// skip silently.
func TestTimeCardService_UpdateTimeCard_SkipsMirrorWhenNoSNSysIDStored(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := domain.UpdateTimeCardRequest{ID: testUUID, WorkLogComment: strPtrTC("updated")}

	repo := &stubTimeCardRepo{
		updateTimeCardFields: func(context.Context, domain.UpdateTimeCardRequest, string) (domain.TimeCardView, error) {
			return domain.TimeCardView{ID: testUUID}, nil
		},
		getTimeCardSNSysID: func(context.Context, string) (*string, error) {
			return nil, nil
		},
	}
	mirrorCalled := make(chan struct{}, 1)
	mirror := &stubMirrorTimeCardService{
		updateTimeCard: func(context.Context, domain.UpdateTimeCardRequest) (domain.TimeCardMutationResponse, error) {
			mirrorCalled <- struct{}{}
			return domain.TimeCardMutationResponse{}, nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	svc := NewTimeCardServiceWithSNWriteback(repo, stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: testUUID, Email: "jane.doe@example.com"}, nil
		},
	}, dispatcher, mirror)

	if _, err := svc.UpdateTimeCard(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case <-mirrorCalled:
		t.Fatal("mirror.UpdateTimeCard was called despite no ServiceNow mapping being stored")
	case <-time.After(300 * time.Millisecond):
		// expected: no mirror call
	}
	if got := failures.count(); got != 0 {
		t.Errorf("expected 0 sn_writeback_failures records for a deliberate skip, got %d", got)
	}
}

// TestTimeCardService_DeleteTimeCard_MirrorsWithStoredSNSysID covers the
// DELETE mirror's happy path: the sys_id lookup happens BEFORE the Postgres
// delete (the row disappears afterward), and the mirror fires with it.
func TestTimeCardService_DeleteTimeCard_MirrorsWithStoredSNSysID(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := domain.DeleteTimeCardRequest{ID: testUUID}

	storedSysID := uuidToSysid(testSNSysID2)
	repo := &stubTimeCardRepo{
		deleteTimeCard: func(context.Context, string, string) error { return nil },
		getTimeCardSNSysID: func(context.Context, string) (*string, error) {
			return &storedSysID, nil
		},
	}
	called := make(chan domain.DeleteTimeCardRequest, 1)
	mirror := &stubMirrorTimeCardService{
		deleteTimeCard: func(_ context.Context, mirrorReq domain.DeleteTimeCardRequest) (domain.DeleteTimeCardResponse, error) {
			called <- mirrorReq
			return domain.DeleteTimeCardResponse{}, nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	svc := NewTimeCardServiceWithSNWriteback(repo, stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: testUUID, Email: "jane.doe@example.com"}, nil
		},
	}, dispatcher, mirror)

	if _, err := svc.DeleteTimeCard(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case got := <-called:
		if got.ID != testSNSysID2 {
			t.Errorf("mirror DeleteTimeCard ID = %q, want %q (sysidToUUID of the stored sys_id)", got.ID, testSNSysID2)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("mirror.DeleteTimeCard was never called")
	}
	if got := failures.count(); got != 0 {
		t.Errorf("expected 0 sn_writeback_failures records for a successful mirror, got %d", got)
	}
}

// TestTimeCardService_DeleteTimeCard_SkipsMirrorWhenNoSNSysIDStored covers
// the pre-migration-row case for DELETE: a NULL sn_sys_id must make the
// mirror skip silently, and the Postgres delete must still succeed.
func TestTimeCardService_DeleteTimeCard_SkipsMirrorWhenNoSNSysIDStored(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := domain.DeleteTimeCardRequest{ID: testUUID}

	repo := &stubTimeCardRepo{
		deleteTimeCard: func(context.Context, string, string) error { return nil },
		getTimeCardSNSysID: func(context.Context, string) (*string, error) {
			return nil, nil
		},
	}
	mirrorCalled := make(chan struct{}, 1)
	mirror := &stubMirrorTimeCardService{
		deleteTimeCard: func(context.Context, domain.DeleteTimeCardRequest) (domain.DeleteTimeCardResponse, error) {
			mirrorCalled <- struct{}{}
			return domain.DeleteTimeCardResponse{}, nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	svc := NewTimeCardServiceWithSNWriteback(repo, stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: testUUID, Email: "jane.doe@example.com"}, nil
		},
	}, dispatcher, mirror)

	if _, err := svc.DeleteTimeCard(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case <-mirrorCalled:
		t.Fatal("mirror.DeleteTimeCard was called despite no ServiceNow mapping being stored")
	case <-time.After(300 * time.Millisecond):
		// expected: no mirror call
	}
	if got := failures.count(); got != 0 {
		t.Errorf("expected 0 sn_writeback_failures records for a deliberate skip, got %d", got)
	}
}

// TestTimeCardService_DeleteTimeCard_RecordsSNWritebackFailureOnLookupError
// covers the real-error half of the same lookup: unlike a genuinely
// successful lookup that finds no mapping (nil, nil -- see
// TestTimeCardService_DeleteTimeCard_SkipsMirrorWhenNoSNSysIDStored), a
// lookup that fails outright must not be silently discarded. The Postgres
// delete still succeeds and is still authoritative, but the lookup failure
// is recorded to sn_writeback_failures for manual backfill instead of
// vanishing.
func TestTimeCardService_DeleteTimeCard_RecordsSNWritebackFailureOnLookupError(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := domain.DeleteTimeCardRequest{ID: testUUID}

	lookupErr := errors.New("sn sys id lookup: connection reset")
	repo := &stubTimeCardRepo{
		deleteTimeCard: func(context.Context, string, string) error { return nil },
		getTimeCardSNSysID: func(context.Context, string) (*string, error) {
			return nil, lookupErr
		},
	}
	mirrorCalled := make(chan struct{}, 1)
	mirror := &stubMirrorTimeCardService{
		deleteTimeCard: func(context.Context, domain.DeleteTimeCardRequest) (domain.DeleteTimeCardResponse, error) {
			mirrorCalled <- struct{}{}
			return domain.DeleteTimeCardResponse{}, nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	svc := NewTimeCardServiceWithSNWriteback(repo, stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: testUUID, Email: "jane.doe@example.com"}, nil
		},
	}, dispatcher, mirror)

	if _, err := svc.DeleteTimeCard(ctx, req); err != nil {
		t.Fatalf("Postgres delete must still be reported as a success despite the lookup error, got: %v", err)
	}

	select {
	case <-mirrorCalled:
		t.Fatal("mirror.DeleteTimeCard was called despite the sys_id lookup having failed")
	case <-time.After(300 * time.Millisecond):
		// expected: no mirror call, the lookup error means we don't know
		// what to delete
	}

	waitFor(t, func() bool { return failures.count() == 1 })
	failReq := failures.calls[0]
	if failReq.EntityType != "time_card" || failReq.EntityID != testUUID || failReq.Operation != "delete" {
		t.Errorf("unexpected failure record: %+v", failReq)
	}
	if failReq.Error != lookupErr.Error() {
		t.Errorf("failure record Error = %q, want %q", failReq.Error, lookupErr.Error())
	}
}

// TestTimeCardService_CreateTimeCard_RejectsSelfApprover verifies the
// backend half of the "no self-approval" rule: a submitter naming
// themselves as their own card's approver is rejected before the
// repository is ever called.
func TestTimeCardService_CreateTimeCard_RejectsSelfApprover(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := domain.CreateTimeCardRequest{
		CaseID: testUUID, Date: "2026-09-01",
		ApproverIDs: []string{testUUID}, TimeAnalyzing: 10,
	}

	repo := &stubTimeCardRepo{
		createTimeCard: func(context.Context, domain.CreateTimeCardRequest, string) (domain.TimeCardView, error) {
			t.Fatal("repository.CreateTimeCard must not be called when the submitter named themselves as their own approver")
			return domain.TimeCardView{}, nil
		},
	}
	svc := NewTimeCardService(repo, stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: testUUID, Email: "jane.doe@example.com"}, nil
		},
	})

	_, err := svc.CreateTimeCard(ctx, req)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if got := err.Error(); got != "you cannot be your own time card approver" {
		t.Errorf("error = %q, want the self-approval message", got)
	}
}

// TestTimeCardService_UpdateTimeCard_RejectsSelfApprover is the same check
// on the field-edit path: UpdateTimeCardFields's own WHERE clause already
// guarantees the row's submitter equals the acting caller, so comparing
// ApproverIDs against actorID here is both cheap and correct.
func TestTimeCardService_UpdateTimeCard_RejectsSelfApprover(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := domain.UpdateTimeCardRequest{ID: testUUID, ApproverIDs: []string{testUUID}}

	repo := &stubTimeCardRepo{
		updateTimeCardFields: func(context.Context, domain.UpdateTimeCardRequest, string) (domain.TimeCardView, error) {
			t.Fatal("repository.UpdateTimeCardFields must not be called when the actor named themselves as their own approver")
			return domain.TimeCardView{}, nil
		},
	}
	svc := NewTimeCardService(repo, stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: testUUID, Email: "jane.doe@example.com"}, nil
		},
	})

	_, err := svc.UpdateTimeCard(ctx, req)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if got := err.Error(); got != "you cannot be your own time card approver" {
		t.Errorf("error = %q, want the self-approval message", got)
	}
}

// TestTimeCardService_CreateTimeCard_PerCardLimit: the limit is 480 minutes
// per card (sum of the five duration fields). 481 is rejected before the
// repository is called, 480 is accepted, and a second card on the same
// case/day is judged on its own, not against a cumulative total.
func TestTimeCardService_CreateTimeCard_PerCardLimit(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	userRepo := stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: testUUID, Email: "jane.doe@example.com"}, nil
		},
	}

	t.Run("481 minutes in one field is rejected", func(t *testing.T) {
		req := validCreateTimeCardRequest()
		req.TimeAnalyzing = 481
		repo := &stubTimeCardRepo{
			createTimeCard: func(context.Context, domain.CreateTimeCardRequest, string) (domain.TimeCardView, error) {
				t.Fatal("repository.CreateTimeCard must not be called for an over-limit card")
				return domain.TimeCardView{}, nil
			},
		}
		_, err := NewTimeCardService(repo, userRepo).CreateTimeCard(ctx, req)
		if err == nil || err.Error() != domain.MaxTimeCardMinutesMessage {
			t.Fatalf("error = %v, want %q", err, domain.MaxTimeCardMinutesMessage)
		}
	})

	t.Run("481 minutes summed across fields is rejected", func(t *testing.T) {
		req := validCreateTimeCardRequest()
		req.TimeAnalyzing, req.TimeSettingUp, req.TimeReproducingDebugging, req.TimeProvidingSolution, req.TimePatching = 100, 100, 100, 100, 81
		repo := &stubTimeCardRepo{
			createTimeCard: func(context.Context, domain.CreateTimeCardRequest, string) (domain.TimeCardView, error) {
				t.Fatal("repository.CreateTimeCard must not be called for an over-limit card")
				return domain.TimeCardView{}, nil
			},
		}
		_, err := NewTimeCardService(repo, userRepo).CreateTimeCard(ctx, req)
		if err == nil || err.Error() != domain.MaxTimeCardMinutesMessage {
			t.Fatalf("error = %v, want %q", err, domain.MaxTimeCardMinutesMessage)
		}
	})

	t.Run("values whose sum would overflow are rejected", func(t *testing.T) {
		req := validCreateTimeCardRequest()
		req.TimeAnalyzing, req.TimeSettingUp = math.MaxInt64, 1
		repo := &stubTimeCardRepo{
			createTimeCard: func(context.Context, domain.CreateTimeCardRequest, string) (domain.TimeCardView, error) {
				t.Fatal("repository.CreateTimeCard must not be called for an over-limit card")
				return domain.TimeCardView{}, nil
			},
		}
		_, err := NewTimeCardService(repo, userRepo).CreateTimeCard(ctx, req)
		if err == nil || err.Error() != domain.MaxTimeCardMinutesMessage {
			t.Fatalf("error = %v, want %q", err, domain.MaxTimeCardMinutesMessage)
		}
	})

	t.Run("480 then 30 on the same case and day are both accepted", func(t *testing.T) {
		calls := 0
		repo := &stubTimeCardRepo{
			createTimeCard: func(context.Context, domain.CreateTimeCardRequest, string) (domain.TimeCardView, error) {
				calls++
				return domain.TimeCardView{ID: testUUID}, nil
			},
		}
		svc := NewTimeCardService(repo, userRepo)
		for _, minutes := range []int{480, 30} {
			req := validCreateTimeCardRequest()
			req.TimeAnalyzing = minutes
			if _, err := svc.CreateTimeCard(ctx, req); err != nil {
				t.Fatalf("CreateTimeCard(%d) error = %v, want success", minutes, err)
			}
		}
		if calls != 2 {
			t.Errorf("repository.CreateTimeCard calls = %d, want 2", calls)
		}
	})
}

// TestTimeCardService_UpdateTimeCard_PerCardLimit: an edit supplying
// over-limit minutes is rejected before the repository is called; the
// merged-total check against stored values is the repository's own (see its
// integration tests).
func TestTimeCardService_UpdateTimeCard_PerCardLimit(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	userRepo := stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: testUUID, Email: "jane.doe@example.com"}, nil
		},
	}
	over, ok := 481, 480

	repo := &stubTimeCardRepo{
		updateTimeCardFields: func(_ context.Context, req domain.UpdateTimeCardRequest, _ string) (domain.TimeCardView, error) {
			if req.TimeAnalyzing != nil && *req.TimeAnalyzing > domain.MaxTimeCardMinutes {
				t.Fatal("repository.UpdateTimeCardFields must not be called for an over-limit card")
			}
			return domain.TimeCardView{ID: testUUID}, nil
		},
	}
	svc := NewTimeCardService(repo, userRepo)

	_, err := svc.UpdateTimeCard(ctx, domain.UpdateTimeCardRequest{ID: testUUID, TimeAnalyzing: &over})
	if err == nil || err.Error() != domain.MaxTimeCardMinutesMessage {
		t.Fatalf("UpdateTimeCard(481) error = %v, want %q", err, domain.MaxTimeCardMinutesMessage)
	}
	if _, err := svc.UpdateTimeCard(ctx, domain.UpdateTimeCardRequest{ID: testUUID, TimeAnalyzing: &ok}); err != nil {
		t.Fatalf("UpdateTimeCard(480) error = %v, want success", err)
	}
}

// TestValidateApproverIDsExcludeSubmitter pins the helper's own contract
// directly, including the case-insensitive comparison (ids can arrive with
// different casing from different callers).
func TestValidateApproverIDsExcludeSubmitter(t *testing.T) {
	tests := []struct {
		name        string
		approverIDs []string
		submitterID string
		wantErr     bool
	}{
		{"no approvers", nil, testUUID, false},
		{"unrelated approver", []string{testApproverUUID}, testUUID, false},
		{"self among several", []string{testApproverUUID, testUUID}, testUUID, true},
		{"self, different case", []string{strings.ToUpper(testUUID)}, testUUID, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateApproverIDsExcludeSubmitter(tt.approverIDs, tt.submitterID)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateApproverIDsExcludeSubmitter(%v, %q) error = %v, wantErr %v", tt.approverIDs, tt.submitterID, err, tt.wantErr)
			}
		})
	}
}

func strPtrTC(s string) *string { return &s }
