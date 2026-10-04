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

// Package announcementpublish is a narrow entity-service client for the
// automatic-publish path: it finds every announcement_requests row whose
// scheduled_on has arrived, then triggers entity-service's own in-process
// fan-out for each one. Unlike internal/entitycases, this client never
// creates a case or attaches a tag itself — that logic lives entirely in
// entity-service's AnnouncementRequestService.AutoPublish, in-process there,
// specifically because CaseService.CreateCase/AddCaseTag both hard-require a
// real browser user's x-user-id-token to resolve who's acting, which this
// component (an OAuth2 client-credentials caller with no user session at
// all) never has. See that service's own doc comment for the full reasoning.
package announcementpublish

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/entityhttp"
)

// autoPublishTimeout bounds the single AutoPublish HTTP call this client
// makes -- applied per-call via context.WithTimeout in AutoPublish below,
// not as this client's shared http.Client.Timeout, since that field would
// apply to every request this client makes, including SearchDueIDs (see
// searchTimeout below), which needs a much shorter budget. See its own use
// for why this one is minutes rather than the usual few-seconds REST-call
// budget. Kept above entity-service's own total server-side budget
// (currently 11m30s: autoPublishHandlerTimeout + autoPublishWriteDeadlineBuffer,
// announcement_request_handler.go) so this client is never the one giving up
// first, and below both that service's own autoPublishClaimStaleAfter (13m)
// and this task's own ~15-minute tick cadence.
var autoPublishTimeout = 12 * time.Minute

// searchTimeout bounds each SearchDueIDs page request -- a plain, fast REST
// call (unlike AutoPublish), so it keeps the usual short budget rather than
// inheriting autoPublishTimeout's multi-minute one.
var searchTimeout = 25 * time.Second

// Config holds this client's configuration.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
	// Transport, when set, is a shared entityhttp transport; when nil the
	// client builds its own from the credential fields.
	Transport http.RoundTripper
}

// Client is a narrow HTTP client for entity-service's announcement-request
// search and auto-publish endpoints — kept as its own package for the same
// reason internal/entitycases is separate from internal/ledger: a different
// concern that happens to point at the same entity-service deployment.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient constructs a Client authenticated via the OAuth2 client
// credentials grant. cfg.TokenURL/cfg.BaseURL must both be https (loopback
// http is allowed for local development — see httpsec.RequireSecureURL)
// since both carry credentials or a bearer token. The resulting client's
// Bearer token must satisfy entity-service's own AUTH_INTERNAL_CLIENT_IDS —
// AutoPublish rejects any other caller with a ForbiddenError.
//
// No blanket client timeout (0): SearchDueIDs and AutoPublish need very
// different budgets (see searchTimeout/autoPublishTimeout above), applied
// per call via context.WithTimeout in each method instead.
func NewClient(cfg Config) (*Client, error) {
	httpClient, err := entityhttp.ClientFor(cfg.Transport, entityhttp.Credentials{
		TokenURL: cfg.TokenURL, ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret, Scopes: cfg.Scopes,
	}, cfg.BaseURL, 0)
	if err != nil {
		return nil, fmt.Errorf("announcementpublish: %w", err)
	}
	return &Client{http: httpClient, baseURL: entityhttp.TrimBase(cfg.BaseURL)}, nil
}

// do executes an authenticated request — see entityhttp.Do.
func (c *Client) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	return entityhttp.Do(ctx, c.http, c.baseURL, "announcementpublish", method, path, body)
}

// searchPageSize/maxSearchPages mirror internal/entitycases' own bounded
// "fetch everything" convention — see that package's own doc comment on
// searchCases for the full reasoning (a safety net against an unexpectedly
// large or malformed result turning one tick into an unbounded loop).
const (
	searchPageSize = 50
	maxSearchPages = 20
)

type searchAnnouncementRequestsPayload struct {
	ReadyForScheduledPublish bool       `json:"readyForScheduledPublish"`
	Pagination               pagination `json:"pagination"`
}

type pagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type announcementRequestView struct {
	ID string `json:"id"`
}

type searchAnnouncementRequestsResponse struct {
	Requests []announcementRequestView `json:"requests"`
	Total    int                       `json:"total"`
}

// SearchDueIDs returns the id of every announcement_requests row that is
// approved, scheduled, and whose scheduled_on has already arrived — see
// entity-service's SearchAnnouncementRequestsRequest.ReadyForScheduledPublish
// for the exact query this maps to. Only ids are read: PublishDue re-fetches
// each one's full detail immediately before acting on it, so this doesn't
// need (or trust) anything more than "these are the candidates right now."
func (c *Client) SearchDueIDs(ctx context.Context) ([]string, error) {
	var ids []string
	offset := 0
	for page := 0; page < maxSearchPages; page++ {
		reqBody, err := json.Marshal(searchAnnouncementRequestsPayload{
			ReadyForScheduledPublish: true,
			Pagination:               pagination{Limit: searchPageSize, Offset: offset},
		})
		if err != nil {
			return nil, fmt.Errorf("announcementpublish: encode search request: %w", err)
		}

		pageCtx, cancel := context.WithTimeout(ctx, searchTimeout)
		respBody, err := c.do(pageCtx, http.MethodPost, "/announcement-requests/search", reqBody)
		cancel()
		if err != nil {
			return nil, err
		}
		var resp searchAnnouncementRequestsResponse
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return nil, fmt.Errorf("announcementpublish: decode search response: %w", err)
		}

		for _, r := range resp.Requests {
			ids = append(ids, r.ID)
		}

		offset += len(resp.Requests)
		if entityhttp.PageDone(len(resp.Requests), offset, resp.Total, searchPageSize) {
			return ids, nil
		}
		if page == maxSearchPages-1 {
			return nil, fmt.Errorf("announcementpublish: due-request search exceeds the %d-row pagination cap (total=%d) — refusing to silently miss some",
				maxSearchPages*searchPageSize, resp.Total)
		}
	}
	// Unreachable: every iteration above returns before the loop can exit on
	// its own condition. Required only because the compiler can't prove that.
	return ids, nil
}

// AutoPublish calls POST /announcement-requests/{id}/auto-publish — see
// entity-service's AnnouncementRequestService.AutoPublish for what this
// actually does. Returns the raw *apierror.Error on a 409 (not yet fully
// delivered this pass — expected on a partial failure, the caller decides
// whether that's a Handler-level failure worth alerting on) or any other
// non-2xx status.
func (c *Client) AutoPublish(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, autoPublishTimeout)
	defer cancel()
	_, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/announcement-requests/%s/auto-publish", url.PathEscape(id)), nil)
	return err
}
