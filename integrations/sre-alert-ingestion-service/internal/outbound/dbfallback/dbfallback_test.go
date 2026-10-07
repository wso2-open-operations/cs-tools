// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package dbfallback

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"sre-alert-ingestion-service/internal/model"
)

func discard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

type chatServer struct {
	srv     *httptest.Server
	calls   atomic.Int64
	mu      sync.Mutex
	bodies  []string
	options []string
}

// newChatServer answers each post with status(call number), starting at 1.
func newChatServer(t *testing.T, status func(int64) int) *chatServer {
	t.Helper()
	cs := &chatServer{}
	cs.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := cs.calls.Add(1)
		b, _ := io.ReadAll(r.Body)
		cs.mu.Lock()
		cs.bodies = append(cs.bodies, string(b))
		cs.options = append(cs.options, r.URL.Query().Get("messageReplyOption"))
		cs.mu.Unlock()
		w.WriteHeader(status(n))
	}))
	t.Cleanup(cs.srv.Close)
	return cs
}

func (cs *chatServer) posts() []string {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return append([]string(nil), cs.bodies...)
}

func (cs *chatServer) replyOptions() []string {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return append([]string(nil), cs.options...)
}

func newTestClient(t *testing.T, cs *chatServer) *Client {
	t.Helper()
	c, err := New(discard(), cs.srv.URL+"/v1/spaces/SPACE1/messages?key=k&token=t", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	c.http = cs.srv.Client()
	c.gap = time.Millisecond
	return c
}

func ok(int64) int { return http.StatusOK }

func threadKey(t *testing.T, body string) string {
	t.Helper()
	var m struct {
		Thread struct {
			ThreadKey string `json:"threadKey"`
		} `json:"thread"`
	}
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatal(err)
	}
	return m.Thread.ThreadKey
}

func isParent(body string) bool { return strings.Contains(body, "DATABASE CONNECTION FAILURE") }

func TestNew_RejectsNonHTTPSWithoutEchoingURL(t *testing.T) {
	for _, u := range []string{"http://chat.googleapis.com/v1/spaces/X/messages?key=secret", "not a url", "https://"} {
		_, err := New(discard(), u, time.Second)
		if err == nil {
			t.Errorf("New(%q) succeeded, want error", u)
			continue
		}
		if strings.Contains(err.Error(), "secret") {
			t.Errorf("error leaks the URL: %v", err)
		}
	}
}

func TestNotify_PostsFailureCardThenOneReplyPerAlert(t *testing.T) {
	cs := newChatServer(t, ok)
	c := newTestClient(t, cs)
	c.Notify("aws", "req-1", []model.Alert{
		{Service: "<users/all>", Severity: "Critical", MetricName: "cpu", Description: "not shown"},
		{Service: "api", Severity: "Major", Environment: "Production"},
	})
	c.wait(context.Background())

	posts := cs.posts()
	if len(posts) != 3 {
		t.Fatalf("posts = %d, want the failure card and 2 replies", len(posts))
	}
	if !isParent(posts[0]) || !strings.Contains(posts[0], "Alerting Component | started ") {
		t.Errorf("first post should be the failure card: %s", posts[0])
	}
	key := threadKey(t, posts[0])
	options := cs.replyOptions()
	for i, p := range posts {
		if threadKey(t, p) != key || key == "" {
			t.Errorf("post %d is not in the outage thread %q: %s", i, key, p)
		}
		if options[i] != "REPLY_MESSAGE_FALLBACK_TO_NEW_THREAD" {
			t.Errorf("post %d messageReplyOption = %q", i, options[i])
		}
	}
	for _, want := range []string{"Alert not stored.", "Severity: Critical", "Metric: cpu", "Source: aws", "Request ID: req-1", "\\u0026lt;users/all\\u0026gt;"} {
		if !strings.Contains(posts[1], want) {
			t.Errorf("first reply missing %q: %s", want, posts[1])
		}
	}
	if strings.Contains(posts[1], "<users/all>") || strings.Contains(posts[1], "not shown") || strings.Contains(posts[1], "Category") {
		t.Errorf("reply should be escaped, carry no description, and skip empty fields: %s", posts[1])
	}
}

func TestNotify_OneThreadPerOutage(t *testing.T) {
	cs := newChatServer(t, ok)
	c := newTestClient(t, cs)

	c.Notify("aws", "r1", []model.Alert{{Service: "one"}})
	c.wait(context.Background())
	c.Notify("aws", "r2", []model.Alert{{Service: "two"}})
	c.wait(context.Background())
	c.Recovered()
	c.Notify("aws", "r3", []model.Alert{{Service: "three"}})
	c.wait(context.Background())

	posts := cs.posts()
	if len(posts) != 5 {
		t.Fatalf("posts = %d, want card, reply, reply, card, reply", len(posts))
	}
	first, second := threadKey(t, posts[0]), threadKey(t, posts[3])
	if !isParent(posts[0]) || isParent(posts[2]) || !isParent(posts[3]) {
		t.Error("a failure card should open each outage, and only once")
	}
	if threadKey(t, posts[2]) != first {
		t.Error("alerts before recovery should stay in the first thread")
	}
	if first == second || threadKey(t, posts[4]) != second {
		t.Error("alerts after recovery should go to a new thread")
	}
}

