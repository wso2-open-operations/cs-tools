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
	"net/url"
	"strings"
	"testing"
	"time"
)

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
		// Timeouts carry their real defaults for the same reason: Load always
		// populates them and Validate rejects non-positive values.
		ServerReadTimeout:     DefaultServerReadTimeout,
		ServerWriteTimeout:    DefaultServerWriteTimeout,
		RequestTimeout:        DefaultRequestTimeout,
		UpstreamClientTimeout: DefaultUpstreamClientTimeout,
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
		ServerReadTimeout:      DefaultServerReadTimeout,
		ServerWriteTimeout:     DefaultServerWriteTimeout,
		RequestTimeout:         DefaultRequestTimeout,
		UpstreamClientTimeout:  DefaultUpstreamClientTimeout,
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

// TestConfig_Validate_CSMPortalBackendClientIDAndDomainAllOrNothing pins the pairing
// requirement: CSM_PORTAL_BACKEND_CLIENT_ID and CSM_PORTAL_USER_DOMAIN are only
// meaningful together (ResolveScope's domain check needs both), so a
// deployment setting only one almost certainly meant to set both.
func TestConfig_Validate_CSMPortalBackendClientIDAndDomainAllOrNothing(t *testing.T) {
	c := baseValidConfig()
	c.CSMPortalBackendClientID = "csm-portal"
	if err := c.Validate(); err == nil {
		t.Error("CSMPortalBackendClientID with no CSMPortalUserDomain: want an error, got nil")
	}

	c = baseValidConfig()
	c.CSMPortalUserDomain = "wso2.com"
	if err := c.Validate(); err == nil {
		t.Error("CSMPortalUserDomain with no CSMPortalBackendClientID: want an error, got nil")
	}

	c = baseValidConfig()
	c.CSMPortalBackendClientID = "csm-portal"
	c.CSMPortalUserDomain = "wso2.com"
	if err := c.Validate(); err != nil {
		t.Errorf("both set together: unexpected error: %v", err)
	}
}

// TestConfig_Validate_RejectsSameClientIDForCSMAndCustomerPortal pins the
// guard against the one config value that can't be resolved by ResolveScope's
// own ordering: CSMPortalBackendClientID and CustomerPortalBackendClientID being equal would
// mean a single client id is both "unrestricted given a matching domain" and
// "never unrestricted, full stop" at once -- a copy-paste mistake, not a
// valid deployment.
func TestConfig_Validate_RejectsSameClientIDForCSMAndCustomerPortal(t *testing.T) {
	c := baseValidConfig()
	c.CSMPortalBackendClientID = "shared-id"
	c.CSMPortalUserDomain = "wso2.com"
	c.CustomerPortalBackendClientID = "shared-id"
	if err := c.Validate(); err == nil {
		t.Error("CSMPortalBackendClientID == CustomerPortalBackendClientID: want an error, got nil")
	}
}

