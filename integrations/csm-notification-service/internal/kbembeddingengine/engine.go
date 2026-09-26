// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

// Package kbembeddingengine implements Flow 2 of the KB auto-generation
// work: on a KB article being published, fetch its title+body, generate an
// embedding + a short symptom summary via Azure OpenAI, and upsert both
// into Pinecone so NOVA's chatbot can find it. Structured the same way as
// internal/kbdraftengine -- its own Handle matching eventbus.Handle, run
// on the SAME shared consumer group as kbdraftengine
// (csm-notification-service-kb-embedding, both flows subscribe to the same
// case-events topic per Sajith's Sep 22 confirmation).
//
// Replaces logic that used to live entirely inside ServiceNow itself as
// internal automation (confirmed directly by Sasmitha, Sep 24) -- once
// ServiceNow's subscription ends, nothing else takes over this job, so
// this package is the sole thing keeping NOVA's knowledge from going stale
// after new articles are published on the new platform.
package kbembeddingengine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/kbclient"
)

// Engine implements eventbus.Handle for the kb.article_published event.
type Engine struct {
	kb       *kbclient.Client
	generate EmbeddingGenerator
	pinecone PineconeUpserter
}

func New(kb *kbclient.Client, generate EmbeddingGenerator, pinecone PineconeUpserter) *Engine {
	return &Engine{kb: kb, generate: generate, pinecone: pinecone}
}

// Handle implements eventbus.Handle.
func (e *Engine) Handle(ctx context.Context, record eventbus.Record) error {
	var env events.Envelope
	if err := json.Unmarshal(record.Value, &env); err != nil {
		return fmt.Errorf("kbembeddingengine: decode envelope: %w", err)
	}
	if env.Type != events.TypeKBArticlePublished {
		return nil
	}

	var p events.KBArticlePublishedPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return fmt.Errorf("kbembeddingengine: decode kb.article_published payload: %w", err)
	}

	return e.embedAndUpsert(ctx, p.KnowledgeArticleID)
}

func (e *Engine) embedAndUpsert(ctx context.Context, articleID string) error {
	slog.InfoContext(ctx, "kbembeddingengine: received kb.article_published", "articleID", articleID)

	article, err := e.kb.GetKBArticle(ctx, articleID)
	if err != nil {
		return fmt.Errorf("kbembeddingengine: fetch article %s: %w", articleID, err)
	}
	slog.InfoContext(ctx, "kbembeddingengine: fetched article", "articleID", articleID, "title", article.Title, "bodyLen", len(article.Body))

	symptom, err := e.generate.SummarizeSymptom(ctx, article.Title, article.Body)
	if err != nil {
		return fmt.Errorf("kbembeddingengine: summarize symptom for article %s: %w", articleID, err)
	}
	slog.InfoContext(ctx, "kbembeddingengine: generated symptom summary", "articleID", articleID, "symptom", symptom)

	// Embed title+body+symptom together, so the vector captures both the
	// article's own content and the plain-language problem framing --
	// matches what a user is actually searching with more closely than
	// title+body alone would.
	embedInput := article.Title + "\n\n" + article.Body + "\n\n" + symptom
	vector, err := e.generate.Embed(ctx, embedInput)
	if err != nil {
		return fmt.Errorf("kbembeddingengine: generate embedding for article %s: %w", articleID, err)
	}
	slog.InfoContext(ctx, "kbembeddingengine: generated embedding", "articleID", articleID, "vectorDims", len(vector))

	metadata := map[string]string{
		"title":      article.Title,
		"article_id": article.ID,
		"text":       article.Body,
		"symptom":    symptom,
	}
	if err := e.pinecone.Upsert(ctx, article.ID, vector, metadata); err != nil {
		return fmt.Errorf("kbembeddingengine: upsert article %s: %w", articleID, err)
	}
	slog.InfoContext(ctx, "kbembeddingengine: upserted to pinecone", "articleID", articleID)
	return nil
}
