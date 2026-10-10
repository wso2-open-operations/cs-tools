// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

package kbdraftengine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type AzureOpenAIDraftGenerator struct {
	Endpoint       string
	APIKey         string
	DeploymentName string
	http           *http.Client
}

func NewAzureOpenAIDraftGenerator(endpoint, apiKey, deploymentName string) *AzureOpenAIDraftGenerator {
	return &AzureOpenAIDraftGenerator{
		Endpoint:       endpoint,
		APIKey:         apiKey,
		DeploymentName: deploymentName,
		http:           &http.Client{Timeout: 60 * time.Second},
	}
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

func (g *AzureOpenAIDraftGenerator) Generate(ctx context.Context, caseID string, comments []string) ([]DraftArticle, error) {
	prompt := fmt.Sprintf(
		"Write a short knowledge base article (title and body) based on the following closed support case comments:\n\n%s",
		joinComments(comments),
	)

	reqBody, err := json.Marshal(azureChatRequest{
		Messages: []azureChatMessage{
			{Role: "system", Content: "You write concise, technical knowledge base articles from support case comments."},
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("azure openai: encode request: %w", err)
	}

	url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=2024-02-15-preview",
		strings.TrimRight(g.Endpoint, "/"), g.DeploymentName)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("azure openai: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", g.APIKey)

	resp, err := g.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("azure openai: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("azure openai: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("azure openai: status %d: %s", resp.StatusCode, string(body))
	}

	var parsed azureChatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("azure openai: decode response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("azure openai: no choices returned")
	}

	return []DraftArticle{
		{
			Title: "[AI DRAFT] Generated from case " + caseID,
			Body:  parsed.Choices[0].Message.Content,
		},
	}, nil
}

// Summarize implements DraftGenerator. Uses the same chat-completions
// endpoint as Generate, with a summarization-focused prompt instead of
// an article-writing one -- condenses one batch of comments into a
// short summary, explicitly instructed to preserve technical detail
// rather than write polished prose, since its output feeds back into
// Generate as input, not directly to a reader.
func (g *AzureOpenAIDraftGenerator) Summarize(ctx context.Context, caseID string, comments []string) (string, error) {
	prompt := fmt.Sprintf(
		"Summarize the key technical details, symptoms, and resolution steps from the following support case comments. Preserve specific facts (error messages, versions, configuration values) rather than writing polished prose -- this summary will be combined with others to write a KB article later:\n\n%s",
		joinComments(comments),
	)

	reqBody, err := json.Marshal(azureChatRequest{
		Messages: []azureChatMessage{
			{Role: "system", Content: "You write concise, technically precise summaries of support case comments for later use in writing a knowledge base article."},
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return "", fmt.Errorf("azure openai: encode summarize request: %w", err)
	}

	url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=2024-02-15-preview",
		strings.TrimRight(g.Endpoint, "/"), g.DeploymentName)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("azure openai: build summarize request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", g.APIKey)

	resp, err := g.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("azure openai: summarize request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("azure openai: read summarize response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("azure openai: summarize status %d: %s", resp.StatusCode, string(body))
	}

	var parsed azureChatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("azure openai: decode summarize response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("azure openai: summarize returned no choices")
	}

	return parsed.Choices[0].Message.Content, nil
}

func joinComments(comments []string) string {
	out := ""
	for i, c := range comments {
		out += fmt.Sprintf("%d. %s\n", i+1, c)
	}
	return out
}