// TestConfig_Validate_DistinctCSMAndCustomerPortalBackendClientIDsAreValid guards
// against the above check being too broad and rejecting the normal case.
func TestConfig_Validate_DistinctCSMAndCustomerPortalBackendClientIDsAreValid(t *testing.T) {
	c := baseValidConfig()
	c.CSMPortalBackendClientID = "csm-portal"
	c.CSMPortalUserDomain = "wso2.com"
	c.CustomerPortalBackendClientID = "customer-portal"
	if err := c.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestLoad_CSMPortalUserDomain pins CSM_PORTAL_USER_DOMAIN's one bit of
// normalization: a value typed with a leading "@" (an easy mistake, since
// email addresses are usually written that way) is accepted the same as one
// without, so isCSMPortalUserDomain's own "@"+domain suffix match is never
// built from a doubled "@@".
func TestLoad_CSMPortalUserDomain(t *testing.T) {
	t.Setenv("CSM_PORTAL_USER_DOMAIN", "@wso2.com")
	if got := Load().CSMPortalUserDomain; got != "wso2.com" {
		t.Errorf("CSMPortalUserDomain = %q, want %q (leading @ stripped)", got, "wso2.com")
	}

	t.Setenv("CSM_PORTAL_USER_DOMAIN", "wso2.com")
	if got := Load().CSMPortalUserDomain; got != "wso2.com" {
		t.Errorf("CSMPortalUserDomain = %q, want %q (unchanged)", got, "wso2.com")
	}
}

// TestLoad_M2MClientIDsFieldName guards against M2M_CLIENT_IDS silently
// going unread after the AUTH_INTERNAL_CLIENT_IDS rename -- a stale env var
// name here would leave every M2M caller unexpectedly unauthorized.
func TestLoad_M2MClientIDsFieldName(t *testing.T) {
	t.Setenv("M2M_CLIENT_IDS", "svc-a,svc-b")
	got := Load().M2MClientIDs
	if !got["svc-a"] || !got["svc-b"] || len(got) != 2 {
		t.Errorf("M2MClientIDs = %v, want {svc-a, svc-b}", got)
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
	// Dual-write serves memberships from Postgres too; ServiceNow gets them
	// from Salesforce directly, so the writes run there as well.
	dualWrite := complete()
	dualWrite.DataSource = DataSourcePostgresServiceNowDualWrite
	if !dualWrite.HasPortalMembershipWrites() {
		t.Error("dual-write with the flag on must enable the writes")
	}
	for name, mod := range map[string]func(*Config){
		"flag off":               func(c *Config) { c.CSMMigrationPortalWritesEnabled = false },
		"servicenow data source": func(c *Config) { c.DataSource = DataSourceServiceNow },
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

// TestConfig_Validate_EscalationGroupIDsOptional confirms every
// Escalation*GroupID stays optional -- a completely unset set must still
// validate, since not every deployment configures every tier on day one.
func TestConfig_Validate_EscalationGroupIDsOptional(t *testing.T) {
	c := baseValidConfig()
	if err := c.Validate(); err != nil {
		t.Fatalf("unexpected error with every Escalation*GroupID unset: %v", err)
	}
}

// TestConfig_Validate_EscalationGroupIDsMustBeUUIDsIfSet confirms a SET
// Escalation*GroupID is checked for being a well-formed UUID -- a typo'd
// group id would otherwise silently resolve zero recipients at request time
// instead of failing loudly at startup.
func TestConfig_Validate_EscalationGroupIDsMustBeUUIDsIfSet(t *testing.T) {
	validID := "11111111-1111-1111-1111-111111111111"
	c := baseValidConfig()
	c.EscalationEL1AmericasTLGroupID = validID
	if err := c.Validate(); err != nil {
		t.Fatalf("a well-formed group id must not be rejected: %v", err)
	}

	c = baseValidConfig()
	c.EscalationEL5CEOGroupID = "not-a-uuid"
	if err := c.Validate(); err == nil {
		t.Fatal("want a startup error for a malformed group id")
	}
}

// TestConfig_PostgresAuthoritative: the onboarding features run wherever
// PostgreSQL is the system of record, and nowhere else.
func TestConfig_PostgresAuthoritative(t *testing.T) {
	for ds, want := range map[DataSource]bool{
		DataSourcePostgres:                    true,
		DataSourcePostgresServiceNowDualWrite: true,
		DataSourceServiceNow:                  false,
		"":                                    false,
	} {
		c := Config{DataSource: ds}
		if got := c.PostgresAuthoritative(); got != want {
			t.Errorf("DataSource %q: PostgresAuthoritative() = %v, want %v", ds, got, want)
		}
	}
}

// TestLoad_SalesforceIngestRetryInterval pins the one interval that can be
// switched off: unset is the 5m default, an explicit zero disables the retry
// job, and a typo or a negative value costs the override, not the default.
func TestLoad_SalesforceIngestRetryInterval(t *testing.T) {
	for value, want := range map[string]time.Duration{
		"": 5 * time.Minute, "0": 0, "0s": 0, "0m": 0, "2m": 2 * time.Minute, "90s": 90 * time.Second,
		// Invalid values fail closed: disabled, not the default.
		"bogus": 0, "off": 0, "-1m": 0,
	} {
		t.Setenv("SALESFORCE_INGEST_RETRY_INTERVAL", value)
		if got := Load().SalesforceIngestRetryInterval; got != want {
			t.Errorf("SALESFORCE_INGEST_RETRY_INTERVAL=%q -> %v, want %v", value, got, want)
		}
	}
}

func TestConfig_Validate_CustomerEngagementFirefightingTypeID(t *testing.T) {
	c := baseValidConfig()
	c.CSMMigrationCustomerEngagementIngestEnabled = true
	if err := c.Validate(); err != nil {
		t.Fatalf("an unset type id must not fail startup: %v", err)
	}
	c.CustomerEngagementFirefightingTypeID = "fc7f2d171b81f910d64e64a2604bcb9b"
	if err := c.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c.CustomerEngagementFirefightingTypeID = "not-a-sys-id"
	if c.Validate() == nil {
		t.Error("Validate() = nil for a malformed type id")
	}
	if !c.HasCustomerEngagementIngest() {
		t.Error("HasCustomerEngagementIngest() = false on a Postgres config")
	}
	c.DataSource = DataSourceServiceNow
	if c.HasCustomerEngagementIngest() {
		t.Error("HasCustomerEngagementIngest() = true on a ServiceNow config")
	}
}

// TestConfig_Validate_RedisURL: a malformed REDIS_URL fails startup, and the
// error never echoes the URL, since it carries the Redis password.
func TestConfig_Validate_RedisURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "unset", url: "", wantErr: false},
		{name: "tls", url: "rediss://:s3cr3t%3D@cache.example.net:10000", wantErr: false},
		{name: "plain", url: "redis://localhost:6379/0", wantErr: false},
		{name: "wrong scheme", url: "https://:s3cr3t@cache.example.net", wantErr: true},
		{name: "no host", url: "rediss://:s3cr3t@", wantErr: true},
		{name: "unparseable", url: "rediss://:s3cr3t@[::1", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := baseValidConfig()
			c.RedisURL = tt.url
			err := c.Validate()
			if tt.wantErr != (err != nil) {
				t.Fatalf("Validate() = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && strings.Contains(err.Error(), "s3cr3t") {
				t.Errorf("Validate() error leaks the password: %v", err)
			}
		})
	}
}

func TestLoad_TimeoutDefaults(t *testing.T) {
	for _, k := range []string{"SERVER_READ_TIMEOUT", "SERVER_WRITE_TIMEOUT", "REQUEST_TIMEOUT", "UPSTREAM_CLIENT_TIMEOUT"} {
		t.Setenv(k, "")
	}
	c := Load()
	if c.ServerReadTimeout != 60*time.Second || c.ServerWriteTimeout != 60*time.Second ||
		c.RequestTimeout != 60*time.Second || c.UpstreamClientTimeout != 60*time.Second {
		t.Errorf("defaults = %v/%v/%v/%v, want 60s/60s/60s/60s",
			c.ServerReadTimeout, c.ServerWriteTimeout, c.RequestTimeout, c.UpstreamClientTimeout)
	}
	c.DBUser, c.DBPassword, c.DBName = "u", "p", "d"
	c.AuthIssuer, c.AuthJWKSURL, c.AuthUserTokenAudiences = "https://issuer.example/token", "https://issuer.example/jwks", []string{"spa"}
	if err := c.Validate(); err != nil {
		t.Errorf("Validate() with defaults = %v, want nil", err)
	}
}

func TestLoad_TimeoutOverrides(t *testing.T) {
	t.Setenv("SERVER_READ_TIMEOUT", "2m")
	t.Setenv("SERVER_WRITE_TIMEOUT", "90s")
	t.Setenv("REQUEST_TIMEOUT", "80s")
	t.Setenv("UPSTREAM_CLIENT_TIMEOUT", "75s")
	c := Load()
	c.DBUser, c.DBPassword, c.DBName = "u", "p", "d"
	c.AuthIssuer, c.AuthJWKSURL, c.AuthUserTokenAudiences = "https://issuer.example/token", "https://issuer.example/jwks", []string{"spa"}
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
	if c.ServerReadTimeout != 2*time.Minute || c.ServerWriteTimeout != 90*time.Second ||
		c.RequestTimeout != 80*time.Second || c.UpstreamClientTimeout != 75*time.Second {
		t.Errorf("overrides not applied: %v/%v/%v/%v",
			c.ServerReadTimeout, c.ServerWriteTimeout, c.RequestTimeout, c.UpstreamClientTimeout)
	}
}

func TestLoad_InvalidTimeoutFailsValidate(t *testing.T) {
	for _, k := range []string{"SERVER_READ_TIMEOUT", "SERVER_WRITE_TIMEOUT", "REQUEST_TIMEOUT", "UPSTREAM_CLIENT_TIMEOUT"} {
		t.Run(k, func(t *testing.T) {
			t.Setenv(k, "fifty")
			c := Load()
			c.DBUser, c.DBPassword, c.DBName = "u", "p", "d"
			c.AuthIssuer, c.AuthJWKSURL, c.AuthUserTokenAudiences = "https://issuer.example/token", "https://issuer.example/jwks", []string{"spa"}
			err := c.Validate()
			if err == nil || !strings.Contains(err.Error(), k) {
				t.Errorf("Validate() = %v, want an error naming %s", err, k)
			}
		})
	}
}

func TestLoad_DBPoolDefaults(t *testing.T) {
	for _, k := range []string{"DB_POOL_MAX_CONNS", "DB_POOL_MIN_CONNS", "DB_POOL_MAX_CONN_LIFETIME", "DB_POOL_MAX_CONN_IDLE_TIME"} {
		t.Setenv(k, "")
	}
	c := Load()
	if c.DBPoolMaxConns != 20 || c.DBPoolMinConns != 2 ||
		c.DBPoolMaxConnLifetime != 30*time.Minute || c.DBPoolMaxConnIdleTime != 5*time.Minute {
		t.Errorf("defaults = %d/%d/%v/%v, want 20/2/30m/5m",
			c.DBPoolMaxConns, c.DBPoolMinConns, c.DBPoolMaxConnLifetime, c.DBPoolMaxConnIdleTime)
	}
	c.DBUser, c.DBPassword, c.DBName = "u", "p", "d"
	c.AuthIssuer, c.AuthJWKSURL, c.AuthUserTokenAudiences = "https://issuer.example/token", "https://issuer.example/jwks", []string{"spa"}
	if err := c.Validate(); err != nil {
		t.Errorf("Validate() with defaults = %v, want nil", err)
	}
}

func TestLoad_DBPoolOverrides(t *testing.T) {
	t.Setenv("DB_POOL_MAX_CONNS", "50")
	t.Setenv("DB_POOL_MIN_CONNS", "5")
	t.Setenv("DB_POOL_MAX_CONN_LIFETIME", "10m")
	t.Setenv("DB_POOL_MAX_CONN_IDLE_TIME", "2m")
	c := Load()
	if c.DBPoolMaxConns != 50 || c.DBPoolMinConns != 5 ||
		c.DBPoolMaxConnLifetime != 10*time.Minute || c.DBPoolMaxConnIdleTime != 2*time.Minute {
		t.Errorf("overrides not applied: %d/%d/%v/%v",
			c.DBPoolMaxConns, c.DBPoolMinConns, c.DBPoolMaxConnLifetime, c.DBPoolMaxConnIdleTime)
	}
}

func TestLoad_InvalidDBPoolIntFallsBackToDefaultAndFailsValidate(t *testing.T) {
	for _, tc := range []struct {
		key, value string
	}{
		{"DB_POOL_MAX_CONNS", "fifty"},
		{"DB_POOL_MIN_CONNS", "-1"},
		{"DB_POOL_MAX_CONNS", "-3"},
		{"DB_POOL_MAX_CONNS", "0"},
	} {
		t.Run(tc.key+"="+tc.value, func(t *testing.T) {
			t.Setenv(tc.key, tc.value)
			c := Load()
			c.DBUser, c.DBPassword, c.DBName = "u", "p", "d"
			c.AuthIssuer, c.AuthJWKSURL, c.AuthUserTokenAudiences = "https://issuer.example/token", "https://issuer.example/jwks", []string{"spa"}
			if c.DBPoolMaxConns != 20 && tc.key == "DB_POOL_MAX_CONNS" {
				t.Errorf("DBPoolMaxConns = %d, want the default (20) on an invalid value", c.DBPoolMaxConns)
			}
			if c.DBPoolMinConns != 2 && tc.key == "DB_POOL_MIN_CONNS" {
				t.Errorf("DBPoolMinConns = %d, want the default (2) on an invalid value", c.DBPoolMinConns)
			}
			err := c.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.key) {
				t.Errorf("Validate() = %v, want an error naming %s", err, tc.key)
			}
		})
	}
}

