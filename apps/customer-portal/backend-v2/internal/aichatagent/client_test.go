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

package aichatagent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func chatServer(t *testing.T, status int, body string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/chat" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return &Client{http: srv.Client(), baseURL: srv.URL}
}

func TestCreateChat_RemovesReasoningFromTheAnswer(t *testing.T) {
	client := chatServer(t, http.StatusOK,
		`{"message":"<thinking>internal notes</thinking>\n\nWhich gateway?","sessionId":"s1","conversationId":"c1","resolved":false}`)

	got, err := client.CreateChat(context.Background(), ChatPayload{})
	if err != nil {
		t.Fatalf("CreateChat: %v", err)
	}
	if got.Message != "Which gateway?" {
		t.Errorf("Message = %q, want the answer alone", got.Message)
	}
	if got.SessionID != "s1" || got.ConversationID != "c1" || got.Resolved == nil || *got.Resolved {
		t.Errorf("other fields changed: %+v", got)
	}
}

func TestCreateChat_LeavesAnOrdinaryAnswerAlone(t *testing.T) {
	client := chatServer(t, http.StatusOK, `{"message":"Which <b>gateway</b>?","sessionId":"s1","conversationId":"c1"}`)

	got, err := client.CreateChat(context.Background(), ChatPayload{})
	if err != nil {
		t.Fatalf("CreateChat: %v", err)
	}
	if got.Message != "Which <b>gateway</b>?" {
		t.Errorf("Message = %q", got.Message)
	}
}

func TestCreateChat_StillReportsAnUpstreamError(t *testing.T) {
	client := chatServer(t, http.StatusBadGateway, `{"message":"agent down"}`)

	if _, err := client.CreateChat(context.Background(), ChatPayload{}); err == nil {
		t.Fatal("want an error for a 502")
	}
}
