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
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type fakeSavedFilterViewRepo struct {
	lastUserID string

	list    []domain.SavedFilterView
	listErr error

	saveCalled bool
	savedName  string
	savedQs    string
	saveResult []domain.SavedFilterView
	saveErr    error

	deleteCalled bool
	deletedName  string
	deleteResult []domain.SavedFilterView

	moveCalled bool
	movedName  string
	movedDir   domain.SavedFilterMoveDirection
	moveResult []domain.SavedFilterView

	moveToCalled bool
	movedTo      int
}

func (f *fakeSavedFilterViewRepo) List(_ context.Context, userID string, _ domain.SavedFilterListKey) ([]domain.SavedFilterView, error) {
	f.lastUserID = userID
	return f.list, f.listErr
}
func (f *fakeSavedFilterViewRepo) Count(context.Context, string, domain.SavedFilterListKey) (int, error) {
	return len(f.list), nil
}
func (f *fakeSavedFilterViewRepo) Save(_ context.Context, _ string, _ domain.SavedFilterListKey, name, qs string) ([]domain.SavedFilterView, error) {
	f.saveCalled = true
	f.savedName = name
	f.savedQs = qs
	if f.saveErr != nil {
		return nil, f.saveErr
	}
	if f.saveResult != nil {
		return f.saveResult, nil
	}
	return []domain.SavedFilterView{{Name: name, Qs: qs}}, nil
}
func (f *fakeSavedFilterViewRepo) Delete(_ context.Context, _ string, _ domain.SavedFilterListKey, name string) ([]domain.SavedFilterView, error) {
	f.deleteCalled = true
	f.deletedName = name
	if f.deleteResult != nil {
		return f.deleteResult, nil
	}
	return []domain.SavedFilterView{}, nil
}
func (f *fakeSavedFilterViewRepo) Move(_ context.Context, _ string, _ domain.SavedFilterListKey, name string, direction domain.SavedFilterMoveDirection) ([]domain.SavedFilterView, error) {
	f.moveCalled = true
	f.movedName = name
	f.movedDir = direction
	if f.moveResult != nil {
		return f.moveResult, nil
	}
	return f.list, nil
}
func (f *fakeSavedFilterViewRepo) MoveTo(_ context.Context, _ string, _ domain.SavedFilterListKey, name string, position int) ([]domain.SavedFilterView, error) {
	f.moveToCalled = true
	f.movedName = name
	f.movedTo = position
	if f.moveResult != nil {
		return f.moveResult, nil
	}
	return f.list, nil
}

const testPlatformUserID = "user-1"

func fakeJWTWithUserID(t *testing.T, userID string) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payloadBytes, err := json.Marshal(map[string]string{"userid": userID})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	return header + "." + payload + ".sig"
}

func savedFilterViewUsers() repository.UserRepository {
	return stubUserRepo{
		getUserByEmail: func(_ context.Context, email string) (domain.User, error) {
			return domain.User{ID: testPlatformUserID, Email: email}, nil
		},
	}
}

func savedFilterViewSvc(t *testing.T, repo *fakeSavedFilterViewRepo) (SavedFilterViewService, context.Context) {
	t.Helper()
	return NewSavedFilterViewService(repo, savedFilterViewUsers()), contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
}

func TestSavedFilterViewService_List_RejectsInvalidListKey(t *testing.T) {
	svc, ctx := savedFilterViewSvc(t, &fakeSavedFilterViewRepo{})
	_, err := svc.List(ctx, "nope")
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("got %T %v, want ValidationError", err, err)
	}
}

func TestSavedFilterViewService_List_RequiresToken(t *testing.T) {
	svc := NewSavedFilterViewService(&fakeSavedFilterViewRepo{}, savedFilterViewUsers())
	_, err := svc.List(context.Background(), domain.SavedFilterListKeyCases)
	if _, ok := err.(*apierror.UnauthorizedError); !ok {
		t.Fatalf("got %T %v, want UnauthorizedError", err, err)
	}
}

func TestSavedFilterViewService_List_RequiresEmailClaim(t *testing.T) {
	svc := NewSavedFilterViewService(&fakeSavedFilterViewRepo{}, savedFilterViewUsers())
	ctx := contextWithUserIDToken(fakeJWTWithUserID(t, "asgardeo-only"))
	_, err := svc.List(ctx, domain.SavedFilterListKeyCases)
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("got %T %v, want ValidationError", err, err)
	}
}

func TestSavedFilterViewService_List_UnknownEmail(t *testing.T) {
	svc := NewSavedFilterViewService(&fakeSavedFilterViewRepo{}, stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{}, &apierror.NotFoundError{Msg: "no user found"}
		},
	})
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "nobody@example.com"))
	_, err := svc.List(ctx, domain.SavedFilterListKeyCases)
	if _, ok := err.(*apierror.NotFoundError); !ok {
		t.Fatalf("got %T %v, want NotFoundError", err, err)
	}
}

