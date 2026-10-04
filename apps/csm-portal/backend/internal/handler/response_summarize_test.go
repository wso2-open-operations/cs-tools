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
	"errors"
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

func TestSummarizeErr(t *testing.T) {
	urlErr := &url.Error{Op: "Get", URL: "https://entity.example.com/x?note=jane.doe@example.com", Err: errors.New("connection refused")}
	cases := []struct {
		err  error
		want string
	}{
		{nil, ""},
		{&apierror.Error{StatusCode: 409, Body: "secret body"}, "upstream status 409"},
		{fmt.Errorf("wrapped: %w", &apierror.Error{StatusCode: 502}), "upstream status 502"},
		{fmt.Errorf("entity: GET: %w", context.DeadlineExceeded), "upstream request timed out"},
		{context.Canceled, "upstream request canceled"},
		{urlErr, "upstream request failed"},
	}
	for _, tc := range cases {
		got := summarizeErr(tc.err)
		if got != tc.want {
			t.Errorf("summarizeErr(%v) = %q, want %q", tc.err, got, tc.want)
		}
		if strings.Contains(got, "jane.doe") || strings.Contains(got, "secret") {
			t.Errorf("summarizeErr leaked detail: %q", got)
		}
	}
}

func TestMapUpstreamError_PlainTextBodyIsNeverEchoed(t *testing.T) {
	for _, status := range []int{400, 409, 422} {
		w := httptest.NewRecorder()
		mapUpstreamError(w, &apierror.Error{StatusCode: status, Body: "internal detail from upstream"}, "fallback")
		if w.Code != status {
			t.Errorf("%d: status = %d", status, w.Code)
		}
		if strings.Contains(w.Body.String(), "internal detail") {
			t.Errorf("%d: plain-text body echoed: %s", status, w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	mapUpstreamError(w, &apierror.Error{StatusCode: 422, Body: `{"message":"Invalid state transition."}`}, "fallback")
	if !strings.Contains(w.Body.String(), "Invalid state transition.") {
		t.Errorf("envelope message lost: %s", w.Body.String())
	}
}
