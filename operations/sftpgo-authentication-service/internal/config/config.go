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
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for the application, loaded from environment variables.
type Config struct {
	// Service Configuration
	Port              string
	LogLevel          string
	HTTPTimeout       int
	ReadTimeout       int
	WriteTimeout      int
	IdleTimeout       int
	EmailRegexPattern string
	HookAPIKey        string

	// IdP Configuration (Internal Users)
	InternalClientID     string
	InternalClientSecret string
	OAuthCallbackURL     string
	InternalIdPBasePath  string
	SCIMScope            string
	InternalUserSuffix   string

	// IdP Configuration (External Users)
	ExternalClientID     string
	ExternalClientSecret string
	ExternalIdPBasePath  string

	// SFTPGo Configuration
	SFTPGoBasePath string
	AdminUser      string
	AdminKey       string
	FolderPath     string
	DIRPath        string
	CheckRole      string

	// External APIs
	SubscriptionAPI string
	ProjectAPI      string

	// Database Configuration
	DBConnString      string
	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxLifetime time.Duration

	// Computed endpoints
	AdminTokenEP    string
	SftpgoFoldersEP string
	SftpgoUsersEP   string
	IdPTokenEP      string
	IdPSCIMUsersEP  string
	IdPAuthnEP      string
	IdPAuthorizeEP  string

	// External org computed endpoints
	ExternalIdPTokenEP     string
	ExternalIdPSCIMUsersEP string
	ExternalIdPAuthnEP     string
	ExternalIdPAuthorizeEP string
}

// EnvVar defines a structure for checking environment variables.
type EnvVar struct {
	Name        string
	Value       *string
	IsSensitive bool
	IsCritical  bool
}

