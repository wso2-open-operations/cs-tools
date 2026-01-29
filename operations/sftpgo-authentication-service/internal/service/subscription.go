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
)

// SubscriptionService handles interactions with the external subscription/folder API.
type SubscriptionService struct {
	// cfg is the application configuration.
	cfg *config.Config
	// logger is the application-wide logger.
	logger *log.AppLogger
	// client is the HTTP client used for subscription API calls.
	client *http.Client
}

// NewSubscriptionService creates a new SubscriptionService.
func NewSubscriptionService(cfg *config.Config, logger *log.AppLogger) *SubscriptionService {
	return &SubscriptionService{
		cfg:    cfg,
		logger: logger,
		client: httpclient.NewLoggingClient(10*time.Second, logger),
	}
}

// GetUserFolderList retrieves a custom folder list for a user.
func (s *SubscriptionService) GetUserFolderList(username string) []string {
	s.logger.Debug("Attempting to retrieve custom folder list for user: %s", username)
	apiURL := fmt.Sprintf(s.cfg.SubscriptionAPI, url.QueryEscape(username))

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		s.logger.Error("Folder list request creation error for user %s: %v", username, err)
		return nil
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		s.logger.Error("Failed to send folder list request for user %s: %v", username, err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		s.logger.Warn("Folder list API for user %s returned status %d, body: %s", username, resp.StatusCode, body)
		return nil
	}

	var folderResp models.FolderResponse
	if err := json.NewDecoder(resp.Body).Decode(&folderResp); err != nil {
		s.logger.Error("Failed to decode custom folder list for user %s: %v", username, err)
		return nil
	}

	if !folderResp.IsValidCustomer {
		s.logger.Debug("User %s is not a valid customer. No project keys returned.", username)
		return nil
	}

	var lowercaseKeys []string
	for _, key := range folderResp.ProjectKeys {
		lowercaseKeys = append(lowercaseKeys, strings.ToLower(key))
	}

	s.logger.Debug("Successfully retrieved %d custom folders for user %s.", len(lowercaseKeys), username)
	return lowercaseKeys
}

// IsValidProjectKey checks if the provided project key is valid.
func (s *SubscriptionService) IsValidProjectKey(projectKey string) bool {
	s.logger.Debug("Attempting to validate the project key: %s", projectKey)
	apiURL := fmt.Sprintf(s.cfg.ProjectAPI, url.QueryEscape(projectKey))

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		s.logger.Error("Project key validation request creation error for %s: %v", projectKey, err)
		return false
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		s.logger.Error("Failed to send project key validation request for %s: %v", projectKey, err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.logger.Debug("Project key validation API for %s returned status %d", projectKey, resp.StatusCode)
		return false
	}

	s.logger.Debug("Project key %s is valid.", projectKey)
	return true
}
