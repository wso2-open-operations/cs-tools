// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

package kbembeddingengine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// EmbeddingGenerator turns an article's title+body into a vector, and a
// short symptom/problem summary -- so Pinecone's "symptom" metadata field
// (confirmed by reading NOVA's own search code, which reads title,
// article_id, symptom, and text/article off each match) has something real
// to match a user's own wording of their problem against, not just the
// article's title.
type EmbeddingGenerator interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	SummarizeSymptom(ctx context.Context, title, body string) (string, error)
}

// MockEmbeddingGenerator stands in for a real Azure OpenAI key, same
// reasoning as kbdraftengine.MockDraftGenerator -- lets the rest of this
// flow (event filtering, article-fetching, Pinecone upsert) be built and
// tested before real embedding/chat keys are wired in. Never use in a real
// deployment -- returns an all-zero vector, which is useless for real
// search relevance.
type MockEmbeddingGenerator struct{}

func (MockEmbeddingGenerator) Embed(_ context.Context, text string) ([]float32, error) {
	return make([]float32, 3072), nil // text-embedding-3-large's real dimension
}

func (MockEmbeddingGenerator) SummarizeSymptom(_ context.Context, title, _ string) (string, error) {
	return "[MOCK SYMPTOM] " + title, nil
}

// AzureOpenAIEmbeddingGenerator calls Azure OpenAI's embeddings endpoint
// (model text-embedding-3-large, per Sasmitha's Sep 24 config) for Embed,
// and reuses the same chat-completions pattern kbdraftengine's
// AzureOpenAIDraftGenerator already uses for SummarizeSymptom -- same
// underlying chat deployment as Flow 1's draft generation, just a
// different prompt, so no new chat-side config is needed for that half.
type AzureOpenAIEmbeddingGenerator struct {
	// EmbeddingURL is the FULL embeddings endpoint URL, deployment name
	// and api-version already baked in (Sasmitha gave this as a complete
	// URL, unlike the chat endpoint's separate Endpoint+DeploymentName
	// pattern) -- e.g.
	// https://wso2-openai-us.openai.azure.com/openai/deployments/text-embedding-3-large/embeddings?api-version=2023-05-15
	EmbeddingURL string
	EmbeddingKey string

	// ChatEndpoint/ChatAPIKey/ChatDeploymentName reuse the SAME chat
	// deployment kbdraftengine's AzureOpenAIDraftGenerator already talks
	// to (Flow 1's existing AZURE_US_OPENAI_* config) -- symptom
	// summarization is a text-generation task like draft generation
	// itself, no separate chat deployment needed for it.
	ChatEndpoint       string
	ChatAPIKey         string
	ChatDeploymentName string

	http *http.Client
}

func NewAzureOpenAIEmbeddingGenerator(embeddingURL, embeddingKey, chatEndpoint, chatAPIKey, chatDeploymentName string) *AzureOpenAIEmbeddingGenerator {
	return &AzureOpenAIEmbeddingGenerator{
		EmbeddingURL:       embeddingURL,
		EmbeddingKey:       embeddingKey,
		ChatEndpoint:       chatEndpoint,
		ChatAPIKey:         chatAPIKey,
		ChatDeploymentName: chatDeploymentName,
		http:               &http.Client{Timeout: 60 * time.Second},
	}
}

type azureEmbeddingRequest struct {
	Input string `json:"input"`
}

type azureEmbeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

// Embed implements EmbeddingGenerator.
func (g *AzureOpenAIEmbeddingGenerator) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody, err := json.Marshal(azureEmbeddingRequest{Input: text})
	if err != nil {
		return nil, fmt.Errorf("azure openai: encode embedding request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.EmbeddingURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("azure openai: build embedding request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", g.EmbeddingKey)

	resp, err := g.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("azure openai: embedding request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("azure openai: read embedding response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("azure openai: embedding status %d: %s", resp.StatusCode, string(body))
	}

	var parsed azureEmbeddingResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("azure openai: decode embedding response: %w", err)
	}
	if len(parsed.Data) == 0 {
		return nil, fmt.Errorf("azure openai: embedding returned no data")
	}
	return parsed.Data[0].Embedding, nil
}

type azureChatRequest struct {
	Messages []azureChatMessage `json:"messages"`
}

type azureChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type azureChatResponse struct {
	Choices []struct {
		Message azureChatMessage `json:"message"`
	} `json:"choices"`
}

// SummarizeSymptom implements EmbeddingGenerator. Mirrors
// kbdraftengine.AzureOpenAIDraftGenerator.Summarize's exact HTTP pattern,
// same chat deployment, different prompt: a short 1-2 sentence
// problem/symptom description a user might actually type when stuck,
// rather than a technical summary.
func (g *AzureOpenAIEmbeddingGenerator) SummarizeSymptom(ctx context.Context, title, body string) (string, error) {
	prompt := fmt.Sprintf(
		"In 1-2 plain sentences, describe the core problem or symptom this knowledge base article addresses -- phrased the way a user experiencing the issue might describe it, not a technical summary of the fix.\n\nTitle: %s\n\nBody:\n%s",
		title, body,
	)

	reqBody, err := json.Marshal(azureChatRequest{
		Messages: []azureChatMessage{
			{Role: "system", Content: "You write short, plain-language problem/symptom descriptions for knowledge base search matching."},
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return "", fmt.Errorf("azure openai: encode symptom request: %w", err)
	}

	url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=2024-02-15-preview",
		trimRightSlash(g.ChatEndpoint), g.ChatDeploymentName)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("azure openai: build symptom request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", g.ChatAPIKey)

	resp, err := g.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("azure openai: symptom request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("azure openai: read symptom response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("azure openai: symptom status %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed azureChatResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("azure openai: decode symptom response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("azure openai: symptom returned no choices")
	}
	return parsed.Choices[0].Message.Content, nil
}

func trimRightSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}
