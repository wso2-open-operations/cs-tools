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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

// mockGoogleChatAlertSender is a test double for googleChatAlertSender.
type mockGoogleChatAlertSender struct {
	err                                                     error
	gotProduct, gotTitle, gotShortDescription, gotPortalURL string
	called                                                  bool
}

func (m *mockGoogleChatAlertSender) SendIncidentAlert(ctx context.Context, product, title, shortDescription, portalURL string) error {
	m.called = true
	m.gotProduct, m.gotTitle, m.gotShortDescription, m.gotPortalURL = product, title, shortDescription, portalURL
	return m.err
}

// mockAlertIncidents is a test double for alertIncidentClient.
type mockAlertIncidents struct {
	body  string
	err   error
	gotID string
}

func (m *mockAlertIncidents) GetIncident(_ context.Context, id string) ([]byte, error) {
	m.gotID = id
	return []byte(m.body), m.err
}

const testAlertIncidentID = "42424242-4242-4242-4242-424242424242"

const validGoogleChatAlertBody = `{"product":"api-manager","title":"caller title","shortDescription":"caller text","caseId":"` + testAlertIncidentID + `"}`

const testAlertIncident = `{"id":"` + testAlertIncidentID + `","number":"INC0001234","priority":"CRITICAL","subject":"Gateway <b>down</b> & \"unreachable\""}`

func newTestNotificationHandler(chat *mockGoogleChatAlertSender, inc *mockAlertIncidents, base string) *NotificationHandler {
	return NewNotificationHandler(chat, inc, base)
}

func TestPostGoogleChatAlert_RequiresAuth(t *testing.T) {
	h := newTestNotificationHandler(&mockGoogleChatAlertSender{}, &mockAlertIncidents{}, "https://portal.example.com")

	r := httptest.NewRequest(http.MethodPost, "/notifications/google-chat/alerts", nil)
	w := httptest.NewRecorder()
	h.PostGoogleChatAlert(w, r)

	assertStatus(t, w, http.StatusUnauthorized)
}

