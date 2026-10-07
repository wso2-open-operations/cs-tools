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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/apierror"
)

func TestMapUpstreamError_BadRequestPassesThroughUpstreamMessage(t *testing.T) {
	err := &apierror.Error{StatusCode: http.StatusBadRequest, Body: "caseTypes must be valid UUIDs"}
	rec := httptest.NewRecorder()

	mapUpstreamError(rec, err, "Failed to search cases.")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	var body errorBody
	if decodeErr := json.NewDecoder(rec.Body).Decode(&body); decodeErr != nil {
		t.Fatalf("decode response: %v", decodeErr)
	}
	if body.Message != "caseTypes must be valid UUIDs" {
		t.Fatalf("expected upstream message passed through, got %q", body.Message)
	}
}

func TestMapUpstreamError_BadRequestFallsBackWhenBodyEmpty(t *testing.T) {
	err := &apierror.Error{StatusCode: http.StatusBadRequest, Body: ""}
	rec := httptest.NewRecorder()

	mapUpstreamError(rec, err, "Failed to search cases.")

	var body errorBody
	if decodeErr := json.NewDecoder(rec.Body).Decode(&body); decodeErr != nil {
		t.Fatalf("decode response: %v", decodeErr)
	}
	if body.Message != ErrMsgBadRequest {
		t.Fatalf("expected generic fallback, got %q", body.Message)
	}
}

func TestMapUpstreamError_UnauthorizedUsesFixedMessageNotUpstreamBody(t *testing.T) {
	err := &apierror.Error{StatusCode: http.StatusUnauthorized, Body: "some internal upstream detail"}
	rec := httptest.NewRecorder()

	mapUpstreamError(rec, err, "fallback")

	var body errorBody
	if decodeErr := json.NewDecoder(rec.Body).Decode(&body); decodeErr != nil {
		t.Fatalf("decode response: %v", decodeErr)
	}
	if body.Message != ErrMsgUnauthorized {
		t.Fatalf("expected fixed unauthorized message, got %q", body.Message)
	}
}

func TestSummarizeErr_IncludesUpstreamBody(t *testing.T) {
	err := &apierror.Error{StatusCode: http.StatusBadRequest, Body: "caseTypes must be valid UUIDs"}

	got := summarizeErr(err)

	want := "upstream status 400: caseTypes must be valid UUIDs"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

// TestSummarizeErr_OmitsEmptyBody guards against ever logging a raw,
// unbounded upstream response body: entity.newUpstreamError leaves Body
// empty when the upstream response isn't the documented {"message":...}
// shape, and summarizeErr must not turn that into a misleading
// "status N: " with a dangling empty message.
func TestSummarizeErr_OmitsEmptyBody(t *testing.T) {
	err := &apierror.Error{StatusCode: http.StatusBadGateway, Body: ""}

	got := summarizeErr(err)

	want := "upstream status 502"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

// decodeErrorBody decodes a response as the raw JSON object it is, so a test can
// tell a key that is absent from one that is empty.
func decodeErrorBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not a JSON object: %v (%s)", err, rec.Body.String())
	}
	return body
}

// The machine-readable name entity-service gave a refusal goes on to the client
// with the status it belongs to: a 409 with its message, a 403 with the fixed
// message (never upstream text), a 400 with its message. The status and the
// message are what they were without it.
func TestMapUpstreamError_PassesTheMachineReadableCodeThrough(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		upstream   string
		wantStatus int
		wantMsg    string
	}{
		{"409 keeps the message", http.StatusConflict, "this change request is on hold", http.StatusConflict, "this change request is on hold"},
		{"403 keeps the fixed message", http.StatusForbidden, "only members asked may answer", http.StatusForbidden, ErrMsgForbidden},
		{"400 keeps the message", http.StatusBadRequest, "plannedStartOn is in the past", http.StatusBadRequest, "plannedStartOn is in the past"},
		{"422 keeps the message", http.StatusUnprocessableEntity, "unprocessable", http.StatusUnprocessableEntity, "unprocessable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			mapUpstreamError(rec, &apierror.Error{StatusCode: tc.status, Body: tc.upstream, Code: "change_request_on_hold"}, "fallback")
			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			body := decodeErrorBody(t, rec)
			if body["message"] != tc.wantMsg {
				t.Errorf("message = %v, want %q", body["message"], tc.wantMsg)
			}
			if body["errorCode"] != "change_request_on_hold" {
				t.Errorf("errorCode = %v, want change_request_on_hold", body["errorCode"])
			}
		})
	}
}

// No code from the upstream (an older entity-service, or a refusal without a name)
// leaves the body as it has always been: the key is absent, not empty.
func TestMapUpstreamError_AddsNoCodeKeyWhenTheUpstreamSentNone(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusForbidden, http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity,
		http.StatusUnauthorized, http.StatusServiceUnavailable, http.StatusInternalServerError} {
		rec := httptest.NewRecorder()
		mapUpstreamError(rec, &apierror.Error{StatusCode: status, Body: "upstream text"}, "fallback")
		if _, has := decodeErrorBody(t, rec)["errorCode"]; has {
			t.Errorf("status %d: body %s carries an errorCode", status, rec.Body.String())
		}
	}
}

// Only the 4xx a client branches on carry it: a code never rides on a 401 (the
// session), a 404 (a change request that is not the caller's is the same 404
// whatever the upstream said), or on any failure of the upstream itself.
func TestMapUpstreamError_CarriesNoCodeOnTheOtherStatuses(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusNotFound, http.StatusBadGateway, http.StatusServiceUnavailable,
		http.StatusGatewayTimeout, http.StatusInternalServerError, http.StatusTeapot} {
		rec := httptest.NewRecorder()
		mapUpstreamError(rec, &apierror.Error{StatusCode: status, Body: "x", Code: "change_request_on_hold"}, "fallback")
		if _, has := decodeErrorBody(t, rec)["errorCode"]; has {
			t.Errorf("status %d: body %s carries an errorCode", status, rec.Body.String())
		}
	}
}

func TestWriteError_HasNoCodeKey(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, http.StatusConflict, "stale")
	if got := rec.Body.String(); got != "{\"message\":\"stale\"}\n" {
		t.Errorf("body = %q, want the body this API has always written", got)
	}
}