// TestLoad_DBPoolMinConnsZeroIsValid is the regression guard for the
// CodeRabbit-caught overreach: pgxpool genuinely permits MinConns=0 (a
// deployment that doesn't want to retain any idle connections at all), so
// DB_POOL_MIN_CONNS=0 must be accepted, not treated as an invalid value
// that falls back to the default.
func TestLoad_DBPoolMinConnsZeroIsValid(t *testing.T) {
	t.Setenv("DB_POOL_MIN_CONNS", "0")
	c := Load()
	if c.DBPoolMinConns != 0 {
		t.Errorf("DBPoolMinConns = %d, want 0", c.DBPoolMinConns)
	}
	c.DBUser, c.DBPassword, c.DBName = "u", "p", "d"
	c.AuthIssuer, c.AuthJWKSURL, c.AuthUserTokenAudiences = "https://issuer.example/token", "https://issuer.example/jwks", []string{"spa"}
	if err := c.Validate(); err != nil {
		t.Errorf("Validate() with DB_POOL_MIN_CONNS=0 = %v, want nil", err)
	}
}

func TestLoad_Redis(t *testing.T) {
	t.Setenv("REDIS_URL", "")
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("USER_CACHE_TTL", "")
	c := Load()
	if c.HasRedis() {
		t.Error("HasRedis() = true with neither REDIS_URL nor REDIS_ADDR set")
	}
	if c.UserCacheTTL != 10*time.Minute {
		t.Errorf("UserCacheTTL = %v, want the 10m default", c.UserCacheTTL)
	}

	t.Setenv("REDIS_ADDR", " localhost:6379 ")
	t.Setenv("USER_CACHE_TTL", "90s")
	c = Load()
	if !c.HasRedis() || c.RedisAddr != "localhost:6379" {
		t.Errorf("HasRedis() = %v, RedisAddr = %q; want true, %q", c.HasRedis(), c.RedisAddr, "localhost:6379")
	}
	if c.UserCacheTTL != 90*time.Second {
		t.Errorf("UserCacheTTL = %v, want 90s", c.UserCacheTTL)
	}

	t.Setenv("REDIS_ADDR", "")
	t.Setenv("REDIS_URL", "rediss://:pw@cache.example.net:10000")
	if !Load().HasRedis() {
		t.Error("HasRedis() = false with REDIS_URL set")
	}
}

