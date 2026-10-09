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
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	testConversationID        = "22222222-0000-4000-8000-000000000003"
	testConversationProjectID = "22222222-0000-4000-8000-000000000004"
)

// stubConversationRepo is a minimal repository.ConversationRepository whose
// unconfigured methods panic if called -- same convention as
// stubCallRequestRepo.
type stubConversationRepo struct {
	searchConversations func(ctx context.Context, req domain.SearchConversationsRequest, callerEmail string) ([]domain.SearchConversationView, int, error)
	updateConversation  func(ctx context.Context, id string, state domain.ConversationState, actorEmail string) (domain.UpdatedConversation, error)
	createConversation  func(ctx context.Context, in repository.CreateConversationInput) (domain.CreatedConversation, error)
}

func (s *stubConversationRepo) SearchConversations(ctx context.Context, req domain.SearchConversationsRequest, callerEmail string) ([]domain.SearchConversationView, int, error) {
	if s.searchConversations != nil {
		return s.searchConversations(ctx, req, callerEmail)
	}
	panic("not implemented")
}
func (s *stubConversationRepo) GetConversation(context.Context, string) (domain.ConversationDetails, error) {
	panic("not implemented")
}
func (s *stubConversationRepo) UpdateConversation(ctx context.Context, id string, state domain.ConversationState, actorEmail string) (domain.UpdatedConversation, error) {
	if s.updateConversation != nil {
		return s.updateConversation(ctx, id, state, actorEmail)
	}
	panic("not implemented")
}
func (s *stubConversationRepo) CreateConversation(ctx context.Context, in repository.CreateConversationInput) (domain.CreatedConversation, error) {
	if s.createConversation != nil {
		return s.createConversation(ctx, in)
	}
	panic("not implemented")
}

// stubMirrorConversationService embeds ConversationService (nil) and
// overrides only UpdateConversation -- same convention as
// stubMirrorCallRequestService.
type stubMirrorConversationService struct {
	ConversationService
	updateConversation func(ctx context.Context, id string, req domain.UpdateConversationRequest) (domain.UpdateConversationResponse, error)
	createConversation func(ctx context.Context, req domain.CreateConversationRequest) (domain.CreateConversationResponse, error)
}

func (s *stubMirrorConversationService) CreateConversation(ctx context.Context, req domain.CreateConversationRequest) (domain.CreateConversationResponse, error) {
	return s.createConversation(ctx, req)
}

func (s *stubMirrorConversationService) UpdateConversation(ctx context.Context, id string, req domain.UpdateConversationRequest) (domain.UpdateConversationResponse, error) {
	return s.updateConversation(ctx, id, req)
}

