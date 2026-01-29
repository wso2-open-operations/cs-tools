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

package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/config"
	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/httpclient"
	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/log"
	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/models"
	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/util"
)

// IdPService handles interactions with the Identity Provider.
type IdPService struct {
	cfg    *config.Config
	logger *log.AppLogger
	client *http.Client
}

// NewIdPService creates a new IdPService.
func NewIdPService(cfg *config.Config, logger *log.AppLogger) *IdPService {
	return &IdPService{
		cfg:    cfg,
		logger: logger,
		client: httpclient.NewLoggingClient(time.Duration(cfg.HTTPTimeout)*time.Second, logger),
	}
}

type orgContext struct {
	clientID     string
	clientSecret string
	tokenEP      string
	scimEP       string
	authnEP      string
	authorizeEP  string
}

// getOrgContext returns the appropriate organization context based on whether the user is internal.
func (s *IdPService) getOrgContext(isInternal bool) orgContext {
	if isInternal {
		return orgContext{
			clientID:     s.cfg.InternalClientID,
			clientSecret: s.cfg.InternalClientSecret,
			tokenEP:      s.cfg.IdPTokenEP,
			scimEP:       s.cfg.IdPSCIMUsersEP,
			authnEP:      s.cfg.IdPAuthnEP,
			authorizeEP:  s.cfg.IdPAuthorizeEP,
		}
	}
	return orgContext{
		clientID:     s.cfg.ExternalClientID,
		clientSecret: s.cfg.ExternalClientSecret,
		tokenEP:      s.cfg.ExternalIdPTokenEP,
		scimEP:       s.cfg.ExternalIdPSCIMUsersEP,
		authnEP:      s.cfg.ExternalIdPAuthnEP,
		authorizeEP:  s.cfg.ExternalIdPAuthorizeEP,
	}
}

// isInternalUser checks if a username belongs to an internal user based on configured suffix
func (s *IdPService) isInternalUser(username string) bool {
	return util.IsInternalUser(username, s.cfg.InternalUserSuffix)
}

// GetAsgardeoUser fetches user details from Asgardeo via SCIM API.
func (s *IdPService) GetAsgardeoUser(username string) (*models.AsgardeoUser, error) {
	isInternal := s.isInternalUser(username)
	token, err := s.getBearerToken(isInternal)
	if err != nil {
		s.logger.Error("failed to get bearer token for SCIM API: %v", err)
		return nil, err
	}

	asgUser := username
	if !strings.Contains(username, "/") {
		asgUser = "DEFAULT/" + username
	}
	// Security: Escape quotes in username to prevent SCIM filter injection
	safeUsername := strings.ReplaceAll(asgUser, `"`, `\"`)
	filter := fmt.Sprintf(`userName eq "%s"`, safeUsername)
	params := url.Values{}
	params.Set("filter", filter)

	// Use appropriate org endpoint
	ctx := s.getOrgContext(isInternal)
	scimURL := fmt.Sprintf("%s?%s", ctx.scimEP, params.Encode())

	req, err := http.NewRequest("GET", scimURL, nil)
	if err != nil {
		return nil, s.logger.Errorf("failed to create SCIM request for user %s: %v", username, err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, s.logger.Errorf("SCIM request failed for user %s: %v", username, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, s.logger.Errorf("SCIM request failed for user %s: status %d, body: %s", username, resp.StatusCode, body)
	}

	var result struct {
		Resources []models.AsgardeoUser `json:"Resources"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, s.logger.Errorf("failed to parse SCIM response for user %s: %v", username, err)
	}
	if len(result.Resources) == 0 {
		return nil, fmt.Errorf("user not found")
	}
	return &result.Resources[0], nil
}

// InitFlow initiates an authentication flow with the IdP.
func (s *IdPService) InitFlow(username string) (*models.IdPResponse, error) {
	isInternal := s.isInternalUser(username)
	ctx := s.getOrgContext(isInternal)

	form := url.Values{}
	form.Set("client_id", ctx.clientID)
	form.Set("client_secret", ctx.clientSecret)
	form.Set("response_type", "code")
	form.Set("redirect_uri", s.cfg.OAuthCallbackURL)
	form.Set("scope", "openid")
	form.Set("response_mode", "direct")

	req, err := http.NewRequest("POST", ctx.authorizeEP, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, s.logger.Errorf("failed to create flow ID request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := s.client.Do(req)
	if err != nil {
		return nil, s.logger.Errorf("failed to send flow ID request: %v", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, s.logger.Errorf("failed to read flow ID response body: %v", err)
	}

	if res.StatusCode >= 400 {
		return nil, s.logger.Errorf("HTTP error %d getting flow ID: %s", res.StatusCode, string(body))
	}

	var idpResp models.IdPResponse
	if err := json.Unmarshal(body, &idpResp); err != nil {
		return nil, s.logger.Errorf("failed to unmarshal flow ID response: %v", err)
	}
	if idpResp.FlowID == "" {
		return nil, s.logger.Errorf("flowId not found in IdP response: %s", string(body))
	}
	return &idpResp, nil
}

// PostToAuthnEndpoint sends a payload to the IdP's authentication endpoint.
func (s *IdPService) PostToAuthnEndpoint(payload interface{}, isInternal bool) (*models.IdPResponse, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, s.logger.Errorf("failed to marshal IdP payload: %v", err)
	}

	ctx := s.getOrgContext(isInternal)
	res, err := s.client.Post(ctx.authnEP, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, s.logger.Errorf("failed to send IdP authn request: %v", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, s.logger.Errorf("failed to read IdP authn response body: %v", err)
	}

	if res.StatusCode >= 400 {
		s.logger.Warn("Received HTTP error %d from IdP authn endpoint: %s", res.StatusCode, string(body))
	}

	var idpResp models.IdPResponse
	if err := json.Unmarshal(body, &idpResp); err != nil {
		return nil, s.logger.Errorf("failed to unmarshal IdP authn response: %v", err)
	}

	return &idpResp, nil
}

// getBearerToken retrieves an OAuth2 bearer token for SCIM API access.
func (s *IdPService) getBearerToken(isInternal bool) (string, error) {
	ctx := s.getOrgContext(isInternal)

	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", ctx.clientID)
	data.Set("client_secret", ctx.clientSecret)
	data.Set("scope", s.cfg.SCIMScope)

	req, err := http.NewRequest("POST", ctx.tokenEP, strings.NewReader(data.Encode()))
	if err != nil {
		return "", s.logger.Errorf("failed to create token request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", s.logger.Errorf("failed to send token request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", s.logger.Errorf("token request failed: status %d, body: %s", resp.StatusCode, body)
	}

	var tr models.TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", s.logger.Errorf("failed to parse token response: %v", err)
	}
	return tr.AccessToken, nil
}
