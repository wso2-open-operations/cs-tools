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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/config"
	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/log"
	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/models"
)

func TestSubscriptionService_GetUserFolderList(t *testing.T) {
	logger := log.NewAppLogger("INFO")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("customerEmail") == "valid@example.com" {
			resp := models.FolderResponse{
				IsValidCustomer: true,
				ProjectKeys:     []string{"PROJ1", "proj2"},
			}
			json.NewEncoder(w).Encode(resp)
		} else {
			resp := models.FolderResponse{IsValidCustomer: false}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	cfg := &config.Config{
		SubscriptionAPI: server.URL + "?customerEmail=%s",
	}
	s := NewSubscriptionService(cfg, logger)

	t.Run("Valid Customer", func(t *testing.T) {
		folders := s.GetUserFolderList("valid@example.com")
		if len(folders) != 2 {
			t.Errorf("Expected 2 folders, got %d", len(folders))
		}
		if folders[0] != "proj1" || folders[1] != "proj2" {
			t.Errorf("Expected [proj1, proj2], got %v", folders)
		}
	})

	t.Run("Invalid Customer", func(t *testing.T) {
		folders := s.GetUserFolderList("invalid@example.com")
		if folders != nil {
			t.Errorf("Expected nil folders for invalid customer, got %v", folders)
		}
	})
}

func TestSubscriptionService_IsValidProjectKey(t *testing.T) {
	logger := log.NewAppLogger("INFO")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("projectKey") == "VALID" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cfg := &config.Config{
		ProjectAPI: server.URL + "?projectKey=%s",
	}
	s := NewSubscriptionService(cfg, logger)

	t.Run("Valid Key", func(t *testing.T) {
		if !s.IsValidProjectKey("VALID") {
			t.Error("Expected IsValidProjectKey to return true for VALID")
		}
	})

	t.Run("Invalid Key", func(t *testing.T) {
		if s.IsValidProjectKey("INVALID") {
			t.Error("Expected IsValidProjectKey to return false for INVALID")
		}
	})
}
