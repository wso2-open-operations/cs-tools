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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// stubTimeCardRepo is a configurable repository.TimeCardRepository for the
// service tests: each method delegates to a func field, or panics if unset.
type stubTimeCardRepo struct {
	create     func(ctx context.Context, req domain.CreateTimeCardRequest, submitter string) (domain.TimeCardView, error)
	getByID    func(ctx context.Context, id string) (domain.TimeCardView, error)
	search     func(ctx context.Context, req domain.SearchTimeCardsRequest) ([]domain.TimeCardView, int, error)
	updateFlds func(ctx context.Context, req domain.UpdateTimeCardRequest) (domain.TimeCardView, error)
	transition func(ctx context.Context, id string, state domain.TimeCardState, approvedBy string, rejectionReason *string) (domain.TimeCardView, error)
	del        func(ctx context.Context, id string) error
}

func (s *stubTimeCardRepo) CreateTimeCard(ctx context.Context, req domain.CreateTimeCardRequest, submitter string) (domain.TimeCardView, error) {
	if s.create != nil {
		return s.create(ctx, req, submitter)
	}
	panic("CreateTimeCard called unexpectedly")
}
func (s *stubTimeCardRepo) GetTimeCardByID(ctx context.Context, id string) (domain.TimeCardView, error) {
	if s.getByID != nil {
		return s.getByID(ctx, id)
	}
	panic("GetTimeCardByID called unexpectedly")
}
func (s *stubTimeCardRepo) SearchTimeCards(ctx context.Context, req domain.SearchTimeCardsRequest) ([]domain.TimeCardView, int, error) {
	if s.search != nil {
		return s.search(ctx, req)
	}
	panic("SearchTimeCards called unexpectedly")
}
func (s *stubTimeCardRepo) UpdateTimeCardFields(ctx context.Context, req domain.UpdateTimeCardRequest) (domain.TimeCardView, error) {
	if s.updateFlds != nil {
		return s.updateFlds(ctx, req)
	}
	panic("UpdateTimeCardFields called unexpectedly")
}
func (s *stubTimeCardRepo) TransitionTimeCard(ctx context.Context, id string, state domain.TimeCardState, approvedBy string, rejectionReason *string) (domain.TimeCardView, error) {
	if s.transition != nil {
		return s.transition(ctx, id, state, approvedBy, rejectionReason)
	}
	panic("TransitionTimeCard called unexpectedly")
}
func (s *stubTimeCardRepo) DeleteTimeCard(ctx context.Context, id string) error {
	if s.del != nil {
		return s.del(ctx, id)
	}
	panic("DeleteTimeCard called unexpectedly")
}

const (
	tcSubmitterID = "11111111-1111-1111-1111-111111111111"
	tcApproverID  = "22222222-2222-2222-2222-222222222222"
	tcCardID      = "33333333-3333-3333-3333-333333333333"
)

func tcActorCtx(t *testing.T) context.Context {
	return contextWithUserIDToken(fakeJWTWithEmail(t, "actor@wso2.com"))
}
func tcUserRepo(id string) stubUserRepo {
	return stubUserRepo{getUserByEmail: func(context.Context, string) (domain.User, error) {
		return domain.User{ID: id, Email: "actor@wso2.com"}, nil
	}}
}
func strp(s string) *string { return &s }
func intp(i int) *int       { return &i }

func submittedCard(submitter string, approvers ...string) domain.TimeCardView {
	st := string(domain.TimeCardStateSubmitted)
	v := domain.TimeCardView{ID: tcCardID, State: &st, User: &domain.TimeCardRef{ID: submitter}}
	for _, a := range approvers {
		v.Approvers = append(v.Approvers, domain.TimeCardRef{ID: a})
	}
	return v
}

func asVErr(t *testing.T, err error) {
	t.Helper()
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}
func asForbidden(t *testing.T, err error) {
	t.Helper()
	if _, ok := err.(*apierror.ForbiddenError); !ok {
		t.Fatalf("expected *apierror.ForbiddenError, got %T: %v", err, err)
	}
}

