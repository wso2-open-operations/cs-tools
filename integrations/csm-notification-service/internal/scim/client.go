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

// Package scim is a minimal client for the SCIM operations service
// (digiops-infra's scim-operations-service), used only by
// internal/dispatch's project_contact.invited handler to make sure an
// invited customer contact has an Asgardeo identity to sign in with. Like
// internal/entity — and unlike the channel clients in internal/notifications
// — this is not a notification channel: it provisions an account, it doesn't
// send anything to a person, so it lives in its own package rather than
// following the <Name>Config/<Name>Client-in-internal/notifications pattern
// (see that package's doc.go). Mirrors apps/csm-portal/backend's own
// internal/scim client's OAuth2 client-credentials shape, narrowed to the
// one operation this service needs; deliberately not imported from there
// (this module is its own deployable, and that client carries end-user
// identity/tracing headers a Kafka consumer has nothing to fill).
package scim

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/oauthhttp"
)

// Config holds the configuration for the SCIM operations client. Like
// entity.CustomerEntityConfig, cmd/server/main.go fills TokenURL/ClientID/
// ClientSecret from this service's shared OAUTH2_* app — the SCIM
// operations service authenticates every caller through the same shared
// gateway app, scoped via SCIM_SCOPES — and only BaseURL/Scopes from this
// client's own SCIM_* env vars. The fields stay generic so a future caller
// could still construct this with independent credentials.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// Client is an HTTP client for the SCIM operations service, authenticated
// via the OAuth2 client credentials grant. Tokens are acquired and refreshed
// automatically; callers need not manage them.
//
// NewClient never fails and never contacts the token endpoint, so it is safe
// to construct with a zero-value Config (e.g. when CSM_MIGRATION_ONBOARD_IDENTITY_ENABLED
// is off for a given deployment) — a missing or invalid configuration only
// surfaces as an error the first time EnsureExternalUser is called.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient constructs a Client that authenticates against the SCIM
// operations service using the OAuth2 client credentials grant type.
func NewClient(cfg Config) *Client {
	httpClient := oauthhttp.NewClient(oauthhttp.Config{
		TokenURL:     cfg.TokenURL,
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Scopes:       cfg.Scopes,
	})

	return &Client{
		http:    httpClient,
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
	}
}

// ExternalUser is EnsureExternalUser's result: the Asgardeo user id and
// userName of the "external" (customer) organization's user for the invited
// email, and whether that user already existed before the call — which is
// what decides between the "welcome, your account was just created" and
// "your existing account now has access to another project" invitation
// wordings (see internal/dispatch.handleProjectContactInvited).
type ExternalUser struct {
	ID       string `json:"id"`
	UserName string `json:"userName"`
	Existed  bool   `json:"existed"`
}

// createExternalUserRequest is the wire shape of POST
// /organizations/external/users. There is deliberately no password or
// askPassword field: the SCIM operations service generates a random
// password the user never sees and Asgardeo sends no email of its own —
// the invitee's first sign-in goes through an email code, and the
// invitation email this service sends is the only message they get.
type createExternalUserRequest struct {
	UserName   string   `json:"userName"`
	GivenName  string   `json:"givenName,omitempty"`
	FamilyName string   `json:"familyName,omitempty"`
	Emails     []string `json:"emails"`
}

// EnsureExternalUser calls POST /organizations/external/users so that an
// Asgardeo user exists for email in the external (customer) organization,
// returning it. Idempotent on the SCIM service's side: 201 means the user
// was just created, 200 means one already existed for that userName — both
// are successes, distinguished by ExternalUser.Existed, and both must carry
// the user's id in the body. Any other status is returned as an
// *apierror.Error carrying the status only.
func (c *Client) EnsureExternalUser(ctx context.Context, email, givenName, familyName string) (ExternalUser, error) {
	if email == "" {
		return ExternalUser{}, fmt.Errorf("scim: email is required")
	}
	reqBody, err := json.Marshal(createExternalUserRequest{
		UserName:   email,
		GivenName:  givenName,
		FamilyName: familyName,
		Emails:     []string{email},
	})
	if err != nil {
		return ExternalUser{}, fmt.Errorf("scim: encode create-user request: %w", err)
	}

	status, respBody, err := c.do(ctx, http.MethodPost, "/organizations/external/users", reqBody)
	if err != nil {
		return ExternalUser{}, err
	}

	// A 200/201 without a body, or without the user's id, is not proof that
	// an Asgardeo account exists — treat it as an upstream fault so the
	// consumer retries instead of recording IDENTITY=SUCCEEDED and sending
	// an invitation for an account nobody confirmed.
	if len(respBody) == 0 {
		return ExternalUser{}, fmt.Errorf("scim: create-user returned %d with an empty body", status)
	}
	var user ExternalUser
	if err := json.Unmarshal(respBody, &user); err != nil {
		return ExternalUser{}, fmt.Errorf("scim: decode create-user response: %w", err)
	}
	if user.ID == "" {
		return ExternalUser{}, fmt.Errorf("scim: create-user returned %d without a user id", status)
	}
	// The status code is authoritative for Existed; the body's own
	// "existed" flag is only a fallback for a response that omits it
	// (both say the same thing when present).
	switch status {
	case http.StatusCreated:
		user.Existed = false
	case http.StatusOK:
		user.Existed = true
	}
	return user, nil
}

// do executes an authenticated HTTP request against the SCIM operations
// service and returns the response's status code and raw body. Only 200 and
// 201 are successes: EnsureExternalUser needs to tell them apart, so unlike
// this repo's other do() implementations (which accept any 2xx and return
// the body alone) the status is returned too. Any other status is an
// *apierror.Error carrying the status only: the body is deliberately not
// retained, because a SCIM validation error echoes the submitted userName
// (the invitee's email) and this error ends up persisted as an onboarding
// step's lastError. Same choice internal/entity makes.
func (c *Client) do(ctx context.Context, method, path string, body []byte) (int, []byte, error) {
	var reqBody io.Reader
	if len(body) > 0 {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return 0, nil, fmt.Errorf("scim: build request %s %s: %w", method, path, err)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("scim: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("scim: read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return resp.StatusCode, nil, &apierror.Error{StatusCode: resp.StatusCode}
	}

	return resp.StatusCode, respBody, nil
}
