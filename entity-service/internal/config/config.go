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
	"log/slog"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// DataSource identifies which backend the service reads from.
type DataSource string

const (
	// DataSourcePostgres uses the local PostgreSQL database.
	DataSourcePostgres DataSource = "postgres"
	// DataSourceServiceNow uses the Choreo ServiceNow API.
	DataSourceServiceNow DataSource = "servicenow"
	// DataSourcePostgresServiceNowDualWrite serves every read and write from
	// PostgreSQL (authoritative, same as DataSourcePostgres) and additionally
	// best-effort mirrors writes to ServiceNow afterward, so that ServiceNow
	// stays a genuine rollback target rather than going silently stale ahead
	// of the Postgres cutover. One-way (Postgres -> ServiceNow) and
	// asynchronous: ServiceNow is never read from in this mode, never
	// authoritative, and a failed mirror write is recorded (see
	// SNWritebackFailureRepository) rather than retried or surfaced to the
	// caller. Piloted on the account entity only — see routes.go.
	DataSourcePostgresServiceNowDualWrite DataSource = "postgres-servicenow-dual-write"
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
	// HealthPort is the listen port for the separate, minimal health
	// server (internal/server.NewHealthServer). It is deliberately NOT
	// ServerPort: that mux carries every business route and is exposed at
	// organization visibility, while the health server is exposed
	// publicly so external alerting can reach it without credentials.
	// Separate listeners mean the public deployment surface is only ever
	// the handful of routes registered on the health mux — no basePath or
	// gateway rule stands between a misconfiguration and the whole API.
	HealthPort string
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
	// ConsumptionOperationBaseURL is the base URL of the Choreo subscription
	// operation (operations/choreo-subscription-on-project-create), with the
	// client credentials it is reached with.
	//
	// There is deliberately no default. The operation creates Choreo
	// applications and issues signed licences for real customers, so a
	// deployment that forgets to configure it must fail to register the
	// licence route rather than quietly provision against whatever
	// environment a baked-in default names.
	ConsumptionOperationBaseURL      string
	ConsumptionOperationTokenURL     string
	ConsumptionOperationClientID     string
	ConsumptionOperationClientSecret string
	ConsumptionOperationScopes       string
	// ConsumptionDualWriteEnabled controls whether provisioning state and
	// artifacts are mirrored into Postgres alongside ServiceNow. Defaults to true.
	ConsumptionDualWriteEnabled bool
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
	// QueryHourChoreoBaseURL is the Choreo Sales Operations base URL for the
	// subscription-closure push — the port of ServiceNow's REST message
	// "Choreo API Sales Operations" / "Update Subscription Closure State".
	// Empty disables pushing: the query-hour recompute still runs and still
	// records its result, it just does not tell Choreo. Safe by default, the
	// same way EventPublishingEnabled is.
	QueryHourChoreoBaseURL string
	// QueryHourChoreoAPIKey is sent as the `api-key` header when set.
	QueryHourChoreoAPIKey string
	// QueryHourNotificationsEnabled gates the 75/90/100 threshold email
	// independently of Event Hub being configured at all.
	//
	// It exists because the Choreo kill switch alone was not enough: with
	// EVENT_PUBLISHING_ENABLED already on for other events — which it is in
	// any environment publishing case events — a parallel run with
	// QUERY_HOUR_CHOREO_BASE_URL unset would still have emailed every
	// threshold notice, duplicating the ServiceNow flow that is still live.
	// That is exactly the double-fire the cutover is meant to avoid.
	//
	// Defaults to false. Turn it on at cutover, in the same change that
	// deactivates the ServiceNow flow.
	QueryHourNotificationsEnabled bool

	// EventPublishingEnabled is a separate kill switch on top of
	// EventHubBroker being set — it defaults to false (safe-by-default: an
	// environment can have Event Hub fully configured and still not publish
	// a single event until this is explicitly turned on). routes.go only
	// constructs EventPublisherService when both this is true AND
	// EventHubBroker is set.
	EventPublishingEnabled bool
	// CSMMigrationSalesforceMembershipIngestEnabled turns on the
	// Project_Contact__c / Contact branch of POST /salesforce/events (the
	// customer onboarding database write), from
	// CSM_MIGRATION_SALESFORCE_MEMBERSHIP_INGEST_ENABLED=true. Defaults to
	// false: those envelopes are then acknowledged and ignored, as before
	// the branch existed. The Account branch is unaffected by this flag.
	CSMMigrationSalesforceMembershipIngestEnabled bool
	// CSMMigrationMembershipRegistrationEnabled turns on POST /users/me/memberships/register,
	// which marks the signed-in user's still-INVITED memberships as
	// REGISTERED in Salesforce (see membership_registration_service.go). Defaults to
	// false, and while it is false routes.go does not register the route at
	// all — it 404s, and nothing on this path can write to Salesforce.
	CSMMigrationMembershipRegistrationEnabled bool
	// CSMMigrationPortalWritesEnabled turns on the portal-driven membership
	// write endpoints (POST/PATCH/DELETE /projects/{id}/contacts[/{email}]
	// and the resend-invitation call). Both portals invite, re-role and
	// deactivate customer users through them, and each one writes Postgres
	// and Salesforce together.
	//
	// OFF BY DEFAULT (the value must be exactly "true"), and with it off the
	// routes are not registered at all rather than answering 403: until the
	// Sales Entity create endpoints this depends on are deployed, a portal
	// that called them would write the database and leave Salesforce behind.
	CSMMigrationPortalWritesEnabled bool
	// GithubIntegrationEnabled gates the GitHub change-request sync: the
	// webhook endpoint and the client that answers it.
	//
	// OFF BY DEFAULT. The endpoint is reachable without a bearer token -- the
	// HMAC signature is its authentication -- so it must not appear merely
	// because a database happens to be configured.
	GithubIntegrationEnabled bool
	// GithubBaseURL is the API root: api.github.com, or an Enterprise host.
	GithubBaseURL string
	// GithubToken authenticates our calls out to GitHub.
	GithubToken string
	// GithubWebhookSecret is the HMAC key GitHub signs deliveries with. This
	// IS the authentication on the webhook endpoint, so an empty value makes
	// VerifySignature refuse everything rather than accept everything.
	GithubWebhookSecret string
	// GithubIntegrationLogin is our own GitHub account. Events it sent are our
	// own writes coming back, and are dropped by identity rather than by
	// pattern-matching the comment body.
	GithubIntegrationLogin string
	// GithubOutboundInterval is how often to drain the outbound queue when the
	// last pass came back short. A backlog drains at full speed regardless.
	GithubOutboundInterval time.Duration

	// CSMPortalBaseURL builds the link back to a change request in comments
	// posted to GitHub. Empty omits the link rather than rendering a broken one.
	CSMPortalBaseURL string
	// GitHub label vocabulary overrides. Empty keeps .github/labels.yml's value.
	GithubLabelTypeIncident       string
	GithubLabelTypeServiceRequest string
	GithubLabelsClass             string
	GithubLabelStatusAssigned     string

	// CRNoticesEnabled turns on the change-request notice drainer: the poller
	// that reads event_outbox and asks csm-notification-service to send the
	// approval and plan-start-date mails.
	//
	// OFF BY DEFAULT, AND THAT IS THE POINT. ServiceNow still sends these
	// notices today. Turning this on is a paired change with disabling its
	// ServiceNow counterparts -- two senders for one event means every
	// approver gets the mail twice -- so it must never come on merely because
	// a database happens to be configured.
	CRNoticesEnabled bool
	// CREventHubTopic is the topic the change-request notices are published
	// to. SEPARATE FROM EventHubTopic ON PURPOSE. Every consumer group reads
	// its whole topic, so putting these on case-events would make the case
	// consumer read and discard every change-request record, and vice versa.
	// A distinct topic is what actually isolates the two volumes; a distinct
	// consumer group alone would only isolate the processing.
	CREventHubTopic string

	// ProjectEventHubTopic carries the onboarding events
	// (project_contact.invited) rather than the shared EventHubTopic, so a
	// backlog of case events can never delay an invitation and the
	// onboarding dead-letter queue can be watched on its own.
	// csm-notification-service consumes it with its own consumer group.
	ProjectEventHubTopic string
	// CRNoticePollInterval is how often to poll event_outbox when the last
	// pass came back short. A backlog drains at full speed regardless, so this
	// governs only the idle case: notice latency against query volume.
	CRNoticePollInterval time.Duration
	// CustomerRoles is a comma-separated list of ServiceNow role names
	// (organisation-specific vocabulary, the same reasoning
	// apps/csm-portal/backend's own CSM_TEAM_REGISTRY uses for not shipping
	// a committed default) whose presence on a case comment's
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
	// CSEngineerRole is the ServiceNow role name (e.g. an org-specific
	// "sn_*" role) whose presence on a case comment's resolved author marks
	// that comment as a qualifying CS-engineer response — see
	// sn_case_service.go's applyResponseSLAOnComment, which the CSM-native
	// SLA engine (internal/service/sla_engine_service.go) uses to complete
	// a case's "response" SLA clock. Deliberately no committed default:
	// this is organisation-specific vocabulary, same reasoning
	// CustomerRoles' own doc comment gives. Left unset, that function
	// simply can't confirm engineer-authorship and skips (logged) — not
	// fatal, not required by Validate.
	CSEngineerRole string
	// SLARecomputeInterval is how often SLAEngineRecomputeWorker
	// recomputes every CSM-native "sla" row's elapsed percentage/breach
	// status (internal/service/sla_engine_recompute_worker.go). Same
	// envDuration convention as CRNoticePollInterval/GithubOutboundInterval
	// above.
	SLARecomputeInterval time.Duration
	// Auth* configure token validation (internal/auth), always on -- there is
	// no config flag to disable it. AuthIssuer/AuthJWKSURL/
	// AuthUserTokenAudiences are required (Validate rejects startup without
	// them, and NewRouter panics if the JWKS can't be loaded), so a caller's
	// identity is always verified.
	//
	// AuthIssuer/AuthJWKSURL locate the Asgardeo issuer's signing keys, used to
	// validate both tokens a request can carry: the end user's ID token in
	// x-user-id-token, and the calling application's client-credentials access
	// token in Authorization: Bearer. AuthUserTokenAudiences are the client ids
	// (Asgardeo SPA/application ids) an ID token's aud must contain to be
	// accepted as a user token.
	AuthIssuer             string
	AuthJWKSURL            string
	AuthUserTokenAudiences []string
	AuthClockSkew          time.Duration
	// AuthInternalClientIDsRaw is the AUTH_INTERNAL_CLIENT_IDS value, a
	// comma-separated list of Asgardeo application client ids;
	// AuthInternalClientIDs is its parsed set. A request whose
	// Authorization: Bearer client-credentials token names one of these ids
	// is unconditionally treated as an internal caller with unrestricted
	// access to every project and case, regardless of any x-user-id-token it
	// also carries -- a forwarded user token from an internal caller is used
	// only for attribution (created_by/updated_by), never for scoping,
	// because every caller this deployment configures here is itself an
	// already-trusted internal service.
	//
	// A client id NOT in this set is resolved purely from its
	// x-user-id-token: an INTERNAL user_type still sees everything, an
	// EXTERNAL (customer) user sees only their REGISTERED project_contact
	// projects, and no user token at all is refused. Which real client ids
	// go in this list is a deployment decision, not something this file
	// prescribes.
	AuthInternalClientIDsRaw string
	AuthInternalClientIDs    map[string]bool
	// SalesEntity* is the Choreo connection to REST sales/sales-entity-service
	// (POST /customer-search), not GraphQL sales/entity-graphql-service and not
	// Salesforce. The four connection fields are all-or-nothing like Event Hub.
	// Scopes are optional (same as SERVICENOW_INTEGRATION_SERVICE_SCOPES).
	SalesEntityBaseURL      string
	SalesEntityTokenURL     string
	SalesEntityClientID     string
	SalesEntityClientSecret string
	SalesEntityScopes       string
	// M2MTrustedActorEmails is the allowlist of service-account emails an
	// M2M caller (no x-user-id-token, e.g. UMT via csm-integration-service)
	// may claim as the acting user via AddCaseTagRequest.ActorEmail. An
	// unset/empty var means no email is trusted and every such request is
	// rejected -- this is deliberately not a default-open list, since it
	// exists specifically to stop an M2M caller from spoofing an arbitrary
	// actor. Compared case-insensitively in the handler.
	M2MTrustedActorEmails []string
}

