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

// Package oauthhttp builds the *http.Client every upstream client in this
// service uses to call another WSO2 service with the OAuth2 client
// credentials grant. Tokens are fetched and refreshed transparently by
// golang.org/x/oauth2; callers never see them. It exists so the four
// upstream clients (entity-service, email service, SLA entity, SCIM) share
// one copy of the plumbing and one place to change timeouts.
package oauthhttp

import (
	"context"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// TokenFetchTimeout bounds each request to the token endpoint. A variable,
// not a constant, so tests can shorten it.
var TokenFetchTimeout = 10 * time.Second

// RequestTimeout bounds each authenticated request to the upstream service.
const RequestTimeout = 25 * time.Second

// Config is the client credentials grant for one upstream service.
type Config struct {
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// NewClient returns an *http.Client that attaches a bearer token from cfg
// to every request. It never contacts the token endpoint itself: a missing
// or invalid configuration only surfaces as an error on the first request.
func NewClient(cfg Config) *http.Client {
	cc := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
		Scopes:       cfg.Scopes,
	}
	tokenCtx := context.WithValue(context.Background(), oauth2.HTTPClient,
		&http.Client{Timeout: TokenFetchTimeout})
	httpClient := cc.Client(tokenCtx)
	httpClient.Timeout = RequestTimeout
	return httpClient
}
