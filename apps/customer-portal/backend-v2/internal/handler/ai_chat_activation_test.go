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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// These tests cover the REST counterpart of websocket_test.go's
// TestHandleWebSocket_ActivatesOpenConversationOnReply /
// TestHandleWebSocket_DoesNotReactivateANonOpenConversation — the same
// maybeActivateConversation wiring, exercised through CreateConversation's
// and SendConversationMessage's own call sites instead of handleMessage.

func TestCreateConversation_ActivatesWhenNewlyCreatedStateIsOpen(t *testing.T) {
	open := "OPEN"
	ai, ent := &chatSpy{}, &convEntity{createdState: &open}
	h := NewAIChatHandler(ai, ent)

	req := authed(httptest.NewRequest(http.MethodPost,
		"/projects/"+testProjectID+"/conversations", strings.NewReader(`{"message":"hi"}`)))
	req.SetPathValue("id", testProjectID)
	h.CreateConversation(httptest.NewRecorder(), req)

	if got := ent.updatedStates; len(got) != 1 || got[0] != createEntityStateActive {
		t.Errorf("UpdateConversation states = %v, want exactly [%q]", got, createEntityStateActive)
	}
}

func TestCreateConversation_DoesNotReactivateANonOpenConversation(t *testing.T) {
	for _, state := range []string{"ACTIVE", "RESOLVED", "CONVERTED", "CLOSED", "ABANDONED"} {
		t.Run(state, func(t *testing.T) {
			s := state
			ai, ent := &chatSpy{}, &convEntity{createdState: &s}
			h := NewAIChatHandler(ai, ent)

			req := authed(httptest.NewRequest(http.MethodPost,
				"/projects/"+testProjectID+"/conversations", strings.NewReader(`{"message":"hi"}`)))
			req.SetPathValue("id", testProjectID)
			h.CreateConversation(httptest.NewRecorder(), req)

			if got := ent.updatedStates; len(got) != 0 {
				t.Errorf("UpdateConversation states = %v, want none", got)
			}
		})
	}
}

// A nil State (plain-Postgres deployments never override the "starts ACTIVE"
// default — see createEntityStateActive's own doc comment) must not panic
// maybeActivateConversation's nil check, and must not activate.
func TestCreateConversation_DoesNotActivateWhenStateIsAbsent(t *testing.T) {
	ai, ent := &chatSpy{}, &convEntity{}
	h := NewAIChatHandler(ai, ent)

	req := authed(httptest.NewRequest(http.MethodPost,
		"/projects/"+testProjectID+"/conversations", strings.NewReader(`{"message":"hi"}`)))
	req.SetPathValue("id", testProjectID)
	h.CreateConversation(httptest.NewRecorder(), req)

	if got := ent.updatedStates; len(got) != 0 {
		t.Errorf("UpdateConversation states = %v, want none", got)
	}
}

func TestSendConversationMessage_ActivatesWhenCurrentStateIsOpen(t *testing.T) {
	open := "OPEN"
	ai, ent := &chatSpy{}, &convEntity{conversationState: &open}
	h := NewAIChatHandler(ai, ent)

	req := authed(httptest.NewRequest(http.MethodPost,
		"/projects/"+testProjectID+"/conversations/"+testConversationID+"/messages",
		strings.NewReader(`{"message":"hi"}`)))
	req.SetPathValue("projectId", testProjectID)
	req.SetPathValue("conversationId", testConversationID)
	h.SendConversationMessage(httptest.NewRecorder(), req)

	if got := ent.updatedStates; len(got) != 1 || got[0] != createEntityStateActive {
		t.Errorf("UpdateConversation states = %v, want exactly [%q]", got, createEntityStateActive)
	}
}

func TestSendConversationMessage_DoesNotReactivateANonOpenConversation(t *testing.T) {
	for _, state := range []string{"ACTIVE", "RESOLVED", "CONVERTED", "CLOSED", "ABANDONED"} {
		t.Run(state, func(t *testing.T) {
			s := state
			ai, ent := &chatSpy{}, &convEntity{conversationState: &s}
			h := NewAIChatHandler(ai, ent)

			req := authed(httptest.NewRequest(http.MethodPost,
				"/projects/"+testProjectID+"/conversations/"+testConversationID+"/messages",
				strings.NewReader(`{"message":"hi"}`)))
			req.SetPathValue("projectId", testProjectID)
			req.SetPathValue("conversationId", testConversationID)
			h.SendConversationMessage(httptest.NewRecorder(), req)

			if got := ent.updatedStates; len(got) != 0 {
				t.Errorf("UpdateConversation states = %v, want none", got)
			}
		})
	}
}

// A failed GetConversation must not fail the request or activate.
func TestSendConversationMessage_ActivationSkippedWhenGetConversationFails(t *testing.T) {
	ai, ent := &chatSpy{}, &convEntity{conversationErr: errors.New("upstream unavailable")}
	h := NewAIChatHandler(ai, ent)

	req := authed(httptest.NewRequest(http.MethodPost,
		"/projects/"+testProjectID+"/conversations/"+testConversationID+"/messages",
		strings.NewReader(`{"message":"hi"}`)))
	req.SetPathValue("projectId", testProjectID)
	req.SetPathValue("conversationId", testConversationID)
	rec := httptest.NewRecorder()
	h.SendConversationMessage(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d despite the failed GetConversation", rec.Code, http.StatusOK)
	}
	if got := ent.updatedStates; len(got) != 0 {
		t.Errorf("UpdateConversation states = %v, want none", got)
	}
}
