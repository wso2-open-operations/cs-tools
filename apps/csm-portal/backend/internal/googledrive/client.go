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

// Package googledrive is a client for the Google Drive v3 REST API, used by
// SupportPortalLite to browse a customer's shared Drive folder. Ported from
// the Ballerina backend's modules/googledrive package, which used the
// ballerinax/googleapis.drive connector; this package calls the Drive v3
// REST API directly over plain net/http, matching this codebase's existing
// raw-HTTP-client philosophy (see internal/entity) rather than pulling in
// the full google.golang.org/api SDK for two read-only calls.
package googledrive

import (
	"context"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/upstreamhttp"
)

// driveAPIBaseURL is the Google Drive v3 REST API root. Not configurable:
// unlike the backing system/entity, this is a fixed Google endpoint, not a
// per-deployment host.
const driveAPIBaseURL = "https://www.googleapis.com/drive/v3"

// googleOAuth2Endpoint is Google's OAuth2 token endpoint, used for the
// refresh-token grant. Declared literally instead of importing
// golang.org/x/oauth2/google (which pulls in the full Google API client
// machinery for one constant); refreshUrl in the Ballerina config defaulted
// to the connector's own built-in REFRESH_URL, which is this same standard
// Google endpoint.
var googleOAuth2Endpoint = oauth2.Endpoint{ // #nosec G101 -- public well-known endpoint URLs, not credentials
	AuthURL:  "https://accounts.google.com/o/oauth2/auth",
	TokenURL: "https://oauth2.googleapis.com/token",
}

// Config holds the configuration for the Google Drive client — an OAuth2
// refresh-token grant (not a service account), matching the Ballerina
// clientId/clientSecret/refreshToken configurables.
type Config struct {
	ClientID     string
	ClientSecret string
	RefreshToken string
}

// Client is an HTTP client for the Google Drive v3 REST API, authenticated
// via the OAuth2 refresh-token grant. Tokens are refreshed automatically;
// callers need not manage them.
//
// Unlike the Ballerina driveClient (a package-level nilable var, set to nil
// at module-init if the configured credential is rejected, because the
// underlying connector's auth handler fetches a token eagerly during
// construction and panics rather than returning an error on failure — see
// google_drive.bal's initDriveClientOrNil and its `trap` guard),
// NewClient here never fails and never contacts the token endpoint: Go's
// oauth2.Config.TokenSource is lazy, only fetching (and only being able to
// fail) on the first actual API call. That first-call failure already comes
// back as a normal Go error from ListFiles/SearchFolder, which the handler
// layer already maps to a 5xx response — so no init-time trap/nil-guard
// equivalent is needed in this port; a bad refresh token simply makes the
// first real request fail with a clear error instead of crashing anything
// at startup.
type Client struct {
	http *http.Client
}

// NewClient constructs a Client that authenticates against Google Drive
// using the OAuth2 refresh-token grant.
func NewClient(cfg Config) *Client {
	oauthCfg := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint:     googleOAuth2Endpoint,
	}
	// The token refresh runs on its own client: without one it would use
	// http.DefaultClient, which has no timeout, so a hung token endpoint
	// would hang every Drive call behind it.
	tokenCtx := context.WithValue(context.Background(), oauth2.HTTPClient, upstreamhttp.TokenClient(tokenRefreshTimeout))
	tokenSource := oauthCfg.TokenSource(tokenCtx, &oauth2.Token{RefreshToken: cfg.RefreshToken})
	httpClient := oauth2.NewClient(tokenCtx, tokenSource)
	httpClient.Timeout = 25 * time.Second

	return &Client{http: httpClient}
}

// tokenRefreshTimeout bounds one refresh-token grant against Google's token
// endpoint.
var tokenRefreshTimeout = 10 * time.Second

// escapeDriveQueryValue escapes a value for safe interpolation inside a
// single-quoted string literal in a Drive API "q" query, per Google's Drive
// query-language escaping rules (backslash and single-quote must be
// backslash-escaped). The Ballerina source interpolated folderId directly
// into its filter string with no escaping at all
// ('${folderId}' in parents and trashed=false); this closes that same class
// of query-injection gap the backing system's sysparm_query concatenation has,
// without changing which Drive API or endpoint is called.
func escapeDriveQueryValue(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `'`, `\'`)
	return value
}