func TestSavedFilterViewService_List_ReturnsViews(t *testing.T) {
	repo := &fakeSavedFilterViewRepo{list: []domain.SavedFilterView{{Name: "A", Qs: "q=1"}}}
	svc, ctx := savedFilterViewSvc(t, repo)
	got, err := svc.List(ctx, domain.SavedFilterListKeyCases)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got.Views) != 1 || got.Views[0].Name != "A" {
		t.Fatalf("unexpected: %+v", got)
	}
	if repo.lastUserID != testPlatformUserID {
		t.Fatalf("user id = %q, want platform user.id %q", repo.lastUserID, testPlatformUserID)
	}
}

func TestSavedFilterViewService_Save_RejectsEmptyName(t *testing.T) {
	svc, ctx := savedFilterViewSvc(t, &fakeSavedFilterViewRepo{})
	_, err := svc.Save(ctx, domain.SaveSavedFilterViewRequest{ListKey: domain.SavedFilterListKeyCases, Name: "  ", Qs: "q=1"})
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("got %T %v, want ValidationError", err, err)
	}
}

func TestSavedFilterViewService_Save_EnforcesMaxOnNewName(t *testing.T) {
	existing := make([]domain.SavedFilterView, domain.MaxSavedFilterViews)
	for i := range existing {
		existing[i] = domain.SavedFilterView{Name: "v", Qs: "q"}
		existing[i].Name = existing[i].Name + string(rune('A'+i%26)) + string(rune('0'+i/26))
	}
	repo := &fakeSavedFilterViewRepo{list: existing}
	svc, ctx := savedFilterViewSvc(t, repo)
	_, err := svc.Save(ctx, domain.SaveSavedFilterViewRequest{ListKey: domain.SavedFilterListKeyIncidents, Name: "new", Qs: "q=1"})
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("got %T %v, want ValidationError", err, err)
	}
	if repo.saveCalled {
		t.Fatal("Save must not be called when the cap is already reached")
	}
}

func TestSavedFilterViewService_Save_OverwriteDoesNotHitCap(t *testing.T) {
	existing := make([]domain.SavedFilterView, domain.MaxSavedFilterViews)
	for i := range existing {
		existing[i] = domain.SavedFilterView{Name: "view-" + string(rune('a'+(i%26))) + string(rune('0'+(i/26))), Qs: "q"}
	}
	existing[0].Name = "Keep"
	repo := &fakeSavedFilterViewRepo{list: existing, saveResult: existing}
	svc, ctx := savedFilterViewSvc(t, repo)
	_, err := svc.Save(ctx, domain.SaveSavedFilterViewRequest{ListKey: domain.SavedFilterListKeyCases, Name: "keep", Qs: "q=2"})
	if err != nil {
		t.Fatalf("Save overwrite: %v", err)
	}
	if !repo.saveCalled || repo.savedName != "keep" || repo.savedQs != "q=2" {
		t.Fatalf("expected overwrite forwarded, got name=%q qs=%q called=%v", repo.savedName, repo.savedQs, repo.saveCalled)
	}
}

func TestSavedFilterViewService_Delete_ForwardsTrimmedName(t *testing.T) {
	repo := &fakeSavedFilterViewRepo{}
	svc, ctx := savedFilterViewSvc(t, repo)
	_, err := svc.Delete(ctx, domain.SavedFilterListKeyProblems, "  Drop  ")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if repo.deletedName != "Drop" {
		t.Fatalf("deleted name = %q, want Drop", repo.deletedName)
	}
}

func TestSavedFilterViewService_Reorder_RejectsBadDirection(t *testing.T) {
	svc, ctx := savedFilterViewSvc(t, &fakeSavedFilterViewRepo{})
	_, err := svc.Reorder(ctx, domain.ReorderSavedFilterViewRequest{
		ListKey: domain.SavedFilterListKeyChangeRequests, Name: "A", Direction: "sideways",
	})
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("got %T %v, want ValidationError", err, err)
	}
}

func TestSavedFilterViewService_Reorder_Forwards(t *testing.T) {
	repo := &fakeSavedFilterViewRepo{list: []domain.SavedFilterView{{Name: "A", Qs: "a"}, {Name: "B", Qs: "b"}}}
	svc, ctx := savedFilterViewSvc(t, repo)
	_, err := svc.Reorder(ctx, domain.ReorderSavedFilterViewRequest{
		ListKey: domain.SavedFilterListKeyCases, Name: "A", Direction: domain.SavedFilterMoveDown,
	})
	if err != nil {
		t.Fatalf("Reorder: %v", err)
	}
	if !repo.moveCalled || repo.movedName != "A" || repo.movedDir != domain.SavedFilterMoveDown {
		t.Fatalf("move not forwarded: %+v", repo)
	}
}

func TestSavedFilterViewService_Reorder_ForwardsPosition(t *testing.T) {
	repo := &fakeSavedFilterViewRepo{}
	svc, ctx := savedFilterViewSvc(t, repo)
	pos := 2
	_, err := svc.Reorder(ctx, domain.ReorderSavedFilterViewRequest{
		ListKey: domain.SavedFilterListKeyCases, Name: "  A  ", Position: &pos,
	})
	if err != nil {
		t.Fatalf("Reorder: %v", err)
	}
	if !repo.moveToCalled || repo.movedName != "A" || repo.movedTo != 2 || repo.moveCalled {
		t.Fatalf("position not forwarded: %+v", repo)
	}
}
