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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// stubCommentRepo is a minimal repository.CommentRepository whose
// unconfigured methods panic if called -- same convention as
// stubCallRequestRepo (call_request_service_test.go).
type stubCommentRepo struct {
	createComment         func(ctx context.Context, referenceID string, referenceType domain.ReferenceType, typeEnum, content, createdBy string) (repository.CommentRow, error)
	getCommentByID        func(ctx context.Context, id string) (repository.CommentRow, error)
	getCommentEditHistory func(ctx context.Context, id string) ([]repository.CommentEditHistoryRow, error)
}

func (s *stubCommentRepo) CreateComment(ctx context.Context, referenceID string, referenceType domain.ReferenceType, typeEnum, content, createdBy string) (repository.CommentRow, error) {
	if s.createComment != nil {
		return s.createComment(ctx, referenceID, referenceType, typeEnum, content, createdBy)
	}
	panic("not implemented")
}
func (s *stubCommentRepo) SearchComments(context.Context, string, domain.ReferenceType, *string, bool, domain.Pagination) ([]repository.CommentRow, int, error) {
	panic("not implemented")
}
func (s *stubCommentRepo) UpdateComment(context.Context, string, string, string) (repository.CommentRow, error) {
	panic("not implemented")
}
func (s *stubCommentRepo) SoftDeleteComment(context.Context, string, string) error {
	panic("not implemented")
}
func (s *stubCommentRepo) GetCommentEditHistory(ctx context.Context, id string) ([]repository.CommentEditHistoryRow, error) {
	if s.getCommentEditHistory != nil {
		return s.getCommentEditHistory(ctx, id)
	}
	panic("not implemented")
}
func (s *stubCommentRepo) GetCommentByID(ctx context.Context, id string) (repository.CommentRow, error) {
	if s.getCommentByID != nil {
		return s.getCommentByID(ctx, id)
	}
	panic("not implemented")
}

// stubMirrorCommentService embeds CommentService (nil) and overrides only
// CreateComment -- same convention as stubMirrorCallRequestService.
type stubMirrorCommentService struct {
	CommentService
	createComment func(ctx context.Context, req domain.CreateCommentRequest) (domain.CreateCommentResponse, error)
}

func (s *stubMirrorCommentService) CreateComment(ctx context.Context, req domain.CreateCommentRequest) (domain.CreateCommentResponse, error) {
	return s.createComment(ctx, req)
}

// TestCommentService_CreateComment_MirrorsToServiceNow covers the writeback
// wiring: on a successful Postgres create, the mirror's CreateComment is
// dispatched asynchronously and does not block or affect the response.
func TestCommentService_CreateComment_MirrorsToServiceNow(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := domain.CreateCommentRequest{ReferenceID: testUUID, ReferenceType: domain.ReferenceTypeIncident, Type: domain.CommentTypeComment, Content: "hello"}

	called := make(chan domain.CreateCommentRequest, 1)
	mirror := &stubMirrorCommentService{
		createComment: func(_ context.Context, mirrorReq domain.CreateCommentRequest) (domain.CreateCommentResponse, error) {
			called <- mirrorReq
			return domain.CreateCommentResponse{}, nil
		},
	}
	repo := &stubCommentRepo{
		createComment: func(context.Context, string, domain.ReferenceType, string, string, string) (repository.CommentRow, error) {
			return repository.CommentRow{ID: testUUID}, nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	svc := NewCommentServiceWithSNWriteback(repo, stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: testUUID, Email: "jane.doe@example.com"}, nil
		},
	}, dispatcher, mirror)

	if _, err := svc.CreateComment(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case got := <-called:
		if got.ReferenceID != req.ReferenceID || got.Content != req.Content {
			t.Errorf("mirror got %+v, want referenceId/content to match %+v", got, req)
		}
		if got.CreatedBy != "jane.doe@example.com" {
			t.Errorf("mirror got createdBy %q, want the resolved caller email", got.CreatedBy)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("mirror.CreateComment was never called")
	}
	if got := failures.count(); got != 0 {
		t.Errorf("expected 0 sn_writeback_failures records for a successful mirror, got %d", got)
	}
}

