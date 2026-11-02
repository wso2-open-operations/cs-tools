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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

const testCustomDashboardID = "5e636a12-0000-4000-8000-000000000001"

type fakeCustomDashboardRepo struct {
	lastOwnerID string

	list    []domain.CustomDashboardSummary
	listErr error

	get *domain.CustomDashboard

	upsertCalled bool
	upsertedID   string
	upserted     domain.SaveCustomDashboardRequest
	upsertNil    bool
	upsertErr    error

	deleteCalled bool
	deleteFound  bool
}

func (f *fakeCustomDashboardRepo) List(_ context.Context, ownerID string) ([]domain.CustomDashboardSummary, error) {
	f.lastOwnerID = ownerID
	return f.list, f.listErr
}
func (f *fakeCustomDashboardRepo) Get(_ context.Context, ownerID, _ string) (*domain.CustomDashboard, error) {
	f.lastOwnerID = ownerID
	return f.get, nil
}
func (f *fakeCustomDashboardRepo) Upsert(_ context.Context, ownerID, id string, req domain.SaveCustomDashboardRequest) (*domain.CustomDashboard, error) {
	f.lastOwnerID = ownerID
	f.upsertCalled = true
	f.upsertedID = id
	f.upserted = req
	if f.upsertErr != nil {
		return nil, f.upsertErr
	}
	if f.upsertNil {
		return nil, nil
	}
	return &domain.CustomDashboard{ID: id, Name: req.Name, Definition: req.Definition}, nil
}
func (f *fakeCustomDashboardRepo) Delete(_ context.Context, ownerID, _ string) (bool, error) {
	f.lastOwnerID = ownerID
	f.deleteCalled = true
	return f.deleteFound, nil
}

func customDashboardSvc(t *testing.T, repo *fakeCustomDashboardRepo) (CustomDashboardService, context.Context) {
	t.Helper()
	// savedFilterViewUsers resolves every email to testPlatformUserID.
	return NewCustomDashboardService(repo, savedFilterViewUsers()), contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
}

func validDashboardSave() domain.SaveCustomDashboardRequest {
	return domain.SaveCustomDashboardRequest{Name: "My Cases", Definition: `{"displayName":"My Cases","widgets":[]}`}
}

func wantValidationError(t *testing.T, err error) {
	t.Helper()
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("got %T %v, want ValidationError", err, err)
	}
}

func wantNotFoundError(t *testing.T, err error) {
	t.Helper()
	var nf *apierror.NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("got %T %v, want NotFoundError", err, err)
	}
}

// --- caller

func TestCustomDashboardService_RequiresToken(t *testing.T) {
	svc := NewCustomDashboardService(&fakeCustomDashboardRepo{}, savedFilterViewUsers())
	_, err := svc.List(context.Background())
	if _, ok := err.(*apierror.UnauthorizedError); !ok {
		t.Fatalf("got %T %v, want UnauthorizedError", err, err)
	}
}

func TestCustomDashboardService_RequiresEmailClaim(t *testing.T) {
	svc := NewCustomDashboardService(&fakeCustomDashboardRepo{}, savedFilterViewUsers())
	ctx := contextWithUserIDToken(fakeJWTWithUserID(t, "no-email-claim"))
	_, err := svc.List(ctx)
	wantValidationError(t, err)
}

func TestCustomDashboardService_UnknownEmail(t *testing.T) {
	svc := NewCustomDashboardService(&fakeCustomDashboardRepo{}, stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{}, &apierror.NotFoundError{Msg: "no user found"}
		},
	})
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "nobody@example.com"))
	_, err := svc.List(ctx)
	wantNotFoundError(t, err)
}

// --- List

func TestCustomDashboardService_List_UsesCallerAndNeverReturnsNull(t *testing.T) {
	repo := &fakeCustomDashboardRepo{}
	svc, ctx := customDashboardSvc(t, repo)
	got, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if repo.lastOwnerID != testPlatformUserID {
		t.Errorf("owner = %q, want %q", repo.lastOwnerID, testPlatformUserID)
	}
	if got.Dashboards == nil || len(got.Dashboards) != 0 {
		t.Errorf("Dashboards = %#v, want an empty, non-nil slice", got.Dashboards)
	}
}

// --- Get

func TestCustomDashboardService_Get_RejectsNonUUID(t *testing.T) {
	svc, ctx := customDashboardSvc(t, &fakeCustomDashboardRepo{})
	_, err := svc.Get(ctx, "not-a-uuid")
	wantValidationError(t, err)
}

func TestCustomDashboardService_Get_MissingIsNotFound(t *testing.T) {
	svc, ctx := customDashboardSvc(t, &fakeCustomDashboardRepo{})
	_, err := svc.Get(ctx, testCustomDashboardID)
	wantNotFoundError(t, err)
}

func TestCustomDashboardService_Get_ReturnsDashboard(t *testing.T) {
	repo := &fakeCustomDashboardRepo{get: &domain.CustomDashboard{ID: testCustomDashboardID, Name: "My Cases"}}
	svc, ctx := customDashboardSvc(t, repo)
	got, err := svc.Get(ctx, testCustomDashboardID)
	if err != nil || got.Name != "My Cases" {
		t.Fatalf("Get = %+v, %v", got, err)
	}
	if repo.lastOwnerID != testPlatformUserID {
		t.Errorf("owner = %q, want %q", repo.lastOwnerID, testPlatformUserID)
	}
}

// --- Save

