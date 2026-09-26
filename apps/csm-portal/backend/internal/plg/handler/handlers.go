package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/domain"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/service"
)

// Handlers holds every HTTP handler and the services they delegate to.
type Handlers struct {
	reference service.ReferenceService
	orgs      service.OrganizationService
	pairings  service.OrgPlatformService
	playbooks service.PlaybookService
	analytics service.AnalyticsService
}

// NewHandlers wires the HTTP layer over the services.
func NewHandlers(
	reference service.ReferenceService,
	orgs service.OrganizationService,
	pairings service.OrgPlatformService,
	playbooks service.PlaybookService,
	analytics service.AnalyticsService,
) *Handlers {
	return &Handlers{
		reference: reference, orgs: orgs, pairings: pairings,
		playbooks: playbooks, analytics: analytics,
	}
}

// actor returns the calling engineer's email, as resolved by Identity.
func actor(r *http.Request) string { return middleware.UserIDFromContext(r.Context()) }

// ---------------------------------------------------------------------------
// Reference
// ---------------------------------------------------------------------------

// Health answers the liveness probe.
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ListProducts serves GET /products.
func (h *Handlers) ListProducts(w http.ResponseWriter, r *http.Request) {
	items, err := h.reference.ListProducts(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"products": items})
}

// ListCSUsers serves GET /cs-users.
func (h *Handlers) ListCSUsers(w http.ResponseWriter, r *http.Request) {
	items, err := h.reference.ListCSUsers(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": items})
}

// Me serves GET /me — who the portal thinks is calling.
func (h *Handlers) Me(w http.ResponseWriter, r *http.Request) {
	// Read from the context, not by looking the caller up again. The identity
	// middleware already resolved the caller's email to a full UserRef in order
	// to let the request through at all, so fetching it a second time would be a
	// round trip to learn what we were told.
	//
	// Note `actor(r)` is an id, not an email — passing it to a by-email lookup
	// would put a UUID in the email field.
	if user, ok := middleware.UserRefFromContext(r.Context()); ok {
		writeJSON(w, http.StatusOK, user)
		return
	}
	// No identity at all — only reachable in "header" mode with no default
	// configured, which is a local-development shape.
	writeJSON(w, http.StatusOK, domain.UserRef{})
}

// Lifecycle serves GET /lifecycle — the nine stages and the seven playbook paths.
func (h *Handlers) Lifecycle(w http.ResponseWriter, r *http.Request) {
	cat, err := h.reference.LifecycleCatalogue(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, cat)
}

// ---------------------------------------------------------------------------
// Organisations
// ---------------------------------------------------------------------------

// SearchOrganizations serves POST /organizations/search.
func (h *Handlers) SearchOrganizations(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchOrganizationsRequest
	if !decodeOptionalRequest(w, r, &req) {
		return
	}
	res, err := h.orgs.Search(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// GetOrganization serves GET /organizations/{organizationId}.
func (h *Handlers) GetOrganization(w http.ResponseWriter, r *http.Request) {
	org, err := h.orgs.Get(r.Context(), r.PathValue("organizationId"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, org)
}

// PatchOrganization serves PATCH /organizations/{organizationId}.
func (h *Handlers) PatchOrganization(w http.ResponseWriter, r *http.Request) {
	var req domain.PatchOrganizationRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.ID = r.PathValue("organizationId")

	org, err := h.orgs.Patch(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, org)
}

// ---------------------------------------------------------------------------
// The product tab
// ---------------------------------------------------------------------------

// GetProduct serves GET /organizations/{organizationId}/products/{product}.
func (h *Handlers) GetProduct(w http.ResponseWriter, r *http.Request) {
	detail, err := h.pairings.Get(r.Context(), r.PathValue("organizationId"), r.PathValue("product"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// PatchProduct serves PATCH /organizations/{organizationId}/products/{product}.
func (h *Handlers) PatchProduct(w http.ResponseWriter, r *http.Request) {
	var req domain.PatchOrgPlatformRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.OrganizationID, req.ProductCode = r.PathValue("organizationId"), r.PathValue("product")

	detail, err := h.pairings.Patch(r.Context(), req, actor(r))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// AttachPlaybook serves POST /organizations/{organizationId}/products/{product}/playbook-runs.
func (h *Handlers) AttachPlaybook(w http.ResponseWriter, r *http.Request) {
	var req domain.AttachPlaybookRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.OrganizationID, req.ProductCode = r.PathValue("organizationId"), r.PathValue("product")

	detail, err := h.pairings.AttachPlaybook(r.Context(), req, actor(r))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, detail)
}

// DetachRun serves DELETE /playbook-runs/{playbookRunId}.
func (h *Handlers) DetachRun(w http.ResponseWriter, r *http.Request) {
	detail, err := h.pairings.DetachRun(r.Context(), r.PathValue("playbookRunId"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// PatchRunTask serves PATCH /playbook-run-tasks/{taskId}.
func (h *Handlers) PatchRunTask(w http.ResponseWriter, r *http.Request) {
	var req domain.PatchRunTaskRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.ID = r.PathValue("taskId")

	detail, err := h.pairings.PatchRunTask(r.Context(), req, actor(r))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// PatchNote serves PATCH /notes/{noteId}.
//
// Addressed by note id alone rather than under the pairing: a note belongs to
// one pairing already, so repeating that in the path would let the two disagree.
// Answers with the reloaded ProductDetail, like every other write on the tab.
func (h *Handlers) PatchNote(w http.ResponseWriter, r *http.Request) {
	var req domain.UpdateNoteRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.ID = r.PathValue("noteId")

	detail, err := h.pairings.UpdateNote(r.Context(), req, actor(r))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// CreateNote serves POST /organizations/{organizationId}/products/{product}/notes.
func (h *Handlers) CreateNote(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateNoteRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.OrganizationID, req.ProductCode = r.PathValue("organizationId"), r.PathValue("product")

	detail, err := h.pairings.CreateNote(r.Context(), req, actor(r))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, detail)
}

// ---------------------------------------------------------------------------
// Registrations
// ---------------------------------------------------------------------------

// SearchRegistrations serves POST /registrations/search.
func (h *Handlers) SearchRegistrations(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchRegistrationsRequest
	if !decodeOptionalRequest(w, r, &req) {
		return
	}
	res, err := h.pairings.SearchRegistrations(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// Acknowledge serves POST /registrations/{orgPlatformId}/acknowledge.
func (h *Handlers) Acknowledge(w http.ResponseWriter, r *http.Request) {
	var req domain.AcknowledgeRequest
	if !decodeOptionalRequest(w, r, &req) {
		return
	}
	req.OrgPlatformID = r.PathValue("orgPlatformId")
	// Acknowledging without naming an owner means claiming it yourself, which is
	// what the button on the panel does. actor(r) is an id — the middleware
	// resolved it from the caller's email before this handler ran.
	if req.OwnerID == nil && actor(r) != "" {
		id := actor(r)
		req.OwnerID = &id
	}

	detail, err := h.pairings.Acknowledge(r.Context(), req, actor(r))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// ---------------------------------------------------------------------------
// Playbook manager
// ---------------------------------------------------------------------------

// ListPlaybooks serves GET /playbooks, optionally narrowed with ?product=CODE.
func (h *Handlers) ListPlaybooks(w http.ResponseWriter, r *http.Request) {
	var (
		items []domain.Playbook
		err   error
	)
	if code := strings.TrimSpace(r.URL.Query().Get("product")); code != "" {
		items, err = h.playbooks.ListByProduct(r.Context(), code)
	} else {
		items, err = h.playbooks.ListAll(r.Context())
	}
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"playbooks": items})
}

// GetPlaybook serves GET /playbooks/{playbookId}.
func (h *Handlers) GetPlaybook(w http.ResponseWriter, r *http.Request) {
	pb, err := h.playbooks.Get(r.Context(), r.PathValue("playbookId"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, pb)
}

// CreatePlaybook serves POST /products/{product}/playbooks.
func (h *Handlers) CreatePlaybook(w http.ResponseWriter, r *http.Request) {
	var req domain.CreatePlaybookRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.ProductCode = r.PathValue("product")

	pb, err := h.playbooks.Create(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, pb)
}

// PatchPlaybook serves PATCH /playbooks/{playbookId}.
func (h *Handlers) PatchPlaybook(w http.ResponseWriter, r *http.Request) {
	var req domain.PatchPlaybookRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.ID = r.PathValue("playbookId")

	pb, err := h.playbooks.Patch(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, pb)
}

// ReplacePlaybookTasks serves PUT /playbooks/{playbookId}/tasks.
func (h *Handlers) ReplacePlaybookTasks(w http.ResponseWriter, r *http.Request) {
	var req domain.ReplacePlaybookTasksRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.PlaybookID = r.PathValue("playbookId")

	pb, err := h.playbooks.ReplaceTasks(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, pb)
}

// DeletePlaybook serves DELETE /playbooks/{playbookId}.
func (h *Handlers) DeletePlaybook(w http.ResponseWriter, r *http.Request) {
	if err := h.playbooks.Delete(r.Context(), r.PathValue("playbookId")); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Analytics
// ---------------------------------------------------------------------------

// Dashboard serves GET /analytics/dashboard?from=&to=.
func (h *Handlers) Dashboard(w http.ResponseWriter, r *http.Request) {
	rng, err := analyticsRange(r)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	res, err := h.analytics.Dashboard(r.Context(), rng)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// WorkQueue serves GET /work-queue with repeatable filter parameters.
func (h *Handlers) WorkQueue(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := domain.WorkQueueFilters{
		HealthStates:    enumsOf[domain.HealthState](csvParam(q["health"])),
		OwnerIDs:        csvParam(q["owner"]),
		OrganizationIDs: csvParam(q["organizationId"]),
		ProductCodes:    csvParam(q["product"]),
		LifecycleStages: enumsOf[domain.LifecycleStage](csvParam(q["stage"])),
		Reasons:         enumsOf[domain.QueueReason](csvParam(q["reason"])),
		PlaybookIDs:     csvParam(q["playbookId"]),
		TaskCodes:       csvParam(q["taskCode"]),
	}
	// "mine" is the shortcut the queue opens with, and it stays a BFF concern.
	// entity-service has no notion of a current user — it is handed a list of
	// owner ids like any other filter — so this is where "me" becomes one.
	if q.Get("mine") == "true" {
		f.OwnerIDs = []string{actor(r)}
	}

	res, err := h.analytics.WorkQueue(r.Context(), f)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// ---------------------------------------------------------------------------
// Query-parameter helpers
// ---------------------------------------------------------------------------

// enumsOf converts query strings into a string-kinded enum slice. Whether the
// values are valid is the service layer's business, not the decoder's.
func enumsOf[T ~string](values []string) []T {
	out := make([]T, 0, len(values))
	for _, v := range values {
		out = append(out, T(v))
	}
	return out
}

// csvParam accepts both repeated parameters (?product=A&product=B) and comma
// separated ones (?product=A,B), because both are natural to type into a URL.
func csvParam(values []string) []string {
	out := []string{}
	for _, v := range values {
		for _, part := range strings.Split(v, ",") {
			if part = strings.TrimSpace(part); part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

// analyticsRange reads from/to, accepting either an RFC 3339 timestamp or a
// plain date.
func analyticsRange(r *http.Request) (domain.AnalyticsRange, error) {
	var rng domain.AnalyticsRange
	from, err := parseTimeParam(r.URL.Query().Get("from"), "from")
	if err != nil {
		return rng, err
	}
	to, err := parseTimeParam(r.URL.Query().Get("to"), "to")
	if err != nil {
		return rng, err
	}
	rng.From, rng.To = from, to
	return rng, nil
}

func parseTimeParam(raw, field string) (*time.Time, error) {
	if raw = strings.TrimSpace(raw); raw == "" {
		return nil, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return &t, nil
		}
	}
	return nil, apierror.Validation(field + " must be a date (2006-01-02) or an RFC 3339 timestamp")
}
