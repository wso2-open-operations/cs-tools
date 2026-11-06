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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

// Package pinecone is a minimal read-only client for the vector index that
// backs related-article suggestions. It returns only match ids and scores:
// match metadata is never read, so nothing stored in the index (titles
// included) can reach a caller unverified.
package pinecone

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	maxResponseBytes = 1 << 20
	defaultTimeout   = 5 * time.Second
)

// Match is one similar vector.
type Match struct {
	ID    string
	Score float64
}

// Client queries a Pinecone index by vector id.
type Client struct {
	http   *http.Client
	host   string
	apiKey string
}

// New validates host (an https URL) and returns a Client. The HTTP client never
// follows redirects, so the API key header cannot be replayed to another host.
func New(host, apiKey string) (*Client, error) {
	u, err := url.Parse(strings.TrimSpace(host))
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, fmt.Errorf("pinecone: host must be an absolute http(s) URL")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("pinecone: API key is empty")
	}
	return &Client{
		http: &http.Client{
			Timeout:       defaultTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		host:   strings.TrimRight(u.String(), "/"),
		apiKey: apiKey,
	}, nil
}

type queryRequest struct {
	ID              string `json:"id"`
	TopK            int    `json:"topK"`
	IncludeMetadata bool   `json:"includeMetadata"`
}

type queryResponse struct {
	Matches []struct {
		ID    string  `json:"id"`
		Score float64 `json:"score"`
	} `json:"matches"`
}

// Similar returns up to topK vectors most similar to the vector with the given id.
func (c *Client) Similar(ctx context.Context, id string, topK int) ([]Match, error) {
	body, err := json.Marshal(queryRequest{ID: id, TopK: topK})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.host+"/query", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Api-Key", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pinecone: query: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pinecone: query returned status %d", resp.StatusCode)
	}
	var out queryResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&out); err != nil {
		return nil, fmt.Errorf("pinecone: decode: %w", err)
	}
	matches := make([]Match, 0, len(out.Matches))
	for _, m := range out.Matches {
		matches = append(matches, Match{ID: m.ID, Score: m.Score})
	}
	return matches, nil
}
