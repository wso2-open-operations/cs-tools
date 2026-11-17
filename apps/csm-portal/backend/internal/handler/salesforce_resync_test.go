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
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

const resyncTargetID = "00000000-0000-0000-0000-000000000000"

type mockSalesforceResyncClient struct {
	previewBody, applyBody   []byte
	previewCalls, applyCalls int
	result                   []byte
	err                      error
}

func (m *mockSalesforceResyncClient) PreviewSalesforceResync(_ context.Context, body []byte) ([]byte, error) {
	m.previewCalls++
	m.previewBody = body
	return m.result, m.err
}

func (m *mockSalesforceResyncClient) ApplySalesforceResync(_ context.Context, body []byte) ([]byte, error) {
	m.applyCalls++
	m.applyBody = body
	return m.result, m.err
}

func resyncRequest(path, body string, user bool) *http.Request {
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	if user {
		r = withUser(r)
	}
	return r
}

func TestSalesforceResyncHandler_Success(t *testing.T) {
	previewJSON := `{"target":{"scope":"account","id":"` + resyncTargetID + `"},"capped":false,"components":[]}`
	applyJSON := `{"components":[{"key":"account","status":"applied","succeeded":1,"failed":0,"skippedStale":0}]}`

	t.Run("preview passes the body through and returns the entity JSON", func(t *testing.T) {
		m := &mockSalesforceResyncClient{result: []byte(previewJSON)}
		body := `{"scope":"account","id":"` + resyncTargetID + `"}`
		w := httptest.NewRecorder()
		NewSalesforceResyncHandler(m).Preview(w, resyncRequest("/salesforce/resync/preview", body, true))
		assertStatus(t, w, http.StatusOK)
		if w.Body.String() != previewJSON {
			t.Errorf("body = %s, want %s", w.Body.String(), previewJSON)
		}
		if got := w.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("Cache-Control = %q, want no-store", got)
		}
		if string(m.previewBody) != body || m.applyCalls != 0 {
			t.Errorf("preview body = %s, apply calls = %d", m.previewBody, m.applyCalls)
		}
	})

	t.Run("apply passes the body through and returns the entity JSON", func(t *testing.T) {
		m := &mockSalesforceResyncClient{result: []byte(applyJSON)}
		body := `{"scope":"project","id":"` + resyncTargetID + `","components":["project","invoices"]}`
		w := httptest.NewRecorder()
		NewSalesforceResyncHandler(m).Apply(w, resyncRequest("/salesforce/resync/apply", body, true))
		assertStatus(t, w, http.StatusOK)
		if w.Body.String() != applyJSON {
			t.Errorf("body = %s, want %s", w.Body.String(), applyJSON)
		}
		if got := w.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("Cache-Control = %q, want no-store", got)
		}
		if string(m.applyBody) != body || m.previewCalls != 0 {
			t.Errorf("apply body = %s, preview calls = %d", m.applyBody, m.previewCalls)
		}
	})
}

func TestSalesforceResyncHandler_Unauthorized(t *testing.T) {
	m := &mockSalesforceResyncClient{}
	h := NewSalesforceResyncHandler(m)
	body := `{"scope":"account","id":"` + resyncTargetID + `","components":["account"]}`
	for name, fn := range map[string]http.HandlerFunc{"preview": h.Preview, "apply": h.Apply} {
		w := httptest.NewRecorder()
		fn(w, resyncRequest("/salesforce/resync/"+name, body, false))
		assertStatus(t, w, http.StatusUnauthorized)
		if got := w.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("%s: Cache-Control = %q, want no-store", name, got)
		}
	}
	if m.previewCalls+m.applyCalls != 0 {
		t.Error("entity must not be called without a user")
	}
}

