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
	// Port is the port on which the HTTP server will listen.
	Port string
	// LogLevel defines the verbosity of logging (TRACE, DEBUG, INFO, WARN, ERROR).
	LogLevel string
	// HTTPTimeout is the timeout for outgoing HTTP requests in seconds.
	HTTPTimeout int
	// ReadTimeout is the maximum duration for reading the entire request, including the body.
	ReadTimeout int
	// WriteTimeout is the maximum duration before timing out writes of the response.
	WriteTimeout int
	// IdleTimeout is the maximum amount of time to wait for the next request when keep-alives are enabled.
	IdleTimeout int
	// EmailRegexPattern is an optional custom regex pattern for email validation.
	EmailRegexPattern string
	// HookAPIKey is the expected X-API-Key header value for authenticating incoming hook requests.
	HookAPIKey string

	// InternalClientID is the OAuth2 client ID for the internal organization.
	InternalClientID string
	// InternalClientSecret is the OAuth2 client secret for the internal organization.
	InternalClientSecret string
	// OAuthCallbackURL is the registered callback URL for OAuth2 flows.
	OAuthCallbackURL string
	// InternalIdPBasePath is the base URL for the internal Identity Provider.
	InternalIdPBasePath string
	// SCIMScope is the required OAuth2 scope for SCIM API access.
	SCIMScope string
	// InternalUserSuffix is the domain suffix used to identify internal users (e.g., "@wso2.com").
	InternalUserSuffix string

	// ExternalClientID is the OAuth2 client ID for external organizations.
	ExternalClientID string
	// ExternalClientSecret is the OAuth2 client secret for external organizations.
	ExternalClientSecret string
	// ExternalIdPBasePath is the base URL for the external Identity Provider.
	ExternalIdPBasePath string

	// SFTPGoBasePath is the base URL for the SFTPGo Admin API.
	SFTPGoBasePath string
	// AdminUser is the SFTPGo admin username.
	AdminUser string
	// AdminKey is the SFTPGo admin API key or password.
	AdminKey string
	// FolderPath is the base system path where SFTPGo managed folders are located.
	FolderPath string
	// DIRPath is the base system path for user home directories.
	DIRPath string
	// CheckRole is the name of the role required for a user to be authorized.
	CheckRole string

	// SubscriptionAPI is the endpoint for retrieving a user's subscription and folder details.
	SubscriptionAPI string
	// ProjectAPI is the endpoint for validating a project's existence and details.
	ProjectAPI string

	// DBConnString is the DSN for the MySQL database connection.
	DBConnString string
	// DBMaxOpenConns is the maximum number of open connections to the database.
	DBMaxOpenConns int
	// DBMaxIdleConns is the maximum number of connections in the idle connection pool.
	DBMaxIdleConns int
	// DBConnMaxLifetime is the maximum amount of time a connection may be reused.
	DBConnMaxLifetime time.Duration

	// AdminTokenEP is the computed endpoint for obtaining an SFTPGo admin token.
	AdminTokenEP string
	// SftpgoFoldersEP is the computed endpoint for SFTPGo folder management.
	SftpgoFoldersEP string
	// SftpgoUsersEP is the computed endpoint for SFTPGo user management.
	SftpgoUsersEP string
	// IdPTokenEP is the computed endpoint for obtaining internal IdP OAuth2 tokens.
	IdPTokenEP string
	// IdPSCIMUsersEP is the computed endpoint for the internal IdP's SCIM Users API.
	IdPSCIMUsersEP string
	// IdPAuthnEP is the computed endpoint for internal IdP app-native authentication.
	IdPAuthnEP string
	// IdPAuthorizeEP is the computed endpoint for internal IdP authorization.
	IdPAuthorizeEP string

	// ExternalIdPTokenEP is the computed endpoint for obtaining external IdP OAuth2 tokens.
	ExternalIdPTokenEP string
	// ExternalIdPSCIMUsersEP is the computed endpoint for the external IdP's SCIM Users API.
	ExternalIdPSCIMUsersEP string
	// ExternalIdPAuthnEP is the computed endpoint for external IdP app-native authentication.
	ExternalIdPAuthnEP string
	// ExternalIdPAuthorizeEP is the computed endpoint for external IdP authorization.
	ExternalIdPAuthorizeEP string
}

// EnvVar defines a structure for checking environment variables.
type EnvVar struct {
	// Name is the name of the environment variable.
	Name string
	// Value is a pointer to the Config field where the variable's value is stored.
	Value *string
	// IsSensitive indicates if the variable contains sensitive information (e.g., secrets).
	IsSensitive bool
	// IsCritical indicates if the application cannot start without this variable.
	IsCritical bool
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
