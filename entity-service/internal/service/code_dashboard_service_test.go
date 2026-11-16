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

type fakeCodeDashboardRepo struct {
	lastOwnerID string

	list    []domain.CodeDashboardSummary
	listErr error

	get *domain.CodeDashboard

	upsertCalled bool
	upsertedID   string
	upserted     domain.SaveCodeDashboardRequest
	upsertNil    bool
	upsertErr    error

	deleteCalled bool
	deleteFound  bool
}

func (f *fakeCodeDashboardRepo) List(_ context.Context, ownerID string) ([]domain.CodeDashboardSummary, error) {
	f.lastOwnerID = ownerID
	return f.list, f.listErr
}
func (f *fakeCodeDashboardRepo) Get(_ context.Context, ownerID, _ string) (*domain.CodeDashboard, error) {
	f.lastOwnerID = ownerID
	return f.get, nil
}
func (f *fakeCodeDashboardRepo) Upsert(_ context.Context, ownerID, id string, req domain.SaveCodeDashboardRequest) (*domain.CodeDashboard, error) {
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
	return &domain.CodeDashboard{ID: id, Name: req.Name, Source: req.Source}, nil
}
func (f *fakeCodeDashboardRepo) Delete(_ context.Context, ownerID, _ string) (bool, error) {
	f.lastOwnerID = ownerID
	f.deleteCalled = true
	return f.deleteFound, nil
}

func codeDashboardSvc(t *testing.T, repo *fakeCodeDashboardRepo) (CodeDashboardService, context.Context) {
	t.Helper()
	// savedFilterViewUsers resolves every email to testPlatformUserID.
	return NewCodeDashboardService(repo, savedFilterViewUsers()), contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
}

func validCodeDashboardSave() domain.SaveCodeDashboardRequest {
	return domain.SaveCodeDashboardRequest{Name: "Open Cases", Source: "async function render(portal, root) {}"}
}

// --- caller

func TestCodeDashboardService_RequiresToken(t *testing.T) {
	svc := NewCodeDashboardService(&fakeCodeDashboardRepo{}, savedFilterViewUsers())
	ctx := context.Background()
	if _, err := svc.List(ctx); !isUnauthorized(err) {
		t.Errorf("List: got %T %v, want UnauthorizedError", err, err)
	}
	if _, err := svc.Get(ctx, testCustomDashboardID); !isUnauthorized(err) {
		t.Errorf("Get: got %T %v, want UnauthorizedError", err, err)
	}
	if _, err := svc.Save(ctx, testCustomDashboardID, validCodeDashboardSave()); !isUnauthorized(err) {
		t.Errorf("Save: got %T %v, want UnauthorizedError", err, err)
	}
	if err := svc.Delete(ctx, testCustomDashboardID); !isUnauthorized(err) {
		t.Errorf("Delete: got %T %v, want UnauthorizedError", err, err)
	}
}

func isUnauthorized(err error) bool {
	var u *apierror.UnauthorizedError
	return errors.As(err, &u)
}

func TestCodeDashboardService_UnknownEmail(t *testing.T) {
	svc := NewCodeDashboardService(&fakeCodeDashboardRepo{}, stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{}, &apierror.NotFoundError{Msg: "no user found"}
		},
	})
	_, err := svc.List(contextWithUserIDToken(fakeJWTWithEmail(t, "nobody@example.com")))
	wantNotFoundError(t, err)
}

// --- List and Get

func TestCodeDashboardService_List_UsesCallerAndNeverReturnsNull(t *testing.T) {
	repo := &fakeCodeDashboardRepo{}
	svc, ctx := codeDashboardSvc(t, repo)
	got, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if repo.lastOwnerID != testPlatformUserID {
		t.Errorf("owner = %q, want %q", repo.lastOwnerID, testPlatformUserID)
	}
	if got.Dashboards == nil {
		t.Error("Dashboards is nil, want an empty list")
	}
}

func TestCodeDashboardService_List_PassesErrorsThrough(t *testing.T) {
	svc, ctx := codeDashboardSvc(t, &fakeCodeDashboardRepo{listErr: errors.New("db down")})
	if _, err := svc.List(ctx); err == nil || err.Error() != "db down" {
		t.Fatalf("List err = %v, want the repository's", err)
	}
}

func TestCodeDashboardService_Get(t *testing.T) {
	t.Run("non-UUID id", func(t *testing.T) {
		svc, ctx := codeDashboardSvc(t, &fakeCodeDashboardRepo{})
		_, err := svc.Get(ctx, "not-a-uuid")
		wantValidationError(t, err)
	})
	t.Run("missing", func(t *testing.T) {
		svc, ctx := codeDashboardSvc(t, &fakeCodeDashboardRepo{})
		_, err := svc.Get(ctx, testCustomDashboardID)
		wantNotFoundError(t, err)
	})
	t.Run("found", func(t *testing.T) {
		repo := &fakeCodeDashboardRepo{get: &domain.CodeDashboard{ID: testCustomDashboardID, Name: "Open Cases", Source: "x"}}
		svc, ctx := codeDashboardSvc(t, repo)
		got, err := svc.Get(ctx, testCustomDashboardID)
		if err != nil || got.Name != "Open Cases" || got.Source != "x" {
			t.Fatalf("Get = %+v, %v", got, err)
		}
		if repo.lastOwnerID != testPlatformUserID {
			t.Errorf("owner = %q, want %q", repo.lastOwnerID, testPlatformUserID)
		}
	})
}

