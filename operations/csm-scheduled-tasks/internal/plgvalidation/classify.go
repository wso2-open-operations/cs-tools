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

package plgvalidation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// maxReplyBytes caps a Directory or AI service reply.
const maxReplyBytes = 1 << 20

// Directory reads a Cloud user's email. The email is personal data: never
// stored, logged or put in errors.
type Directory interface {
	UserEmail(ctx context.Context, userID string) (string, error)
}

// DomainClassifier classifies an email domain (the CSM AI service).
type DomainClassifier interface {
	Classify(ctx context.Context, domain string) (string, error)
}

// DirectoryConfig is the Cloud Directory API's.
type DirectoryConfig struct {
	Config
	UserEmailPath string // with {userId} where the id goes
}

// DirectoryClient calls the Cloud Directory API.
type DirectoryClient struct {
	http    *http.Client
	baseURL string
	path    string
}

// NewDirectoryClient returns a client authenticated via client credentials.
func NewDirectoryClient(cfg DirectoryConfig) (*DirectoryClient, error) {
	httpClient, err := oauthClient(cfg.Config)
	if err != nil {
		return nil, err
	}
	return &DirectoryClient{http: httpClient, baseURL: strings.TrimRight(cfg.BaseURL, "/"), path: cfg.UserEmailPath}, nil
}

// UserEmail returns the user's email.
func (c *DirectoryClient) UserEmail(ctx context.Context, userID string) (string, error) {
	var out struct {
		Email string `json:"email"`
	}
	path := strings.ReplaceAll(c.path, "{userId}", url.PathEscape(userID))
	if err := call(ctx, c.http, http.MethodGet, c.baseURL+path, nil, &out); err != nil {
		return "", fmt.Errorf("cloud directory: %w", err)
	}
	return out.Email, nil
}

// AIClient calls the CSM AI service.
type AIClient struct {
	http    *http.Client
	baseURL string
}

// NewAIClient returns a client authenticated via client credentials.
func NewAIClient(cfg Config) (*AIClient, error) {
	httpClient, err := oauthClient(cfg)
	if err != nil {
		return nil, err
	}
	return &AIClient{http: httpClient, baseURL: strings.TrimRight(cfg.BaseURL, "/")}, nil
}

// Classify returns the domain's classification code.
func (c *AIClient) Classify(ctx context.Context, domain string) (string, error) {
	var out struct {
		Classification string `json:"classification"`
	}
	if err := call(ctx, c.http, http.MethodPost, c.baseURL+"/ai/email-domains/classify", map[string]string{"domain": domain}, &out); err != nil {
		return "", fmt.Errorf("csm ai service: %w", err)
	}
	return out.Classification, nil
}

// call makes one JSON call; errors carry the status, never the body.
func call(ctx context.Context, client *http.Client, method, target string, payload, out any) error {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return errors.New("build request")
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxReplyBytes))
	if err != nil {
		return errors.New("read reply")
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	if json.Unmarshal(raw, out) != nil {
		return errors.New("decode reply")
	}
	return nil
}

// roleLocalParts are shared or test mailboxes.
var roleLocalParts = map[string]bool{
	"admin": true, "administrator": true, "info": true, "support": true, "sales": true,
	"contact": true, "help": true, "billing": true, "noreply": true, "no-reply": true,
	"postmaster": true, "webmaster": true, "hostmaster": true, "abuse": true,
	"marketing": true, "hr": true, "jobs": true, "careers": true, "office": true,
	"test": true, "testing": true, "qa": true, "demo": true, "example": true, "sample": true,
	"foo": true, "bar": true, "asdf": true, "user": true, "dummy": true,
}

// splitEmail returns the domain and whether the local part is a role or test
// mailbox (test+1@, admin2@ included). ok is false for a malformed address.
func splitEmail(email string) (domain string, role, ok bool) {
	email = strings.ToLower(strings.TrimSpace(email))
	at := strings.LastIndexByte(email, '@')
	if at <= 0 || at == len(email)-1 {
		return "", false, false
	}
	local := email[:at]
	if i := strings.IndexByte(local, '+'); i >= 0 {
		local = local[:i]
	}
	role = roleLocalParts[local] || roleLocalParts[strings.TrimRight(local, "0123456789-_.")]
	return email[at+1:], role, true
}
