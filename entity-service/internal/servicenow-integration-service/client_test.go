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

package integrationservice

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

func TestStripInternalErrorTag(t *testing.T) {
	tests := []struct {
		name          string
		in            string
		wantClientMsg string
		wantTag       string
	}{
		{
			name:          "servicenow tag stripped",
			in:            "[SERVICENOW_ERROR] State transition rejected",
			wantClientMsg: "State transition rejected",
			wantTag:       "[SERVICENOW_ERROR]",
		},
		{
			name:          "other bracket tag stripped",
			in:            "[ENTITY_SERVICE_ERROR] duplicate key",
			wantClientMsg: "duplicate key",
			wantTag:       "[ENTITY_SERVICE_ERROR]",
		},
		{
			name:          "no tag left untouched",
			in:            "State transition rejected",
			wantClientMsg: "State transition rejected",
			wantTag:       "",
		},
		{
			name:          "lowercase bracket content is not treated as a tag",
			in:            "[case 123] rejected",
			wantClientMsg: "[case 123] rejected",
			wantTag:       "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotMsg, gotTag := stripInternalErrorTag(tc.in)
			if gotMsg != tc.wantClientMsg {
				t.Errorf("clientMsg = %q, want %q", gotMsg, tc.wantClientMsg)
			}
			if gotTag != tc.wantTag {
				t.Errorf("tag = %q, want %q", gotTag, tc.wantTag)
			}
		})
	}
}

// TestClient_TaggedDownstreamMessage_NotLeakedToClient reproduces the
// production observation: the downstream service returns a 409 body whose
// "message" field carries a "[SERVICENOW_ERROR]" prefix. The error returned
// to the caller (and ultimately serialized into the HTTP response the FE
// sees) must have the tag stripped while keeping the rest of the message.
func TestClient_TaggedDownstreamMessage_NotLeakedToClient(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "test-token", "expires_in": 3600})
	})
	mux.HandleFunc("/cases/abc", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": "[SERVICENOW_ERROR] State transition rejected",
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := New(srv.URL, ClientCredentialsConfig{
		TokenURL:     srv.URL + "/oauth2/token",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	}, 45*time.Second)

	_, err := client.Patch(context.Background(), "/cases/abc", "test-id-token", map[string]any{"workState": "ongoing"})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	ce, ok := err.(*apierror.ConflictError)
	if !ok {
		t.Fatalf("expected *apierror.ConflictError, got %T: %v", err, err)
	}

	const wantMsg = "State transition rejected"
	if ce.Msg != wantMsg {
		t.Errorf("ConflictError.Msg = %q, want %q (internal tag must not reach the client-facing message)", ce.Msg, wantMsg)
	}
}

// TestExtractDownstreamMessage_TagOnly_FallsBackToDefault covers a downstream
// payload whose "message" is nothing but the internal tag (no text after it).
// Stripping the tag would otherwise leave an empty client-facing message.
func TestExtractDownstreamMessage_TagOnly_FallsBackToDefault(t *testing.T) {
	const defaultMsg = "request conflicts with current state of the resource"
	body := []byte(`{"message":"[SERVICENOW_ERROR]"}`)

	got := extractDownstreamMessage(body, defaultMsg)
	if got != defaultMsg {
		t.Errorf("extractDownstreamMessage() = %q, want fallback %q", got, defaultMsg)
	}
}
