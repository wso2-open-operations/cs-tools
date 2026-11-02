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

package dashboards

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-ai-integration-service/internal/claude"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-ai-integration-service/internal/teams"
)

const validGenerated = `{"displayName":"Top Customers by Open Cases","widgets":[{"id":"top","resourceType":"case","shape":"bar"}]}`

type fixedCatalogue struct {
	text string
}

func (c fixedCatalogue) Text() string { return c.text }

// fakeGateway answers every Messages call with one text block and the given
// stop reason (or with status/body when status is set), and records the last
// request body.
type fakeGateway struct {
	text       string
	stopReason string
	status     int
	body       string
	delay      time.Duration
	calls      atomic.Int32
	lastBody   atomic.Value
}

func (f *fakeGateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.calls.Add(1)
	raw, _ := io.ReadAll(r.Body)
	f.lastBody.Store(string(raw))
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-r.Context().Done():
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	if f.status != 0 {
		w.WriteHeader(f.status)
		_, _ = io.WriteString(w, f.body)
		return
	}
	stop := f.stopReason
	if stop == "" {
		stop = "end_turn"
	}
	reply, _ := json.Marshal(map[string]any{
		"id": "msg_1", "type": "message", "role": "assistant", "model": "claude-sonnet-5-5",
		"content":     []any{map[string]any{"type": "text", "text": f.text}},
		"stop_reason": stop, "stop_sequence": nil,
		"usage": map[string]any{"input_tokens": 5000, "output_tokens": 400},
	})
	_, _ = w.Write(reply)
}

// request is what the generator sent, decoded.
func (f *fakeGateway) request(t *testing.T) (system, user string, maxTokens int, model string) {
	t.Helper()
	var req struct {
		Model     string `json:"model"`
		MaxTokens int    `json:"max_tokens"`
		System    []struct {
			Text string `json:"text"`
		} `json:"system"`
		Messages []struct {
			Role    string `json:"role"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"messages"`
	}
	body, _ := f.lastBody.Load().(string)
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("decode request %s: %v", body, err)
	}
	if len(req.System) != 1 || len(req.Messages) != 1 || req.Messages[0].Role != "user" || len(req.Messages[0].Content) != 1 {
		t.Fatalf("request shape = %+v, want one system block and one user message", req)
	}
	return req.System[0].Text, req.Messages[0].Content[0].Text, req.MaxTokens, req.Model
}

func newTestGenerator(t *testing.T, gw *fakeGateway, cat Catalogue, timeout time.Duration) *Generator {
	t.Helper()
	srv := httptest.NewServer(gw)
	t.Cleanup(srv.Close)
	client := claude.NewClient(claude.Config{APIKey: "test-key", BaseURL: srv.URL + "/proxy", Timeout: timeout})
	return NewGenerator(client, "claude-sonnet-5-5", timeout, cat)
}

func TestGenerateSendsTheCatalogueWithTeamsAndThePrompt(t *testing.T) {
	gw := &fakeGateway{text: validGenerated}
	team := []teams.Team{{Key: "rigel", Name: "Rigel", Family: "cre-abt", Field: "creTeam"}}
	res, err := newTestGenerator(t, gw, fixedCatalogue{text: "CATALOGUE\n"}, 5*time.Second).
		Generate(context.Background(), Request{Prompt: "Top 10 customers by open cases.", Teams: team})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(res.Candidates) != 1 || string(res.Candidates[0]) != validGenerated || res.Declined != "" {
		t.Errorf("result = %+v, want the one object", res)
	}
	if res.InputTokens != 5000 || res.OutputTokens != 400 {
		t.Errorf("usage = %d/%d, want 5000/400", res.InputTokens, res.OutputTokens)
	}
	system, user, maxTok, model := gw.request(t)
	if system != teams.WithTeams("CATALOGUE\n", team) || !strings.Contains(system, "| Rigel | `rigel` | CRE ABT | `creTeam` |") {
		t.Errorf("system = %q, want the catalogue with the Teams section", system)
	}
	if user != "Top 10 customers by open cases." || maxTok != maxTokens || model != "claude-sonnet-5-5" {
		t.Errorf("user, max_tokens, model = %q, %d, %q", user, maxTok, model)
	}
}

