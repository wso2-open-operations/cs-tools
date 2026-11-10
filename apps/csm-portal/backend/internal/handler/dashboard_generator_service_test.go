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
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/csmai"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/dashboard"
)

// validGenerated is a dashboard as the model writes it (file format, no id).
const validGenerated = `{"displayName":"Top Customers by Open Cases","type":"cs","isDefault":false,"targetTeam":"","isTeamBased":false,
"widgets":[{"id":"top","displayName":"Top 10","resourceType":"case","shape":"bar","gridWidth":12,
"query":{"filters":[{"field":"state","op":"notIn","values":["closed"]}]},"groupBy":{"field":"account","maxGroups":10}}]}`

type fakeAIClient struct {
	res    csmai.DashboardResult
	err    error
	gotReq csmai.DashboardRequest
}

func (f *fakeAIClient) GenerateDashboard(_ context.Context, req csmai.DashboardRequest) (csmai.DashboardResult, error) {
	f.gotReq = req
	return f.res, f.err
}

func TestServiceGenerator_KeepsTheLastValidCandidate(t *testing.T) {
	ai := &fakeAIClient{res: csmai.DashboardResult{Candidates: []json.RawMessage{
		json.RawMessage(validGenerated), json.RawMessage(`{"displayName":"broken","widgets":[{"id":"x","linkResourceType":"account"}]}`),
	}}}
	got, err := NewServiceDashboardGenerator(ai, nil).Generate(context.Background(), "p", nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if got.Dashboard.DisplayName != "Top Customers by Open Cases" || ai.gotReq.Prompt != "p" || ai.gotReq.Draft != nil {
		t.Errorf("got %+v, request %+v", got.Dashboard.DisplayName, ai.gotReq)
	}
}

// The "model replied" line carries the call's whole input: the uncached part
// and the prompt cache's write and read counts.
func TestServiceGenerator_LogsTheCacheTokenCounts(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	ai := &fakeAIClient{res: csmai.DashboardResult{
		Candidates: []json.RawMessage{json.RawMessage(validGenerated)},
		Usage:      csmai.Usage{InputTokens: 120, CacheCreationInputTokens: 30, CacheReadInputTokens: 15000, OutputTokens: 400},
	}}
	if _, err := NewServiceDashboardGenerator(ai, nil).Generate(context.Background(), "p", nil); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	line := buf.String()
	for _, want := range []string{"inputTokens=120", "cacheCreationInputTokens=30", "cacheReadInputTokens=15000", "outputTokens=400"} {
		if !strings.Contains(line, want) {
			t.Errorf("log %q has no %s", line, want)
		}
	}
}

func TestServiceGenerator_DeclinedAndNoValidCandidate(t *testing.T) {
	var declined *generateDeclinedError
	_, err := NewServiceDashboardGenerator(&fakeAIClient{res: csmai.DashboardResult{Declined: "No revenue field."}}, nil).
		Generate(context.Background(), "p", nil)
	if !errors.As(err, &declined) || declined.Reason != "No revenue field." {
		t.Errorf("err = %v, want the decline", err)
	}
	_, err = NewServiceDashboardGenerator(&fakeAIClient{res: csmai.DashboardResult{Candidates: []json.RawMessage{json.RawMessage(`{}`)}}}, nil).
		Generate(context.Background(), "p", nil)
	if !errors.Is(err, errGenerateUnusable) {
		t.Errorf("err = %v, want errGenerateUnusable", err)
	}
}

// TestServiceGenerateError_MapsEveryCode checks that every AI service error
// code maps to its generate error (only a timeout counts as one), and that a
// transport failure is a failed call.
func TestServiceGenerateError_MapsEveryCode(t *testing.T) {
	for code, want := range map[string]error{
		csmai.CodeNotConfigured:   ErrGenerateUnavailable,
		csmai.CodeRefused:         errGenerateRefused,
		csmai.CodeTruncated:       errGenerateTruncated,
		csmai.CodeUnusable:        errGenerateUnusable,
		csmai.CodeTimeout:         errGenerateCall,
		csmai.CodeModelCallFailed: errGenerateCall,
		csmai.CodeUsageLimit:      errGenerateUsageLimit,
		csmai.CodeInvalidRequest:  errGenerateCall,
	} {
		err := serviceGenerateError(&csmai.Error{StatusCode: 500, Code: code})
		if !errors.Is(err, want) {
			t.Errorf("%s: err = %v, want %v", code, err, want)
		}
		if (code == csmai.CodeTimeout) != isTimeout(err) {
			t.Errorf("%s: isTimeout = %v", code, isTimeout(err))
		}
	}
	if err := serviceGenerateError(csmai.ErrCall); !errors.Is(err, errGenerateCall) {
		t.Errorf("transport: err = %v", err)
	}
}

func TestServiceGenerator_SendsOnlyTheTeamsADashboardCanFilterBy(t *testing.T) {
	ai := &fakeAIClient{res: csmai.DashboardResult{Candidates: []json.RawMessage{json.RawMessage(validGenerated)}}}
	if _, err := NewServiceDashboardGenerator(ai, testTeamResults(t)).Generate(context.Background(), "p", nil); err != nil {
		t.Fatal(err)
	}
	if len(ai.gotReq.Teams) == 0 {
		t.Fatal("no teams sent")
	}
	want := map[string]bool{}
	for _, team := range newDashboardTeams(testTeamResults(t)).list {
		if team.GroupID != "" {
			want[team.Key] = true
		}
	}
	for i, team := range ai.gotReq.Teams {
		if !want[team.Key] || team.Name == "" || (team.Field != "creTeam" && team.Field != "sreTeam") || team.Family == "" {
			t.Errorf("team %d = %+v, want a registry team with a group, its name, family and field", i, team)
		}
	}
	if len(ai.gotReq.Teams) != len(want) {
		t.Errorf("sent %d teams, want %d (every team with a group)", len(ai.gotReq.Teams), len(want))
	}
}

// TestServiceGenerator_PutsTheNoteInTheReplyOnly checks that a closest
// dashboard's note starts the chat reply and is not part of the dashboard.
func TestServiceGenerator_PutsTheNoteInTheReplyOnly(t *testing.T) {
	withNote := strings.Replace(validGenerated, `{"displayName"`,
		`{"note":"  Accounts can't be ranked by revenue, so this ranks them by open cases instead. ","displayName"`, 1)
	got, err := NewServiceDashboardGenerator(&fakeAIClient{res: csmai.DashboardResult{
		Candidates: []json.RawMessage{json.RawMessage(withNote)},
	}}, nil).Generate(context.Background(), "p", nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	want := `Accounts can't be ranked by revenue, so this ranks them by open cases instead. Here's "Top Customers by Open Cases", 1 widget.`
	if got.Reply != want {
		t.Errorf("reply = %q, want %q", got.Reply, want)
	}
	if got.Dashboard.DisplayName != "Top Customers by Open Cases" || len(got.Dashboard.Widgets) != 1 {
		t.Errorf("dashboard = %+v", got.Dashboard)
	}
}

// TestServiceGenerator_NoNoteKeepsThePlainReply checks that a dashboard
// built as asked (no note, or an empty one) has the usual reply.
func TestServiceGenerator_NoNoteKeepsThePlainReply(t *testing.T) {
	emptyNote := strings.Replace(validGenerated, `{"displayName"`, `{"note":" ","displayName"`, 1)
	for _, def := range []string{validGenerated, emptyNote} {
		got, err := NewServiceDashboardGenerator(&fakeAIClient{res: csmai.DashboardResult{
			Candidates: []json.RawMessage{json.RawMessage(def)},
		}}, nil).Generate(context.Background(), "p", nil)
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		if want := `Here's "Top Customers by Open Cases", 1 widget.`; got.Reply != want {
			t.Errorf("reply = %q, want %q", got.Reply, want)
		}
	}
}

// TestSplitGeneratedNote_RejectsABadNote checks that a note that is not a
// string (null included), or too long in characters, not bytes, makes the
// definition invalid, and that a definition that is not an object is passed
// on for ParseGenerated to reject.
func TestSplitGeneratedNote_RejectsABadNote(t *testing.T) {
	for _, note := range []string{`42`, `["a"]`, `null`, ` null `,
		`"` + strings.Repeat("x", generatedNoteMaxChars+1) + `"`,
		`"` + strings.Repeat("é", generatedNoteMaxChars+1) + `"`} {
		_, _, err := splitGeneratedNote(json.RawMessage(`{"note":` + note + `,"displayName":"D"}`))
		if !errors.Is(err, dashboard.ErrGeneratedInvalid) {
			t.Errorf("note %.20s: err = %v, want ErrGeneratedInvalid", note, err)
		}
	}
	if _, _, err := splitGeneratedNote(json.RawMessage(`{"note":"` + strings.Repeat("é", generatedNoteMaxChars) + `"}`)); err != nil {
		t.Errorf("a note of exactly %d characters: %v", generatedNoteMaxChars, err)
	}
	def, note, err := splitGeneratedNote(json.RawMessage(`[1]`))
	if err != nil || note != "" || string(def) != `[1]` {
		t.Errorf("not an object: %s, %q, %v", def, note, err)
	}
}

// TestSplitGeneratedNote_TrimsInvisibleCharacters checks that zero-width and
// other format characters are trimmed like spaces: a note of only those is
// blank, and one around a sentence keeps only the sentence.
func TestSplitGeneratedNote_TrimsInvisibleCharacters(t *testing.T) {
	cases := map[string]string{
		`"\u200b"`:                      "",
		`" \ufeff\u200b\u2060 "`:        "",
		`"\u200bShown by state.\u200d"`: "Shown by state.",
	}
	for raw, want := range cases {
		_, note, err := splitGeneratedNote(json.RawMessage(`{"note":` + raw + `,"displayName":"D"}`))
		if err != nil || note != want {
			t.Errorf("note %s: got %q, %v; want %q", raw, note, err, want)
		}
	}
}

// TestServiceGenerator_ANoteOnlyAtTheTop checks that only the top-level
// "note" is taken off: one inside a widget, or a differently cased key, is an
// unknown field and the candidate is unusable.
func TestServiceGenerator_ANoteOnlyAtTheTop(t *testing.T) {
	inWidget := strings.Replace(validGenerated, `{"id":"top"`, `{"note":"Shown by account.","id":"top"`, 1)
	capitalised := strings.Replace(validGenerated, `{"displayName"`, `{"Note":"Shown by account.","displayName"`, 1)
	for name, def := range map[string]string{"in a widget": inWidget, "Note": capitalised} {
		_, err := NewServiceDashboardGenerator(&fakeAIClient{res: csmai.DashboardResult{
			Candidates: []json.RawMessage{json.RawMessage(def)},
		}}, nil).Generate(context.Background(), "p", nil)
		if !errors.Is(err, errGenerateUnusable) {
			t.Errorf("%s: err = %v, want errGenerateUnusable", name, err)
		}
	}
}

// TestServiceGenerator_AValidNoteOnAnInvalidDashboard checks that a candidate
// whose note is fine but whose dashboard fails ParseGenerated is skipped, and
// that its note does not reach the reply of the earlier candidate used instead.
func TestServiceGenerator_AValidNoteOnAnInvalidDashboard(t *testing.T) {
	invalid := `{"note":"Shown by state instead.","displayName":"broken","widgets":[{"id":"x","linkResourceType":"account"}]}`
	got, err := NewServiceDashboardGenerator(&fakeAIClient{res: csmai.DashboardResult{
		Candidates: []json.RawMessage{json.RawMessage(validGenerated), json.RawMessage(invalid)},
	}}, nil).Generate(context.Background(), "p", nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if want := `Here's "Top Customers by Open Cases", 1 widget.`; got.Reply != want {
		t.Errorf("reply = %q, want %q (no note from the skipped candidate)", got.Reply, want)
	}
	_, err = NewServiceDashboardGenerator(&fakeAIClient{res: csmai.DashboardResult{
		Candidates: []json.RawMessage{json.RawMessage(invalid)},
	}}, nil).Generate(context.Background(), "p", nil)
	if !errors.Is(err, errGenerateUnusable) {
		t.Errorf("only that candidate: err = %v, want errGenerateUnusable", err)
	}
}

// TestServiceGenerator_ABadNoteFallsBackToAnEarlierCandidate checks that a
// candidate with a bad note is skipped like any other invalid one.
func TestServiceGenerator_ABadNoteFallsBackToAnEarlierCandidate(t *testing.T) {
	badNote := strings.Replace(validGenerated, `{"displayName":"Top Customers by Open Cases"`, `{"note":7,"displayName":"Other"`, 1)
	got, err := NewServiceDashboardGenerator(&fakeAIClient{res: csmai.DashboardResult{
		Candidates: []json.RawMessage{json.RawMessage(validGenerated), json.RawMessage(badNote)},
	}}, nil).Generate(context.Background(), "p", nil)
	if err != nil || got.Dashboard.DisplayName != "Top Customers by Open Cases" {
		t.Errorf("got %q, %v; want the first candidate", got.Dashboard.DisplayName, err)
	}
}
