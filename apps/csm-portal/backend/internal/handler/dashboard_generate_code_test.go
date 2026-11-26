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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newGenerateCodeRequest(body string) *http.Request {
	return httptest.NewRequest(http.MethodPost, "/dashboards/generate-code", strings.NewReader(body))
}

func TestGenerateCodeDashboard_Unauthorized(t *testing.T) {
	w := httptest.NewRecorder()
	NewDashboardGenerateCodeHandler(NewSampleCodeDashboardGenerator()).GenerateCodeDashboard(w, newGenerateCodeRequest(`{"prompt":"x"}`))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestGenerateCodeDashboard_PromptChecks(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		message string
	}{
		{"missing", `{}`, errMsgGeneratePromptEmpty},
		{"blank", `{"prompt":"   "}`, errMsgGeneratePromptEmpty},
		{"too long", `{"prompt":"` + strings.Repeat("a", generatePromptMaxLength+1) + `"}`, errMsgGeneratePromptTooLong},
		{"invalid JSON", `{"prompt":`, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			NewDashboardGenerateCodeHandler(NewSampleCodeDashboardGenerator()).GenerateCodeDashboard(w, withUser(newGenerateCodeRequest(tc.body)))
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", w.Code)
			}
			if tc.message == "" {
				return
			}
			body := decodeJSON[map[string]any](t, w)
			if body["message"] != tc.message {
				t.Fatalf("message = %v, want %q", body["message"], tc.message)
			}
		})
	}
}

func TestGenerateCodeDashboard_ReturnsTheSampleSourceAndAReply(t *testing.T) {
	w := httptest.NewRecorder()
	req := withUser(newGenerateCodeRequest(`{"prompt":"My open cases by severity","current":"async function render() {}"}`))
	NewDashboardGenerateCodeHandler(NewSampleCodeDashboardGenerator()).GenerateCodeDashboard(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", w.Code, w.Body.String())
	}
	body := decodeJSON[generateCodeDashboardResponse](t, w)
	if body.Source != sampleCodeDashboardSource {
		t.Fatalf("source is not the sample:\n%s", body.Source)
	}
	if body.Reply != sampleCodeDashboardReply {
		t.Fatalf("reply = %q, want %q", body.Reply, sampleCodeDashboardReply)
	}
	// The sample suggests its own title as the name.
	if body.Name != "My Cases and Escalations" || !strings.Contains(body.Source, `"`+body.Name+`"`) {
		t.Fatalf("name = %q, want the sample's title", body.Name)
	}
	// The sample shows the code catalogue's contract.
	for _, want := range []string{"async function render(portal, root)", "portal.countCases(", "portal.searchCases(", "portal.currentUser("} {
		if !strings.Contains(body.Source, want) {
			t.Fatalf("sample source lacks %q", want)
		}
	}
}

// recordingCodeGenerator returns result / err and records what it was given.
type recordingCodeGenerator struct {
	result  GeneratedCodeDashboard
	err     error
	calls   int
	prompt  string
	current string
}

func (g *recordingCodeGenerator) Generate(_ context.Context, prompt, current string) (GeneratedCodeDashboard, error) {
	g.calls++
	g.prompt, g.current = prompt, current
	return g.result, g.err
}

