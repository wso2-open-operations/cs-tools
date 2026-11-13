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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

const testSystemDashboardClientID = "csm-portal-backend-client-id"

type fakeSystemDashboardRepo struct {
	list []domain.SystemDashboard
	get  *domain.SystemDashboard

	upsertCalled bool
	upsertedKey  string
	upserted     domain.SaveSystemDashboardRequest
	upsertErr    error
	getCalled    bool

	deleteCalled bool
	deletedKey   string
	deleteFound  bool
}

func (f *fakeSystemDashboardRepo) List(context.Context) ([]domain.SystemDashboard, error) {
	return f.list, nil
}
func (f *fakeSystemDashboardRepo) Get(context.Context, string) (*domain.SystemDashboard, error) {
	f.getCalled = true
	return f.get, nil
}
func (f *fakeSystemDashboardRepo) Delete(_ context.Context, key string) (bool, error) {
	f.deleteCalled = true
	f.deletedKey = key
	return f.deleteFound, nil
}
func (f *fakeSystemDashboardRepo) Upsert(_ context.Context, key string, req domain.SaveSystemDashboardRequest) (*domain.SystemDashboard, error) {
	f.upsertCalled = true
	f.upsertedKey = key
	f.upserted = req
	if f.upsertErr != nil {
		return nil, f.upsertErr
	}
	return &domain.SystemDashboard{ID: "d1", SystemKey: key, IsSystem: true, Name: req.Name, Definition: req.Definition}, nil
}

// systemDashboardBackendCtx is the CSM portal backend calling on its own, at
// startup: its client id and no user token.
func systemDashboardBackendCtx() context.Context {
	return auth.WithIdentity(context.Background(), auth.Identity{Validated: true, ClientID: testSystemDashboardClientID})
}

func newTestSystemDashboardService(repo *fakeSystemDashboardRepo) SystemDashboardService {
	return NewSystemDashboardService(repo, testSystemDashboardClientID)
}

func TestSystemDashboardService_OnlyTheCSMPortalBackendIsAdmitted(t *testing.T) {
	repo := &fakeSystemDashboardRepo{get: &domain.SystemDashboard{ID: "d1"}}
	svc := newTestSystemDashboardService(repo)
	req := domain.SaveSystemDashboardRequest{Name: "Overview", Definition: `{}`}
	callers := map[string]context.Context{
		"no identity":     context.Background(),
		"another client":  auth.WithIdentity(context.Background(), auth.Identity{Validated: true, ClientID: "other-client"}),
		"a user only":     auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: "jane.doe@example.com"}),
		"unvalidated":     auth.WithIdentity(context.Background(), auth.Identity{ClientID: testSystemDashboardClientID}),
		"user with Roles": auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: "jane.doe@example.com", Roles: []string{"cs_admin"}}),
	}
	for name, ctx := range callers {
		if _, err := svc.List(ctx); err == nil {
			t.Errorf("%s: List admitted", name)
		}
		if _, err := svc.Get(ctx, "overview.json"); err == nil {
			t.Errorf("%s: Get admitted", name)
		}
		if _, err := svc.Save(ctx, "overview.json", req); err == nil {
			t.Errorf("%s: Save admitted", name)
		}
		if err := svc.Delete(ctx, "overview.json"); err == nil {
			t.Errorf("%s: Delete admitted", name)
		}
	}
	var forbidden *apierror.ForbiddenError
	if _, err := svc.Save(callers["another client"], "overview.json", req); !errors.As(err, &forbidden) {
		t.Errorf("another client = %v, want a ForbiddenError", err)
	}
	if repo.upsertCalled || repo.getCalled || repo.deleteCalled {
		t.Error("the repository was reached by a refused caller")
	}

	// An unset configured client id admits nobody, not even an empty client id.
	open := NewSystemDashboardService(repo, "")
	ctx := auth.WithIdentity(context.Background(), auth.Identity{Validated: true})
	if _, err := open.List(ctx); err == nil {
		t.Error("an empty configured client id admitted an anonymous caller")
	}
}

