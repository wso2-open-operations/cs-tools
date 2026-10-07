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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	testFeedbackCaseUUID  = "11111111-1111-1111-1111-111111111111"
	testFeedbackEmojiUUID = "22222222-2222-2222-2222-222222222222"
	testFeedbackChipUUID  = "33333333-3333-3333-3333-333333333333"
)

func TestCaseService_GetCaseFeedback_NotFoundWhenNoneSubmitted(t *testing.T) {
	repo := &stubCaseRepo{
		getCaseFeedback: func(context.Context, string) (repository.CaseFeedbackRow, bool, error) {
			return repository.CaseFeedbackRow{}, false, nil
		},
	}
	svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)

	_, err := svc.GetCaseFeedback(context.Background(), testFeedbackCaseUUID)
	var notFound *apierror.NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("expected NotFoundError, got %v", err)
	}
}

func TestCaseService_GetCaseFeedback_MapsRow(t *testing.T) {
	comment := "great support"
	repo := &stubCaseRepo{
		getCaseFeedback: func(ctx context.Context, caseID string) (repository.CaseFeedbackRow, bool, error) {
			if caseID != testFeedbackCaseUUID {
				t.Fatalf("unexpected caseID: %s", caseID)
			}
			return repository.CaseFeedbackRow{
				ID:                 "feedback-id",
				EmojiID:            testFeedbackEmojiUUID,
				EmojiName:          "Very Satisfied",
				EmojiSelectedImage: "/assets/feedback/very-satisfied-selected.svg",
				ChipIDs:            []string{testFeedbackChipUUID},
				CreatedBy:          "Jane Doe",
				CreatedOn:          "2026-10-06T00:00:00Z",
				AdditionalComment:  &comment,
			}, true, nil
		},
	}
	svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)

	got, err := svc.GetCaseFeedback(context.Background(), testFeedbackCaseUUID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := domain.CaseEmojiFeedback{
		ID: "feedback-id",
		Emoji: domain.CaseFeedbackEmojiRef{
			ID:            testFeedbackEmojiUUID,
			Name:          "Very Satisfied",
			SelectedImage: "/assets/feedback/very-satisfied-selected.svg",
		},
		ChipIDs:           []string{testFeedbackChipUUID},
		CreatedBy:         "Jane Doe",
		CreatedOn:         "2026-10-06T00:00:00Z",
		AdditionalComment: &comment,
	}
	if got.ID != want.ID || got.Emoji != want.Emoji || got.CreatedBy != want.CreatedBy ||
		got.CreatedOn != want.CreatedOn || len(got.ChipIDs) != 1 || got.ChipIDs[0] != testFeedbackChipUUID ||
		got.AdditionalComment == nil || *got.AdditionalComment != comment {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// TestCaseService_GetCaseFeedback_RejectsExternalCaller is the regression
// guard for the explicit product decision that a case's submitted feedback
// (the customer's own satisfaction rating/comment) is never shown back to
// an external/customer caller -- only internal users may view it, regardless
// of whether the external caller is themselves a registered contact on the
// case's own project. The repository must never even be reached.
func TestCaseService_GetCaseFeedback_RejectsExternalCaller(t *testing.T) {
	reached := false
	repo := &stubCaseRepo{
		getCaseFeedback: func(context.Context, string) (repository.CaseFeedbackRow, bool, error) {
			reached = true
			return repository.CaseFeedbackRow{}, false, nil
		},
	}
	svc := NewCaseService(repo, stubUserRepo{}, nil, stubAccess{scope: AccessScope{Unrestricted: false}}, nil)

	_, err := svc.GetCaseFeedback(context.Background(), testFeedbackCaseUUID)
	var forbidden *apierror.ForbiddenError
	if !errors.As(err, &forbidden) {
		t.Fatalf("expected ForbiddenError, got %v", err)
	}
	if reached {
		t.Fatal("GetCaseFeedback reached the repository for a non-internal caller")
	}
}

func TestCaseService_GetCaseFeedback_RejectsInvalidID(t *testing.T) {
	svc := NewCaseService(&stubCaseRepo{}, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)

	_, err := svc.GetCaseFeedback(context.Background(), "not-a-uuid")
	var validationErr *apierror.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
}

func TestCaseService_SubmitCaseFeedback_Success(t *testing.T) {
	var gotParams repository.CreateCaseFeedbackParams
	comment := "thanks"
	repo := &stubCaseRepo{
		createCaseFeedback: func(ctx context.Context, caseID string, params repository.CreateCaseFeedbackParams) (repository.CaseFeedbackCreated, error) {
			if caseID != testFeedbackCaseUUID {
				t.Fatalf("unexpected caseID: %s", caseID)
			}
			gotParams = params
			return repository.CaseFeedbackCreated{ID: "new-feedback-id", CreatedOn: "2026-10-06T00:00:00Z"}, nil
		},
	}
	svc := NewCaseService(repo, stubUserRepo{
		getUserByEmail: func(ctx context.Context, email string) (domain.User, error) {
			return domain.User{ID: "user-id", Email: email}, nil
		},
	}, nil, alwaysUnrestrictedAccess{}, nil)

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	resp, err := svc.SubmitCaseFeedback(ctx, testFeedbackCaseUUID, domain.SubmitCaseFeedbackRequest{
		EmojiID:           testFeedbackEmojiUUID,
		ChipIDs:           []string{testFeedbackChipUUID},
		AdditionalComment: &comment,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Feedback.ID != "new-feedback-id" || resp.Feedback.CaseID != testFeedbackCaseUUID ||
		resp.Feedback.CreatedBy != "jane.doe@example.com" || resp.Feedback.CreatedOn != "2026-10-06T00:00:00Z" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if gotParams.EmojiID != testFeedbackEmojiUUID || len(gotParams.ChipIDs) != 1 ||
		gotParams.ChipIDs[0] != testFeedbackChipUUID || gotParams.SubmittedByUserID != "user-id" ||
		gotParams.ActorEmail != "jane.doe@example.com" || gotParams.AdditionalComment != &comment {
		t.Fatalf("unexpected repository params: %+v", gotParams)
	}
}

func TestCaseService_SubmitCaseFeedback_PropagatesConflict(t *testing.T) {
	repo := &stubCaseRepo{
		createCaseFeedback: func(context.Context, string, repository.CreateCaseFeedbackParams) (repository.CaseFeedbackCreated, error) {
			return repository.CaseFeedbackCreated{}, &apierror.ConflictError{Msg: "feedback has already been submitted for this case"}
		},
	}
	svc := NewCaseService(repo, stubUserRepo{
		getUserByEmail: func(ctx context.Context, email string) (domain.User, error) {
			return domain.User{ID: "user-id", Email: email}, nil
		},
	}, nil, alwaysUnrestrictedAccess{}, nil)

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	_, err := svc.SubmitCaseFeedback(ctx, testFeedbackCaseUUID, domain.SubmitCaseFeedbackRequest{EmojiID: testFeedbackEmojiUUID})
	var conflict *apierror.ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("expected ConflictError, got %v", err)
	}
}

// "A caller may only submit feedback for a case they actually have access
// to" is NOT tested here: it's enforced entirely by RLS on the real
// CreateCaseFeedback query (case_feedback_repo.go), driven by the identity
// callerIdentityMiddleware stamps onto every request's context -- a stub
// CaseRepository has no RLS to exercise, so the real regression guard is
// TestCaseFeedbackIntegration_RejectsSubmissionForAnOutOfScopeCase
// (case_feedback_repo_integration_test.go), against a real Postgres.

func TestCaseService_SubmitCaseFeedback_RejectsInvalidIDs(t *testing.T) {
	svc := NewCaseService(&stubCaseRepo{}, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	cases := []domain.SubmitCaseFeedbackRequest{
		{EmojiID: "not-a-uuid"},
		{EmojiID: testFeedbackEmojiUUID, ChipIDs: []string{"not-a-uuid"}},
	}
	for _, req := range cases {
		_, err := svc.SubmitCaseFeedback(ctx, testFeedbackCaseUUID, req)
		var validationErr *apierror.ValidationError
		if !errors.As(err, &validationErr) {
			t.Fatalf("expected ValidationError for %+v, got %v", req, err)
		}
	}
}