func TestGenerateCodeDashboard_PassesPromptAndDraftToTheGenerator(t *testing.T) {
	gen := &recordingCodeGenerator{result: GeneratedCodeDashboard{Source: "async function render(portal, root) {}\n", Reply: "Done."}}
	w := httptest.NewRecorder()
	NewDashboardGenerateCodeHandler(gen).GenerateCodeDashboard(w,
		withUser(newGenerateCodeRequest(`{"prompt":"  Add a title  ","current":"async function render() {}"}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", w.Code, w.Body.String())
	}
	if gen.prompt != "Add a title" || gen.current != "async function render() {}" {
		t.Errorf("generator got prompt %q, current %q", gen.prompt, gen.current)
	}
	body := decodeJSON[generateCodeDashboardResponse](t, w)
	if body.Source != gen.result.Source || body.Reply != "Done." {
		t.Errorf("response = %+v", body)
	}
}

func TestGenerateCodeDashboard_SuggestedName(t *testing.T) {
	cases := []struct {
		name, given string
		want        any // nil: left out of the response
	}{
		{"passed through", "Open Cases", "Open Cases"},
		{"trimmed", "  Open Cases \n", "Open Cases"},
		{"none", "", nil},
		{"only spaces", "   ", nil},
		{"at the 255 limit", strings.Repeat("é", 255), strings.Repeat("é", 255)},
		{"over it: no suggestion", strings.Repeat("é", 256), nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gen := &recordingCodeGenerator{result: GeneratedCodeDashboard{Source: "async function render(portal, root) {}\n", Reply: "Done.", Name: tc.given}}
			w := httptest.NewRecorder()
			NewDashboardGenerateCodeHandler(gen).GenerateCodeDashboard(w, withUser(newGenerateCodeRequest(`{"prompt":"cases"}`)))
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body %s", w.Code, w.Body.String())
			}
			body := decodeJSON[map[string]any](t, w)
			if got := body["name"]; got != tc.want {
				t.Errorf("name = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestGenerateCodeDashboard_RefusesAnOversizedDraft(t *testing.T) {
	gen := &recordingCodeGenerator{}
	w := httptest.NewRecorder()
	body := `{"prompt":"x","current":"` + strings.Repeat("é", generatedCodeMaxLength+1) + `"}`
	NewDashboardGenerateCodeHandler(gen).GenerateCodeDashboard(w, withUser(newGenerateCodeRequest(body)))
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", w.Code)
	}
	got := decodeJSON[map[string]any](t, w)
	if got["message"] != errMsgGenerateDraftTooBig || got["errorCode"] != errCodeGenerateDraftTooBig {
		t.Errorf("body = %v, want the draft-too-large message and code", got)
	}
	if gen.calls != 0 {
		t.Errorf("generator calls = %d, want 0", gen.calls)
	}

	// A source exactly at the cap (in characters, not bytes) is sent.
	w = httptest.NewRecorder()
	body = `{"prompt":"x","current":"` + strings.Repeat("é", generatedCodeMaxLength) + `"}`
	NewDashboardGenerateCodeHandler(gen).GenerateCodeDashboard(w, withUser(newGenerateCodeRequest(body)))
	if w.Code != http.StatusOK || gen.calls != 1 {
		t.Errorf("at the cap: status %d, calls %d, want 200 and one call", w.Code, gen.calls)
	}
}

// The code approach counts against the same per-user limiter as
// POST /dashboards/generate, checked after the request and before the model.
func TestGenerateCodeDashboard_RateLimited(t *testing.T) {
	limiter := NewGenerateRateLimiter(1, 100)
	gen := &recordingCodeGenerator{result: GeneratedCodeDashboard{Source: "x", Reply: "y"}}
	h := NewDashboardGenerateCodeHandler(gen).WithRateLimit(limiter)

	w := httptest.NewRecorder()
	h.GenerateCodeDashboard(w, withUser(newGenerateCodeRequest(`{"prompt":"   "}`)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("blank prompt: status = %d, want 400", w.Code)
	}
	w = httptest.NewRecorder()
	h.GenerateCodeDashboard(w, withUser(newGenerateCodeRequest(`{"prompt":"x"}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("first: status = %d, want 200 (a refused request does not count)", w.Code)
	}
	w = httptest.NewRecorder()
	h.GenerateCodeDashboard(w, withUser(newGenerateCodeRequest(`{"prompt":"x"}`)))
	if w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") == "" {
		t.Fatalf("second: status = %d, Retry-After %q, want 429 with Retry-After", w.Code, w.Header().Get("Retry-After"))
	}
	if gen.calls != 1 {
		t.Errorf("generator calls = %d, want 1", gen.calls)
	}

	// The JSON approach shares the limiter: its next call is refused too.
	w = httptest.NewRecorder()
	NewDashboardGenerateHandler(NewSampleDashboardGenerator()).WithRateLimit(limiter).
		GenerateDashboard(w, withUser(newGenerateRequest(`{"prompt":"x"}`)))
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("generate after the code call: status = %d, want 429 (one limit for both)", w.Code)
	}
}

func TestGenerateCodeDashboard_MapsGeneratorErrors(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		message string
	}{
		{"declined", &generateDeclinedError{Reason: "Unassigned cases cannot be filtered."}, http.StatusUnprocessableEntity, "Unassigned cases cannot be filtered."},
		{"refused", errGenerateRefused, http.StatusUnprocessableEntity, errMsgGenerateRefused},
		{"unusable", errGenerateUnusable, http.StatusBadGateway, errMsgGenerateUnusable},
		{"cut off", errGenerateTruncated, http.StatusBadGateway, errMsgGenerateTruncated},
		{"usage limit", errGenerateUsageLimit, http.StatusServiceUnavailable, errMsgGenerateUsageLimit},
		{"call failed", errGenerateCall, http.StatusBadGateway, errMsgGenerateCall},
		{"timed out", errors.Join(errGenerateCall, context.DeadlineExceeded), http.StatusGatewayTimeout, errMsgGenerateTimeout},
		{"unavailable", ErrGenerateUnavailable, http.StatusServiceUnavailable, errMsgGenerateUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			NewDashboardGenerateCodeHandler(&recordingCodeGenerator{err: tc.err}).
				GenerateCodeDashboard(w, withUser(newGenerateCodeRequest(`{"prompt":"x"}`)))
			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d", w.Code, tc.status)
			}
			if got := decodeJSON[map[string]any](t, w)["message"]; got != tc.message {
				t.Errorf("message = %v, want %q", got, tc.message)
			}
		})
	}
}