func TestSystemDashboardService_SaveValidatesAndTrims(t *testing.T) {
	big := strings.Repeat("x", domain.MaxCustomDashboardDefinitionBytes+1)
	cases := []struct {
		name string
		key  string
		req  domain.SaveSystemDashboardRequest
		ok   bool
	}{
		{"ok", "overview.json", domain.SaveSystemDashboardRequest{Name: "Overview", Definition: `{}`}, true},
		{"definition above a user's cap", "overview.json", domain.SaveSystemDashboardRequest{Name: "Overview", Definition: big}, true},
		{"definition over the system cap", "overview.json", domain.SaveSystemDashboardRequest{Name: "Overview", Definition: strings.Repeat("x", domain.MaxSystemDashboardDefinitionBytes+1)}, false},
		{"blank name", "overview.json", domain.SaveSystemDashboardRequest{Name: "   ", Definition: `{}`}, false},
		{"long name", "overview.json", domain.SaveSystemDashboardRequest{Name: strings.Repeat("n", 256), Definition: `{}`}, false},
		{"blank definition", "overview.json", domain.SaveSystemDashboardRequest{Name: "Overview", Definition: " \n"}, false},
		{"NUL in definition", "overview.json", domain.SaveSystemDashboardRequest{Name: "Overview", Definition: "{\x00}"}, false},
		{"empty key", "", domain.SaveSystemDashboardRequest{Name: "Overview", Definition: `{}`}, false},
		{"dot key", "..", domain.SaveSystemDashboardRequest{Name: "Overview", Definition: `{}`}, false},
		{"key with a slash", "a/b", domain.SaveSystemDashboardRequest{Name: "Overview", Definition: `{}`}, false},
		{"key with a space", "a b", domain.SaveSystemDashboardRequest{Name: "Overview", Definition: `{}`}, false},
		{"long key", strings.Repeat("k", 256), domain.SaveSystemDashboardRequest{Name: "Overview", Definition: `{}`}, false},
		{"max key", strings.Repeat("k", 255), domain.SaveSystemDashboardRequest{Name: "Overview", Definition: `{}`}, true},
	}
	for _, c := range cases {
		repo := &fakeSystemDashboardRepo{}
		_, err := newTestSystemDashboardService(repo).Save(systemDashboardBackendCtx(), c.key, c.req)
		if c.ok && err != nil {
			t.Errorf("%s: %v", c.name, err)
		}
		if !c.ok {
			var verr *apierror.ValidationError
			if !errors.As(err, &verr) {
				t.Errorf("%s: err = %v, want a ValidationError", c.name, err)
			}
			if repo.upsertCalled {
				t.Errorf("%s: the repository was reached", c.name)
			}
		}
	}

	repo := &fakeSystemDashboardRepo{}
	got, err := newTestSystemDashboardService(repo).Save(systemDashboardBackendCtx(), "overview.json",
		domain.SaveSystemDashboardRequest{Name: "  Overview  ", Definition: ` {"a":1} `})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if repo.upserted.Name != "Overview" || repo.upserted.Definition != ` {"a":1} ` || repo.upsertedKey != "overview.json" {
		t.Errorf("upserted %q %+v, want the name trimmed and the definition as sent", repo.upsertedKey, repo.upserted)
	}
	if !got.IsSystem || got.SystemKey != "overview.json" {
		t.Errorf("Save = %+v", got)
	}
}

func TestSystemDashboardService_ListNeverNilAndGetNotFound(t *testing.T) {
	svc := newTestSystemDashboardService(&fakeSystemDashboardRepo{})
	list, err := svc.List(systemDashboardBackendCtx())
	if err != nil || list.Dashboards == nil || len(list.Dashboards) != 0 {
		t.Errorf("List = %+v, %v; want an empty, non-nil slice", list, err)
	}
	var nf *apierror.NotFoundError
	if _, err := svc.Get(systemDashboardBackendCtx(), "missing.json"); !errors.As(err, &nf) {
		t.Errorf("Get missing = %v, want a NotFoundError", err)
	}
}

func TestSystemDashboardService_Delete(t *testing.T) {
	repo := &fakeSystemDashboardRepo{deleteFound: true}
	svc := newTestSystemDashboardService(repo)
	if err := svc.Delete(systemDashboardBackendCtx(), "overview.json"); err != nil || repo.deletedKey != "overview.json" {
		t.Errorf("Delete = %v key %q", err, repo.deletedKey)
	}

	repo = &fakeSystemDashboardRepo{}
	var nf *apierror.NotFoundError
	if err := newTestSystemDashboardService(repo).Delete(systemDashboardBackendCtx(), "missing.json"); !errors.As(err, &nf) {
		t.Errorf("Delete missing = %v, want a NotFoundError", err)
	}

	repo = &fakeSystemDashboardRepo{deleteFound: true}
	var verr *apierror.ValidationError
	if err := newTestSystemDashboardService(repo).Delete(systemDashboardBackendCtx(), "a/b"); !errors.As(err, &verr) || repo.deleteCalled {
		t.Errorf("Delete bad key = %v (repository reached: %v), want a ValidationError", err, repo.deleteCalled)
	}
}