func TestCustomDashboardService_Save_RejectsBadInput(t *testing.T) {
	cases := map[string]struct {
		id  string
		req domain.SaveCustomDashboardRequest
	}{
		"non-UUID id":        {"abc", validDashboardSave()},
		"blank name":         {testCustomDashboardID, domain.SaveCustomDashboardRequest{Name: "  ", Definition: "{}"}},
		"name too long":      {testCustomDashboardID, domain.SaveCustomDashboardRequest{Name: strings.Repeat("é", domain.MaxCustomDashboardNameLength+1), Definition: "{}"}},
		"blank definition":   {testCustomDashboardID, domain.SaveCustomDashboardRequest{Name: "X", Definition: " \n "}},
		"definition too big": {testCustomDashboardID, domain.SaveCustomDashboardRequest{Name: "X", Definition: strings.Repeat("a", domain.MaxCustomDashboardDefinitionBytes+1)}},
		"NUL in name":        {testCustomDashboardID, domain.SaveCustomDashboardRequest{Name: "a\x00b", Definition: "{}"}},
		"NUL in definition":  {testCustomDashboardID, domain.SaveCustomDashboardRequest{Name: "X", Definition: "{\"a\":\"\x00\"}"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &fakeCustomDashboardRepo{}
			svc, ctx := customDashboardSvc(t, repo)
			_, err := svc.Save(ctx, tc.id, tc.req)
			wantValidationError(t, err)
			if repo.upsertCalled {
				t.Error("Upsert was called for invalid input")
			}
		})
	}
}

func TestCustomDashboardService_Save_AcceptsLimits(t *testing.T) {
	repo := &fakeCustomDashboardRepo{}
	svc, ctx := customDashboardSvc(t, repo)
	req := domain.SaveCustomDashboardRequest{
		// 255 two-byte characters: the limit is in characters, not bytes.
		Name:       strings.Repeat("é", domain.MaxCustomDashboardNameLength),
		Definition: strings.Repeat("a", domain.MaxCustomDashboardDefinitionBytes),
	}
	if _, err := svc.Save(ctx, testCustomDashboardID, req); err != nil {
		t.Fatalf("Save at the limits: %v", err)
	}
}

func TestCustomDashboardService_Save_TrimsNameKeepsDefinition(t *testing.T) {
	repo := &fakeCustomDashboardRepo{}
	svc, ctx := customDashboardSvc(t, repo)
	def := "  {\"widgets\":[]}\n"
	if _, err := svc.Save(ctx, testCustomDashboardID, domain.SaveCustomDashboardRequest{Name: "  My Cases  ", Definition: def}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if repo.upserted.Name != "My Cases" {
		t.Errorf("name = %q, want trimmed", repo.upserted.Name)
	}
	if repo.upserted.Definition != def {
		t.Errorf("definition = %q, want it unchanged", repo.upserted.Definition)
	}
	if repo.upsertedID != testCustomDashboardID || repo.lastOwnerID != testPlatformUserID {
		t.Errorf("Upsert(owner %q, id %q), want (%q, %q)", repo.lastOwnerID, repo.upsertedID, testPlatformUserID, testCustomDashboardID)
	}
}

func TestCustomDashboardService_Save_LeavesTheLimitToTheRepository(t *testing.T) {
	// The limit is checked once, by the repository under the owner's lock:
	// Save does not list the owner's dashboards first.
	repo := &fakeCustomDashboardRepo{listErr: errors.New("List must not be called")}
	svc, ctx := customDashboardSvc(t, repo)
	if _, err := svc.Save(ctx, testCustomDashboardID, validDashboardSave()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if !repo.upsertCalled {
		t.Error("Upsert was not called")
	}
}

func TestCustomDashboardService_Save_PassesTheLimitThrough(t *testing.T) {
	repo := &fakeCustomDashboardRepo{upsertErr: &apierror.ValidationError{Msg: "at most 50 saved dashboards are allowed"}}
	svc, ctx := customDashboardSvc(t, repo)
	_, err := svc.Save(ctx, testCustomDashboardID, validDashboardSave())
	wantValidationError(t, err)
}

func TestCustomDashboardService_Save_OtherOwnersIDIsNotFound(t *testing.T) {
	repo := &fakeCustomDashboardRepo{upsertNil: true}
	svc, ctx := customDashboardSvc(t, repo)
	_, err := svc.Save(ctx, testCustomDashboardID, validDashboardSave())
	wantNotFoundError(t, err)
}

func TestCustomDashboardService_Save_PassesConflictThrough(t *testing.T) {
	repo := &fakeCustomDashboardRepo{upsertErr: &apierror.ConflictError{Msg: "you already have a dashboard named: My Cases"}}
	svc, ctx := customDashboardSvc(t, repo)
	_, err := svc.Save(ctx, testCustomDashboardID, validDashboardSave())
	var ce *apierror.ConflictError
	if !errors.As(err, &ce) {
		t.Fatalf("got %T %v, want ConflictError", err, err)
	}
}

// --- Delete

func TestCustomDashboardService_Delete(t *testing.T) {
	t.Run("non-UUID id", func(t *testing.T) {
		repo := &fakeCustomDashboardRepo{}
		svc, ctx := customDashboardSvc(t, repo)
		wantValidationError(t, svc.Delete(ctx, "abc"))
		if repo.deleteCalled {
			t.Error("Delete was called for a non-UUID id")
		}
	})
	t.Run("missing", func(t *testing.T) {
		svc, ctx := customDashboardSvc(t, &fakeCustomDashboardRepo{})
		wantNotFoundError(t, svc.Delete(ctx, testCustomDashboardID))
	})
	t.Run("found", func(t *testing.T) {
		repo := &fakeCustomDashboardRepo{deleteFound: true}
		svc, ctx := customDashboardSvc(t, repo)
		if err := svc.Delete(ctx, testCustomDashboardID); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if repo.lastOwnerID != testPlatformUserID {
			t.Errorf("owner = %q, want %q", repo.lastOwnerID, testPlatformUserID)
		}
	})
}