// Load reads environment variables into the Config struct and validates them.
func Load() (*Config, error) {
	cfg := &Config{
		InternalClientID:     os.Getenv("INTERNAL_CLIENT_ID"),
		InternalClientSecret: os.Getenv("INTERNAL_CLIENT_SECRET"),
		OAuthCallbackURL:     os.Getenv("OAUTH_CALLBACK_URL"),
		InternalIdPBasePath:  os.Getenv("INTERNAL_IDP_BASE_PATH"),
		SubscriptionAPI:      os.Getenv("SUBSCRIPTION_API"),
		ProjectAPI:           os.Getenv("PROJECT_API"),
		SFTPGoBasePath:       os.Getenv("SFTPGO_API_BASE"),
		AdminUser:            os.Getenv("ADMIN_USER"),
		AdminKey:             os.Getenv("ADMIN_KEY"),
		FolderPath:           os.Getenv("FOLDER_PATH"),
		CheckRole:            os.Getenv("CHECK_ROLE"),
		DIRPath:              os.Getenv("DIR_PATH"),
		SCIMScope:            os.Getenv("SCIM_SCOPE"),
		InternalUserSuffix:   os.Getenv("INTERNAL_USER_SUFFIX"),
		LogLevel:             os.Getenv("LOG_LEVEL"),
		DBConnString:         os.Getenv("DB_CONN_STRING"),
		Port:                 os.Getenv("PORT"),

		// External org config
		ExternalClientID:     os.Getenv("EXTERNAL_CLIENT_ID"),
		ExternalClientSecret: os.Getenv("EXTERNAL_CLIENT_SECRET"),
		ExternalIdPBasePath:  os.Getenv("EXTERNAL_IDP_BASE_PATH"),

		// Email regex pattern (optional)
		EmailRegexPattern: os.Getenv("EMAIL_REGEX_PATTERN"),

		// API Key for hooks
		HookAPIKey: os.Getenv("HOOK_API_KEY"),
	}

	// Load HTTP Timeout
	if timeoutStr := os.Getenv("HTTP_TIMEOUT"); timeoutStr != "" {
		if timeout, err := strconv.Atoi(timeoutStr); err == nil && timeout > 0 {
			cfg.HTTPTimeout = timeout
		}
	}
	if cfg.HTTPTimeout == 0 {
		cfg.HTTPTimeout = 15 // Default to 15 seconds
	}

	// Load HTTP Server Timeouts
	if timeoutStr := os.Getenv("READ_TIMEOUT"); timeoutStr != "" {
		if timeout, err := strconv.Atoi(timeoutStr); err == nil && timeout > 0 {
			cfg.ReadTimeout = timeout
		}
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 5 // Default to 5 seconds
	}

	if timeoutStr := os.Getenv("WRITE_TIMEOUT"); timeoutStr != "" {
		if timeout, err := strconv.Atoi(timeoutStr); err == nil && timeout > 0 {
			cfg.WriteTimeout = timeout
		}
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 10 // Default to 10 seconds
	}

	if timeoutStr := os.Getenv("IDLE_TIMEOUT"); timeoutStr != "" {
		if timeout, err := strconv.Atoi(timeoutStr); err == nil && timeout > 0 {
			cfg.IdleTimeout = timeout
		}
	}
	if cfg.IdleTimeout == 0 {
		cfg.IdleTimeout = 120 // Default to 120 seconds
	}

	// Load Database Connection Pooling Settings
	if val := os.Getenv("DB_MAX_OPEN_CONNS"); val != "" {
		if i, err := strconv.Atoi(val); err == nil && i > 0 {
			cfg.DBMaxOpenConns = i
		}
	}
	if cfg.DBMaxOpenConns == 0 {
		cfg.DBMaxOpenConns = 25 // Default
	}

	if val := os.Getenv("DB_MAX_IDLE_CONNS"); val != "" {
		if i, err := strconv.Atoi(val); err == nil && i > 0 {
			cfg.DBMaxIdleConns = i
		}
	}
	if cfg.DBMaxIdleConns == 0 {
		cfg.DBMaxIdleConns = 25 // Default
	}

	if val := os.Getenv("DB_CONN_MAX_LIFETIME"); val != "" {
		if d, err := time.ParseDuration(val); err == nil && d > 0 {
			cfg.DBConnMaxLifetime = d
		}
	}
	if cfg.DBConnMaxLifetime == 0 {
		cfg.DBConnMaxLifetime = 5 * time.Minute // Default
	}

	// Compute endpoints
	cfg.AdminTokenEP = cfg.SFTPGoBasePath + "/token"
	cfg.SftpgoFoldersEP = cfg.SFTPGoBasePath + "/folders"
	cfg.SftpgoUsersEP = cfg.SFTPGoBasePath + "/users"
	cfg.IdPTokenEP = cfg.InternalIdPBasePath + "/oauth2/token"
	cfg.IdPSCIMUsersEP = cfg.InternalIdPBasePath + "/scim2/Users"
	cfg.IdPAuthnEP = cfg.InternalIdPBasePath + "/oauth2/authn"
	cfg.IdPAuthorizeEP = cfg.InternalIdPBasePath + "/oauth2/authorize/"

	// Compute external org endpoints
	if cfg.ExternalIdPBasePath != "" {
		cfg.ExternalIdPTokenEP = cfg.ExternalIdPBasePath + "/oauth2/token"
		cfg.ExternalIdPSCIMUsersEP = cfg.ExternalIdPBasePath + "/scim2/Users"
		cfg.ExternalIdPAuthnEP = cfg.ExternalIdPBasePath + "/oauth2/authn"
		cfg.ExternalIdPAuthorizeEP = cfg.ExternalIdPBasePath + "/oauth2/authorize/"
	}

	// Set default port if not provided
	if cfg.Port == "" {
		cfg.Port = "9090"
	}

	// Set default internal user suffix if not provided
	if cfg.InternalUserSuffix == "" {
		cfg.InternalUserSuffix = "@wso2.com"
	}

	// Validate variables
	if err := validateEnvVars(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// validateEnvVars checks if all critical environment variables are set.
func validateEnvVars(cfg *Config) error {
	envVars := []EnvVar{
		// Service Configuration
		{"PORT", &cfg.Port, false, false},
		{"LOG_LEVEL", &cfg.LogLevel, false, false},
		{"EMAIL_REGEX_PATTERN", &cfg.EmailRegexPattern, false, false},
		{"HOOK_API_KEY", &cfg.HookAPIKey, true, false},

		// IdP Configuration (Internal Users)
		{"INTERNAL_CLIENT_ID", &cfg.InternalClientID, false, true},
		{"INTERNAL_CLIENT_SECRET", &cfg.InternalClientSecret, true, true},
		{"OAUTH_CALLBACK_URL", &cfg.OAuthCallbackURL, false, true},
		{"INTERNAL_IDP_BASE_PATH", &cfg.InternalIdPBasePath, false, true},
		{"SCIM_SCOPE", &cfg.SCIMScope, false, true},
		{"INTERNAL_USER_SUFFIX", &cfg.InternalUserSuffix, false, false},

		// IdP Configuration (External Users)
		{"EXTERNAL_CLIENT_ID", &cfg.ExternalClientID, false, false},
		{"EXTERNAL_CLIENT_SECRET", &cfg.ExternalClientSecret, true, false},
		{"EXTERNAL_IDP_BASE_PATH", &cfg.ExternalIdPBasePath, false, false},

		// SFTPGo Configuration
		{"SFTPGO_API_BASE", &cfg.SFTPGoBasePath, false, true},
		{"ADMIN_USER", &cfg.AdminUser, false, true},
		{"ADMIN_KEY", &cfg.AdminKey, true, true},
		{"FOLDER_PATH", &cfg.FolderPath, false, true},
		{"DIR_PATH", &cfg.DIRPath, false, true},
		{"CHECK_ROLE", &cfg.CheckRole, false, true},

		// External APIs
		{"SUBSCRIPTION_API", &cfg.SubscriptionAPI, false, true},
		{"PROJECT_API", &cfg.ProjectAPI, false, true},

		// Database Configuration
		{"DB_CONN_STRING", &cfg.DBConnString, true, true},
	}

	var missingCriticalVars []string
	for _, v := range envVars {
		if v.IsCritical && *v.Value == "" {
			missingCriticalVars = append(missingCriticalVars, v.Name)
		}
	}

	if len(missingCriticalVars) > 0 {
		return fmt.Errorf("missing critical environment variables: %s", strings.Join(missingCriticalVars, ", "))
	}

	return nil
}
