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

// Package servicenow is a client for SupportPortalLite's ServiceNow
// integration: the Table API (GET /api/now/table/{table}) plus a custom
// scoped-app REST API on the same host (e.g. GET
// /api/wso2/wso2_team_schedule/schedule), both Basic-Auth-protected. Ported
// from the Ballerina backend's modules/operations package (its single
// snClient, shared by every function in that module).
package servicenow

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

// Config holds the configuration for the ServiceNow client.
type Config struct {
	// BaseURL is snHost in the Ballerina config — the ServiceNow instance
	// root, e.g. "https://backing-system.example.com" (configuration only; there is
	// no default). Both the Table API and the
	// custom scoped-app API live under this same host.
	BaseURL  string
	Username string
	Password string

	// EscalationTemplateID is escalationTemplateId in the Ballerina config —
	// the "type" field value ServiceNow expects when creating a new
	// escalation record (see Client.EscalateCase).
	EscalationTemplateID string
	// TeamScheduleURL is teamScheduleUrl in the Ballerina config — a base
	// URL an integration-CS-team sys_id is appended to when building
	// AccountDetails.IntegrationCSTeamScheduleURL, and also returned
	// verbatim as ABTTeamScheduleData's snURL.
	TeamScheduleURL string
}

// Client is an HTTP client for ServiceNow's Table API and SupportPortalLite's
// custom scoped-app REST API, authenticated via HTTP Basic Auth.
type Client struct {
	http                 *http.Client
	baseURL              string
	username             string
	password             string
	escalationTemplateID string
	teamScheduleURL      string
	// escalationLocks serializes EscalateCase's read-then-write per
	// accountSysID within this process -- see EscalateCase's own doc
	// comment for why.
	escalationLocks sync.Map // accountSysID (string) -> *sync.Mutex
}

// NewClient constructs a ServiceNow Client. Requests are retried up to twice
// on 500/502/503/504/408, mirroring the Ballerina snClient's retryConfig
// (count: 2, on the same status codes).
func NewClient(cfg Config) *Client {
	return &Client{
		http: &http.Client{
			Transport: &retryTransport{base: http.DefaultTransport, maxRetries: 2},
			Timeout:   30 * time.Second,
		},
		baseURL:              strings.TrimRight(cfg.BaseURL, "/"),
		username:             cfg.Username,
		password:             cfg.Password,
		escalationTemplateID: cfg.EscalationTemplateID,
		teamScheduleURL:      cfg.TeamScheduleURL,
	}
}

// lockAccountEscalation returns an unlock func for accountSysID's escalation
// lock, blocking until it's held. Callers must defer the returned func.
func (c *Client) lockAccountEscalation(accountSysID string) func() {
	m, _ := c.escalationLocks.LoadOrStore(accountSysID, &sync.Mutex{})
	mu := m.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// TableQuery performs a GET against the ServiceNow Table API for the given
// table, e.g. TableQuery(ctx, "customer_account", url.Values{"sysparm_query":
// {"number=123"}, "sysparm_limit": {"1"}}) mirrors the Ballerina
// snClient->/api/now/'table/customer_account(sysparm_query=...,
// sysparm_limit=...) call shape. Returns the raw JSON response body.
func (c *Client) TableQuery(ctx context.Context, table string, params url.Values) ([]byte, error) {
	return c.get(ctx, "/api/now/table/"+url.PathEscape(table), params)
}

// TableQueryWithHeaders is TableQuery, additionally returning the upstream
// response headers — needed for endpoints that read ServiceNow's
// "X-Total-Count" pagination header alongside the body (e.g. case search,
// comment/worknote listing).
func (c *Client) TableQueryWithHeaders(ctx context.Context, table string, params url.Values) ([]byte, http.Header, error) {
	resp, err := c.doRaw(ctx, http.MethodGet, "/api/now/table/"+url.PathEscape(table), params, nil)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxUpstreamResponseBytes+1))
	if err != nil {
		return nil, nil, fmt.Errorf("servicenow: read response body: %w", err)
	}
	if len(respBody) > maxUpstreamResponseBytes {
		return nil, nil, fmt.Errorf("servicenow: read response body: response exceeds %d bytes", maxUpstreamResponseBytes)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		const maxErrBody = 256
		excerpt := respBody
		if len(excerpt) > maxErrBody {
			excerpt = excerpt[:maxErrBody]
		}
		return nil, nil, &apierror.Error{StatusCode: resp.StatusCode, Body: string(excerpt)}
	}

	return respBody, resp.Header, nil
}

// TablePatch performs a PATCH against a single ServiceNow Table API record
// identified by its sys_id, e.g. TablePatch(ctx, "sn_customerservice_case",
// caseSysID, body) mirrors the Ballerina
// snClient->/api/now/'table/sn_customerservice_case/[caseSysId].patch(...)
// call shape. Returns the raw JSON response body.
func (c *Client) TablePatch(ctx context.Context, table, sysID string, body []byte) ([]byte, error) {
	path := "/api/now/table/" + url.PathEscape(table) + "/" + url.PathEscape(sysID)
	return c.do(ctx, http.MethodPatch, path, nil, body)
}

// CustomGet performs a GET against SupportPortalLite's custom scoped-app
// REST API, which lives on the same ServiceNow host under its own path
// namespace (e.g. "/api/wso2/wso2_team_schedule/schedule"). path must start
// with "/". Returns the raw JSON response body.
func (c *Client) CustomGet(ctx context.Context, path string, params url.Values) ([]byte, error) {
	return c.get(ctx, path, params)
}

