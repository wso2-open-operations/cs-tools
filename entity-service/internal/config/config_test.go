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

import "testing"

// baseValidConfig returns a minimally valid postgres-backed Config so each
// test only needs to override the field(s) under test.
func baseValidConfig() Config {
	return Config{
		DataSource: DataSourcePostgres,
		DBUser:     "user",
		DBPassword: "password",
		DBName:     "db",
	}
}

func TestConfig_Validate_BaseConfigIsValid(t *testing.T) {
	c := baseValidConfig()
	if err := c.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestConfig_Validate_EventHubAllOrNothing verifies EVENT_HUB_BROKER/
// EVENT_HUB_CONNECTION_STRING/EVENT_HUB_TOPIC must be set together or not at
// all — a partial set would let routes.go construct EventPublisherService
// with an empty connection string or topic, so every publish attempt would
// fail silently while the deployment otherwise looks healthy.
func TestConfig_Validate_EventHubAllOrNothing(t *testing.T) {
	tests := []struct {
		name          string
		broker        string
		connectionStr string
		topic         string
		wantErr       bool
	}{
		{name: "none set", wantErr: false},
		{name: "all three set", broker: "b", connectionStr: "c", topic: "t", wantErr: false},
		{name: "only broker", broker: "b", wantErr: true},
		{name: "only connection string", connectionStr: "c", wantErr: true},
		{name: "only topic", topic: "t", wantErr: true},
		{name: "broker and connection string, missing topic", broker: "b", connectionStr: "c", wantErr: true},
		{name: "broker and topic, missing connection string", broker: "b", topic: "t", wantErr: true},
		{name: "connection string and topic, missing broker", connectionStr: "c", topic: "t", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := baseValidConfig()
			c.EventHubBroker = tt.broker
			c.EventHubConnectionString = tt.connectionStr
			c.EventHubTopic = tt.topic

			err := c.Validate()
			if tt.wantErr && err == nil {
				t.Error("Validate() = nil, want an error for a partial Event Hub configuration")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestConfig_Validate_SalesforceAllOrNothing(t *testing.T) {
	tests := []struct {
		name         string
		baseURL      string
		tokenURL     string
		clientID     string
		clientSecret string
		refreshToken string
		wantErr      bool
	}{
		{name: "none set", wantErr: false},
		{name: "all five set", baseURL: "b", tokenURL: "t", clientID: "c", clientSecret: "s", refreshToken: "r", wantErr: false},
		{name: "only base URL", baseURL: "b", wantErr: true},
		{name: "only token URL", tokenURL: "t", wantErr: true},
		{name: "only client ID", clientID: "c", wantErr: true},
		{name: "missing refresh token", baseURL: "b", tokenURL: "t", clientID: "c", clientSecret: "s", wantErr: true},
		{name: "missing base URL", tokenURL: "t", clientID: "c", clientSecret: "s", refreshToken: "r", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := baseValidConfig()
			c.SalesforceBaseURL = tt.baseURL
			c.SalesforceTokenURL = tt.tokenURL
			c.SalesforceClientID = tt.clientID
			c.SalesforceClientSecret = tt.clientSecret
			c.SalesforceRefreshToken = tt.refreshToken

			err := c.Validate()
			if tt.wantErr && err == nil {
				t.Error("Validate() = nil, want an error for a partial Salesforce configuration")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestConfig_SalesforceConfigured(t *testing.T) {
	c := baseValidConfig()
	if c.SalesforceConfigured() {
		t.Fatal("SalesforceConfigured() = true, want false when unset")
	}
	c.SalesforceBaseURL = "b"
	c.SalesforceTokenURL = "t"
	c.SalesforceClientID = "c"
	c.SalesforceClientSecret = "s"
	c.SalesforceRefreshToken = "r"
	if !c.SalesforceConfigured() {
		t.Fatal("SalesforceConfigured() = false, want true when all five are set")
	}
}

func TestConfig_Validate_InvalidDataSource(t *testing.T) {
	c := baseValidConfig()
	c.DataSource = DataSource("not-a-real-source")
	if err := c.Validate(); err == nil {
		t.Error("Validate() = nil, want an error for an invalid DATA_SOURCE")
	}
}

func TestConfig_Validate_RequiresDBFields(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(c *Config)
	}{
		{"missing DBUser", func(c *Config) { c.DBUser = "" }},
		{"missing DBPassword", func(c *Config) { c.DBPassword = "" }},
		{"missing DBName", func(c *Config) { c.DBName = "" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := baseValidConfig()
			tt.mutate(&c)
			if err := c.Validate(); err == nil {
				t.Errorf("Validate() = nil, want an error when %s", tt.name)
			}
		})
	}
}

func TestConfig_Validate_ServiceNowRequiresIntegrationServiceFields(t *testing.T) {
	base := func() Config {
		c := baseValidConfig()
		c.DataSource = DataSourceServiceNow
		c.ServiceNowIntegrationServiceBaseURL = "https://example.com"
		c.ServiceNowIntegrationServiceTokenURL = "https://example.com/token"
		c.ServiceNowIntegrationServiceClientID = "client-id"
		c.ServiceNowIntegrationServiceClientSecret = "client-secret"
		return c
	}

	valid := base()
	if err := valid.Validate(); err != nil {
		t.Fatalf("unexpected error for a fully configured servicenow source: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(c *Config)
	}{
		{"missing base URL", func(c *Config) { c.ServiceNowIntegrationServiceBaseURL = "" }},
		{"missing token URL", func(c *Config) { c.ServiceNowIntegrationServiceTokenURL = "" }},
		{"missing client ID", func(c *Config) { c.ServiceNowIntegrationServiceClientID = "" }},
		{"missing client secret", func(c *Config) { c.ServiceNowIntegrationServiceClientSecret = "" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := base()
			tt.mutate(&c)
			if err := c.Validate(); err == nil {
				t.Errorf("Validate() = nil, want an error when %s", tt.name)
			}
		})
	}
}

// baseValidServiceNowConfig returns a minimally valid servicenow-backed
// Config with NO database configured — the DB-less deployment shape this
// service must keep supporting.
func baseValidServiceNowConfig() Config {
	return Config{
		DataSource:                               DataSourceServiceNow,
		ServiceNowIntegrationServiceBaseURL:      "https://example.com",
		ServiceNowIntegrationServiceTokenURL:     "https://example.com/token",
		ServiceNowIntegrationServiceClientID:     "client-id",
		ServiceNowIntegrationServiceClientSecret: "client-secret",
	}
}

// TestConfig_Validate_ServiceNowDatabaseIsOptional is the regression guard for
// the crash-loop this branch exists to prevent: requiring DB_USER/DB_PASSWORD/
// DB_NAME in every mode would fail startup for existing DB-less
// DATA_SOURCE=servicenow deployments, which serve every entity endpoint from
// the SN integration service and never touch Postgres.
func TestConfig_Validate_ServiceNowDatabaseIsOptional(t *testing.T) {
	c := baseValidServiceNowConfig()
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil for servicenow with no database configured", err)
	}
	if c.HasDatabase() {
		t.Error("HasDatabase() = true, want false when no DB variables are set")
	}
}

// TestConfig_Validate_ServiceNowAcceptsAFullDatabase covers the other valid
// servicenow shape — a database IS configured, so event_publish_failures and
// sla_clocks stay available.
func TestConfig_Validate_ServiceNowAcceptsAFullDatabase(t *testing.T) {
	c := baseValidServiceNowConfig()
	c.DBUser, c.DBPassword, c.DBName = "user", "password", "db"
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil for servicenow with a full database config", err)
	}
	if !c.HasDatabase() {
		t.Error("HasDatabase() = false, want true when all three DB variables are set")
	}
}

// TestConfig_Validate_DatabaseAllOrNothing verifies a partial DB set is
// rejected in BOTH modes. Without this, a typo in one variable would silently
// disable the Postgres-only endpoints on a servicenow deployment rather than
// failing loudly — the same reasoning as the Event Hub group.
func TestConfig_Validate_DatabaseAllOrNothing(t *testing.T) {
	tests := []struct {
		name     string
		user     string
		password string
		dbName   string
		wantErr  bool
	}{
		{name: "none set", wantErr: false},
		{name: "all three set", user: "u", password: "p", dbName: "d", wantErr: false},
		{name: "only user", user: "u", wantErr: true},
		{name: "only password", password: "p", wantErr: true},
		{name: "only name", dbName: "d", wantErr: true},
		{name: "user and password, missing name", user: "u", password: "p", wantErr: true},
		{name: "user and name, missing password", user: "u", dbName: "d", wantErr: true},
		{name: "password and name, missing user", password: "p", dbName: "d", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := baseValidServiceNowConfig()
			c.DBUser, c.DBPassword, c.DBName = tt.user, tt.password, tt.dbName

			err := c.Validate()
			if tt.wantErr && err == nil {
				t.Error("Validate() = nil, want an error for a partial database configuration")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Validate() = %v, want nil", err)
			}
		})
	}
}

// TestConfig_Validate_PostgresStillRequiresDatabase guards the other side:
// making the DB optional for servicenow must not make it optional for
// postgres, where every entity read and write depends on the pool.
func TestConfig_Validate_PostgresStillRequiresDatabase(t *testing.T) {
	c := Config{DataSource: DataSourcePostgres}
	if err := c.Validate(); err == nil {
		t.Error("Validate() = nil, want an error for postgres with no database configured")
	}
}
