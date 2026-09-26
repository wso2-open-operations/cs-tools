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

// PineconeUpserter writes one vector into Pinecone. A real implementation
// (PineconeClient) is used once real host/key config is available;
// MockPineconeUpserter stands in until then, same reasoning as
// MockEmbeddingGenerator/kbdraftengine.MockDraftGenerator.
type PineconeUpserter interface {
	Upsert(ctx context.Context, id string, vector []float32, metadata map[string]string) error
}

// MockPineconeUpserter logs and does nothing -- never use in a real
// deployment.
type MockPineconeUpserter struct{}

func (MockPineconeUpserter) Upsert(_ context.Context, _ string, _ []float32, _ map[string]string) error {
	return nil
}

// PineconeClient calls Pinecone's real REST API directly (no SDK
// dependency, matching kbclient/AzureOpenAIDraftGenerator's own
// plain-net/http style throughout this service).
//
// Namespace is deliberately the default/blank namespace ("") -- confirmed
// by reading NOVA's own search code (mcp_servers/novera_tools.py's
// query_params has no "namespace" key at all), which in Pinecone means it
// only ever searches the default namespace. Writing anywhere else would
// make new articles invisible to NOVA regardless of what namespace value
// was chosen, so this isn't a guess -- it's the only namespace that could
// possibly work given NOVA's own confirmed read behavior.
type PineconeClient struct {
	Host   string // e.g. https://knowledge-db-dev-9zzbo4t.svc.aped-4627-b74a.pinecone.io
	APIKey string
	http   *http.Client
}

func NewPineconeClient(host, apiKey string) *PineconeClient {
	return &PineconeClient{Host: host, APIKey: apiKey, http: &http.Client{Timeout: 30 * time.Second}}
}

type pineconeVector struct {
	ID       string            `json:"id"`
	Values   []float32         `json:"values"`
	Metadata map[string]string `json:"metadata"`
}

type pineconeUpsertRequest struct {
	Vectors   []pineconeVector `json:"vectors"`
	Namespace string           `json:"namespace"`
}

// Upsert implements PineconeUpserter, via POST {host}/vectors/upsert.
func (c *PineconeClient) Upsert(ctx context.Context, id string, vector []float32, metadata map[string]string) error {
	reqBody, err := json.Marshal(pineconeUpsertRequest{
		Vectors: []pineconeVector{
			{ID: id, Values: vector, Metadata: metadata},
		},
		Namespace: "",
	})
	if err != nil {
		return fmt.Errorf("pinecone: encode upsert request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, trimRightSlash(c.Host)+"/vectors/upsert", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("pinecone: build upsert request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Api-Key", c.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("pinecone: upsert request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("pinecone: read upsert response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pinecone: upsert status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
