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

package standardchangesuggestions

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/apierror"
)

type fakeRefresher struct {
	pages []Response
	errAt int   // 1-based page that fails; 0 never
	err   error // what it fails with (a 503 when nil)
	reqs  []Request
}

func (f *fakeRefresher) Refresh(_ context.Context, req Request) (Response, error) {
	f.reqs = append(f.reqs, req)
	n := len(f.reqs)
	if n == f.errAt {
		if f.err != nil {
			return Response{}, f.err
		}
		return Response{}, errors.New("503 from entity-service")
	}
	return f.pages[n-1], nil
}

func TestRun_PagesUntilPublished(t *testing.T) {
	f := &fakeRefresher{pages: []Response{
		{RunID: "r1", Customers: 50, More: true, NextCursor: "a50"},
		{RunID: "r1", Customers: 50, More: true, NextCursor: "a100"},
		{RunID: "r1", Customers: 7, ComputedOn: "2026-11-06T14:00:05Z", Totals: &Totals{Customers: 107, ChangeRequests: 9000, Clusters: 300, Eligible: 12}},
	}}
	if err := Run(f)(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []Request{{Limit: PageSize}, {RunID: "r1", Cursor: "a50", Limit: PageSize}, {RunID: "r1", Cursor: "a100", Limit: PageSize}}
	if len(f.reqs) != len(want) {
		t.Fatalf("requests = %+v", f.reqs)
	}
	for i := range want {
		if f.reqs[i] != want[i] {
			t.Errorf("request %d = %+v, want %+v", i+1, f.reqs[i], want[i])
		}
	}
}

func TestRun_AFailedPageFailsTheRun(t *testing.T) {
	f := &fakeRefresher{errAt: 2, pages: []Response{{RunID: "r1", More: true, NextCursor: "a1"}}}
	err := Run(f)(context.Background())
	if err == nil || !strings.Contains(err.Error(), "page 2") || !strings.Contains(err.Error(), "nothing was published") {
		t.Fatalf("err = %v", err)
	}
}

func TestRun_NoProgressFailsTheRun(t *testing.T) {
	for name, pages := range map[string][]Response{
		"cursor did not move": {{RunID: "r1", More: true, NextCursor: "a1"}, {RunID: "r1", More: true, NextCursor: "a1"}},
		"no cursor":           {{RunID: "r1", More: true}},
		"another run":         {{RunID: "r1", More: true, NextCursor: "a1"}, {RunID: "r2", More: true, NextCursor: "a2"}},
	} {
		t.Run(name, func(t *testing.T) {
			err := Run(&fakeRefresher{pages: pages})(context.Background())
			if err == nil || !strings.Contains(err.Error(), "no progress") {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestRun_StopsWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f := &fakeRefresher{pages: []Response{{RunID: "r1"}}}
	if err := Run(f)(ctx); err == nil || len(f.reqs) != 0 {
		t.Fatalf("err = %v, requests = %d", err, len(f.reqs))
	}
}

// A run with skipped customers is still published (nil), and says so.
func TestRun_ReportsSkippedCustomers(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	f := &fakeRefresher{pages: []Response{
		{RunID: "r1", Customers: 49, Skipped: 1, More: true, NextCursor: "a50"},
		{RunID: "r1", Customers: 7, ComputedOn: "2026-11-06T14:00:05Z", Totals: &Totals{Customers: 56, Skipped: 1}},
	}}
	if err := Run(f)(context.Background()); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "skipped=1") || !strings.Contains(out, "level=WARN") {
		t.Errorf("log = %s, want the skipped count and a warning", out)
	}
}

// Another run (a run-now beside the scheduled one) published first and deleted
// this one: its next page is a 409. The first to publish wins; this one stops
// without an error (no alert), having published nothing.
func TestRun_SupersededRunIsNotAFailure(t *testing.T) {
	gone := &apierror.Error{StatusCode: http.StatusConflict, Body: `{"message":"this refresh run is no longer open"}`}
	f := &fakeRefresher{pages: []Response{{RunID: "r1", Customers: 50, More: true, NextCursor: "a50"}}, errAt: 2, err: gone}
	if err := Run(f)(context.Background()); err != nil {
		t.Fatalf("superseded run: %v, want nil", err)
	}
	// A 409 on the first page (no run yet) is still a failure.
	f = &fakeRefresher{errAt: 1, err: gone}
	if err := Run(f)(context.Background()); err == nil {
		t.Fatal("a 409 starting a run: nil, want an error")
	}
}