func TestGenerateWithNoTeamsSendsTheCatalogueAsIs(t *testing.T) {
	gw := &fakeGateway{text: validGenerated}
	if _, err := newTestGenerator(t, gw, fixedCatalogue{text: "CATALOGUE"}, 5*time.Second).
		Generate(context.Background(), Request{Prompt: "p"}); err != nil {
		t.Fatal(err)
	}
	if system, _, _, _ := gw.request(t); system != "CATALOGUE" {
		t.Errorf("system = %q, want the catalogue alone", system)
	}
}

func TestGenerateFollowUpSendsTheDraftBeforeThePrompt(t *testing.T) {
	gw := &fakeGateway{text: validGenerated}
	draft := json.RawMessage(`{"displayName":"Old","widgets":[]}`)
	if _, err := newTestGenerator(t, gw, fixedCatalogue{text: "C"}, 5*time.Second).
		Generate(context.Background(), Request{Prompt: "Add a pie.", Draft: draft}); err != nil {
		t.Fatal(err)
	}
	_, user, _, _ := gw.request(t)
	want := "This is the dashboard I am editing:\n\n```json\n" + string(draft) +
		"\n```\n\nChange it as follows, and reply with the whole updated dashboard:\n\nAdd a pie."
	if user != want {
		t.Errorf("user message =\n%q\nwant\n%q", user, want)
	}
}

func TestGenerateReturnsEveryObjectInOrder(t *testing.T) {
	first := `{"displayName":"First try"}`
	gw := &fakeGateway{text: "```json\n" + first + "\n```\nWait, corrected {as asked}:\n" + validGenerated}
	res, err := newTestGenerator(t, gw, fixedCatalogue{text: "C"}, 5*time.Second).
		Generate(context.Background(), Request{Prompt: "p"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 2 || string(res.Candidates[0]) != first || string(res.Candidates[1]) != validGenerated {
		t.Errorf("candidates = %s, want both objects in order", res.Candidates)
	}
}

func TestGenerateDeclined(t *testing.T) {
	gw := &fakeGateway{text: `{"error": "  The catalogue has no field for revenue. "}`}
	res, err := newTestGenerator(t, gw, fixedCatalogue{text: "C"}, 5*time.Second).
		Generate(context.Background(), Request{Prompt: "p"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Declined != "The catalogue has no field for revenue." || res.Candidates != nil {
		t.Errorf("result = %+v, want the trimmed decline sentence only", res)
	}
}

func TestGenerateErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		gw   *fakeGateway
		want error
	}{
		{"refusal", &fakeGateway{text: "no", stopReason: "refusal"}, ErrRefused},
		{"cut off", &fakeGateway{text: `{"displayName":`, stopReason: "max_tokens"}, ErrTruncated},
		{"no JSON", &fakeGateway{text: "I cannot help with that."}, ErrUnusable},
		{"gateway error", &fakeGateway{status: http.StatusUnauthorized, body: `{"type":"error","error":{"type":"authentication_error","message":"bad key"}}`}, ErrCall},
		{"timeout", &fakeGateway{text: validGenerated, delay: 2 * time.Second}, ErrCall},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := newTestGenerator(t, tc.gw, fixedCatalogue{text: "C"}, 300*time.Millisecond).
				Generate(context.Background(), Request{Prompt: "p"})
			if !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
			if tc.name == "timeout" && !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("timeout err = %v, want a deadline", err)
			}
		})
	}
}

