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

package server

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/cache"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/config"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/github"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/handler"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesentity"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// NewRouter builds the dependency graph (repository → service → handler),
// registers all routes, and wraps the mux with the middleware chain:
// CorrelationID → Recovery → Logger → UserIDToken → Timeout.
//
// It also returns a shutdown function that closes EVERY Kafka producer it
// constructed — the shared-topic publisher and the onboarding-topic one —
// so the caller (server.New, then cmd/api/main.go) releases both. Returning
// one of the publishers instead, as this used to, left the second producer's
// connections open and a buffered project_contact.invited unflushed at exit.
// The function is never nil; with publishing unconfigured it simply has
// nothing to close. It also closes the user cache's Redis client, when one
// was built.
func NewRouter(db *pgxpool.Pool, cfg *config.Config) (http.Handler, func()) {
	// The customer-visibility policy of change requests (CR_STRICT_VISIBILITY_FROM),
	// shared by every repository that reads or writes a change request.
	crVisibility := CRVisibilityFromConfig(cfg)
	if db != nil {
		// Nothing reads the policy without a database (the ServiceNow data source
		// serves change requests itself), so only say which mode this process is in
		// when it matters.
		logCRVisibility(cfg)
	}
	userRepo := repository.NewUserRepository(db)
	userSvc := service.NewUserService(userRepo)

	// The user cache in front of GET /users/{id} and GET /users/me, on when
	// REDIS_URL or REDIS_ADDR is set (see internal/cache). Postgres-only:
	// those routes are served by the ServiceNow handlers in servicenow mode.
	// userCacheInvalidator stays a nil interface without it -- never a nil
	// *cache.UserCache inside a non-nil one -- so the writers below can test
	// it against nil.
	var userCacheInvalidator service.UserCacheInvalidator
	closeUserCache := func() {}
	if db != nil && cfg.HasRedis() {
		rdb, err := cache.NewRedisClient(cfg)
		if err != nil {
			panic("user cache: " + err.Error())
		}
		userCache := cache.NewUserCache(rdb, cfg.UserCacheTTL)
		userSvc = service.NewCachedUserService(userSvc, userCache)
		userCacheInvalidator = userCache
		closeUserCache = func() {
			if err := rdb.Close(); err != nil {
				log.Printf("user cache: close redis client: %v", err)
			}
		}
		log.Printf("user cache enabled (ttl %s)", cfg.UserCacheTTL)
	}
	userHandler := handler.NewUserHandler(userSvc)

	// accessSvc resolves the caller's AccessScope from the validated identity
	// auth.Middleware attaches to every request (see AccessService's own doc
	// comment for the full decision table). Constructed once and shared by
	// every Postgres-backed service that scopes its reads by it. It only takes
	// effect on the Postgres data source: in ServiceNow mode the project/case
	// reads go to ServiceNow itself with the forwarded x-user-id-token, so
	// scoping there is ServiceNow's own (snProjectService/snCaseService hold a
	// pgFallback but do not route GetProjectByID/GetCaseByID through it).
	accessSvc := service.NewAccessService(repository.NewAccessRepository(db), service.AccessClientConfig{
		M2MClientIDs:                  cfg.M2MClientIDs,
		CSMPortalBackendClientID:      cfg.CSMPortalBackendClientID,
		CSMPortalUserDomain:           cfg.CSMPortalUserDomain,
		CustomerPortalBackendClientID: cfg.CustomerPortalBackendClientID,
	})

	var savedFilterViewHandler *handler.SavedFilterViewHandler
	if db != nil {
		savedFilterViewHandler = handler.NewSavedFilterViewHandler(
			service.NewSavedFilterViewService(repository.NewSavedFilterViewRepository(db), userRepo),
		)
	}

	// event_publish_failures, sla_clocks, scheduled_task_run, and
	// alert_incident_mapping have no ServiceNow equivalent. They are
	// Postgres-backed and registered only when a pool is available
	// (db.NewPoolIfNeeded returns nil for DATA_SOURCE=servicenow so local
	// SN-mode startups are not blocked). Gate the whole chain on db != nil:
	// nil handler means the routes below are never registered, and nil
	// service means EventPublisherService records nothing rather than
	// dereferencing a nil pool.
	var eventPublishFailureSvc service.EventPublishFailureService
	var eventPublishFailureHandler *handler.EventPublishFailureHandler
	if db != nil {
		eventPublishFailureSvc = service.NewEventPublishFailureService(repository.NewEventPublishFailureRepository(db))
		eventPublishFailureHandler = handler.NewEventPublishFailureHandler(eventPublishFailureSvc)
	}

	// EventPublisherService is optional, like every ServiceNow-only
	// dependency below — gated on EventHubBroker rather than cfg.DataSource,
	// since publishing is a distinct concern from which backend serves reads
	// (see config.Config.EventHubBroker's doc comment). Also gated on
	// EventPublishingEnabled, a separate safe-by-default kill switch: Event
	// Hub can be fully configured and this still stays nil until that's
	// explicitly turned on. nil when unset; every caller (snCaseService,
	// snIncidentService) already handles that. eventPublishFailureSvc is
	// nil without a pool; Publish then skips durable recording.
	var eventPublisher service.EventPublisherService
	if cfg.EventHubBroker != "" && cfg.EventPublishingEnabled {
		eventPublisher = service.NewEventPublisherService(
			eventbus.NewProducer(eventbus.Config{
				Broker:           cfg.EventHubBroker,
				ConnectionString: cfg.EventHubConnectionString,
				Topic:            cfg.EventHubTopic,
			}),
			eventPublishFailureSvc,
		)
	}

	// Onboarding events go to their own topic, not the shared one, so a
	// case-event backlog cannot delay an invitation and the onboarding
	// dead-letter queue can be watched separately. Same broker, same
	// credentials, same failure recording -- only the topic differs. nil
	// under exactly the same conditions as eventPublisher above, and the
	// membership ingest already treats a nil publisher as "write the rows,
	// send nothing".
	var projectEventPublisher service.EventPublisherService
	if cfg.EventHubBroker != "" && cfg.EventPublishingEnabled {
		projectEventPublisher = service.NewEventPublisherService(
			eventbus.NewProducer(eventbus.Config{
				Broker:           cfg.EventHubBroker,
				ConnectionString: cfg.EventHubConnectionString,
				Topic:            cfg.ProjectEventHubTopic,
			}),
			eventPublishFailureSvc,
		)
	}

	// Incident events go to their own topic when IncidentEventHubTopic is set
	// (incidents are SRE work; case-events keeps the customer cases), and to
	// the shared one otherwise -- the same publisher as eventPublisher then.
	// Every incident.* event uses this one, so an incident's stop signals
	// always travel on the topic its trigger did.
	incidentEventPublisher := eventPublisher
	if cfg.EventHubBroker != "" && cfg.EventPublishingEnabled &&
		cfg.IncidentEventHubTopic != "" && cfg.IncidentEventHubTopic != cfg.EventHubTopic {
		incidentEventPublisher = service.NewEventPublisherService(
			eventbus.NewProducer(eventbus.Config{
				Broker:           cfg.EventHubBroker,
				ConnectionString: cfg.EventHubConnectionString,
				Topic:            cfg.IncidentEventHubTopic,
			}),
			eventPublishFailureSvc,
		)
		slog.Info("incident events have their own topic", "topic", cfg.IncidentEventHubTopic)
	}

	// sla-status reads the "sla" table directly (ServiceNow's own SLA data,
	// synced in) — no ServiceNow equivalent of its own, gated on the pool for
	// the same reason as event_publish_failures above. Replaces the old
	// sla_clocks table entirely; see domain.SLAStatus's own doc comment.
	var slaStatusHandler *handler.SLAStatusHandler
	if db != nil {
		slaStatusRepo := repository.NewSLAStatusRepository(repository.NewScoped(db))
		slaStatusHandler = handler.NewSLAStatusHandler(service.NewSLAStatusService(slaStatusRepo, accessSvc))
	}

	// Also constructed for DataSourcePostgresServiceNowDualWrite: that mode's
	// active services stay Postgres-backed (see the case wiring below), but
	// its best-effort mirror writes to the previous system still need this
	// client. config.Validate requires the same four credentials for both
	// modes. Built here, ahead of the GitHub sync, because that sync's state
	// writes are mirrored too (snChangeRequestMirrorSvc below).
	var serviceNowIntegrationServiceClient *integrationservice.Client
	if cfg.DataSource == config.DataSourceServiceNow || cfg.DataSource == config.DataSourcePostgresServiceNowDualWrite {
		serviceNowIntegrationServiceClient = integrationservice.New(cfg.ServiceNowIntegrationServiceBaseURL, integrationservice.ClientCredentialsConfig{
			TokenURL:     cfg.ServiceNowIntegrationServiceTokenURL,
			ClientID:     cfg.ServiceNowIntegrationServiceClientID,
			ClientSecret: cfg.ServiceNowIntegrationServiceClientSecret,
			Scopes:       cfg.ServiceNowIntegrationServiceScopes,
		}, cfg.UpstreamClientTimeout)
	}

	// snWritebackDispatcher is the single shared SNWritebackDispatcher for
	// every DATA_SOURCE=postgres-servicenow-dual-write best-effort mirror
	// write (see SNWritebackDispatcher's own doc comment) -- one dispatcher,
	// one small worker pool, reused by every entity's mirror rather than each
	// constructing its own: the GitHub sync's change request state writes
	// (immediately below), project, and case, call_request, time_card,
	// comment, change_request, and case tags/watch list (all further below,
	// riding on the case dispatch). nil in every other mode. Originally
	// constructed only inline for the case pilot; hoisted once a second
	// entity (project) needed the same instance, and again ahead of the
	// GitHub sync.
	//
	// snChangeRequestMirrorSvc is the previous system's change request
	// service the dual-write mode mirrors change request writes onto
	// (CreateChangeRequest / PatchChangeRequest / DecideChangeRequestApproval
	// are the only methods of it ever called); it is never the active
	// service -- reads stay on Postgres. nil in every other mode.
	var snWritebackDispatcher *service.SNWritebackDispatcher
	var snChangeRequestMirrorSvc service.ChangeRequestService
	var snWritebackFailureHandler *handler.SNWritebackFailureHandler
	if cfg.DataSource == config.DataSourcePostgresServiceNowDualWrite {
		snWritebackDispatcher = service.NewSNWritebackDispatcher(repository.NewSNWritebackFailureRepository(db))
		snChangeRequestMirrorSvc = service.NewServiceNowChangeRequestService(serviceNowIntegrationServiceClient)
		// The operator's view of what the previous system is missing and the
		// replay of one recorded write (internal callers only, registered
		// further down with the other Postgres-only operator routes).
		snWritebackFailureHandler = handler.NewSNWritebackFailureHandler(snWritebackDispatcher)
	}

	// scheduled_task_run has no ServiceNow equivalent either — same
	// reasoning as sla_clocks/event_publish_failures above. Backs
	// operations/csm-scheduled-tasks; see that component's own CLAUDE.md
	// and this service's CLAUDE.md ("Scheduled task runs").
	// The GitHub change-request sync needs a pool (the repository mapping and
	// the delivery log are tables) and its own switch. Gated on both, so the
	// webhook endpoint is not registered merely because a database exists --
	// it authenticates by HMAC rather than by bearer token, and an endpoint
	// that mutates change requests should appear only when asked for.
	var githubDeliveryHandler *handler.GithubDeliveryHandler
	var githubServiceRequestHandler *handler.GithubServiceRequestHandler

	// Assigned inside the GitHub-integration block below and read further down,
	// where activeCaseSvc finally exists, to build the native issue-filing
	// service. Both halves of the sync then share one client and one mapping
	// table.
	var (
		githubSyncRepo repository.GithubSyncRepository
		githubClient   *github.Client
		githubLabelSet service.GithubLabels
		// githubSync is the issue sync, kept for the SR automation set up
		// further down (WithGithubSRNotices).
		githubSync service.GithubSyncService
	)
	var scheduledTaskRunHandler *handler.ScheduledTaskRunHandler
	if db != nil {
		scheduledTaskRunHandler = handler.NewScheduledTaskRunHandler(service.NewScheduledTaskRunService(repository.NewScheduledTaskRunRepository(db)))
		if cfg.HasGithubIntegration() {
			githubLabels, labelErr := service.NewGithubLabels(service.GithubLabelOverrides{
				TypeIncident:       cfg.GithubLabelTypeIncident,
				TypeServiceRequest: cfg.GithubLabelTypeServiceRequest,
				Class:              cfg.GithubLabelsClass,
				StatusAssigned:     cfg.GithubLabelStatusAssigned,
			})
			if labelErr != nil {
				// A label override that does not parse would leave the sync
				// silently recognising nothing -- the exact failure that took
				// ServiceNow's integration down. Refuse to start instead.
				log.Fatalf("invalid GitHub label configuration: %v", labelErr)
			}
			// The outbound worker is started by cmd/api, which owns process
			// lifetime; routes.go only builds what the HTTP surface needs.
			githubSyncRepo = repository.NewGithubSyncRepository(repository.NewScoped(db))
			githubClient = github.NewClient(github.Config{
				BaseURL: cfg.GithubBaseURL,
				Token:   cfg.GithubToken,
			})
			githubLabelSet = githubLabels
			// Under dual-write a state the sync writes on a change request is
			// mirrored to the previous system like a staff PATCH's would be
			// (the one state rule: service.MirrorGithubStateChanges).
			githubMutations := repository.NewGithubMutationRepository(repository.NewScoped(db))
			if snWritebackDispatcher != nil {
				githubMutations = service.MirrorGithubStateChanges(githubMutations, snChangeRequestMirrorSvc, snWritebackDispatcher)
			}
			githubSyncSvc := service.NewGithubSyncServiceWriting(
				githubSyncRepo,
				githubMutations,
				githubClient,
				cfg.GithubIntegrationLogin,
				githubLabels,
			)
			// Each repository declares its own account and project in a file
			// it carries; account_github_repo is the fallback for one that
			// does not.
			githubSyncSvc = service.WithGithubRepoConfig(githubSyncSvc, githubClient, cfg.GithubRepoConfigPath)
			githubSync = githubSyncSvc
			githubDeliveryHandler = handler.NewGithubDeliveryHandler(githubSyncSvc, cfg.M2MClientIDs)
			githubServiceRequestHandler = handler.NewGithubServiceRequestHandler(githubSyncSvc, cfg.M2MClientIDs)
		}
	}

	// alert_incident_mapping has no ServiceNow equivalent either — same
	// reasoning as sla_clocks/scheduled_task_run/event_publish_failures above,
	// gated the same way: nil db means nil handler means the routes below are
	// never registered, rather than panicking on a nil pool.
	var alertIncidentMappingHandler *handler.AlertIncidentMappingHandler
	if db != nil {
		alertIncidentMappingRepo := repository.NewAlertIncidentMappingRepository(db)
		alertIncidentMappingHandler = handler.NewAlertIncidentMappingHandler(service.NewAlertIncidentMappingService(alertIncidentMappingRepo))
	}

	// announcement_requests has no ServiceNow equivalent either — same
	// reasoning as sla_clocks/scheduled_task_run/alert_incident_mapping
	// above, gated the same way: nil db means nil handler means the routes
	// below are never registered, rather than panicking on a nil pool.
	// Constructed further below (once activeCaseSvc exists), not here —
	// AutoPublish needs it for its own in-process case-creation fan-out.
	var announcementRequestHandler *handler.AnnouncementRequestHandler
	// Team Schedule is portal-native, like announcement_requests: it exists
	// only in Postgres, so its routes are registered only when a pool is
	// configured.
	var scheduleHandler *handler.ScheduleHandler
	// The SaaS SRE rota's "Generate month" action. Postgres-only, like the
	// rest of the Team Schedule.
	var rotaGenerateHandler *handler.RotaGenerateHandler
	var teamMemberHandler *handler.TeamMemberHandler
	var pagingChainHandler *handler.PagingChainHandler

	accountRepo := repository.NewAccountRepository(repository.NewScoped(db))
	accountHandler := handler.NewAccountHandler(service.NewAccountService(accountRepo))

	// teamHandler has no ServiceNow-backed counterpart to switch on — see
	// TeamService's own doc comment for why this is a new capability, not a
	// migrated one. Still gated on db != nil like every other Postgres-only
	// handler above: NewPoolIfNeeded returns a nil pool for
	// DATA_SOURCE=servicenow, and TeamRepository's queries would nil-pointer
	// on the first request rather than being unregistered like the rest.
	var teamHandler *handler.TeamHandler
	if db != nil {
		teamRepo := repository.NewTeamRepository(db)
		teamHandler = handler.NewTeamHandler(service.NewTeamService(teamRepo))
	}

	// groupDetailHandler (GET /groups/{id}: one group and its members, opened
	// from an approval stage's assignment group) is Postgres-only for the same
	// reason as teamHandler above, and gated on db != nil the same way.
	var groupDetailHandler *handler.GroupDetailHandler
	if db != nil {
		groupDetailHandler = handler.NewGroupDetailHandler(
			service.NewGroupDetailService(repository.NewGroupDetailRepository(db), accessSvc))
	}

	var salesforceEventHandler *handler.SalesforceEventHandler
	// salesforcePartnerHandler is set when the partner refresh is on.
	var salesforcePartnerHandler *handler.SalesforcePartnerHandler
	// membershipRegistrationHandler and projectContactSyncHandler both need
	// the very same membership-ingest-enabled SalesforceEventService this
	// block builds, so all three are wired together rather than side by side.
	var membershipIngestSvc service.SalesforceEventService
	var salesEntityClient *salesentity.Client
	// ingestRetryCtx stops the Salesforce ingest retry worker; closePublishers
	// (returned to cmd/api/main.go) cancels it on shutdown and waits on
	// ingestRetryWG for it to return. A worker that was never started leaves
	// the WaitGroup at zero, so the wait returns at once.
	ingestRetryCtx, stopIngestRetry := context.WithCancel(context.Background())
	var ingestRetryWG sync.WaitGroup
	// PostgresAuthoritative, not DATA_SOURCE=postgres alone: the dual-write
	// mode serves memberships from PostgreSQL too, and memberships reach
	// ServiceNow from Salesforce directly, so nothing here needs its mirror.
	if db != nil && cfg.PostgresAuthoritative() && cfg.SalesEntityConfigured() {
		salesEntityClient = salesentity.New(cfg.SalesEntityBaseURL, salesentity.ClientCredentialsConfig{
			TokenURL:     cfg.SalesEntityTokenURL,
			ClientID:     cfg.SalesEntityClientID,
			ClientSecret: cfg.SalesEntityClientSecret,
			Scopes:       cfg.SalesEntityScopes,
		})
		// A nil account repository turns the Account branch off: its envelopes
		// are acknowledged and ignored (see CSMMigrationSalesforceAccountIngestEnabled).
		var accountIngestRepo repository.AccountRepository
		if cfg.CSMMigrationSalesforceAccountIngestEnabled {
			accountIngestRepo = accountRepo
		}
		// The shared ingest support is flag-independent: the full account
		// repository serves EnsureAccount's read even while the Account
		// branch (the write side) is off, and the salesforce_ingest_state
		// ledger is where every non-membership family records its version.
		projectIngestRepo := repository.NewSalesforceProjectRepository(repository.NewScoped(db))
		ingestSupport := service.SalesforceIngestSupport{
			Accounts: accountRepo,
			States:   repository.NewSalesforceIngestStateRepository(db),
			Projects: projectIngestRepo,
		}
		// The Project branch is update-only unless its insert switch is on
		// too (csm-sync-service still inserts project rows until cutover).
		withProjectIngest := func(svc service.SalesforceEventService) service.SalesforceEventService {
			if !cfg.CSMMigrationSalesforceProjectIngestEnabled {
				return svc
			}
			return service.WithProjectIngest(svc, service.ProjectIngest{
				Projects:      projectIngestRepo,
				SalesEntity:   salesEntityClient,
				InsertEnabled: cfg.CSMMigrationSalesforceProjectInsertEnabled,
			})
		}
		// The Opportunity branch (sf_opportunity + derived line items) and
		// the Project branch are attached to whichever service variant is
		// built below; off, the envelopes are acknowledged and ignored.
		withOpportunityIngest := func(svc service.SalesforceEventService) service.SalesforceEventService {
			svc = withProjectIngest(svc)
			if !cfg.CSMMigrationSalesforceOpportunityIngestEnabled {
				return svc
			}
			svc = service.WithOpportunityIngest(svc, service.OpportunityIngest{
				Opportunities: repository.NewSalesforceOpportunityRepository(db),
				SalesEntity:   salesEntityClient,
			})
			// Linked_Opportunity__c belongs to the Opportunity family.
			svc = service.WithLinkedOpportunityIngest(svc, service.LinkedOpportunityIngest{
				Links:       repository.NewSalesforceOpportunityLinkRepository(db),
				SalesEntity: salesEntityClient,
			})
			// Invoice__c events ride on the same flag: an invoice needs its
			// opportunity, which the Opportunity branch ingests inline.
			opportunityLookup := repository.NewSalesforceOpportunityLookup(db)
			svc = service.WithInvoiceIngest(svc, service.InvoiceIngest{
				Invoices:      repository.NewSalesforceInvoiceRepository(db),
				Opportunities: opportunityLookup,
				SalesEntity:   salesEntityClient,
			})
			// So do standalone OpportunityLineItem events.
			return service.WithOpportunityLineItemIngest(svc, service.OpportunityLineItemIngest{
				LineItems:     repository.NewSalesforceOpportunityLineItemRepository(db),
				Opportunities: opportunityLookup,
				SalesEntity:   salesEntityClient,
			})
		}
		// The partner-link refresh is attached the same way; off, nothing
		// refreshes partners and the internal route is not registered.
		withPartnerIngest := func(svc service.SalesforceEventService) service.SalesforceEventService {
			if !cfg.CSMMigrationSalesforcePartnerIngestEnabled {
				return svc
			}
			svc = service.WithPartnerIngest(svc, service.PartnerIngest{
				Partners:    repository.NewAccountPartnerWriteRepository(db),
				SalesEntity: salesEntityClient,
			})
			if refresher, ok := svc.(service.PartnerRefresher); ok {
				salesforcePartnerHandler = handler.NewSalesforcePartnerHandler(service.NewPartnerRefreshService(refresher, accessSvc))
			}
			return svc
		}
		if cfg.CSMMigrationSalesforceMembershipIngestEnabled {
			// The membership branch (Project_Contact__c / Contact envelopes)
			// writes user/account_contact/project_contact rows and the
			// DATABASE onboarding step, and publishes project_contact.invited
			// when eventPublisher is configured (nil is a no-op there). Its
			// Contact writer writes a contact's user/account_contact rows
			// even without a membership, recording the ledger.
			stepRepo := repository.NewOnboardingStepRepository(db)
			membershipIngestSvc = service.NewSalesforceEventServiceWithMembershipIngest(
				accountIngestRepo, salesEntityClient, ingestSupport, service.MembershipIngest{
					Memberships: repository.NewProjectMembershipRepository(repository.NewScoped(db)),
					Steps:       stepRepo,
					SalesEntity: salesEntityClient,
					Contacts:    repository.NewSalesforceContactRepository(db),
					Publisher:   projectEventPublisher,
					UserCache:   userCacheInvalidator,
				})
			membershipIngestSvc = withPartnerIngest(withOpportunityIngest(membershipIngestSvc))
			salesforceEventHandler = handler.NewSalesforceEventHandler(membershipIngestSvc)

			// The delayed-retry job re-runs memberships whose project or
			// account was not in CSM when their event arrived. It lives here
			// rather than in cmd/api/main.go because it needs this very
			// service (with its publisher, so a re-run invitation is still
			// announced) and only makes sense when the membership ingest is
			// on. SALESFORCE_INGEST_RETRY_INTERVAL=0 turns it off.
			if cfg.SalesforceIngestRetryInterval > 0 {
				retrier, ok := membershipIngestSvc.(service.MembershipReingester)
				if !ok {
					panic("salesforce: membership ingest service does not implement MembershipReingester")
				}
				retryWorker := service.NewSalesforceIngestRetryWorker(stepRepo, retrier, ingestSupport.States, cfg.SalesforceIngestRetryInterval)
				if opp, ok := membershipIngestSvc.(service.OpportunityReingester); ok && cfg.CSMMigrationSalesforceOpportunityIngestEnabled {
					retryWorker.EntityRetriers[domain.SalesforceIngestEntityOpportunity] = opp.RetryOpportunityIngest
				}
				if link, ok := membershipIngestSvc.(service.LinkedOpportunityReingester); ok && cfg.CSMMigrationSalesforceOpportunityIngestEnabled {
					retryWorker.EntityRetriers[domain.SalesforceIngestEntityLinkedOpportunity] = link.RetryLinkedOpportunityIngest
				}
				if project, ok := membershipIngestSvc.(service.ProjectReingester); ok && cfg.CSMMigrationSalesforceProjectIngestEnabled {
					retryWorker.EntityRetriers[domain.SalesforceIngestEntityProject] = project.RetryProjectIngest
				}
				if inv, ok := membershipIngestSvc.(service.InvoiceReingester); ok && cfg.CSMMigrationSalesforceOpportunityIngestEnabled {
					retryWorker.EntityRetriers[domain.SalesforceIngestEntityInvoice] = inv.RetryInvoiceIngest
				}
				if li, ok := membershipIngestSvc.(service.OpportunityLineItemReingester); ok && cfg.CSMMigrationSalesforceOpportunityIngestEnabled {
					retryWorker.EntityRetriers[domain.SalesforceIngestEntityOpportunityLineItem] = li.RetryOpportunityLineItemIngest
				}
				if partners, ok := membershipIngestSvc.(service.PartnerReingester); ok && cfg.CSMMigrationSalesforcePartnerIngestEnabled {
					retryWorker.EntityRetriers[domain.SalesforceIngestEntityAccountPartners] = partners.RetryPartnerRefresh
				}
				// The Contact writer runs under the membership ingest, which is
				// on whenever this job runs, so its retrier needs no extra flag.
				if contact, ok := membershipIngestSvc.(service.ContactReingester); ok {
					retryWorker.EntityRetriers[domain.SalesforceIngestEntityContact] = contact.RetryContactIngest
				}
				ingestRetryWG.Add(1)
				go func() {
					defer ingestRetryWG.Done()
					retryWorker.Run(ingestRetryCtx)
				}()
				log.Printf("salesforce ingest retry worker enabled (every %s)", cfg.SalesforceIngestRetryInterval)
			}
		} else {
			salesforceEventHandler = handler.NewSalesforceEventHandler(withPartnerIngest(withOpportunityIngest(service.NewSalesforceEventService(accountIngestRepo, salesEntityClient, ingestSupport))))
		}
	}

	// POST /users/me/memberships/register (H-0 of the customer onboarding flow).
	// Postgres-only, and off unless CSM_MIGRATION_MEMBERSHIP_REGISTRATION_ENABLED is
	// exactly "true": with the flag off the route is not registered at all, so
	// it 404s and nothing on this path can write to Salesforce. It also needs
	// what it depends on to exist — the SALES_ENTITY_* client for the two
	// PATCHes, and the membership-ingest service to re-ingest each flipped
	// membership — so CSM_MIGRATION_SALESFORCE_MEMBERSHIP_INGEST_ENABLED being off leaves
	// this 404 too, rather than flipping Salesforce with no matching database
	// write.
	var membershipRegistrationHandler *handler.MembershipRegistrationHandler
	if db != nil && cfg.CSMMigrationMembershipRegistrationEnabled && salesEntityClient != nil && membershipIngestSvc != nil {
		membershipRegistrationHandler = handler.NewMembershipRegistrationHandler(service.NewMembershipRegistrationService(
			repository.NewMembershipRegistrationRepository(db),
			salesEntityClient,
			membershipIngestSvc,
			repository.NewOnboardingStepRepository(db),
		))
	}

	// onboarding_step has no ServiceNow equivalent; Postgres-only, like
	// scheduled_task_run above.
	var onboardingStepHandler *handler.OnboardingStepHandler
	if db != nil {
		onboardingStepHandler = handler.NewOnboardingStepHandler(service.NewOnboardingStepService(repository.NewOnboardingStepRepository(db), accessSvc))
	}

	// The portal-driven membership writes. Gated on a pool AND
	// cfg.HasPortalMembershipWrites() -- the flag, a Postgres-authoritative data source
	// and a complete sales-entity-service connection -- because every one of
	// these writes is half a Postgres transaction and half a Salesforce
	// call. Off by default: nil handler means the routes below are
	// never registered, so a portal built against them fails loudly with a
	// 404 rather than writing one system and not the other.
	var projectMembershipHandler *handler.ProjectMembershipHandler
	if db != nil && cfg.HasPortalMembershipWrites() && salesEntityClient != nil {
		projectMembershipHandler = handler.NewProjectMembershipHandler(service.NewProjectMembershipWriteService(service.MembershipWriteDeps{
			Memberships: repository.NewProjectMembershipRepository(repository.NewScoped(db)),
			Steps:       repository.NewOnboardingStepRepository(db),
			SalesEntity: salesEntityClient,
			Publisher:   projectEventPublisher,
			Failures:    eventPublishFailureSvc,
			Access:      accessSvc,
			Invitations: service.NewInvitationValidator(salesEntityClient, repository.NewAccountPartnerRepository(db)),
			Admins:      repository.NewAccountAdminRepository(db),
			UserCache:   userCacheInvalidator,
		}))
	}

	// Allocation-app events into customer engagements; off by default (route not registered).
	var customerEngagementAllocationHandler *handler.CustomerEngagementAllocationHandler
	if db != nil && cfg.HasCustomerEngagementIngest() {
		customerEngagementAllocationHandler = handler.NewCustomerEngagementAllocationHandler(
			service.NewCustomerEngagementAllocationService(
				repository.NewCustomerEngagementAllocationRepository(repository.NewScoped(db)),
				cfg.CustomerEngagementFirefightingTypeID),
			cfg.M2MClientIDs)
	}

	var snAccountHandler *handler.SNAccountHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		snAccountHandler = handler.NewSNAccountHandler(service.NewServiceNowAccountService(serviceNowIntegrationServiceClient))
	}

	accountContactRepo := repository.NewAccountContactRepository(db)
	var activeAccountContactSvc service.AccountContactService
	if cfg.DataSource == config.DataSourceServiceNow {
		activeAccountContactSvc = service.NewServiceNowAccountContactService(serviceNowIntegrationServiceClient)
	} else {
		activeAccountContactSvc = service.NewAccountContactService(accountContactRepo, accessSvc)
	}
	accountContactHandler := handler.NewAccountContactHandler(activeAccountContactSvc)

	// Postgres modes read the Salesforce-ingested sf_* tables in the same shapes.
	var opportunityHandler *handler.OpportunityHandler
	var invoiceHandler *handler.InvoiceHandler
	var projectOpportunityLinkHandler *handler.ProjectOpportunityLinkHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		opportunityHandler = handler.NewOpportunityHandler(service.NewServiceNowOpportunityService(serviceNowIntegrationServiceClient))
		invoiceHandler = handler.NewInvoiceHandler(service.NewServiceNowInvoiceService(serviceNowIntegrationServiceClient))
		projectOpportunityLinkHandler = handler.NewProjectOpportunityLinkHandler(service.NewServiceNowProjectOpportunityLinkService(serviceNowIntegrationServiceClient))
	} else {
		sfReadRepo := repository.NewSalesforceReadRepository(db)
		opportunityHandler = handler.NewOpportunityHandler(service.NewOpportunityService(sfReadRepo, accessSvc))
		invoiceHandler = handler.NewInvoiceHandler(service.NewInvoiceService(sfReadRepo, accessSvc))
		projectOpportunityLinkHandler = handler.NewProjectOpportunityLinkHandler(service.NewProjectOpportunityLinkService(sfReadRepo, accessSvc))
	}

	projectRepo := repository.NewProjectRepository(repository.NewScoped(db))
	pgProjectSvc := service.NewProjectService(projectRepo, accessSvc)
	var activeProjectSvc service.ProjectService
	if cfg.DataSource == config.DataSourceServiceNow {
		activeProjectSvc = service.NewServiceNowProjectService(serviceNowIntegrationServiceClient, pgProjectSvc)
	} else {
		activeProjectSvc = pgProjectSvc
	}
	projectHandler := handler.NewProjectHandler(activeProjectSvc)

	projectContactRepo := repository.NewProjectContactRepository(db)
	var activeProjectContactSvc service.ProjectContactService
	if cfg.DataSource == config.DataSourceServiceNow {
		activeProjectContactSvc = service.NewServiceNowProjectContactService(serviceNowIntegrationServiceClient)
	} else {
		activeProjectContactSvc = service.NewProjectContactService(projectContactRepo, accessSvc)
	}
	projectContactHandler := handler.NewProjectContactHandler(activeProjectContactSvc)

	// activeProjectUpdateSvc backs PATCH /projects/{id} on every data source
	// -- see pgProjectUpdateService's own doc comment for exactly which
	// fields the Postgres data sources accept (a subset of the ServiceNow
	// contract). DATA_SOURCE=postgres-servicenow-dual-write also
	// mirrors a successful write to ServiceNow, asynchronously, via
	// snWritebackDispatcher -- plain DATA_SOURCE=postgres never touches
	// ServiceNow at all (snWritebackDispatcher is nil in that mode, so the
	// nil-check inside NewProjectUpdateServiceWithSNWriteback's caller here
	// never fires for it). projectRepo/pgProjectSvc were already constructed
	// above for GetProject/SearchProjects.
	var activeProjectUpdateSvc service.ProjectUpdateService
	switch cfg.DataSource {
	case config.DataSourceServiceNow:
		activeProjectUpdateSvc = service.NewServiceNowProjectUpdateService(serviceNowIntegrationServiceClient)
	case config.DataSourcePostgresServiceNowDualWrite:
		snProjectMirrorSvc := service.NewServiceNowProjectUpdateService(serviceNowIntegrationServiceClient)
		activeProjectUpdateSvc = service.NewProjectUpdateServiceWithSNWriteback(projectRepo, userRepo, accessSvc, snWritebackDispatcher, snProjectMirrorSvc)
	default:
		activeProjectUpdateSvc = service.NewProjectUpdateService(projectRepo, userRepo, accessSvc)
	}
	projectUpdateHandler := handler.NewProjectUpdateHandler(activeProjectUpdateSvc)

	// referenceDataRepo backs GET /projects/{id}/metadata and GET /metadata's
	// Postgres-mode choice lists (project_type rows, enum labels) -- see
	// ReferenceDataRepository's own doc comment.
	referenceDataRepo := repository.NewReferenceDataRepository(db)

	// sla-duration-policy reads the small, static sla_duration_policy
	// reference table (migration 0192) -- gated on db != nil like
	// slaStatusHandler above, since there's nothing to read on a deployment
	// with no Postgres pool at all. integrations/csm-notification-service
	// fetches this once at startup to compute its own SLA due dates.
	var slaDurationPolicyHandler *handler.SLADurationPolicyHandler
	if db != nil {
		slaDurationPolicyHandler = handler.NewSLADurationPolicyHandler(
			service.NewSLADurationPolicyService(referenceDataRepo, accessSvc))
	}

	// Every project-stats route is available on both data sources. In
	// ServiceNow mode one client-backed value satisfies all three interfaces
	// structurally, so it is built once and shared; in Postgres mode the
	// narrower metadata and case-stats services are built first and composed
	// into the full ProjectStatsService, which delegates those two methods to
	// them rather than reimplementing either.
	//
	// ProjectMetadataService and ProjectCaseStatsService remain separate
	// interfaces, and keep their own handlers, because each was portable to
	// Postgres before the rest of the bundle was.
	var (
		projectMetadataSvc  service.ProjectMetadataService
		projectCaseStatsSvc service.ProjectCaseStatsService
		projectStatsSvc     service.ProjectStatsService
	)
	if cfg.DataSource == config.DataSourceServiceNow {
		snProjectStatsSvc := service.NewServiceNowProjectStatsService(serviceNowIntegrationServiceClient)
		projectMetadataSvc, projectCaseStatsSvc, projectStatsSvc = snProjectStatsSvc, snProjectStatsSvc, snProjectStatsSvc
	} else {
		projectMetadataSvc = service.NewProjectMetadataService(referenceDataRepo)
		// accessSvc is passed in so a by-id stats read is scoped to what the
		// caller may see. Unlike the scoped list endpoints, which fold the
		// scope into their WHERE clause, the project id here comes from the
		// path and needs an explicit check.
		projectCaseStatsSvc = service.NewProjectCaseStatsService(
			repository.NewProjectCaseStatsRepository(repository.NewScoped(db)), referenceDataRepo, accessSvc)
		projectStatsSvc = service.NewProjectStatsService(
			repository.NewProjectStatsRepository(repository.NewScoped(db), crVisibility), referenceDataRepo, accessSvc,
			projectMetadataSvc, projectCaseStatsSvc)
	}
	projectMetadataHandler := handler.NewProjectMetadataHandler(projectMetadataSvc)
	projectCaseStatsHandler := handler.NewProjectCaseStatsHandler(projectCaseStatsSvc)
	projectStatsHandler := handler.NewProjectStatsHandler(projectStatsSvc)

	productRepo := repository.NewProductRepository(db)
	productSvc := service.NewProductService(productRepo)
	productHandler := handler.NewProductHandler(productSvc)

	var snProductHandler *handler.SNProductHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		snProductHandler = handler.NewSNProductHandler(service.NewServiceNowProductService(serviceNowIntegrationServiceClient))
	}

	var productVersionHandler *handler.ProductVersionHandler
	var snProductVersionHandler *handler.SNProductVersionHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		snProductVersionHandler = handler.NewSNProductVersionHandler(service.NewServiceNowProductVersionService(serviceNowIntegrationServiceClient))
	} else {
		productVersionRepo := repository.NewProductVersionRepository(db)
		productVersionSvc := service.NewProductVersionService(productVersionRepo)
		productVersionHandler = handler.NewProductVersionHandler(productVersionSvc)
	}

	deploymentRepo := repository.NewDeploymentRepository(repository.NewScoped(db))
	var activeDeploymentSvc service.DeploymentService
	switch cfg.DataSource {
	case config.DataSourceServiceNow:
		activeDeploymentSvc = service.NewServiceNowDeploymentService(serviceNowIntegrationServiceClient)
	case config.DataSourcePostgresServiceNowDualWrite:
		// CreateDeployment is ServiceNow-first and synchronous; UpdateDeployment
		// is Postgres-first with an asynchronous ServiceNow mirror -- see
		// deploymentService.createDeploymentSNFirst/UpdateDeployment's own doc
		// comments for the full reasoning (the same CREATE-vs-UPDATE asymmetry
		// as caseService).
		snDeploymentMirrorSvc := service.NewServiceNowDeploymentService(serviceNowIntegrationServiceClient)
		activeDeploymentSvc = service.NewDeploymentServiceWithSNWriteback(deploymentRepo, snWritebackDispatcher, snDeploymentMirrorSvc)
	default:
		activeDeploymentSvc = service.NewDeploymentService(deploymentRepo)
	}
	deploymentHandler := handler.NewDeploymentHandler(activeDeploymentSvc)

	// KB features are Postgres-only, like event_publish_failures above --
	// db may be nil under DATA_SOURCE=servicenow, and a nil pool panics on
	// first query rather than at construction (CodeRabbit finding). Gate
	// the whole chain on it: nil handlers mean the KB routes below are
	// never registered when there's no database to serve them.
	var kbArticleHandler *handler.KBArticleHandler
	var kbManagerUserHandler *handler.KBManagerUserHandler
	var kbManagerGroupHandler *handler.KBManagerGroupHandler
	var knowledgeBaseHandler *handler.KnowledgeBaseHandler
	if db != nil {
		kbArticleRepo := repository.NewKBArticleRepository(db)
		kbArticleSvc := service.NewKBArticleService(kbArticleRepo, eventPublisher)
		kbArticleHandler = handler.NewKBArticleHandler(kbArticleSvc)
		kbManagerUserRepo := repository.NewKBManagerUserRepository(db)
		kbManagerUserSvc := service.NewKBManagerUserService(kbManagerUserRepo)
		kbManagerUserHandler = handler.NewKBManagerUserHandler(kbManagerUserSvc)
		kbManagerGroupRepo := repository.NewKBManagerGroupRepository(db)
		kbManagerGroupSvc := service.NewKBManagerGroupService(kbManagerGroupRepo)
		kbManagerGroupHandler = handler.NewKBManagerGroupHandler(kbManagerGroupSvc)

		knowledgeBaseRepo := repository.NewKnowledgeBaseRepository(db)
		knowledgeBaseSvc := service.NewKnowledgeBaseService(knowledgeBaseRepo)
		knowledgeBaseHandler = handler.NewKnowledgeBaseHandler(knowledgeBaseSvc)
	}

	deployedProductRepo := repository.NewDeployedProductRepository(repository.NewScoped(db))
	var activeDeployedProductSvc service.DeployedProductService
	switch cfg.DataSource {
	case config.DataSourceServiceNow:
		activeDeployedProductSvc = service.NewServiceNowDeployedProductService(serviceNowIntegrationServiceClient, activeDeploymentSvc, activeProjectSvc)
	case config.DataSourcePostgresServiceNowDualWrite:
		// CreateDeployedProduct is ServiceNow-first and synchronous;
		// UpdateDeployedProduct is Postgres-first with an asynchronous
		// ServiceNow mirror -- see
		// deployedProductService.createDeployedProductSNFirst/UpdateDeployedProduct's
		// own doc comments for the full reasoning (the same CREATE-vs-UPDATE
		// asymmetry as deploymentService/caseService).
		snDeployedProductMirrorSvc := service.NewServiceNowDeployedProductService(serviceNowIntegrationServiceClient, activeDeploymentSvc, activeProjectSvc)
		activeDeployedProductSvc = service.NewDeployedProductServiceWithSNWriteback(deployedProductRepo, snWritebackDispatcher, snDeployedProductMirrorSvc)
	default:
		activeDeployedProductSvc = service.NewDeployedProductService(deployedProductRepo)
	}
	deployedProductHandler := handler.NewDeployedProductHandler(activeDeployedProductSvc)

	// Constructed here (rather than down near snUserHandler below) so
	// NewServiceNowCaseService can also take it — see that constructor's
	// own doc comment for what it uses it for (a direct, in-process role
	// lookup backing applyResponseSLAOnComment, not routed through HTTP).
	// Also constructed for DataSourcePostgresServiceNowDualWrite, for the same
	// reason serviceNowIntegrationServiceClient above is: the case pilot's
	// SN-mirror snCaseService instance below needs it too.
	var snUserService service.SNUserService
	if cfg.DataSource == config.DataSourceServiceNow || cfg.DataSource == config.DataSourcePostgresServiceNowDualWrite {
		snUserService = service.NewServiceNowUserService(serviceNowIntegrationServiceClient)
	}

	// The CSM-native SLA engine (internal/service/sla_engine_service.go)
	// writes its own source='CSM' rows into the "sla" table (migration 0134),
	// resolving durations from the deterministic, severity-keyed sla_policy
	// rows migration 0203 seeds -- gated on db the same way slaStatusHandler
	// above is: nowhere to store a clock at all with no database configured.
	var slaEngineSvc service.SLAEngineService
	if db != nil {
		slaEngineSvc = service.NewSLAEngineService(repository.NewSLAEngineRepository(repository.NewScoped(db)))
	}

	caseRepo := repository.NewCaseRepository(repository.NewScoped(db), crVisibility)
	var activeCaseSvc service.CaseService
	// caseAttachmentOverrideSvc, when non-nil, is the CaseService case
	// attachment routes (registered further below) use INSTEAD of
	// activeCaseSvc -- see its assignment in the DataSourcePostgresServiceNowDualWrite
	// case for why. nil in every other mode: attachments follow activeCaseSvc
	// exactly as before this override existed.
	var caseAttachmentOverrideSvc service.CaseService
	switch cfg.DataSource {
	case config.DataSourceServiceNow:
		pgCaseFallbackSvc := service.NewCaseService(caseRepo, userRepo, eventPublisher, accessSvc, projectContactRepo)
		activeCaseSvc = service.NewServiceNowCaseService(serviceNowIntegrationServiceClient, pgCaseFallbackSvc, eventPublisher, snUserService, cfg.CustomerRoles, cfg.CSEngineerRole, slaEngineSvc)
	case config.DataSourcePostgresServiceNowDualWrite:
		// Pilot: case CREATE, and UPDATE's WorkState field only.
		//
		// CREATE is ServiceNow-first and synchronous — see
		// caseService.createCaseSNFirst's own doc comment for the full
		// reasoning (a Postgres-first async create could leave a permanent
		// orphan: a Postgres row with no ServiceNow counterpart). This is
		// also what finally makes case creation work on Postgres in this
		// mode at all: CaseRepository.CreateCase's own doc comment explains
		// why Postgres can't generate work_item.number/wso2_id itself (no
		// sequence was ever added); CreateCaseFromServiceNow sidesteps that
		// by using the identity ServiceNow already generated, rather than
		// answering the still-unresolved question of what a Postgres-native
		// case number would even look like. The plain (non-fallback)
		// CreateCase path above (DataSourceServiceNow's pgCaseFallbackSvc,
		// and DataSourcePostgres/default below) is UNCHANGED and still
		// deliberately non-functional — this only unblocks the fallback
		// mode's own path.
		//
		// UPDATE mirrors State/Severity/WorkState, asynchronously, after
		// Postgres — see caseService.UpdateCase's own doc comment for
		// exactly what this mirrors and why. State/Severity joined the
		// mirror later than WorkState did, once patchCaseFields
		// (sn_case_service.go) existed: a bare PATCH with none of
		// snCaseService.UpdateCase's own read-before-write behavior (that
		// method still does a live GetCaseByID before PATCHing State/
		// Severity, which this mode must never do — patchCaseFields is a
		// separate, additional method precisely so UpdateCase itself stays
		// unchanged for live DataSource=servicenow traffic).
		//
		// CreateCaseComment mirrors the comment's content, asynchronously,
		// after Postgres — see that method's own doc comment. It uses
		// CreateBareCaseComment (sn_case_service.go), not the full
		// CreateCaseComment, for the same reason patchCaseFields exists:
		// Postgres already decided the real outcome, so ServiceNow's own
		// state-transition/event side effects must not re-run.
		//
		// snCaseMirrorSvc is a full snCaseService, exactly as constructed
		// for DataSourceServiceNow above, but it is never made the active
		// CaseService — reads always stay on Postgres in this mode. It
		// serves four purposes: CreateCase calls its CreateCase directly and
		// synchronously; UpdateCase dispatches to its patchCaseFields (via
		// the snFieldPatcher interface) through snWritebackDispatcher, asynchronously;
		// CreateCaseComment dispatches to its CreateBareCaseComment (via the
		// snCommentMirror interface) through snWritebackDispatcher, asynchronously;
		// and it is caseAttachmentOverrideSvc below, for case attachments
		// specifically.
		// snCaseMirrorSvc's own slaEngine is deliberately nil here, mirroring
		// its nil publisher just above: SLA clock registration must not run
		// as a side effect of this synchronous, SN-first CreateCase call —
		// its own Postgres work_item row doesn't exist yet at this point
		// (created in caseService.createCaseSNFirst right after this call
		// returns), and "sla".work_item_id has a hard foreign key against
		// it. caseService itself registers the clocks, via its own
		// slaEngine below, once that insert has actually succeeded — see
		// registerCaseSLAClocksEvent's own doc comment for the bug this
		// fixed (every dual-write case creation silently failed clock
		// registration with a foreign-key violation).
		snCaseMirrorSvc := service.NewServiceNowCaseService(serviceNowIntegrationServiceClient, nil, nil, snUserService, cfg.CustomerRoles, cfg.CSEngineerRole, nil)
		activeCaseSvc = service.NewCaseServiceWithSNWriteback(caseRepo, userRepo, eventPublisher, accessSvc, projectContactRepo, snWritebackDispatcher, snCaseMirrorSvc, slaEngineSvc, cfg.CSEngineerRole)
		// CreateCase rejects a case/SR whose deployed product's category
		// doesn't satisfy its project type's own Default Case/SR Creation
		// Product Category allow-list — see
		// caseService.validateDeployedProductCategoryForType's own doc
		// comment. Wired here (and in the plain-Postgres default branch
		// below), not for DataSourceServiceNow: that mode's CreateCase never
		// reaches this service's Postgres data at all.
		activeCaseSvc = service.WithProductCategoryEnforcement(activeCaseSvc, referenceDataRepo, deployedProductRepo)
		// PATCH /cases/{id} with a "type" (the CSM portal's "Change case type"):
		// moves the case between extension tables, asking ServiceNow first from
		// inside the same Postgres transaction -- see caseService.transferCaseType.
		// Without this the request is refused ("only supported for the ServiceNow
		// data source"), which is how a type change failed for every case here.
		activeCaseSvc = service.WithCaseTypeTransfer(activeCaseSvc, caseRepo)
		// Case ATTACHMENTS: file bytes still live ONLY in ServiceNow in this
		// mode (SFTPGo, the CSM-native/Postgres storage backend, is never
		// used here) — but metadata is now ALSO written into and read from
		// Postgres, matching this codebase's "reads come from Postgres,
		// writes go to both" policy for every other entity. This replaces
		// the previous permanent "attachments are ServiceNow-only" override:
		// that reasoning (the sftpgo-backed Postgres attachment
		// implementation wasn't production-ready for the Oct 4 go-live) is
		// now obsolete — case_attachment.storage_key is nullable (migration
		// 0185) precisely so a ServiceNow-sourced row can have metadata-only
		// Postgres rows alongside the existing SFTPGo-backed ones.
		//
		// service.NewCaseAttachmentDualWriteService wraps activeCaseSvc
		// (itself a *caseService, reused for its Postgres-backed
		// SearchCaseAttachments/GetAttachmentByID and its repo/resolveActor)
		// and overrides CreateCaseAttachment (ServiceNow-first/synchronous,
		// then a Postgres metadata insert via
		// CaseRepository.CreateCaseAttachmentFromServiceNow),
		// ConfirmCaseAttachment (still 503s — there is no pending/
		// in-progress upload state in this mode, same as plain
		// DataSource=servicenow), GetCaseAttachmentContent (bytes only exist
		// in ServiceNow, forwarded to snCaseMirrorSvc), and
		// DeleteCaseAttachment/UpdateAttachment (Postgres-first with a
		// best-effort, asynchronous ServiceNow mirror via
		// snWritebackDispatcher, the same shape as every other dual-write
		// UPDATE/DELETE). SearchCaseAttachments/GetAttachmentByID are
		// inherited unmodified — see that type's own doc comment
		// (case_attachment_dual_write_service.go) for the full design.
		caseAttachmentOverrideSvc = service.NewCaseAttachmentDualWriteService(activeCaseSvc, snCaseMirrorSvc, snWritebackDispatcher)
	default:
		activeCaseSvc = service.NewCaseService(caseRepo, userRepo, eventPublisher, accessSvc, projectContactRepo)
		// See the matching comment in the DataSourcePostgresServiceNowDualWrite
		// case above.
		activeCaseSvc = service.WithProductCategoryEnforcement(activeCaseSvc, referenceDataRepo, deployedProductRepo)
		// Same transfer as in the dual-write branch above, with no ServiceNow step.
		activeCaseSvc = service.WithCaseTypeTransfer(activeCaseSvc, caseRepo)
		// Without this, isSupportEngineerAuthor always returns false on this
		// data source -- the SLA response-clock completion signal and
		// events.CommentAddedPayload.IsSupportEngineerResponse never fire for
		// a plain DATA_SOURCE=postgres deployment. NewCaseServiceWithSNWriteback
		// (above) already takes csEngineerRole as a constructor parameter;
		// NewCaseService has no such parameter, hence this separate step.
		activeCaseSvc = service.WithCSEngineerRole(activeCaseSvc, cfg.CSEngineerRole)
	}
	// Service requests: ServiceNow's "SR New Request - Acknowledge & Chat
	// Alert" flow and the sr.* events, on the operations topic (sre-events)
	// with the change-request and outage notices. Postgres-backed case
	// services only -- WithSRNotices is a no-op on the ServiceNow one -- and
	// only with a topic to publish to: without the Chat cards the flow's
	// acknowledgement would be posted with nothing announcing the SR, which
	// ServiceNow never does. SR_ALERT_SRE_TEAM_IDS then decides which teams'
	// SRs are assigned and acknowledged.
	var srEventPublisher service.EventPublisherService
	if db != nil && cfg.SREEventHubTopic != "" && cfg.EventHubBroker != "" && cfg.EventPublishingEnabled {
		srEventPublisher = service.NewEventPublisherService(
			eventbus.NewProducer(eventbus.Config{
				Broker:           cfg.EventHubBroker,
				ConnectionString: cfg.EventHubConnectionString,
				Topic:            cfg.SREEventHubTopic,
			}),
			eventPublishFailureSvc,
		)
		srNotices := service.NewSRNoticeService(
			repository.NewSRNoticeRepository(repository.NewScoped(db)), srEventPublisher, cfg.SRAlertSRETeamIDs)
		activeCaseSvc = service.WithSRNotices(activeCaseSvc, srNotices)
		// An SR created from a GitHub issue gets the same automation and card.
		// Set in place, so the GitHub handlers built above already have it.
		if githubSync != nil {
			service.WithGithubSRNotices(githubSync, srNotices)
		}
		slog.Info("service request events enabled", "topic", cfg.SREEventHubTopic, "automatedTeams", len(cfg.SRAlertSRETeamIDs))
	}
	caseHandler := handler.NewCaseHandler(activeCaseSvc, cfg.M2MClientIDs)
	if db != nil {
		announcementRequestHandler = handler.NewAnnouncementRequestHandler(
			service.NewAnnouncementRequestService(repository.NewAnnouncementRequestRepository(db), activeCaseSvc, accessSvc),
		)
		scheduleHandler = handler.NewScheduleHandler(
			service.NewScheduleService(repository.NewScheduleRepository(db), accessSvc),
		)
		rotaGenerateHandler = handler.NewRotaGenerateHandler(
			service.NewRotaGenerateService(repository.NewScheduleRepository(db), repository.NewRotaGenerateRepository(db), accessSvc),
		)
		// Postgres-only, and internal-staff-only like every other read in this
		// module: it returns staff names, addresses and rank.
		teamMemberHandler = handler.NewTeamMemberHandler(
			service.NewTeamMemberService(repository.NewTeamMemberRepository(db), accessSvc),
		)
	}
	// activeAttachmentSvc backs the case-attachment routes registered below
	// (POST/GET/PATCH/DELETE /attachments...) — see caseAttachmentOverrideSvc's
	// own doc comment above for when and why it differs from activeCaseSvc.
	activeAttachmentSvc := activeCaseSvc
	if caseAttachmentOverrideSvc != nil {
		activeAttachmentSvc = caseAttachmentOverrideSvc
	}
	attachmentHandler := handler.NewCaseHandler(activeAttachmentSvc, cfg.M2MClientIDs)

	// customer_call (migration 0073) backs call requests on the Postgres
	// data source, so these routes are registered for both data sources.
	callRequestRepo := repository.NewCallRequestRepository(repository.NewScoped(db))
	var activeCallRequestSvc service.CallRequestService
	switch cfg.DataSource {
	case config.DataSourceServiceNow:
		activeCallRequestSvc = service.NewServiceNowCallRequestService(serviceNowIntegrationServiceClient)
	case config.DataSourcePostgresServiceNowDualWrite:
		// CreateCallRequest mirrors to ServiceNow, asynchronously, after
		// Postgres -- see callRequestService's own doc comment for why
		// UpdateCallRequest does not (Postgres-first CREATE means
		// customer_call.id has no ServiceNow counterpart to target).
		snCallRequestMirrorSvc := service.NewServiceNowCallRequestService(serviceNowIntegrationServiceClient)
		activeCallRequestSvc = service.NewCallRequestServiceWithSNWriteback(callRequestRepo, userRepo, snWritebackDispatcher, snCallRequestMirrorSvc)
	default:
		activeCallRequestSvc = service.NewCallRequestService(callRequestRepo, userRepo)
	}
	callRequestHandler := handler.NewCallRequestHandler(activeCallRequestSvc)

	// The native implementation when the GitHub integration is enabled,
	// otherwise the ServiceNow proxy. Both are kept: cutover is per account,
	// and an account still on ServiceNow must keep filing issues the old way.
	var caseGithubIssueHandler *handler.CaseGithubIssueHandler
	switch {
	case githubClient != nil:
		// Native: files the issue against GitHub and writes the issue number
		// onto the case, which is what opens the outbound gate for it.
		caseGithubIssueHandler = handler.NewCaseGithubIssueHandler(
			service.NewCaseGithubIssueService(githubClient, githubSyncRepo, activeCaseSvc, githubLabelSet))
	case cfg.DataSource == config.DataSourceServiceNow:
		caseGithubIssueHandler = handler.NewCaseGithubIssueHandler(service.NewServiceNowCaseGithubIssueService(serviceNowIntegrationServiceClient, activeCaseSvc))
	}

	// case_escalation/case_escalation_notification_list (migration 0054)
	// now back both SearchEscalations and CreateEscalation on Postgres for
	// real -- see EscalationRepository.CreateEscalation's own doc comment
	// for the level-transition/notification-recipient rule (a port of
	// ServiceNow's EscalationNotificationUtils). This supersedes an earlier unconditional
	// unavailableCaseEscalationService stand-in that predated the schema.
	escalationNotifyCfg := repository.EscalationNotificationConfig{
		EL1AmericasTLEmails:           cfg.EscalationEL1AmericasTLEmails,
		EL2AmericasTUEmails:           cfg.EscalationEL2AmericasTUEmails,
		EL2ProductServiceEmail:        cfg.EscalationEL2ProductServiceEmail,
		EL2ProductIdentityServerEmail: cfg.EscalationEL2ProductIdentityServerEmail,
		EL2ProductDefaultEmail:        cfg.EscalationEL2ProductDefaultEmail,
	}
	escalationRepo := repository.NewEscalationRepository(repository.NewScoped(db), escalationNotifyCfg)
	var activeEscalationSvc service.EscalationService
	switch cfg.DataSource {
	case config.DataSourceServiceNow:
		activeEscalationSvc = service.NewServiceNowEscalationService(serviceNowIntegrationServiceClient)
	case config.DataSourcePostgresServiceNowDualWrite:
		// CreateEscalation mirrors to ServiceNow asynchronously, best-effort,
		// via the shared snWritebackDispatcher -- see
		// escalationService.CreateEscalation's own doc comment.
		snEscalationMirrorSvc := service.NewServiceNowEscalationService(serviceNowIntegrationServiceClient)
		activeEscalationSvc = service.NewEscalationServiceWithSNWriteback(escalationRepo, userRepo, caseRepo, accessSvc, snWritebackDispatcher, snEscalationMirrorSvc)
	default:
		activeEscalationSvc = service.NewEscalationService(escalationRepo, userRepo, caseRepo, accessSvc)
	}
	// case.escalated is what emails an escalation's notification list -- on
	// for DATA_SOURCE=postgres and dual-write alike, off with
	// CASE_ESCALATION_NOTICES_ENABLED=false. Under dual-write ServiceNow's
	// "Internal Escalation notification" flow (record create on
	// sn_customerservice_case_escalation, discovery scripts 79/80) would mail
	// the mirrored row too; it is switched off where SN delivers mail. See
	// Config.CaseEscalationNoticesOn.
	if cfg.CaseEscalationNoticesOn() {
		activeEscalationSvc = service.WithEscalationNotices(activeEscalationSvc, eventPublisher)
	}
	escalationHandler := handler.NewEscalationHandler(activeEscalationSvc)
	caseEscalationSvc := service.NewCaseEscalationService(activeEscalationSvc, activeCaseSvc)
	if db != nil {
		// The ABT team leads (who may de-escalate) live in Postgres team data
		// whatever DATA_SOURCE is.
		caseEscalationSvc = service.WithCaseTeamLeads(caseEscalationSvc, escalationRepo)
	}
	caseEscalationHandler := handler.NewCaseEscalationHandler(caseEscalationSvc)

	changeRequestRepo := repository.NewChangeRequestRepository(repository.NewScoped(db), crVisibility)
	var activeChangeRequestSvc service.ChangeRequestService
	switch cfg.DataSource {
	case config.DataSourceServiceNow:
		activeChangeRequestSvc = service.NewServiceNowChangeRequestService(serviceNowIntegrationServiceClient)
	case config.DataSourcePostgresServiceNowDualWrite:
		// Pilot extension: change request CREATE (ServiceNow-first,
		// synchronous -- see changeRequestService.createChangeRequestSNFirst's
		// own doc comment), PatchChangeRequest's best-effort asynchronous
		// mirror write to the previous system, DecideChangeRequestApproval's
		// best-effort asynchronous mirror write, and the mirror of every
		// state move either commits (see those methods' own doc comments and
		// changeRequestStateMirror). Reads (GetChangeRequest,
		// GetChangeRequestApprovals) stay on Postgres in this mode;
		// snChangeRequestMirrorSvc (built above, shared with the GitHub
		// sync's state mirror) is never the active service.
		activeChangeRequestSvc = service.NewChangeRequestServiceWithSNWriteback(changeRequestRepo, userRepo, snChangeRequestMirrorSvc, snWritebackDispatcher)
		// Create checks the assignee and requester against ServiceNow first, which
		// refuses a user it has never heard of with a bare 404.
		activeChangeRequestSvc = service.WithChangeRequestSNUserLookup(activeChangeRequestSvc, snUserService)
	default:
		activeChangeRequestSvc = service.NewChangeRequestService(changeRequestRepo, userRepo)
	}
	changeRequestHandler := handler.NewChangeRequestHandler(activeChangeRequestSvc)

	timeCardRepo := repository.NewTimeCardRepository(repository.NewScoped(db))
	var activeTimeCardSvc service.TimeCardService
	switch cfg.DataSource {
	case config.DataSourceServiceNow:
		activeTimeCardSvc = service.NewServiceNowTimeCardService(serviceNowIntegrationServiceClient)
	case config.DataSourcePostgresServiceNowDualWrite:
		// CreateTimeCard mirrors to ServiceNow, asynchronously, after
		// Postgres -- see timeCardService's own doc comment for why
		// Update/DeleteTimeCard do not (Postgres-first CREATE means
		// time_card.id has no ServiceNow counterpart to target).
		snTimeCardMirrorSvc := service.NewServiceNowTimeCardService(serviceNowIntegrationServiceClient)
		activeTimeCardSvc = service.NewTimeCardServiceWithSNWriteback(timeCardRepo, userRepo, snWritebackDispatcher, snTimeCardMirrorSvc)
	default:
		activeTimeCardSvc = service.NewTimeCardService(timeCardRepo, userRepo)
	}
	timeCardHandler := handler.NewTimeCardHandler(activeTimeCardSvc)

	// sr_category/catalog_item/catalog_item_category/catalog_variable/
	// sr_category_routing_rule (migrations 000067-000071) back the service
	// request catalog on the Postgres data source, so these routes are
	// registered for both data sources. Under dual-write, SearchCatalogs
	// still reads from ServiceNow -- see catalogService.snMirror's own doc
	// comment for why (deployed_product/routing-rule data was never
	// backfilled from ServiceNow, same gap as deployments/deployed-products/
	// instances). GetCatalogItemVariables is the exception: catalog_variable's
	// extra fields and its catalog_variable_choice table (migration 0125) are
	// kept current by a separate sync service, so that one read goes to
	// Postgres under dual-write too -- see catalogService.GetCatalogItemVariables'
	// own doc comment.
	catalogRepo := repository.NewCatalogRepository(repository.NewScoped(db))
	var activeCatalogSvc service.CatalogService
	switch cfg.DataSource {
	case config.DataSourceServiceNow:
		activeCatalogSvc = service.NewServiceNowCatalogService(serviceNowIntegrationServiceClient)
	case config.DataSourcePostgresServiceNowDualWrite:
		snCatalogMirrorSvc := service.NewServiceNowCatalogService(serviceNowIntegrationServiceClient)
		activeCatalogSvc = service.NewCatalogServiceWithSNFallback(catalogRepo, snCatalogMirrorSvc)
	default:
		activeCatalogSvc = service.NewCatalogService(catalogRepo)
	}
	catalogHandler := handler.NewCatalogHandler(activeCatalogSvc)
	// After the switch above: a service request raised from the catalog form
	// takes its subject and description from its answers. activeCaseSvc is the
	// same *caseService the case handler already holds, so this reaches it.
	activeCaseSvc = service.WithServiceRequestCatalog(activeCaseSvc, activeCatalogSvc)

	// Case feedback (CSAT submissions): the ServiceNow data source reads it
	// from the backing system; both Postgres data sources read
	// work_item_feedback (migration 0102). Dual write deliberately does NOT
	// read from the backing system -- like every other read in that mode it
	// stays on Postgres -- and the CSM side never writes feedback, so there is
	// no writeback wrapper here. Routes are registered for every data source;
	// NewUnavailableFeedbackService remains only as the documented-503
	// fallback for a source with no feedback store.
	var activeFeedbackSvc service.FeedbackService
	switch cfg.DataSource {
	case config.DataSourceServiceNow:
		activeFeedbackSvc = service.NewServiceNowFeedbackService(serviceNowIntegrationServiceClient)
	case config.DataSourcePostgres, config.DataSourcePostgresServiceNowDualWrite:
		activeFeedbackSvc = service.NewPostgresFeedbackService(repository.NewFeedbackRepository(repository.NewScoped(db)))
	default:
		activeFeedbackSvc = service.NewUnavailableFeedbackService()
	}
	feedbackHandler := handler.NewFeedbackHandler(activeFeedbackSvc)

	productVulnerabilityRepo := repository.NewProductVulnerabilityRepository(db)
	var activeProductVulnerabilitySvc service.ProductVulnerabilityService
	if cfg.DataSource == config.DataSourceServiceNow {
		activeProductVulnerabilitySvc = service.NewServiceNowProductVulnerabilityService(serviceNowIntegrationServiceClient)
	} else {
		activeProductVulnerabilitySvc = service.NewProductVulnerabilityService(productVulnerabilityRepo)
	}
	productVulnerabilityHandler := handler.NewProductVulnerabilityHandler(activeProductVulnerabilitySvc)

	// incident/problem/incident_task/conversation (migrations 000057-000060,
	// 000066) -- see incident_repo.go/problem_repo.go's own doc comments for
	// what's implemented (reads) vs. still ServiceUnavailableError (writes
	// needing work_item.number generation or undiscoverable business rules).
	incidentRepo := repository.NewIncidentRepository(repository.NewScoped(db))
	var activeIncidentSvc service.IncidentService
	switch cfg.DataSource {
	case config.DataSourceServiceNow:
		activeIncidentSvc = service.NewServiceNowIncidentService(serviceNowIntegrationServiceClient, incidentEventPublisher)
	case config.DataSourcePostgresServiceNowDualWrite:
		// Pilot extension: incident CREATE only, same ServiceNow-first,
		// synchronous shape as the case pilot above -- see
		// incidentService.createIncidentSNFirst's own doc comment. Reads
		// stay on Postgres in this mode; snIncidentMirrorSvc's CreateIncident
		// is the only method of it this mode ever calls.
		//
		// eventPublisher is passed through here (unlike snCaseMirrorSvc's nil
		// publisher/access args above, which are inert for case because
		// caseService's own CreateCase response building doesn't need them).
		// The mirror is built with publisher=nil deliberately (unlike a
		// plain DataSourceServiceNow instance) -- its own automatic publish
		// fires right after the ServiceNow POST returns, before the
		// Postgres insert this mode's reads depend on has even been
		// attempted, which is exactly the premature-event bug CodeRabbit
		// flagged on PR #1922. incident.created is instead published by
		// createIncidentSNFirst itself, after that Postgres insert
		// succeeds -- see NewIncidentServiceWithSNMirror's own doc comment
		// and publishIncidentCreatedEvent's.
		//
		// snWritebackDispatcher (the single shared instance constructed once
		// above) is reused as-is for incident UPDATE's async ServiceNow
		// mirror -- a *SNWritebackDispatcher is just a fixed background
		// worker pool plus one sn_writeback_failures repository, nothing
		// case-specific about it, so a second instance would only mean a
		// second, redundant worker pool.
		snIncidentMirrorSvc := service.NewServiceNowIncidentMirrorService(serviceNowIntegrationServiceClient)
		activeIncidentSvc = service.NewIncidentServiceWithSNMirror(incidentRepo, userRepo, snIncidentMirrorSvc, incidentEventPublisher, snWritebackDispatcher)
	default:
		// DATA_SOURCE=postgres: the platform creates incidents itself, so it publishes incident.created
		// (what the call-escalation ladders start from) and takes work notes, with no ServiceNow behind it.
		activeIncidentSvc = service.NewIncidentServiceWithPublisher(incidentRepo, userRepo, incidentEventPublisher)
	}
	// Which Special Ops team a handoff goes to, and which GitHub repository
	// its internal issue goes to, is configuration. A value that does not
	// parse would silently offer no handoff anywhere; refuse to start instead.
	handoffConfig, handoffErr := service.ParseSpecialistHandoffConfig(cfg.SpecialistHandoffConfig)
	if handoffErr != nil {
		log.Fatalf("invalid specialist handoff configuration: %v", handoffErr)
	}
	activeIncidentSvc = service.WithSpecialistHandoffConfig(activeIncidentSvc, handoffConfig)
	if db != nil {
		// Case Paging: who is on each tier of a team's paging chain, picked on
		// the Team Schedule's Case Paging tab, and whether each chain would
		// reach someone over the next days. Postgres-only, like the
		// memberships and the rota it reads. Built here, after the handoff
		// routing, because the readiness check reads which SME team each
		// handoff dialog team pages.
		pagingChainHandler = handler.NewPagingChainHandler(
			// Test calls go out on the main shared topic (eventPublisher), not
			// the incident one: a test call belongs to no incident.
			service.NewPagingChainService(repository.NewPagingChainRepository(db), repository.NewPagingContactRepository(db),
				accessSvc, handoffConfig, eventPublisher),
		)
	}
	// One GitHub client per credential the products name, independent of the
	// change-request sync. A credential with no token is logged, not fatal:
	// its handoffs still go through and report that no issue was filed.
	handoffToken, tokenErr := service.ParseSpecialistHandoffGithubTokens(cfg.SpecialistHandoffGithubTokens, cfg.GithubToken)
	if tokenErr != nil {
		log.Fatalf("invalid specialist handoff GitHub tokens: %v", tokenErr)
	}
	handoffIssueClients := service.SpecialistHandoffIssueClients{}
	for _, credential := range handoffConfig.Credentials() {
		token := handoffToken(credential)
		if token == "" {
			slog.Warn("specialist handoff: no GitHub token for credential; its handoffs will file no issue", "credential", credential)
			continue
		}
		handoffIssueClients[credential] = github.NewClient(github.Config{BaseURL: cfg.GithubBaseURL, Token: token})
	}
	activeIncidentSvc = service.WithHandoffIssueCreators(activeIncidentSvc, handoffIssueClients)
	// The team an incident gets when its service has no support group
	// (resolveAssignmentGroup). Checked once in the background: a missing or
	// groupless default is logged, never fatal -- the create still works, it
	// just leaves such incidents unassigned.
	activeIncidentSvc = service.WithIncidentDefaultService(activeIncidentSvc, cfg.IncidentDefaultServiceID)
	if cfg.IncidentDefaultServiceID != "" && (db != nil || cfg.DataSource == config.DataSourceServiceNow) {
		go service.CheckIncidentDefaultService(context.Background(), activeIncidentSvc, cfg.IncidentDefaultServiceID)
	}
	incidentHandler := handler.NewIncidentHandler(activeIncidentSvc)

	problemRepo := repository.NewProblemRepository(repository.NewScoped(db))
	var activeProblemSvc service.ProblemService
	switch cfg.DataSource {
	case config.DataSourceServiceNow:
		activeProblemSvc = service.NewServiceNowProblemService(serviceNowIntegrationServiceClient)
	case config.DataSourcePostgresServiceNowDualWrite:
		// Pilot extension: problem CREATE (ServiceNow-first, synchronous,
		// same shape as the case/incident/change-request pilots above -- see
		// problemService.createProblemSNFirst's own doc comment) plus problem
		// UPDATE (Postgres-first, best-effort async ServiceNow mirror -- see
		// problemService.UpdateProblem's own doc comment). Reads stay on
		// Postgres in this mode; snProblemMirrorSvc's CreateProblem/
		// UpdateProblem are the only methods of it this mode ever calls.
		//
		// snWritebackDispatcher (the single shared instance constructed once
		// above) is reused as-is for problem UPDATE's async ServiceNow
		// mirror, same as incident's own dual-write branch above.
		snProblemMirrorSvc := service.NewServiceNowProblemService(serviceNowIntegrationServiceClient)
		activeProblemSvc = service.NewProblemServiceWithSNMirror(problemRepo, snProblemMirrorSvc, snWritebackDispatcher)
		// Resolving an incident as Solved (Workaround) creates its problem in
		// both stores, in the resolve request (workaround_problem.go); the
		// background post-resolution flow skips it in this mode (main.go).
		// Set in place, so incidentHandler above already has it.
		if creator, ok := activeProblemSvc.(service.WorkaroundProblemCreator); ok {
			activeIncidentSvc = service.WithWorkaroundProblemCreator(activeIncidentSvc, creator)
		}
	default:
		activeProblemSvc = service.NewProblemService(problemRepo)
	}
	problemHandler := handler.NewProblemHandler(activeProblemSvc)

	var alertHandler *handler.AlertHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		alertHandler = handler.NewAlertHandler(service.NewServiceNowAlertService(serviceNowIntegrationServiceClient))
	}

	var smartAlertHandler *handler.SmartAlertHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		smartAlertHandler = handler.NewSmartAlertHandler(service.NewServiceNowSmartAlertService(serviceNowIntegrationServiceClient))
	}

	incidentTaskRepo := repository.NewIncidentTaskRepository(repository.NewScoped(db))
	var activeIncidentTaskSvc service.IncidentTaskService
	if cfg.DataSource == config.DataSourceServiceNow {
		activeIncidentTaskSvc = service.NewServiceNowIncidentTaskService(serviceNowIntegrationServiceClient)
	} else {
		activeIncidentTaskSvc = service.NewIncidentTaskService(incidentTaskRepo)
	}
	incidentTaskHandler := handler.NewIncidentTaskHandler(activeIncidentTaskSvc)

	conversationRepo := repository.NewConversationRepository(repository.NewScoped(db))
	var activeConversationSvc service.ConversationService
	switch cfg.DataSource {
	case config.DataSourceServiceNow:
		activeConversationSvc = service.NewServiceNowConversationService(serviceNowIntegrationServiceClient)
	case config.DataSourcePostgresServiceNowDualWrite:
		// CreateConversation is ServiceNow-first and synchronous;
		// UpdateConversation mirrors to ServiceNow asynchronously,
		// best-effort, via the shared snWritebackDispatcher -- see
		// conversationService's snMirror doc comment.
		snConversationMirrorSvc := service.NewServiceNowConversationService(serviceNowIntegrationServiceClient)
		activeConversationSvc = service.NewConversationServiceWithSNWriteback(conversationRepo, snWritebackDispatcher, snConversationMirrorSvc)
	default:
		activeConversationSvc = service.NewConversationService(conversationRepo)
	}
	conversationHandler := handler.NewConversationHandler(activeConversationSvc)

	var outageNotificationHandler *handler.OutageNotificationHandler
	var outageCommunicationHandler *handler.OutageCommunicationHandler
	if cfg.HasDatabase() {
		outageNotificationHandler = handler.NewOutageNotificationHandler(
			service.NewOutageNotificationService(
				repository.NewOutageNotificationRepository(db), accessSvc))
		outageCommunicationHandler = handler.NewOutageCommunicationHandler(
			service.NewOutageCommunicationService(
				repository.NewOutageCommunicationRepository(db), accessSvc))
	}

	// *** THE else BRANCH IS THE WHOLE FIX. *** Before it, these endpoints
	// existed ONLY under DataSourceServiceNow, so a deployment running
	// postgres or postgres-servicenow-dual-write registered no outage routes
	// at all and the portal's Outages page got 404 page not found from the
	// mux -- which is exactly what staging was serving.
	//
	// It also matters for the cloud status port: its sweep reads the Postgres
	// outage table, so a portal-created outage has to land there to be seen.
	// Creating it in ServiceNow instead leaves the sweep reading nothing and
	// looks like a defect in the webhook port rather than a routing choice.
	var outageHandler *handler.OutageHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		outageHandler = handler.NewOutageHandler(service.NewServiceNowOutageService(serviceNowIntegrationServiceClient))
	} else if cfg.HasDatabase() {
		outageSvc := service.NewOutageService(repository.NewOutageRepository(repository.NewScoped(db)))
		// The status page hears about a declared or ended outage from the
		// write itself: outage.status_page_due on sre-events, posted by
		// csm-notification-service. Needs the operations topic and the
		// status-page scope; without either, csm-scheduled-tasks posts on
		// its tick as before.
		if srEventPublisher != nil && len(cfg.CloudStatusServiceIDs) > 0 {
			outageSvc = service.WithOutageCloudStatus(outageSvc, service.WithCloudStatusPublisher(
				service.NewCloudStatusService(repository.NewCloudStatusRepository(db), cfg.CloudStatusServiceIDs),
				srEventPublisher))
			slog.Info("cloud status: outage writes publish outage.status_page_due", "topic", cfg.SREEventHubTopic)
		}
		outageHandler = handler.NewOutageHandler(outageSvc)
	}

	// cloudStatusHandler is Postgres-only, and unconditionally so even though
	// the outage endpoints above are ServiceNow-only. It reads the mirrored
	// `outage` table directly rather than through the integration service:
	// the sweep is a background decision over every in-scope outage, not a
	// user-facing read, and routing it through ServiceNow would both reproduce
	// the flow it is replacing and make the port depend on the system being
	// decommissioned.
	// The public status dashboard's reads, consumed by
	// wso2-enterprise/uptime-dashboard via csm-integration-service. They
	// replace five ServiceNow Scripted REST APIs and have no
	// ServiceNow-backed counterpart here.
	//
	// *** GATED ON db != nil LIKE EVERY OTHER POSTGRES-ONLY HANDLER. *** An
	// earlier version built these unconditionally. With
	// DATA_SOURCE=servicenow the pool is nil, so the first request to any
	// /cloud-status or /internal/cloud-status route dereferenced nil --
	// Recovery caught the panic and returned a 500, which is a confusing
	// answer to a route that simply is not available in that mode. Leaving
	// them unregistered gives an honest 404 instead.
	var cloudStatusDashboardHandler *handler.CloudStatusDashboardHandler
	var cloudStatusHandler *handler.CloudStatusHandler
	// availabilityHandler is the PRODUCER for the three dashboard endpoints
	// above. They read service_availability; until now nothing in Postgres
	// wrote it -- csm-sync-service mirrors ServiceNow's output, so at cutover
	// the uptime figures would stop advancing with no error anywhere.
	var availabilityHandler *handler.AvailabilityHandler
	if db != nil {
		cloudStatusDashboardHandler = handler.NewCloudStatusDashboardHandler(
			service.NewCloudStatusDashboardService(repository.NewCloudStatusDashboardRepository(db)),
		)
		cloudStatusHandler = handler.NewCloudStatusHandler(
			service.NewCloudStatusService(repository.NewCloudStatusRepository(db), cfg.CloudStatusServiceIDs),
		)
		availabilitySvc, err := service.NewAvailabilityService(
			repository.NewAvailabilityRepository(db), cfg.AvailabilityTimezone)
		if err != nil {
			// A bad zone mis-dates every row the sweep writes, so this is a
			// startup error rather than a fallback.
			log.Fatalf("availability service: %v", err)
		}
		availabilityHandler = handler.NewAvailabilityHandler(availabilitySvc)
	}
	// globalHandler is wired for both data sources now: GetSystemMetadata has
	// a Postgres-backed implementation (globalService, reusing
	// referenceDataRepo above); GlobalSearch does not yet, and returns a
	// clear error in Postgres mode rather than 404 -- see globalService's own
	// doc comment.
	var globalHandler *handler.GlobalHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		globalHandler = handler.NewGlobalHandler(service.NewServiceNowGlobalService(serviceNowIntegrationServiceClient))
	} else {
		globalHandler = handler.NewGlobalHandler(service.NewGlobalService(
			referenceDataRepo,
			repository.NewGlobalSearchRepository(repository.NewScoped(db), crVisibility),
			accessSvc,
		))
	}

	// instance/usage tracking tables (migration 0054) -- see
	// instance_repo.go's own doc comment for the caveats around resolving an
	// instance's project/deployment/deployed-product references on this data
	// source.
	instanceRepo := repository.NewInstanceRepository(repository.NewScoped(db))
	var activeInstanceSvc service.InstanceService
	switch cfg.DataSource {
	case config.DataSourceServiceNow:
		activeInstanceSvc = service.NewServiceNowInstanceService(serviceNowIntegrationServiceClient)
	case config.DataSourcePostgresServiceNowDualWrite:
		// Read-only fallback to ServiceNow, same reasoning as
		// deploymentService/deployedProductService under this data source
		// (see instanceService.snMirror's own doc comment): Postgres's
		// instance/usage-tracking tables were never backfilled with
		// ServiceNow's existing history.
		snInstanceMirrorSvc := service.NewServiceNowInstanceService(serviceNowIntegrationServiceClient)
		activeInstanceSvc = service.NewInstanceServiceWithSNFallback(instanceRepo, snInstanceMirrorSvc)
	default:
		activeInstanceSvc = service.NewInstanceService(instanceRepo)
	}
	instanceHandler := handler.NewInstanceHandler(activeInstanceSvc)

	itServiceRepo := repository.NewITServiceRepository(db)
	var activeITServiceSvc service.ITServiceService
	if cfg.DataSource == config.DataSourceServiceNow {
		activeITServiceSvc = service.NewServiceNowITServiceService(serviceNowIntegrationServiceClient)
	} else {
		activeITServiceSvc = service.NewITServiceService(itServiceRepo)
	}
	itServiceHandler := handler.NewITServiceHandler(activeITServiceSvc)

	serviceOfferingRepo := repository.NewServiceOfferingRepository(db)
	var activeServiceOfferingSvc service.ServiceOfferingService
	if cfg.DataSource == config.DataSourceServiceNow {
		activeServiceOfferingSvc = service.NewServiceNowServiceOfferingService(serviceNowIntegrationServiceClient)
	} else {
		activeServiceOfferingSvc = service.NewServiceOfferingService(serviceOfferingRepo)
	}
	serviceOfferingHandler := handler.NewServiceOfferingHandler(activeServiceOfferingSvc)

	groupRepo := repository.NewGroupRepository(db)
	var activeGroupSvc service.GroupService
	if cfg.DataSource == config.DataSourceServiceNow {
		activeGroupSvc = service.NewServiceNowGroupService(serviceNowIntegrationServiceClient)
	} else {
		activeGroupSvc = service.NewGroupService(groupRepo)
	}
	groupHandler := handler.NewGroupHandler(activeGroupSvc)

	// Same gap as the outage handler above, and the outage form depends on
	// it: its configuration-item picker is this search, so a nil handler here
	// makes the create page unusable even once the outage routes exist.
	var configurationItemHandler *handler.ConfigurationItemHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		configurationItemHandler = handler.NewConfigurationItemHandler(service.NewServiceNowConfigurationItemService(serviceNowIntegrationServiceClient))
	} else if cfg.HasDatabase() {
		configurationItemHandler = handler.NewConfigurationItemHandler(service.NewConfigurationItemService(db))
	}

	commentRepo := repository.NewCommentRepository(repository.NewScoped(db), crVisibility)
	var activeCommentSvc service.CommentService
	switch cfg.DataSource {
	case config.DataSourceServiceNow:
		// Its only event is incident.comment_added, so it takes the incident
		// publisher.
		activeCommentSvc = service.NewServiceNowCommentService(serviceNowIntegrationServiceClient, incidentEventPublisher)
	case config.DataSourcePostgresServiceNowDualWrite:
		// CreateComment mirrors to ServiceNow, asynchronously, after
		// Postgres -- see commentService's own doc comment. This is separate
		// from case's own comment mirror (CreateCaseComment/CreateBareCaseComment),
		// which backs the case-scoped comment routes, not these generic ones.
		//
		// The mirror carries eventPublisher rather than nil: it is the only
		// thing on this path that publishes at all (the Postgres comment
		// service publishes nothing), so without it an incident comment in
		// dual-write mode would emit no incident.comment_added, and an
		// elevation-triggered escalation ladder would lose its only stop
		// signal. There is no double-publish risk for the same reason.
		snCommentMirrorSvc := service.NewServiceNowCommentService(serviceNowIntegrationServiceClient, incidentEventPublisher)
		activeCommentSvc = service.NewCommentServiceWithSNWriteback(commentRepo, userRepo, snWritebackDispatcher, snCommentMirrorSvc)
	default:
		activeCommentSvc = service.NewCommentService(commentRepo, userRepo)
	}
	commentHandler := handler.NewCommentHandler(activeCommentSvc)

	taskSlaRepo := repository.NewTaskSlaRepository(repository.NewScoped(db))
	var activeTaskSlaSvc service.TaskSlaService
	if cfg.DataSource == config.DataSourceServiceNow {
		activeTaskSlaSvc = service.NewServiceNowTaskSlaService(serviceNowIntegrationServiceClient)
	} else {
		activeTaskSlaSvc = service.NewTaskSlaService(taskSlaRepo)
	}
	taskSlaHandler := handler.NewTaskSlaHandler(activeTaskSlaSvc)

	var snUserHandler *handler.SNUserHandler
	if cfg.DataSource == config.DataSourceServiceNow {
		snUserHandler = handler.NewSNUserHandler(snUserService)
	}

	// Tasks are a ServiceNow-only entity, but the routes are registered for both
	// data sources: with no handler the mux answers 404, while the OpenAPI spec
	// documents a 503 ErrorResponse for these paths. The Postgres stand-in
	// supplies that 503 -- same shape as caseService's ServiceNow-only tag
	// operations.
	var activeTaskSvc service.TaskService
	if cfg.DataSource == config.DataSourceServiceNow {
		activeTaskSvc = service.NewServiceNowTaskService(serviceNowIntegrationServiceClient)
	} else {
		activeTaskSvc = service.NewUnavailableTaskService()
	}
	taskHandler := handler.NewTaskHandler(activeTaskSvc)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", handler.HealthCheck)

	if salesforceEventHandler != nil {
		mux.HandleFunc("POST /salesforce/events", salesforceEventHandler.HandleEvent)
	}
	if salesforcePartnerHandler != nil {
		mux.HandleFunc("POST /salesforce/accounts/{sfId}/refresh-partners", salesforcePartnerHandler.RefreshPartners)
	}
	if customerEngagementAllocationHandler != nil {
		mux.HandleFunc("POST /customer-engagements/allocation-events", customerEngagementAllocationHandler.ProcessAllocationEvent)
	}
	if onboardingStepHandler != nil {
		mux.HandleFunc("PUT /onboarding-steps/{membershipSfId}/{step}", onboardingStepHandler.UpsertOnboardingStep)
		mux.HandleFunc("GET /onboarding-steps/{membershipSfId}", onboardingStepHandler.GetOnboardingSteps)
		mux.HandleFunc("POST /onboarding-steps/search", onboardingStepHandler.SearchOnboardingSteps)
	}

	// event_publish_failures, sla_clocks, scheduled_task_run and
	// alert_incident_mapping are not data-source specific, but all four are
	// Postgres-backed, and the database is optional when
	// DATA_SOURCE=servicenow — so unlike the role catalogue and team
	// registry below, these are registered only when a pool exists. With no
	// database they 404 rather than panicking on a nil pool.
	if eventPublishFailureHandler != nil {
		mux.HandleFunc("POST /event-publish-failures", eventPublishFailureHandler.CreateEventPublishFailure)
		mux.HandleFunc("POST /event-publish-failures/search", eventPublishFailureHandler.SearchEventPublishFailures)
		mux.HandleFunc("POST /event-publish-failures/{id}/resolve", eventPublishFailureHandler.ResolveEventPublishFailure)
	}
	// sn_writeback_failures: DATA_SOURCE=postgres-servicenow-dual-write only
	// (the dispatcher that records them exists in no other mode). Internal
	// callers only: what the previous system is missing, and the replay, are
	// operations concerns a customer is never shown.
	if snWritebackFailureHandler != nil {
		mux.HandleFunc("GET /sn-writeback-failures", internalOnly(accessSvc, snWritebackFailureHandler.ListSNWritebackFailures))
		mux.HandleFunc("POST /sn-writeback-failures/{id}/replay", internalOnly(accessSvc, snWritebackFailureHandler.ReplaySNWritebackFailure))
	}
	if slaStatusHandler != nil {
		mux.HandleFunc("GET /sla-status", slaStatusHandler.SearchActiveSLAStatuses)
		mux.HandleFunc("GET /sla-status/clock-state", slaStatusHandler.GetClockState)
	}
	if slaDurationPolicyHandler != nil {
		mux.HandleFunc("GET /sla-duration-policy", slaDurationPolicyHandler.ListSLADurationPolicy)
	}
	if githubDeliveryHandler != nil {
		mux.HandleFunc("POST /github/deliveries", githubDeliveryHandler.Handle)
		mux.HandleFunc("POST /github/service-requests", githubServiceRequestHandler.Create)
	}

	if scheduledTaskRunHandler != nil {
		mux.HandleFunc("POST /scheduled-tasks/attempts", scheduledTaskRunHandler.AttemptScheduledTaskRun)
		mux.HandleFunc("PATCH /scheduled-tasks/attempts/{id}", scheduledTaskRunHandler.UpdateScheduledTaskRunAttempt)
		mux.HandleFunc("GET /scheduled-tasks/attempts", scheduledTaskRunHandler.ListScheduledTaskRuns)
		mux.HandleFunc("DELETE /scheduled-tasks/attempts", scheduledTaskRunHandler.DeleteScheduledTaskRuns)
	}
	if alertIncidentMappingHandler != nil {
		mux.HandleFunc("POST /alert-incident-mappings", alertIncidentMappingHandler.CreateAlertIncidentMapping)
		mux.HandleFunc("POST /alert-incident-mappings/lookup", alertIncidentMappingHandler.LookupAlertIncidentMappings)
	}
	if scheduleHandler != nil {
		mux.HandleFunc("GET /team-schedule/catalogue", scheduleHandler.GetScheduleCatalogue)
		mux.HandleFunc("POST /team-schedule/assignments/search", scheduleHandler.SearchScheduleAssignments)
		mux.HandleFunc("POST /team-schedule/absences/search", scheduleHandler.SearchScheduleAbsences)
		mux.HandleFunc("GET /team-schedule/on-duty", scheduleHandler.GetScheduleOnDuty)
	}
	if teamMemberHandler != nil {
		// Who holds which rank. The call-escalation ladder resolves four of its
		// five rungs from this, and the fifth by intersecting it with on-duty
		// above.
		mux.HandleFunc("GET /team-schedule/members", teamMemberHandler.GetTeamMembers)
	}
	if pagingChainHandler != nil {
		// Who is on each tier of the CRE and SRE paging chains, what the caller
		// may change, and changing one membership. The service reads the team
		// from the row being changed, never from the request.
		mux.HandleFunc("GET /team-schedule/paging-chain", pagingChainHandler.GetPagingChain)
		mux.HandleFunc("PATCH /team-schedule/paging-chain/members/{membershipId}", pagingChainHandler.UpdatePagingMember)
		mux.HandleFunc("GET /team-schedule/paging-readiness", pagingChainHandler.GetPagingReadiness)
		// Paging-only phone numbers. The reads and the test-result report are
		// for csm-notification-service too; the service checks who may do what.
		mux.HandleFunc("GET /team-schedule/paging-contacts", pagingChainHandler.ListPagingContacts)
		mux.HandleFunc("PUT /team-schedule/paging-contacts/{userId}", pagingChainHandler.SetPagingPhone)
		mux.HandleFunc("DELETE /team-schedule/paging-contacts/{userId}", pagingChainHandler.DeletePagingPhone)
		mux.HandleFunc("POST /team-schedule/paging-contacts/{userId}/test", pagingChainHandler.RequestTestCall)
		mux.HandleFunc("PUT /team-schedule/paging-contacts/{userId}/test-result", pagingChainHandler.RecordTestResult)
	}
	if scheduleHandler != nil {

		// Lead edit. Own team only -- the service reads the team from the row
		// being changed rather than from the request, so naming your own team
		// does not let you edit somebody else's slot.
		mux.HandleFunc("POST /team-schedule/assignments", scheduleHandler.CreateScheduleAssignment)
		mux.HandleFunc("PATCH /team-schedule/assignments/{id}", scheduleHandler.UpdateScheduleAssignment)
		mux.HandleFunc("DELETE /team-schedule/assignments/{id}", scheduleHandler.DeleteScheduleAssignment)
		mux.HandleFunc("GET /team-schedule/activity", scheduleHandler.GetScheduleActivity)
		mux.HandleFunc("GET /team-schedule/edit-markers", scheduleHandler.GetScheduleEditMarkers)
		mux.HandleFunc("GET /team-schedule/my-lead-teams", scheduleHandler.GetMyLeadTeams)
		mux.HandleFunc("POST /team-schedule/assignments/apply", scheduleHandler.ApplyScheduleRange)
		mux.HandleFunc("POST /team-schedule/absences/apply", scheduleHandler.ApplyScheduleAbsence)
		mux.HandleFunc("DELETE /team-schedule/absences/{id}", scheduleHandler.DeleteScheduleAbsence)
		mux.HandleFunc("POST /team-schedule/absence-kinds", scheduleHandler.CreateScheduleAbsenceKind)
		mux.HandleFunc("DELETE /team-schedule/absence-kinds/{code}", scheduleHandler.DeleteScheduleAbsenceKind)
	}
	if rotaGenerateHandler != nil {
		// A lead of the rota (or its admin) works out a month from the
		// availability marked on the roster; dryRun previews it.
		mux.HandleFunc("POST /team-schedule/rotas/{code}/generate", rotaGenerateHandler.GenerateRotaMonth)
	}
	if announcementRequestHandler != nil {
		mux.HandleFunc("POST /announcement-requests", announcementRequestHandler.CreateAnnouncementRequest)
		mux.HandleFunc("GET /announcement-requests/{id}", announcementRequestHandler.GetAnnouncementRequest)
		mux.HandleFunc("POST /announcement-requests/search", announcementRequestHandler.SearchAnnouncementRequests)
		mux.HandleFunc("PATCH /announcement-requests/{id}", announcementRequestHandler.UpdateAnnouncementRequest)
		mux.HandleFunc("POST /announcement-requests/{id}/dry-run", announcementRequestHandler.RecordAnnouncementRequestDryRun)
		mux.HandleFunc("POST /announcement-requests/{id}/submit", announcementRequestHandler.SubmitAnnouncementRequest)
		mux.HandleFunc("POST /announcement-requests/{id}/approve", announcementRequestHandler.ApproveAnnouncementRequest)
		mux.HandleFunc("POST /announcement-requests/{id}/schedule", announcementRequestHandler.ScheduleAnnouncementRequest)
		mux.HandleFunc("POST /announcement-requests/{id}/auto-publish", announcementRequestHandler.AutoPublishAnnouncementRequest)
		mux.HandleFunc("POST /announcement-requests/{id}/publish", announcementRequestHandler.PublishAnnouncementRequest)
		mux.HandleFunc("POST /announcement-requests/{id}/updates", announcementRequestHandler.CreateAnnouncementRequestUpdate)
		mux.HandleFunc("GET /announcement-requests/{id}/updates", announcementRequestHandler.ListAnnouncementRequestUpdates)
		mux.HandleFunc("POST /announcement-requests/{id}/deliveries", announcementRequestHandler.RecordAnnouncementRequestDeliveries)
		mux.HandleFunc("GET /announcement-requests/{id}/deliveries", announcementRequestHandler.ListAnnouncementRequestDeliveries)
	}
	if savedFilterViewHandler != nil {
		mux.HandleFunc("GET /users/me/saved-filter-views", savedFilterViewHandler.List)
		mux.HandleFunc("PATCH /users/me/saved-filter-views", savedFilterViewHandler.Save)
		mux.HandleFunc("DELETE /users/me/saved-filter-views", savedFilterViewHandler.Delete)
		mux.HandleFunc("POST /users/me/saved-filter-views/reorder", savedFilterViewHandler.Reorder)
	}

	if membershipRegistrationHandler != nil {
		mux.HandleFunc("POST /users/me/memberships/register", membershipRegistrationHandler.RegisterInvitedMemberships)
	}

	if snUserHandler != nil {
		mux.HandleFunc("GET /users/{id}", snUserHandler.GetUser)
		mux.HandleFunc("GET /users/me", snUserHandler.GetMe)
		mux.HandleFunc("PATCH /users/me", snUserHandler.PatchMe)
		mux.HandleFunc("POST /users/search", snUserHandler.SearchUsers)
	} else {
		mux.HandleFunc("GET /users/{id}", userHandler.GetUser)
		mux.HandleFunc("GET /users/me", userHandler.GetMe)
		mux.HandleFunc("PATCH /users/me", userHandler.PatchMe)
		mux.HandleFunc("POST /users/search", userHandler.SearchUsers)
		mux.HandleFunc("POST /users/by-ids", userHandler.GetUsersByIDs)
		mux.HandleFunc("POST /users", userHandler.CreateUser)
	}
	if snAccountHandler != nil {
		mux.HandleFunc("GET /accounts/{id}", internalOnly(accessSvc, snAccountHandler.GetAccount))
		mux.HandleFunc("POST /accounts/search", internalOnly(accessSvc, snAccountHandler.SearchAccounts))
	} else {
		mux.HandleFunc("GET /accounts/{id}", internalOnly(accessSvc, accountHandler.GetAccount))
		mux.HandleFunc("POST /accounts/search", internalOnly(accessSvc, accountHandler.SearchAccounts))
		mux.HandleFunc("PATCH /accounts/{id}", internalOnly(accessSvc, accountHandler.PatchAccountTeams))
	}
	if teamHandler != nil {
		mux.HandleFunc("GET /teams/{id}/members", teamHandler.GetTeamMembers)
		mux.HandleFunc("POST /teams/search", teamHandler.SearchTeams)
	}
	mux.HandleFunc("POST /accounts/{id}/contacts/search", internalOnly(accessSvc, accountContactHandler.SearchAccountContacts))
	if opportunityHandler != nil {
		mux.HandleFunc("POST /opportunities/search", opportunityHandler.SearchOpportunities)
		mux.HandleFunc("GET /opportunities/{id}", opportunityHandler.GetOpportunity)
	}
	if invoiceHandler != nil {
		mux.HandleFunc("POST /invoices/search", invoiceHandler.SearchInvoices)
		mux.HandleFunc("GET /invoices/{id}", invoiceHandler.GetInvoice)
	}
	if projectOpportunityLinkHandler != nil {
		mux.HandleFunc("POST /project-opportunity-links/search", projectOpportunityLinkHandler.SearchProjectOpportunityLinks)
	}
	mux.HandleFunc("GET /projects/{id}", projectHandler.GetProject)
	mux.HandleFunc("POST /projects/search", projectHandler.SearchProjects)
	mux.HandleFunc("POST /projects/{id}/contacts/search", projectMemberOnly(accessSvc, projectContactHandler.SearchProjectContacts))
	mux.HandleFunc("GET /projects/{id}/contacts/{contactId}", projectMemberOnly(accessSvc, projectContactHandler.GetProjectContact))
	if projectMembershipHandler != nil {
		// Beside the search and get above, in the same namespace. {email}
		// keys a membership; {contactId} on the GET above is a user id, and
		// the two never collide because the methods differ.
		mux.HandleFunc("POST /projects/{id}/contacts", projectMembershipHandler.InviteProjectContact)
		// The invitation's dry run. The literal "validate" segment wins over
		// {email} by specificity, and no POST is registered on {email} anyway.
		mux.HandleFunc("POST /projects/{id}/contacts/validate", projectMembershipHandler.ValidateProjectContact)
		mux.HandleFunc("PATCH /projects/{id}/contacts/{email}", projectMembershipHandler.UpdateProjectContactRoles)
		mux.HandleFunc("DELETE /projects/{id}/contacts/{email}", projectMembershipHandler.DeactivateProjectContact)
		mux.HandleFunc("POST /projects/{id}/contacts/{email}/resend-invitation", projectMembershipHandler.ResendProjectContactInvitation)
	}
	mux.HandleFunc("PATCH /projects/{id}", projectUpdateHandler.UpdateProject)
	mux.HandleFunc("GET /projects/{id}/metadata", projectMetadataHandler.GetProjectMetadata)
	mux.HandleFunc("GET /projects/{id}/cases/stats", projectCaseStatsHandler.GetProjectCaseStats)
	mux.HandleFunc("GET /projects/{id}/stats", projectStatsHandler.GetProjectStats)
	mux.HandleFunc("GET /projects/{id}/conversations/stats", projectStatsHandler.GetProjectConversationStats)
	mux.HandleFunc("GET /projects/{id}/deployments/stats", projectStatsHandler.GetProjectDeploymentStats)
	mux.HandleFunc("GET /projects/{id}/time-cards/stats", projectStatsHandler.GetProjectTimeCardStats)
	mux.HandleFunc("GET /projects/{id}/change-requests/stats", projectStatsHandler.GetProjectChangeRequestStats)
	if snProductHandler != nil {
		mux.HandleFunc("POST /products/search", snProductHandler.SearchProducts)
	} else {
		mux.HandleFunc("POST /products/search", productHandler.SearchProducts)
	}
	if snProductVersionHandler != nil {
		mux.HandleFunc("POST /products/{id}/versions/search", snProductVersionHandler.SearchProductVersions)
	} else if productVersionHandler != nil {
		mux.HandleFunc("POST /products/{id}/versions/search", productVersionHandler.SearchProductVersions)
	}
	// Always Postgres, including when products themselves are read from
	// ServiceNow. The literal path does not collide with /products/{id}.
	if db != nil {
		productRepoMappingHandler := handler.NewProductRepoMappingHandler(
			service.NewProductRepoMappingService(repository.NewProductRepoMappingRepository(db)))
		mux.HandleFunc("GET /products/github-repo", productRepoMappingHandler.Get)
	}
	mux.HandleFunc("POST /deployments", deploymentHandler.CreateDeployment)
	mux.HandleFunc("POST /deployments/search", deploymentHandler.SearchDeployments)
	mux.HandleFunc("PATCH /deployments/{id}", deploymentHandler.PatchDeployment)
	mux.HandleFunc("POST /deployed-products", deployedProductHandler.CreateDeployedProduct)
	mux.HandleFunc("POST /deployed-products/search", deployedProductHandler.SearchDeployedProducts)
	mux.HandleFunc("POST /deployed-products/projects/search", deployedProductHandler.SearchProjectsByProductVersion)
	mux.HandleFunc("PATCH /deployed-products/{id}", deployedProductHandler.PatchDeployedProduct)
	mux.HandleFunc("POST /deployed-products/{id}/metrics/search", deployedProductHandler.SearchDeployedProductMetrics)
	mux.HandleFunc("POST /deployed-products/{id}/metrics/usage-counts/search", deployedProductHandler.SearchDeployedProductUsageCounts)
	mux.HandleFunc("GET /cases/{id}", caseHandler.GetCase)
	mux.HandleFunc("PATCH /cases/{id}", caseHandler.PatchCase)
	mux.HandleFunc("POST /cases", caseHandler.CreateCase)
	mux.HandleFunc("POST /cases/search", caseHandler.SearchCases)
	// One-shot read of every announcement case for the CSM announcement
	// registry (it groups the whole set, so paging 50 at a time through
	// /cases/search was ~100 slow queries). Postgres-backed case data only;
	// where this route is absent the registry falls back to paging
	// /cases/search. Internal callers only.
	if cfg.DataSource != config.DataSourceServiceNow {
		announcementRegistryHandler := handler.NewAnnouncementRegistryHandler(
			service.NewAnnouncementRegistryService(repository.NewAnnouncementRegistryRepository(repository.NewScoped(db)), accessSvc))
		mux.HandleFunc("POST /announcements/registry/cases", internalOnly(accessSvc, announcementRegistryHandler.SearchRegistryCases))
	}
	mux.HandleFunc("POST /cases/aggregate", caseHandler.AggregateCases)
	mux.HandleFunc("POST /cases/feedback/search", feedbackHandler.SearchFeedback)
	mux.HandleFunc("POST /cases/feedback/aggregate", feedbackHandler.AggregateFeedback)
	mux.HandleFunc("POST /cases/{id}/comments", caseHandler.CreateCaseComment)
	mux.HandleFunc("POST /cases/{id}/comments/search", caseHandler.SearchCaseComments)
	mux.HandleFunc("POST /cases/{id}/activities/search", caseHandler.SearchCaseActivities)
	mux.HandleFunc("POST /attachments", attachmentHandler.CreateCaseAttachment)
	mux.HandleFunc("POST /attachments/{id}/confirm", attachmentHandler.ConfirmCaseAttachment)
	mux.HandleFunc("POST /attachments/search", attachmentHandler.SearchCaseAttachments)
	mux.HandleFunc("GET /attachments/{id}/content", attachmentHandler.GetCaseAttachmentContent)
	mux.HandleFunc("GET /attachments/{id}", attachmentHandler.GetAttachmentByID)
	mux.HandleFunc("PATCH /attachments/{id}", attachmentHandler.UpdateAttachment)
	mux.HandleFunc("DELETE /attachments/{id}", attachmentHandler.DeleteCaseAttachment)
	mux.HandleFunc("GET /cases/{id}/feedback", caseHandler.GetCaseFeedback)
	mux.HandleFunc("POST /cases/{id}/feedback", caseHandler.SubmitCaseFeedback)
	mux.HandleFunc("POST /cases/{id}/tags", caseHandler.AddCaseTag)
	mux.HandleFunc("DELETE /cases/{id}/tags/{tagId}", caseHandler.RemoveCaseTag)
	mux.HandleFunc("POST /tags/search", caseHandler.SearchTags)
	// Deprecated: the query-parameter form of tag search, kept for one release
	// so callers can be rolled out independently of this service. Remove it
	// (and CaseHandler.SearchTagsQuery) once they are all on the POST.
	//nolint:staticcheck // SA1019: intentional one-release compatibility route; remove with the handler.
	mux.HandleFunc("GET /tags/search", caseHandler.SearchTagsQuery)
	// KB routes are only registered when db != nil (see the KB handler
	// construction above) -- matches the eventPublishFailureHandler
	// pattern below: referencing a method on a nil handler here would
	// panic at server startup, before any request ever arrives.
	if kbArticleHandler != nil {
		mux.HandleFunc("POST /kb-articles", kbArticleHandler.CreateKBArticle)
		mux.HandleFunc("GET /kb-articles/{id}", kbArticleHandler.GetKBArticle)
		mux.HandleFunc("POST /kb-articles/search", kbArticleHandler.SearchKBArticles)
		mux.HandleFunc("PATCH /kb-articles/{id}/state", kbArticleHandler.PatchKBArticleState)
		mux.HandleFunc("POST /kb-manager-users/search", kbManagerUserHandler.SearchKBManagerUsers)
		mux.HandleFunc("POST /kb-manager-users", kbManagerUserHandler.CreateKBManagerUser)
		mux.HandleFunc("DELETE /kb-manager-users", kbManagerUserHandler.DeleteKBManagerUser)
		mux.HandleFunc("POST /kb-manager-groups/search", kbManagerGroupHandler.SearchKBManagerGroups)
		mux.HandleFunc("POST /kb-manager-groups", kbManagerGroupHandler.CreateKBManagerGroup)
		mux.HandleFunc("DELETE /kb-manager-groups", kbManagerGroupHandler.DeleteKBManagerGroup)
		mux.HandleFunc("PATCH /kb-articles/{id}", kbArticleHandler.PatchKBArticleContent)
		mux.HandleFunc("GET /knowledge-bases", knowledgeBaseHandler.ListKnowledgeBases)
		mux.HandleFunc("POST /knowledge-bases", knowledgeBaseHandler.CreateKnowledgeBase)
		mux.HandleFunc("PATCH /knowledge-bases/{id}", knowledgeBaseHandler.UpdateKnowledgeBaseName)
		mux.HandleFunc("PATCH /knowledge-bases/{id}/active", knowledgeBaseHandler.SetKnowledgeBaseActive)
		mux.HandleFunc("DELETE /kb-articles/{id}", kbArticleHandler.DeleteKBArticle)
		mux.HandleFunc("GET /kb-articles/{id}/history", kbArticleHandler.ListKBArticleHistory)
	}

	mux.HandleFunc("POST /call-requests", callRequestHandler.CreateCallRequest)
	mux.HandleFunc("POST /call-requests/search", callRequestHandler.SearchCallRequests)
	mux.HandleFunc("POST /call-requests/search-all", callRequestHandler.SearchAllCallRequests)
	mux.HandleFunc("PATCH /call-requests/{id}", callRequestHandler.PatchCallRequest)

	if caseGithubIssueHandler != nil {
		mux.HandleFunc("POST /cases/{id}/github-issues", caseGithubIssueHandler.CreateCaseGithubIssue)
	}

	mux.HandleFunc("GET /cases/{id}/escalations", caseEscalationHandler.SearchCaseEscalations)
	mux.HandleFunc("POST /cases/{id}/escalations", caseEscalationHandler.CreateCaseEscalation)

	// internalOnly: migration 0145's change_request_write_internal_only RLS
	// policy only ever permits an internal caller to INSERT into
	// change_request (the optional projectId on CreateChangeRequestRequest is
	// the customer project the change is raised for, not the creator's own
	// membership) -- same posture already established
	// for POST /incidents and POST /problems below. A non-internal caller
	// reaching changeRequestService.CreateChangeRequest's plain-Postgres
	// path would otherwise fail the RLS check with a raw 42501 (mapped to a
	// clean NotFoundError, but still a request that was never going to
	// succeed) -- gating here rejects it before any write is attempted.
	mux.HandleFunc("POST /change-requests", internalOnly(accessSvc, changeRequestHandler.CreateChangeRequest))
	mux.HandleFunc("POST /change-requests/search", changeRequestHandler.SearchChangeRequests)
	mux.HandleFunc("POST /change-requests/aggregate", changeRequestHandler.AggregateChangeRequests)
	// Internal callers only: the response lists a project's deployments and its
	// registered customer contacts by project id, which an external caller must
	// not be able to enumerate for projects they are not a member of.
	mux.HandleFunc("POST /change-requests/link-options", internalOnly(accessSvc, changeRequestHandler.GetChangeRequestLinkOptions))
	mux.HandleFunc("GET /change-requests/{id}", changeRequestHandler.GetChangeRequest)
	mux.HandleFunc("PATCH /change-requests/{id}", changeRequestHandler.PatchChangeRequest)
	mux.HandleFunc("GET /change-requests/{id}/approvals", changeRequestHandler.GetChangeRequestApprovals)
	mux.HandleFunc("POST /change-requests/{id}/approvals/decision", changeRequestHandler.DecideChangeRequestApproval)

	mux.HandleFunc("POST /time-cards/search", timeCardHandler.SearchTimeCards)
	// Time cards are logged, edited and deleted by internal staff only;
	// customers can read them (the two search routes) but never write. The
	// gate makes that a property of entity-service itself rather than of
	// whichever caller happens to sit in front of it. It costs no extra
	// query: callerIdentityMiddleware has already resolved the caller's scope
	// and internalOnly reads it back from the request context.
	mux.HandleFunc("POST /time-cards", internalOnly(accessSvc, timeCardHandler.CreateTimeCard))
	mux.HandleFunc("PATCH /time-cards/{id}", internalOnly(accessSvc, timeCardHandler.UpdateTimeCard))
	mux.HandleFunc("POST /cases/time-cards/search", timeCardHandler.SearchCaseTimeCards)
	mux.HandleFunc("DELETE /time-cards/{id}", internalOnly(accessSvc, timeCardHandler.DeleteTimeCard))

	mux.HandleFunc("POST /catalogs/search", catalogHandler.SearchCatalogs)
	mux.HandleFunc("GET /catalogs/{catalogId}/items/{catalogItemId}/variables", catalogHandler.GetCatalogItemVariables)

	mux.HandleFunc("POST /products/vulnerabilities/search", productVulnerabilityHandler.SearchProductVulnerabilities)
	mux.HandleFunc("GET /products/vulnerabilities/{id}", productVulnerabilityHandler.GetProductVulnerability)
	mux.HandleFunc("GET /products/vulnerabilities/meta", productVulnerabilityHandler.GetVulnerabilityMeta)
	mux.HandleFunc("POST /products/vulnerabilities/sync", productVulnerabilityHandler.SyncProductVulnerabilities)

	mux.HandleFunc("POST /services/search", itServiceHandler.SearchITServices)

	mux.HandleFunc("POST /service-offerings/search", serviceOfferingHandler.SearchServiceOfferings)

	mux.HandleFunc("POST /groups/search", groupHandler.SearchGroups)
	if groupDetailHandler != nil {
		mux.HandleFunc("GET /groups/{id}", groupDetailHandler.GetGroup)
	}

	if configurationItemHandler != nil {
		mux.HandleFunc("POST /configuration-items/search", configurationItemHandler.SearchConfigurationItems)
	}

	// CreateComment/SearchComments stay open to any validated caller --
	// customer-portal-backend-v2 calls both so customers can post and read
	// comments on their own cases (SearchComments' own row-level security,
	// migrations 0147/0175/0191, is what scopes a customer to comments they
	// may see). GetComment/UpdateComment/DeleteComment/GetCommentEditHistory
	// are internalOnly: unlike create/search, no customer-facing caller uses
	// any of the four (confirmed: customer-portal-backend-v2's own entity
	// client only ever calls CreateComment/SearchComments), and -- now that
	// none of the four performs its own author/role check any more, that
	// decision having moved entirely to csm-portal-backend (see
	// comment_service.go's own doc comments) -- leaving them open would let
	// any caller with a merely-valid token edit, delete, or read any
	// comment/its history directly, bypassing csm-portal-backend's checks
	// entirely. This does not reintroduce any per-comment author/role check
	// here; it only restricts WHO may reach these four routes at all, back to
	// the same "internal caller" boundary nearly every other mutating route
	// in this file already uses.
	mux.HandleFunc("POST /comments", commentHandler.CreateComment)
	mux.HandleFunc("POST /comments/search", commentHandler.SearchComments)
	mux.HandleFunc("GET /comments/{id}", internalOnly(accessSvc, commentHandler.GetComment))
	mux.HandleFunc("PATCH /comments/{id}", internalOnly(accessSvc, commentHandler.UpdateComment))
	mux.HandleFunc("DELETE /comments/{id}", internalOnly(accessSvc, commentHandler.DeleteComment))
	mux.HandleFunc("GET /comments/{id}/history", internalOnly(accessSvc, commentHandler.GetCommentEditHistory))

	mux.HandleFunc("GET /slas/{id}", internalOnly(accessSvc, taskSlaHandler.GetTaskSla))
	mux.HandleFunc("POST /slas/search", internalOnly(accessSvc, taskSlaHandler.SearchTaskSlas))

	// Registered unconditionally; the non-ServiceNow data source is served by
	// service.NewUnavailableTaskService, which answers 503 (see above).
	mux.HandleFunc("POST /cases/{id}/tasks/search", taskHandler.SearchCaseTasks)
	mux.HandleFunc("POST /tasks/search", taskHandler.SearchTasks)
	mux.HandleFunc("GET /tasks/{id}", taskHandler.GetTask)
	mux.HandleFunc("POST /cases/{id}/tasks", taskHandler.CreateCaseTask)
	mux.HandleFunc("PATCH /tasks/{id}", taskHandler.UpdateTask)

	mux.HandleFunc("GET /incidents/{id}", internalOnly(accessSvc, incidentHandler.GetIncident))
	mux.HandleFunc("PATCH /incidents/{id}", internalOnly(accessSvc, incidentHandler.PatchIncident))
	mux.HandleFunc("POST /incidents", internalOnly(accessSvc, incidentHandler.CreateIncident))
	mux.HandleFunc("GET /incidents/create-defaults", internalOnly(accessSvc, incidentHandler.GetIncidentCreateDefaults))
	mux.HandleFunc("POST /incidents/search", internalOnly(accessSvc, incidentHandler.SearchIncidents))
	mux.HandleFunc("POST /incidents/aggregate", internalOnly(accessSvc, incidentHandler.AggregateIncidents))
	mux.HandleFunc("POST /incidents/{id}/activities/search", internalOnly(accessSvc, incidentHandler.SearchIncidentActivities))
	mux.HandleFunc("POST /incidents/{id}/specialist-handoffs", internalOnly(accessSvc, incidentHandler.HandOffIncidentToSpecialist))
	mux.HandleFunc("GET /specialist-handoff-teams", internalOnly(accessSvc, incidentHandler.ListSpecialistHandoffTeams))

	// Postgres-backed, and deliberately separate from outageHandler above:
	// that one is the ServiceNow-backed outage entity API, this is only the
	// internal-stakeholder notification sweep. They will converge when the
	// outage entity itself moves to Postgres.
	if outageNotificationHandler != nil {
		mux.HandleFunc("POST /outage-notifications/sweep", outageNotificationHandler.SweepOutageNotifications)
		mux.HandleFunc("GET /outages/{id}/notification-state", outageNotificationHandler.GetOutageNotificationState)
	}

	// The SECOND outage notifier, and a different flow from the one above.
	// That is the internal-STAKEHOLDER notice; this is the SRE-facing
	// declaration/resolution pair (ServiceNow's `Outage Communication`).
	// They share the outage table and nothing else -- different audience,
	// different content, different idempotency mechanism.
	//
	// Keyed on the outage NUMBER rather than its id, because the
	// communication log has never carried anything else: ServiceNow's own
	// table has a reference column that is empty on all 336 rows.
	if outageCommunicationHandler != nil {
		mux.HandleFunc("POST /outage-communications/sweep", outageCommunicationHandler.SweepOutageCommunications)
		mux.HandleFunc("GET /outages/{number}/communication-log", outageCommunicationHandler.GetOutageCommunicationLog)
	}

	if outageHandler != nil {
		mux.HandleFunc("POST /outages", outageHandler.CreateOutage)
		mux.HandleFunc("POST /outages/search", outageHandler.SearchOutages)
		mux.HandleFunc("GET /outages/metadata", outageHandler.GetOutageMetadata)
		mux.HandleFunc("GET /outages/{id}", outageHandler.GetOutage)
		mux.HandleFunc("PATCH /outages/{id}", outageHandler.PatchOutage)
		mux.HandleFunc("POST /outages/{id}/communications", outageHandler.AddOutageCommunication)
		mux.HandleFunc("POST /outages/{id}/communications/search", outageHandler.SearchOutageCommunications)
	}

	// Cloud status webhooks: service-to-service, called by csm-scheduled-tasks.
	if cloudStatusDashboardHandler != nil {
		mux.HandleFunc("GET /cloud-status/monitors", cloudStatusDashboardHandler.Monitors)
		mux.HandleFunc("GET /cloud-status/incidents", cloudStatusDashboardHandler.Incidents)
		mux.HandleFunc("GET /cloud-status/availabilities", cloudStatusDashboardHandler.Availabilities)
		mux.HandleFunc("GET /cloud-status/availability-history", cloudStatusDashboardHandler.AvailabilityHistory)
		mux.HandleFunc("GET /cloud-status/incidents/{id}", cloudStatusDashboardHandler.IncidentDetail)
	}
	if availabilityHandler != nil {
		// Internal, and it WRITES: every other /cloud-status route is a read
		// the dashboard makes, this one recomputes and replaces rows.
		// internalOnly: auth.Middleware only validates a token, so without
		// this a customer's user token could trigger the recompute.
		mux.HandleFunc("POST /internal/availability/sweep", internalOnly(accessSvc, availabilityHandler.Sweep))
	}
	if cloudStatusHandler != nil {
		mux.HandleFunc("POST /internal/cloud-status/sweep", cloudStatusHandler.Sweep)
		mux.HandleFunc("GET /internal/cloud-status/pending", cloudStatusHandler.Pending)
		mux.HandleFunc("POST /internal/cloud-status/{id}/delivery", cloudStatusHandler.RecordDelivery)
		mux.HandleFunc("POST /internal/cloud-status/{id}/claim", cloudStatusHandler.Claim)
	}

	mux.HandleFunc("POST /problems", internalOnly(accessSvc, problemHandler.CreateProblem))
	mux.HandleFunc("POST /problems/search", internalOnly(accessSvc, problemHandler.SearchProblems))
	mux.HandleFunc("POST /problems/aggregate", internalOnly(accessSvc, problemHandler.AggregateProblems))
	mux.HandleFunc("GET /problems/{id}", internalOnly(accessSvc, problemHandler.GetProblem))
	mux.HandleFunc("PATCH /problems/{id}", internalOnly(accessSvc, problemHandler.PatchProblem))

	mux.HandleFunc("POST /incident-tasks/search", internalOnly(accessSvc, incidentTaskHandler.SearchIncidentTasks))
	mux.HandleFunc("POST /incident-tasks/aggregate", internalOnly(accessSvc, incidentTaskHandler.AggregateIncidentTasks))
	mux.HandleFunc("GET /incident-tasks/{id}", internalOnly(accessSvc, incidentTaskHandler.GetIncidentTask))
	mux.HandleFunc("PATCH /incident-tasks/{id}", internalOnly(accessSvc, incidentTaskHandler.PatchIncidentTask))

	if alertHandler != nil {
		mux.HandleFunc("GET /alerts/{id}", alertHandler.GetAlert)
	}

	if smartAlertHandler != nil {
		mux.HandleFunc("GET /smart-alerts/{id}", smartAlertHandler.GetSmartAlert)
	}

	mux.HandleFunc("POST /conversations/search", conversationHandler.SearchConversations)
	mux.HandleFunc("GET /conversations/{id}", conversationHandler.GetConversation)
	mux.HandleFunc("POST /conversations", conversationHandler.CreateConversation)
	mux.HandleFunc("PATCH /conversations/{id}", conversationHandler.UpdateConversation)

	mux.HandleFunc("GET /metadata", globalHandler.GetSystemMetadata)
	mux.HandleFunc("POST /search", globalHandler.GlobalSearch)

	mux.HandleFunc("POST /escalations/search", escalationHandler.SearchEscalations)
	mux.HandleFunc("POST /escalations", escalationHandler.CreateEscalation)

	mux.HandleFunc("POST /instances/search", instanceHandler.SearchInstances)
	mux.HandleFunc("POST /instances/metrics/search", instanceHandler.SearchInstanceMetrics)
	mux.HandleFunc("POST /instances/usages/search", instanceHandler.SearchInstanceUsage)
	mux.HandleFunc("POST /instances/metrics/stats/search", instanceHandler.SearchInstanceMetricsStats)
	mux.HandleFunc("POST /instances/usages/stats/search", instanceHandler.SearchInstanceUsageStats)

	// Token validation always runs -- there is no config flag to disable it.
	// The JWKS must load right here at startup: a wrong URL panics now instead
	// of silently rejecting every token later (same fail-fast posture as
	// apps/csm-portal/backend).
	tokenValidator, err := auth.NewValidator(context.Background(), auth.Config{
		Issuer:             cfg.AuthIssuer,
		JWKSURL:            cfg.AuthJWKSURL,
		UserTokenAudiences: cfg.AuthUserTokenAudiences,
		ClockSkew:          cfg.AuthClockSkew,
	})
	if err != nil {
		panic("auth: could not initialise token validation: " + err.Error())
	}

	// Both producers are closed together: they are constructed under the
	// same conditions and neither caller has any reason to outlive the
	// other. The Salesforce ingest retry worker stops first, and is waited
	// for, so a re-run in flight is not handed a publisher that has already
	// gone away. Cancelling its context also cancels the re-run's own
	// context (retryOne derives from it), so the wait is short.
	closePublishers := func() {
		stopIngestRetry()
		ingestRetryWG.Wait()
		if eventPublisher != nil {
			eventPublisher.Close()
		}
		if projectEventPublisher != nil {
			projectEventPublisher.Close()
		}
		if incidentEventPublisher != nil && incidentEventPublisher != eventPublisher {
			incidentEventPublisher.Close()
		}
		if srEventPublisher != nil {
			srEventPublisher.Close()
		}
		// Last: the retry worker above can still be invalidating users
		// until it has stopped.
		closeUserCache()
	}

	// PLG Customer Success Portal. Every repository, service, handler and route
	// it needs is in plg_routes.go — this is the only part of entity-service's
	// own wiring the merge touches.
	//
	// Gated on db != nil like every other Postgres-backed route above:
	// NewPoolIfNeeded returns a nil pool for DATA_SOURCE=servicenow, and PLG is
	// Postgres-only by construction — its tables do not exist in that mode.
	// Registering anyway would start cleanly and then nil-pointer on the first
	// query of every PLG request. Not registering means those paths 404, which
	// is the truthful answer where PLG has no data to serve.
	if db != nil {
		registerPLGRoutes(mux, db)
	}

	return middleware.CorrelationID(
		middleware.Recovery(
			middleware.Logger(
				middleware.UserIDToken(
					auth.Middleware(tokenValidator)(
						// Timeout wraps the identity lookup too: for an external caller
						// with no cached identity, ResolveScope runs two database
						// queries on the request context, and they must share the
						// same deadline as the handler instead of running unbounded.
						middleware.Timeout(cfg.RequestTimeout)(
							callerIdentityMiddleware(accessSvc)(mux),
						),
					),
				),
			),
		),
	), closePublishers
}