// Load reads configuration from environment variables and returns a populated
// Config. Missing variables fall back to sensible defaults; callers should
// validate required fields (e.g. DBUser, DBPassword, DBName) before use.
func Load() *Config {
	cfg := &Config{
		DBHost:                                   getEnvOrDefault("DB_HOST", "localhost"),
		DBPort:                                   getEnvOrDefault("DB_PORT", "5432"),
		DBUser:                                   os.Getenv("DB_USER"),
		DBPassword:                               os.Getenv("DB_PASSWORD"),
		DBName:                                   os.Getenv("DB_NAME"),
		DBSSLMode:                                os.Getenv("DB_SSLMODE"),
		ServerPort:                               getEnvOrDefault("SERVER_PORT", "8080"),
		HealthPort:                               getEnvOrDefault("HEALTH_PORT", "8081"),
		DataSource:                               DataSource(getEnvOrDefault("DATA_SOURCE", string(DataSourcePostgres))),
		ServiceNowIntegrationServiceBaseURL:      os.Getenv("SERVICENOW_INTEGRATION_SERVICE_BASE_URL"),
		ServiceNowIntegrationServiceTokenURL:     os.Getenv("SERVICENOW_INTEGRATION_SERVICE_TOKEN_URL"),
		ServiceNowIntegrationServiceClientID:     os.Getenv("SERVICENOW_INTEGRATION_SERVICE_CLIENT_ID"),
		ServiceNowIntegrationServiceClientSecret: os.Getenv("SERVICENOW_INTEGRATION_SERVICE_CLIENT_SECRET"),
		ServiceNowIntegrationServiceScopes:       os.Getenv("SERVICENOW_INTEGRATION_SERVICE_SCOPES"),
		ConsumptionOperationBaseURL:              os.Getenv("PRODUCT_CONSUMPTION_OPERATION_URL"),
		ConsumptionOperationTokenURL:             os.Getenv("PRODUCT_CONSUMPTION_OPERATION_TOKEN_URL"),
		ConsumptionOperationClientID:             os.Getenv("PRODUCT_CONSUMPTION_OPERATION_CLIENT_ID"),
		ConsumptionOperationClientSecret:         os.Getenv("PRODUCT_CONSUMPTION_OPERATION_CLIENT_SECRET"),
		ConsumptionOperationScopes:               os.Getenv("PRODUCT_CONSUMPTION_OPERATION_SCOPES"),
		ConsumptionDualWriteEnabled:              getBoolOrDefault("CONSUMPTION_DUAL_WRITE_ENABLED", true),
		EventHubBroker:                           os.Getenv("EVENT_HUB_BROKER"),
		EventHubConnectionString:                 os.Getenv("EVENT_HUB_CONNECTION_STRING"),
		EventHubTopic:                            os.Getenv("EVENT_HUB_TOPIC"),
		QueryHourChoreoBaseURL:                   os.Getenv("QUERY_HOUR_CHOREO_BASE_URL"),
		QueryHourChoreoAPIKey:                    os.Getenv("QUERY_HOUR_CHOREO_API_KEY"),
		QueryHourNotificationsEnabled:            os.Getenv("QUERY_HOUR_NOTIFICATIONS_ENABLED") == "true",
		EventPublishingEnabled:                   os.Getenv("EVENT_PUBLISHING_ENABLED") == "true",
		GithubIntegrationEnabled:                 os.Getenv("GITHUB_INTEGRATION_ENABLED") == "true",
		GithubBaseURL:                            getEnvOrDefault("GITHUB_API_BASE_URL", "https://api.github.com"),
		GithubToken:                              os.Getenv("GITHUB_TOKEN"),
		GithubWebhookSecret:                      os.Getenv("GITHUB_WEBHOOK_SECRET"),
		GithubIntegrationLogin:                   os.Getenv("GITHUB_INTEGRATION_LOGIN"),
		GithubOutboundInterval:                   envDuration("GITHUB_OUTBOUND_INTERVAL", 15*time.Second),
		CSMPortalBaseURL:                         os.Getenv("CSM_PORTAL_BASE_URL"),
		GithubLabelTypeIncident:                  os.Getenv("GITHUB_LABEL_TYPE_INCIDENT"),
		GithubLabelTypeServiceRequest:            os.Getenv("GITHUB_LABEL_TYPE_SERVICE_REQUEST"),
		GithubLabelsClass:                        os.Getenv("GITHUB_LABELS_CLASS"),
		GithubLabelStatusAssigned:                os.Getenv("GITHUB_LABEL_STATUS_ASSIGNED"),
		CRNoticesEnabled:                         os.Getenv("CR_NOTICES_ENABLED") == "true",
		CSMMigrationSalesforceMembershipIngestEnabled: os.Getenv("CSM_MIGRATION_SALESFORCE_MEMBERSHIP_INGEST_ENABLED") == "true",
		CSMMigrationPortalWritesEnabled:               os.Getenv("CSM_MIGRATION_PORTAL_WRITES_ENABLED") == "true",
		CREventHubTopic:                               getEnvOrDefault("CR_EVENT_HUB_TOPIC", "cr-events"),
		ProjectEventHubTopic:                          getEnvOrDefault("PROJECT_EVENT_HUB_TOPIC", "project-events"),
		CRNoticePollInterval:                          envDuration("CR_NOTICE_POLL_INTERVAL", 5*time.Second),
		AuthIssuer:                                    os.Getenv("AUTH_ISSUER"),
		AuthJWKSURL:                                   os.Getenv("AUTH_JWKS_URL"),
		AuthUserTokenAudiences:                        splitComma(os.Getenv("AUTH_USER_TOKEN_AUDIENCES")),
		AuthClockSkew:                                 envDuration("AUTH_CLOCK_SKEW", 30*time.Second),
		AuthInternalClientIDsRaw:                      os.Getenv("AUTH_INTERNAL_CLIENT_IDS"),
		CustomerRoles:                                 splitComma(os.Getenv("CUSTOMER_ROLES")),
		CSEngineerRole:                                os.Getenv("CS_ENGINEER_ROLE"),
		SLARecomputeInterval:                          envDuration("SLA_RECOMPUTE_INTERVAL", 45*time.Second),
		SalesEntityBaseURL:                            os.Getenv("SALES_ENTITY_BASE_URL"),
		SalesEntityTokenURL:                           os.Getenv("SALES_ENTITY_TOKEN_URL"),
		SalesEntityClientID:                           os.Getenv("SALES_ENTITY_CLIENT_ID"),
		SalesEntityClientSecret:                       os.Getenv("SALES_ENTITY_CLIENT_SECRET"),
		SalesEntityScopes:                             os.Getenv("SALES_ENTITY_SCOPES"),
		CSMMigrationMembershipRegistrationEnabled:     os.Getenv("CSM_MIGRATION_MEMBERSHIP_REGISTRATION_ENABLED") == "true",
		M2MTrustedActorEmails:                         splitComma(os.Getenv("M2M_TRUSTED_ACTOR_EMAILS")),
	}
	cfg.AuthInternalClientIDs = ParseInternalClientIDs(cfg.AuthInternalClientIDsRaw)
	return cfg
}

