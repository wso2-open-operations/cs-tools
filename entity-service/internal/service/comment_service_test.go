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
		if got.CreatedBy != "" {
			t.Errorf("mirror got createdBy %q, want it omitted so ServiceNow resolves the author from the token", got.CreatedBy)
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

// TestCommentService_GetCommentEditHistory_NoAuthorizationCheck pins the
// current design: this service performs no author/role gate of its own any
// more (that decision moved entirely to csm-portal-backend, the sole caller
// of this path -- see GetComment's own doc comment for why). A caller who is
// neither the comment's author nor anything special may still read its
// history; only the comment's own existence (via GetCommentByID) and a valid
// actor identity (via resolveCommentActor) are required.
func TestCommentService_GetCommentEditHistory_NoAuthorizationCheck(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "stranger@example.com"))
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
			return domain.User{ID: testUUID, Email: "stranger@example.com"}, nil
		},
	}
	svc := NewCommentService(repo, userRepo)

	resp, err := svc.GetCommentEditHistory(ctx, testUUID)
	if err != nil {
		t.Fatalf("expected no author/role gate at this layer, got error: %v", err)
	}
	if len(resp.History) != 1 {
		t.Fatalf("History = %+v, want 1 entry", resp.History)
	}
}

// TestCommentService_GetCommentEditHistory_UnknownIDIs404 covers the one
// check this method still performs: GetCommentByID's own existence check, so
// a bad id is a 404 rather than a silently empty history list.
func TestCommentService_GetCommentEditHistory_UnknownIDIs404(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	repo := &stubCommentRepo{
		getCommentByID: func(context.Context, string) (repository.CommentRow, error) {
			return repository.CommentRow{}, &apierror.NotFoundError{Msg: "comment not found"}
		},
	}
	userRepo := stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: testUUID, Email: "jane.doe@example.com"}, nil
		},
	}
	svc := NewCommentService(repo, userRepo)

	_, err := svc.GetCommentEditHistory(ctx, testUUID)
	var notFound *apierror.NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("err = %v (%T), want *apierror.NotFoundError", err, err)
	}
}

