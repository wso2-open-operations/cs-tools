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
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-ai-integration-service/internal/dashboards"
)

type fakeGenerator struct {
	res   dashboards.Result
	err   error
	got   *dashboards.Request
	calls int
}

func (f *fakeGenerator) Generate(_ context.Context, req dashboards.Request) (dashboards.Result, error) {
	f.calls++
	f.got = &req
	return f.res, f.err
}

func post(t *testing.T, h *DashboardHandler, body string) (int, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.Generate(rec, httptest.NewRequest(http.MethodPost, "/ai/dashboards/generate", strings.NewReader(body)))
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("response %q is not JSON: %v", rec.Body.String(), err)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	return rec.Code, out
}

func TestGenerateReturnsTheCandidates(t *testing.T) {
	gen := &fakeGenerator{res: dashboards.Result{
		Candidates:  []json.RawMessage{json.RawMessage(`{"displayName":"A"}`), json.RawMessage(`{"displayName":"B"}`)},
		InputTokens: 10, OutputTokens: 2,
	}}
	status, out := post(t, &DashboardHandler{generator: gen}, `{"prompt":"  Open cases by severity  ",
		"draft":{"displayName":"Old"},"teams":[{"key":"rigel","name":"Rigel","family":"cre-abt","field":"creTeam"}]}`)
	if status != http.StatusOK {
		t.Fatalf("status = %d, body %v", status, out)
	}
	if c, _ := out["candidates"].([]any); len(c) != 2 || out["declined"] != nil {
		t.Errorf("body = %v, want two candidates and no decline", out)
	}
	if u, _ := out["usage"].(map[string]any); u["inputTokens"] != float64(10) || u["outputTokens"] != float64(2) {
		t.Errorf("usage = %v", out["usage"])
	}
	if gen.got.Prompt != "Open cases by severity" || string(gen.got.Draft) != `{"displayName":"Old"}` ||
		len(gen.got.Teams) != 1 || gen.got.Teams[0].Key != "rigel" || gen.got.Teams[0].Field != "creTeam" {
		t.Errorf("generator got %+v", gen.got)
	}
}

func TestGenerateNullOrAbsentDraftIsAFirstMessage(t *testing.T) {
	for _, body := range []string{`{"prompt":"p"}`, `{"prompt":"p","draft":null,"teams":[]}`} {
		gen := &fakeGenerator{res: dashboards.Result{Candidates: []json.RawMessage{json.RawMessage(`{}`)}}}
		if status, _ := post(t, &DashboardHandler{generator: gen}, body); status != http.StatusOK || gen.got.Draft != nil {
			t.Errorf("%s: status %d, draft %q, want 200 and no draft", body, status, gen.got.Draft)
		}
	}
}

func TestGenerateReturnsTheDecline(t *testing.T) {
	gen := &fakeGenerator{res: dashboards.Result{Declined: "No field for revenue."}}
	status, out := post(t, &DashboardHandler{generator: gen}, `{"prompt":"p"}`)
	if status != http.StatusOK || out["declined"] != "No field for revenue." || out["candidates"] != nil {
		t.Errorf("status %d body %v, want 200 with the decline only", status, out)
	}
}

