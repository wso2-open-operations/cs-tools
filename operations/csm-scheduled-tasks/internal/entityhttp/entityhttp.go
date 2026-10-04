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

// Package entityhttp is the one OAuth2 client-credentials HTTP stack every
// outbound client in this component uses — the ledger, case search,
// announcement publish, cloud status, both outage sweeps and the e-mail
// client. Each of those packages keeps its own narrow method set and its
// own request timeout; what they share is how a bearer token is obtained
// and attached, the https/redirect guards, and the request/response helper.
//
// Sharing the Transport is what makes it one token grant per credential set
// per invocation: an oauth2.Transport caches its token, so every client
// built on the same Transport reuses the first grant instead of each
// performing its own.
package entityhttp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/httpsec"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// TokenFetchTimeout is the HTTP client timeout for token-endpoint requests.
var TokenFetchTimeout = 10 * time.Second

// Credentials is one OAuth2 client-credentials app plus the scopes to ask
// for.
type Credentials struct {
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// NewTransport returns a RoundTripper that attaches a bearer token obtained
// with creds, fetching it once and reusing it until it expires. TokenURL
// must be https (loopback http allowed for local development — see
// httpsec.RequireSecureURL). It never contacts the token endpoint itself; a
// wrong URL surfaces on the first request.
func NewTransport(creds Credentials) (http.RoundTripper, error) {
	if err := httpsec.RequireSecureURL(creds.TokenURL); err != nil {
		return nil, fmt.Errorf("token URL: %w", err)
	}
	cc := clientcredentials.Config{
		ClientID:     creds.ClientID,
		ClientSecret: creds.ClientSecret,
		TokenURL:     creds.TokenURL,
		Scopes:       creds.Scopes,
	}
	tokenHTTPClient := &http.Client{Timeout: TokenFetchTimeout}
	httpsec.RejectInsecureRedirects(tokenHTTPClient)
	tokenCtx := context.WithValue(context.Background(), oauth2.HTTPClient, tokenHTTPClient)
	return &oauth2.Transport{
		Source: oauth2.ReuseTokenSource(nil, cc.TokenSource(tokenCtx)),
		Base:   http.DefaultTransport,
	}, nil
}

// NewClient returns an *http.Client on rt with the given overall request
// timeout (zero means none — the caller bounds each call with its own
// context instead) and the insecure-redirect guard.
func NewClient(rt http.RoundTripper, timeout time.Duration) *http.Client {
	c := &http.Client{Transport: rt, Timeout: timeout}
	httpsec.RejectInsecureRedirects(c)
	return c
}

// ClientFor is the constructor every narrow client uses: it builds on
// shared when non-nil, otherwise on a Transport of its own from creds (the
// stand-alone shape tests and one-off callers use). baseURL must be https
// (loopback allowed) since it receives the bearer token.
func ClientFor(shared http.RoundTripper, creds Credentials, baseURL string, timeout time.Duration) (*http.Client, error) {
	rt := shared
	if rt == nil {
		var err error
		if rt, err = NewTransport(creds); err != nil {
			return nil, err
		}
	}
	if err := httpsec.RequireSecureURL(baseURL); err != nil {
		return nil, fmt.Errorf("base URL: %w", err)
	}
	return NewClient(rt, timeout), nil
}

// PageDone reports whether an offset-paginated search is finished after a
// page of got rows, with offset already advanced by got (callers advance by
// rows returned, never by the page size asked for, so a short non-final
// page cannot make them skip rows). An empty page always ends it. With a
// total, it ends once offset reaches it. Without one (a response that omits
// or zeroes total), only a short page — fewer rows than pageSize — ends it,
// rather than stopping after the first page.
func PageDone(got, offset, total, pageSize int) bool {
	if got == 0 {
		return true
	}
	if total > 0 {
		return offset >= total
	}
	return got < pageSize
}

// TrimBase normalises a base URL for path concatenation.
func TrimBase(baseURL string) string { return strings.TrimRight(baseURL, "/") }

// Do executes an authenticated request against baseURL+path and returns the
// raw response body, or an *apierror.Error for a non-2xx status. prefix
// labels wrapped transport errors with the calling package's name.
func Do(ctx context.Context, c *http.Client, baseURL, prefix, method, path string, body []byte) ([]byte, error) {
	var reqBody io.Reader
	if len(body) > 0 {
		reqBody = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("%s: build request %s %s: %w", prefix, method, path, err)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %s %s: %w", prefix, method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s: read response body: %w", prefix, err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, &apierror.Error{StatusCode: resp.StatusCode, Body: string(respBody)}
	}
	return respBody, nil
}
