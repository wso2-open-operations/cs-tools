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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-ai-integration-service/internal/catalogue"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-ai-integration-service/internal/claude"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-ai-integration-service/internal/playbooks"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-ai-integration-service/internal/teams"
)

// validCodeSource is a dashboard as the model writes it.
const validCodeSource = `async function render(portal, root) {
  const r = await portal.countCases({ groupBy: "severity" });
  root.textContent = String(r.total);
}`

// validCodeReply is a whole reply in the code catalogue's output contract.
const validCodeReply = "Your cases by severity.\n\n```js\n" + validCodeSource + "\n```\n"

func newTestCodeGenerator(t *testing.T, gw *fakeGateway, cat Catalogue, timeout time.Duration) *CodeGenerator {
	t.Helper()
	srv := httptest.NewServer(gw)
	t.Cleanup(srv.Close)
	client := claude.NewClient(claude.Config{APIKey: "test-key", BaseURL: srv.URL + "/proxy", Timeout: timeout})
	return NewCodeGenerator(client, "claude-sonnet-5-5", timeout, cat)
}

func TestCodeGeneratorReadsTheCode(t *testing.T) {
	replies := map[string]string{
		"js fence":         validCodeReply,
		"javascript fence": "Your cases by severity.\n```javascript\n" + validCodeSource + "\n```",
		"bare fence":       "Your cases by severity.\n```\n" + validCodeSource + "\n```",
		"self-corrected":   "Your cases by severity.\n```js\nfunction draw() {}\n```\nThat is wrong; corrected:\n```js\n" + validCodeSource + "\n```",
	}
	for name, reply := range replies {
		t.Run(name, func(t *testing.T) {
			useFixedToday(t)
			gw := &fakeGateway{text: reply}
			got, err := newTestCodeGenerator(t, gw, fixedCatalogue{text: "CODE CATALOGUE"}, 5*time.Second).
				Generate(context.Background(), "My cases by severity.", "", nil, nil)
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			if got.Source != validCodeSource+"\n" || got.Declined != "" {
				t.Fatalf("result = %+v, want the (last) block", got)
			}
			if got.Reply != "Your cases by severity." {
				t.Errorf("reply = %q", got.Reply)
			}
			if got.InputTokens != 5000 || got.CacheCreationInputTokens != 700 || got.CacheReadInputTokens != 9000 || got.OutputTokens != 400 {
				t.Errorf("usage = %+v", got)
			}
			system, user, maxTok, model := gw.request(t)
			if system != "CODE CATALOGUE" || user != "My cases by severity.\n\nToday's date: "+fixedToday || maxTok != codeMaxTokens || model != "claude-sonnet-5-5" {
				t.Errorf("request = system %q, user %q, max_tokens %d, model %q", system, user, maxTok, model)
			}
		})
	}
}

// The teams follow the code catalogue in the cached system prompt, as for the
// dashboard generator; with none, the system prompt is the catalogue alone.
func TestCodeGeneratorSendsTheTeamsInTheSystemPrompt(t *testing.T) {
	useFixedToday(t)
	gw := &fakeGateway{text: validCodeReply}
	team := []teams.Team{{Key: "rigel", Name: "Rigel", Family: "cre-abt", Field: "creTeam"}}
	if _, err := newTestCodeGenerator(t, gw, fixedCatalogue{text: "CODE CATALOGUE\n"}, 5*time.Second).
		Generate(context.Background(), "Rigel's open cases.", "", team, nil); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	system, user, _, _ := gw.request(t)
	if system != teams.WithTeams("CODE CATALOGUE\n", team) || !strings.Contains(system, "| Rigel | `rigel` | CRE ABT | `creTeam` |") {
		t.Errorf("system prompt = %q, want the catalogue then the Teams section", system)
	}
	if strings.Contains(user, "Rigel |") {
		t.Errorf("user message carries the teams: %q", user)
	}
}

// The PLG playbook answers follow the Teams section in the cached system
// prompt, in the dashboard generator's order.
func TestCodeGeneratorSendsThePlaybookAnswersAfterTheTeams(t *testing.T) {
	useFixedToday(t)
	gw := &fakeGateway{text: validCodeReply}
	team := []teams.Team{{Key: "rigel", Name: "Rigel", Family: "cre-abt", Field: "creTeam"}}
	answers := []playbooks.Field{{Code: "OUTCOME", Name: "Outcome", ValueType: "SINGLE_SELECT", Options: []playbooks.Option{{Code: "MONITOR", Label: "Monitor"}}}}
	if _, err := newTestCodeGenerator(t, gw, fixedCatalogue{text: "CODE CATALOGUE\n"}, 5*time.Second).
		Generate(context.Background(), "Sign-ups by outcome.", "", team, answers); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	system, _, _, _ := gw.request(t)
	if want := playbooks.WithPlaybookAnswers(teams.WithTeams("CODE CATALOGUE\n", team), answers); system != want {
		t.Errorf("system prompt =\n%q\nwant\n%q", system, want)
	}
	if !strings.Contains(system, "| Outcome | `OUTCOME` | one answer | `MONITOR` Monitor |") {
		t.Errorf("system prompt has no answers row: %q", system)
	}
}