func TestGenerateRejectsABadRequestWithoutACall(t *testing.T) {
	for name, body := range map[string]string{
		"not JSON":       `prompt`,
		"two objects":    `{"prompt":"p"}{"prompt":"q"}`,
		"unknown field":  `{"prompt":"p","current":{}}`,
		"empty prompt":   `{"prompt":"   "}`,
		"long prompt":    `{"prompt":"` + strings.Repeat("é", promptMaxLength+1) + `"}`,
		"draft an array": `{"prompt":"p","draft":[1]}`,
		"team no key":    `{"prompt":"p","teams":[{"name":"Rigel","field":"creTeam"}]}`,
		"team pipe":      `{"prompt":"p","teams":[{"key":"r","name":"Ri|gel","field":"creTeam"}]}`,
		"team field":     `{"prompt":"p","teams":[{"key":"r","name":"Rigel","field":"accountId"}]}`,
		"team family":    `{"prompt":"p","teams":[{"key":"r","name":"Rigel","family":"sme","field":"creTeam"}]}`,
		"body too big":   `{"prompt":"p","draft":{"x":"` + strings.Repeat("a", maxRequestBodyBytes) + `"}}`,
		"draft too big":  `{"prompt":"p","draft":` + draftOfSize(draftMaxBytes+1) + `}`,
	} {
		gen := &fakeGenerator{}
		status, out := post(t, &DashboardHandler{generator: gen}, body)
		if status != http.StatusBadRequest || out["code"] != CodeInvalidRequest || gen.calls != 0 {
			t.Errorf("%s: status %d body %v calls %d, want 400 invalid_request and no call", name, status, out, gen.calls)
		}
	}
	gen := &fakeGenerator{res: dashboards.Result{Candidates: []json.RawMessage{json.RawMessage(`{}`)}}}
	if status, _ := post(t, &DashboardHandler{generator: gen}, `{"prompt":"`+strings.Repeat("é", promptMaxLength)+`"}`); status != http.StatusOK {
		t.Errorf("a prompt of exactly %d characters: status %d, want 200", promptMaxLength, status)
	}
	if status, _ := post(t, &DashboardHandler{generator: gen}, `{"prompt":"p","draft":`+draftOfSize(draftMaxBytes)+`}`); status != http.StatusOK {
		t.Errorf("a draft of exactly %d bytes: status %d, want 200", draftMaxBytes, status)
	}
}

// draftOfSize is a JSON object of exactly n bytes.
func draftOfSize(n int) string {
	return `{"x":"` + strings.Repeat("a", n-len(`{"x":""}`)) + `"}`
}

func TestGenerateWithoutAKeyIsNotConfigured(t *testing.T) {
	status, out := post(t, NewDashboardHandler(nil), `{"prompt":"p"}`)
	if status != http.StatusServiceUnavailable || out["code"] != CodeNotConfigured {
		t.Errorf("status %d body %v, want 503 not_configured", status, out)
	}
}

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

var _ net.Error = timeoutErr{}

func TestGenerateMapsEveryError(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{dashboards.ErrRefused, http.StatusUnprocessableEntity, CodeRefused},
		{dashboards.ErrTruncated, http.StatusBadGateway, CodeTruncated},
		{dashboards.ErrUnusable, http.StatusBadGateway, CodeUnusable},
		{fmt.Errorf("%w: %w", dashboards.ErrCall, context.DeadlineExceeded), http.StatusGatewayTimeout, CodeTimeout},
		{fmt.Errorf("%w: %w", dashboards.ErrCall, timeoutErr{}), http.StatusGatewayTimeout, CodeTimeout},
		{fmt.Errorf("%w: 401 bad key", dashboards.ErrCall), http.StatusBadGateway, CodeModelCallFailed},
		{fmt.Errorf("something else"), http.StatusInternalServerError, CodeInternal},
	} {
		status, out := post(t, &DashboardHandler{generator: &fakeGenerator{err: tc.err}}, `{"prompt":"p"}`)
		if status != tc.status || out["code"] != tc.code {
			t.Errorf("%v: status %d code %v, want %d %s", tc.err, status, out["code"], tc.status, tc.code)
		}
		if msg, _ := out["message"].(string); strings.Contains(msg, "bad key") || msg == "" {
			t.Errorf("%v: message %q, want a fixed message without upstream detail", tc.err, msg)
		}
	}
}

func TestGenerateAcceptsEveryKnownFamilyAndNone(t *testing.T) {
	for _, family := range []string{"", "cre-abt", "cre", "sre-abt", "sre"} {
		gen := &fakeGenerator{res: dashboards.Result{Candidates: []json.RawMessage{json.RawMessage(`{}`)}}}
		body := `{"prompt":"p","teams":[{"key":"r","name":"Rigel","family":"` + family + `","field":"sreTeam"}]}`
		if status, out := post(t, &DashboardHandler{generator: gen}, body); status != http.StatusOK {
			t.Errorf("family %q: status %d body %v, want 200", family, status, out)
		}
	}
}
