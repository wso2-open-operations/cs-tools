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

package apierror

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestNewUpstreamError_ExtractsMessageField(t *testing.T) {
	raw := []byte(`{"code":400,"message":"caseTypes must be valid UUIDs"}`)

	err := NewUpstreamError(http.StatusBadRequest, raw)

	if err.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", err.StatusCode)
	}
	if err.Body != "caseTypes must be valid UUIDs" {
		t.Fatalf("expected extracted message, got %q", err.Body)
	}
}

// TestNewUpstreamError_LeavesBodyEmptyWhenNotJSON guards against ever
// logging or returning to the frontend a raw, unbounded upstream response
// body (e.g. a gateway error page) — Body must stay empty so callers'
// existing "empty Body means no specific message" fallback kicks in,
// instead of surfacing arbitrary upstream content.
func TestNewUpstreamError_LeavesBodyEmptyWhenNotJSON(t *testing.T) {
	raw := []byte("<html>502 Bad Gateway</html>")

	err := NewUpstreamError(http.StatusBadGateway, raw)

	if err.Body != "" {
		t.Fatalf("expected empty Body for a non-JSON response, got %q", err.Body)
	}
}

func TestNewUpstreamError_LeavesBodyEmptyWhenMessageFieldMissing(t *testing.T) {
	raw := []byte(`{"code":500}`)

	err := NewUpstreamError(http.StatusInternalServerError, raw)

	if err.Body != "" {
		t.Fatalf("expected empty Body when message field is absent, got %q", err.Body)
	}
}

// An upstream refusal that names itself (entity-service's errorCode) keeps the
// name beside the message; the name is data a client branches on, so only a
// plain lower-case name is kept.
func TestNewUpstreamError_KeepsAWellFormedErrorCode(t *testing.T) {
	raw := []byte(`{"code":409,"message":"this change request is on hold","errorCode":"change_request_on_hold"}`)

	err := NewUpstreamError(http.StatusConflict, raw)

	if err.StatusCode != http.StatusConflict || err.Body != "this change request is on hold" {
		t.Fatalf("status/body = %d / %q", err.StatusCode, err.Body)
	}
	if err.Code != "change_request_on_hold" {
		t.Fatalf("Code = %q, want change_request_on_hold", err.Code)
	}
}

func TestNewUpstreamError_HasNoCodeWhenTheUpstreamSentNone(t *testing.T) {
	err := NewUpstreamError(http.StatusConflict, []byte(`{"code":409,"message":"stale"}`))
	if err.Code != "" || err.Body != "stale" {
		t.Fatalf("Code/Body = %q / %q, want none / stale", err.Code, err.Body)
	}
}

func TestNewUpstreamError_DropsACodeThatIsNotAPlainName(t *testing.T) {
	for name, code := range map[string]string{
		"upper case":     "Change_Request_On_Hold",
		"a space":        "on hold",
		"markup":         "<script>alert(1)</script>",
		"a leading dash": "-on_hold",
		"a double dash":  "on__hold",
		"empty":          "",
		"too long":       strings.Repeat("a", 65),
	} {
		raw, _ := json.Marshal(map[string]any{"code": 409, "message": "m", "errorCode": code})
		err := NewUpstreamError(http.StatusConflict, raw)
		if err.Code != "" {
			t.Errorf("%s: Code = %q, want it dropped", name, err.Code)
		}
		if err.Body != "m" {
			t.Errorf("%s: Body = %q, want the message kept", name, err.Body)
		}
	}
	// A code that is not even a string is dropped, and the message kept.
	if err := NewUpstreamError(http.StatusConflict, []byte(`{"message":"m","errorCode":7}`)); err.Code != "" || err.Body != "m" {
		t.Errorf("a numeric errorCode: Code/Body = %q / %q, want none / m", err.Code, err.Body)
	}
}

func TestNewUpstreamError_KeepsTheCodeOfAMessagelessBody(t *testing.T) {
	err := NewUpstreamError(http.StatusForbidden, []byte(`{"errorCode":"change_request_not_asked"}`))
	if err.Code != "change_request_not_asked" || err.Body != "" {
		t.Fatalf("Code/Body = %q / %q", err.Code, err.Body)
	}
}