// CustomPost performs a POST with a JSON body against SupportPortalLite's
// custom scoped-app REST API. Returns the raw JSON response body.
func (c *Client) CustomPost(ctx context.Context, path string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, path, nil, body)
}

// maxBinaryResponseBytes bounds how much of a GetBinary response this
// client will hold in memory at once (e.g. an attachment download) --
// without it, a large or malicious upstream response could exhaust backend
// memory. 25 MiB comfortably covers ordinary attachments (documents,
// images) while still failing fast on anything unreasonably large.
const maxBinaryResponseBytes = 25 << 20

// GetBinary performs a GET and returns the raw response body together with
// the upstream Content-Type and Content-Disposition headers, for endpoints
// that return non-JSON binary content (e.g. attachment download).
func (c *Client) GetBinary(ctx context.Context, path string, params url.Values) (body []byte, contentType string, contentDisposition string, err error) {
	resp, err := c.doRaw(ctx, http.MethodGet, path, params, nil)
	if err != nil {
		return nil, "", "", err
	}
	defer resp.Body.Close()

	// Read one byte past the limit so a response that's exactly at the cap
	// isn't mistaken for one that exceeds it.
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxBinaryResponseBytes+1))
	if err != nil {
		return nil, "", "", fmt.Errorf("servicenow: read response body: %w", err)
	}
	if len(respBody) > maxBinaryResponseBytes {
		return nil, "", "", fmt.Errorf("servicenow: response body exceeds %d byte limit", maxBinaryResponseBytes)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		const maxErrBody = 256
		excerpt := respBody
		if len(excerpt) > maxErrBody {
			excerpt = excerpt[:maxErrBody]
		}
		return nil, "", "", &apierror.Error{StatusCode: resp.StatusCode, Body: string(excerpt)}
	}

	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "application/octet-stream"
	}
	return respBody, ct, resp.Header.Get("Content-Disposition"), nil
}

func (c *Client) get(ctx context.Context, path string, params url.Values) ([]byte, error) {
	return c.do(ctx, http.MethodGet, path, params, nil)
}

func (c *Client) do(ctx context.Context, method, path string, params url.Values, body []byte) ([]byte, error) {
	resp, err := c.doRaw(ctx, method, path, params, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxUpstreamResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("servicenow: read response body: %w", err)
	}
	if len(respBody) > maxUpstreamResponseBytes {
		return nil, fmt.Errorf("servicenow: read response body: response exceeds %d bytes", maxUpstreamResponseBytes)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		const maxErrBody = 256
		excerpt := respBody
		if len(excerpt) > maxErrBody {
			excerpt = excerpt[:maxErrBody]
		}
		return nil, &apierror.Error{StatusCode: resp.StatusCode, Body: string(excerpt)}
	}

	return respBody, nil
}

func (c *Client) doRaw(ctx context.Context, method, path string, params url.Values, body []byte) (*http.Response, error) {
	reqURL := c.baseURL + path
	if len(params) > 0 {
		reqURL += "?" + params.Encode()
	}

	var reqBody io.Reader
	if len(body) > 0 {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("servicenow: build request %s %s: %w", method, path, err)
	}
	req.SetBasicAuth(c.username, c.password)
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("servicenow: %s %s: %w", method, path, err)
	}
	return resp, nil
}

// retryTransport retries a request up to maxRetries times when the upstream
// response status is one Ballerina's snClient retryConfig also retries on:
// 500, 502, 503, 504, and 408. Only requests with no body, or a body that
// can be safely re-read (http.Request.GetBody set, which
// http.NewRequestWithContext populates automatically for the []byte-backed
// readers this package uses), are retried -- and only when the method is
// idempotent (isIdempotentMethod): retrying a POST that creates a record
// (e.g. createNewEscalation) or a PATCH that changes a record (e.g.
// linkCaseToEscalation) risks a second create or update when the
// first attempt actually succeeded upstream but the response was lost or
// timed out. retryBackoff is a short pause between attempts rather than an
// immediate retry, giving a transient upstream hiccup a moment to clear.
type retryTransport struct {
	base       http.RoundTripper
	maxRetries int
}

const retryBackoff = 200 * time.Millisecond

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if !isIdempotentMethod(req.Method) {
		return resp, err
	}
	for attempt := 0; attempt < t.maxRetries && shouldRetry(resp, err); attempt++ {
		if resp != nil {
			_ = resp.Body.Close()
		}
		// RoundTripper's own contract (net/http.RoundTripper) says an
		// implementation must not modify the request -- and a base
		// transport may still hold internal references to it after
		// returning, even with the body closed. Retrying via a fresh
		// clone (with its own body from GetBody) rather than mutating and
		// resubmitting the same *http.Request keeps this transport a
		// well-behaved RoundTripper instead of relying on undefined
		// behavior that happens to work with the current base transport.
		next := req.Clone(req.Context())
		if req.GetBody != nil {
			body, gbErr := req.GetBody()
			if gbErr != nil {
				break
			}
			next.Body = body
		}
		select {
		case <-time.After(retryBackoff):
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
		req = next
		resp, err = t.base.RoundTrip(req)
	}
	return resp, err
}

// isIdempotentMethod reports whether method may be safely retried -- POST
// and PATCH are excluded here since this package uses them exclusively for
// non-idempotent creates/appends (see retryTransport's own doc comment).
func isIdempotentMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete, http.MethodOptions:
		return true
	default:
		return false
	}
}

func shouldRetry(resp *http.Response, err error) bool {
	if err != nil {
		return true
	}
	switch resp.StatusCode {
	case http.StatusRequestTimeout, http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}
