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
		// Both ports carry their real defaults: Load always populates them,
		// and Validate rejects the two being equal — which a zero-value
		// Config would be.
		ServerPort: "8080",
		HealthPort: "8081",
		// Token validation is always on (no config flag disables it), so
		// these three are as mandatory to a valid Config as the DB settings
		// above — see TestConfig_Validate_Auth for the dedicated tests.
		AuthIssuer:             "https://api.asgardeo.io/t/x/oauth2/token",
		AuthJWKSURL:            "https://api.asgardeo.io/t/x/oauth2/jwks",
		AuthUserTokenAudiences: []string{"spa"},
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

func TestConfig_Validate_SalesEntityAllOrNothing(t *testing.T) {
	tests := []struct {
		name         string
		baseURL      string
		tokenURL     string
		clientID     string
		clientSecret string
		scopes       string
		wantErr      bool
	}{
		{name: "none set", wantErr: false},
		{name: "all four set", baseURL: "b", tokenURL: "t", clientID: "c", clientSecret: "s", wantErr: false},
		{name: "four plus scopes", baseURL: "b", tokenURL: "t", clientID: "c", clientSecret: "s", scopes: "x", wantErr: false},
		{name: "only scopes", scopes: "x", wantErr: true},
		{name: "only base URL", baseURL: "b", wantErr: true},
		{name: "only token URL", tokenURL: "t", wantErr: true},
		{name: "only client ID", clientID: "c", wantErr: true},
		{name: "missing client secret", baseURL: "b", tokenURL: "t", clientID: "c", wantErr: true},
		{name: "missing base URL", tokenURL: "t", clientID: "c", clientSecret: "s", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := baseValidConfig()
			c.SalesEntityBaseURL = tt.baseURL
			c.SalesEntityTokenURL = tt.tokenURL
			c.SalesEntityClientID = tt.clientID
			c.SalesEntityClientSecret = tt.clientSecret
			c.SalesEntityScopes = tt.scopes

			err := c.Validate()
			if tt.wantErr && err == nil {
				t.Error("Validate() = nil, want an error for a partial sales-entity configuration")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestConfig_SalesEntityConfigured(t *testing.T) {
	c := baseValidConfig()
	if c.SalesEntityConfigured() {
		t.Fatal("SalesEntityConfigured() = true, want false when unset")
	}
	c.SalesEntityBaseURL = "b"
	c.SalesEntityTokenURL = "t"
	c.SalesEntityClientID = "c"
	c.SalesEntityClientSecret = "s"
	if !c.SalesEntityConfigured() {
		t.Fatal("SalesEntityConfigured() = false, want true when all four are set")
	}
}

func TestConfig_Validate_InvalidDataSource(t *testing.T) {
	c := baseValidConfig()
	c.DataSource = DataSource("not-a-real-source")
	if err := c.Validate(); err == nil {
		t.Error("Validate() = nil, want an error for an invalid DATA_SOURCE")
	}
}

// baseValidPostgresServiceNowDualWriteConfig returns a minimally valid Config
// for DATA_SOURCE=postgres-servicenow-dual-write: both a full database (reads
// and writes are always Postgres-authoritative in this mode) AND full
// ServiceNow integration service credentials (the best-effort mirror write
// goes there) are required.
func baseValidPostgresServiceNowDualWriteConfig() Config {
	c := baseValidConfig()
	c.DataSource = DataSourcePostgresServiceNowDualWrite
	c.ServiceNowIntegrationServiceBaseURL = "https://example.com"
	c.ServiceNowIntegrationServiceTokenURL = "https://example.com/token"
	c.ServiceNowIntegrationServiceClientID = "client-id"
	c.ServiceNowIntegrationServiceClientSecret = "client-secret"
	return c
}

func TestConfig_Validate_PostgresServiceNowDualWriteIsValid(t *testing.T) {
	c := baseValidPostgresServiceNowDualWriteConfig()
	if err := c.Validate(); err != nil {
		t.Fatalf("unexpected error for a fully configured postgres-servicenow-dual-write source: %v", err)
	}
}

// TestConfig_Validate_PostgresServiceNowDualWriteRequiresDBFields guards the
// "Postgres is authoritative" half of the new mode: unlike plain
// DATA_SOURCE=servicenow, the DB cannot be dropped here.
func TestConfig_Validate_PostgresServiceNowDualWriteRequiresDBFields(t *testing.T) {
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
			c := baseValidPostgresServiceNowDualWriteConfig()
			tt.mutate(&c)
			if err := c.Validate(); err == nil {
				t.Errorf("Validate() = nil, want an error when %s", tt.name)
			}
		})
	}
}

// TestConfig_Validate_PostgresServiceNowDualWriteRequiresIntegrationServiceFields
// guards the "ServiceNow mirror write" half: unlike plain
// DATA_SOURCE=postgres, the SN integration service credentials cannot be
// dropped here — Dispatch has nowhere to send the mirror write without them.
func TestConfig_Validate_PostgresServiceNowDualWriteRequiresIntegrationServiceFields(t *testing.T) {
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
			c := baseValidPostgresServiceNowDualWriteConfig()
			tt.mutate(&c)
			if err := c.Validate(); err == nil {
				t.Errorf("Validate() = nil, want an error when %s", tt.name)
			}
		})
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

