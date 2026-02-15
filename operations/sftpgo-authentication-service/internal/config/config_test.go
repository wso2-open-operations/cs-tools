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

package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoad_Success(t *testing.T) {
	// Setup environment variables
	os.Setenv("INTERNAL_CLIENT_ID", "test_client_id")
	os.Setenv("INTERNAL_CLIENT_SECRET", "test_client_secret")
	os.Setenv("OAUTH_CALLBACK_URL", "http://localhost/callback")
	os.Setenv("INTERNAL_IDP_BASE_PATH", "http://idp.example.com")
	os.Setenv("SUBSCRIPTION_API", "http://sub.example.com")
	os.Setenv("PROJECT_API", "http://proj.example.com")
	os.Setenv("SFTPGO_API_BASE", "http://sftpgo.example.com")
	os.Setenv("ADMIN_USER", "admin")
	os.Setenv("ADMIN_KEY", "secret")
	os.Setenv("FOLDER_PATH", "/tmp/sftpgo")
	os.Setenv("DIR_PATH", "/tmp/data")
	os.Setenv("CHECK_ROLE", "internal")
	os.Setenv("SCIM_SCOPE", "scope")
	os.Setenv("DB_CONN_STRING", "db_conn")
	os.Setenv("HTTP_TIMEOUT", "20")
	os.Setenv("HOOK_API_KEY", "test_hook_key")

	defer os.Clearenv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if cfg.HookAPIKey != "test_hook_key" {
		t.Errorf("Expected HookAPIKey 'test_hook_key', got '%s'", cfg.HookAPIKey)
	}

	if cfg.InternalClientID != "test_client_id" {
		t.Errorf("Expected InternalClientID 'test_client_id', got '%s'", cfg.InternalClientID)
	}
	if cfg.HTTPTimeout != 20 {
		t.Errorf("Expected HTTPTimeout 20, got %d", cfg.HTTPTimeout)
	}
	if cfg.AdminTokenEP != "http://sftpgo.example.com/token" {
		t.Errorf("Expected AdminTokenEP 'http://sftpgo.example.com/token', got '%s'", cfg.AdminTokenEP)
	}
}

func TestLoad_MissingCritical(t *testing.T) {
	// Snapshot current environment
	oldEnv := os.Environ()
	defer func() {
		// Restore environment
		os.Clearenv()
		for _, e := range oldEnv {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) == 2 {
				os.Setenv(parts[0], parts[1])
			}
		}
	}()

	os.Clearenv()
	// Only set one, missing others
	os.Setenv("INTERNAL_CLIENT_ID", "test_client_id")

	_, err := Load()
	if err == nil {
		t.Fatal("Expected error due to missing critical variables, got nil")
	}
}

func TestLoad_Defaults(t *testing.T) {
	// Setup minimal critical variables
	os.Setenv("INTERNAL_CLIENT_ID", "test_client_id")
	os.Setenv("INTERNAL_CLIENT_SECRET", "test_client_secret")
	os.Setenv("OAUTH_CALLBACK_URL", "u")
	os.Setenv("INTERNAL_IDP_BASE_PATH", "u")
	os.Setenv("SUBSCRIPTION_API", "u")
	os.Setenv("PROJECT_API", "u")
	os.Setenv("SFTPGO_API_BASE", "u")
	os.Setenv("ADMIN_USER", "u")
	os.Setenv("ADMIN_KEY", "u")
	os.Setenv("FOLDER_PATH", "u")
	os.Setenv("DIR_PATH", "u")
	os.Setenv("CHECK_ROLE", "u")
	os.Setenv("SCIM_SCOPE", "u")
	os.Setenv("DB_CONN_STRING", "u")

	// Unset optional ones that have defaults
	os.Unsetenv("PORT")
	os.Unsetenv("HTTP_TIMEOUT")

	defer os.Clearenv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if cfg.Port != "9090" {
		t.Errorf("Expected default Port '9090', got '%s'", cfg.Port)
	}
	if cfg.HTTPTimeout != 15 {
		t.Errorf("Expected default HTTPTimeout 15, got %d", cfg.HTTPTimeout)
	}
}
