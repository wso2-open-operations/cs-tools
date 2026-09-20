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

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

type fakeFeedbackCaseClient struct {
	entityCaseClient
	caseState      string
	getCaseErr     error
	submitFeedback bool
}

func (f *fakeFeedbackCaseClient) GetCase(ctx context.Context, id string) (entity.CaseView, error) {
	if f.getCaseErr != nil {
		return entity.CaseView{}, f.getCaseErr
	}
	return entity.CaseView{ID: id, State: f.caseState}, nil
}

func (f *fakeFeedbackCaseClient) SubmitCaseFeedback(ctx context.Context, caseID string, req entity.SubmitCaseFeedbackRequest) (entity.SubmitCaseFeedbackResponse, error) {
	f.submitFeedback = true
	return entity.SubmitCaseFeedbackResponse{
		Message: "Feedback submitted successfully.",
		Feedback: entity.CaseFeedbackResult{
			ID:           "fb-1",
			AssessmentID: "as-1",
			CaseID:       caseID,
			CreatedBy:    "customer@example.com",
		},
	}, nil
}

func decodeFeedbackMessage(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return body.Message
}

func TestSubmitCaseFeedback_NonClosedCases(t *testing.T) {
	const validBody = `{"emojiId":"11111111-1111-1111-1111-111111111111","chipIds":["22222222-2222-2222-2222-222222222222"]}`
	nonClosedStates := []string{
		"open",
		"Open",
		"work_in_progress",
		"Work In Progress",
		"waiting_on_wso2",
		"awaiting_info",
		"solution_proposed",
		"Solution Proposed",
		"reopened",
	}

	for _, state := range nonClosedStates {
		t.Run("state_"+state, func(t *testing.T) {
			fake := &fakeFeedbackCaseClient{caseState: state}
			h := NewCaseHandler(fake)
			mux := http.NewServeMux()
			mux.HandleFunc("POST /cases/{id}/feedback", h.SubmitCaseFeedback)

			req := authedRequest(http.MethodPost, "/cases/"+testCaseID+"/feedback", validBody)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
			if got := decodeFeedbackMessage(t, rec); got != ErrMsgCaseNotClosedForFeedback {
				t.Fatalf("message = %q, want %q", got, ErrMsgCaseNotClosedForFeedback)
			}
			if fake.submitFeedback {
				t.Fatal("SubmitCaseFeedback reached entity-service on a non-closed case")
			}
		})
	}
}

func TestSubmitCaseFeedback_ClosedCases(t *testing.T) {
	const validBody = `{"emojiId":"11111111-1111-1111-1111-111111111111","chipIds":["22222222-2222-2222-2222-222222222222"]}`
	closedStates := []string{"closed", "Closed", "CLOSED", "3"}

	for _, state := range closedStates {
		t.Run("state_"+state, func(t *testing.T) {
			fake := &fakeFeedbackCaseClient{caseState: state}
			h := NewCaseHandler(fake)
			mux := http.NewServeMux()
			mux.HandleFunc("POST /cases/{id}/feedback", h.SubmitCaseFeedback)

			req := authedRequest(http.MethodPost, "/cases/"+testCaseID+"/feedback", validBody)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusCreated {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusCreated, rec.Body.String())
			}
			if !fake.submitFeedback {
				t.Fatal("SubmitCaseFeedback was not reached for a closed case")
			}
		})
	}
}

func TestSubmitCaseFeedback_LookupFailureFailsOpen(t *testing.T) {
	const validBody = `{"emojiId":"11111111-1111-1111-1111-111111111111","chipIds":["22222222-2222-2222-2222-222222222222"]}`

	fake := &fakeFeedbackCaseClient{
		getCaseErr: errors.New("upstream timeout"),
	}
	h := NewCaseHandler(fake)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /cases/{id}/feedback", h.SubmitCaseFeedback)

	req := authedRequest(http.MethodPost, "/cases/"+testCaseID+"/feedback", validBody)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if !fake.submitFeedback {
		t.Fatal("expected SubmitCaseFeedback to proceed when GetCase fails")
	}
}

func TestSubmitCaseFeedback_Unauthenticated(t *testing.T) {
	h := NewCaseHandler(&fakeFeedbackCaseClient{})
	mux := http.NewServeMux()
	mux.HandleFunc("POST /cases/{id}/feedback", h.SubmitCaseFeedback)

	req := httptest.NewRequest(http.MethodPost, "/cases/"+testCaseID+"/feedback", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestSubmitCaseFeedback_InvalidUUID(t *testing.T) {
	h := NewCaseHandler(&fakeFeedbackCaseClient{})
	mux := http.NewServeMux()
	mux.HandleFunc("POST /cases/{id}/feedback", h.SubmitCaseFeedback)

	req := authedRequest(http.MethodPost, "/cases/not-a-uuid/feedback", `{}`)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if got := decodeFeedbackMessage(t, rec); got != ErrMsgInvalidUUID {
		t.Fatalf("message = %q, want %q", got, ErrMsgInvalidUUID)
	}
}
