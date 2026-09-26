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

// Package slaengine is the SLA breach-alerting engine: it periodically polls
// entity-service's GET /sla-status — which reads live from the "sla" table
// ServiceNow's own SLA engine populates via sync, not a value this service
// computes — diffs each clock's businessElapsedPercent against the last
// tier this engine alerted for (tracked in Redis, see redis.go), and sends
// a Google Chat breach alert plus events.TypeSLATierReached the first time a
// 50%/75%/100% checkpoint is crossed (see engine.go). Replaces an earlier
// design that hand-registered a durable clock per case on a now-removed
// entity-service "sla_clocks" table (a stand-in built before the real,
// ServiceNow-synced "sla" table existed) and scheduled wake-ups off a
// locally-computed due date — see entity-service's own CLAUDE.md ("SLA
// status") for the full history. This service still has no database of its
// own, by design (see this package's own CLAUDE.md section) — Redis here
// holds only the small "last alerted tier per clock" cursor, not the SLA
// data itself.
package slaengine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/oauthhttp"
)

// EntityConfig holds the configuration for the entity-service client below.
// BaseURL/Scopes are this client's own SLA_ENTITY_* env vars; TokenURL/
// ClientID/ClientSecret are filled by cmd/server/main.go from whichever
// OAuth2 app is appropriate for this deployment — unlike
// internal/entity.CustomerEntityConfig, there's no existing shared-app
// precedent to follow here since this is a new, independent capability, so
// main.go is free to point it at the same shared OAUTH2_* app or a
// dedicated one.
type EntityConfig struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// EntityClient is a narrow HTTP client for entity-service's GET /sla-status
// endpoint — the only entity-service call this engine makes; unlike the
// design it replaced, there's no register/get/patch trio to maintain since
// this service no longer owns any SLA state of its own. Mirrors
// internal/entity.CustomerEntityClient's do()/OAuth2 shape exactly.
type EntityClient struct {
	http    *http.Client
	baseURL string
}

// NewEntityClient constructs an EntityClient authenticated via the OAuth2
// client credentials grant. Never fails and never contacts the token
// endpoint — a missing/invalid configuration only surfaces as an error the
// first time a method below is called.
func NewEntityClient(cfg EntityConfig) *EntityClient {
	httpClient := oauthhttp.NewClient(oauthhttp.Config{
		TokenURL:     cfg.TokenURL,
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Scopes:       cfg.Scopes,
	})

	return &EntityClient{
		http:    httpClient,
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
	}
}

// do executes an authenticated HTTP request against entity-service and
// returns the raw JSON response body, or an *apierror.Error for a non-2xx
// status.
func (c *EntityClient) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	var reqBody io.Reader
	if len(body) > 0 {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("slaengine: build request %s %s: %w", method, path, err)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("slaengine: %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("slaengine: read response body: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, &apierror.Error{StatusCode: resp.StatusCode, Body: string(respBody)}
	}
	return respBody, nil
}

// SLAStatus mirrors entity-service's domain.SLAStatus — one case-like work
// item's current standing against one SLA policy target (response/
// workaround/resolution), read live from entity-service's own "sla" table.
// Field names/JSON tags match entity-service's response exactly.
type SLAStatus struct {
	CaseID                 string     `json:"caseId"`
	ClockType              string     `json:"clockType"`
	BusinessElapsedPercent float64    `json:"businessElapsedPercent"`
	HasBreached            bool       `json:"hasBreached"`
	IsPaused               bool       `json:"isPaused"`
	StartedOn              *time.Time `json:"startedOn"`
	CaseNumber             string     `json:"caseNumber,omitempty"`
	WSO2CaseID             string     `json:"wso2CaseId,omitempty"`
	CaseTitle              string     `json:"caseTitle,omitempty"`
	CaseType               string     `json:"caseType,omitempty"`
	Product                string     `json:"product,omitempty"`
	Team                   string     `json:"team,omitempty"`
	Priority               string     `json:"priority,omitempty"`
	State                  string     `json:"state,omitempty"`
}

// searchSLAStatusResponse mirrors entity-service's domain.SearchSLAStatusResponse.
type searchSLAStatusResponse struct {
	Statuses []SLAStatus `json:"statuses"`
	Total    int         `json:"total"`
	Limit    int         `json:"limit"`
	Offset   int         `json:"offset"`
}

// listPageSize is the page size this client requests per GET /sla-status
// call — entity-service's own maxSLAStatusLimit (2000), so a full poll of
// today's ~5,500 active clocks takes about three round trips rather than
// the generic endpoint's would-be hundred-plus.
const listPageSize = 2000

// ListActiveSLAStatuses calls GET /sla-status?limit=&offset=.
func (c *EntityClient) ListActiveSLAStatuses(ctx context.Context, limit, offset int) (searchSLAStatusResponse, error) {
	path := "/sla-status?limit=" + url.QueryEscape(strconv.Itoa(limit)) + "&offset=" + url.QueryEscape(strconv.Itoa(offset))
	respBody, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return searchSLAStatusResponse{}, err
	}
	var resp searchSLAStatusResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return searchSLAStatusResponse{}, fmt.Errorf("slaengine: decode ListActiveSLAStatuses response: %w", err)
	}
	return resp, nil
}

// FetchAllActiveSLAStatuses pages through every currently-active SLA clock
// across every case-like work item, via repeated ListActiveSLAStatuses
// calls — Engine.Tick's one entry point into entity-service per poll.
func (c *EntityClient) FetchAllActiveSLAStatuses(ctx context.Context) ([]SLAStatus, error) {
	var all []SLAStatus
	offset := 0
	for {
		page, err := c.ListActiveSLAStatuses(ctx, listPageSize, offset)
		if err != nil {
			return nil, fmt.Errorf("slaengine: fetch active sla statuses at offset %d: %w", offset, err)
		}
		all = append(all, page.Statuses...)
		offset += len(page.Statuses)
		if len(page.Statuses) < listPageSize || offset >= page.Total {
			return all, nil
		}
	}
}
