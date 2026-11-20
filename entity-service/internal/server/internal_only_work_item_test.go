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

package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/config"
)

type fakeRejecter struct {
	err   error
	gotID string
}

func (f *fakeRejecter) RejectInternalOnly(_ context.Context, id string) error {
	f.gotID = id
	return f.err
}

func serveGuarded(t *testing.T, g internalOnlyWorkItemRejecter) (*httptest.ResponseRecorder, *bool) {
	t.Helper()
	called := false
	mux := http.NewServeMux()
	mux.HandleFunc("POST /cases/{id}/comments", refuseInternalOnlyWorkItem(g, func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
	}))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/cases/0190a4b0-0000-7000-8000-000000000001/comments", nil))
	return rec, &called
}

func TestRefuseInternalOnlyWorkItem(t *testing.T) {
	t.Run("an ordinary work item reaches the handler", func(t *testing.T) {
		f := &fakeRejecter{}
		rec, called := serveGuarded(t, f)
		if !*called || rec.Code != http.StatusCreated || f.gotID != "0190a4b0-0000-7000-8000-000000000001" {
			t.Fatalf("called=%v code=%d id=%q", *called, rec.Code, f.gotID)
		}
	})
	t.Run("an internal-only work item gets the 404 and the handler does not run", func(t *testing.T) {
		rec, called := serveGuarded(t, &fakeRejecter{err: &apierror.NotFoundError{Msg: "case not found"}})
		if *called || rec.Code != http.StatusNotFound {
			t.Fatalf("called=%v code=%d", *called, rec.Code)
		}
	})
	t.Run("a failed lookup fails open: the request reaches the route", func(t *testing.T) {
		rec, called := serveGuarded(t, &fakeRejecter{err: errors.New("db down")})
		if !*called || rec.Code != http.StatusCreated {
			t.Fatalf("called=%v code=%d", *called, rec.Code)
		}
	})
}

type countingRejecter struct {
	calls   int
	refusal map[string]bool
}

func (c *countingRejecter) RejectInternalOnly(_ context.Context, id string) error {
	c.calls++
	if c.refusal[id] {
		return &apierror.NotFoundError{Msg: "case not found"}
	}
	return nil
}

// routerWithGuard builds the real router over a pool that points at nothing (pgx
// connects lazily) with the guard's lookup replaced by g.
func routerWithGuard(t *testing.T, gitInternals bool, g internalOnlyWorkItemRejecter) http.Handler {
	t.Helper()
	orig := newWorkItemTypeGuard
	newWorkItemTypeGuard = func(*pgxpool.Pool) internalOnlyWorkItemRejecter { return g }
	t.Cleanup(func() { newWorkItemTypeGuard = orig })
	pool, err := pgxpool.New(context.Background(), "postgres://entity:entity@127.0.0.1:1/entity?sslmode=disable")
	if err != nil {
		t.Fatalf("build pool: %v", err)
	}
	t.Cleanup(pool.Close)
	cfg := &config.Config{DataSource: config.DataSourcePostgres}
	if gitInternals {
		cfg.GitInternalsEnabled = true
		cfg.EngineeringEntityBaseURL = "http://engineering.invalid"
	}
	withTestAuth(t, cfg)
	router, _ := NewRouter(pool, cfg)
	return router
}

const guardedProductIssueID = "0190a4b0-0000-7000-8000-0000000000c1"

func TestCaseIDGuard_WiredOnlyWhenGitInternalsIsActive(t *testing.T) {
	routes := []struct{ method, path string }{
		{http.MethodPatch, "/cases/" + guardedProductIssueID},
		{http.MethodPost, "/cases/" + guardedProductIssueID + "/comments"},
		{http.MethodPost, "/cases/" + guardedProductIssueID + "/tags"},
		{http.MethodGet, "/cases/" + guardedProductIssueID + "/escalations"},
		{http.MethodPost, "/cases/" + guardedProductIssueID + "/comments/search"},
	}
	t.Run("flag off: no route is wrapped, the lookup never runs", func(t *testing.T) {
		g := &countingRejecter{refusal: map[string]bool{guardedProductIssueID: true}}
		router := routerWithGuard(t, false, g)
		for _, rt := range routes {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(rt.method, rt.path, strings.NewReader(`{}`)))
			if strings.Contains(rec.Body.String(), "case not found") && g.calls > 0 {
				t.Fatalf("%s %s was guarded with the flag off", rt.method, rt.path)
			}
		}
		if g.calls != 0 {
			t.Fatalf("the guard ran %d times with the flag off", g.calls)
		}
	})
	t.Run("flag on: a product issue id gets 404 on every wrapped route; another id passes", func(t *testing.T) {
		g := &countingRejecter{refusal: map[string]bool{guardedProductIssueID: true}}
		router := routerWithGuard(t, true, g)
		for _, rt := range routes {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(rt.method, rt.path, strings.NewReader(`{}`)))
			if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "case not found") {
				t.Errorf("%s %s = %d %s, want the guard's 404", rt.method, rt.path, rec.Code, rec.Body.String())
			}
		}
		before := g.calls
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/cases/0190a4b0-0000-7000-8000-0000000000c2/comments", strings.NewReader(`{}`)))
		if g.calls != before+1 || strings.Contains(rec.Body.String(), "case not found") {
			t.Errorf("ordinary id: calls %d->%d body=%s", before, g.calls, rec.Body.String())
		}
	})
}

func TestNewCaseIDGuard_IsTheIdentityWithoutTheFlagOrAPool(t *testing.T) {
	called := 0
	h := func(http.ResponseWriter, *http.Request) { called++ }
	orig := newWorkItemTypeGuard
	t.Cleanup(func() { newWorkItemTypeGuard = orig })
	newWorkItemTypeGuard = func(*pgxpool.Pool) internalOnlyWorkItemRejecter {
		t.Fatal("the lookup must not even be built")
		return nil
	}
	on := &config.Config{DataSource: config.DataSourcePostgres, GitInternalsEnabled: true, EngineeringEntityBaseURL: "http://e.invalid"}
	for name, g := range map[string]func(http.HandlerFunc) http.HandlerFunc{
		"flag off":       newCaseIDGuard(&config.Config{DataSource: config.DataSourcePostgres}, &pgxpool.Pool{}),
		"no pool":        newCaseIDGuard(on, nil),
		"no engineering": newCaseIDGuard(&config.Config{DataSource: config.DataSourcePostgres, GitInternalsEnabled: true}, &pgxpool.Pool{}),
	} {
		g(h)(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/cases/x/comments", nil))
		if called == 0 {
			t.Errorf("%s: handler not reached", name)
		}
		called = 0
	}
}
