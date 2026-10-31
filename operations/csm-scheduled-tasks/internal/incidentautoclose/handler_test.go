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

package incidentautoclose

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

// fakeCloser answers each call with the next scripted page; once the script
// runs out it answers with the last page again.
type fakeCloser struct {
	pages []Response
	errAt int // 1-based call that fails; 0 = none
	calls []Request
}

func (f *fakeCloser) AutoClose(_ context.Context, req Request) (Response, error) {
	f.calls = append(f.calls, req)
	n := len(f.calls)
	if n == f.errAt {
		return Response{}, errors.New("upstream returned 503")
	}
	if len(f.pages) == 0 {
		return Response{DryRun: req.DryRun}, nil
	}
	return f.pages[min(n, len(f.pages))-1], nil
}

func closedPage(prefix string, n int, more bool) Response {
	r := Response{More: more}
	for i := range n {
		id := fmt.Sprintf("%s%d", prefix, i)
		r.Closed = append(r.Closed, Closed{ID: id, Number: "INC-" + id, ResolvedOn: "2026-10-01T00:00:00Z"})
	}
	return r
}

func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var logged bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &logged
}

func TestRun_NothingDueIsASuccess(t *testing.T) {
	f := &fakeCloser{}
	if err := Run(f, Config{Days: 14})(context.Background()); err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(f.calls) != 1 {
		t.Errorf("calls = %d, want 1", len(f.calls))
	}
}

