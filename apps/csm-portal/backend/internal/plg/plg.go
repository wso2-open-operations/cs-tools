// Package plg mounts the PLG Customer Success Portal onto csm-portal's backend.
//
// ONE CALL IS ALL csm-portal's main.go NEEDS. Everything PLG requires — its
// config, its entity-service client, its services, its handlers, its identity
// middleware and its 26 routes — is assembled here. The merge adds files; it
// does not edit csm-portal's own.
//
// WHAT CHANGED FROM THE STANDALONE BFF:
//
//   - No server of its own. PLG's handlers mount on csm-portal's mux and run
//     inside its middleware chain: CORS, correlation IDs, logging, security
//     headers and, crucially, JWT validation are all csm-portal's.
//   - No X-PLG-User header. See middleware/identity.go — the caller's email now
//     comes from the validated token, so there is no header to spoof.
//   - No /plg/health. csm-portal has /health; a second liveness endpoint
//     answering for one application inside a shared service is a probe that lies.
//   - No registration feed. Registrations are landed by the webhook-queue
//     service, which posts them straight to entity-service's ingest — see
//     WHERE REGISTRATIONS COME FROM below. This package only reads them back.
//
// WHERE REGISTRATIONS COME FROM. Nothing here ingests. The analytics source
// publishes to the webhook-queue service, and that service translates each
// record out of the source's vocabulary and posts it to entity-service's
// POST /plg/registrations/ingest on a timer.
//
// This backend polled that queue and did the translating itself, which put a
// poller, a source map, a record resolver and a set of ingest webhooks inside a
// backend-for-frontend — some 1,900 lines with no bearing on serving the
// frontend. The queue service already receives the analytics
// source's traffic, so the knowledge of that source belongs there, and the
// transaction was always entity-service's.
//
// What this means for anyone reading the configuration: there is no PLG_QUEUE_*,
// no PLG_SOURCE_MAP_PATH and no PLG_INGEST_SHARED_SECRET any more. They moved to
// the queue service as ENTITY_BASE_URL, SOURCE_MAP_PATH and the OAUTH2_* pair.
package plg

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"

	csmhandler "github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/handler"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/config"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/entityclient"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/handler"
	plgmw "github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/service"
)

// RouteFunc registers one route with the permission its caller must hold.
//
// This is csm-portal's own route() helper (cmd/server/main.go) seen from the
// inside: PLG is handed the ability to register a guarded route without being
// handed the guard. It therefore knows which permission each of its routes
// requires — a PLG decision — and nothing about how that permission is checked,
// which is csm-portal's.
//
// The one thing this type does NOT capture is middleware ordering, and it
// matters: whatever the caller wraps around h ends up OUTSIDE anything PLG
// wraps. That is what keeps the cheap roles check ahead of the identity
// resolver's entity-service call. See register().
type RouteFunc func(pattern string, perm csmhandler.Permission, h http.HandlerFunc)