func TestConfig_Validate_ServiceNowDoesNotRequireDBFields(t *testing.T) {
	c := baseValidServiceNowConfig()
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil when DATA_SOURCE=servicenow has no DB credentials", err)
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

func TestConfig_Validate_RejectsPortsThatResolveToTheSameNumber(t *testing.T) {
	// A string comparison would wave "8080"/"08080" through: different
	// strings, same TCP port, so both listeners race for one port and the
	// process ends up half dead.
	c := baseValidConfig()
	c.ServerPort = "8080"
	c.HealthPort = "08080"
	if err := c.Validate(); err == nil {
		t.Error("Validate() = nil, want an error when the two ports resolve to the same number")
	}
}

func TestConfig_Validate_RejectsUnbindablePort(t *testing.T) {
	// Caught here, naming the offending variable, rather than at
	// ListenAndServe time inside a goroutine.
	for _, port := range []string{"99999", "not-a-port", "-1"} {
		t.Run(port, func(t *testing.T) {
			c := baseValidConfig()
			c.HealthPort = port
			if err := c.Validate(); err == nil {
				t.Errorf("Validate() = nil, want an error for HEALTH_PORT %q", port)
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
		// Both ports carry their real defaults, same reasoning as
		// baseValidConfig above — Validate rejects the two being equal,
		// which a zero-value Config would be.
		ServerPort: "8080",
		HealthPort: "8081",
		// Token validation is always on regardless of DataSource — see
		// baseValidConfig's own comment.
		AuthIssuer:             "https://api.asgardeo.io/t/x/oauth2/token",
		AuthJWKSURL:            "https://api.asgardeo.io/t/x/oauth2/jwks",
		AuthUserTokenAudiences: []string{"spa"},
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
// sla-status stay available.
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

func TestConfig_Validate_RejectsHealthPortCollidingWithServerPort(t *testing.T) {
	// The health listener is a separate server precisely so only its own
	// routes are reachable at public visibility. Sharing a port would mean
	// the second ListenAndServe fails with "address already in use" after
	// the first is already serving — the process stays up with one of the
	// two ports simply dead, which is exactly the kind of failure the
	// health endpoint is meant to surface rather than suffer from.
	c := baseValidConfig()
	c.HealthPort = c.ServerPort
	if err := c.Validate(); err == nil {
		t.Error("Validate() = nil, want an error when HEALTH_PORT equals SERVER_PORT")
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

func TestParseInternalClientIDs(t *testing.T) {
	got := ParseInternalClientIDs(" csm-portal , csm-integration ,")
	if len(got) != 2 || !got["csm-portal"] || !got["csm-integration"] {
		t.Fatalf("got %v", got)
	}
	if got := ParseInternalClientIDs(""); len(got) != 0 {
		t.Fatalf("empty must be a valid empty set, got %v", got)
	}
	// A client id absent from the set is simply not internal -- there is no
	// error case here (unlike the old clientId=role grammar): any non-empty,
	// trimmed entry is a valid client id.
	if got := ParseInternalClientIDs("a,,b"); len(got) != 2 || !got["a"] || !got["b"] {
		t.Errorf("blank entries between commas should just be skipped, got %v", got)
	}
}

// TestLoad_CSMMigrationPortalWritesEnabled pins the kill switch's parsing:
// only the exact string "true" turns the portal membership writes on, so a
// typo, a "1", or a "TRUE" leaves them off rather than half-enabling a write
// path that touches Salesforce.
func TestLoad_CSMMigrationPortalWritesEnabled(t *testing.T) {
	for value, want := range map[string]bool{
		"true": true, "TRUE": false, "True": false, "1": false, "yes": false, "": false, " true ": false,
	} {
		t.Setenv("CSM_MIGRATION_PORTAL_WRITES_ENABLED", value)
		if got := Load().CSMMigrationPortalWritesEnabled; got != want {
			t.Errorf("CSM_MIGRATION_PORTAL_WRITES_ENABLED=%q -> %v, want %v", value, got, want)
		}
	}
}

// TestLoad_CSMMigrationMembershipRegistrationEnabled pins the same parse for
// the registration kill switch. It gates POST /users/me/memberships/register,
// which clears a contact's Salesforce lockout and flips the membership to
// REGISTERED, so a "TRUE" or a "1" must leave the route unregistered rather
// than half-enabling a path that writes to Salesforce.
func TestLoad_CSMMigrationMembershipRegistrationEnabled(t *testing.T) {
	for value, want := range map[string]bool{
		"true": true, "TRUE": false, "True": false, "1": false, "yes": false, "": false, " true ": false,
	} {
		t.Setenv("CSM_MIGRATION_MEMBERSHIP_REGISTRATION_ENABLED", value)
		if got := Load().CSMMigrationMembershipRegistrationEnabled; got != want {
			t.Errorf("CSM_MIGRATION_MEMBERSHIP_REGISTRATION_ENABLED=%q -> %v, want %v", value, got, want)
		}
	}
}

// TestConfig_HasPortalMembershipWrites covers the whole gate, not just the
// flag: the writes are a Postgres transaction whose other half is a
// Salesforce call, so both the data source and a complete
// sales-entity-service connection are part of it.
func TestConfig_HasPortalMembershipWrites(t *testing.T) {
	complete := func() Config {
		c := baseValidConfig()
		c.DataSource = DataSourcePostgres
		c.SalesEntityBaseURL = "https://example.invalid"
		c.SalesEntityTokenURL = "https://example.invalid/oauth2/token"
		c.SalesEntityClientID = "id"
		c.SalesEntityClientSecret = "secret"
		c.CSMMigrationPortalWritesEnabled = true
		return c
	}
	if c := complete(); !c.HasPortalMembershipWrites() {
		t.Error("a complete configuration with the flag on must enable the writes")
	}
	for name, mod := range map[string]func(*Config){
		"flag off":               func(c *Config) { c.CSMMigrationPortalWritesEnabled = false },
		"servicenow data source": func(c *Config) { c.DataSource = DataSourceServiceNow },
		"dual-write data source": func(c *Config) { c.DataSource = DataSourcePostgresServiceNowDualWrite },
		"no sales entity base":   func(c *Config) { c.SalesEntityBaseURL = "" },
		"no sales entity secret": func(c *Config) { c.SalesEntityClientSecret = "" },
		"no sales entity at all": func(c *Config) {
			c.SalesEntityBaseURL, c.SalesEntityTokenURL, c.SalesEntityClientID, c.SalesEntityClientSecret = "", "", "", ""
		},
		"no sales entity client": func(c *Config) { c.SalesEntityClientID = "" },
		"no sales entity token":  func(c *Config) { c.SalesEntityTokenURL = "" },
	} {
		c := complete()
		mod(&c)
		if c.HasPortalMembershipWrites() {
			t.Errorf("%s: the writes must stay off", name)
		}
	}
}

// TestConfig_Validate_Auth locks in that token validation has no off switch:
// AuthIssuer/AuthJWKSURL/AuthUserTokenAudiences are as mandatory to a valid
// Config as the DB settings baseValidConfig() already supplies.
func TestConfig_Validate_Auth(t *testing.T) {
	if c := baseValidConfig(); c.Validate() != nil {
		t.Fatalf("complete auth config rejected: %v", c.Validate())
	}
	for name, mod := range map[string]func(*Config){
		"missing issuer":    func(c *Config) { c.AuthIssuer = "" },
		"missing JWKS URL":  func(c *Config) { c.AuthJWKSURL = "" },
		"missing audiences": func(c *Config) { c.AuthUserTokenAudiences = nil },
	} {
		c := baseValidConfig()
		mod(&c)
		if err := c.Validate(); err == nil {
			t.Errorf("%s: want a startup error", name)
		}
	}
}