func TestSalesforceResyncHandler_BadRequest(t *testing.T) {
	cases := []struct {
		name, body string
		apply      bool
	}{
		{"preview not JSON", `{`, false},
		{"preview missing scope", `{"id":"` + resyncTargetID + `"}`, false},
		{"preview bad scope", `{"scope":"team","id":"` + resyncTargetID + `"}`, false},
		{"preview missing id", `{"scope":"account"}`, false},
		{"preview blank id", `{"scope":"account","id":"  "}`, false},
		{"apply bad scope", `{"scope":"x","id":"` + resyncTargetID + `","components":["account"]}`, true},
		{"apply missing id", `{"scope":"account","components":["account"]}`, true},
		{"apply missing components", `{"scope":"account","id":"` + resyncTargetID + `"}`, true},
		{"apply empty components", `{"scope":"account","id":"` + resyncTargetID + `","components":[]}`, true},
		{"apply components not an array", `{"scope":"account","id":"` + resyncTargetID + `","components":"account"}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := &mockSalesforceResyncClient{}
			h := NewSalesforceResyncHandler(m)
			w := httptest.NewRecorder()
			if tc.apply {
				h.Apply(w, resyncRequest("/salesforce/resync/apply", tc.body, true))
			} else {
				h.Preview(w, resyncRequest("/salesforce/resync/preview", tc.body, true))
			}
			assertStatus(t, w, http.StatusBadRequest)
			if m.previewCalls+m.applyCalls != 0 {
				t.Error("entity must not be called for an invalid request")
			}
		})
	}

	t.Run("preview with an empty components list is fine", func(t *testing.T) {
		m := &mockSalesforceResyncClient{result: []byte(`{}`)}
		w := httptest.NewRecorder()
		NewSalesforceResyncHandler(m).Preview(w, resyncRequest("/salesforce/resync/preview", `{"scope":"project","id":"`+resyncTargetID+`"}`, true))
		assertStatus(t, w, http.StatusOK)
	})

	t.Run("oversized body is 413", func(t *testing.T) {
		m := &mockSalesforceResyncClient{}
		big := `{"scope":"account","id":"` + strings.Repeat("a", maxRequestBodyBytes+1) + `"}`
		w := httptest.NewRecorder()
		NewSalesforceResyncHandler(m).Preview(w, resyncRequest("/salesforce/resync/preview", big, true))
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
	})
}

func TestSalesforceResyncHandler_UpstreamErrors(t *testing.T) {
	const fallback = "Failed to compare the Salesforce data."
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantMsg    string
	}{
		{"403 keeps the fixed message", &apierror.Error{StatusCode: http.StatusForbidden, Body: `{"message":"internal detail"}`}, http.StatusForbidden, ErrMsgForbidden},
		{"404 keeps the fixed message", &apierror.Error{StatusCode: http.StatusNotFound}, http.StatusNotFound, ErrMsgNotFound},
		{"409 surfaces the reason", &apierror.Error{StatusCode: http.StatusConflict, Body: `{"message":"A resync is already running for this account."}`}, http.StatusConflict, "A resync is already running for this account."},
		{"400 surfaces the validation reason", &apierror.Error{StatusCode: http.StatusBadRequest, Body: `{"message":"component contacts is not part of scope project"}`}, http.StatusBadRequest, "component contacts is not part of scope project"},
		{"502 becomes 503", &apierror.Error{StatusCode: http.StatusBadGateway}, http.StatusServiceUnavailable, fallback},
		{"503 stays 503", &apierror.Error{StatusCode: http.StatusServiceUnavailable}, http.StatusServiceUnavailable, fallback},
		{"500 is not echoed", &apierror.Error{StatusCode: http.StatusInternalServerError, Body: `{"message":"internal detail"}`}, http.StatusInternalServerError, fallback},
		{"transport error", errors.New("connection refused"), http.StatusInternalServerError, fallback},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := &mockSalesforceResyncClient{err: tc.err}
			w := httptest.NewRecorder()
			NewSalesforceResyncHandler(m).Preview(w, resyncRequest("/salesforce/resync/preview", `{"scope":"account","id":"`+resyncTargetID+`"}`, true))
			assertStatus(t, w, tc.wantStatus)
			assertErrorMessage(t, w, tc.wantMsg)
		})
	}

	t.Run("apply maps a 409 the same way", func(t *testing.T) {
		m := &mockSalesforceResyncClient{err: &apierror.Error{StatusCode: http.StatusConflict, Body: `{"message":"busy"}`}}
		w := httptest.NewRecorder()
		NewSalesforceResyncHandler(m).Apply(w, resyncRequest("/salesforce/resync/apply", `{"scope":"account","id":"`+resyncTargetID+`","components":["account"]}`, true))
		assertStatus(t, w, http.StatusConflict)
		assertErrorMessage(t, w, "busy")
	})
}

func TestSalesforceResyncHandler_DoesNotLogRequestBody(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	const marker = "11111111-2222-3333-4444-555555555555"
	m := &mockSalesforceResyncClient{err: &apierror.Error{StatusCode: http.StatusConflict, Body: `{"message":"busy ` + marker + `"}`}}
	w := httptest.NewRecorder()
	NewSalesforceResyncHandler(m).Preview(w, resyncRequest("/salesforce/resync/preview", `{"scope":"account","id":"`+marker+`"}`, true))
	assertStatus(t, w, http.StatusConflict)
	if buf.Len() == 0 {
		t.Fatal("expected the failure to be logged")
	}
	if strings.Contains(buf.String(), marker) {
		t.Errorf("log carries the request body or upstream body: %s", buf.String())
	}
}
