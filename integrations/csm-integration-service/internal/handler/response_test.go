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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-integration-service/internal/apierror"
)

func TestMapUpstreamError_RetryAfterPassThrough(t *testing.T) {
	cases := []struct {
		name, retryAfter, wantHeader string
	}{
		{"delta seconds", "30", "30"},
		{"delta seconds with spaces", " 120 ", "120"},
		{"http date", "Wed, 21 Oct 2026 07:28:00 GMT", "Wed, 21 Oct 2026 07:28:00 GMT"},
		{"absent", "", ""},
		{"negative", "-5", ""},
		{"garbage", "soon", ""},
		{"too many digits", "123456789", ""},
		{"header injection", "30\r\nX-Evil: 1", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			mapUpstreamError(w, &apierror.Error{StatusCode: http.StatusTooManyRequests, RetryAfter: tc.retryAfter}, "fallback")
			assertStatus(t, w, http.StatusTooManyRequests)
			assertErrorMessage(t, w, ErrMsgRateLimited)
			if got := w.Header().Get("Retry-After"); got != tc.wantHeader {
				t.Errorf("Retry-After = %q, want %q", got, tc.wantHeader)
			}
		})
	}
}

func TestMapUpstreamError_TimeoutsBecome504(t *testing.T) {
	for _, code := range []int{http.StatusRequestTimeout, http.StatusGatewayTimeout} {
		w := httptest.NewRecorder()
		mapUpstreamError(w, &apierror.Error{StatusCode: code}, "fallback")
		assertStatus(t, w, http.StatusGatewayTimeout)
		assertErrorMessage(t, w, "fallback")
	}
}

func TestSafeUpstreamMessage_Fallback(t *testing.T) {
	if got := safeUpstreamMessage(`not json`, "fb"); got != "fb" {
		t.Errorf("got %q, want fallback", got)
	}
	if got := safeUpstreamMessage(`{"message":"fine"}`, "fb"); got != "fine" {
		t.Errorf("got %q, want upstream message", got)
	}
}

func TestUpstreamBadRequestMessage(t *testing.T) {
	cases := []struct {
		name, body, want string
	}{
		{"entity-service message passes through",
			`{"code":400,"message":"excludeClosureStates: \"suspended\" is not a valid closure state; use one of Open, Restricted, Suspended"}`,
			`excludeClosureStates: "suspended" is not a valid closure state; use one of Open, Restricted, Suspended`},
		{"empty body", ``, ErrMsgBadRequest},
		{"not JSON", `Bad Request`, ErrMsgBadRequest},
		{"truncated JSON", `{"code":400,"message":"sortBy: `, ErrMsgBadRequest},
		{"blank message", `{"code":400,"message":"  "}`, ErrMsgBadRequest},
		{"foreign-key detail", `{"code":400,"message":"one or more referenced IDs do not exist: Key (project_id)=(x) is not present in table \"project\"."}`, ErrMsgBadRequest},
		{"column type detail", `{"code":400,"message":"a field value is too long: value too long for type character varying(512)"}`, ErrMsgBadRequest},
		{"multi-line", `{"code":400,"message":"bad\nstack"}`, ErrMsgBadRequest},
		{"oversized", `{"code":400,"message":"` + strings.Repeat("x", maxPassThroughMsgLen+1) + `"}`, ErrMsgBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := safeUpstreamMessage(tc.body, ErrMsgBadRequest); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSearchProjects_PassesEntityServiceValidationMessage(t *testing.T) {
	const msg = `arrTodayGte is not supported on this data source yet: account ARR is not stored; remove the filter`
	client := &mockEntityProjectClient{
		searchProjectsFn: func(_ context.Context, _ []byte) ([]byte, error) {
			return nil, &apierror.Error{StatusCode: http.StatusBadRequest, Body: `{"code":400,"message":"` + msg + `"}`}
		},
	}
	w := httptest.NewRecorder()
	NewProjectHandler(client).SearchProjects(w, httptest.NewRequest(http.MethodPost, "/projects/search", strings.NewReader(`{"arrTodayGte":"1"}`)))
	assertStatus(t, w, http.StatusBadRequest)
	assertErrorMessage(t, w, msg)
}

func TestUpdateProject_PassesEntityServiceValidationMessage(t *testing.T) {
	const msg = `complianceViolationClosureState: "Restricted" is not a valid closure state; use one of Open, Suspended`
	client := &mockEntityProjectClient{
		updateProjectFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
			return nil, &apierror.Error{StatusCode: http.StatusBadRequest, Body: `{"code":400,"message":"` + strings.ReplaceAll(msg, `"`, `\"`) + `"}`}
		},
	}
	r := httptest.NewRequest(http.MethodPatch, "/projects/e3e87599-1bc7-6650-182c-0dc5604bcb68", strings.NewReader(`{"complianceViolationClosureState":"Restricted"}`))
	r.SetPathValue("id", "e3e87599-1bc7-6650-182c-0dc5604bcb68")
	w := httptest.NewRecorder()
	NewProjectHandler(client).UpdateProject(w, r)
	assertStatus(t, w, http.StatusBadRequest)
	assertErrorMessage(t, w, msg)
}
