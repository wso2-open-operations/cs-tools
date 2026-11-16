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
// Package auditlog is the daily upkeep of entity-service's field-level audit
// log: it asks entity-service to create the coming monthly partitions, check the
// log's health and put back audit triggers that went missing, and turns what
// entity-service found into a task failure, so the engine retries and mails it.
//
// *** A TRIGGER, NOT A CALCULATOR. *** The work runs in the database, behind
// entity-service's POST /audit-log/maintenance. This component holds no
// database credentials. Same split as internal/usagerollup.
package auditlog

import (
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

var tokenFetchTimeout = 10 * time.Second

// maintenanceTimeout bounds one call. The upkeep is a few quick statements.
const maintenanceTimeout = 2 * time.Minute

// Config holds this client's configuration.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// Client calls entity-service's audit log upkeep endpoint. It accepts service
// credentials only: this component's client id must be in entity-service's
// M2M_CLIENT_IDS.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient constructs a Client authenticated via the client credentials grant.
func NewClient(cfg Config) (*Client, error) {
	if err := httpsec.RequireSecureURL(cfg.TokenURL); err != nil {
		return nil, fmt.Errorf("auditlog: token URL: %w", err)
	}
	if err := httpsec.RequireSecureURL(cfg.BaseURL); err != nil {
		return nil, fmt.Errorf("auditlog: base URL: %w", err)
	}
	cc := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
		Scopes:       cfg.Scopes,
	}
	tokenHTTP := &http.Client{Timeout: tokenFetchTimeout}
	httpsec.RejectInsecureRedirects(tokenHTTP)
	httpClient := cc.Client(context.WithValue(context.Background(), oauth2.HTTPClient, tokenHTTP))
	httpClient.Timeout = maintenanceTimeout
	httpsec.RejectInsecureRedirects(httpClient)

	return &Client{http: httpClient, baseURL: strings.TrimRight(cfg.BaseURL, "/")}, nil
}

// TriggerProblem is one mismatch between the audit registry and the triggers.
type TriggerProblem struct {
	Schema  string `json:"schema"`
	Table   string `json:"table"`
	Problem string `json:"problem"`
}

// Result is what one run of the upkeep found, as entity-service returns it.
type Result struct {
	PartitionsCreated  int              `json:"partitionsCreated"`
	HealthProblems     []string         `json:"healthProblems"`
	RepairedTriggers   []TriggerProblem `json:"repairedTriggers"`
	UnresolvedTriggers []TriggerProblem `json:"unresolvedTriggers"`
	// StepErrors names each step that could not run at all.
	StepErrors []string `json:"stepErrors"`
}

// RunMaintenance runs the upkeep once. Safe to repeat. The error is set when the
// call itself failed; problems the upkeep found, and steps that could not run,
// are in the Result.
func (c *Client) RunMaintenance(ctx context.Context) (Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/audit-log/maintenance", nil)
	if err != nil {
		return Result{}, fmt.Errorf("auditlog: build request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("auditlog: POST /audit-log/maintenance: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Result{}, fmt.Errorf("auditlog: read response body: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return Result{}, &apierror.Error{StatusCode: resp.StatusCode, Body: string(body)}
	}

	var result Result
	if err := json.Unmarshal(body, &result); err != nil {
		return Result{}, fmt.Errorf("auditlog: decode response: %w", err)
	}
	return result, nil
}