// TestCommentService_GetComment covers the method csm-portal-backend calls to
// learn a comment's author before deciding whether to allow an edit/delete --
// see GetComment's own doc comment. No author/role check here either: the
// caller's own identity is irrelevant to this read.
func TestCommentService_GetComment(t *testing.T) {
	repo := &stubCommentRepo{
		getCommentByID: func(context.Context, string) (repository.CommentRow, error) {
			return repository.CommentRow{ID: testUUID, Content: "hello", CreatedBy: "jane.doe@example.com", CreatedByName: "Jane Doe"}, nil
		},
	}
	svc := NewCommentService(repo, stubUserRepo{})

	got, err := svc.GetComment(context.Background(), testUUID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.CreatedBy == nil || got.CreatedBy.Email != "jane.doe@example.com" {
		t.Errorf("CreatedBy = %+v, want email jane.doe@example.com", got.CreatedBy)
	}
}

// TestCommentService_GetComment_UnknownIDIs404 mirrors
// TestCommentService_GetCommentEditHistory_UnknownIDIs404 for this method.
func TestCommentService_GetComment_UnknownIDIs404(t *testing.T) {
	repo := &stubCommentRepo{
		getCommentByID: func(context.Context, string) (repository.CommentRow, error) {
			return repository.CommentRow{}, &apierror.NotFoundError{Msg: "comment not found"}
		},
	}
	svc := NewCommentService(repo, stubUserRepo{})

	_, err := svc.GetComment(context.Background(), testUUID)
	var notFound *apierror.NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("err = %v (%T), want *apierror.NotFoundError", err, err)
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

// TestCommentRowToDomain_AgentReadsBackAsNovera covers Novera chat history on
// Postgres: the assistant's replies are stored with created_by "agent", which
// has no "user" row, and the customer portal only treats a message as the
// assistant's when its author name is "Novera" -- the name ServiceNow gave it.
func TestCommentRowToDomain_AgentReadsBackAsNovera(t *testing.T) {
	row := repository.CommentRow{
		ID:         "c-3",
		WorkItemID: "wi-1",
		Content:    "Try restarting the gateway.",
		CreatedBy:  "agent",
	}

	got := commentRowToDomain(row)

	if got.CreatedBy == nil {
		t.Fatal("CreatedBy is nil, want a reference")
	}
	if got.CreatedBy.Name != "Novera" {
		t.Errorf("CreatedBy.Name = %q, want %q", got.CreatedBy.Name, "Novera")
	}
	if got.CreatedBy.Email != "agent" {
		t.Errorf("CreatedBy.Email = %q, want %q", got.CreatedBy.Email, "agent")
	}
}

func TestCommentRowToDomain_AuthorAndContentTable(t *testing.T) {
	tests := []struct {
		name          string
		createdBy     string
		createdByName string
		content       string
		wantName      string
		wantContent   string
	}{
		{"migrated Novera without resolved name", "Novera", "", "hi", "Novera", "hi"},
		{"migrated Novera lowercase", "novera", "", "hi", "Novera", "hi"},
		{"Novera with resolved name keeps it", "Novera", "Resolved Name", "hi", "Resolved Name", "hi"},
		{"agent sentinel still works", "agent", "", "hi", "Novera", "hi"},
		{"email join name wins", "jane.doe@example.com", "Jane Doe", "hi", "Jane Doe", "hi"},
		{"unknown automation account stays empty", "some_pipeline", "", "hi", "", "hi"},
		{"wrapped content", "Novera", "", "[code]I'm sorry.[/code]", "Novera", "I'm sorry."},
		{"only leading marker", "x", "", "[code]open only", "", "[code]open only"},
		{"whitespace around wrapper", "x", "", "\n [code]body[/code] \n", "", "body"},
		{"interior marker untouched", "x", "", "a [code]b[/code] c", "", "a [code]b[/code] c"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := commentRowToDomain(repository.CommentRow{
				ID: "c-1", WorkItemID: "wi-1",
				Content: tt.content, CreatedBy: tt.createdBy, CreatedByName: tt.createdByName,
			})
			if got.CreatedBy == nil {
				t.Fatal("CreatedBy is nil")
			}
			if got.CreatedBy.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", got.CreatedBy.Name, tt.wantName)
			}
			if got.CreatedBy.Email != tt.createdBy {
				t.Errorf("Email = %q, want %q", got.CreatedBy.Email, tt.createdBy)
			}
			if got.Content != tt.wantContent {
				t.Errorf("Content = %q, want %q", got.Content, tt.wantContent)
			}
		})
	}
}

type recordingLinker struct {
	calls []struct{ comment, workItem, author, content string }
}

func (r *recordingLinker) LinkFromContent(_ context.Context, commentID, workItemID, author, content string) {
	r.calls = append(r.calls, struct{ comment, workItem, author, content string }{commentID, workItemID, author, content})
}

func TestCommentService_CreateComment_LinksTheImagesItShows(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	html := `<img src="/attachments/inline/11111111-1111-4111-8111-111111111111/content">`
	repo := &stubCommentRepo{
		createComment: func(context.Context, string, domain.ReferenceType, string, string, string) (repository.CommentRow, error) {
			return repository.CommentRow{ID: "comment-1", WorkItemID: testUUID}, nil
		},
	}
	linker := &recordingLinker{}
	svc := WithCommentInlineImageLinker(NewCommentService(repo, stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: testUUID, Email: "jane.doe@example.com"}, nil
		},
	}), linker)

	if _, err := svc.CreateComment(ctx, domain.CreateCommentRequest{ReferenceID: testUUID, ReferenceType: domain.ReferenceTypeCase, Type: domain.CommentTypeComment, Content: html}); err != nil {
		t.Fatalf("CreateComment: %v", err)
	}
	if len(linker.calls) != 1 || linker.calls[0].comment != "comment-1" || linker.calls[0].workItem != testUUID ||
		linker.calls[0].author != "jane.doe@example.com" || linker.calls[0].content != html {
		t.Errorf("linker calls = %+v, want one call for the saved comment, its work item and its author", linker.calls)
	}
}