// dsnSearchPath extracts the search_path value DSN embedded in its "options"
// query parameter, so a test can assert on the schema alone rather than the
// whole connection string.
func dsnSearchPath(t *testing.T, dsn string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse DSN %q: %v", dsn, err)
	}
	return strings.TrimPrefix(u.Query().Get("options"), "-c search_path=")
}

// TestConfig_DSN_SchemaFallsBackToDBUserPlusPublic pins DSN's search_path
// behavior: an explicit DBSchema wins verbatim (no "public" appended — an
// operator who set one is assumed to mean it), and an empty one falls back
// to "DBUser,public" (no space — see DSN's own doc comment on why), Postgres'
// own default search_path. "public"
// must survive the fallback: entity-service's migrations create every table
// unqualified, so every deployment's real tables live there, and an explicit
// search_path replaces Postgres' own default rather than extending it — a
// fallback of DBUser alone would make every one of those tables unresolvable.
func TestConfig_DSN_SchemaFallsBackToDBUserPlusPublic(t *testing.T) {
	base := baseValidConfig()
	base.DBHost = "localhost"
	base.DBPort = "5432"

	t.Run("explicit schema wins, verbatim", func(t *testing.T) {
		c := base
		c.DBSchema = "csm"
		if got := dsnSearchPath(t, c.DSN()); got != "csm" {
			t.Errorf("search_path = %q, want %q", got, "csm")
		}
	})

	t.Run("unset schema falls back to DBUser, public", func(t *testing.T) {
		c := base
		c.DBSchema = ""
		want := c.DBUser + ",public"
		if got := dsnSearchPath(t, c.DSN()); got != want {
			t.Errorf("search_path = %q, want %q", got, want)
		}
	})

	t.Run("unset schema and unset DBUser falls back to public alone", func(t *testing.T) {
		c := base
		c.DBSchema = ""
		c.DBUser = ""
		if got := dsnSearchPath(t, c.DSN()); got != "public" {
			t.Errorf("search_path = %q, want %q", got, "public")
		}
	})
}

