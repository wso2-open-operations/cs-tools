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

// Package productconsumption is the HTTP client for the upstream
// product-consumption service(s) — separate services (not entity-service)
// that provision WSO2 API Manager applications/subscriptions/credentials to
// generate per-deployment product licenses, and import deployment usage
// data. The subscription/license API and the usage-tracking API are two
// independently-configurable base URLs — they may happen to be the same
// host in some deployments, but are not guaranteed to be, so this package
// keeps them as two distinct fields rather than assuming they're always the
// same host.
package productconsumption

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

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/apierror"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

var tokenFetchTimeout = 10 * time.Second

const maxResponseBodyBytes = 10 << 20 // 10 MiB

func noRedirect(_ *http.Request, _ []*http.Request) error {
	return http.ErrUseLastResponse
}

// Config holds the configuration for the product-consumption service
// clients. TrackingBaseURL defaults to SubscriptionBaseURL when left empty
// — set it explicitly once the two services are hosted separately.
type Config struct {
	SubscriptionBaseURL string
	TrackingBaseURL     string
	TokenURL            string
	ClientID            string
	ClientSecret        string
	Scopes              []string
}

// Client is an HTTP client for the upstream product-consumption service(s),
// authenticated via the OAuth2 client credentials grant. subscriptionBaseURL
// backs the subscription/license API; trackingBaseURL backs the
// usage-import API — two independently-configurable upstream services,
// kept distinct here even though they're often the same host.
type Client struct {
	http                *http.Client
	subscriptionBaseURL string
	trackingBaseURL     string
}

// NewClient constructs a Client that authenticates against the
// product-consumption service(s) using the OAuth2 client credentials grant type.
func NewClient(cfg Config) *Client {
	cc := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
		Scopes:       cfg.Scopes,
	}

	tokenCtx := context.WithValue(context.Background(), oauth2.HTTPClient,
		&http.Client{Timeout: tokenFetchTimeout, CheckRedirect: noRedirect})

	// clientcredentials.Config.Client's returned *http.Client and its
	// Transport must not be mutated (see the package doc comment) — wrap its
	// Transport in a fresh http.Client we own instead of setting fields
	// directly on the returned one.
	oauthClient := cc.Client(tokenCtx)
	httpClient := &http.Client{
		Transport:     oauthClient.Transport,
		Timeout:       300 * time.Second,
		CheckRedirect: noRedirect,
	}

	trackingBaseURL := cfg.TrackingBaseURL
	if trackingBaseURL == "" {
		trackingBaseURL = cfg.SubscriptionBaseURL
	}

	return &Client{
		http:                httpClient,
		subscriptionBaseURL: strings.TrimRight(cfg.SubscriptionBaseURL, "/"),
		trackingBaseURL:     strings.TrimRight(trackingBaseURL, "/"),
	}
}

func (c *Client) doAt(ctx context.Context, baseURL, method, path, contentType string, body []byte) ([]byte, error) {
	var reqBody io.Reader
	if len(body) > 0 {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("productconsumption: build request %s %s: %w", method, path, err)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("productconsumption: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, maxResponseBodyBytes+1)
	respBody, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("productconsumption: read response body: %w", err)
	}
	if len(respBody) > maxResponseBodyBytes {
		return nil, fmt.Errorf("productconsumption: %s %s: response body exceeds %d bytes", method, path, maxResponseBodyBytes)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, apierror.NewUpstreamError(resp.StatusCode, respBody)
	}

	return respBody, nil
}

func (c *Client) do(ctx context.Context, method, path, contentType string, body []byte) ([]byte, error) {
	return c.doAt(ctx, c.subscriptionBaseURL, method, path, contentType, body)
}

func (c *Client) postJSON(ctx context.Context, path string, reqBody, out any) error {
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("productconsumption: encode request for POST %s: %w", path, err)
	}
	body, err := c.do(ctx, http.MethodPost, path, "application/json", payload)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("productconsumption: decode response for POST %s: %w", path, err)
	}
	return nil
}

// postJSONTracking is postJSON's counterpart for the usage-tracking service
// — the only consumer of trackingBaseURL.
func (c *Client) postJSONTracking(ctx context.Context, path string, reqBody, out any) error {
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("productconsumption: encode request for POST %s: %w", path, err)
	}
	body, err := c.doAt(ctx, c.trackingBaseURL, http.MethodPost, path, "application/json", payload)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("productconsumption: decode response for POST %s: %w", path, err)
	}
	return nil
}

func pathEscape(s string) string {
	return url.PathEscape(s)
}
