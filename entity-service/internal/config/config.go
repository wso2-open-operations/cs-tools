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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/validate"
)

// Defaults for the timeout settings. Create-case carries inline base64
// attachments (up to 15 MiB), so the deadlines must be long enough for it to
// finish. Operators may set any positive values.
const (
	DefaultServerReadTimeout     = 60 * time.Second
	DefaultServerWriteTimeout    = 60 * time.Second
	DefaultRequestTimeout        = 60 * time.Second
	DefaultUpstreamClientTimeout = 60 * time.Second
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
	DBHost string
	DBPort string
	// AvailabilityTimezone is the zone the availability sweep resolves its
	// period boundaries in. Empty uses service.DefaultAvailabilityTimezone
	// (Asia/Colombo), which is what ServiceNow's running engine actually
	// uses — it is the system zone, NOT the commitment's recorded one.
	AvailabilityTimezone string

	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string
	// DBSchema pins the connection's search_path (see DSN) so a role whose
	// native search_path would otherwise resolve to a different same-named
	// schema, or to "public", lands in the intended one instead — same
	// purpose as operations/csm-sync-service's own DB_SCHEMA. Left empty,
	// DSN falls back to "DBUser,public" — Postgres' own default
	// search_path, made explicit here rather than left implicit,
	// since this is the one of the two services that also sets a
	// connection-level option (jit=off) through the same mechanism. The
	// "public" half of that fallback matters: entity-service's migrations
	// create every table unqualified, so every deployment's real tables
	// live there today.
	DBSchema string
	// DBPoolMaxConns/DBPoolMinConns/DBPoolMaxConnLifetime/DBPoolMaxConnIdleTime
	// tune internal/db.NewPool's pgxpool (DB_POOL_MAX_CONNS/DB_POOL_MIN_CONNS/
	// DB_POOL_MAX_CONN_LIFETIME/DB_POOL_MAX_CONN_IDLE_TIME). Defaults (20/2/
	// 30m/5m) are the values this file previously hardcoded in
	// internal/db/postgres.go — an unset deployment behaves exactly as
	// before these existed. DBPoolMaxConns falls back to its default on an
	// unset, non-numeric, or non-positive value (a pool that may open no
	// connections at all can never serve a single query). DBPoolMinConns
	// falls back the same way EXCEPT zero is accepted — pgxpool genuinely
	// permits a minimum of 0 (a deployment that doesn't want to retain any
	// idle connections). DBPoolMaxConnLifetime/DBPoolMaxConnIdleTime fall
	// back to theirs the same way every other duration here does
	// (getDurationOrDefault), via loadErr.
	DBPoolMaxConns        int32
	DBPoolMinConns        int32
	DBPoolMaxConnLifetime time.Duration
	DBPoolMaxConnIdleTime time.Duration
	// DBReadPoolEnabled (DB_READ_POOL_ENABLED, on only when exactly "true")
	// adds a second pool for requests marked read-only (db.WithReadOnly); see
	// internal/db.Router. Off by default: the single write pool then serves
	// everything, exactly as before. The DBRead* connection fields each default
	// to the corresponding DB* value, so an enabled read pool points at the
	// same primary until DB_READ_HOST is set to a replica. The DBReadPool*
	// sizing fields follow the DBPool* rules (defaults 10/2/30m/5m; the read
	// pool is smaller because it only carries opted-in read routes).
	DBReadPoolEnabled         bool
	DBReadHost                string
	DBReadPort                string
	DBReadUser                string
	DBReadPassword            string
	DBReadName                string
	DBReadSSLMode             string
	DBReadPoolMaxConns        int32
	DBReadPoolMinConns        int32
	DBReadPoolMaxConnLifetime time.Duration
	DBReadPoolMaxConnIdleTime time.Duration
	ServerPort                string
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
	// CSMMigrationSalesforceMembershipIngestEnabled turns on the
	// Project_Contact__c / Contact branch of POST /salesforce/events (the
	// customer onboarding database write), from
	// CSM_MIGRATION_SALESFORCE_MEMBERSHIP_INGEST_ENABLED=true. Defaults to
	// false: those envelopes are then acknowledged and ignored, as before
	// the branch existed. The Account branch is unaffected by this flag.
	CSMMigrationSalesforceMembershipIngestEnabled bool
	// CSMMigrationSalesforceAccountIngestEnabled turns on the Account branch
	// of POST /salesforce/events, from
	// CSM_MIGRATION_SALESFORCE_ACCOUNT_INGEST_ENABLED=true. Defaults to false:
	// Account envelopes are then acknowledged and ignored, because the
	// ServiceNow sync still owns the account table and both writing it would
	// fight over the same rows.
	CSMMigrationSalesforceAccountIngestEnabled bool
	// CSMMigrationSalesforceOpportunityIngestEnabled turns on the Opportunity
	// branch of POST /salesforce/events (sf_opportunity plus its
	// sf_opportunity_product line items), from
	// CSM_MIGRATION_SALESFORCE_OPPORTUNITY_INGEST_ENABLED=true. Defaults to
	// false: Opportunity envelopes are then acknowledged and ignored, because
	// csm-sync-service still copies these tables from ServiceNow and the two
	// writers would create duplicate rows (different row ids, non-unique sf_id).
	CSMMigrationSalesforceOpportunityIngestEnabled bool
	// CSMMigrationSalesforceProjectIngestEnabled turns on the Project__c
	// branch of POST /salesforce/events, from
	// CSM_MIGRATION_SALESFORCE_PROJECT_INGEST_ENABLED=true. Defaults to false:
	// Project__c envelopes are acknowledged and ignored. On, it updates the
	// ten Salesforce-owned project columns of rows CSM already has.
	CSMMigrationSalesforceProjectIngestEnabled bool
	// CSMMigrationSalesforceProjectInsertEnabled lets the Project ingest (and
	// EnsureProject, for memberships and linked opportunities) insert
	// projects CSM does not have, from
	// CSM_MIGRATION_SALESFORCE_PROJECT_INSERT_ENABLED=true. Defaults to false,
	// the update-only mode: while csm-sync-service still inserts project rows
	// from ServiceNow, an ingest-created row would make its insert fail on
	// project.key forever. Turn on at cutover, when csm-sync-service stops.
	CSMMigrationSalesforceProjectInsertEnabled bool
	// CSMMigrationSalesforcePartnerIngestEnabled turns on the partner-link
	// refresh (account_relationship "Is Partner Of" / "Is Customer Of") that
	// runs after Account events, after partner-contact membership events and
	// from POST /salesforce/accounts/{sfId}/refresh-partners, from
	// CSM_MIGRATION_SALESFORCE_PARTNER_INGEST_ENABLED=true. Defaults to false:
	// nothing refreshes partners and the route is not registered, because
	// csm-sync-service still copies account_relationship from ServiceNow.
	CSMMigrationSalesforcePartnerIngestEnabled bool
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
	// CSMMigrationCustomerEngagementIngestEnabled registers POST /customer-engagements/allocation-events
	// (CSM_MIGRATION_CUSTOMER_ENGAGEMENT_INGEST_ENABLED); off, the route is not registered.
	CSMMigrationCustomerEngagementIngestEnabled bool
	// CustomerEngagementFirefightingTypeID is the Firefighting type's ServiceNow sys_id
	// (CUSTOMER_ENGAGEMENT_FIREFIGHTING_TYPE_ID); unset skips creating firefighting engagements.
	CustomerEngagementFirefightingTypeID string
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
	// GithubIntegrationLogin is our own GitHub account. Events it sent are our
	// own writes coming back, and are dropped by identity rather than by
	// pattern-matching the comment body.
	GithubIntegrationLogin string
	// GithubOutboundInterval is how often to drain the outbound queue when the
	// last pass came back short. A backlog drains at full speed regardless.
	GithubOutboundInterval time.Duration

	// SpecialistHandoffConfig is SPECIALIST_HANDOFF_CONFIG, raw: the JSON
	// that routes "Escalate to specialist team" handoffs (products, their
	// services, Special Ops teams and GitHub repository). Parsed and
	// validated by service.ParseSpecialistHandoffConfig at startup; empty
	// hands nothing off.
	SpecialistHandoffConfig string
	// SpecialistHandoffGithubTokens is SPECIALIST_HANDOFF_GITHUB_TOKENS, a
	// secret: one line of JSON mapping a credential name to the GitHub token
	// that files a specialist handoff's internal issue (ServiceNow's
	// InternalGitHubIssues REST message), {"wso2-enterprise":"github_pat_..."}.
	// A product's github.credential picks one, defaulting to its owner.
	// A credential the map does not name uses GITHUB_TOKEN; with neither, the
	// handoff still goes through and reports that no issue was filed.
	// Independent of the change-request sync.
	SpecialistHandoffGithubTokens string

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

	// OutageEventHubTopic is where the outage notice drainer publishes the two
	// outage emails (outage.notification_due, outage.communication_due) for
	// csm-notification-service to send. Its own topic for the same reason as
	// CREventHubTopic. The drainer also needs event publishing
	// (EVENT_HUB_BROKER + EVENT_PUBLISHING_ENABLED) and a database; the
	// recipient lists below are what actually switch each email on.
	OutageEventHubTopic string
	// SREEventHubTopic, when set, is the ONE topic both the change-request
	// notices and the outage emails publish to (sre-events), overriding
	// CREventHubTopic and OutageEventHubTopic. csm-notification-service routes
	// them by event type, as it already does on every topic. Empty keeps the
	// two separate topics exactly as before.
	SREEventHubTopic string
	// SRAlertSRETeamIDs are the SRE teams (group ids) whose new service
	// requests are automated the way ServiceNow's "SR New Request -
	// Acknowledge & Chat Alert" flow does it: the SR is assigned to its
	// account's SRE team and gets the automatic acknowledgement comment. The
	// flow's own trigger is limited to one team (MS/PC SRE Group,
	// 6c3db375-1b1c-b2d0-a002-c9d3604bcb0c), hence a list rather than a switch.
	// Empty -- the default -- automates no team. sr.* events are published to
	// SREEventHubTopic regardless; this list only gates the two writes.
	//
	// Leave a team off while ServiceNow's flow still runs for it, or its SRs
	// are assigned, commented on and announced twice.
	SRAlertSRETeamIDs []string
	// OutageNoticePollInterval is the drainer's FALLBACK poll (default 60s).
	// The emails normally go out about a second after an outage changes: the
	// drainer LISTENs for migration 0186's NOTIFY. This interval only catches
	// a change it missed while not listening; if it cannot listen at all it
	// polls every 10s instead.
	OutageNoticePollInterval time.Duration
	// OutageNotificationRecipients is the audience of the internal-stakeholder
	// notification, OutageCommunicationRecipients that of the SRE outage
	// communication (ServiceNow resolves the group "SRE Team"). A flow with no
	// recipients is not swept at all, so its decisions are not used up.
	OutageNotificationRecipients  []string
	OutageCommunicationRecipients []string
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
	// CSEngineerRole is the role name (e.g. an org-specific "sn_*" role)
	// whose presence on a case comment's resolved author marks that comment
	// as a qualifying CS-engineer/support-engineer response — see
	// sn_case_service.go's applyResponseSLAOnComment and
	// case_service.go's completeResponseSLAOnComment, both of which the
	// CSM-native SLA engine (internal/service/sla_engine_service.go) uses
	// to complete a case's "response" SLA clock. Deliberately no committed
	// default: this is organisation-specific vocabulary, same reasoning
	// CustomerRoles' own doc comment gives. Left unset, those functions
	// simply can't confirm engineer-authorship and skip (logged) — not
	// fatal, not required by Validate. Shared by both `snCaseService` (checked
	// via `SNUserService`'s own role lookup) and `caseService` (checked
	// against `repository.UserRepository.GetUserRoles`' own user_role
	// vocabulary) — the same role name is meaningful in both, since
	// "CS engineer" and "support engineer" are the same real-world role,
	// not two different configs.
	CSEngineerRole string
	// SLARecomputeInterval is how often SLAEngineRecomputeWorker
	// recomputes every CSM-native "sla" row's elapsed percentage/breach
	// status (internal/service/sla_engine_recompute_worker.go). Same
	// envDuration convention as CRNoticePollInterval/GithubOutboundInterval
	// above.
	SLARecomputeInterval time.Duration
	// SalesforceIngestRetryInterval is how often SalesforceIngestRetryWorker
	// re-runs Salesforce ingests that FAILED because the record's parent
	// (project, account) was not in CSM yet
	// (internal/service/salesforce_ingest_retry_worker.go), and also how
	// old a failure must be before it is re-run. From
	// SALESFORCE_INGEST_RETRY_INTERVAL; defaults to 5m, the ServiceNow
	// sync's own cadence. Unlike the other intervals, an explicit "0"
	// disables the job (envDurationOrOff), because it makes outbound Sales
	// Entity calls on its own initiative and an operator must be able to
	// stop that without turning the ingest off. An invalid or negative value
	// disables it too (with a warning) rather than falling back to 5m.
	SalesforceIngestRetryInterval time.Duration
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
	// CloudStatusServiceIDs are the business services whose outages are
	// published to the public cloud status dashboard, as a comma-separated
	// list of UUIDs (CLOUD_STATUS_SERVICE_IDS).
	//
	// These are `service` rows, NOT service offerings. The ServiceNow flow
	// this ports dot-walked an outage's configuration item AS a service
	// offering and compared that offering's PARENT against a list of 14 ids.
	// Setting offering ids here instead would match nothing and the sweep
	// would silently never fire.
	//
	// Empty means the sweep is a no-op, which it logs. That is the safe
	// default: an unconfigured deployment posts nothing to a public status
	// page rather than guessing a scope.
	CloudStatusServiceIDs []string

	// CloudStatusDrainerEnabled turns on the background drainer that records
	// outage transitions AND rewrites cloud_monitor.status
	// (CLOUD_STATUS_DRAINER_ENABLED, default false).
	//
	// *** OFF BY DEFAULT BECAUSE OF THE STATUS WRITE. *** While
	// csm-sync-service's one-time bulk migration is still running, both it and
	// this drainer can write cloud_monitor.status. Clearing
	// CLOUD_STATUS_SERVICE_IDS would stop the drainer but also disable the
	// sweep endpoint and the dashboard reads, so the write needs a switch of
	// its own.
	CloudStatusDrainerEnabled bool

	// CloudStatusPollInterval is how often CloudStatusDrainer claims
	// event_outbox rows for `outage` and `outage_affected_ci`
	// (CLOUD_STATUS_POLL_INTERVAL). Same envDuration convention as
	// CRNoticePollInterval.
	//
	// This is the FAST path. The reconciliation sweep in
	// csm-scheduled-tasks reaches the same conclusions on its own schedule
	// and is what makes a missed drain harmless, so this interval governs
	// promptness, not correctness.
	CloudStatusPollInterval time.Duration

	// IncidentReportPollInterval is how often IncidentReportDrainer looks for
	// incident changes in event_outbox (INCIDENT_REPORT_POLL_INTERVAL,
	// default 5s). Same envDuration convention as CRNoticePollInterval. The
	// drainer itself has no on/off switch: like the ServiceNow flows it
	// ports, it runs wherever the data is.
	IncidentReportPollInterval time.Duration

	AuthIssuer             string
	AuthJWKSURL            string
	AuthUserTokenAudiences []string
	AuthClockSkew          time.Duration
	// M2MClientIDsRaw is the M2M_CLIENT_IDS value, a comma-separated list of
	// Asgardeo application client ids for pure machine-to-machine callers --
	// no human in the loop at all (the GitHub webhook delivery/service-
	// request handlers, the Salesforce partner ingest, and similar). M2MClientIDs
	// is its parsed set. A request whose Authorization: Bearer client-
	// credentials token names one of these ids is unconditionally treated as
	// an internal caller with unrestricted access to every project and
	// case, regardless of any x-user-id-token it also carries -- a
	// forwarded user token, if present at all, is used only for
	// attribution (created_by/updated_by), never for scoping.
	//
	// Also gates AddCaseTagRequest.ActorEmail/CreateCaseCommentRequest.ActorEmail
	// (internal/handler/case_handler.go): a caller whose x-jwt-assertion
	// names a client id in this same set may claim ANY actorEmail as the
	// acting user for a tag/comment write -- the same "M2MClientIDs wins
	// outright, unconditionally" trust this list already carries for
	// scoping. This used to be a second, separate email-based allowlist
	// (M2M_TRUSTED_ACTOR_EMAILS), replaced in favor of one list to keep
	// trusted internal callers in: a caller already trusted to bypass RLS
	// entirely needs no second, narrower list just to claim a comment/tag
	// author.
	//
	// This is deliberately NOT where apps/csm-portal/backend or
	// apps/customer-portal/backend-v2 belong, even though both are
	// internal-to-WSO2 services: both forward a human's own request, so
	// both need the human's identity to actually matter for scoping --
	// see CSMPortalBackendClientID and CustomerPortalBackendClientID below, which is why
	// this scheme uses three distinct configs rather than one shared list a
	// customer-facing BFF's id could be accidentally pasted into.
	M2MClientIDsRaw string
	M2MClientIDs    map[string]bool
	// CSMPortalBackendClientID is CSM_PORTAL_BACKEND_CLIENT_ID, the single client id of
	// apps/csm-portal/backend (the internal CS-engineer portal's BFF). A
	// request whose client-credentials token names this id is treated as
	// unrestricted ONLY if the forwarded x-user-id-token's email also ends
	// in CSMPortalUserDomain (case-insensitive) -- unlike M2MClientIDs,
	// trusting the client id alone is not enough, because this caller
	// always forwards a real human's request, and that human might not
	// actually be WSO2/partner staff (a misassigned Asgardeo role, for
	// instance). A caller using this client id whose email doesn't match
	// the domain is refused outright, not silently resolved some other way
	// -- CSM portal traffic is expected to always be WSO2-domain, so a
	// mismatch here means something upstream (the IdP, SCIM provisioning)
	// already got it wrong, which this service should surface, not paper
	// over.
	//
	// This also closes the "internal user with no `user` table row" gap:
	// an email that matches CSMPortalUserDomain is unrestricted on domain
	// alone, with no users-by-email lookup at all, so a WSO2 engineer who
	// hasn't been separately provisioned a `user` row is never blocked by
	// that.
	CSMPortalBackendClientID string
	// CSMPortalUserDomain is CSM_PORTAL_USER_DOMAIN, the email domain
	// (e.g. "wso2.com", no leading "@") CSMPortalBackendClientID's forwarded
	// caller must belong to. Required together with CSMPortalBackendClientID --
	// see Validate.
	CSMPortalUserDomain string
	// CustomerPortalBackendClientID is CUSTOMER_PORTAL_BACKEND_CLIENT_ID, the single
	// client id of apps/customer-portal/backend-v2 (or any successor). It
	// is checked FIRST, before M2MClientIDs or CSMPortalBackendClientID, and
	// always resolves purely from the forwarded x-user-id-token -- never
	// unconditionally trusted, by construction, regardless of what else
	// this client id might accidentally also appear in (M2MClientIDs, or
	// equal to CSMPortalBackendClientID by a copy-paste mistake: see Validate).
	// This is the structural fix for the scenario the three-config split
	// exists to prevent: a customer-facing BFF's client id ending up
	// wired to unconditional, RLS-bypassing access to every project and
	// case for every customer.
	CustomerPortalBackendClientID string
	// SalesEntity* is the Choreo connection to REST sales/sales-entity-service
	// (POST /customer-search), not GraphQL sales/entity-graphql-service and not
	// Salesforce. The four connection fields are all-or-nothing like Event Hub.
	// Scopes are optional (same as SERVICENOW_INTEGRATION_SERVICE_SCOPES).
	SalesEntityBaseURL      string
	SalesEntityTokenURL     string
	SalesEntityClientID     string
	SalesEntityClientSecret string
	SalesEntityScopes       string

	// Escalation* configure the fixed, deployment-specific notification
	// recipient GROUPS EscalationService.CreateEscalation (Postgres data
	// source) layers on top of the per-case-derived ones (account technical
	// owner, CRE team lead, product routing, CSM) -- see that method's own
	// doc comment for the full EL1..EL5 cumulative rule these feed. Each one
	// is a "group".id (migration 0074), resolved to its real member list
	// via team_member.group_id, NOT a single fixed address -- every
	// configured tier notifies however many people are actually in that
	// group. Every one of these is OPTIONAL: an unset/empty value means "no
	// recipients from this slot," never a startup failure or a request
	// error -- not every deployment configures every tier on day one, same
	// reasoning CustomerRoles/CSEngineerRole's own doc comments give for
	// org-specific vocabulary that doesn't belong hardcoded in this repo.
	// None of these are required by Validate for that reason, though a SET
	// value is still checked there for being a well-formed UUID (a
	// misconfigured group id would otherwise silently resolve zero
	// recipients instead of surfacing the typo at startup).
	EscalationEL1AmericasTLGroupID string
	EscalationEL2AmericasTUGroupID string
	// EscalationEL2ServiceProductGroupID/EscalationEL2IdentityServerGroupID/
	// EscalationEL2DefaultProductGroupID are the three product-routed EL2
	// buckets: the case's deployed product's category/business_unit picks
	// exactly one (SERVICE -> service; SOFTWARE with business_unit IAM ->
	// identity server; everything else, including no business_unit -> the
	// software default). A case with no deployed product/product info at
	// all gets none of the three, silently.
	EscalationEL2ServiceProductGroupID string
	EscalationEL2IdentityServerGroupID string
	EscalationEL2DefaultProductGroupID string
	EscalationEL3CREHeadGroupID        string
	EscalationEL4CCOGroupID            string
	EscalationEL4CROGroupID            string
	EscalationEL5CEOGroupID            string

	// RedisURL/RedisAddr/RedisPassword configure the optional user cache in
	// front of GET /users/{id} and GET /users/me (internal/cache), with the
	// same convention as integrations/csm-notification-service: RedisURL is a
	// rediss://:<password>@<host>:<port> connection string for a managed,
	// TLS-only Redis (Azure Managed Redis) and takes priority; RedisAddr/
	// RedisPassword are the plain, non-TLS pair for a local Redis. Neither set
	// means no cache: every read goes to Postgres, as before.
	//
	// The client is a plain redis.NewClient, so the target must be a
	// non-clustered Redis or one under the "Enterprise" clustering policy, not
	// "OSS Cluster".
	RedisURL      string
	RedisAddr     string
	RedisPassword string
	// UserCacheTTL bounds how long a cached user survives without an
	// invalidation (USER_CACHE_TTL, default 10m). Every writer of user, role
	// and membership data invalidates the affected user after it commits, so
	// this is the backstop for a missed invalidation, not the main freshness
	// mechanism.
	UserCacheTTL time.Duration
	// ServerReadTimeout and ServerWriteTimeout are the main API server's
	// http.Server ReadTimeout/WriteTimeout (SERVER_READ_TIMEOUT,
	// SERVER_WRITE_TIMEOUT). The health server keeps its own fixed timeouts.
	ServerReadTimeout  time.Duration
	ServerWriteTimeout time.Duration
	// RequestTimeout cancels each request's context (REQUEST_TIMEOUT).
	// Keeping it shorter than ServerWriteTimeout lets the handler write a
	// clean error, but this is not enforced.
	RequestTimeout time.Duration
	// UpstreamClientTimeout is the data-source HTTP client timeout
	// (UPSTREAM_CLIENT_TIMEOUT).
	UpstreamClientTimeout time.Duration

	// loadErr records the first unparsable environment value seen by Load,
	// which has no error return. Validate reports it.
	loadErr error
}

