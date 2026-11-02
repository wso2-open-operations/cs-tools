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

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/csmai"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/dashboard"
)

// testLimiter is a limiter on a clock the test moves (advance).
func testLimiter(perMinute, perDay int) (*GenerateRateLimiter, func(time.Duration)) {
	now := time.Date(2026, 10, 29, 9, 0, 0, 0, time.UTC)
	l := NewGenerateRateLimiter(perMinute, perDay)
	l.now = func() time.Time { return now }
	return l, func(d time.Duration) { now = now.Add(d) }
}

func TestGenerateRateLimiter_PerMinute(t *testing.T) {
	l, advance := testLimiter(10, 100)
	for i := 0; i < 10; i++ {
		if ok, _, _ := l.Allow("jane"); !ok {
			t.Fatalf("call %d refused, want the first 10 in a minute allowed", i+1)
		}
		advance(time.Second)
	}
	ok, retry, daily := l.Allow("jane")
	if ok || daily || retry != 50*time.Second {
		t.Fatalf("11th call: ok %v, daily %v, retry %v; want refused by the per-minute limit, retry 50s (the first call leaves the window then)", ok, daily, retry)
	}
	if ok, _, _ := l.Allow("ben"); !ok {
		t.Error("another user was refused: the limit is per user")
	}
	advance(50 * time.Second)
	if ok, _, _ := l.Allow("jane"); !ok {
		t.Error("refused once the oldest call left the minute")
	}
}

func TestGenerateRateLimiter_PerDay(t *testing.T) {
	l, advance := testLimiter(10, 3)
	for i := 0; i < 3; i++ {
		if ok, _, _ := l.Allow("jane"); !ok {
			t.Fatalf("call %d refused", i+1)
		}
		advance(time.Hour)
	}
	ok, retry, daily := l.Allow("jane")
	if ok || !daily || retry != 21*time.Hour {
		t.Fatalf("4th call: ok %v, daily %v, retry %v; want refused by the daily limit, retry 21h", ok, daily, retry)
	}
	// A refused call is not recorded, so it does not push the window out.
	advance(21 * time.Hour)
	if ok, _, _ := l.Allow("jane"); !ok {
		t.Error("refused once the first call was 24 hours old")
	}
}

func TestGenerateRateLimiter_SweepDropsIdleUsers(t *testing.T) {
	l, advance := testLimiter(10, 100)
	l.Allow("jane")
	advance(25 * time.Hour)
	l.seen = generateLimitSweepEvery - 1
	l.Allow("ben")
	if _, ok := l.calls["jane"]; ok || len(l.calls) != 1 {
		t.Errorf("users kept: %v, want only ben (jane's last call is over 24 hours old)", l.calls)
	}
}

func TestGenerateDashboard_RateLimited(t *testing.T) {
	l, _ := testLimiter(2, 100)
	fake := &fakeGenerator{result: GeneratedDashboard{Dashboard: dashboard.Dashboard{DisplayName: "X"}, Reply: "ok"}}
	h := NewDashboardGenerateHandler(fake).WithRateLimit(l)
	send := func(body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.GenerateDashboard(w, withUser(newGenerateRequest(body)))
		return w
	}

	// A refused prompt is not counted: it never reaches the model.
	if w := send(`{"prompt":"  "}`); w.Code != http.StatusBadRequest {
		t.Fatalf("blank prompt: %d", w.Code)
	}
	for i := 0; i < 2; i++ {
		if w := send(`{"prompt":"x"}`); w.Code != http.StatusOK {
			t.Fatalf("call %d: %d %s", i+1, w.Code, w.Body.String())
		}
	}
	w := send(`{"prompt":"x"}`)
	if w.Code != http.StatusTooManyRequests || fake.calls != 2 {
		t.Fatalf("3rd call: status %d, generator calls %d; want 429 and 2", w.Code, fake.calls)
	}
	if w.Header().Get("Retry-After") != "60" {
		t.Errorf("Retry-After = %q, want 60", w.Header().Get("Retry-After"))
	}
	if msg := decodeJSON[map[string]any](t, w)["message"]; !strings.Contains(msg.(string), "Try again in 60 seconds") {
		t.Errorf("message = %v", msg)
	}
}

func TestServiceGenerator_RefusesADraftOverTheCap(t *testing.T) {
	big := `{"id":"` + generateTestDraftID + `","displayName":"` + strings.Repeat("a", generateDraftMaxBytes) + `","widgets":[]}`
	ai := &fakeAIClient{}
	_, err := NewServiceDashboardGenerator(ai, nil).Generate(context.Background(), "p", []byte(big))
	if !errors.Is(err, errDraftTooLarge) || ai.gotReq.Prompt != "" {
		t.Fatalf("err = %v, service called with %+v; want errDraftTooLarge and no call", err, ai.gotReq)
	}
	ai = &fakeAIClient{res: csmai.DashboardResult{Candidates: []json.RawMessage{json.RawMessage(validGenerated)}}}
	small := `{"id":"` + generateTestDraftID + `","displayName":"` + strings.Repeat("a", 1000) + `","widgets":[]}`
	if _, err := NewServiceDashboardGenerator(ai, nil).Generate(context.Background(), "p", []byte(small)); err != nil || ai.gotReq.Draft == nil {
		t.Fatalf("a small draft: err %v, draft sent %s", err, ai.gotReq.Draft)
	}
}