func TestPostGoogleChatAlert_RequiresProductAndIncidentID(t *testing.T) {
	cases := map[string]string{
		"missing product":         `{"caseId":"` + testAlertIncidentID + `"}`,
		"missing caseId":          `{"product":"api-manager"}`,
		"caseId not a uuid":       `{"product":"api-manager","caseId":"CASE-42"}`,
		"empty body":              ``,
		"whitespace-only product": `{"product":"   ","caseId":"` + testAlertIncidentID + `"}`,
		"unknown property":        `{"product":"api-manager","caseId":"` + testAlertIncidentID + `","extra":"x"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			chat := &mockGoogleChatAlertSender{}
			h := newTestNotificationHandler(chat, &mockAlertIncidents{body: testAlertIncident}, "https://portal.example.com")

			r := withUser(httptest.NewRequest(http.MethodPost, "/notifications/google-chat/alerts", strings.NewReader(body)))
			w := httptest.NewRecorder()
			h.PostGoogleChatAlert(w, r)

			assertStatus(t, w, http.StatusBadRequest)
			assertErrorMessage(t, w, ErrMsgBadRequest)
			if chat.called {
				t.Error("upstream should not have been called")
			}
		})
	}
}

func TestPostGoogleChatAlert_BuildsCardFromIncidentRecord(t *testing.T) {
	chat := &mockGoogleChatAlertSender{}
	inc := &mockAlertIncidents{body: testAlertIncident}
	h := newTestNotificationHandler(chat, inc, "https://portal.example.com/")

	r := withUser(httptest.NewRequest(http.MethodPost, "/notifications/google-chat/alerts", strings.NewReader(validGoogleChatAlertBody)))
	w := httptest.NewRecorder()
	h.PostGoogleChatAlert(w, r)

	assertStatus(t, w, http.StatusOK)
	if inc.gotID != testAlertIncidentID {
		t.Errorf("incident lookup id = %q", inc.gotID)
	}
	if chat.gotProduct != "api-manager" {
		t.Errorf("product = %q", chat.gotProduct)
	}
	if chat.gotTitle != "CRITICAL Incident - INC0001234" {
		t.Errorf("title = %q, want the record's priority and number", chat.gotTitle)
	}
	if strings.Contains(chat.gotTitle+chat.gotShortDescription, "caller") {
		t.Errorf("caller-supplied text reached the card: %q / %q", chat.gotTitle, chat.gotShortDescription)
	}
	if chat.gotShortDescription != "Gateway &lt;b&gt;down&lt;/b&gt; &amp; &#34;unreachable&#34;" {
		t.Errorf("shortDescription = %q, want the HTML-escaped subject", chat.gotShortDescription)
	}
	if chat.gotPortalURL != "https://portal.example.com/operations/incidents/"+testAlertIncidentID {
		t.Errorf("portalURL = %q", chat.gotPortalURL)
	}
}

func TestPostGoogleChatAlert_UnknownIncidentIs404(t *testing.T) {
	chat := &mockGoogleChatAlertSender{}
	h := newTestNotificationHandler(chat, &mockAlertIncidents{err: &apierror.Error{StatusCode: http.StatusNotFound}}, "https://portal.example.com")

	r := withUser(httptest.NewRequest(http.MethodPost, "/notifications/google-chat/alerts", strings.NewReader(validGoogleChatAlertBody)))
	w := httptest.NewRecorder()
	h.PostGoogleChatAlert(w, r)

	assertStatus(t, w, http.StatusNotFound)
	if chat.called {
		t.Error("no alert may be sent for an unknown incident")
	}
}

func TestPostGoogleChatAlert_RejectsInvalidJSON(t *testing.T) {
	h := newTestNotificationHandler(&mockGoogleChatAlertSender{}, &mockAlertIncidents{}, "https://portal.example.com")

	r := withUser(httptest.NewRequest(http.MethodPost, "/notifications/google-chat/alerts", strings.NewReader("not json")))
	w := httptest.NewRecorder()
	h.PostGoogleChatAlert(w, r)

	assertStatus(t, w, http.StatusBadRequest)
	assertErrorMessage(t, w, ErrMsgBadRequest)
}

func TestPostGoogleChatAlert_MapsUpstreamFailure(t *testing.T) {
	chat := &mockGoogleChatAlertSender{err: fmt.Errorf("webhook unreachable")}
	h := newTestNotificationHandler(chat, &mockAlertIncidents{body: testAlertIncident}, "https://portal.example.com")

	r := withUser(httptest.NewRequest(http.MethodPost, "/notifications/google-chat/alerts", strings.NewReader(validGoogleChatAlertBody)))
	w := httptest.NewRecorder()
	h.PostGoogleChatAlert(w, r)

	assertStatus(t, w, http.StatusInternalServerError)
}

func TestPostGoogleChatAlert_RateLimitedPerCaller(t *testing.T) {
	chat := &mockGoogleChatAlertSender{}
	h := newTestNotificationHandler(chat, &mockAlertIncidents{body: testAlertIncident}, "https://portal.example.com")
	send := func(withCaller func(*http.Request) *http.Request) int {
		w := httptest.NewRecorder()
		h.PostGoogleChatAlert(w, withCaller(httptest.NewRequest(http.MethodPost, "/notifications/google-chat/alerts", strings.NewReader(validGoogleChatAlertBody))))
		return w.Code
	}
	for i := 0; i < alertRateLimit; i++ {
		if code := send(withUser); code != http.StatusOK {
			t.Fatalf("alert %d: status = %d", i+1, code)
		}
	}
	if code := send(withUser); code != http.StatusTooManyRequests {
		t.Fatalf("alert over the limit: status = %d, want 429", code)
	}
	if code := send(withCsEngineerUser); code != http.StatusOK {
		t.Fatalf("another caller: status = %d, want 200", code)
	}
}

func TestAlertRateLimiterWindow(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := newAlertRateLimiter(2, time.Minute, func() time.Time { return now })
	for i := 0; i < 2; i++ {
		if !l.allow("a") {
			t.Fatalf("event %d must be allowed", i+1)
		}
	}
	if l.allow("a") {
		t.Fatal("third event inside the window must be refused")
	}
	now = now.Add(61 * time.Second)
	if !l.allow("a") {
		t.Fatal("event after the window must be allowed")
	}
	now = now.Add(2 * time.Minute)
	l.allow("b")
	if _, ok := l.events["a"]; ok {
		t.Fatal("expired key must be dropped")
	}
}
