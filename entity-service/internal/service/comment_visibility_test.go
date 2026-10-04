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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// searchableCommentRepo is stubCommentRepo with SearchComments wired, and
// records the excludeDeleted flag the service passed.
type searchableCommentRepo struct {
	stubCommentRepo
	rows           []repository.CommentRow
	excludeDeleted bool
}

func (s *searchableCommentRepo) SearchComments(_ context.Context, _ string, _ domain.ReferenceType, _ *string, excludeDeleted bool, _ domain.Pagination) ([]repository.CommentRow, int, error) {
	s.excludeDeleted = excludeDeleted
	return s.rows, len(s.rows), nil
}

func commentRowOfType(id, typ string) repository.CommentRow {
	return repository.CommentRow{ID: id, Type: &typ}
}

func TestCommentService_SearchComments_CustomerScopeGetsNoWorkNotes(t *testing.T) {
	repo := &searchableCommentRepo{rows: []repository.CommentRow{
		commentRowOfType("1", "COMMENT"),
		commentRowOfType("2", "WORK_NOTE"),
		commentRowOfType("3", "APPROVAL_HISTORY"),
	}}
	svc := NewCommentService(repo, customerUserRepo())
	req := domain.SearchCommentsRequest{ReferenceID: testUUID, ReferenceType: domain.ReferenceTypeIncident}

	customerCtx := repository.WithCallerIdentity(
		contextWithUserIDToken(fakeJWTWithEmail(t, "customer@example.com")),
		AccessScope{ProjectIDs: []string{"proj-1"}, ViewerEmail: "customer@example.com"})
	resp, err := svc.SearchComments(customerCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Comments) != 2 || resp.Comments[0].ID != "1" || resp.Comments[1].ID != "3" {
		t.Fatalf("customer page = %+v, want comments 1 and 3", resp.Comments)
	}
	if !repo.excludeDeleted {
		t.Fatal("a customer search must exclude soft-deleted rows")
	}

	workNote := domain.CommentTypeWorkNote
	_, err = svc.SearchComments(customerCtx, domain.SearchCommentsRequest{ReferenceID: testUUID, ReferenceType: domain.ReferenceTypeIncident, Filters: &domain.CommentFilters{Type: &workNote}})
	requireForbidden(t, "explicit work_note filter", err)

	// A caller whose scope cannot be resolved at all is served like a customer.
	if resp, err = svc.SearchComments(contextWithUserIDToken(""), req); err != nil || len(resp.Comments) != 2 {
		t.Fatalf("unresolved caller: got %d comments, err %v; want 2, nil", len(resp.Comments), err)
	}

	internalCtx := repository.WithCallerIdentity(contextWithUserIDToken(fakeJWTWithEmail(t, "eng@example.com")), AccessScope{Unrestricted: true})
	if resp, err = svc.SearchComments(internalCtx, req); err != nil || len(resp.Comments) != 3 {
		t.Fatalf("internal caller: got %d comments, err %v; want 3, nil", len(resp.Comments), err)
	}
	if repo.excludeDeleted {
		t.Fatal("an internal search must still see soft-deleted rows (shown redacted)")
	}
}

func TestCommentService_CreateComment_WorkNoteRequiresInternalScope(t *testing.T) {
	svc := NewCommentService(&stubCommentRepo{}, customerUserRepo())
	req := domain.CreateCommentRequest{ReferenceID: testUUID, ReferenceType: domain.ReferenceTypeIncident, Type: domain.CommentTypeWorkNote, Content: "x"}
	customerCtx := repository.WithCallerIdentity(
		contextWithUserIDToken(fakeJWTWithEmail(t, "customer@example.com")),
		AccessScope{ProjectIDs: []string{"proj-1"}, ViewerEmail: "customer@example.com"})
	_, err := svc.CreateComment(customerCtx, req)
	requireForbidden(t, "customer work_note", err)
}
