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

	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/util"
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
	// BasicAuthenticatorID is the authenticator ID used to identify the BasicAuthenticator in the IdP flow.
	BasicAuthenticatorID string

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

	// AdminTokenEndPoint is the computed endpoint for obtaining an SFTPGo admin token.
	AdminTokenEndPoint string
	// SFTPGoFoldersEndPoint is the computed endpoint for SFTPGo folder management.
	SFTPGoFoldersEndPoint string
	// SFTPGoUsersEndPoint is the computed endpoint for SFTPGo user management.
	SFTPGoUsersEndPoint string
	// IdPTokenEndPoint is the computed endpoint for obtaining internal IdP OAuth2 tokens.
	IdPTokenEndPoint string
	// IdPSCIMUsersEndPoint is the computed endpoint for the internal IdP's SCIM Users API.
	IdPSCIMUsersEndPoint string
	// IdPAuthnEndPoint is the computed endpoint for internal IdP app-native authentication.
	IdPAuthnEndPoint string
	// IdPAuthorizeEndPoint is the computed endpoint for internal IdP authorization.
	IdPAuthorizeEndPoint string

	// ExternalIdPTokenEndPoint is the computed endpoint for obtaining external IdP OAuth2 tokens.
	ExternalIdPTokenEndPoint string
	// ExternalIdPSCIMUsersEndPoint is the computed endpoint for the external IdP's SCIM Users API.
	ExternalIdPSCIMUsersEndPoint string
	// ExternalIdPAuthnEndPoint is the computed endpoint for external IdP app-native authentication.
	ExternalIdPAuthnEndPoint string
	// ExternalIdPAuthorizeEndPoint is the computed endpoint for external IdP authorization.
	ExternalIdPAuthorizeEndPoint string
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

// getEnvStr returns the value of the environment variable key, or fallback if unset or empty.
func getEnvStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getEnvInt parses key as a base-10 positive integer, returning fallback on any error or non-positive value.
func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

// getEnvDuration parses key as a duration string (e.g. "5m"), returning fallback on any error or non-positive value.
func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return fallback
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
		InternalUserSuffix:   getEnvStr("INTERNAL_USER_SUFFIX", "@wso2.com"),
		LogLevel:             os.Getenv("LOG_LEVEL"),
		DBConnString:         os.Getenv("DB_CONN_STRING"),
		Port:                 getEnvStr("PORT", "9090"),
		ExternalClientID:     os.Getenv("EXTERNAL_CLIENT_ID"),
		ExternalClientSecret: os.Getenv("EXTERNAL_CLIENT_SECRET"),
		ExternalIdPBasePath:  os.Getenv("EXTERNAL_IDP_BASE_PATH"),
		EmailRegexPattern:    os.Getenv("EMAIL_REGEX_PATTERN"),
		HookAPIKey:           os.Getenv("HOOK_API_KEY"),
		BasicAuthenticatorID: os.Getenv("BASIC_AUTHENTICATOR_ID"),
		HTTPTimeout:          getEnvInt("HTTP_TIMEOUT", 15),
		ReadTimeout:          getEnvInt("READ_TIMEOUT", 5),
		WriteTimeout:         getEnvInt("WRITE_TIMEOUT", 10),
		IdleTimeout:          getEnvInt("IDLE_TIMEOUT", 120),
		DBMaxOpenConns:       getEnvInt("DB_MAX_OPEN_CONNS", 25),
		DBMaxIdleConns:       getEnvInt("DB_MAX_IDLE_CONNS", 25),
		DBConnMaxLifetime:    getEnvDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
	}

	// Compute endpoints
	cfg.AdminTokenEndPoint = cfg.SFTPGoBasePath + "/token"
	cfg.SFTPGoFoldersEndPoint = cfg.SFTPGoBasePath + "/folders"
	cfg.SFTPGoUsersEndPoint = cfg.SFTPGoBasePath + "/users"
	cfg.IdPTokenEndPoint = cfg.InternalIdPBasePath + "/oauth2/token"
	cfg.IdPSCIMUsersEndPoint = cfg.InternalIdPBasePath + "/scim2/Users"
	cfg.IdPAuthnEndPoint = cfg.InternalIdPBasePath + "/oauth2/authn"
	cfg.IdPAuthorizeEndPoint = cfg.InternalIdPBasePath + "/oauth2/authorize/"

	// Compute external org endpoints
	if cfg.ExternalIdPBasePath != "" {
		cfg.ExternalIdPTokenEndPoint = cfg.ExternalIdPBasePath + "/oauth2/token"
		cfg.ExternalIdPSCIMUsersEndPoint = cfg.ExternalIdPBasePath + "/scim2/Users"
		cfg.ExternalIdPAuthnEndPoint = cfg.ExternalIdPBasePath + "/oauth2/authn"
		cfg.ExternalIdPAuthorizeEndPoint = cfg.ExternalIdPBasePath + "/oauth2/authorize/"
	}

	// Validate variables
	if err := validateEnvVars(cfg); err != nil {
		return nil, err
	}

	// Initialize (and validate) the email regex pattern
	if err := util.InitEmailRegex(cfg.EmailRegexPattern); err != nil {
		return nil, fmt.Errorf("invalid EMAIL_REGEX_PATTERN: %w", err)
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
		{"BASIC_AUTHENTICATOR_ID", &cfg.BasicAuthenticatorID, false, true},

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

	if cfg.LogLevel != "" {
		validLogLevels := map[string]bool{"TRACE": true, "DEBUG": true, "INFO": true, "WARN": true, "ERROR": true}
		if !validLogLevels[cfg.LogLevel] {
			return fmt.Errorf("invalid LOG_LEVEL '%s': must be one of TRACE, DEBUG, INFO, WARN, ERROR", cfg.LogLevel)
		}
	}

	return nil
}
