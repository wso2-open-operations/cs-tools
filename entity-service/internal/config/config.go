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
	"strings"
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
	// SupportEngineerRole is the ServiceNow role name (e.g. an org-specific
	// "sn_*" role) whose presence on a case comment's resolved author marks
	// that comment as a qualifying support-engineer response — see
	// sn_case_service.go's applyResponseSLAOnComment. Deliberately no
	// committed default: this is organisation-specific vocabulary, the same
	// reasoning apps/csm-portal/backend's own CSM_TEAM_REGISTRY uses for not
	// shipping one. Left unset, that function simply can't confirm
	// engineer-authorship and skips (logged) — not fatal, not required by
	// Validate.
	SupportEngineerRole string
	// CustomerRoles is a comma-separated list of ServiceNow role names (see
	// SUPPORT_ENGINEER_ROLE's own doc comment for the same
	// organisation-specific-vocabulary reasoning — deliberately no
	// committed default here either) whose presence on a case comment's
	// resolved author marks that comment as a customer reply — see
	// sn_case_service.go's applyCustomerReplyStateTransition, which moves
	// the case back to Work In Progress when a customer replies while it's
	// Awaiting Info/Solution Proposed. Left unset (or empty), that function
	// can't confirm customer-authorship and skips (logged) — not fatal, not
	// required by Validate. Coincidentally shares its name with an
	// unrelated CUSTOMER_ROLES env var in
	// integrations/csm-notification-service (notification-link routing,
	// nothing to do with case state) — the two are read by separate
	// processes/environments and don't interact.
	CustomerRoles []string
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
		EventHubBroker:                           os.Getenv("EVENT_HUB_BROKER"),
		EventHubConnectionString:                 os.Getenv("EVENT_HUB_CONNECTION_STRING"),
		EventHubTopic:                            os.Getenv("EVENT_HUB_TOPIC"),
		EventPublishingEnabled:                   os.Getenv("EVENT_PUBLISHING_ENABLED") == "true",
		SupportEngineerRole:                      os.Getenv("SUPPORT_ENGINEER_ROLE"),
		CustomerRoles:                            splitComma(os.Getenv("CUSTOMER_ROLES")),
	}
}

func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// splitComma parses a comma-separated env var into a trimmed, non-empty
// slice ("" for an unset/empty var, matching integrations/csm-notification-service's
// own copy of this exact helper).
func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			result = append(result, t)
		}
	}
	return result
}

// HasDatabase reports whether a Postgres connection is configured. It is the
// gate cmd/api/main.go uses to decide whether to open a pool at all, and
// routes.go uses to decide whether to register the Postgres-only endpoints
// (event_publish_failures, sla_clocks).
//
// Validate guarantees this is all-or-nothing: either all three of
// DB_USER/DB_PASSWORD/DB_NAME are set, or none are. So checking DBUser alone
// would be equivalent — all three are named here to make the contract obvious
// at the call site rather than relying on that invariant holding elsewhere.
func (c *Config) HasDatabase() bool {
	return c.DBUser != "" && c.DBPassword != "" && c.DBName != ""
}

// Validate checks that the configuration is self-consistent. It returns an
// error if DATA_SOURCE is an unrecognised value, if the DB variables are
// missing when DATA_SOURCE=postgres or only partially set in either mode, if
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

	// DB_USER/DB_PASSWORD/DB_NAME are required only when DATA_SOURCE=postgres,
	// which serves every entity read and write from this pool.
	//
	// When DATA_SOURCE=servicenow they are OPTIONAL. Entity traffic goes to
	// the SN integration service instead, and the two Postgres-only features
	// (event_publish_failures, sla_clocks) degrade to not being registered at
	// all rather than blocking startup — see HasDatabase's call sites in
	// cmd/api/main.go and internal/server/routes.go. Requiring them in every
	// mode would crash-loop existing DB-less servicenow deployments at boot
	// with "DB_USER is required", which is what this branch exists to prevent.
	dbSet := c.DBUser != "" || c.DBPassword != "" || c.DBName != ""
	dbComplete := c.DBUser != "" && c.DBPassword != "" && c.DBName != ""

	if c.DataSource == DataSourcePostgres && !dbComplete {
		if c.DBUser == "" {
			return fmt.Errorf("DB_USER is required when DATA_SOURCE=%s", DataSourcePostgres)
		}
		if c.DBPassword == "" {
			return fmt.Errorf("DB_PASSWORD is required when DATA_SOURCE=%s", DataSourcePostgres)
		}
		return fmt.Errorf("DB_NAME is required when DATA_SOURCE=%s", DataSourcePostgres)
	}

	// A partial set is always a misconfiguration, in either mode — the same
	// all-or-nothing reasoning as the Event Hub group below. Silently running
	// without a database because one of the three was left unset would
	// disable event_publish_failures and sla_clocks without anyone noticing.
	if dbSet && !dbComplete {
		return fmt.Errorf("DB_USER, DB_PASSWORD, and DB_NAME must be set together or not at all")
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