// TestCommentService_CreateComment_MirrorFailureRecordsWritebackFailure
// covers the failure half: Postgres already succeeded, so the call must
// still report success, but the mirror error lands in sn_writeback_failures
// for manual backfill.
func TestCommentService_CreateComment_MirrorFailureRecordsWritebackFailure(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := domain.CreateCommentRequest{ReferenceID: testUUID, ReferenceType: domain.ReferenceTypeIncident, Type: domain.CommentTypeComment, Content: "hello"}

	mirror := &stubMirrorCommentService{
		createComment: func(context.Context, domain.CreateCommentRequest) (domain.CreateCommentResponse, error) {
			return domain.CreateCommentResponse{}, errors.New("sn downstream unreachable")
		},
	}
	repo := &stubCommentRepo{
		createComment: func(context.Context, string, domain.ReferenceType, string, string, string) (repository.CommentRow, error) {
			return repository.CommentRow{ID: testUUID}, nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	svc := NewCommentServiceWithSNWriteback(repo, stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: testUUID, Email: "jane.doe@example.com"}, nil
		},
	}, dispatcher, mirror)

	if _, err := svc.CreateComment(ctx, req); err != nil {
		t.Fatalf("expected the Postgres-side success to be reported despite the mirror failure, got %v", err)
	}

	waitFor(t, func() bool { return failures.count() == 1 })
}

// TestCommentService_GetCommentEditHistory_AuthorMayView covers the allowed
// case: the comment's own author (case-insensitive email match) may fetch
// its edit history.
func TestCommentService_GetCommentEditHistory_AuthorMayView(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	repo := &stubCommentRepo{
		getCommentByID: func(context.Context, string) (repository.CommentRow, error) {
			return repository.CommentRow{ID: testUUID, CreatedBy: "Jane.Doe@example.com"}, nil
		},
		getCommentEditHistory: func(context.Context, string) ([]repository.CommentEditHistoryRow, error) {
			return []repository.CommentEditHistoryRow{{ID: "h-1", CommentID: testUUID, Body: "old body", EditedBy: "jane.doe@example.com"}}, nil
		},
	}
	userRepo := stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: testUUID, Email: "jane.doe@example.com"}, nil
		},
		getUserRoles: func(context.Context, string) ([]string, error) { return nil, nil },
	}
	svc := NewCommentService(repo, userRepo)

	resp, err := svc.GetCommentEditHistory(ctx, testUUID)
	if err != nil {
		t.Fatalf("expected the author to view the history, got error: %v", err)
	}
	if len(resp.History) != 1 {
		t.Fatalf("History = %+v, want 1 entry", resp.History)
	}
}

// TestCommentService_GetCommentEditHistory_AdminMayView covers the other
// allowed case: a caller holding the admin role, regardless of authorship.
func TestCommentService_GetCommentEditHistory_AdminMayView(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "admin.user@example.com"))
	repo := &stubCommentRepo{
		getCommentByID: func(context.Context, string) (repository.CommentRow, error) {
			return repository.CommentRow{ID: testUUID, CreatedBy: "someone.else@example.com"}, nil
		},
		getCommentEditHistory: func(context.Context, string) ([]repository.CommentEditHistoryRow, error) {
			return []repository.CommentEditHistoryRow{{ID: "h-1", CommentID: testUUID, Body: "old body", EditedBy: "someone.else@example.com"}}, nil
		},
	}
	userRepo := stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: testUUID, Email: "admin.user@example.com"}, nil
		},
		getUserRoles: func(context.Context, string) ([]string, error) { return []string{"admin"}, nil },
	}
	svc := NewCommentService(repo, userRepo)

	resp, err := svc.GetCommentEditHistory(ctx, testUUID)
	if err != nil {
		t.Fatalf("expected an admin to view the history, got error: %v", err)
	}
	if len(resp.History) != 1 {
		t.Fatalf("History = %+v, want 1 entry", resp.History)
	}
}