// Load reads configuration from environment variables and returns a populated
// Config. Missing variables fall back to sensible defaults; callers should
// validate required fields (e.g. DBUser, DBPassword, DBName) before use.
func Load() *Config {
	var loadErr error
	duration := func(key string, def time.Duration) time.Duration {
		d, err := getDurationOrDefault(key, def)
		if err != nil && loadErr == nil {
			loadErr = err
		}
		return d
	}
	intVal := func(key string, def int32, allowZero bool) int32 {
		n, err := getInt32OrDefault(key, def, allowZero)
		if err != nil && loadErr == nil {
			loadErr = err
		}
		return n
	}
	cfg := &Config{
		DBHost:                                   getEnvOrDefault("DB_HOST", "localhost"),
		DBPort:                                   getEnvOrDefault("DB_PORT", "5432"),
		AvailabilityTimezone:                     os.Getenv("AVAILABILITY_TIMEZONE"),
		DBUser:                                   os.Getenv("DB_USER"),
		DBPassword:                               os.Getenv("DB_PASSWORD"),
		DBName:                                   os.Getenv("DB_NAME"),
		DBSSLMode:                                os.Getenv("DB_SSLMODE"),
		DBSchema:                                 os.Getenv("DB_SCHEMA"),
		DBPoolMaxConns:                           intVal("DB_POOL_MAX_CONNS", 20, false),
		DBPoolMinConns:                           intVal("DB_POOL_MIN_CONNS", 2, true),
		DBPoolMaxConnLifetime:                    duration("DB_POOL_MAX_CONN_LIFETIME", 30*time.Minute),
		DBPoolMaxConnIdleTime:                    duration("DB_POOL_MAX_CONN_IDLE_TIME", 5*time.Minute),
		DBReadPoolEnabled:                        os.Getenv("DB_READ_POOL_ENABLED") == "true",
		DBReadPoolMaxConns:                       intVal("DB_READ_POOL_MAX_CONNS", 10, false),
		DBReadPoolMinConns:                       intVal("DB_READ_POOL_MIN_CONNS", 2, true),
		DBReadPoolMaxConnLifetime:                duration("DB_READ_POOL_MAX_CONN_LIFETIME", 30*time.Minute),
		DBReadPoolMaxConnIdleTime:                duration("DB_READ_POOL_MAX_CONN_IDLE_TIME", 5*time.Minute),
		ServerPort:                               getEnvOrDefault("SERVER_PORT", "8080"),
		HealthPort:                               getEnvOrDefault("HEALTH_PORT", "8081"),
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
		GithubIntegrationEnabled:                 os.Getenv("GITHUB_INTEGRATION_ENABLED") == "true",
		GithubBaseURL:                            getEnvOrDefault("GITHUB_API_BASE_URL", "https://api.github.com"),
		GithubToken:                              os.Getenv("GITHUB_TOKEN"),
		GithubIntegrationLogin:                   os.Getenv("GITHUB_INTEGRATION_LOGIN"),
		GithubOutboundInterval:                   envDuration("GITHUB_OUTBOUND_INTERVAL", 15*time.Second),
		CSMPortalBaseURL:                         os.Getenv("CSM_PORTAL_BASE_URL"),
		SpecialistHandoffConfig:                  os.Getenv("SPECIALIST_HANDOFF_CONFIG"),
		SpecialistHandoffGithubTokens:            os.Getenv("SPECIALIST_HANDOFF_GITHUB_TOKENS"),
		GithubLabelTypeIncident:                  os.Getenv("GITHUB_LABEL_TYPE_INCIDENT"),
		GithubLabelTypeServiceRequest:            os.Getenv("GITHUB_LABEL_TYPE_SERVICE_REQUEST"),
		GithubLabelsClass:                        os.Getenv("GITHUB_LABELS_CLASS"),
		GithubLabelStatusAssigned:                os.Getenv("GITHUB_LABEL_STATUS_ASSIGNED"),
		CRNoticesEnabled:                         os.Getenv("CR_NOTICES_ENABLED") == "true",
		CSMMigrationSalesforceMembershipIngestEnabled: os.Getenv("CSM_MIGRATION_SALESFORCE_MEMBERSHIP_INGEST_ENABLED") == "true",
		CSMMigrationSalesforceAccountIngestEnabled:    os.Getenv("CSM_MIGRATION_SALESFORCE_ACCOUNT_INGEST_ENABLED") == "true",
		CSMMigrationPortalWritesEnabled:               os.Getenv("CSM_MIGRATION_PORTAL_WRITES_ENABLED") == "true",
		CREventHubTopic:                               getEnvOrDefault("CR_EVENT_HUB_TOPIC", "cr-events"),
		ProjectEventHubTopic:                          getEnvOrDefault("PROJECT_EVENT_HUB_TOPIC", "project-events"),
		CRNoticePollInterval:                          envDuration("CR_NOTICE_POLL_INTERVAL", 5*time.Second),
		OutageEventHubTopic:                           getEnvOrDefault("OUTAGE_EVENT_HUB_TOPIC", "outage-events"),
		SREEventHubTopic:                              strings.TrimSpace(os.Getenv("SRE_EVENT_HUB_TOPIC")),
		SRAlertSRETeamIDs:                             splitComma(os.Getenv("SR_ALERT_SRE_TEAM_IDS")),
		OutageNoticePollInterval:                      envDuration("OUTAGE_NOTICE_POLL_INTERVAL", 60*time.Second),
		OutageNotificationRecipients:                  splitComma(os.Getenv("OUTAGE_NOTIFICATION_RECIPIENTS")),
		OutageCommunicationRecipients:                 splitComma(os.Getenv("OUTAGE_COMMUNICATION_RECIPIENTS")),
		AuthIssuer:                                    os.Getenv("AUTH_ISSUER"),
		AuthJWKSURL:                                   os.Getenv("AUTH_JWKS_URL"),
		AuthUserTokenAudiences:                        splitComma(os.Getenv("AUTH_USER_TOKEN_AUDIENCES")),
		AuthClockSkew:                                 envDuration("AUTH_CLOCK_SKEW", 30*time.Second),
		M2MClientIDsRaw:                               os.Getenv("M2M_CLIENT_IDS"),
		CSMPortalBackendClientID:                      os.Getenv("CSM_PORTAL_BACKEND_CLIENT_ID"),
		CSMPortalUserDomain:                           strings.TrimPrefix(os.Getenv("CSM_PORTAL_USER_DOMAIN"), "@"),
		CustomerPortalBackendClientID:                 os.Getenv("CUSTOMER_PORTAL_BACKEND_CLIENT_ID"),
		CustomerRoles:                                 splitComma(os.Getenv("CUSTOMER_ROLES")),
		CSEngineerRole:                                os.Getenv("CS_ENGINEER_ROLE"),
		SLARecomputeInterval:                          envDuration("SLA_RECOMPUTE_INTERVAL", 45*time.Second),
		CloudStatusServiceIDs:                         splitComma(os.Getenv("CLOUD_STATUS_SERVICE_IDS")),
		CloudStatusDrainerEnabled:                     os.Getenv("CLOUD_STATUS_DRAINER_ENABLED") == "true",
		CloudStatusPollInterval:                       envDuration("CLOUD_STATUS_POLL_INTERVAL", 10*time.Second),
		IncidentReportPollInterval:                    envDuration("INCIDENT_REPORT_POLL_INTERVAL", 5*time.Second),
		SalesforceIngestRetryInterval:                 envDurationOrOff("SALESFORCE_INGEST_RETRY_INTERVAL", 5*time.Minute),
		SalesEntityBaseURL:                            os.Getenv("SALES_ENTITY_BASE_URL"),
		SalesEntityTokenURL:                           os.Getenv("SALES_ENTITY_TOKEN_URL"),
		SalesEntityClientID:                           os.Getenv("SALES_ENTITY_CLIENT_ID"),
		SalesEntityClientSecret:                       os.Getenv("SALES_ENTITY_CLIENT_SECRET"),
		SalesEntityScopes:                             os.Getenv("SALES_ENTITY_SCOPES"),
		CSMMigrationMembershipRegistrationEnabled:     os.Getenv("CSM_MIGRATION_MEMBERSHIP_REGISTRATION_ENABLED") == "true",
		EscalationEL1AmericasTLGroupID:                os.Getenv("ESCALATION_EL1_AMERICAS_TL_GROUP_ID"),
		EscalationEL2AmericasTUGroupID:                os.Getenv("ESCALATION_EL2_AMERICAS_TU_GROUP_ID"),
		EscalationEL2ServiceProductGroupID:            os.Getenv("ESCALATION_EL2_SERVICE_PRODUCT_GROUP_ID"),
		EscalationEL2IdentityServerGroupID:            os.Getenv("ESCALATION_EL2_IDENTITY_SERVER_GROUP_ID"),
		EscalationEL2DefaultProductGroupID:            os.Getenv("ESCALATION_EL2_DEFAULT_PRODUCT_GROUP_ID"),
		EscalationEL3CREHeadGroupID:                   os.Getenv("ESCALATION_EL3_CRE_HEAD_GROUP_ID"),
		EscalationEL4CCOGroupID:                       os.Getenv("ESCALATION_EL4_CCO_GROUP_ID"),
		EscalationEL4CROGroupID:                       os.Getenv("ESCALATION_EL4_CRO_GROUP_ID"),
		EscalationEL5CEOGroupID:                       os.Getenv("ESCALATION_EL5_CEO_GROUP_ID"),
		ServerReadTimeout:                             duration("SERVER_READ_TIMEOUT", DefaultServerReadTimeout),
		ServerWriteTimeout:                            duration("SERVER_WRITE_TIMEOUT", DefaultServerWriteTimeout),
		RequestTimeout:                                duration("REQUEST_TIMEOUT", DefaultRequestTimeout),
		UpstreamClientTimeout:                         duration("UPSTREAM_CLIENT_TIMEOUT", DefaultUpstreamClientTimeout),
		loadErr:                                       loadErr,
	}
	cfg.M2MClientIDs = ParseInternalClientIDs(cfg.M2MClientIDsRaw)
	if cfg.CustomerPortalBackendClientID != "" && cfg.M2MClientIDs[cfg.CustomerPortalBackendClientID] {
		slog.Warn("CUSTOMER_PORTAL_BACKEND_CLIENT_ID is also listed in M2M_CLIENT_IDS; CustomerPortalBackendClientID is still checked first and always resolved from the forwarded user token, so this has no effect on access, but the M2M_CLIENT_IDS entry is almost certainly a copy-paste mistake",
			"clientId", cfg.CustomerPortalBackendClientID)
	}
	if cfg.CSMPortalBackendClientID != "" && cfg.M2MClientIDs[cfg.CSMPortalBackendClientID] {
		slog.Warn("CSM_PORTAL_BACKEND_CLIENT_ID is also listed in M2M_CLIENT_IDS; CSMPortalBackendClientID is still checked before M2MClientIDs and still requires a matching user-email domain, so this has no effect on access, but the M2M_CLIENT_IDS entry is almost certainly a copy-paste mistake",
			"clientId", cfg.CSMPortalBackendClientID)
	}
	// Set outside the literal so its longer key does not realign every field above.
	cfg.CSMMigrationSalesforceOpportunityIngestEnabled = os.Getenv("CSM_MIGRATION_SALESFORCE_OPPORTUNITY_INGEST_ENABLED") == "true"
	cfg.CSMMigrationSalesforceProjectIngestEnabled = os.Getenv("CSM_MIGRATION_SALESFORCE_PROJECT_INGEST_ENABLED") == "true"
	cfg.CSMMigrationSalesforceProjectInsertEnabled = os.Getenv("CSM_MIGRATION_SALESFORCE_PROJECT_INSERT_ENABLED") == "true"
	cfg.CSMMigrationSalesforcePartnerIngestEnabled = os.Getenv("CSM_MIGRATION_SALESFORCE_PARTNER_INGEST_ENABLED") == "true"
	cfg.CSMMigrationCustomerEngagementIngestEnabled = os.Getenv("CSM_MIGRATION_CUSTOMER_ENGAGEMENT_INGEST_ENABLED") == "true"
	cfg.CustomerEngagementFirefightingTypeID = strings.TrimSpace(os.Getenv("CUSTOMER_ENGAGEMENT_FIREFIGHTING_TYPE_ID"))
	cfg.RedisURL = strings.TrimSpace(os.Getenv("REDIS_URL"))
	cfg.RedisAddr = strings.TrimSpace(os.Getenv("REDIS_ADDR"))
	cfg.RedisPassword = os.Getenv("REDIS_PASSWORD")
	cfg.UserCacheTTL = envDuration("USER_CACHE_TTL", 10*time.Minute)
	cfg.applySREEventHubTopic()
	// Read-pool connection fields fall back to the write pool's so the read
	// pool targets the same primary until a replica host is configured.
	cfg.DBReadHost = getEnvOrDefault("DB_READ_HOST", cfg.DBHost)
	cfg.DBReadPort = getEnvOrDefault("DB_READ_PORT", cfg.DBPort)
	cfg.DBReadUser = getEnvOrDefault("DB_READ_USER", cfg.DBUser)
	cfg.DBReadPassword = getEnvOrDefault("DB_READ_PASSWORD", cfg.DBPassword)
	cfg.DBReadName = getEnvOrDefault("DB_READ_NAME", cfg.DBName)
	cfg.DBReadSSLMode = getEnvOrDefault("DB_READ_SSLMODE", cfg.DBSSLMode)
	return cfg
}

