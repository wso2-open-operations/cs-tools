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

// Package kbdraftengine implements Flow 1 of the KB auto-generation work:
// on a closed case, fetch its comments, generate 1-3 draft KB articles via
// an LLM, and save each as a draft through this platform's existing
// POST /kb-articles endpoint. Structured the same way as internal/slaengine
// and internal/timecardengine -- its own Handle matching eventbus.Handle,
// run on its own dedicated consumer group in cmd/server/main.go.
//
// Reuses the EXISTING case.status_changed event -- no new event type or
// upstream publisher change was needed for this flow. Handle filters for
// NewStatus == "Closed" and ignores every other status change.
//
// TWO OPEN DESIGN QUESTIONS, deliberately left as explicit config rather
// than guessed at:
//   - authorID: who a generated draft's author_id should be. Not
//     necessarily the case's own CreatedBy (the customer) -- likely a
//     dedicated service/bot user id. Needs a real decision (Sajith).
//   - resolveKnowledgeBaseID: which knowledge_bases row a case maps to.
//     Case carries DeployedProductID, not knowledge_bases.product_id
//     directly -- there is no simple 1:1 lookup today. Left as an
//     injected function so this can be wired correctly once that mapping
//     is confirmed, rather than assumed here.
package kbdraftengine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/kbclient"
)

// DraftArticle is one LLM-generated draft, before it's saved.
type DraftArticle struct {
	Title string
	Body  string
}

// DraftGenerator turns a closed case's comments into 1-3 draft articles.
// A real implementation (OpenAI) is swapped in once a test/placeholder key
// is available; MockDraftGenerator stands in for it until then.
type DraftGenerator interface {
	Generate(ctx context.Context, caseID string, comments []string) ([]DraftArticle, error)
	// Summarize condenses one batch of comments into a short summary,
	// preserving key details. Used by generateFromComments to process a
	// long case in commentPageSize-sized chunks before final generation,
	// so a case with more comments than one page doesn't silently lose
	// any of them -- agreed approach with Sasmitha over plain batching
	// (which risked fragmenting one case into several disconnected
	// articles): summarize each chunk, then generate one article from
	// the combined summaries.
	Summarize(ctx context.Context, caseID string, comments []string) (string, error)
}

// MockDraftGenerator returns one fixed, clearly-labeled fake draft per
// call -- lets every other part of this flow (event filtering,
// comment-fetching, draft-saving) be built and tested end to end before a
// real OpenAI key exists. Never wire this into a real deployment.
type MockDraftGenerator struct{}

func (MockDraftGenerator) Generate(_ context.Context, caseID string, comments []string) ([]DraftArticle, error) {
	return []DraftArticle{
		{
			Title: "[MOCK DRAFT] Generated from case " + caseID,
			Body:  fmt.Sprintf("This is a placeholder draft standing in for a real LLM-generated article. %d comment(s) were available on this case.", len(comments)),
		},
	}, nil
}

func (MockDraftGenerator) Summarize(_ context.Context, caseID string, comments []string) (string, error) {
	return fmt.Sprintf("[MOCK SUMMARY] %d comment(s) from case %s", len(comments), caseID), nil
}

// Engine implements eventbus.Handle for the case.status_changed event,
// reacting only when NewStatus == "Closed".
type Engine struct {
	kb       *kbclient.Client
	generate DraftGenerator
	// authorID is the user_id that generated drafts are saved under. See
	// package doc -- TODO: confirm the real value with Sajith before this
	// goes anywhere near a real deployment.
	authorID string
	// resolveKnowledgeBaseID maps a case to the knowledge_bases.id its
	// generated draft(s) should be saved under. See package doc -- TODO:
	// confirm the real case->KB mapping before this goes anywhere near a
	// real deployment. Returning an error here causes Handle to retry
	// (see eventbus.handleAttempts), matching every other Handle in this
	// service.
	resolveKnowledgeBaseID func(ctx context.Context, kb *kbclient.Client, caseID string) (string, error)
}

func New(kb *kbclient.Client, generate DraftGenerator, authorID string, resolveKnowledgeBaseID func(context.Context, *kbclient.Client, string) (string, error)) *Engine {
	return &Engine{kb: kb, generate: generate, authorID: authorID, resolveKnowledgeBaseID: resolveKnowledgeBaseID}
}