func TestRun_SendsDaysLimitAndDryRun(t *testing.T) {
	f := &fakeCloser{}
	if err := Run(f, Config{Days: 21, DryRun: true})(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := f.calls[0]; got != (Request{ResolvedForDays: 21, Limit: PageSize, DryRun: true}) {
		t.Errorf("request = %+v", got)
	}
}

func TestRun_PagesWhileMore(t *testing.T) {
	f := &fakeCloser{pages: []Response{closedPage("a", 3, true), closedPage("b", 3, true), closedPage("c", 1, false)}}
	if err := Run(f, Config{Days: 14})(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 3 {
		t.Errorf("calls = %d, want 3 (stop once more is false)", len(f.calls))
	}
}

func TestRun_StopsAtPageCapAndLogs(t *testing.T) {
	logged := captureLogs(t)
	f := &fakeCloser{pages: []Response{closedPage("a", 2, true)}} // always more
	if err := Run(f, Config{Days: 14})(context.Background()); err != nil {
		t.Fatalf("hitting the cap is not a failure: %v", err)
	}
	if len(f.calls) != MaxPages {
		t.Errorf("calls = %d, want the cap of %d", len(f.calls), MaxPages)
	}
	if !strings.Contains(logged.String(), "page cap reached") {
		t.Errorf("no cap warning logged: %s", logged.String())
	}
}

// A dry run closes nothing, so the next page would be the same page.
func TestRun_DryRunAsksOnce(t *testing.T) {
	f := &fakeCloser{pages: []Response{{DryRun: true, Closed: closedPage("a", 2, true).Closed, More: true}}}
	if err := Run(f, Config{Days: 14, DryRun: true})(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 1 {
		t.Errorf("calls = %d, want 1", len(f.calls))
	}
}

func TestRun_DryRunIgnoredByEntityServiceIsAnError(t *testing.T) {
	f := &fakeCloser{pages: []Response{closedPage("a", 1, false)}}
	err := Run(f, Config{Days: 14, DryRun: true})(context.Background())
	if err == nil || !strings.Contains(err.Error(), "dry run") {
		t.Errorf("err = %v, want a dry-run mismatch", err)
	}
}

func failedPage(prefix string, n int, more bool) Response {
	r := Response{More: more}
	for i := range n {
		id := fmt.Sprintf("%s%d", prefix, i)
		r.Failed = append(r.Failed, Failed{ID: id, Number: "INC-" + id, Error: "boom"})
	}
	return r
}

// entity-service lists the oldest due first with no offset, so incidents that
// keep failing come back at the head of every page. Asking for PageSize plus
// the failures seen gets past them to the incidents behind.
func TestRun_FailuresDoNotHideLaterIncidents(t *testing.T) {
	second := closedPage("b", PageSize, false)
	second.Failed = failedPage("x", PageSize, false).Failed
	f := &fakeCloser{pages: []Response{failedPage("x", PageSize, true), second}}
	err := Run(f, Config{Days: 14})(context.Background())
	if len(f.calls) != 2 || f.calls[0].Limit != PageSize || f.calls[1].Limit != 2*PageSize {
		t.Errorf("calls = %+v, want limits %d then %d", f.calls, PageSize, 2*PageSize)
	}
	if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("%d incidents could not be closed", PageSize)) {
		t.Errorf("err = %v, want the failures named", err)
	}
}

// Once a page holds only incidents that already failed (the limit can grow no
// further), asking again would return the same page: stop, and say that later
// due incidents were not reached rather than only naming the failures.
func TestRun_PageOfOnlyKnownFailuresStopsAndSaysSo(t *testing.T) {
	f := &fakeCloser{pages: []Response{failedPage("x", 1, true)}}
	err := Run(f, Config{Days: 14})(context.Background())
	if len(f.calls) != 2 {
		t.Errorf("calls = %d, want 2 (the second page brings nothing new)", len(f.calls))
	}
	if err == nil || !strings.Contains(err.Error(), "later due incidents were not reached") {
		t.Errorf("err = %v, want it to say later incidents were not reached", err)
	}
}

func TestRun_LimitIsCappedAtTheEndpointMaximum(t *testing.T) {
	f := &fakeCloser{pages: []Response{failedPage("a", maxLimit, true)}}
	_ = Run(f, Config{Days: 14})(context.Background())
	for _, c := range f.calls {
		if c.Limit > maxLimit {
			t.Errorf("limit %d is above the endpoint's maximum %d", c.Limit, maxLimit)
		}
	}
}

func TestRun_FailedEntriesAreAnErrorNamingThem(t *testing.T) {
	page := closedPage("a", 2, true)
	page.Failed = []Failed{{ID: "f1", Number: "INC0001", Error: "lock timeout"}}
	again := closedPage("b", 1, false)
	again.Failed = page.Failed // the same row fails again on the next page
	f := &fakeCloser{pages: []Response{page, again}}
	err := Run(f, Config{Days: 14})(context.Background())
	if err == nil {
		t.Fatal("want an error")
	}
	if !strings.Contains(err.Error(), "INC0001: lock timeout") || strings.Count(err.Error(), "INC0001") != 1 {
		t.Errorf("err = %v, want INC0001 named once", err)
	}
	if len(f.calls) != 2 {
		t.Errorf("calls = %d, want paging to continue past a failure", len(f.calls))
	}
}

func TestRun_ManyFailuresAreBounded(t *testing.T) {
	var page Response
	for i := range 15 {
		page.Failed = append(page.Failed, Failed{ID: fmt.Sprint(i), Number: fmt.Sprintf("INC%02d", i), Error: "x"})
	}
	err := Run(&fakeCloser{pages: []Response{page}}, Config{Days: 14})(context.Background())
	if err == nil || !strings.Contains(err.Error(), "15 incidents") || !strings.Contains(err.Error(), "and 5 more") || strings.Contains(err.Error(), "INC14") {
		t.Errorf("err = %v", err)
	}
}

func TestRun_HTTPErrorIsAnError(t *testing.T) {
	f := &fakeCloser{pages: []Response{closedPage("a", 2, true)}, errAt: 2}
	err := Run(f, Config{Days: 14})(context.Background())
	if err == nil || !strings.Contains(err.Error(), "503") {
		t.Errorf("err = %v, want the failed call", err)
	}
	if len(f.calls) != 2 {
		t.Errorf("calls = %d, want to stop at the failed call", len(f.calls))
	}
}

func TestRun_CancelledContextStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f := &fakeCloser{}
	if err := Run(f, Config{Days: 14})(ctx); err == nil {
		t.Error("want an error")
	}
	if len(f.calls) != 0 {
		t.Errorf("calls = %d, want none", len(f.calls))
	}
}

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestConfigFromEnv_Defaults(t *testing.T) {
	cfg, err := ConfigFromEnv(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if cfg != (Config{Days: 14, DryRun: false}) {
		t.Errorf("cfg = %+v", cfg)
	}
}

func TestConfigFromEnv_Values(t *testing.T) {
	cfg, err := ConfigFromEnv(env(map[string]string{"INCIDENT_AUTOCLOSE_DAYS": " 7 ", "INCIDENT_AUTOCLOSE_DRY_RUN": "true"}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg != (Config{Days: 7, DryRun: true}) {
		t.Errorf("cfg = %+v", cfg)
	}
}

func TestConfigFromEnv_RejectsBadValues(t *testing.T) {
	for _, m := range []map[string]string{
		{"INCIDENT_AUTOCLOSE_DAYS": "0"},
		{"INCIDENT_AUTOCLOSE_DAYS": "-3"},
		{"INCIDENT_AUTOCLOSE_DAYS": "14d"},
		{"INCIDENT_AUTOCLOSE_DRY_RUN": "yes please"},
	} {
		if _, err := ConfigFromEnv(env(m)); err == nil {
			t.Errorf("%v was accepted", m)
		}
	}
}
