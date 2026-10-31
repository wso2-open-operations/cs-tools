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

// Package incidentautoclose closes incidents that have been Resolved for a set
// number of days: the Postgres port of ServiceNow's hourly incident auto-close.
// On WSO2's instance that job ran at :00:01 every hour and closed an incident
// 14 days after its resolved_at, as the incident's resolver, with no comment and
// no work note.
//
// *** A TRIGGER, NOT A DECIDER. *** Which incidents are due, the row lock, the
// re-check, the close itself and its task cascade all happen in entity-service
// (POST /incidents/autoclose, internal/service/incident_autoclose_service.go).
// This package asks for one page at a time and keeps asking while entity-service
// says the page was full. The
// endpoint is internal-only and Postgres-only: this component's client id must
// be in entity-service's M2M_CLIENT_IDS, and an entity-service running on
// DATA_SOURCE=servicenow answers 503, which fails the run.
package incidentautoclose

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/httpsec"
)

// tokenFetchTimeout is the HTTP client timeout for token-endpoint requests.
var tokenFetchTimeout = 10 * time.Second

// callTimeout bounds one page. The binding limit is entity-service's own:
// REQUEST_TIMEOUT (60s by default) cancels the request, and a sweep cut short
// answers with an error and no closed/failed lists, so waiting longer here gains
// nothing. That error fails the run, but loses no work: each incident commits
// on its own, and the ones not reached are still due next run. PageSize is
// sized to finish well inside it.
var callTimeout = 60 * time.Second

// ClientConfig holds this client's configuration.
type ClientConfig struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// Client calls entity-service's POST /incidents/autoclose with the same OAuth2
// client-credentials grant and httpsec guards as the other clients here.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient constructs a Client. Both URLs must be https (loopback http is
// allowed for local development -- see httpsec.RequireSecureURL).
func NewClient(cfg ClientConfig) (*Client, error) {
	if err := httpsec.RequireSecureURL(cfg.TokenURL); err != nil {
		return nil, fmt.Errorf("incidentautoclose: token URL: %w", err)
	}
	if err := httpsec.RequireSecureURL(cfg.BaseURL); err != nil {
		return nil, fmt.Errorf("incidentautoclose: base URL: %w", err)
	}
	cc := clientcredentials.Config{
		ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret, TokenURL: cfg.TokenURL, Scopes: cfg.Scopes,
	}
	tokenHTTP := &http.Client{Timeout: tokenFetchTimeout}
	httpsec.RejectInsecureRedirects(tokenHTTP)
	httpClient := cc.Client(context.WithValue(context.Background(), oauth2.HTTPClient, tokenHTTP))
	httpsec.RejectInsecureRedirects(httpClient)
	return &Client{http: httpClient, baseURL: strings.TrimRight(cfg.BaseURL, "/")}, nil
}

// Request is the body of POST /incidents/autoclose.
type Request struct {
	ResolvedForDays int  `json:"resolvedForDays"`
	Limit           int  `json:"limit,omitempty"`
	DryRun          bool `json:"dryRun,omitempty"`
}

// Closed is one incident closed (or, in a dry run, that would be).
type Closed struct {
	ID         string `json:"id"`
	Number     string `json:"number"`
	ResolvedOn string `json:"resolvedOn"`
}

// Failed is one incident entity-service could not close; the others on the
// page were still attempted.
type Failed struct {
	ID     string `json:"id"`
	Number string `json:"number"`
	Error  string `json:"error"`
}

// Response is entity-service's answer for one page. More is true when the page
// was full, so there may be more due.
type Response struct {
	DryRun bool     `json:"dryRun"`
	Closed []Closed `json:"closed"`
	Failed []Failed `json:"failed"`
	More   bool     `json:"more"`
}

// AutoClose asks entity-service to close one page of due incidents.
func (c *Client) AutoClose(ctx context.Context, req Request) (Response, error) {
	const path = "/incidents/autoclose"
	var out Response
	body, err := json.Marshal(req)
	if err != nil {
		return out, fmt.Errorf("incidentautoclose: encode request: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return out, fmt.Errorf("incidentautoclose: build request POST %s: %w", path, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return out, fmt.Errorf("incidentautoclose: POST %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return out, fmt.Errorf("incidentautoclose: read response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return out, &apierror.Error{StatusCode: resp.StatusCode, Body: string(raw)}
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("incidentautoclose: decode response: %w", err)
	}
	return out, nil
}
