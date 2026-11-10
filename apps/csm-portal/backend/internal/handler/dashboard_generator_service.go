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
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/csmai"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/dashboard"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/directory"
	plgdomain "github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/domain"
)

// aiDashboardClient is csmai.Client: the model calls, made by
// integrations/csm-ai-integration-service.
type aiDashboardClient interface {
	GenerateDashboard(ctx context.Context, req csmai.DashboardRequest) (csmai.DashboardResult, error)
}

// serviceDashboardGenerator is the generator when the model call is the
// AI integration service's. The service builds the prompt (the catalogue,
// the Teams section from the team list sent here, the prompt and the draft)
// and returns every JSON object of the reply; judging them stays here: the
// last one dashboard.ParseGenerated accepts is the answer, so a definition
// is held to the same rules as a file. The service's error codes become this
// package's generate errors (errGenerateCall, errGenerateRefused, ...:
// serviceGenerateError), so writeGenerateError maps them and the user sees
// the same messages as before the model call moved.
type serviceDashboardGenerator struct {
	client    aiDashboardClient
	teams     []csmai.Team
	playbooks func(context.Context) ([]plgdomain.Playbook, error)
}

// WithPlaybooks makes each Generate read the PLG playbooks with list, send
// the model their answer fields (playbookAnswerFields), and refuse a
// dashboard whose playbook answer filter names a task or an answer they do
// not have (checkPlaybookAnswers). Without it, the model is sent none. g is
// a generator from NewServiceDashboardGenerator; any other is returned as it
// is.
func WithPlaybooks(g DashboardGenerator, list func(context.Context) ([]plgdomain.Playbook, error)) DashboardGenerator {
	if s, ok := g.(*serviceDashboardGenerator); ok {
		s.playbooks = list
	}
	return g
}

// NewServiceDashboardGenerator returns the generator calling the AI
// integration service. teams is the registry (directory.TeamResults); only
// the teams with a group configured are sent, the ones a dashboard can
// filter by (see dashboardTeams).
func NewServiceDashboardGenerator(client aiDashboardClient, teams []directory.TeamResult) DashboardGenerator {
	var sent []csmai.Team
	for _, t := range newDashboardTeams(teams).list {
		if t.GroupID == "" {
			continue
		}
		sent = append(sent, csmai.Team{Key: t.Key, Name: t.Name, Family: t.Family, Field: t.Field})
	}
	return &serviceDashboardGenerator{client: client, teams: sent}
}

// Generate makes one service call for one chat message.
func (g *serviceDashboardGenerator) Generate(ctx context.Context, prompt string, current json.RawMessage) (GeneratedDashboard, error) {
	draft, err := draftToDefinition(current)
	if err != nil {
		return GeneratedDashboard{}, err
	}
	if len(draft) > generateDraftMaxBytes {
		return GeneratedDashboard{}, fmt.Errorf("%w: %d bytes", errDraftTooLarge, len(draft))
	}
	// The playbooks are read on every message (an admin may have edited
	// one); when they cannot be, the model is sent none, and a playbook
	// answer filter in its reply is refused below.
	var answers []csmai.PlaybookAnswerField
	if g.playbooks != nil {
		playbooks, err := g.playbooks(ctx)
		if err != nil {
			slog.WarnContext(ctx, "dashboard generate: PLG playbooks unavailable; sending no playbook answers", "err", err)
		}
		answers = playbookAnswerFields(playbooks)
	}
	res, err := g.client.GenerateDashboard(ctx, csmai.DashboardRequest{Prompt: prompt, Draft: draft, Teams: g.teams, PlaybookAnswers: answers})
	if err != nil {
		return GeneratedDashboard{}, serviceGenerateError(err)
	}
	slog.InfoContext(ctx, "dashboard generate: model replied",
		"inputTokens", res.Usage.InputTokens,
		"cacheCreationInputTokens", res.Usage.CacheCreationInputTokens,
		"cacheReadInputTokens", res.Usage.CacheReadInputTokens,
		"outputTokens", res.Usage.OutputTokens, "candidates", len(res.Candidates))
	if res.Declined != "" {
		return GeneratedDashboard{}, &generateDeclinedError{Reason: res.Declined}
	}
	if len(res.Candidates) == 0 {
		return GeneratedDashboard{}, fmt.Errorf("%w: no JSON object in the reply", errGenerateUnusable)
	}

	// The last valid object wins: a self-correction (wrong, then right) takes
	// the correction, and a "correction" that breaks a valid answer (right,
	// then wrong) keeps the answer. With none valid, the last one's error.
	var lastErr error
	for i := len(res.Candidates) - 1; i >= 0; i-- {
		definition, note, err := splitGeneratedNote(res.Candidates[i])
		var d dashboard.Dashboard
		if err == nil {
			d, err = dashboard.ParseGenerated(definition)
		}
		if err != nil {
			if lastErr == nil {
				lastErr = err
			}
			continue
		}
		if len(res.Candidates) > 1 {
			slog.InfoContext(ctx, "dashboard generate: reply held several JSON objects; using the last valid one",
				"count", len(res.Candidates), "used", i+1)
		}
		if g.playbooks != nil {
			if err := checkPlaybookAnswers(d, answers); err != nil {
				return GeneratedDashboard{}, err
			}
		}
		reply := generatedReply(d)
		if note != "" {
			reply = note + " " + reply
		}
		return GeneratedDashboard{Dashboard: d, Reply: reply}, nil
	}
	return GeneratedDashboard{}, fmt.Errorf("%w: %w", errGenerateUnusable, lastErr)
}