func TestSREEventHubTopicMovesBothOperationsPublishers(t *testing.T) {
	t.Setenv("CR_EVENT_HUB_TOPIC", "cr-events")
	t.Setenv("OUTAGE_EVENT_HUB_TOPIC", "outage-events")

	t.Setenv("SRE_EVENT_HUB_TOPIC", "")
	if c := Load(); c.CREventHubTopic != "cr-events" || c.OutageEventHubTopic != "outage-events" {
		t.Errorf("unset SRE topic changed the publishers: cr=%q outage=%q", c.CREventHubTopic, c.OutageEventHubTopic)
	}

	t.Setenv("SRE_EVENT_HUB_TOPIC", " sre-events ")
	c := Load()
	if c.CREventHubTopic != "sre-events" || c.OutageEventHubTopic != "sre-events" {
		t.Errorf("SRE topic set: cr=%q outage=%q, want both sre-events", c.CREventHubTopic, c.OutageEventHubTopic)
	}
	if c.EventHubTopic == "sre-events" {
		t.Error("the case-events topic must not move")
	}
}

func TestConfig_Validate_Timeouts(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{"zero read", func(c *Config) { c.ServerReadTimeout = 0 }, "SERVER_READ_TIMEOUT"},
		{"negative write", func(c *Config) { c.ServerWriteTimeout = -time.Second }, "SERVER_WRITE_TIMEOUT"},
		{"zero request", func(c *Config) { c.RequestTimeout = 0 }, "REQUEST_TIMEOUT"},
		{"zero upstream", func(c *Config) { c.UpstreamClientTimeout = 0 }, "UPSTREAM_CLIENT_TIMEOUT"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := baseValidConfig()
			tt.mutate(&c)
			err := c.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Validate() = %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}

// CR_STRICT_VISIBILITY_FROM is the cutover instant of the customer-visibility
// rule of change requests: unset is "no cutover" (every change request is
// legacy, today's behaviour), a value is an RFC 3339 instant WITH a zone, and
// anything else refuses to start so a typo can never silently change who sees what.
func TestConfig_CRStrictVisibilityFrom(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      *time.Time
		wantErr   bool
	}{
		{"unset", "", nil, false},
		{"blank", "   ", nil, false},
		{"UTC", "2026-11-01T00:00:00Z", ptrTime(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)), false},
		{"with an offset, normalised to UTC", "2026-11-01T05:30:00+05:30", ptrTime(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)), false},
		{"padded", " 2026-11-01T00:00:00Z ", ptrTime(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)), false},
		{"a date with no zone", "2026-11-01", nil, true},
		{"a datetime with no zone", "2026-11-01T00:00:00", nil, true},
		{"words", "tomorrow", nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CR_STRICT_VISIBILITY_FROM", tc.raw)
			c := Load()
			got, err := c.CRStrictVisibilityFrom()
			if (err != nil) != tc.wantErr {
				t.Fatalf("CRStrictVisibilityFrom() error = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr {
				if !strings.Contains(err.Error(), "CR_STRICT_VISIBILITY_FROM") {
					t.Errorf("error %q must name the variable", err)
				}
				return
			}
			if (got == nil) != (tc.want == nil) || (got != nil && !got.Equal(*tc.want)) {
				t.Fatalf("CRStrictVisibilityFrom() = %v, want %v", got, tc.want)
			}
			if got != nil && got.Location() != time.UTC {
				t.Errorf("the instant is in %v, want UTC", got.Location())
			}
		})
	}
}

func TestConfig_Validate_CRStrictVisibilityFromRefusesAnUnparsableValue(t *testing.T) {
	c := baseValidConfig()
	c.CRStrictVisibilityFromRaw = "next tuesday"
	err := c.Validate()
	if err == nil || !strings.Contains(err.Error(), "CR_STRICT_VISIBILITY_FROM") {
		t.Fatalf("Validate() = %v, want an error naming CR_STRICT_VISIBILITY_FROM", err)
	}
	c.CRStrictVisibilityFromRaw = "2026-11-01T00:00:00Z"
	if err := c.Validate(); err != nil {
		t.Fatalf("a valid cutover refused: %v", err)
	}
	c.CRStrictVisibilityFromRaw = ""
	if err := c.Validate(); err != nil {
		t.Fatalf("an unset cutover refused: %v", err)
	}
}

func ptrTime(t time.Time) *time.Time { return &t }