// HasRedis reports whether a Redis connection is configured, which turns on
// the user cache. Either REDIS_URL or REDIS_ADDR is enough.
func (c *Config) HasRedis() bool {
	return c.RedisURL != "" || c.RedisAddr != ""
}

// ParseInternalClientIDs parses a comma-separated client id list (M2M_CLIENT_IDS)
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

// getDurationOrDefault parses key as a Go duration string (e.g. "60s"). An
// unset or empty value yields defaultVal; an unparsable one is an error.
func getDurationOrDefault(key string, defaultVal time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return defaultVal, fmt.Errorf("invalid %s %q: %w", key, v, err)
	}
	return d, nil
}

// getInt32OrDefault parses key as a base-10 integer. An unset/empty value
// yields defaultVal; a non-numeric one is an error (and also falls back to
// defaultVal) -- same fail-safe-to-default posture as an unparseable
// duration (see getDurationOrDefault) rather than passing a bad value
// through. allowZero distinguishes DB_POOL_MIN_CONNS (pgxpool genuinely
// accepts 0 -- a deployment that doesn't want to retain any idle
// connections at all) from DB_POOL_MAX_CONNS (0 or negative would
// misconfigure pgxpool outright, since a pool that may open no connections
// at all can never serve a single query).
func getInt32OrDefault(key string, defaultVal int32, allowZero bool) (int32, error) {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal, nil
	}
	n, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		return defaultVal, fmt.Errorf("invalid %s %q: %w", key, v, err)
	}
	if n < 0 || (n == 0 && !allowZero) {
		want := "a positive integer"
		if allowZero {
			want = "a non-negative integer"
		}
		return defaultVal, fmt.Errorf("invalid %s %q: must be %s", key, v, want)
	}
	return int32(n), nil
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
// EVENT_HUB_TOPIC are only partially set, if the SALES_ENTITY_* vars are
// only partially set, or if SERVER_READ_TIMEOUT/SERVER_WRITE_TIMEOUT/
// REQUEST_TIMEOUT/UPSTREAM_CLIENT_TIMEOUT are unparsable or not positive.
func (c *Config) Validate() error {
	if c.loadErr != nil {
		return c.loadErr
	}
	for _, t := range []struct {
		name string
		val  time.Duration
	}{
		{"SERVER_READ_TIMEOUT", c.ServerReadTimeout},
		{"SERVER_WRITE_TIMEOUT", c.ServerWriteTimeout},
		{"REQUEST_TIMEOUT", c.RequestTimeout},
		{"UPSTREAM_CLIENT_TIMEOUT", c.UpstreamClientTimeout},
	} {
		if t.val <= 0 {
			return fmt.Errorf("%s must be greater than 0, got %s", t.name, t.val)
		}
	}
	// A team id that is not a UUID can never match account.sre_team_id, so
	// the team it was meant to automate would silently never be.
	for _, id := range c.SRAlertSRETeamIDs {
		if !validate.IsUUID(id) {
			return fmt.Errorf("SR_ALERT_SRE_TEAM_IDS: %q is not a UUID", id)
		}
	}
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
	// The read pool is an addition to the write pool, never a substitute: with
	// no database configured there is nothing for it to read from. Fail at
	// startup rather than run with a flag that silently does nothing.
	if c.DBReadPoolEnabled {
		if !c.HasDatabase() {
			return fmt.Errorf("DB_READ_POOL_ENABLED=true requires a configured database (DB_USER, DB_PASSWORD, DB_NAME)")
		}
		if c.DBReadPoolMaxConns <= 0 {
			return fmt.Errorf("DB_READ_POOL_MAX_CONNS must be greater than 0, got %d", c.DBReadPoolMaxConns)
		}
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
	if (c.CSMPortalBackendClientID == "") != (c.CSMPortalUserDomain == "") {
		return fmt.Errorf("CSM_PORTAL_BACKEND_CLIENT_ID and CSM_PORTAL_USER_DOMAIN must be set together or not at all")
	}
	// Equal and non-empty is almost certainly a copy-paste mistake: the two
	// roles are opposite by design (CSMPortalBackendClientID can reach unrestricted
	// access given a matching domain; CustomerPortalBackendClientID structurally
	// never can, see ResolveScope), so one client id can never correctly
	// serve both at once.
	if c.CSMPortalBackendClientID != "" && c.CSMPortalBackendClientID == c.CustomerPortalBackendClientID {
		return fmt.Errorf("CSM_PORTAL_BACKEND_CLIENT_ID and CUSTOMER_PORTAL_BACKEND_CLIENT_ID must not be the same client id")
	}
	// Each Escalation*GroupID is optional (unset = no recipients from that
	// slot, see the field's own doc comment) but, if SET, must be a
	// well-formed "group".id -- otherwise a typo'd env var would silently
	// resolve to zero recipients at request time instead of failing loudly
	// at startup where it's actually actionable.
	escalationGroupIDs := map[string]string{
		"ESCALATION_EL1_AMERICAS_TL_GROUP_ID":     c.EscalationEL1AmericasTLGroupID,
		"ESCALATION_EL2_AMERICAS_TU_GROUP_ID":     c.EscalationEL2AmericasTUGroupID,
		"ESCALATION_EL2_SERVICE_PRODUCT_GROUP_ID": c.EscalationEL2ServiceProductGroupID,
		"ESCALATION_EL2_IDENTITY_SERVER_GROUP_ID": c.EscalationEL2IdentityServerGroupID,
		"ESCALATION_EL2_DEFAULT_PRODUCT_GROUP_ID": c.EscalationEL2DefaultProductGroupID,
		"ESCALATION_EL3_CRE_HEAD_GROUP_ID":        c.EscalationEL3CREHeadGroupID,
		"ESCALATION_EL4_CCO_GROUP_ID":             c.EscalationEL4CCOGroupID,
		"ESCALATION_EL4_CRO_GROUP_ID":             c.EscalationEL4CROGroupID,
		"ESCALATION_EL5_CEO_GROUP_ID":             c.EscalationEL5CEOGroupID,
	}
	for envVar, value := range escalationGroupIDs {
		if value != "" && !validate.IsUUID(value) {
			return fmt.Errorf("%s %q is not a valid UUID", envVar, value)
		}
	}
	if v := c.CustomerEngagementFirefightingTypeID; v != "" && !isSysID(v) {
		return fmt.Errorf("CUSTOMER_ENGAGEMENT_FIREFIGHTING_TYPE_ID must be a 32-character hex sys_id")
	}
	// The URL carries the Redis password, so neither it nor url.Parse's own
	// error (which quotes its input) may appear in this message.
	if c.RedisURL != "" {
		u, err := url.Parse(c.RedisURL)
		if err != nil || (u.Scheme != "redis" && u.Scheme != "rediss") || u.Host == "" {
			return fmt.Errorf("REDIS_URL must be a redis:// or rediss:// connection string with a host")
		}
	}
	return nil
}

// isSysID reports whether v is a 32-character lowercase hex ServiceNow sys_id.
func isSysID(v string) bool {
	if len(v) != 32 {
		return false
	}
	for _, r := range v {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

// PostgresAuthoritative reports whether PostgreSQL is the system of record:
// DATA_SOURCE=postgres, or postgres-servicenow-dual-write, which serves every
// read and write from PostgreSQL too and only mirrors some writes to
// ServiceNow afterwards.
//
// The customer onboarding features (the Salesforce membership ingest, the
// portal membership writes, first-access registration) need exactly this and
// nothing more. Memberships never go through the ServiceNow mirror: ServiceNow
// gets them from Salesforce, through its own Service Bus subscription, so it
// stays current in either mode.
func (c *Config) PostgresAuthoritative() bool {
	return c.DataSource == DataSourcePostgres || c.DataSource == DataSourcePostgresServiceNowDualWrite
}

// HasPortalMembershipWrites reports whether the portal-driven membership
// write endpoints may be registered: the flag is on, PostgreSQL is
// authoritative (the write is a Postgres transaction — there is no ServiceNow
// equivalent), and the REST sales/sales-entity-service connection is
// complete, since half of every one of those writes goes to Salesforce.
// routes.go ANDs this with db != nil, the same way every other
// Postgres-only feature set is gated.
func (c *Config) HasPortalMembershipWrites() bool {
	return c.CSMMigrationPortalWritesEnabled &&
		c.PostgresAuthoritative() &&
		c.SalesEntityConfigured()
}

// HasCustomerEngagementIngest reports whether POST /customer-engagements/allocation-events
// may be registered: the flag is on and PostgreSQL is authoritative.
func (c *Config) HasCustomerEngagementIngest() bool {
	return c.CSMMigrationCustomerEngagementIngestEnabled && c.PostgresAuthoritative()
}

// SalesEntityConfigured reports whether every REST sales/sales-entity-service env var is set.
func (c *Config) SalesEntityConfigured() bool {
	return c.SalesEntityBaseURL != "" &&
		c.SalesEntityTokenURL != "" &&
		c.SalesEntityClientID != "" &&
		c.SalesEntityClientSecret != ""
}

// DSN constructs a PostgreSQL connection string from the config fields.
//
// Pins search_path to DBSchema via the "options" connection parameter, the
// same mechanism operations/csm-sync-service's own withSchema uses. When
// DBSchema is unset, falls back to "DBUser,public" (or just "public" with
// no DBUser) — Postgres' own default search_path, made explicit here rather
// than left to that default, since setting search_path at all replaces it
// rather than extending it, and every deployment's tables today live in
// "public" (unqualified migrations, no deployment sets DB_SCHEMA yet).
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

	schema := c.DBSchema
	if schema == "" {
		// No schema configured -- mirror Postgres' own default search_path
		// ("$user", public) explicitly, not just the "$user" half of it.
		// An explicit search_path completely replaces Postgres' own
		// default rather than extending it, and every deployment's tables
		// today live in "public" (entity-service's migrations create them
		// unqualified, and no deployment sets DB_SCHEMA yet) -- dropping
		// "public" here would make every one of those tables unresolvable
		// the moment this shipped. No space after the comma: the "options"
		// connection parameter tokenizes on whitespace to separate multiple
		// "-c name=value" entries, so a space here splits "public" off into
		// its own (invalid) token and Postgres sees a truncated search_path
		// value instead of the full list -- confirmed against a real
		// connection, which rejected "<user>," as an invalid value.
		if c.DBUser != "" {
			schema = c.DBUser + ",public"
		} else {
			schema = "public"
		}
	}
	// url.Values.Encode() would percent-encode the space in
	// "-c search_path=..." as "+" (the HTML-form convention) -- pgconn's own
	// URI parser does not decode "+" back to a space, so Postgres received a
	// literal "+" and rejected it as an unrecognized configuration parameter
	// (confirmed against a real connection). Escape by hand with %20
	// instead, which pgconn does handle.
	opts := strings.ReplaceAll(url.QueryEscape("-c search_path="+schema), "+", "%20")
	u.RawQuery += "&options=" + opts
	return u.String()
}

// ReadDSN is DSN for the read pool: the DBRead* connection fields, with the
// same search_path handling. Everything else about the connection is shared
// with the write pool, so a replica sees the same schema resolution.
func (c *Config) ReadDSN() string {
	rc := *c
	rc.DBHost, rc.DBPort = c.DBReadHost, c.DBReadPort
	rc.DBUser, rc.DBPassword = c.DBReadUser, c.DBReadPassword
	rc.DBName, rc.DBSSLMode = c.DBReadName, c.DBReadSSLMode
	// A hand-built Config (tests) may leave the read fields empty; behave as
	// Load does and fall back to the write values.
	if rc.DBHost == "" {
		rc.DBHost = c.DBHost
	}
	if rc.DBPort == "" {
		rc.DBPort = c.DBPort
	}
	if rc.DBUser == "" {
		rc.DBUser = c.DBUser
	}
	if rc.DBPassword == "" {
		rc.DBPassword = c.DBPassword
	}
	if rc.DBName == "" {
		rc.DBName = c.DBName
	}
	if rc.DBSSLMode == "" {
		rc.DBSSLMode = c.DBSSLMode
	}
	return rc.DSN()
}

// HasGithubIntegration reports whether the GitHub sync is both switched on and
// configured well enough to run.
//
// *** GITHUB_WEBHOOK_SECRET IS NO LONGER PART OF THIS, AND ITS ABSENCE HERE
// IS NOT AN OVERSIGHT. *** The HMAC check moved to
// operations/csm-webhooks along with the public endpoint, so this
// service never sees a signature and holding the secret would only imply it
// did. What still gates the integration is the outbound half: a token to
// call GitHub with, and the login whose own events must be ignored as ours.
func (c *Config) HasGithubIntegration() bool {
	return c.GithubIntegrationEnabled &&
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

// envDurationOrOff is envDuration for an interval that can be switched off:
// unset returns def, and an explicit zero ("0", "0s", "0m") returns 0, which
// the caller reads as "disabled". An unparseable or negative value also
// returns 0, with a warning: it fails closed, because an operator who wrote
// "off" or "-1" meant to stop the job, and falling back to def would start a
// worker that makes outbound calls they tried to turn off.
func envDurationOrOff(key string, def time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d < 0 {
		slog.Warn("invalid duration configuration value, treating it as disabled",
			"key", key, "value", v)
		return 0
	}
	return d
}

// applySREEventHubTopic points both operations publishers at SRE_EVENT_HUB_TOPIC
// when it is set. Done once here so every reader of CREventHubTopic and
// OutageEventHubTopic -- the publishers and their startup log lines -- agrees.
func (c *Config) applySREEventHubTopic() {
	if c.SREEventHubTopic == "" {
		return
	}
	c.CREventHubTopic = c.SREEventHubTopic
	c.OutageEventHubTopic = c.SREEventHubTopic
}