// Handle implements eventbus.Handle.
func (e *Engine) Handle(ctx context.Context, record eventbus.Record) error {
	var env events.Envelope
	if err := json.Unmarshal(record.Value, &env); err != nil {
		return fmt.Errorf("kbdraftengine: decode envelope: %w", err)
	}
	if env.Type != events.TypeStatusChanged {
		return nil
	}
	if err := events.Validate(env.EntityID, env.Type, env.Payload); err != nil {
		return fmt.Errorf("kbdraftengine: invalid payload: %w", err)
	}

	var p events.StatusChangedPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return fmt.Errorf("kbdraftengine: decode case.status_changed payload: %w", err)
	}
	if p.NewStatus != "Closed" {
		return nil
	}

	return e.generateDrafts(ctx, p.CaseID)
}

func (e *Engine) generateDrafts(ctx context.Context, caseID string) error {
	texts, err := e.fetchAllCommentTexts(ctx, caseID)
	if err != nil {
		return fmt.Errorf("kbdraftengine: fetch comments for case %s: %w", caseID, err)
	}

	drafts, err := e.generateFromComments(ctx, caseID, texts)
	if err != nil {
		return fmt.Errorf("kbdraftengine: generate drafts for case %s: %w", caseID, err)
	}

	if os.Getenv("SKIP_KB_SAVE") == "true" {
		for _, d := range drafts {
			slog.InfoContext(ctx, "kbdraftengine: [TEST] generated draft (not saved)", "caseID", caseID, "title", d.Title, "body", d.Body)
		}
		return nil
	}

	knowledgeBaseID, err := e.resolveKnowledgeBaseID(ctx, e.kb, caseID)
	if err != nil {
		return fmt.Errorf("kbdraftengine: resolve knowledge base for case %s: %w", caseID, err)
	}

	for _, d := range drafts {
		req := kbclient.CreateKBArticleRequest{
			KnowledgeBaseID: knowledgeBaseID,
			Title:           d.Title,
			Body:            d.Body,
			AuthorID:        e.authorID,
		}
		if err := e.kb.CreateKBArticle(ctx, req); err != nil {
			return fmt.Errorf("kbdraftengine: save draft for case %s: %w", caseID, err)
		}
		slog.InfoContext(ctx, "kbdraftengine: saved draft article", "caseID", caseID, "knowledgeBaseID", knowledgeBaseID)
	}
	return nil
}

// commentPageSize matches entity-service's hard per-request cap on
// SearchCaseComments (see its user_service.go maxLimit -- 50, because
// the backing data source rejects anything above that). This is a
// shared, general-purpose limit used by other searches too, so it's
// deliberately left untouched -- fetchAllCommentTexts pages around it
// on this side instead of raising it, so no other feature is affected.
const commentPageSize = 50

// fetchAllCommentTexts pages through every comment on the case,
// commentPageSize at a time, and returns all of their content in one
// flat list, oldest-first -- whole and unabridged regardless of how
// many comments the case has. Callers don't need to know pagination is
// happening underneath.
func (e *Engine) fetchAllCommentTexts(ctx context.Context, caseID string) ([]string, error) {
	var all []string
	offset := 0
	for {
		page, err := e.kb.SearchCaseComments(ctx, caseID, commentPageSize, offset)
		if err != nil {
			return nil, err
		}
		for _, c := range page {
			all = append(all, c.Content)
		}
		if len(page) < commentPageSize {
			break // last page
		}
		offset += commentPageSize
	}
	return all, nil
}

// generateFromComments produces the draft article(s) for a case's full
// comment set. A case within one page's worth of comments is sent
// straight to Generate, unchanged from before. A longer case is
// summarized in commentPageSize-sized chunks first -- agreed with
// Sasmitha over plain batching into multiple separate articles, which
// risked losing context and fragmenting the result -- then Generate is
// called once on the combined summaries, producing one coherent
// article per case with no comment silently dropped.
func (e *Engine) generateFromComments(ctx context.Context, caseID string, texts []string) ([]DraftArticle, error) {
	if len(texts) <= commentPageSize {
		return e.generate.Generate(ctx, caseID, texts)
	}

	var summaries []string
	for i := 0; i < len(texts); i += commentPageSize {
		end := i + commentPageSize
		if end > len(texts) {
			end = len(texts)
		}
		summary, err := e.generate.Summarize(ctx, caseID, texts[i:end])
		if err != nil {
			return nil, fmt.Errorf("summarize comments %d-%d: %w", i, end, err)
		}
		summaries = append(summaries, summary)
	}
	return e.generate.Generate(ctx, caseID, summaries)
}