// TestCommentService_GetCommentEditHistory_NonAuthorNonAdminForbidden is the
// real data-exposure bug this closes: a caller who is neither the comment's
// author nor an admin must not be able to read its edit history just by
// knowing the comment's UUID.
func TestCommentService_GetCommentEditHistory_NonAuthorNonAdminForbidden(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "stranger@example.com"))
	repo := &stubCommentRepo{
		getCommentByID: func(context.Context, string) (repository.CommentRow, error) {
			return repository.CommentRow{ID: testUUID, CreatedBy: "someone.else@example.com"}, nil
		},
	}
	userRepo := stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: testUUID, Email: "stranger@example.com"}, nil
		},
		getUserRoles: func(context.Context, string) ([]string, error) { return nil, nil },
	}
	svc := NewCommentService(repo, userRepo)

	_, err := svc.GetCommentEditHistory(ctx, testUUID)
	if err == nil {
		t.Fatal("expected a ForbiddenError, got nil")
	}
	var forbidden *apierror.ForbiddenError
	if !errors.As(err, &forbidden) {
		t.Fatalf("err = %v (%T), want *apierror.ForbiddenError", err, err)
	}
}

// TestCommentRowToDomain_PrefersResolvedName covers a real bug found live:
// the webapp showed a commenter's raw email ("dinithin@wso2.com") instead of
// their resolved display name, while sibling activity-feed entries (state
// changes, attachments) on the same case already showed a real name --
// because SearchComments never resolved CommentRow.CreatedByName at all, so
// commentRowToDomain always built CreatedBy with an empty Name, and the
// webapp's own authorDisplayName helper falls back to the email whenever
// Name is empty.
func TestCommentRowToDomain_PrefersResolvedName(t *testing.T) {
	row := repository.CommentRow{
		ID:            "c-1",
		WorkItemID:    "wi-1",
		Content:       "hello",
		CreatedBy:     "jane@example.com",
		CreatedByName: "Jane Doe",
	}

	got := commentRowToDomain(row)

	if got.CreatedBy == nil {
		t.Fatal("CreatedBy is nil, want a reference")
	}
	if got.CreatedBy.Name != "Jane Doe" {
		t.Errorf("CreatedBy.Name = %q, want %q", got.CreatedBy.Name, "Jane Doe")
	}
	if got.CreatedBy.Email != "jane@example.com" {
		t.Errorf("CreatedBy.Email = %q, want %q", got.CreatedBy.Email, "jane@example.com")
	}
}

// TestCommentRowToDomain_NoMatchingUserLeavesNameEmpty covers the other half:
// an automation/integration account (e.g. "github_pipeline") has no "user"
// row to resolve against, so SearchComments' LEFT JOIN leaves
// CreatedByName "" -- commentRowToDomain must pass that through as-is, not
// synthesize a name, so the webapp's own email fallback still applies for
// this legitimate case.
func TestCommentRowToDomain_NoMatchingUserLeavesNameEmpty(t *testing.T) {
	row := repository.CommentRow{
		ID:            "c-2",
		WorkItemID:    "wi-1",
		Content:       "automated update",
		CreatedBy:     "github_pipeline",
		CreatedByName: "",
	}

	got := commentRowToDomain(row)

	if got.CreatedBy == nil {
		t.Fatal("CreatedBy is nil, want a reference")
	}
	if got.CreatedBy.Name != "" {
		t.Errorf("CreatedBy.Name = %q, want empty", got.CreatedBy.Name)
	}
	if got.CreatedBy.Email != "github_pipeline" {
		t.Errorf("CreatedBy.Email = %q, want %q", got.CreatedBy.Email, "github_pipeline")
	}
}
