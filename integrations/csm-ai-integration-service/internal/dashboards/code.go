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
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-ai-integration-service/internal/playbooks"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-ai-integration-service/internal/teams"
)

// codeMaxTokens caps one code reply. Generated code is longer than a JSON
// definition (the catalogue's example is about 1,500 output tokens); the
// catalogue asks for under 200 lines, which stays well inside this.
const codeMaxTokens = 10000

// CodeMaxLength caps a source, in characters (runes): a reply over it is
// unusable, and a draft over it is refused before any model call, since it
// is sent back to the model on every follow-up.
const CodeMaxLength = 60000

// codeReplyMaxLength caps the model's sentence shown in the chat.
const codeReplyMaxLength = 500

// codeDefaultReply is shown when the model wrote no sentence.
const codeDefaultReply = "Here's your dashboard."

// codeNameMaxLength caps the suggested name, in characters (runes): the
// longest name a saved dashboard can have, so the suggestion can always be
// saved as it is.
const codeNameMaxLength = 255

// codeNameLineRe matches the output contract's "Name: ..." line, capturing
// the name; "name:" and a bold "**Name:**" are taken too (codeName drops the
// closing bold marks).
var codeNameLineRe = regexp.MustCompile(`(?mi)^[ \t]*(?:\*\*)?name(?:\*\*)?[ \t]*:[ \t]*(.*?)[ \t]*\r?$\n?`)

// codeFenceRe matches a fenced code block (```js, ```javascript or a bare
// ```), capturing its body.
var codeFenceRe = regexp.MustCompile("(?s)```[ \\t]*(?:js|javascript)?[ \\t]*\\r?\\n(.*?)\\r?\\n?```")

// renderFuncRe is the one thing a source must contain: a render function.
var renderFuncRe = regexp.MustCompile(`\bfunction\s+render\s*\(`)

// codeDeclinePrefix starts the code catalogue's refusal line.
const codeDeclinePrefix = "CANNOT:"

// CodeResult is what the model answered for the code approach: Declined
// holds its sentence when it refused in the catalogue's words ("CANNOT:
// ..."); otherwise Source is the code to run, Reply the chat sentence and
// Name the name it suggests saving the dashboard under ("" when it gave
// none). The token counts are as on Result.
type CodeResult struct {
	Source                   string
	Reply                    string
	Name                     string
	Declined                 string
	InputTokens              int64
	CacheCreationInputTokens int64
	CacheReadInputTokens     int64
	OutputTokens             int64
}

// CodeGenerator asks the model for dashboard code: the code catalogue plus
// the Teams and PLG playbook answers sections is the system prompt; the user message is the prompt and, on a follow-up, the
// source being edited. The reply is read as the catalogue's output contract
// (parseCodeReply).
type CodeGenerator struct {
	client    anthropic.Client
	model     string
	timeout   time.Duration
	catalogue Catalogue
}

// NewCodeGenerator returns a generator calling model through client
// (claude.NewClient) with the code catalogue as its system prompt. timeout
// bounds one chat message as a whole, the client's retry included; zero means
// no bound beyond the request's own.
func NewCodeGenerator(client anthropic.Client, model string, timeout time.Duration, catalogue Catalogue) *CodeGenerator {
	return &CodeGenerator{client: client, model: model, timeout: timeout, catalogue: catalogue}
}

// Generate makes one model call for one chat message. current is the source
// being edited, or "" for the first message; registry is the teams a
// dashboard can filter by and answers the PLG playbook answer fields (the
// Teams and PLG playbook answers sections, as for the dashboard generator).
func (g *CodeGenerator) Generate(ctx context.Context, prompt, current string, registry []teams.Team, answers []playbooks.Field) (CodeResult, error) {
	if g.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, g.timeout)
		defer cancel()
	}
	start := time.Now()
	msg, err := g.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(g.model),
		MaxTokens: codeMaxTokens,
		// The code catalogue, the teams and the playbook answers are the same
		// for every call, so they are cached as the dashboard catalogue is
		// (see Generator.Generate).
		System:   []anthropic.TextBlockParam{{Text: playbooks.WithPlaybookAnswers(teams.WithTeams(g.catalogue.Text(), registry), answers), CacheControl: anthropic.NewCacheControlEphemeralParam()}},
		Messages: []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(codeUserMessage(prompt, current, now())))},
	})
	took := time.Since(start)
	if err != nil {
		if isUsageLimit(err) {
			logUsage(ctx, g.model, outcomeUsageLimit, "", anthropic.Usage{}, took, "approach", "code")
			return CodeResult{}, fmt.Errorf("%w: %w: %w", ErrCall, ErrUsageLimit, err)
		}
		logUsage(ctx, g.model, outcomeFailed, "", anthropic.Usage{}, took, "approach", "code")
		return CodeResult{}, fmt.Errorf("%w: %w", ErrCall, err)
	}
	res, outcome, err := readCodeReply(msg)
	logUsage(ctx, g.model, outcome, msg.StopReason, msg.Usage, took, "approach", "code")
	return res, err
}