// lockedBuffer is a log destination the gateway's goroutines and the test can
// share.
type lockedBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// captureLog sends the default logger to a buffer, as JSON, for the test.
func captureLog(t *testing.T) *lockedBuffer {
	t.Helper()
	out := &lockedBuffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(out, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return out
}

// usageLines are the token usage lines in out, decoded.
func usageLines(t *testing.T, out string) []map[string]any {
	t.Helper()
	var lines []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
		if l == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(l), &rec); err != nil {
			t.Fatalf("log line %q: %v", l, err)
		}
		if rec["msg"] == "dashboard generate: token usage" {
			lines = append(lines, rec)
		}
	}
	return lines
}

// TestGenerateLogsTokenUsageForEveryOutcome: every model call writes exactly one
// token usage line with the model, the outcome, the tokens and the time the
// call took, a failed call with no tokens, and the line never holds the
// prompt or the reply.
func TestGenerateLogsTokenUsageForEveryOutcome(t *testing.T) {
	for _, tc := range []struct {
		name    string
		gw      *fakeGateway
		outcome string
		tokens  float64
	}{
		{"ok", &fakeGateway{text: validGenerated, delay: 50 * time.Millisecond}, "ok", 5000},
		{"declined", &fakeGateway{text: `{"error":"Not a dashboard request."}`}, "declined", 5000},
		{"refusal", &fakeGateway{text: "no", stopReason: "refusal"}, "refused", 5000},
		{"cut off", &fakeGateway{text: `{"displayName":`, stopReason: "max_tokens"}, "truncated", 5000},
		{"no JSON", &fakeGateway{text: "I cannot help with that."}, "unusable", 5000},
		{"gateway error", &fakeGateway{status: http.StatusUnauthorized, body: `{"type":"error","error":{"type":"authentication_error","message":"bad key"}}`}, "failed", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := captureLog(t)
			_, _ = newTestGenerator(t, tc.gw, fixedCatalogue{text: "C"}, 5*time.Second).
				Generate(context.Background(), Request{Prompt: "secret prompt text"})

			lines := usageLines(t, out.String())
			if len(lines) != 1 {
				t.Fatalf("usage lines = %v, want one", lines)
			}
			l := lines[0]
			if l["model"] != "claude-sonnet-5-5" || l["outcome"] != tc.outcome || l["inputTokens"] != tc.tokens {
				t.Errorf("usage line = %v, want outcome %s and %v input tokens", l, tc.outcome, tc.tokens)
			}
			if tc.tokens > 0 && l["outputTokens"] != float64(400) {
				t.Errorf("output tokens = %v, want 400", l["outputTokens"])
			}
			if ms, ok := l["durationMs"].(float64); !ok || (tc.name == "ok" && ms < 50) {
				t.Errorf("durationMs = %v, want the call's time (at least the gateway's 50ms on ok)", l["durationMs"])
			}
			if strings.Contains(out.String(), "secret prompt") || strings.Contains(out.String(), "Top Customers") {
				t.Errorf("log holds the prompt or the reply: %s", out.String())
			}
		})
	}
}

func TestJSONObjects(t *testing.T) {
	for text, want := range map[string][]string{
		"":                             nil,
		"no braces":                    nil,
		"{not json":                    nil,
		`{"a":{"b":1}}`:                {`{"a":{"b":1}}`},
		"x {y} " + `{"a":1} z {"b":2}`: {`{"a":1}`, `{"b":2}`},
	} {
		got := jsonObjects(text)
		if len(got) != len(want) {
			t.Errorf("jsonObjects(%q) = %s, want %v", text, got, want)
			continue
		}
		for i := range got {
			if string(got[i]) != want[i] {
				t.Errorf("jsonObjects(%q)[%d] = %s, want %s", text, i, got[i], want[i])
			}
		}
	}
}

func TestDeclineReasonIsOnlyTheExactShape(t *testing.T) {
	for raw, want := range map[string]bool{
		`{"error":"x"}`:              true,
		`{"error":"x","widgets":[]}`: false,
		`{"error":1}`:                false,
		`{"displayName":"x"}`:        false,
	} {
		if _, ok := declineReason(json.RawMessage(raw)); ok != want {
			t.Errorf("declineReason(%s) = %v, want %v", raw, ok, want)
		}
	}
}