// CRVisibilityFromConfig builds the change request customer-visibility policy
// from CR_STRICT_VISIBILITY_FROM (see config.Config.CRStrictVisibilityFromRaw).
//
// Unset means no cutover: every change request is legacy, which is what
// customers saw before the strict rule existed (the safe default and the
// rollback). An unparsable value is refused at startup by Config.Validate;
// should this ever be reached with one (a caller that skipped Validate) it fails
// CLOSED: strict for every change request, so a typo can never widen what a
// customer sees.
func CRVisibilityFromConfig(cfg *config.Config) repository.CRVisibility {
	from, err := cfg.CRStrictVisibilityFrom()
	if err != nil {
		// Strict for every change request that can exist: a change request is
		// strict when it was created AT OR AFTER the instant, so the instant is
		// the earliest one a row could carry, never a far-future one (which would
		// make every row legacy, the opposite of failing closed).
		epoch := time.Unix(0, 0).UTC()
		return repository.CRVisibility{StrictFrom: &epoch}
	}
	return repository.CRVisibility{StrictFrom: from}
}

// logCRVisibility says at startup which visibility mode this process runs in:
// unset is also the state in which a deployment that means to be strict is
// silently NOT strict, so it is a WARN, not a note.
func logCRVisibility(cfg *config.Config) {
	from, err := cfg.CRStrictVisibilityFrom()
	switch {
	case err != nil:
		log.Printf("ERROR: %v -- treating EVERY change request as strict (visible to a customer only when designated to them)", err)
	case from == nil:
		log.Printf("WARN: CR_STRICT_VISIBILITY_FROM is not set: every change request is treated as legacy, so a customer sees every change request of their project past Authorize (and, in addition, the ones designated to them); set it to this release's instant to turn the designated-only rule on for change requests created from then on")
	default:
		log.Printf("change request customer visibility: strict (designated-only) for change requests created at or after %s; earlier ones are legacy", from.Format(time.RFC3339))
	}
}