// readCodeReply reads the model's reply into a code result, and names its
// outcome.
func readCodeReply(msg *anthropic.Message) (CodeResult, string, error) {
	switch msg.StopReason {
	case anthropic.StopReasonRefusal:
		return CodeResult{}, outcomeRefused, ErrRefused
	case anthropic.StopReasonMaxTokens:
		return CodeResult{}, outcomeTruncated, ErrTruncated
	}
	res, err := parseCodeReply(messageText(msg))
	if err != nil {
		return CodeResult{}, outcomeUnusable, err
	}
	res.InputTokens = msg.Usage.InputTokens
	res.CacheCreationInputTokens = msg.Usage.CacheCreationInputTokens
	res.CacheReadInputTokens = msg.Usage.CacheReadInputTokens
	res.OutputTokens = msg.Usage.OutputTokens
	if res.Declined != "" {
		return res, outcomeDeclined, nil
	}
	return res, outcomeOK, nil
}

// codeUserMessage is the prompt alone on a first message, or the source
// followed by the prompt as the change to make, and then today's date (UTC),
// as userMessage does: the code catalogue resolves a month named without a
// year against it, and the date stays out of the cached system prompt.
func codeUserMessage(prompt, current string, today time.Time) string {
	date := "\n\nToday's date: " + today.UTC().Format("2006-01-02")
	if current == "" {
		return prompt + date
	}
	return "This is the dashboard code I am editing:\n\n```js\n" + current +
		"\n```\n\nChange it as follows, and reply with the whole updated code:\n\n" + prompt + date
}

// parseCodeReply reads a reply in the code catalogue's output contract: a
// "Name: ..." line, a sentence or two, then one fenced code block. The last
// block is taken (the model correcting itself), and it must define a render
// function. A reply with no block that starts with "CANNOT:" is the
// catalogue's refusal (after any Name line); anything else without a block
// is unusable. The name
// is the first Name line before the block (codeName); a reply without one is
// still usable, with no name. The reply text is the rest of the prose before
// the block, collapsed onto one line and capped.
func parseCodeReply(text string) (CodeResult, error) {
	matches := codeFenceRe.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		// A decline needs no name; one written out of habit is dropped.
		trimmed := strings.TrimSpace(codeNameLineRe.ReplaceAllString(text, ""))
		if reason, ok := strings.CutPrefix(trimmed, codeDeclinePrefix); ok {
			return CodeResult{Declined: capRunes(collapseSpace(reason), codeReplyMaxLength)}, nil
		}
		return CodeResult{}, fmt.Errorf("%w: no code block in the reply", ErrUnusable)
	}
	last := matches[len(matches)-1]
	source := strings.TrimSpace(text[last[2]:last[3]])
	if source == "" {
		return CodeResult{}, fmt.Errorf("%w: empty code block", ErrUnusable)
	}
	if utf8.RuneCountInString(source) > CodeMaxLength {
		return CodeResult{}, fmt.Errorf("%w: code over %d characters", ErrUnusable, CodeMaxLength)
	}
	if !renderFuncRe.MatchString(source) {
		return CodeResult{}, fmt.Errorf("%w: code defines no render function", ErrUnusable)
	}

	prose := text[:matches[0][0]]
	name := ""
	if loc := codeNameLineRe.FindStringSubmatchIndex(prose); loc != nil {
		name = codeName(prose[loc[2]:loc[3]])
		prose = prose[:loc[0]] + prose[loc[1]:]
	}
	reply := capRunes(collapseSpace(prose), codeReplyMaxLength)
	if reply == "" {
		reply = codeDefaultReply
	}
	return CodeResult{Source: source + "\n", Reply: reply, Name: name}, nil
}

// codeName is the name on a Name line: quotes, backticks and bold marks
// around it dropped, collapsed onto one line and capped at codeNameMaxLength.
func codeName(s string) string {
	return capRunes(collapseSpace(strings.Trim(s, "\"'`*“”‘’ \t")), codeNameMaxLength)
}

// collapseSpace trims s and joins its lines and runs of spaces with one space.
func collapseSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// capRunes cuts s to at most n characters, ending in "..." when cut.
func capRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return strings.TrimSpace(string(r[:n-3])) + "..."
}
