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
	// ConsumptionSecretKey is the base64-encoded 32-byte AES key used to
	// encrypt the product-consumption credentials at rest (see
	// internal/crypto). Optional, and meaningful only when DataSource is
	// "postgres": when it is empty the project-consumption routes are not
	// registered at all, exactly as the ServiceNow-only routes are absent from
	// a Postgres deployment.
	//
	// It is deliberately not required by Validate. Requiring it would break
	// every existing Postgres deployment that has no interest in product
	// consumption, and a missing key must never silently degrade into storing
	// these credentials in the clear — so "absent" disables the feature rather
	// than weakening it.
	ConsumptionSecretKey string
	// EventHubBroker/EventHubConnectionString/EventHubTopic configure this
	// service's EventPublisherService (internal/service/
	// event_publisher_service.go). Optional — gated on EventHubBroker being
	// set (see routes.go), not required by Validate, mirroring
	// apps/csm-portal/backend's own optional Event Hub wiring: when unset,
	// case.created/incident.created are simply never published and
	// CreateCase/CreateIncident behave exactly as before this was wired in.
	EventHubBroker           string
	EventHubConnectionString string
	EventHubTopic            string
	// EventPublishingEnabled is a separate kill switch on top of
	// EventHubBroker being set — it defaults to false (safe-by-default: an
	// environment can have Event Hub fully configured and still not publish
	// a single event until this is explicitly turned on). routes.go only
	// constructs EventPublisherService when both this is true AND
	// EventHubBroker is set.
	EventPublishingEnabled bool
}

// Load reads configuration from environment variables and returns a populated
// Config. Missing variables fall back to sensible defaults; callers should
// validate required fields (e.g. DBUser, DBPassword, DBName) before use.
func Load() *Config {
	return &Config{
		DBHost:                                   getEnvOrDefault("DB_HOST", "localhost"),
		DBPort:                                   getEnvOrDefault("DB_PORT", "5432"),
		DBUser:                                   os.Getenv("DB_USER"),
		DBPassword:                               os.Getenv("DB_PASSWORD"),
		DBName:                                   os.Getenv("DB_NAME"),
		DBSSLMode:                                os.Getenv("DB_SSLMODE"),
		ServerPort:                               getEnvOrDefault("SERVER_PORT", "8080"),
		DataSource:                               DataSource(getEnvOrDefault("DATA_SOURCE", string(DataSourcePostgres))),
		ServiceNowIntegrationServiceBaseURL:      os.Getenv("SERVICENOW_INTEGRATION_SERVICE_BASE_URL"),
		ServiceNowIntegrationServiceTokenURL:     os.Getenv("SERVICENOW_INTEGRATION_SERVICE_TOKEN_URL"),
		ServiceNowIntegrationServiceClientID:     os.Getenv("SERVICENOW_INTEGRATION_SERVICE_CLIENT_ID"),
		ServiceNowIntegrationServiceClientSecret: os.Getenv("SERVICENOW_INTEGRATION_SERVICE_CLIENT_SECRET"),
		ServiceNowIntegrationServiceScopes:       os.Getenv("SERVICENOW_INTEGRATION_SERVICE_SCOPES"),
		ConsumptionSecretKey:                     os.Getenv("CONSUMPTION_SECRET_KEY"),
		EventHubBroker:                           os.Getenv("EVENT_HUB_BROKER"),
		EventHubConnectionString:                 os.Getenv("EVENT_HUB_CONNECTION_STRING"),
		EventHubTopic:                            os.Getenv("EVENT_HUB_TOPIC"),
		EventPublishingEnabled:                   os.Getenv("EVENT_PUBLISHING_ENABLED") == "true",
	}
}

func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// Validate checks that the configuration is self-consistent. It returns an
// error if DATA_SOURCE is an unrecognised value, if DB_USER/DB_PASSWORD/DB_NAME
// are missing when DATA_SOURCE=postgres (see db.NewPoolIfNeeded), if
// SERVICENOW_INTEGRATION_SERVICE_BASE_URL is missing when
// DATA_SOURCE=servicenow, or if EVENT_HUB_BROKER/EVENT_HUB_CONNECTION_STRING/
// EVENT_HUB_TOPIC are only partially set.
func (c *Config) Validate() error {
	switch c.DataSource {
	case DataSourcePostgres, DataSourceServiceNow:
		// valid
	default:
		return fmt.Errorf("invalid DATA_SOURCE %q: must be %q or %q", c.DataSource, DataSourcePostgres, DataSourceServiceNow)
	}
	// Postgres credentials are required only for DATA_SOURCE=postgres.
	// servicenow mode skips the pool (db.NewPoolIfNeeded) so a local
	// customer-portal can start without a reachable database. Side tables
	// that have no ServiceNow equivalent are registered only when a pool
	// is available — see routes.go.
	if c.DataSource == DataSourcePostgres {
		if c.DBUser == "" {
			return fmt.Errorf("DB_USER is required when DATA_SOURCE=postgres")
		}
		if c.DBPassword == "" {
			return fmt.Errorf("DB_PASSWORD is required when DATA_SOURCE=postgres")
		}
		if c.DBName == "" {
			return fmt.Errorf("DB_NAME is required when DATA_SOURCE=postgres")
		}
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
	// EVENT_HUB_BROKER/EVENT_HUB_CONNECTION_STRING/EVENT_HUB_TOPIC are
	// all-or-nothing (see EventHubBroker's own doc comment and routes.go's
	// EventPublisherService wiring, which only checks EventHubBroker): a
	// partial set would let EventPublisherService get constructed with an
	// empty connection string or topic, so every publish attempt fails
	// silently (logged, doesn't fail case/incident creation — see
	// publishCaseCreated/publishIncidentCreated) while the deployment
	// otherwise looks healthy. Reject that combination at startup instead.
	eventHubSet := c.EventHubBroker != "" || c.EventHubConnectionString != "" || c.EventHubTopic != ""
	eventHubComplete := c.EventHubBroker != "" && c.EventHubConnectionString != "" && c.EventHubTopic != ""
	if eventHubSet && !eventHubComplete {
		return fmt.Errorf("EVENT_HUB_BROKER, EVENT_HUB_CONNECTION_STRING, and EVENT_HUB_TOPIC must be set together or not at all")
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
