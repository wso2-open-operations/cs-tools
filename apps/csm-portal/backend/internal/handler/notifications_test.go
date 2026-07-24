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

const validGoogleChatAlertBody = `{"product":"api-manager","title":"P1 Incident - CASE-42","shortDescription":"Something broke","caseId":"CASE-42"}`

func TestPostGoogleChatAlert_RequiresAuth(t *testing.T) {
	h := NewNotificationHandler(&mockGoogleChatAlertSender{}, "https://portal.example.com")

	r := httptest.NewRequest(http.MethodPost, "/notifications/google-chat/alerts", nil)
	w := httptest.NewRecorder()
	h.PostGoogleChatAlert(w, r)

	assertStatus(t, w, http.StatusUnauthorized)
}

func TestPostGoogleChatAlert_RequiresAllFields(t *testing.T) {
	cases := map[string]string{
		"missing product":          `{"title":"t","shortDescription":"d","caseId":"c"}`,
		"missing title":            `{"product":"api-manager","shortDescription":"d","caseId":"c"}`,
		"missing shortDescription": `{"product":"api-manager","title":"t","caseId":"c"}`,
		"missing caseId":           `{"product":"api-manager","title":"t","shortDescription":"d"}`,
		"empty body":               ``,
		"whitespace-only product":  `{"product":"   ","title":"t","shortDescription":"d","caseId":"c"}`,
		"unknown property":         `{"product":"api-manager","title":"t","shortDescription":"d","caseId":"c","extra":"x"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			mock := &mockGoogleChatAlertSender{}
			h := NewNotificationHandler(mock, "https://portal.example.com")

			r := withUser(httptest.NewRequest(http.MethodPost, "/notifications/google-chat/alerts", strings.NewReader(body)))
			w := httptest.NewRecorder()
			h.PostGoogleChatAlert(w, r)

			assertStatus(t, w, http.StatusBadRequest)
			assertErrorMessage(t, w, ErrMsgBadRequest)
			if mock.called {
				t.Error("upstream should not have been called with a missing required field")
			}
		})
	}
}

func TestPostGoogleChatAlert_SendsAlertFromRequestBody(t *testing.T) {
	mock := &mockGoogleChatAlertSender{}
	h := NewNotificationHandler(mock, "https://portal.example.com/")

	r := withUser(httptest.NewRequest(http.MethodPost, "/notifications/google-chat/alerts", strings.NewReader(validGoogleChatAlertBody)))
	w := httptest.NewRecorder()
	h.PostGoogleChatAlert(w, r)

	assertStatus(t, w, http.StatusOK)
	if !mock.called {
		t.Fatal("expected SendIncidentAlert to be called")
	}
	if mock.gotProduct != "api-manager" {
		t.Errorf("product = %q, want %q", mock.gotProduct, "api-manager")
	}
	if mock.gotTitle != "P1 Incident - CASE-42" {
		t.Errorf("title = %q, want %q", mock.gotTitle, "P1 Incident - CASE-42")
	}
	if mock.gotShortDescription != "Something broke" {
		t.Errorf("shortDescription = %q, want %q", mock.gotShortDescription, "Something broke")
	}
	if mock.gotPortalURL != "https://portal.example.com/operations/incidents/CASE-42" {
		t.Errorf("portalURL = %q, want %q", mock.gotPortalURL, "https://portal.example.com/operations/incidents/CASE-42")
	}
}

func TestPostGoogleChatAlert_RejectsInvalidJSON(t *testing.T) {
	h := NewNotificationHandler(&mockGoogleChatAlertSender{}, "https://portal.example.com")

	r := withUser(httptest.NewRequest(http.MethodPost, "/notifications/google-chat/alerts", strings.NewReader("not json")))
	w := httptest.NewRecorder()
	h.PostGoogleChatAlert(w, r)

	assertStatus(t, w, http.StatusBadRequest)
	assertErrorMessage(t, w, ErrMsgBadRequest)
}

func TestPostGoogleChatAlert_MapsUpstreamFailure(t *testing.T) {
	mock := &mockGoogleChatAlertSender{err: fmt.Errorf("webhook unreachable")}
	h := NewNotificationHandler(mock, "https://portal.example.com")

	r := withUser(httptest.NewRequest(http.MethodPost, "/notifications/google-chat/alerts", strings.NewReader(validGoogleChatAlertBody)))
	w := httptest.NewRecorder()
	h.PostGoogleChatAlert(w, r)

	assertStatus(t, w, http.StatusInternalServerError)
}