// Mount wires PLG onto csm-portal's router.
//
// cfgPath points at PLG's own config.json — or nothing, in which case every
// setting comes from the PLG_* environment variables. Choreo deploys from
// environment alone, so the file is optional by design.
//
// route is csm-portal's own route() helper from cmd/server/main.go, passed in
// rather than reimplemented. Every route in this backend goes through it, PLG's
// included, so there is one place where a pattern is bound to the permission it
// requires. PLG therefore takes no mux and holds no AccessGuard: it declares
// what each route needs and lets the caller enforce it. See register() for the
// permissions and for why the identity middleware is nested inside.
//
// It returns no shutdown function because it starts nothing: PLG is a set of
// handlers on csm-portal's mux and owns no goroutine. It did own one, for the
// queue poller, until registrations moved to the webhook-queue service.
func Mount(
	cfgPath string,
	entityDefaults config.EntityDefaults,
	route RouteFunc,
) error {
	cfg, err := config.LoadWith(cfgPath, entityDefaults)
	if err != nil {
		return fmt.Errorf("plg: %w", err)
	}

	// One client, shared. It holds a connection pool of its own (net/http's)
	// and a timeout, so building a second would mean two.
	//
	// AUTHENTICATED, because entity-service sits behind a gateway that wants a
	// token. PLG presents the same OAuth2 client-credentials application as
	// every other upstream client in this backend — csm-portal's own entity
	// client, updates and SCIM all authenticate as it too. The credentials are
	// inherited rather than configured: see config.EntityDefaults.
	timeout := time.Duration(cfg.Entity.TimeoutSeconds) * time.Second
	entity := entityclient.New(entityclient.Config{
		BaseURL:    cfg.Entity.BaseURL,
		Timeout:    timeout,
		HTTPClient: entityHTTPClient(cfg.Entity.OAuth, timeout),
	})

	handlers := handler.NewHandlers(
		service.NewReferenceService(entityclient.ReferenceRepo{Client: entity}),
		service.NewOrganizationService(entityclient.OrganizationRepo{Client: entity}),
		service.NewOrgPlatformService(entityclient.OrgPlatformRepo{Client: entity}),
		service.NewPlaybookService(entityclient.PlaybookRepo{Client: entity}),
		service.NewAnalyticsService(entityclient.AnalyticsRepo{Client: entity}),
	)

	// PLG's routes carry one extra middleware of their own: the identity
	// resolver, which turns the validated caller into the "user".id every PLG
	// write records. It wraps only this subtree — csm-portal's routes neither
	// need it nor pay for it.
	register(handlers, plgmw.ResolveIdentity(entity), route)

	slog.Info("plg: mounted", "entityBaseURL", cfg.Entity.BaseURL)
	return nil
}