func TestTimeCardService_SearchCaseTimeCards_UnsupportedNatively(t *testing.T) {
	svc := NewTimeCardService(&stubTimeCardRepo{}, stubUserRepo{})
	_, err := svc.SearchCaseTimeCards(context.Background(), domain.SearchTimeCardsRequest{})
	if _, ok := err.(*apierror.ServiceUnavailableError); !ok {
		t.Fatalf("expected *apierror.ServiceUnavailableError, got %T: %v", err, err)
	}
}

func TestTimeCardService_CreateTimeCard_Validation(t *testing.T) {
	svc := NewTimeCardService(&stubTimeCardRepo{}, tcUserRepo(tcSubmitterID))
	ctx := tcActorCtx(t)
	base := func() domain.CreateTimeCardRequest {
		return domain.CreateTimeCardRequest{CaseID: tcCardID, ProjectID: "proj-1", Date: "2026-09-07", ApproverIDs: []string{tcApproverID}}
	}
	t.Run("missing caseId", func(t *testing.T) {
		r := base()
		r.CaseID = ""
		_, err := svc.CreateTimeCard(ctx, r)
		asVErr(t, err)
	})
	t.Run("bad date", func(t *testing.T) {
		r := base()
		r.Date = "07/09/2026"
		_, err := svc.CreateTimeCard(ctx, r)
		asVErr(t, err)
	})
	t.Run("no approvers", func(t *testing.T) {
		r := base()
		r.ApproverIDs = nil
		_, err := svc.CreateTimeCard(ctx, r)
		asVErr(t, err)
	})
	t.Run("negative time", func(t *testing.T) {
		r := base()
		r.TimeAnalyzing = -5
		_, err := svc.CreateTimeCard(ctx, r)
		asVErr(t, err)
	})
	t.Run("submitter as own approver", func(t *testing.T) {
		r := base()
		r.ApproverIDs = []string{tcSubmitterID}
		_, err := svc.CreateTimeCard(ctx, r)
		asVErr(t, err)
	})
	t.Run("happy path creates in submitted state", func(t *testing.T) {
		called := false
		repo := &stubTimeCardRepo{create: func(_ context.Context, req domain.CreateTimeCardRequest, submitter string) (domain.TimeCardView, error) {
			called = true
			if submitter != tcSubmitterID {
				t.Fatalf("submitter = %q, want %q", submitter, tcSubmitterID)
			}
			return domain.TimeCardView{ID: tcCardID}, nil
		}}
		svc := NewTimeCardService(repo, tcUserRepo(tcSubmitterID))
		resp, err := svc.CreateTimeCard(ctx, base())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !called || resp.TimeCard == nil {
			t.Fatal("expected repo.CreateTimeCard to run and a card to be returned")
		}
	})
}