func TestNotify_CapsPendingAndSummarisesTheRest(t *testing.T) {
	cs := newChatServer(t, ok)
	c := newTestClient(t, cs)
	alerts := make([]model.Alert, maxPending+50)
	for i := range alerts {
		alerts[i] = model.Alert{Service: "svc"}
	}
	c.mu.Lock()
	c.running = true // hold the loop so every alert is queued at once
	c.mu.Unlock()
	c.Notify("aws", "r", alerts)
	c.mu.Lock()
	pending, dropped := len(c.pending), c.dropped
	c.running = false
	c.mu.Unlock()
	if pending != maxPending || dropped != 50 {
		t.Fatalf("pending = %d, dropped = %d; want %d, 50", pending, dropped, maxPending)
	}

	c.Notify("aws", "r", nil)
	c.wait(context.Background())
	posts := cs.posts()
	if len(posts) != maxPending+2 {
		t.Fatalf("posts = %d, want the card, %d replies and one summary", len(posts), maxPending)
	}
	if last := posts[len(posts)-1]; !strings.Contains(last, "50 more alert(s) not stored.") {
		t.Errorf("last post should summarise the dropped alerts: %s", last)
	}
}

func TestClose_SummarisesQueueAtTheUsualPace(t *testing.T) {
	cs := newChatServer(t, ok)
	c := newTestClient(t, cs)
	c.gap = 100 * time.Millisecond
	alerts := make([]model.Alert, 5)
	for i := range alerts {
		alerts[i] = model.Alert{Service: "svc"}
	}
	start := time.Now()
	c.Notify("aws", "r", alerts)
	c.Close(context.Background())

	posts := cs.posts()
	if len(posts) != 2 || !isParent(posts[0]) {
		t.Fatalf("posts = %d, want the failure card and one summary instead of 5 replies", len(posts))
	}
	if !strings.Contains(posts[1], "5 more alert(s) not stored.") {
		t.Errorf("summary should count every queued alert: %s", posts[1])
	}
	if elapsed := time.Since(start); elapsed < c.gap {
		t.Errorf("shutdown posted after %v, before the %v gap", elapsed, c.gap)
	}
}

func TestNotify_KeepsOnlyBoundedSummary(t *testing.T) {
	cs := newChatServer(t, ok)
	c := newTestClient(t, cs)
	big := strings.Repeat("x", 1<<20)
	c.mu.Lock()
	c.running = true // hold the loop so the entry stays queued
	c.mu.Unlock()
	c.Notify(big, big, []model.Alert{{Service: big, MetricName: big, Severity: big, Category: big, Environment: big, UniqueIdentifier: big, Description: big}})
	c.mu.Lock()
	e := c.pending[0]
	c.running = false
	c.mu.Unlock()
	size := len(e.source) + len(e.requestID) + len(e.severity) + len(e.service) + len(e.metric) + len(e.environment) + len(e.category) + len(e.id)
	if limit := 8 * (maxField + 3); size > limit {
		t.Errorf("queued entry holds %d bytes, want at most %d", size, limit)
	}
}

func TestSend_RetriesServerErrorsButNotClientErrors(t *testing.T) {
	cases := map[string]struct {
		status func(int64) int
		want   int64
	}{
		"5xx then ok": {func(n int64) int {
			if n == 1 {
				return http.StatusServiceUnavailable
			}
			return http.StatusOK
		}, 2},
		"429 retried":     {func(int64) int { return http.StatusTooManyRequests }, sendAttempts},
		"400 not retried": {func(int64) int { return http.StatusBadRequest }, 1},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cs := newChatServer(t, tc.status)
			c := newTestClient(t, cs)
			c.send(reply("thread", "text"), 1)
			if got := cs.calls.Load(); got != tc.want {
				t.Errorf("calls = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestFailureCard_ShowsStartInIST(t *testing.T) {
	started := time.Date(2026, 10, 7, 9, 16, 45, 0, time.UTC)
	body, err := json.Marshal(failureCard("thread", started))
	if err != nil {
		t.Fatal(err)
	}
	if want := "Alerting Component | started 2026-10-07 14:46:45 IST"; !strings.Contains(string(body), want) {
		t.Errorf("card missing %q: %s", want, body)
	}
}

func TestSpaceID(t *testing.T) {
	if got := spaceID("https://chat.googleapis.com/v1/spaces/AAA/messages?key=k"); got != "AAA" {
		t.Errorf("spaceID = %q, want AAA", got)
	}
	if got := spaceID("https://example.com/hook"); got != "unknown" {
		t.Errorf("spaceID = %q, want unknown", got)
	}
}