// TestConversationService_UpdateConversation_MirrorsToServiceNow covers the
// writeback wiring added for fix 3: on a successful Postgres update, the
// mirror's UpdateConversation is dispatched asynchronously and does not
// block or affect the response, and a successful mirror records 0
// sn_writeback_failures.
func TestConversationService_UpdateConversation_MirrorsToServiceNow(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := domain.UpdateConversationRequest{State: domain.ConversationStateActive}

	type call struct {
		id  string
		req domain.UpdateConversationRequest
	}
	called := make(chan call, 1)
	mirror := &stubMirrorConversationService{
		updateConversation: func(_ context.Context, id string, mirrorReq domain.UpdateConversationRequest) (domain.UpdateConversationResponse, error) {
			called <- call{id: id, req: mirrorReq}
			return domain.UpdateConversationResponse{}, nil
		},
	}
	repo := &stubConversationRepo{
		updateConversation: func(context.Context, string, domain.ConversationState, string) (domain.UpdatedConversation, error) {
			return domain.UpdatedConversation{ID: testConversationID}, nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	svc := NewConversationServiceWithSNWriteback(repo, dispatcher, mirror)

	if _, err := svc.UpdateConversation(ctx, testConversationID, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case got := <-called:
		if got.id != testConversationID {
			t.Errorf("mirror UpdateConversation id = %q, want %q", got.id, testConversationID)
		}
		if got.req.State != req.State {
			t.Errorf("mirror UpdateConversation state = %q, want %q", got.req.State, req.State)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("mirror.UpdateConversation was never called")
	}
	if got := failures.count(); got != 0 {
		t.Errorf("expected 0 sn_writeback_failures records for a successful mirror, got %d", got)
	}
}

// TestConversationService_UpdateConversation_MirrorFailureRecordsWritebackFailure
// covers the failure half: Postgres already succeeded, so the call must
// still report success, but the mirror error lands in sn_writeback_failures
// for manual backfill.
func TestConversationService_UpdateConversation_MirrorFailureRecordsWritebackFailure(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := domain.UpdateConversationRequest{State: domain.ConversationStateResolved}

	mirror := &stubMirrorConversationService{
		updateConversation: func(context.Context, string, domain.UpdateConversationRequest) (domain.UpdateConversationResponse, error) {
			return domain.UpdateConversationResponse{}, errors.New("sn downstream unreachable")
		},
	}
	repo := &stubConversationRepo{
		updateConversation: func(context.Context, string, domain.ConversationState, string) (domain.UpdatedConversation, error) {
			return domain.UpdatedConversation{ID: testConversationID}, nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	svc := NewConversationServiceWithSNWriteback(repo, dispatcher, mirror)

	if _, err := svc.UpdateConversation(ctx, testConversationID, req); err != nil {
		t.Fatalf("expected the Postgres-side success to be reported despite the mirror failure, got %v", err)
	}

	waitFor(t, func() bool { return failures.count() == 1 })
}

// TestConversationService_UpdateConversation_NoMirrorOnPlainPostgres covers
// the plain-Postgres (non-dual-write) regression guard: with snWriteback/
// snMirror both nil (NewConversationService, not the SNWriteback
// constructor), UpdateConversation must still succeed and must never touch
// any mirror.
func TestConversationService_UpdateConversation_NoMirrorOnPlainPostgres(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := domain.UpdateConversationRequest{State: domain.ConversationStateClosed}

	repo := &stubConversationRepo{
		updateConversation: func(context.Context, string, domain.ConversationState, string) (domain.UpdatedConversation, error) {
			return domain.UpdatedConversation{ID: testConversationID}, nil
		},
	}
	svc := NewConversationService(repo)

	if _, err := svc.UpdateConversation(ctx, testConversationID, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestConversationService_CreateConversation_Postgres covers the native
// create: the row is attributed to the caller's token email, starts ACTIVE,
// carries the full first message as its description and a 100-rune subject,
// and leaves id/number for the repository to generate.
func TestConversationService_CreateConversation_Postgres(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	msg := strings.Repeat("é", 150)

	var got repository.CreateConversationInput
	repo := &stubConversationRepo{
		createConversation: func(_ context.Context, in repository.CreateConversationInput) (domain.CreatedConversation, error) {
			got = in
			return domain.CreatedConversation{ID: testConversationID, Number: "CS-PORTAL-000001"}, nil
		},
	}
	resp, err := NewConversationService(repo).CreateConversation(ctx, domain.CreateConversationRequest{
		ProjectID: testConversationProjectID, InitialMessage: msg,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Conversation.ID != testConversationID {
		t.Errorf("conversation id = %q, want %q", resp.Conversation.ID, testConversationID)
	}
	if got.ID != "" || got.Number != "" {
		t.Errorf("id/number = %q/%q, want both empty so the repository generates them", got.ID, got.Number)
	}
	if got.CreatedBy != "jane.doe@example.com" {
		t.Errorf("createdBy = %q, want the token email", got.CreatedBy)
	}
	if got.State != domain.ConversationStateActive {
		t.Errorf("state = %q, want ACTIVE", got.State)
	}
	if got.InitialMessage != msg {
		t.Error("initial message was not passed through in full")
	}
	if want := strings.Repeat("é", 100); got.Subject != want {
		t.Errorf("subject has %d runes, want 100", utf8.RuneCountInString(got.Subject))
	}
}

// TestConversationService_CreateConversation_Validation covers the checks
// that run before anything is written.
func TestConversationService_CreateConversation_Validation(t *testing.T) {
	svc := NewConversationService(&stubConversationRepo{}) // repo panics if reached
	withToken := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	cases := []struct {
		name string
		ctx  context.Context
		req  domain.CreateConversationRequest
		want any
	}{
		{"bad project id", withToken, domain.CreateConversationRequest{ProjectID: "nope", InitialMessage: "hi"}, &apierror.ValidationError{}},
		{"empty message", withToken, domain.CreateConversationRequest{ProjectID: testConversationProjectID}, &apierror.ValidationError{}},
		{"no token", context.Background(), domain.CreateConversationRequest{ProjectID: testConversationProjectID, InitialMessage: "hi"}, &apierror.UnauthorizedError{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateConversation(tc.ctx, tc.req)
			switch tc.want.(type) {
			case *apierror.ValidationError:
				var target *apierror.ValidationError
				if !errors.As(err, &target) {
					t.Errorf("err = %v, want a ValidationError", err)
				}
			case *apierror.UnauthorizedError:
				var target *apierror.UnauthorizedError
				if !errors.As(err, &target) {
					t.Errorf("err = %v, want an UnauthorizedError", err)
				}
			}
		})
	}
}

// TestConversationService_CreateConversation_DualWriteUsesServiceNowIdentity
// covers the ServiceNow-first path: the Postgres row takes ServiceNow's id,
// number and state, so later comment/state mirrors target a record
// ServiceNow knows.
func TestConversationService_CreateConversation_DualWriteUsesServiceNowIdentity(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	active := "ACTIVE"
	mirror := &stubMirrorConversationService{
		createConversation: func(context.Context, domain.CreateConversationRequest) (domain.CreateConversationResponse, error) {
			return domain.CreateConversationResponse{Conversation: domain.CreatedConversation{
				ID: testConversationID, Number: "CHAT0001234", State: &active,
			}}, nil
		},
	}
	var got repository.CreateConversationInput
	repo := &stubConversationRepo{
		createConversation: func(_ context.Context, in repository.CreateConversationInput) (domain.CreatedConversation, error) {
			got = in
			return domain.CreatedConversation{ID: in.ID, Number: in.Number}, nil
		},
	}
	svc := NewConversationServiceWithSNWriteback(repo, NewSNWritebackDispatcher(&recordingSNWritebackFailures{}), mirror)

	if _, err := svc.CreateConversation(ctx, domain.CreateConversationRequest{ProjectID: testConversationProjectID, InitialMessage: "hi"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != testConversationID || got.Number != "CHAT0001234" || got.State != domain.ConversationStateActive {
		t.Errorf("repo input id/number/state = %q/%q/%q, want ServiceNow's", got.ID, got.Number, got.State)
	}
}

// TestConversationService_CreateConversation_DualWriteServiceNowFailure
// covers the failure half: ServiceNow rejecting the create writes nothing
// to Postgres.
func TestConversationService_CreateConversation_DualWriteServiceNowFailure(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	mirror := &stubMirrorConversationService{
		createConversation: func(context.Context, domain.CreateConversationRequest) (domain.CreateConversationResponse, error) {
			return domain.CreateConversationResponse{}, errors.New("sn downstream unreachable")
		},
	}
	svc := NewConversationServiceWithSNWriteback(&stubConversationRepo{}, NewSNWritebackDispatcher(&recordingSNWritebackFailures{}), mirror)

	if _, err := svc.CreateConversation(ctx, domain.CreateConversationRequest{ProjectID: testConversationProjectID, InitialMessage: "hi"}); err == nil {
		t.Fatal("expected the ServiceNow error to be returned")
	}
}

// The Support page's "Resolved via Chat (Last 30d)" list sends the window as an
// updated-date range; it must reach the repository untouched, and an inverted
// range is a 400 before anything is queried.
func TestConversationService_SearchConversations_UpdatedDateWindow(t *testing.T) {
	end := time.Now()
	start := end.Add(-30 * 24 * time.Hour)

	var got domain.SearchConversationsFilters
	repo := &stubConversationRepo{
		searchConversations: func(_ context.Context, req domain.SearchConversationsRequest, _ string) ([]domain.SearchConversationView, int, error) {
			got = req.Filters
			return nil, 0, nil
		},
	}
	svc := NewConversationService(repo)

	if _, err := svc.SearchConversations(context.Background(), domain.SearchConversationsRequest{
		Filters: domain.SearchConversationsFilters{
			ProjectIDs: []string{testConversationProjectID}, StartUpdatedDate: &start, EndUpdatedDate: &end,
		},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.StartUpdatedDate == nil || !got.StartUpdatedDate.Equal(start) || got.EndUpdatedDate == nil || !got.EndUpdatedDate.Equal(end) {
		t.Errorf("window reaching the repository = %v..%v, want %v..%v", got.StartUpdatedDate, got.EndUpdatedDate, start, end)
	}

	_, err := svc.SearchConversations(context.Background(), domain.SearchConversationsRequest{
		Filters: domain.SearchConversationsFilters{
			ProjectIDs: []string{testConversationProjectID}, StartUpdatedDate: &end, EndUpdatedDate: &start,
		},
	})
	var verr *apierror.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("an end before the start = %v, want a ValidationError", err)
	}
}
