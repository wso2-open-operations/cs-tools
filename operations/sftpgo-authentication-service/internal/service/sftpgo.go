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
	"path/filepath"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/config"
	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/httpclient"
	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/log"
	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/models"
	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/util"
)

// SFTPGoService handles interactions with the SFTPGo Admin API.
type SFTPGoService struct {
	cfg    *config.Config
	logger *log.AppLogger
	client *http.Client
}

// NewSFTPGoService creates a new SFTPGoService.
func NewSFTPGoService(cfg *config.Config, logger *log.AppLogger) *SFTPGoService {
	return &SFTPGoService{
		cfg:    cfg,
		logger: logger,
		client: httpclient.NewLoggingClient(time.Duration(cfg.HTTPTimeout)*time.Second, logger),
	}
}

// UpdateUser updates a user's configuration in SFTPGo.
func (s *SFTPGoService) UpdateUser(username, projectKey string, perms map[string][]string, vfs []models.UserVirtualFolder) error {
	s.logger.Debug("Updating user %s to provide access to %s", username, projectKey)
	token, err := s.getAdminToken()
	if err != nil {
		return s.logger.Errorf("failed to get SFTPGo admin token for updating user '%s': %v", username, err)
	}

	exists, err := s.checkFolderExists(projectKey, token)
	if err != nil {
		return s.logger.Errorf("failed to check folder '%s' before user update: %v", projectKey, err)
	}
	if !exists {
		return s.logger.Errorf("folder '%s' does not exist, cannot update user", projectKey)
	}

	home := filepath.Join(s.cfg.DIRPath, sanitizeUsername(username))
	userForUpdate := models.MinimalSFTPGoUser{
		Username:       username,
		HomeDir:        home,
		Permissions:    perms,
		Status:         1,
		VirtualFolders: vfs,
	}

	jsonData, err := json.Marshal(userForUpdate)
	if err != nil {
		return s.logger.Errorf("error marshaling JSON for user update: %v", err)
	}

	sftpgoUserEP := s.cfg.SftpgoUsersEP + "/" + username
	req, err := http.NewRequest("PUT", sftpgoUserEP, bytes.NewBuffer(jsonData))
	if err != nil {
		return s.logger.Errorf("failed to create update user request for '%s': %v", username, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := s.client.Do(req)
	if err != nil {
		return s.logger.Errorf("failed to send update user request for '%s': %v", username, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return s.logger.Errorf("unexpected status %d when updating user '%s': %s", resp.StatusCode, username, body)
	}
	s.logger.Debug("SFTPGo user '%s' updated successfully.", username)
	return nil
}

// ProvisionFolders ensures necessary folders exist for a user in SFTPGo.
func (s *SFTPGoService) ProvisionFolders(folders []string) error {
	s.logger.Debug("Starting folder provisioning. Folders to check/create: %v", folders)
	token, err := s.getAdminToken()
	if err != nil {
		return err
	}

	for _, f := range folders {
		if err := util.ValidateFolderName(f); err != nil {
			return fmt.Errorf("invalid folder name '%s': %w", f, err)
		}

		exists, err := s.checkFolderExists(f, token)
		if err != nil {
			return fmt.Errorf("failed to check folder '%s': %w", f, err)
		}

		if !exists {
			if err := s.createFolder(f, token); err != nil {
				return fmt.Errorf("failed to create folder '%s': %w", f, err)
			}
		} else {
			s.logger.Debug("Folder '%s' already exists. No action needed.", f)
		}
	}
	s.logger.Debug("Finished folder provisioning.")
	return nil
}

func (s *SFTPGoService) getAdminToken() (string, error) {
	req, err := http.NewRequest("GET", s.cfg.AdminTokenEP, nil)
	if err != nil {
		return "", s.logger.Errorf("failed to create SFTPGo admin token request: %v", err)
	}
	req.SetBasicAuth(s.cfg.AdminUser, s.cfg.AdminKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", s.logger.Errorf("failed to send SFTPGo admin token request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", s.logger.Errorf("SFTPGo admin token request failed: status %d, body: %s", resp.StatusCode, body)
	}

	var tr models.TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", s.logger.Errorf("failed to parse SFTPGo admin token response: %v", err)
	}
	return tr.AccessToken, nil
}

func (s *SFTPGoService) checkFolderExists(name, token string) (bool, error) {
	endpoint := s.cfg.SftpgoFoldersEP + "/" + name
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return false, s.logger.Errorf("failed to create folder check request for '%s': %v", name, err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := s.client.Do(req)
	if err != nil {
		return false, s.logger.Errorf("failed to send folder check request for '%s': %v", name, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		body, _ := io.ReadAll(resp.Body)
		return false, s.logger.Errorf("unexpected status %d when checking folder '%s': %s", resp.StatusCode, name, body)
	}
}

func (s *SFTPGoService) createFolder(name, token string) error {
	s.logger.Debug("Attempting to create SFTPGo folder '%s'.", name)
	path := filepath.Join(s.cfg.FolderPath, name)
	payload := map[string]interface{}{
		"name":        name,
		"mapped_path": path,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return s.logger.Errorf("failed to marshal create folder payload for '%s': %v", name, err)
	}
	if s.logger.IsDebugEnabled() {
		s.logger.Debug("Creating folder with payload: %s", string(b))
	}

	req, err := http.NewRequest("POST", s.cfg.SftpgoFoldersEP, bytes.NewBuffer(b))
	if err != nil {
		return s.logger.Errorf("failed to create SFTPGo folder creation request for '%s': %v", name, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := s.client.Do(req)
	if err != nil {
		return s.logger.Errorf("failed to send SFTPGo folder creation request for '%s': %v", name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return s.logger.Errorf("SFTPGo folder creation failed for '%s': status %d, body: %s", name, resp.StatusCode, body)
	}
	s.logger.Debug("Successfully created SFTPGo folder '%s'.", name)
	return nil
}

// sanitizeUsername is a helper function specific to this service's needs.
func sanitizeUsername(u string) string {
	return util.SanitizeUsername(u)
}