// register mounts PLG's routes, each wrapped in the identity middleware.
//
// Every path is prefixed /plg — csm-portal's backend has 116 routes of its own
// and `/products` means a different thing to each side. Verified before the
// merge: zero path overlaps.
// EVERY ROUTE PASSES TWO GUARDS, AND THEY ANSWER DIFFERENT QUESTIONS.
//
//   - `accessGuard.Require(perm, …)` — csm-portal's own, the same one its 116
//     routes use. A set lookup against the token's `roles` claim, no upstream
//     call. It asks: what does this token entitle you to?
//   - `identity` — PLG's own, see middleware/identity.go. It resolves the
//     caller's email to the `"user".id` every PLG write records as its actor,
//     and refuses anyone who is not ACTIVE INTERNAL staff. It asks: who are you
//     in the database, and are you still an employee?
//
// Neither subsumes the other. A token's roles claim cannot tell you that
// somebody was offboarded this morning — role revocation in the IdP and
// `is_active` in the platform database are separate lifecycles, and whichever
// happens second leaves a window the other check covers. Equally, a `"user"`
// row cannot tell you what the person is entitled to do.
//
// THE GUARD RUNS FIRST, DELIBERATELY. It is a map lookup; `identity` is an HTTP
// round trip to entity-service on every request. Ordering them this way means a
// caller who fails the role check costs nothing upstream.
//
// THE POLICY. PermUsePlg — CS engineer and admin — covers every route except
// the four that author a playbook template, which are PermManagePlaybooks and
// admin-only. Within PermUsePlg there is deliberately no view/write split: the
// queue is a shared worklist, and an engineer who can see a pairing is expected
// to act on it. Note that playbook *assignment* is PermUsePlg, not
// PermManagePlaybooks — an engineer runs playbooks, they just cannot edit the
// templates.
//
// Every route below is a read or a write made by a person. There is no machine
// caller: registrations reach the database through the webhook-queue service
// posting to entity-service, never through this backend.
func register(
	h *handler.Handlers,
	identity func(http.Handler) http.Handler,
	route RouteFunc,
) {
	// perm is a required argument with no default, exactly as in csm-portal's
	// own route() — a new PLG route cannot go live without someone choosing one.
	//
	// identity is applied HERE, inside what route() will wrap, so the composed
	// order stays guard-then-identity: route() puts accessGuard.Require on the
	// outside, and a caller whose roles are wrong is refused before the identity
	// resolver makes its entity-service call.
	add := func(pattern string, perm csmhandler.Permission, fn http.HandlerFunc) {
		// Both forwarders wrap identity rather than the other way round: the
		// identity resolver makes an entity-service call of its own, and it
		// should be correlated AND attributed like every other hop. See their
		// doc comments.
		route(pattern, perm,
			plgmw.ForwardCorrelationID(
				plgmw.ForwardUserIDToken(identity(fn)),
			).ServeHTTP)
	}

	// Reference data.
	add("GET /plg/me", csmhandler.PermUsePlg, h.Me)
	add("GET /plg/products", csmhandler.PermUsePlg, h.ListProducts)
	add("GET /plg/cs-users", csmhandler.PermUsePlg, h.ListCSUsers)
	add("GET /plg/lifecycle", csmhandler.PermUsePlg, h.Lifecycle)

	// Organisations and the overview tab.
	add("POST /plg/organizations/search", csmhandler.PermUsePlg, h.SearchOrganizations)
	add("GET /plg/organizations/{organizationId}", csmhandler.PermUsePlg, h.GetOrganization)
	add("PATCH /plg/organizations/{organizationId}", csmhandler.PermUsePlg, h.PatchOrganization)

	// The product tab — one organisation, one platform.
	add("GET /plg/organizations/{organizationId}/products/{product}", csmhandler.PermUsePlg, h.GetProduct)
	add("PATCH /plg/organizations/{organizationId}/products/{product}", csmhandler.PermUsePlg, h.PatchProduct)
	add("POST /plg/organizations/{organizationId}/products/{product}/playbook-runs", csmhandler.PermUsePlg, h.AttachPlaybook)
	add("POST /plg/organizations/{organizationId}/products/{product}/notes", csmhandler.PermUsePlg, h.CreateNote)

	// Playbook execution.
	add("DELETE /plg/playbook-runs/{playbookRunId}", csmhandler.PermUsePlg, h.DetachRun)
	add("PATCH /plg/playbook-run-tasks/{taskId}", csmhandler.PermUsePlg, h.PatchRunTask)
	add("PATCH /plg/notes/{noteId}", csmhandler.PermUsePlg, h.PatchNote)

	// New registrations.
	add("POST /plg/registrations/search", csmhandler.PermUsePlg, h.SearchRegistrations)
	add("POST /plg/registrations/{orgPlatformId}/acknowledge", csmhandler.PermUsePlg, h.Acknowledge)

	// Playbook manager.
	add("GET /plg/playbooks", csmhandler.PermUsePlg, h.ListPlaybooks)
	add("POST /plg/products/{product}/playbooks", csmhandler.PermManagePlaybooks, h.CreatePlaybook)
	add("GET /plg/playbooks/{playbookId}", csmhandler.PermUsePlg, h.GetPlaybook)
	add("PATCH /plg/playbooks/{playbookId}", csmhandler.PermManagePlaybooks, h.PatchPlaybook)
	add("PUT /plg/playbooks/{playbookId}/tasks", csmhandler.PermManagePlaybooks, h.ReplacePlaybookTasks)
	add("DELETE /plg/playbooks/{playbookId}", csmhandler.PermManagePlaybooks, h.DeletePlaybook)

	// Analytics.
	add("GET /plg/analytics/dashboard", csmhandler.PermUsePlg, h.Dashboard)
	add("GET /plg/work-queue", csmhandler.PermUsePlg, h.WorkQueue)
}

// entityHTTPClient returns a client that attaches an OAuth2 bearer token to
// every request, or nil when no grant is configured.
//
// Nil is the right answer for a local stack where entity-service is reachable
// without one; it is the wrong answer anywhere the gateway is in front, and
// there the symptom is a 401 from every PLG request rather than a startup
// failure — the grant is optional precisely so local development needs no
// credentials.
//
// Tokens are cached and refreshed by the oauth2 library. This is a second token
// source alongside csm-portal's own, which costs one extra token fetch per
// expiry window and keeps the two halves independently configurable.
func entityHTTPClient(cfg config.OAuth2Config, timeout time.Duration) *http.Client {
	if !cfg.Enabled() {
		return nil
	}
	cc := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
	}
	if cfg.Scope != "" {
		cc.Scopes = strings.Split(cfg.Scope, ",")
	}
	tokenCtx := context.WithValue(context.Background(), oauth2.HTTPClient,
		&http.Client{Timeout: timeout})
	client := cc.Client(tokenCtx)
	client.Timeout = timeout
	return client
}
