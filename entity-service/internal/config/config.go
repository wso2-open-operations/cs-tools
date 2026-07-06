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

// Package config loads runtime configuration from environment variables.
package config

import (
	"fmt"
	"net/url"
	"os"
)

// DataSource identifies which backend the service reads from.
type DataSource string

const (
	// DataSourcePostgres uses the local PostgreSQL database.
	DataSourcePostgres DataSource = "postgres"
	// DataSourceServiceNow uses the Choreo ServiceNow API.
	DataSourceServiceNow DataSource = "servicenow"
)

// Config holds all environment-driven settings for the service.
type Config struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string
	ServerPort string
	// DataSource controls which backend is used. Defaults to "postgres".
	DataSource DataSource
	// ServiceNowIntegrationServiceBaseURL is the base URL for the ServiceNow integration service API.
	// Required when DataSource is "servicenow".
	ServiceNowIntegrationServiceBaseURL string
	// OAuth2 client credentials for the ServiceNow integration service.
	// All four fields are required when DataSource is "servicenow".
	ServiceNowIntegrationServiceTokenURL     string
	ServiceNowIntegrationServiceClientID     string
	ServiceNowIntegrationServiceClientSecret string
	ServiceNowIntegrationServiceScopes       string
}

// Load reads configuration from environment variables and returns a populated
// Config. Missing variables fall back to sensible defaults; callers should
// validate required fields (e.g. DBUser, DBPassword, DBName) before use.
func Load() *Config {
	return &Config{
		DBHost:     getEnvOrDefault("DB_HOST", "localhost"),
		DBPort:     getEnvOrDefault("DB_PORT", "5432"),
		DBUser:     os.Getenv("DB_USER"),
		DBPassword: os.Getenv("DB_PASSWORD"),
		DBName:     os.Getenv("DB_NAME"),
		DBSSLMode:  os.Getenv("DB_SSLMODE"),
		ServerPort: getEnvOrDefault("SERVER_PORT", "8080"),
		DataSource:             DataSource(getEnvOrDefault("DATA_SOURCE", string(DataSourcePostgres))),
		ServiceNowIntegrationServiceBaseURL:      os.Getenv("SERVICENOW_INTEGRATION_SERVICE_BASE_URL"),
		ServiceNowIntegrationServiceTokenURL:     os.Getenv("SERVICENOW_INTEGRATION_SERVICE_TOKEN_URL"),
		ServiceNowIntegrationServiceClientID:     os.Getenv("SERVICENOW_INTEGRATION_SERVICE_CLIENT_ID"),
		ServiceNowIntegrationServiceClientSecret: os.Getenv("SERVICENOW_INTEGRATION_SERVICE_CLIENT_SECRET"),
		ServiceNowIntegrationServiceScopes:       os.Getenv("SERVICENOW_INTEGRATION_SERVICE_SCOPES"),
	}
}

func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// Validate checks that the configuration is self-consistent. It returns an
// error if DATA_SOURCE is an unrecognised value, or if SERVICENOW_INTEGRATION_SERVICE_BASE_URL is missing
// when DATA_SOURCE=servicenow.
func (c *Config) Validate() error {
	switch c.DataSource {
	case DataSourcePostgres, DataSourceServiceNow:
		// valid
	default:
		return fmt.Errorf("invalid DATA_SOURCE %q: must be %q or %q", c.DataSource, DataSourcePostgres, DataSourceServiceNow)
	}
	if c.DataSource == DataSourceServiceNow {
		if c.ServiceNowIntegrationServiceBaseURL == "" {
			return fmt.Errorf("SERVICENOW_INTEGRATION_SERVICE_BASE_URL is required when DATA_SOURCE=servicenow")
		}
		if c.ServiceNowIntegrationServiceTokenURL == "" {
			return fmt.Errorf("SERVICENOW_INTEGRATION_SERVICE_TOKEN_URL is required when DATA_SOURCE=servicenow")
		}
		if c.ServiceNowIntegrationServiceClientID == "" {
			return fmt.Errorf("SERVICENOW_INTEGRATION_SERVICE_CLIENT_ID is required when DATA_SOURCE=servicenow")
		}
		if c.ServiceNowIntegrationServiceClientSecret == "" {
			return fmt.Errorf("SERVICENOW_INTEGRATION_SERVICE_CLIENT_SECRET is required when DATA_SOURCE=servicenow")
		}
	}
	return nil
}

// DSN constructs a PostgreSQL connection string from the config fields.
func (c *Config) DSN() string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.DBUser, c.DBPassword),
		Host:   c.DBHost + ":" + c.DBPort,
		Path:   c.DBName,
	}
	q := u.Query()
	q.Set("sslmode", c.DBSSLMode)
	u.RawQuery = q.Encode()
	return u.String()
}
