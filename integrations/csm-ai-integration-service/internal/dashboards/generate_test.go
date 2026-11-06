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

// ServeHTTP records the request body, waits out delay (unless the caller
// gives up first), then answers with status/body when status is set, or with
// one text block and fixed usage: input 5000, cache write 700, cache read 9000,
// output 400.
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
		"usage": map[string]any{"input_tokens": 5000, "output_tokens": 400,
			"cache_creation_input_tokens": 700, "cache_read_input_tokens": 9000},
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

// fixedToday is the date every test's user message carries (see useFixedToday).
const fixedToday = "2026-11-06"

// useFixedToday sets the clock userMessage reads to fixedToday for one test.
func useFixedToday(t *testing.T) {
	t.Helper()
	day, err := time.Parse("2006-01-02", fixedToday)
	if err != nil {
		t.Fatal(err)
	}
	prev := now
	now = func() time.Time { return day.Add(15 * time.Hour) }
	t.Cleanup(func() { now = prev })
}

// TestGenerateSendsTheCatalogueWithTeamsAndThePrompt: the system prompt is the
// catalogue with the Teams section, the user message is the prompt, and the
// result carries the candidate and all four token counts.
func TestGenerateSendsTheCatalogueWithTeamsAndThePrompt(t *testing.T) {
	useFixedToday(t)
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
	if res.InputTokens != 5000 || res.OutputTokens != 400 || res.CacheCreationInputTokens != 700 || res.CacheReadInputTokens != 9000 {
		t.Errorf("usage = %+v, want input 5000, cache write 700, cache read 9000, output 400", res)
	}
	system, user, maxTok, model := gw.request(t)
	if system != teams.WithTeams("CATALOGUE\n", team) || !strings.Contains(system, "| Rigel | `rigel` | CRE ABT | `creTeam` |") {
		t.Errorf("system = %q, want the catalogue with the Teams section", system)
	}
	if user != "Top 10 customers by open cases.\n\nToday's date: "+fixedToday || maxTok != maxTokens || model != "claude-sonnet-5-5" {
		t.Errorf("user, max_tokens, model = %q, %d, %q", user, maxTok, model)
	}
}

// The system prompt (catalogue and teams) is the same on every call, so it
// carries the one cache breakpoint; the user message, which changes per
// request, carries none.
func TestGenerateCachesTheSystemPromptOnly(t *testing.T) {
	gw := &fakeGateway{text: validGenerated}
	if _, err := newTestGenerator(t, gw, fixedCatalogue{text: "CATALOGUE\n"}, 5*time.Second).
		Generate(context.Background(), Request{Prompt: "My open cases.", Draft: json.RawMessage(`{"displayName":"x"}`)}); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	var req struct {
		System []struct {
			CacheControl map[string]any `json:"cache_control"`
		} `json:"system"`
		Messages []struct {
			Content []struct {
				CacheControl map[string]any `json:"cache_control"`
			} `json:"content"`
		} `json:"messages"`
	}
	body, _ := gw.lastBody.Load().(string)
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	if len(req.System) != 1 || len(req.System[0].CacheControl) != 1 || req.System[0].CacheControl["type"] != "ephemeral" {
		t.Errorf("system cache_control = %+v, want exactly {type: ephemeral} (the default 5-minute lifetime)", req.System)
	}
	for _, m := range req.Messages {
		for _, c := range m.Content {
			if c.CacheControl != nil {
				t.Errorf("user message cache_control = %v, want none", c.CacheControl)
			}
		}
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
	useFixedToday(t)
	gw := &fakeGateway{text: validGenerated}
	draft := json.RawMessage(`{"displayName":"Old","widgets":[]}`)
	if _, err := newTestGenerator(t, gw, fixedCatalogue{text: "C"}, 5*time.Second).
		Generate(context.Background(), Request{Prompt: "Add a pie.", Draft: draft}); err != nil {
		t.Fatal(err)
	}
	_, user, _, _ := gw.request(t)
	want := "This is the dashboard I am editing:\n\n```json\n" + string(draft) +
		"\n```\n\nChange it as follows, and reply with the whole updated dashboard:\n\nAdd a pie." +
		"\n\nToday's date: " + fixedToday
	if user != want {
		t.Errorf("user message =\n%q\nwant\n%q", user, want)
	}
}

// Today's date is in the user message, never the system prompt: the system
// prompt is the same on two different days, so the cache still hits, and the
// date the model sees is the UTC day, also late in a Colombo evening.
func TestGenerateSendsTodaysDateInTheUserMessageOnly(t *testing.T) {
	gw := &fakeGateway{text: validGenerated}
	gen := newTestGenerator(t, gw, fixedCatalogue{text: "CATALOGUE\n"}, 5*time.Second)
	prev := now
	t.Cleanup(func() { now = prev })

	var systems []string
	for _, c := range []struct{ at, date string }{
		{"2026-11-06T23:30:00+05:30", "2026-11-06"}, // 23:30 in Colombo is still 6 November in UTC
		{"2026-11-07T09:00:00Z", "2026-11-07"},
	} {
		at, err := time.Parse(time.RFC3339, c.at)
		if err != nil {
			t.Fatal(err)
		}
		now = func() time.Time { return at }
		if _, err := gen.Generate(context.Background(), Request{Prompt: "Cases created in December."}); err != nil {
			t.Fatalf("Generate: %v", err)
		}
		system, user, _, _ := gw.request(t)
		systems = append(systems, system)
		if want := "Cases created in December.\n\nToday's date: " + c.date; user != want {
			t.Errorf("at %s, user message = %q, want %q", c.at, user, want)
		}
		if strings.Contains(system, "Today's date") {
			t.Errorf("at %s, the system prompt carries the date: %q", c.at, system)
		}
	}
	if systems[0] != systems[1] {
		t.Errorf("the system prompt changed between days:\n%q\n%q", systems[0], systems[1])
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
			// The cache counts, 0 when the call failed.
			wantWrite, wantRead := float64(700), float64(9000)
			if tc.tokens == 0 {
				wantWrite, wantRead = 0, 0
			}
			if l["cacheCreationInputTokens"] != wantWrite || l["cacheReadInputTokens"] != wantRead {
				t.Errorf("cache tokens = %v / %v, want %v / %v", l["cacheCreationInputTokens"], l["cacheReadInputTokens"], wantWrite, wantRead)
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
