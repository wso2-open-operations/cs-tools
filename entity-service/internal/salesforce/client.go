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

// Package salesforce provides a Salesforce REST client used by
// POST /salesforce/events to fetch Account records.
package salesforce

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

const (
	accountAPIPath   = "/services/data/v54.0/sobjects/Account/"
	defaultTimeout   = 15 * time.Second
	tokenExpirySlack = 30 * time.Second
)

// Config holds the OAuth2 refresh-token credentials used to call Salesforce.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	RefreshToken string
}

// Account is the Salesforce Account fields this service maps onto Postgres.
type Account struct {
	ID                    string `json:"Id"`
	Name                  string `json:"Name"`
	AccountNumber         string `json:"AccountNumber"`
	Industry              string `json:"Industry"`
	Region                string `json:"Region__c"`
	GlobalPOD             string `json:"Global_POD__c"`
	Phone                 string `json:"Phone"`
	SalesRegions          string `json:"Sales_Regions__c"`
	SubRegion             string `json:"Sub_Region__c"`
	AccountVertical       string `json:"Account_Vertical__c"`
	AccountStatus         string `json:"Account_Status__c"`
	NAICSIndustry         string `json:"NAICS_Industry__c"`
	SubIndustry           string `json:"Sub_Industry__c"`
	AccountClassification string `json:"Account_Classification__c"`
	TechnicalOwner        string `json:"Technical_Owner__c"`
	TechnicalOwner2       string `json:"Technical_Owner_2__c"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

var errAccountUnauthorized = errors.New("salesforce: account request unauthorized")

// Client fetches Salesforce Account records using a refresh-token grant.
type Client struct {
	baseURL      string
	tokenURL     string
	clientID     string
	clientSecret string
	refreshToken string
	httpClient   *http.Client

	mu          sync.Mutex
	cachedToken string
	tokenExpiry time.Time
}

// New constructs a Client with a default 15-second timeout.
func New(cfg Config) *Client {
	return &Client{
		baseURL:      strings.TrimRight(cfg.BaseURL, "/"),
		tokenURL:     cfg.TokenURL,
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		refreshToken: cfg.RefreshToken,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// GetAccount returns the Salesforce Account with the given Id.
// A 404 is mapped to ServiceUnavailableError so the caller can retry (the
// event can arrive before Salesforce commits). On 401 the token is
// refreshed and the request is retried once.
func (c *Client) GetAccount(ctx context.Context, id string) (Account, error) {
	acct, err := c.getAccount(ctx, id)
	if errors.Is(err, errAccountUnauthorized) {
		c.invalidateToken()
		return c.getAccount(ctx, id)
	}
	return acct, err
}

func (c *Client) getAccount(ctx context.Context, id string) (Account, error) {
	token, err := c.accessToken(ctx)
	if err != nil {
		return Account{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+accountAPIPath+url.PathEscape(id), nil)
	if err != nil {
		return Account{}, fmt.Errorf("salesforce: build account request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return Account{}, err
		}
		return Account{}, &apierror.ServiceUnavailableError{Msg: fmt.Sprintf("salesforce: account request: %v", err)}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return Account{}, fmt.Errorf("salesforce: read account response: %w", err)
	}

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		var acct Account
		if err := json.Unmarshal(raw, &acct); err != nil {
			return Account{}, fmt.Errorf("salesforce: parse account response: %w", err)
		}
		return acct, nil
	case resp.StatusCode == http.StatusUnauthorized:
		return Account{}, errAccountUnauthorized
	case resp.StatusCode == http.StatusNotFound:
		return Account{}, &apierror.ServiceUnavailableError{Msg: "salesforce: account not found"}
	case resp.StatusCode >= 500:
		return Account{}, &apierror.ServiceUnavailableError{Msg: fmt.Sprintf("salesforce: account endpoint returned %d", resp.StatusCode)}
	default:
		return Account{}, &apierror.DownstreamError{Msg: fmt.Sprintf("salesforce rejected account request (status %d)", resp.StatusCode)}
	}
}

func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cachedToken != "" && time.Now().Add(tokenExpirySlack).Before(c.tokenExpiry) {
		return c.cachedToken, nil
	}

	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("client_id", c.clientID)
	data.Set("client_secret", c.clientSecret)
	data.Set("refresh_token", c.refreshToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("salesforce: build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return "", err
		}
		return "", &apierror.ServiceUnavailableError{Msg: fmt.Sprintf("salesforce: token endpoint unavailable: %v", err)}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("salesforce: read token response: %w", err)
	}
	switch resp.StatusCode {
	case http.StatusOK:
		// ok
	case http.StatusUnauthorized, http.StatusForbidden:
		return "", &apierror.UnauthorizedError{Msg: fmt.Sprintf("salesforce: token endpoint rejected credentials (status %d)", resp.StatusCode)}
	default:
		return "", &apierror.ServiceUnavailableError{Msg: fmt.Sprintf("salesforce: token endpoint returned %d", resp.StatusCode)}
	}

	var tr tokenResponse
	if err := json.Unmarshal(raw, &tr); err != nil {
		return "", fmt.Errorf("salesforce: parse token response: %w", err)
	}
	if tr.AccessToken == "" {
		return "", fmt.Errorf("salesforce: token endpoint returned empty access_token")
	}

	c.cachedToken = tr.AccessToken
	if tr.ExpiresIn > 0 {
		c.tokenExpiry = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	} else {
		c.tokenExpiry = time.Now().Add(3600 * time.Second)
	}
	return c.cachedToken, nil
}

func (c *Client) invalidateToken() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cachedToken = ""
	c.tokenExpiry = time.Time{}
}