// ParseInternalClientIDs parses AUTH_INTERNAL_CLIENT_IDS ("clientId,clientId")
// into a set for O(1) membership checks. Unlike most of this file's other
// comma-separated values, this one has no per-entry validation to fail: any
// non-empty, trimmed entry is a valid client id.
func ParseInternalClientIDs(raw string) map[string]bool {
	out := make(map[string]bool)
	for _, id := range splitComma(raw) {
		out[id] = true
	}
	return out
}

func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// getBoolOrDefault parses a boolean env var, falling back to defaultVal when it
// is unset or unparseable.
//
// The other boolean flags here compare against "true" directly, which is safe
// for a flag that defaults to off: a typo leaves it off, as it already was.
// This one exists for flags that default to ON — there, "TRUE" or "1" silently
// turning the flag off is a real failure, so accept everything ParseBool does.
func getBoolOrDefault(key string, defaultVal bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		slog.Warn("ignoring unparseable boolean configuration value",
			"key", key, "value", v, "using", defaultVal)
		return defaultVal
	}
	return parsed
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
// error if SERVER_PORT/HEALTH_PORT are unusable or resolve to the same
// port, if DATA_SOURCE is an unrecognised value, if the DB variables are
// missing when DATA_SOURCE=postgres (see db.NewPoolIfNeeded) or only
// partially set in either mode, if
// SERVICENOW_INTEGRATION_SERVICE_BASE_URL is missing when
// DATA_SOURCE=servicenow, if EVENT_HUB_BROKER/EVENT_HUB_CONNECTION_STRING/
// EVENT_HUB_TOPIC are only partially set, or if the SALES_ENTITY_* vars are
// only partially set.
func (c *Config) Validate() error {
	// The health server is a separate listener precisely so that only its
	// own routes are reachable at public visibility (see HealthPort). Two
	// listeners cannot share a port: the second ListenAndServe would fail
	// with "address already in use" after the first has already started
	// serving, leaving the process up but one of the two ports dead. Reject
	// that at startup, where it is unambiguous.
	//
	// Resolved to numbers first rather than compared as strings: "8080" and
	// "08080" are the same TCP port but not the same string, so a string
	// comparison would wave that pair through into exactly the half-dead
	// startup described above. Resolving also rejects a port that could
	// never be bound at all ("http-alt-typo", "99999") here, with the
	// offending variable named, instead of at ListenAndServe time inside a
	// goroutine.
	serverPortNum, err := net.LookupPort("tcp", c.ServerPort)
	if err != nil {
		return fmt.Errorf("invalid SERVER_PORT %q: %w", c.ServerPort, err)
	}
	healthPortNum, err := net.LookupPort("tcp", c.HealthPort)
	if err != nil {
		return fmt.Errorf("invalid HEALTH_PORT %q: %w", c.HealthPort, err)
	}
	if serverPortNum == healthPortNum {
		return fmt.Errorf("HEALTH_PORT (%s) must differ from SERVER_PORT (%s)", c.HealthPort, c.ServerPort)
	}

	switch c.DataSource {
	case DataSourcePostgres, DataSourceServiceNow, DataSourcePostgresServiceNowDualWrite:
		// valid
	default:
		return fmt.Errorf("invalid DATA_SOURCE %q: must be %q, %q, or %q", c.DataSource, DataSourcePostgres, DataSourceServiceNow, DataSourcePostgresServiceNowDualWrite)
	}
	// Postgres credentials are required for DATA_SOURCE=postgres and
	// DATA_SOURCE=postgres-servicenow-dual-write — both serve every entity read
	// and write from the pool (the fallback mode's ServiceNow leg is a
	// best-effort mirror on top, never a read source). servicenow mode skips
	// the pool (db.NewPoolIfNeeded) so a local customer-portal can start
	// without a reachable database. Side tables that have no ServiceNow
	// equivalent are registered only when a pool is available — see
	// routes.go.
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
	dbRequired := c.DataSource == DataSourcePostgres || c.DataSource == DataSourcePostgresServiceNowDualWrite

	if dbRequired && !dbComplete {
		if c.DBUser == "" {
			return fmt.Errorf("DB_USER is required when DATA_SOURCE=%s", c.DataSource)
		}
		if c.DBPassword == "" {
			return fmt.Errorf("DB_PASSWORD is required when DATA_SOURCE=%s", c.DataSource)
		}
		return fmt.Errorf("DB_NAME is required when DATA_SOURCE=%s", c.DataSource)
	}

	// A partial set is always a misconfiguration, in either mode — the same
	// all-or-nothing reasoning as the Event Hub group below. Silently running
	// without a database because one of the three was left unset would
	// disable event_publish_failures and sla_clocks without anyone noticing.
	if dbSet && !dbComplete {
		return fmt.Errorf("DB_USER, DB_PASSWORD, and DB_NAME must be set together or not at all")
	}
	// ServiceNow integration service credentials are required for
	// DATA_SOURCE=servicenow (reads go there) and also for
	// DATA_SOURCE=postgres-servicenow-dual-write (the best-effort mirror write
	// goes there, via the same client — see SNWritebackDispatcher).
	snRequired := c.DataSource == DataSourceServiceNow || c.DataSource == DataSourcePostgresServiceNowDualWrite
	if snRequired {
		if c.ServiceNowIntegrationServiceBaseURL == "" {
			return fmt.Errorf("SERVICENOW_INTEGRATION_SERVICE_BASE_URL is required when DATA_SOURCE=%s", c.DataSource)
		}
		if c.ServiceNowIntegrationServiceTokenURL == "" {
			return fmt.Errorf("SERVICENOW_INTEGRATION_SERVICE_TOKEN_URL is required when DATA_SOURCE=%s", c.DataSource)
		}
		if c.ServiceNowIntegrationServiceClientID == "" {
			return fmt.Errorf("SERVICENOW_INTEGRATION_SERVICE_CLIENT_ID is required when DATA_SOURCE=%s", c.DataSource)
		}
		if c.ServiceNowIntegrationServiceClientSecret == "" {
			return fmt.Errorf("SERVICENOW_INTEGRATION_SERVICE_CLIENT_SECRET is required when DATA_SOURCE=%s", c.DataSource)
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
	// Token validation is always on and unconditionally needs to know whose
	// keys to trust and which audiences make an ID token a user token, or it
	// would either accept everything or reject everything. Reject that at
	// startup rather than at the first request.
	if c.AuthIssuer == "" || c.AuthJWKSURL == "" {
		return fmt.Errorf("AUTH_ISSUER and AUTH_JWKS_URL are required")
	}
	if len(c.AuthUserTokenAudiences) == 0 {
		return fmt.Errorf("AUTH_USER_TOKEN_AUDIENCES is required")
	}
	salesEntitySet := c.SalesEntityBaseURL != "" || c.SalesEntityTokenURL != "" || c.SalesEntityClientID != "" || c.SalesEntityClientSecret != "" || c.SalesEntityScopes != ""
	if salesEntitySet && !c.SalesEntityConfigured() {
		return fmt.Errorf("SALES_ENTITY_BASE_URL, SALES_ENTITY_TOKEN_URL, SALES_ENTITY_CLIENT_ID, and SALES_ENTITY_CLIENT_SECRET must be set together or not at all")
	}
	return nil
}

// HasPortalMembershipWrites reports whether the portal-driven membership
// write endpoints may be registered: the flag is on, the data source is
// Postgres (the write is a Postgres transaction — there is no ServiceNow
// equivalent), and the REST sales/sales-entity-service connection is
// complete, since half of every one of those writes goes to Salesforce.
// routes.go ANDs this with db != nil, the same way every other
// Postgres-only feature set is gated.
func (c *Config) HasPortalMembershipWrites() bool {
	return c.CSMMigrationPortalWritesEnabled &&
		c.DataSource == DataSourcePostgres &&
		c.SalesEntityConfigured()
}

// SalesEntityConfigured reports whether every REST sales/sales-entity-service env var is set.
func (c *Config) SalesEntityConfigured() bool {
	return c.SalesEntityBaseURL != "" &&
		c.SalesEntityTokenURL != "" &&
		c.SalesEntityClientID != "" &&
		c.SalesEntityClientSecret != ""
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

// HasGithubIntegration reports whether the GitHub sync is both switched on and
// configured well enough to run. The webhook secret is required rather than
// optional: without it the endpoint could not authenticate a caller, and an
// endpoint that mutates change requests must never be reachable unverified.
func (c *Config) HasGithubIntegration() bool {
	return c.GithubIntegrationEnabled &&
		c.GithubWebhookSecret != "" &&
		c.GithubToken != "" &&
		c.GithubIntegrationLogin != ""
}

// envDuration reads a Go duration string (e.g. "5s", "500ms"), falling back to
// def when unset or unparseable -- a typo should cost the override, not stop
// the service starting.
func envDuration(key string, def time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return def
	}
	return d
}
