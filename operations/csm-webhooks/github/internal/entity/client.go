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

// Package entity forwards a verified GitHub delivery to entity-service.
//
// THE FORWARD IS THE WHOLE POINT OF THE SPLIT. This component is public so
// GitHub can reach it, and therefore holds no database credentials and makes
// no decisions: it proves the delivery is genuinely from GitHub, then hands
// it to entity-service over the internal network, authenticated as an
// internal client. Everything that reads or writes CSM data stays behind
// that boundary.
package entity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// Delivery is what entity-service's internal endpoint accepts: the two
// headers that identify a delivery, plus the body exactly as GitHub sent it.
//
// Payload is json.RawMessage rather than a parsed struct DELIBERATELY. This
// component does not model GitHub's payload and must not: every field it
// decoded would be a field to keep in step with entity-service's own view of
// the same JSON.
type Delivery struct {
	ID      string          `json:"id"`
	Event   string          `json:"event"`
	Payload json.RawMessage `json:"payload"`
}

// Config holds the entity-service connection settings.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// Client posts verified deliveries to entity-service.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient constructs the client. The OAuth2 client-credentials token is
// fetched and refreshed by the transport, so callers never handle it.
func NewClient(cfg Config) *Client {
	cc := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
		Scopes:       cfg.Scopes,
	}
	// The token fetch gets its own bounded client. cc.Client stores this
	// context in the token source, and the transport fetches the token
	// BEFORE it sends the delivery -- so without oauth2.HTTPClient here that
	// fetch runs on http.DefaultClient, which has no timeout at all. The 30s
	// below applies only to the delivery request, never to the token one, and
	// Deliver's own ctx cannot reach the token source either. A token
	// endpoint that accepts the connection and then stalls would hold the
	// webhook handler open indefinitely.
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, &http.Client{
		Timeout: 15 * time.Second,
	})
	httpClient := cc.Client(ctx)
	httpClient.Timeout = 30 * time.Second
	return &Client{http: httpClient, baseURL: cfg.BaseURL}
}

// ErrDuplicate reports a delivery entity-service has already applied. It is
// not a failure: GitHub redelivers, and the second attempt succeeded the
// first time.
var ErrDuplicate = fmt.Errorf("entity: duplicate delivery")

// Deliver forwards one verified delivery and returns entity-service's own
// outcome JSON, so this component adds nothing to the response GitHub sees
// beyond passing it along.
func (c *Client) Deliver(ctx context.Context, d Delivery) ([]byte, error) {
	body, err := json.Marshal(d)
	if err != nil {
		return nil, fmt.Errorf("entity: encode delivery: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/github/deliveries", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("entity: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("entity: post delivery: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	out, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("entity: read response: %w", err)
	}
	// 409 is entity-service saying it has seen this delivery id before.
	if resp.StatusCode == http.StatusConflict {
		return out, ErrDuplicate
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return out, fmt.Errorf("entity: delivery returned %d", resp.StatusCode)
	}
	return out, nil
}