func TestTimeCardService_UpdateTimeCard_StateMachineAndAuthz(t *testing.T) {
	closed := domain.TimeCardStateApproved
	t.Run("state cannot combine with field edits", func(t *testing.T) {
		svc := NewTimeCardService(&stubTimeCardRepo{}, tcUserRepo(tcApproverID))
		_, err := svc.UpdateTimeCard(tcActorCtx(t), domain.UpdateTimeCardRequest{ID: tcCardID, State: &closed, IsBillable: func() *bool { b := true; return &b }()})
		asVErr(t, err)
	})
	t.Run("field edit by non-submitter is forbidden", func(t *testing.T) {
		repo := &stubTimeCardRepo{getByID: func(context.Context, string) (domain.TimeCardView, error) {
			return submittedCard("someone-else", tcApproverID), nil
		}}
		svc := NewTimeCardService(repo, tcUserRepo(tcApproverID))
		_, err := svc.UpdateTimeCard(tcActorCtx(t), domain.UpdateTimeCardRequest{ID: tcCardID, TimeAnalyzing: intp(10)})
		asForbidden(t, err)
	})
	t.Run("submitter cannot approve own card", func(t *testing.T) {
		repo := &stubTimeCardRepo{getByID: func(context.Context, string) (domain.TimeCardView, error) {
			return submittedCard(tcSubmitterID, tcApproverID), nil
		}}
		svc := NewTimeCardService(repo, tcUserRepo(tcSubmitterID))
		_, err := svc.UpdateTimeCard(tcActorCtx(t), domain.UpdateTimeCardRequest{ID: tcCardID, State: &closed})
		asForbidden(t, err)
	})
	t.Run("non-approver cannot approve", func(t *testing.T) {
		repo := &stubTimeCardRepo{getByID: func(context.Context, string) (domain.TimeCardView, error) {
			return submittedCard(tcSubmitterID, "other-approver"), nil
		}}
		svc := NewTimeCardService(repo, tcUserRepo(tcApproverID))
		_, err := svc.UpdateTimeCard(tcActorCtx(t), domain.UpdateTimeCardRequest{ID: tcCardID, State: &closed})
		asForbidden(t, err)
	})
	t.Run("eligible approver approves", func(t *testing.T) {
		repo := &stubTimeCardRepo{
			getByID: func(context.Context, string) (domain.TimeCardView, error) {
				return submittedCard(tcSubmitterID, tcApproverID), nil
			},
			transition: func(_ context.Context, id string, state domain.TimeCardState, approvedBy string, _ *string) (domain.TimeCardView, error) {
				if state != domain.TimeCardStateApproved || approvedBy != tcApproverID {
					t.Fatalf("unexpected transition: state=%v approvedBy=%v", state, approvedBy)
				}
				return domain.TimeCardView{ID: id}, nil
			},
		}
		svc := NewTimeCardService(repo, tcUserRepo(tcApproverID))
		if _, err := svc.UpdateTimeCard(tcActorCtx(t), domain.UpdateTimeCardRequest{ID: tcCardID, State: &closed}); err != nil {
			t.Fatalf("expected approve to succeed, got %v", err)
		}
	})
	t.Run("state must be approved or rejected", func(t *testing.T) {
		processed := domain.TimeCardStateProcessed
		repo := &stubTimeCardRepo{getByID: func(context.Context, string) (domain.TimeCardView, error) {
			return submittedCard(tcSubmitterID, tcApproverID), nil
		}}
		svc := NewTimeCardService(repo, tcUserRepo(tcApproverID))
		_, err := svc.UpdateTimeCard(tcActorCtx(t), domain.UpdateTimeCardRequest{ID: tcCardID, State: &processed})
		asVErr(t, err)
	})
}

func TestTimeCardService_DeleteTimeCard_Authz(t *testing.T) {
	t.Run("non-submitter forbidden", func(t *testing.T) {
		repo := &stubTimeCardRepo{getByID: func(context.Context, string) (domain.TimeCardView, error) {
			return submittedCard("someone-else"), nil
		}}
		svc := NewTimeCardService(repo, tcUserRepo(tcSubmitterID))
		_, err := svc.DeleteTimeCard(tcActorCtx(t), domain.DeleteTimeCardRequest{ID: tcCardID})
		asForbidden(t, err)
	})
	t.Run("submitter deletes submitted card", func(t *testing.T) {
		delCalled := false
		repo := &stubTimeCardRepo{
			getByID: func(context.Context, string) (domain.TimeCardView, error) { return submittedCard(tcSubmitterID), nil },
			del:     func(context.Context, string) error { delCalled = true; return nil },
		}
		svc := NewTimeCardService(repo, tcUserRepo(tcSubmitterID))
		if _, err := svc.DeleteTimeCard(tcActorCtx(t), domain.DeleteTimeCardRequest{ID: tcCardID}); err != nil {
			t.Fatalf("expected delete to succeed, got %v", err)
		}
		if !delCalled {
			t.Fatal("expected repo.DeleteTimeCard to run")
		}
	})
}

// keep strp referenced (used by future edit-path tests)
var _ = strp
