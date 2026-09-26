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
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// stubTimeCardRepo is a minimal repository.TimeCardRepository whose
// unconfigured methods panic if called -- same convention as
// stubCallRequestRepo (call_request_service_test.go).
type stubTimeCardRepo struct {
	createTimeCard func(ctx context.Context, req domain.CreateTimeCardRequest, userID string) (domain.TimeCardView, error)
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
func (s *stubTimeCardRepo) UpdateTimeCardFields(context.Context, domain.UpdateTimeCardRequest, string) (domain.TimeCardView, error) {
	panic("not implemented")
}
func (s *stubTimeCardRepo) TransitionTimeCardState(context.Context, string, domain.TimeCardState, *string, string) (domain.TimeCardView, error) {
	panic("not implemented")
}
func (s *stubTimeCardRepo) DeleteTimeCard(context.Context, string, string) error {
	panic("not implemented")
}

// stubMirrorTimeCardService embeds TimeCardService (nil) and overrides only
// CreateTimeCard -- same convention as stubMirrorCallRequestService.
type stubMirrorTimeCardService struct {
	TimeCardService
	createTimeCard func(ctx context.Context, req domain.CreateTimeCardRequest) (domain.TimeCardMutationResponse, error)
}

func (s *stubMirrorTimeCardService) CreateTimeCard(ctx context.Context, req domain.CreateTimeCardRequest) (domain.TimeCardMutationResponse, error) {
	return s.createTimeCard(ctx, req)
}

func validCreateTimeCardRequest() domain.CreateTimeCardRequest {
	return domain.CreateTimeCardRequest{
		CaseID: testUUID, ProjectID: testUUID, Date: "2026-09-01",
		ApproverIDs: []string{testUUID}, TimeAnalyzing: 10,
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
		getUserByEmail: func(context.Context, string) (domain.User, error) { return domain.User{ID: testUUID, Email: "jane.doe@example.com"}, nil },
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
		getUserByEmail: func(context.Context, string) (domain.User, error) { return domain.User{ID: testUUID, Email: "jane.doe@example.com"}, nil },
	}, dispatcher, mirror)

	if _, err := svc.CreateTimeCard(ctx, req); err != nil {
		t.Fatalf("expected the Postgres-side success to be reported despite the mirror failure, got %v", err)
	}

	waitFor(t, func() bool { return failures.count() == 1 })
}
