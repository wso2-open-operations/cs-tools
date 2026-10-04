package server

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/handler"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// registerPLGRoutes wires the PLG Customer Success Portal onto an existing mux.
//
// EVERYTHING PLG NEEDS FROM routes.go IS ONE CALL. That is the point of this
// file: the merge adds PLG to entity-service without editing a single line of
// the implementations csm-portal depends on. Repositories, services, handlers
// and all 28 route registrations live here; `NewRouter` gains one line.
//
// Go offers no way to self-register without an init() hack and a package-level
// mux, which would be worse than the one line — it would hide the dependency
// and make route ordering depend on import order.
//
// EVERY ROUTE IS PREFIXED /plg/, and that is not cosmetic. entity-service
// already owns `GET /users/me` and `POST /products/search`, and PLG has its own
// notion of both: a pairing's *platform* is not a WSO2 product, and PLG's
// caller is not entity-service's. The prefix is what lets the two coexist with
// neither side changing. Confirmed mechanically before the merge: 28 PLG routes
// against 132 of entity-service's, zero path overlaps.
//
// `GET /plg/health` is deliberately absent. entity-service has its own
// `/health`, and a second liveness endpoint answering for one application
// inside a shared service is a probe that lies.
func registerPLGRoutes(mux *http.ServeMux, db *pgxpool.Pool, access service.AccessService, users service.UserService, internalClientIDs map[string]bool) {
	// Reference data — the platform catalogue and the lifecycle stages.
	referenceRepo := repository.NewReferenceRepository(db)
	referenceHandler := handler.NewPlgReferenceHandler(service.NewReferenceService(referenceRepo))

	// The shared "user" table. Search only: PLG reads the CS team, it does not
	// administer it, so there is no insert, update or delete path anywhere in
	// the slice. Every query is restricted to INTERNAL staff — see
	// plg_users_repo.go, where that condition is prepended rather than left to
	// the caller.
	//
	// Being ACTIVE is a separate filter and the caller chooses it: an owner
	// picker asks for active engineers only, while an attribution lookup does
	// not, because a note written by someone who has since left must still
	// render their name. The portal backend asks for both.
	//
	// Every PLG route is internalOnly: the portal backend (an allow-listed
	// internal client) and internal staff reach them, nobody else does. The
	// pairing writes attribute through PlgActorResolver, which takes the actor
	// from the validated identity and accepts a body actorId only from an
	// allow-listed internal client.
	userRepo := repository.NewPlgUserRepository(db)
	usersHandler := handler.NewPlgUsersHandler(service.NewPlgUserService(userRepo))

	organizationRepo := repository.NewOrganizationRepository(db)
	organizationHandler := handler.NewPlgOrganizationHandler(
		service.NewOrganizationService(organizationRepo))

	// The pairing repository takes the playbook repository: the product tab
	// serves the playbooks available at the pairing's stage, and which ones
	// those are follows from its health.
	playbookRepo := repository.NewPlaybookRepository(db)
	playbookHandler := handler.NewPlgPlaybookHandler(service.NewPlaybookService(playbookRepo))

	pairingRepo := repository.NewOrgPlatformRepository(db, playbookRepo)
	pairingHandler := handler.NewPlgPairingHandler(service.NewPairingService(pairingRepo),
		handler.NewPlgActorResolver(users, internalClientIDs))

	ingestHandler := handler.NewPlgIngestHandler(service.NewIngestService(
		repository.NewIngestRepository(db), repository.NewFailureRepository(db)))

	analyticsRepo := repository.NewAnalyticsRepository(db)
	analyticsHandler := handler.NewPlgAnalyticsHandler(service.NewAnalyticsService(analyticsRepo))

	// --- reference -----------------------------------------------------------
	mux.HandleFunc("GET /plg/products", internalOnly(access, referenceHandler.ListProducts))
	mux.HandleFunc("GET /plg/lifecycle", internalOnly(access, referenceHandler.Lifecycle))
	mux.HandleFunc("POST /plg/users/search", internalOnly(access, usersHandler.SearchUsers))

	// --- organisations -------------------------------------------------------
	mux.HandleFunc("POST /plg/organizations/search", internalOnly(access, organizationHandler.SearchOrganizations))
	mux.HandleFunc("GET /plg/organizations/{organizationId}", internalOnly(access, organizationHandler.GetOrganization))
	mux.HandleFunc("PATCH /plg/organizations/{organizationId}", internalOnly(access, organizationHandler.PatchOrganization))

	// --- the product tab: one organisation, one platform ---------------------
	mux.HandleFunc("GET /plg/organizations/{organizationId}/products/{productCode}", internalOnly(access, pairingHandler.GetPairing))
	mux.HandleFunc("PATCH /plg/organizations/{organizationId}/products/{productCode}", internalOnly(access, pairingHandler.PatchPairing))                // W2
	mux.HandleFunc("POST /plg/organizations/{organizationId}/products/{productCode}/playbook-runs", internalOnly(access, pairingHandler.AttachPlaybook)) // W3
	mux.HandleFunc("POST /plg/organizations/{organizationId}/products/{productCode}/notes", internalOnly(access, pairingHandler.CreateNote))             // S2

	// --- playbook execution --------------------------------------------------
	mux.HandleFunc("DELETE /plg/playbook-runs/{playbookRunId}", internalOnly(access, pairingHandler.DetachRun)) // W4
	mux.HandleFunc("PATCH /plg/playbook-run-tasks/{taskId}", internalOnly(access, pairingHandler.PatchRunTask)) // W5
	mux.HandleFunc("PATCH /plg/notes/{noteId}", internalOnly(access, pairingHandler.UpdateNote))                // W6

	// Two small reads the BFF needs to apply its own rules: the shape lets it
	// reject a value a task cannot hold, by name, before writing; the location
	// turns a pairing id into the org+product the portal addresses it by.
	mux.HandleFunc("GET /plg/playbook-run-tasks/{taskId}/shape", internalOnly(access, pairingHandler.GetRunTaskShape))
	mux.HandleFunc("GET /plg/pairings/{orgPlatformId}/location", internalOnly(access, pairingHandler.LocatePairing))

	// --- registrations -------------------------------------------------------
	mux.HandleFunc("POST /plg/registrations/search", internalOnly(access, pairingHandler.SearchRegistrations))
	mux.HandleFunc("POST /plg/registrations/{orgPlatformId}/acknowledge", internalOnly(access, pairingHandler.Acknowledge)) // W1

	// --- playbook templates --------------------------------------------------
	mux.HandleFunc("GET /plg/playbooks", internalOnly(access, playbookHandler.ListPlaybooks))
	mux.HandleFunc("GET /plg/playbooks/{playbookId}", internalOnly(access, playbookHandler.GetPlaybook))
	mux.HandleFunc("POST /plg/products/{productCode}/playbooks", internalOnly(access, playbookHandler.CreatePlaybook))  // W7
	mux.HandleFunc("PATCH /plg/playbooks/{playbookId}", internalOnly(access, playbookHandler.PatchPlaybook))            // S3
	mux.HandleFunc("PUT /plg/playbooks/{playbookId}/tasks", internalOnly(access, playbookHandler.ReplacePlaybookTasks)) // W8
	mux.HandleFunc("DELETE /plg/playbooks/{playbookId}", internalOnly(access, playbookHandler.DeletePlaybook))          // S4

	// --- ingest --------------------------------------------------------------
	// The poller lives in the BFF and calls these, so the transaction that
	// lands a registration stays on this side of the wire.
	mux.HandleFunc("POST /plg/registrations/ingest", internalOnly(access, ingestHandler.Register)) // I1
	mux.HandleFunc("POST /plg/ingest-failures", internalOnly(access, ingestHandler.RecordFailure)) // I2

	// --- analytics -----------------------------------------------------------
	// The work queue is a POST search because it takes eight repeatable filter
	// dimensions; the dashboard is a GET because it takes two optional dates.
	mux.HandleFunc("GET /plg/analytics/dashboard", internalOnly(access, analyticsHandler.Dashboard))
	mux.HandleFunc("POST /plg/work-queue/search", internalOnly(access, analyticsHandler.SearchWorkQueue))
}