// generatedNoteMaxChars caps the model's note: the catalogue asks for one
// sentence.
const generatedNoteMaxChars = 500

// splitGeneratedNote takes the optional top-level "note" off a dashboard the
// model wrote and returns the definition without it, for ParseGenerated. The
// model writes a note when it built the closest dashboard it could instead of
// the one asked for, saying what differs (catalogue section 1); the note goes
// into the chat reply only, so the draft, and a dashboard saved from it, never
// carries one. A definition without a note, or one that is not a JSON
// object, is returned as it is (ParseGenerated rejects the latter). A note
// that is not a string (null included), or longer than generatedNoteMaxChars,
// makes the definition invalid. Spaces and invisible format characters (such
// as a zero-width space) are trimmed from both ends, so a note of only those
// is blank and keeps the plain reply. Only the top-level "note" is taken off:
// one in a widget, or a differently cased key, stays for ParseGenerated to
// reject as an unknown field.
func splitGeneratedNote(raw json.RawMessage) (json.RawMessage, string, error) {
	var top map[string]json.RawMessage
	if json.Unmarshal(raw, &top) != nil {
		return raw, "", nil
	}
	rawNote, ok := top["note"]
	if !ok {
		return raw, "", nil
	}
	var note string
	if bytes.Equal(bytes.TrimSpace(rawNote), []byte("null")) || json.Unmarshal(rawNote, &note) != nil {
		return nil, "", fmt.Errorf("%w: \"note\" must be a string", dashboard.ErrGeneratedInvalid)
	}
	note = strings.TrimFunc(note, func(r rune) bool { return unicode.IsSpace(r) || unicode.Is(unicode.Cf, r) })
	if utf8.RuneCountInString(note) > generatedNoteMaxChars {
		return nil, "", fmt.Errorf("%w: \"note\" is longer than %d characters", dashboard.ErrGeneratedInvalid, generatedNoteMaxChars)
	}
	delete(top, "note")
	definition, err := json.Marshal(top)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", dashboard.ErrGeneratedInvalid, err)
	}
	return definition, note, nil
}

// serviceGenerateError turns a csmai error into the generator errors
// writeGenerateError maps:
//
//   - not_configured                          ErrGenerateUnavailable (503)
//   - refused                                 errGenerateRefused (422)
//   - truncated / unusable                    errGenerateTruncated / errGenerateUnusable (502)
//   - usage_limit                             errGenerateUsageLimit (503)
//   - timeout, or the call itself timing out  errGenerateCall with a deadline (504)
//   - model_call_failed, anything else        errGenerateCall (502): a 400
//     from the service, a gateway 401/403 or a failed connection is this
//     backend's or the service's problem, never the caller's
func serviceGenerateError(err error) error {
	var se *csmai.Error
	if errors.As(err, &se) {
		switch se.Code {
		case csmai.CodeNotConfigured:
			return fmt.Errorf("%w: %w", ErrGenerateUnavailable, err)
		case csmai.CodeRefused:
			return errGenerateRefused
		case csmai.CodeTruncated:
			return fmt.Errorf("%w: %w", errGenerateTruncated, err)
		case csmai.CodeUnusable:
			return fmt.Errorf("%w: %w", errGenerateUnusable, err)
		case csmai.CodeUsageLimit:
			return fmt.Errorf("%w: %w", errGenerateUsageLimit, err)
		case csmai.CodeTimeout:
			return fmt.Errorf("%w: %w: %w", errGenerateCall, err, context.DeadlineExceeded)
		}
		if se.StatusCode == http.StatusGatewayTimeout {
			return fmt.Errorf("%w: %w: %w", errGenerateCall, err, context.DeadlineExceeded)
		}
	}
	return fmt.Errorf("%w: %w", errGenerateCall, err)
}
