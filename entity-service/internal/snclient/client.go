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

// Package snclient provides an HTTP client for the Choreo ServiceNow API.
package snclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

// Client is a thin HTTP wrapper around the Choreo ServiceNow API base URL.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New constructs a Client with a default 15-second timeout.
func New(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Post sends a POST request to the given path, attaching the caller-supplied
// userIDToken as the x-user-id-token header. It returns the raw response body
// on 2xx; non-2xx responses are mapped to typed apierror values.
func (c *Client) Post(ctx context.Context, path string, userIDToken string, payload any) (json.RawMessage, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("snclient: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("snclient: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-user-id-token", userIDToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, err
		}
		return nil, &apierror.ServiceUnavailableError{Msg: fmt.Sprintf("snclient: %s: %v", path, err)}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("snclient: read response body: %w", err)
	}

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return json.RawMessage(raw), nil
	case resp.StatusCode == http.StatusBadRequest:
		return nil, &apierror.ValidationError{Msg: "downstream service rejected the request"}
	case resp.StatusCode == http.StatusUnauthorized:
		return nil, &apierror.UnauthorizedError{Msg: "invalid or missing x-user-id-token"}
	case resp.StatusCode == http.StatusForbidden:
		return nil, &apierror.ForbiddenError{Msg: "not authorized to access this resource"}
	case resp.StatusCode == http.StatusNotFound:
		return nil, &apierror.NotFoundError{Msg: "resource not found in downstream service"}
	case resp.StatusCode == http.StatusServiceUnavailable:
		return nil, &apierror.ServiceUnavailableError{Msg: "downstream service unavailable"}
	default:
		return nil, fmt.Errorf("snclient: %s: unexpected status %d: %s", path, resp.StatusCode, raw)
	}
}