type rewritingLinker struct {
	recordingLinker
	got string
}

func (r *rewritingLinker) ExtractInlineImages(_ context.Context, workItemID, author, content string) (string, error) {
	r.got = workItemID + "|" + author + "|" + content
	return strings.ReplaceAll(content, "data:image/png;base64,AAAA", "/11111111-1111-4111-8111-111111111111"), nil
}

func TestCommentService_CreateComment_SavesTheExtractedContent(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	var saved string
	repo := &stubCommentRepo{
		createComment: func(_ context.Context, _ string, _ domain.ReferenceType, _ string, content string, _ string) (repository.CommentRow, error) {
			saved = content
			return repository.CommentRow{ID: "comment-1", WorkItemID: testUUID}, nil
		},
	}
	linker := &rewritingLinker{}
	svc := WithCommentInlineImageLinker(NewCommentService(repo, stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: testUUID, Email: "jane.doe@example.com"}, nil
		},
	}), linker)

	in := `<img src="data:image/png;base64,AAAA">`
	if _, err := svc.CreateComment(ctx, domain.CreateCommentRequest{ReferenceID: testUUID, ReferenceType: domain.ReferenceTypeCase, Type: domain.CommentTypeComment, Content: in}); err != nil {
		t.Fatalf("CreateComment: %v", err)
	}
	if want := `<img src="/11111111-1111-4111-8111-111111111111">`; saved != want {
		t.Errorf("saved content = %q, want %q", saved, want)
	}
	if linker.got != testUUID+"|jane.doe@example.com|"+in {
		t.Errorf("extractor was given %q", linker.got)
	}
	if len(linker.calls) != 1 || linker.calls[0].content != saved {
		t.Errorf("the linker must see the saved content: %+v", linker.calls)
	}
}

// The ServiceNow mirror is given the content as the caller sent it, embedded
// image included: ServiceNow extracts that itself and cannot resolve the
// reference Postgres stores in its place.
func TestCommentService_CreateComment_MirrorGetsTheOriginalContent(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	in := `<img src="data:image/png;base64,AAAA">`
	req := domain.CreateCommentRequest{ReferenceID: testUUID, ReferenceType: domain.ReferenceTypeIncident, Type: domain.CommentTypeComment, Content: in}

	called := make(chan domain.CreateCommentRequest, 1)
	mirror := &stubMirrorCommentService{
		createComment: func(_ context.Context, mirrorReq domain.CreateCommentRequest) (domain.CreateCommentResponse, error) {
			called <- mirrorReq
			return domain.CreateCommentResponse{}, nil
		},
	}
	var saved string
	repo := &stubCommentRepo{
		createComment: func(_ context.Context, _ string, _ domain.ReferenceType, _ string, content string, _ string) (repository.CommentRow, error) {
			saved = content
			return repository.CommentRow{ID: testUUID}, nil
		},
	}
	svc := NewCommentServiceWithSNWriteback(repo, stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: testUUID, Email: "jane.doe@example.com"}, nil
		},
	}, NewSNWritebackDispatcher(&recordingSNWritebackFailures{}), mirror)
	svc = WithCommentInlineImageLinker(svc, &rewritingLinker{})

	if _, err := svc.CreateComment(ctx, req); err != nil {
		t.Fatalf("CreateComment: %v", err)
	}
	if want := `<img src="/11111111-1111-4111-8111-111111111111">`; saved != want {
		t.Errorf("Postgres saved %q, want %q", saved, want)
	}
	select {
	case got := <-called:
		if got.Content != in {
			t.Errorf("mirror got %q, want the original %q", got.Content, in)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("mirror.CreateComment was never called")
	}
}
