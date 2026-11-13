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
// Package moesif reads events from Moesif's Management API. It is shared by
// every sub-cron that reads Moesif, so the key, paging and error rules live in
// one place. A response body is never logged or returned: it can carry
// personal data.
package moesif

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/httpsec"
)

// DefaultBaseURL is Moesif's API host.
const DefaultBaseURL = "https://api.moesif.com"

const (
	defaultPageSize = 100
	defaultMaxPages = 100
	defaultTimeout  = 30 * time.Second
	maxResponse     = 16 << 20
	timeLayout      = "2006-01-02T15:04:05.000Z"
)

// ErrUnavailable means Moesif could not answer (network, 429 or 5xx).
var ErrUnavailable = errors.New("moesif unavailable")

// Config holds the client's settings.
type Config struct {
	BaseURL string // a trailing /v1 is accepted
	APIKey  string // Management API token; a secret
	Timeout time.Duration
}

// Client searches Moesif events.
type Client struct {
	http    *http.Client
	baseURL string
	apiKey  string
}

// NewClient builds a Client.
func NewClient(cfg Config) (*Client, error) {
	base := strings.TrimSuffix(strings.TrimRight(cfg.BaseURL, "/"), "/v1")
	if base == "" {
		base = DefaultBaseURL
	}
	if err := httpsec.RequireSecureURL(base); err != nil {
		return nil, fmt.Errorf("moesif: base URL: %w", err)
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errors.New("moesif: API key is required")
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	httpClient := &http.Client{Timeout: timeout}
	httpsec.RejectInsecureRedirects(httpClient)
	return &Client{http: httpClient, baseURL: base, apiKey: cfg.APIKey}, nil
}

// Query selects events in [From, To), oldest first.
type Query struct {
	From, To time.Time
	// Filter is the Elasticsearch-style post_filter, e.g. TermsFilter(...).
	Filter map[string]any
	// Fields limits what Moesif returns; ask only for what is used.
	Fields   []string
	PageSize int
	MaxPages int
}

// Hit is one event.
type Hit struct {
	ID     string
	Source map[string]any
}

// TermsFilter matches events whose field is any of values.
func TermsFilter(field string, values []string) map[string]any {
	return map[string]any{"bool": map[string]any{"must": []map[string]any{
		{"terms": map[string]any{field: values}},
	}}}
}

// SearchEvents pages through the matching events. more is true when MaxPages
// was reached before the window was exhausted.
func (c *Client) SearchEvents(ctx context.Context, q Query) (hits []Hit, more bool, err error) {
	size, pages := q.PageSize, q.MaxPages
	if size <= 0 {
		size = defaultPageSize
	}
	if pages <= 0 {
		pages = defaultMaxPages
	}
	var after []any
	for page := 0; page < pages; page++ {
		got, err := c.searchPage(ctx, q, size, after)
		if err != nil {
			return nil, false, err
		}
		for _, h := range got {
			hits = append(hits, Hit{ID: h.ID, Source: h.Source})
		}
		if len(got) < size {
			return hits, false, nil
		}
		if after = got[len(got)-1].Sort; len(after) == 0 {
			return hits, false, nil
		}
	}
	return hits, true, nil
}

type rawHit struct {
	ID     string         `json:"_id"`
	Source map[string]any `json:"_source"`
	Sort   []any          `json:"sort"`
}

func (c *Client) searchPage(ctx context.Context, q Query, size int, after []any) ([]rawHit, error) {
	body := map[string]any{
		"post_filter": q.Filter,
		"size":        size,
		"sort":        []map[string]any{{"request.time": map[string]any{"order": "asc", "unmapped_type": "long"}}},
	}
	if len(q.Fields) > 0 {
		body["_source"] = q.Fields
	}
	if after != nil {
		body["search_after"] = after
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("moesif: encode query: %w", err)
	}
	endpoint := c.baseURL + "/v1/search/~/search/events?" + url.Values{
		"from": {q.From.UTC().Format(timeLayout)},
		"to":   {q.To.UTC().Format(timeLayout)},
	}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("moesif: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse))
	if err != nil {
		return nil, fmt.Errorf("%w: read response: %v", ErrUnavailable, err)
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError {
		return nil, fmt.Errorf("%w: status %d", ErrUnavailable, resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("moesif: event search returned %d", resp.StatusCode)
	}

	var parsed struct {
		Hits struct {
			Hits []rawHit `json:"hits"`
		} `json:"hits"`
	}
	// UseNumber keeps sort keys exact when echoed as search_after.
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&parsed); err != nil {
		return nil, errors.New("moesif: event search response is unreadable")
	}
	return parsed.Hits.Hits, nil
}

// Text reads a dotted path from an event, as a literal key first, then nested.
func Text(src map[string]any, path string) string {
	if v, ok := src[path].(string); ok {
		return strings.TrimSpace(v)
	}
	var current any = src
	for _, part := range strings.Split(path, ".") {
		obj, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = obj[part]
	}
	v, _ := current.(string)
	return strings.TrimSpace(v)
}

// ParseTime reads an event's request.time: RFC 3339, or no zone meaning UTC.
func ParseTime(s string) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999999"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}