func TestCodeGeneratorFollowUpSendsTheSource(t *testing.T) {
	useFixedToday(t)
	gw := &fakeGateway{text: validCodeReply}
	if _, err := newTestCodeGenerator(t, gw, fixedCatalogue{text: "CODE CATALOGUE"}, 5*time.Second).
		Generate(context.Background(), "Add a list of escalations.", validCodeSource, nil, nil); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	_, user, _, _ := gw.request(t)
	want := "This is the dashboard code I am editing:\n\n```js\n" + validCodeSource +
		"\n```\n\nChange it as follows, and reply with the whole updated code:\n\nAdd a list of escalations.\n\nToday's date: " + fixedToday
	if user != want {
		t.Errorf("user message =\n%q\nwant\n%q", user, want)
	}
}

// Today's date is in the code user message, never the system prompt, as for
// the dashboard generator: the cache still hits on another day, and the date
// is the UTC day.
func TestCodeGeneratorSendsTodaysDateInTheUserMessageOnly(t *testing.T) {
	gw := &fakeGateway{text: validCodeReply}
	gen := newTestCodeGenerator(t, gw, fixedCatalogue{text: "CODE CATALOGUE"}, 5*time.Second)
	prev := now
	t.Cleanup(func() { now = prev })

	var systems []string
	for _, c := range []struct{ at, date string }{
		{"2026-11-07T01:30:00+05:30", "2026-11-06"}, // 01:30 on 7 November in Colombo is still 6 November in UTC
		{"2026-11-07T09:00:00Z", "2026-11-07"},
	} {
		at, err := time.Parse(time.RFC3339, c.at)
		if err != nil {
			t.Fatal(err)
		}
		now = func() time.Time { return at }
		if _, err := gen.Generate(context.Background(), "Cases created in December.", "", nil, nil); err != nil {
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

func TestCodeGeneratorErrors(t *testing.T) {
	cases := []struct {
		name    string
		gw      *fakeGateway
		timeout time.Duration
		want    error
	}{
		{"prose only", &fakeGateway{text: "Here is your dashboard."}, 0, ErrUnusable},
		{"empty reply", &fakeGateway{text: ""}, 0, ErrUnusable},
		{"no render function", &fakeGateway{text: "Cases.\n```js\nfunction draw(portal, root) {}\n```"}, 0, ErrUnusable},
		{"empty block", &fakeGateway{text: "Cases.\n```js\n\n```"}, 0, ErrUnusable},
		{"code too long", &fakeGateway{text: "Cases.\n```js\n" + validCodeSource + strings.Repeat("\n// x", CodeMaxLength/4) + "\n```"}, 0, ErrUnusable},
		{"refusal", &fakeGateway{text: "", stopReason: "refusal"}, 0, ErrRefused},
		{"cut off", &fakeGateway{text: "Cases.\n```js\nasync function render(", stopReason: "max_tokens"}, 0, ErrTruncated},
		{"gateway 401", &fakeGateway{status: http.StatusUnauthorized, body: `{"error":"Unauthorized","message":"Valid API key required"}`}, 0, ErrCall},
		{"timeout", &fakeGateway{text: validCodeReply, delay: 2 * time.Second}, 100 * time.Millisecond, ErrCall},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			timeout := tc.timeout
			if timeout == 0 {
				timeout = 5 * time.Second
			}
			_, err := newTestCodeGenerator(t, tc.gw, fixedCatalogue{text: "CODE CATALOGUE"}, timeout).
				Generate(context.Background(), "x", "", nil, nil)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestCodeGeneratorDeclined(t *testing.T) {
	gw := &fakeGateway{text: "CANNOT: Unassigned cases cannot be filtered\nin this catalogue."}
	got, err := newTestCodeGenerator(t, gw, fixedCatalogue{text: "CODE CATALOGUE"}, 5*time.Second).
		Generate(context.Background(), "Unassigned cases.", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Declined != "Unassigned cases cannot be filtered in this catalogue." || got.Source != "" {
		t.Fatalf("result = %+v, want the model's sentence on one line and no source", got)
	}
}

// The code catalogue carries the one cache breakpoint, as the dashboard
// catalogue does; the user message (the prompt and the source) carries none.
func TestCodeGeneratorCachesTheSystemPromptOnly(t *testing.T) {
	gw := &fakeGateway{text: validCodeReply}
	if _, err := newTestCodeGenerator(t, gw, fixedCatalogue{text: "CODE CATALOGUE"}, 5*time.Second).
		Generate(context.Background(), "Add a list.", validCodeSource, nil, nil); err != nil {
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
	if len(req.System) != 1 || req.System[0].CacheControl["type"] != "ephemeral" {
		t.Errorf("system cache_control = %+v, want {type: ephemeral}", req.System)
	}
	for _, m := range req.Messages {
		for _, c := range m.Content {
			if c.CacheControl != nil {
				t.Errorf("user message cache_control = %v, want none", c.CacheControl)
			}
		}
	}
}

// Every code call writes the same token usage line as a dashboard call, with
// "approach": "code", and never the prompt, the source or the reply.
func TestCodeGeneratorLogsTokenUsageForEveryOutcome(t *testing.T) {
	for _, tc := range []struct {
		name    string
		gw      *fakeGateway
		outcome string
		tokens  float64
	}{
		{"ok", &fakeGateway{text: validCodeReply}, "ok", 5000},
		{"declined", &fakeGateway{text: "CANNOT: Not a dashboard request."}, "declined", 5000},
		{"refusal", &fakeGateway{text: "no", stopReason: "refusal"}, "refused", 5000},
		{"cut off", &fakeGateway{text: "```js\nasync function render(", stopReason: "max_tokens"}, "truncated", 5000},
		{"no code", &fakeGateway{text: "I cannot help with that."}, "unusable", 5000},
		{"gateway error", &fakeGateway{status: http.StatusUnauthorized, body: `{"type":"error","error":{"type":"authentication_error","message":"bad key"}}`}, "failed", 0},
		{"usage limit", usageLimitGateway(), "usage_limit", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := captureLog(t)
			_, err := newTestCodeGenerator(t, tc.gw, fixedCatalogue{text: "C"}, 5*time.Second).
				Generate(context.Background(), "secret prompt text", "const secretSource = 1;", nil, nil)
			if tc.outcome == "usage_limit" && (!errors.Is(err, ErrUsageLimit) || !errors.Is(err, ErrCall)) {
				t.Errorf("err = %v, want ErrUsageLimit and ErrCall", err)
			}

			lines := usageLines(t, out.String())
			if len(lines) != 1 {
				t.Fatalf("usage lines = %v, want one", lines)
			}
			l := lines[0]
			if l["approach"] != "code" || l["model"] != "claude-sonnet-5-5" || l["outcome"] != tc.outcome || l["inputTokens"] != tc.tokens {
				t.Errorf("usage line = %v, want approach code, outcome %s and %v input tokens", l, tc.outcome, tc.tokens)
			}
			if s := out.String(); strings.Contains(s, "secret") || strings.Contains(s, "countCases") {
				t.Errorf("log holds the prompt, the source or the reply: %s", s)
			}
		})
	}
}

func TestParseCodeReplyReplyText(t *testing.T) {
	long := strings.Repeat("word ", 200)
	cases := map[string]struct {
		text string
		want string
	}{
		"no sentence":    {"```js\n" + validCodeSource + "\n```", codeDefaultReply},
		"two lines":      {"Your cases\nby severity.\n```js\n" + validCodeSource + "\n```", "Your cases by severity."},
		"capped":         {long + "\n```js\n" + validCodeSource + "\n```", ""},
		"prose after it": {"Cases.\n```js\n" + validCodeSource + "\n```\nLet me know.", "Cases."},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := parseCodeReply(tc.text)
			if err != nil {
				t.Fatalf("parseCodeReply: %v", err)
			}
			if name == "capped" {
				if n := len([]rune(got.Reply)); n > codeReplyMaxLength || !strings.HasSuffix(got.Reply, "...") {
					t.Fatalf("reply is %d characters (%q), want at most %d ending in ...", n, got.Reply, codeReplyMaxLength)
				}
				return
			}
			if got.Reply != tc.want {
				t.Errorf("reply = %q, want %q", got.Reply, tc.want)
			}
		})
	}
}

func TestParseCodeReplyName(t *testing.T) {
	block := "```js\n" + validCodeSource + "\n```"
	cases := map[string]struct {
		text      string
		wantName  string
		wantReply string
	}{
		"name line first":           {"Name: Cases by Severity\nYour cases by severity.\n" + block, "Cases by Severity", "Your cases by severity."},
		"after the sentence":        {"Your cases by severity.\nName: Cases by Severity\n" + block, "Cases by Severity", "Your cases by severity."},
		"bold and quoted":           {"**Name:** \"Cases by Severity\"\nYour cases.\n" + block, "Cases by Severity", "Your cases."},
		"bold name, plain colon":    {"**Name**: `Cases by Severity`\nYour cases.\n" + block, "Cases by Severity", "Your cases."},
		"lower case, extra spaces":  {"  name:   Cases   by Severity  \nYour cases.\n" + block, "Cases by Severity", "Your cases."},
		"windows line endings":      {"Name: Cases by Severity\r\nYour cases.\r\n" + block, "Cases by Severity", "Your cases."},
		"only the first name line":  {"Name: First\nName: Second\nYour cases.\n" + block, "First", "Name: Second Your cases."},
		"no name line":              {"Your cases by severity.\n" + block, "", "Your cases by severity."},
		"empty name":                {"Name:\nYour cases.\n" + block, "", "Your cases."},
		"name only, no sentence":    {"Name: Cases by Severity\n" + block, "Cases by Severity", codeDefaultReply},
		"a name word in a sentence": {"The name: of the game.\n" + block, "", "The name: of the game."},
		"name inside the code only": {"Your cases.\n```js\n// Name: Not This\n" + validCodeSource + "\n```", "", "Your cases."},
		"name after the code only":  {"Your cases.\n" + block + "\nName: Not This", "", "Your cases."},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := parseCodeReply(tc.text)
			if err != nil {
				t.Fatalf("parseCodeReply: %v", err)
			}
			if got.Name != tc.wantName || got.Reply != tc.wantReply {
				t.Errorf("name %q, reply %q; want %q, %q", got.Name, got.Reply, tc.wantName, tc.wantReply)
			}
		})
	}
}

func TestParseCodeReplyCapsTheName(t *testing.T) {
	got, err := parseCodeReply("Name: " + strings.Repeat("Ä", 300) + "\nCases.\n```js\n" + validCodeSource + "\n```")
	if err != nil {
		t.Fatalf("parseCodeReply: %v", err)
	}
	if n := len([]rune(got.Name)); n != 255 || !strings.HasSuffix(got.Name, "...") {
		t.Fatalf("name is %d characters, want 255 ending in ...", n)
	}
	// 255 is the longest name a saved dashboard can have; a name that long is kept whole.
	at, err := parseCodeReply("Name: " + strings.Repeat("Ä", 255) + "\nCases.\n```js\n" + validCodeSource + "\n```")
	if err != nil {
		t.Fatalf("parseCodeReply: %v", err)
	}
	if at.Name != strings.Repeat("Ä", 255) {
		t.Errorf("a name at the cap was changed: %d characters", len([]rune(at.Name)))
	}
}

func TestParseCodeReplyDeclineDropsAName(t *testing.T) {
	got, err := parseCodeReply("Name: Revenue\nCANNOT: revenue is not available.")
	if err != nil {
		t.Fatalf("parseCodeReply: %v", err)
	}
	if got.Declined != "revenue is not available." || got.Name != "" || got.Source != "" {
		t.Errorf("result = %+v, want the decline only", got)
	}
}

func TestBuiltInCodeCatalogueHoldsTheContract(t *testing.T) {
	text := catalogue.CodeEmbedded()
	for _, want := range []string{"async function render(portal, root)", "portal.currentUser()", "portal.countCases(", "portal.searchCases(", codeDeclinePrefix} {
		if !strings.Contains(text, want) {
			t.Errorf("code catalogue lacks %q", want)
		}
	}
	// The catalogue's own example must pass the reply checks it describes.
	start := strings.Index(text, "## 9. A complete example")
	if start < 0 {
		t.Fatal("code catalogue has no example section")
	}
	example, err := parseCodeReply(text[start:])
	if err != nil {
		t.Fatalf("the catalogue's example is not a usable reply: %v", err)
	}
	if example.Name != "My Cases and Escalations" {
		t.Errorf("the catalogue's example names %q, want its header title", example.Name)
	}
	if !strings.Contains(example.Source, `header(root, "My Cases and Escalations"`) {
		t.Error("the example's Name line is no longer its header title")
	}
}
