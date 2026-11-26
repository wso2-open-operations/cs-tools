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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-ai-integration-service/internal/dashboards"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-ai-integration-service/internal/playbooks"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-ai-integration-service/internal/teams"
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

// TestGenerateReturnsTheCandidates: the generator gets the trimmed prompt, the
// draft and the teams, and the 200 response carries the candidates and all
// four token counts.
func TestGenerateReturnsTheCandidates(t *testing.T) {
	gen := &fakeGenerator{res: dashboards.Result{
		Candidates:  []json.RawMessage{json.RawMessage(`{"displayName":"A"}`), json.RawMessage(`{"displayName":"B"}`)},
		InputTokens: 10, CacheCreationInputTokens: 30, CacheReadInputTokens: 40, OutputTokens: 2,
	}}
	status, out := post(t, &DashboardHandler{generator: gen}, `{"prompt":"  Open cases by severity  ",
		"draft":{"displayName":"Old"},"teams":[{"key":"rigel","name":"Rigel","family":"cre-abt","field":"creTeam"}]}`)
	if status != http.StatusOK {
		t.Fatalf("status = %d, body %v", status, out)
	}
	if c, _ := out["candidates"].([]any); len(c) != 2 || out["declined"] != nil {
		t.Errorf("body = %v, want two candidates and no decline", out)
	}
	if u, _ := out["usage"].(map[string]any); u["inputTokens"] != float64(10) || u["outputTokens"] != float64(2) ||
		u["cacheCreationInputTokens"] != float64(30) || u["cacheReadInputTokens"] != float64(40) {
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
		"not JSON":        `prompt`,
		"two objects":     `{"prompt":"p"}{"prompt":"q"}`,
		"unknown field":   `{"prompt":"p","current":{}}`,
		"empty prompt":    `{"prompt":"   "}`,
		"long prompt":     `{"prompt":"` + strings.Repeat("é", promptMaxLength+1) + `"}`,
		"draft an array":  `{"prompt":"p","draft":[1]}`,
		"team no key":     `{"prompt":"p","teams":[{"name":"Rigel","field":"creTeam"}]}`,
		"team pipe":       `{"prompt":"p","teams":[{"key":"r","name":"Ri|gel","field":"creTeam"}]}`,
		"team field":      `{"prompt":"p","teams":[{"key":"r","name":"Rigel","field":"accountId"}]}`,
		"team family":     `{"prompt":"p","teams":[{"key":"r","name":"Rigel","family":"sme","field":"creTeam"}]}`,
		"body too big":    `{"prompt":"p","draft":{"x":"` + strings.Repeat("a", maxRequestBodyBytes) + `"}}`,
		"draft too big":   `{"prompt":"p","draft":` + draftOfSize(draftMaxBytes+1) + `}`,
		"answers no code": `{"prompt":"p","playbookAnswers":[{"name":"Outcome","valueType":"SINGLE_SELECT","options":[{"code":"A","label":"A"}]}]}`,
		"answers type":    `{"prompt":"p","playbookAnswers":[{"code":"O","name":"Outcome","valueType":"BOOLEAN","options":[{"code":"A","label":"A"}]}]}`,
		"answers none":    `{"prompt":"p","playbookAnswers":[{"code":"O","name":"Outcome","valueType":"CHECKLIST","options":[]}]}`,
		"answers label":   `{"prompt":"p","playbookAnswers":[{"code":"O","name":"Outcome","valueType":"SINGLE_SELECT","options":[{"code":"A","label":" "}]}]}`,
		"answers pipe":    `{"prompt":"p","playbookAnswers":[{"code":"O","name":"Outcome","valueType":"SINGLE_SELECT","options":[{"code":"A","label":"Yes|No"}]}]}`,
		"answers newline": `{"prompt":"p","playbookAnswers":[{"code":"O","name":"Out\ncome","valueType":"SINGLE_SELECT","options":[{"code":"A","label":"A"}]}]}`,
		"answers too many": `{"prompt":"p","playbookAnswers":[` + strings.TrimSuffix(strings.Repeat(
			`{"code":"O","name":"Outcome","valueType":"SINGLE_SELECT","options":[{"code":"A","label":"A"}]},`, maxPlaybookAnswers+1), ",") + `]}`,
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

// TestGeneratePassesThePlaybookAnswersOn: the generator gets the playbook
// answer fields as sent.
func TestGeneratePassesThePlaybookAnswersOn(t *testing.T) {
	gen := &fakeGenerator{res: dashboards.Result{Candidates: []json.RawMessage{json.RawMessage(`{}`)}}}
	status, out := post(t, &DashboardHandler{generator: gen}, `{"prompt":"p","teams":[],"playbookAnswers":[
		{"code":"ELIGIBILITY_ASSESSMENT_OUTCOME","name":"Eligibility Assessment Outcome","valueType":"SINGLE_SELECT",
		 "options":[{"code":"MONITOR","label":"Monitor"},{"code":"EXCLUDED","label":"Excluded"}]}]}`)
	if status != http.StatusOK {
		t.Fatalf("status = %d, body %v", status, out)
	}
	want := []playbooks.Field{{Code: "ELIGIBILITY_ASSESSMENT_OUTCOME", Name: "Eligibility Assessment Outcome", ValueType: "SINGLE_SELECT",
		Options: []playbooks.Option{{Code: "MONITOR", Label: "Monitor"}, {Code: "EXCLUDED", Label: "Excluded"}}}}
	if !reflect.DeepEqual(gen.got.PlaybookAnswers, want) {
		t.Errorf("generator got %+v, want %+v", gen.got.PlaybookAnswers, want)
	}
}

// draftOfSize is a JSON object of exactly n bytes.
func draftOfSize(n int) string {
	return `{"x":"` + strings.Repeat("a", n-len(`{"x":""}`)) + `"}`
}

func TestGenerateWithoutAKeyIsNotConfigured(t *testing.T) {
	status, out := post(t, NewDashboardHandler(nil, nil), `{"prompt":"p"}`)
	if status != http.StatusServiceUnavailable || out["code"] != CodeNotConfigured {
		t.Errorf("status %d body %v, want 503 not_configured", status, out)
	}
}

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

var _ net.Error = timeoutErr{}

// TestGenerateMapsEveryError: every generator error answers its own status and
// code (the usage limit 429 usage_limit, apart from model_call_failed), with
// a fixed message that never carries upstream detail.
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
		{fmt.Errorf("%w: %w: 429 bad key", dashboards.ErrCall, dashboards.ErrUsageLimit), http.StatusTooManyRequests, CodeUsageLimit},
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

// TestGenerateLogsTheUsageLimitCause: the usage limit's log line carries the
// error, so an operator can tell a request, token or spend limit apart (the
// response's message stays fixed).
func TestGenerateLogsTheUsageLimitCause(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	err := fmt.Errorf("%w: %w: 429 token limit exceeded", dashboards.ErrCall, dashboards.ErrUsageLimit)
	if status, _ := post(t, &DashboardHandler{generator: &fakeGenerator{err: err}}, `{"prompt":"p"}`); status != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", status)
	}
	if line := buf.String(); !strings.Contains(line, "usage limit reached") || !strings.Contains(line, "429 token limit exceeded") {
		t.Errorf("log = %q, want the usage limit line with its cause", line)
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

type fakeCodeGenerator struct {
	res             dashboards.CodeResult
	err             error
	prompt, current string
	teams           []teams.Team
	answers         []playbooks.Field
	calls           int
}

func (f *fakeCodeGenerator) Generate(_ context.Context, prompt, current string, registry []teams.Team, answers []playbooks.Field) (dashboards.CodeResult, error) {
	f.calls++
	f.prompt, f.current, f.teams, f.answers = prompt, current, registry, answers
	return f.res, f.err
}

func postCode(t *testing.T, h *DashboardHandler, body string) (int, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.GenerateCode(rec, httptest.NewRequest(http.MethodPost, "/ai/dashboards/generate-code", strings.NewReader(body)))
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("response %q is not JSON: %v", rec.Body.String(), err)
	}
	return rec.Code, out
}

func TestGenerateCodeReturnsTheSourceAndReply(t *testing.T) {
	gen := &fakeCodeGenerator{res: dashboards.CodeResult{Source: "async function render() {}\n", Reply: "Done.",
		InputTokens: 7, CacheCreationInputTokens: 5, CacheReadInputTokens: 9, OutputTokens: 3}}
	status, out := postCode(t, &DashboardHandler{codeGenerator: gen}, `{"prompt":" Add a list ","current":"function render(){}"}`)
	if status != http.StatusOK || out["source"] != "async function render() {}\n" || out["reply"] != "Done." || out["declined"] != nil {
		t.Fatalf("status %d body %v", status, out)
	}
	want := map[string]any{"inputTokens": float64(7), "cacheCreationInputTokens": float64(5), "cacheReadInputTokens": float64(9), "outputTokens": float64(3)}
	if !reflect.DeepEqual(out["usage"], want) {
		t.Errorf("usage = %v, want %v", out["usage"], want)
	}
	if gen.prompt != "Add a list" || gen.current != "function render(){}" || gen.teams != nil {
		t.Errorf("generator got %q, %q, %v", gen.prompt, gen.current, gen.teams)
	}
}

func TestGenerateCodeReturnsTheSuggestedName(t *testing.T) {
	gen := &fakeCodeGenerator{res: dashboards.CodeResult{Source: "x", Reply: "ok", Name: "Cases by Severity"}}
	status, out := postCode(t, &DashboardHandler{codeGenerator: gen}, `{"prompt":"cases"}`)
	if status != http.StatusOK || out["name"] != "Cases by Severity" {
		t.Fatalf("status %d body %v", status, out)
	}
	gen = &fakeCodeGenerator{res: dashboards.CodeResult{Source: "x", Reply: "ok"}}
	if _, out := postCode(t, &DashboardHandler{codeGenerator: gen}, `{"prompt":"cases"}`); out["name"] != nil {
		t.Errorf("name = %v, want it left out when the model gave none", out["name"])
	}
}

func TestGenerateCodePassesThePlaybookAnswers(t *testing.T) {
	gen := &fakeCodeGenerator{res: dashboards.CodeResult{Source: "x", Reply: "ok"}}
	body := `{"prompt":"p","playbookAnswers":[{"code":"OUTCOME","name":"Outcome","valueType":"SINGLE_SELECT","options":[{"code":"MONITOR","label":"Monitor"}]}]}`
	if status, out := postCode(t, &DashboardHandler{codeGenerator: gen}, body); status != http.StatusOK {
		t.Fatalf("status %d body %v", status, out)
	}
	want := []playbooks.Field{{Code: "OUTCOME", Name: "Outcome", ValueType: "SINGLE_SELECT", Options: []playbooks.Option{{Code: "MONITOR", Label: "Monitor"}}}}
	if !reflect.DeepEqual(gen.answers, want) {
		t.Errorf("answers = %+v, want %+v", gen.answers, want)
	}
}

func TestGenerateCodePassesTheTeams(t *testing.T) {
	gen := &fakeCodeGenerator{res: dashboards.CodeResult{Source: "x", Reply: "ok"}}
	body := `{"prompt":"p","teams":[{"key":"rigel","name":"Rigel","family":"cre-abt","field":"creTeam"},{"key":"apollo","name":"Apollo","family":"sre-abt","field":"sreTeam"}]}`
	if status, out := postCode(t, &DashboardHandler{codeGenerator: gen}, body); status != http.StatusOK {
		t.Fatalf("status %d body %v", status, out)
	}
	want := []teams.Team{{Key: "rigel", Name: "Rigel", Family: "cre-abt", Field: "creTeam"}, {Key: "apollo", Name: "Apollo", Family: "sre-abt", Field: "sreTeam"}}
	if !reflect.DeepEqual(gen.teams, want) {
		t.Errorf("teams = %+v, want %+v", gen.teams, want)
	}
}

func TestGenerateCodeReturnsTheDecline(t *testing.T) {
	gen := &fakeCodeGenerator{res: dashboards.CodeResult{Declined: "Not expressible."}}
	status, out := postCode(t, &DashboardHandler{codeGenerator: gen}, `{"prompt":"p"}`)
	if status != http.StatusOK || out["declined"] != "Not expressible." || out["source"] != nil {
		t.Errorf("status %d body %v, want 200 with the decline only", status, out)
	}
}

func TestGenerateCodeRejectsABadRequestWithoutACall(t *testing.T) {
	for name, body := range map[string]string{
		"not JSON":               `x`,
		"unknown field":          `{"prompt":"p","draft":{}}`,
		"empty prompt":           `{"prompt":"  "}`,
		"long prompt":            `{"prompt":"` + strings.Repeat("a", promptMaxLength+1) + `"}`,
		"long current":           `{"prompt":"p","current":"` + strings.Repeat("é", dashboards.CodeMaxLength+1) + `"}`,
		"too many teams":         `{"prompt":"p","teams":[` + strings.TrimSuffix(strings.Repeat(`{"key":"r","name":"R","field":"creTeam"},`, maxTeams+1), ",") + `]}`,
		"a team with no key":     `{"prompt":"p","teams":[{"key":" ","name":"Rigel","field":"creTeam"}]}`,
		"a team's field":         `{"prompt":"p","teams":[{"key":"r","name":"Rigel","field":"team"}]}`,
		"a team's family":        `{"prompt":"p","teams":[{"key":"r","name":"Rigel","family":"abt","field":"creTeam"}]}`,
		"a pipe in a team name":  `{"prompt":"p","teams":[{"key":"r","name":"Ri|gel","field":"creTeam"}]}`,
		"a playbook answer type": `{"prompt":"p","playbookAnswers":[{"code":"O","name":"Outcome","valueType":"BOOLEAN","options":[{"code":"A","label":"A"}]}]}`,
		"a pipe in an answer":    `{"prompt":"p","playbookAnswers":[{"code":"O","name":"Outcome","valueType":"SINGLE_SELECT","options":[{"code":"A","label":"Yes|No"}]}]}`,
	} {
		gen := &fakeCodeGenerator{}
		status, out := postCode(t, &DashboardHandler{codeGenerator: gen}, body)
		if status != http.StatusBadRequest || out["code"] != CodeInvalidRequest || gen.calls != 0 {
			t.Errorf("%s: status %d body %v calls %d, want 400 and no call", name, status, out, gen.calls)
		}
	}
}

func TestGenerateCodeAcceptsACurrentSourceAtTheCap(t *testing.T) {
	gen := &fakeCodeGenerator{res: dashboards.CodeResult{Source: "x", Reply: "ok"}}
	body := `{"prompt":"p","current":"` + strings.Repeat("é", dashboards.CodeMaxLength) + `"}`
	if status, out := postCode(t, &DashboardHandler{codeGenerator: gen}, body); status != http.StatusOK || gen.calls != 1 {
		t.Errorf("status %d body %v calls %d, want 200 and one call", status, out, gen.calls)
	}
}

func TestGenerateCodeErrorsAndNotConfigured(t *testing.T) {
	if status, out := postCode(t, NewDashboardHandler(nil, nil), `{"prompt":"p"}`); status != http.StatusServiceUnavailable || out["code"] != CodeNotConfigured {
		t.Errorf("no key: status %d body %v", status, out)
	}
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{fmt.Errorf("%w: no code block", dashboards.ErrUnusable), http.StatusBadGateway, CodeUnusable},
		{dashboards.ErrTruncated, http.StatusBadGateway, CodeTruncated},
		{dashboards.ErrRefused, http.StatusUnprocessableEntity, CodeRefused},
		{fmt.Errorf("%w: %w", dashboards.ErrCall, dashboards.ErrUsageLimit), http.StatusTooManyRequests, CodeUsageLimit},
	} {
		status, out := postCode(t, &DashboardHandler{codeGenerator: &fakeCodeGenerator{err: tc.err}}, `{"prompt":"p"}`)
		if status != tc.status || out["code"] != tc.code {
			t.Errorf("%v: status %d body %v, want %d %s", tc.err, status, out, tc.status, tc.code)
		}
	}
}
