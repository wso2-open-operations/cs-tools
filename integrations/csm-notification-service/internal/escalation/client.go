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

package escalation

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

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/apierror"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// tokenFetchTimeout is the HTTP client timeout for token-endpoint requests.
// Overridden in tests to keep them fast.
var tokenFetchTimeout = 10 * time.Second

// EntityConfig holds the configuration for the entity-service client below.
// Same shape and same credential story as slaengine.EntityConfig: BaseURL and
// Scopes are this client's own env vars, while TokenURL/ClientID/ClientSecret
// are filled by cmd/server/main.go from the shared OAUTH2_* app.
type EntityConfig struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// EntityClient is a narrow entity-service client with exactly one job:
// appending the execution summary to an incident as a work note (section
// 11.0). A deliberate second client rather than a method on
// slaengine.EntityClient — that one exists for the sla_clocks endpoints, which
// have nothing to do with incidents.
type EntityClient struct {
	http    *http.Client
	baseURL string
}

// NewEntityClient constructs an EntityClient authenticated via the OAuth2
// client credentials grant. Never fails and never contacts the token endpoint
// — a missing or invalid configuration only surfaces as an error the first
// time AppendWorkNote is called.
func NewEntityClient(cfg EntityConfig) *EntityClient {
	cc := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
		Scopes:       cfg.Scopes,
	}

	tokenCtx := context.WithValue(context.Background(), oauth2.HTTPClient,
		&http.Client{Timeout: tokenFetchTimeout})
	httpClient := cc.Client(tokenCtx)
	httpClient.Timeout = 25 * time.Second

	return &EntityClient{
		http:    httpClient,
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
	}
}

// updateIncidentRequest is the subset of entity-service's
// PATCH /incidents/{id} body this client sends. Work notes only, and that
// matters twice over for loop safety:
//
//   - A PATCH touching neither state nor priority publishes no escalation
//     signal (see entity-service's publishEscalationSignals), so writing a
//     summary cannot come back as a new trigger or an acknowledgement.
//   - incident.comment_added is published by CreateComment, a different
//     endpoint this client never calls — and even if it were reached, a work
//     note carries IsPublic false, which the engine ignores.
//
// So the engine writing its own execution summary can never cancel a ladder,
// including the one it is writing the summary for.
type updateIncidentRequest struct {
	WorkNotes string `json:"workNotes"`
}

// AppendWorkNote writes note onto the incident as a work note. ServiceNow
// work-note fields are journals — a write appends an entry rather than
// replacing the field — so this is additive despite being a PATCH.
func (c *EntityClient) AppendWorkNote(ctx context.Context, incidentID, note string) error {
	if strings.TrimSpace(incidentID) == "" {
		return fmt.Errorf("escalation: incidentId is required")
	}
	body, err := json.Marshal(updateIncidentRequest{WorkNotes: note})
	if err != nil {
		return fmt.Errorf("escalation: encode work note: %w", err)
	}
	_, err = c.do(ctx, http.MethodPatch, "/incidents/"+url.PathEscape(incidentID), body)
	return err
}

// do executes an authenticated HTTP request against entity-service and returns
// the raw JSON response body, or an *apierror.Error for a non-2xx status.
// Mirrors slaengine.EntityClient.do exactly.
func (c *EntityClient) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	var reqBody io.Reader
	if len(body) > 0 {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("escalation: build request %s %s: %w", method, path, err)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("escalation: %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("escalation: read response body: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, &apierror.Error{StatusCode: resp.StatusCode, Body: string(respBody)}
	}
	return respBody, nil
}