// --- Save

func TestCodeDashboardService_Save_RejectsBadInput(t *testing.T) {
	cases := map[string]struct {
		id  string
		req domain.SaveCodeDashboardRequest
	}{
		"non-UUID id":    {"abc", validCodeDashboardSave()},
		"blank name":     {testCustomDashboardID, domain.SaveCodeDashboardRequest{Name: "  ", Source: "x"}},
		"name too long":  {testCustomDashboardID, domain.SaveCodeDashboardRequest{Name: strings.Repeat("é", domain.MaxCustomDashboardNameLength+1), Source: "x"}},
		"blank source":   {testCustomDashboardID, domain.SaveCodeDashboardRequest{Name: "X", Source: " \n\t "}},
		"source too big": {testCustomDashboardID, domain.SaveCodeDashboardRequest{Name: "X", Source: strings.Repeat("a", domain.MaxCodeDashboardSourceBytes+1)}},
		"NUL in name":    {testCustomDashboardID, domain.SaveCodeDashboardRequest{Name: "a\x00b", Source: "x"}},
		"NUL in source":  {testCustomDashboardID, domain.SaveCodeDashboardRequest{Name: "X", Source: "let a = '\x00';"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &fakeCodeDashboardRepo{}
			svc, ctx := codeDashboardSvc(t, repo)
			_, err := svc.Save(ctx, tc.id, tc.req)
			wantValidationError(t, err)
			if repo.upsertCalled {
				t.Error("Upsert was called for invalid input")
			}
		})
	}
}

func TestCodeDashboardService_Save_AcceptsLimits(t *testing.T) {
	svc, ctx := codeDashboardSvc(t, &fakeCodeDashboardRepo{})
	req := domain.SaveCodeDashboardRequest{
		// 255 two-byte characters: the name limit is in characters, the source one in bytes.
		Name:   strings.Repeat("é", domain.MaxCustomDashboardNameLength),
		Source: strings.Repeat("a", domain.MaxCodeDashboardSourceBytes),
	}
	if _, err := svc.Save(ctx, testCustomDashboardID, req); err != nil {
		t.Fatalf("Save at the limits: %v", err)
	}
}

func TestCodeDashboardService_Save_TrimsNameKeepsSource(t *testing.T) {
	repo := &fakeCodeDashboardRepo{}
	svc, ctx := codeDashboardSvc(t, repo)
	source := "\n  async function render(portal, root) {}\n"
	got, err := svc.Save(ctx, testCustomDashboardID, domain.SaveCodeDashboardRequest{Name: "  Open Cases  ", Source: source})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if repo.upserted.Name != "Open Cases" || got.Name != "Open Cases" {
		t.Errorf("name = %q / %q, want trimmed", repo.upserted.Name, got.Name)
	}
	if repo.upserted.Source != source {
		t.Errorf("source = %q, want it unchanged", repo.upserted.Source)
	}
	if repo.upsertedID != testCustomDashboardID || repo.lastOwnerID != testPlatformUserID {
		t.Errorf("Upsert(owner %q, id %q), want (%q, %q)", repo.lastOwnerID, repo.upsertedID, testPlatformUserID, testCustomDashboardID)
	}
}

func TestCodeDashboardService_Save_OtherOwnersIDIsNotFound(t *testing.T) {
	svc, ctx := codeDashboardSvc(t, &fakeCodeDashboardRepo{upsertNil: true})
	_, err := svc.Save(ctx, testCustomDashboardID, validCodeDashboardSave())
	wantNotFoundError(t, err)
}

func TestCodeDashboardService_Save_PassesRepositoryErrorsThrough(t *testing.T) {
	limit := &apierror.ValidationError{Msg: "at most 50 saved advanced dashboards are allowed"}
	conflict := &apierror.ConflictError{Msg: "you already have an advanced dashboard named: Open Cases"}
	for _, want := range []error{limit, conflict} {
		svc, ctx := codeDashboardSvc(t, &fakeCodeDashboardRepo{upsertErr: want})
		if _, err := svc.Save(ctx, testCustomDashboardID, validCodeDashboardSave()); err != want {
			t.Errorf("Save err = %v, want %v", err, want)
		}
	}
}

// --- Delete

func TestCodeDashboardService_Delete(t *testing.T) {
	t.Run("non-UUID id", func(t *testing.T) {
		repo := &fakeCodeDashboardRepo{}
		svc, ctx := codeDashboardSvc(t, repo)
		wantValidationError(t, svc.Delete(ctx, "abc"))
		if repo.deleteCalled {
			t.Error("Delete was called for a non-UUID id")
		}
	})
	t.Run("missing", func(t *testing.T) {
		svc, ctx := codeDashboardSvc(t, &fakeCodeDashboardRepo{})
		wantNotFoundError(t, svc.Delete(ctx, testCustomDashboardID))
	})
	t.Run("found", func(t *testing.T) {
		repo := &fakeCodeDashboardRepo{deleteFound: true}
		svc, ctx := codeDashboardSvc(t, repo)
		if err := svc.Delete(ctx, testCustomDashboardID); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if repo.lastOwnerID != testPlatformUserID {
			t.Errorf("owner = %q, want %q", repo.lastOwnerID, testPlatformUserID)
		}
	})
}
