package handler

import (
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// PlgOrganizationHandler serves the organisation endpoints.
type PlgOrganizationHandler struct {
	svc service.OrganizationService
}

// NewPlgOrganizationHandler wires the handler over its service.
func NewPlgOrganizationHandler(svc service.OrganizationService) *PlgOrganizationHandler {
	return &PlgOrganizationHandler{svc: svc}
}

// SearchOrganizations serves POST /plg/organizations/search.
func (h *PlgOrganizationHandler) SearchOrganizations(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchOrganizationsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	result, err := h.svc.Search(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeOK(w, result)
}

// GetOrganization serves GET /plg/organizations/{organizationId}.
func (h *PlgOrganizationHandler) GetOrganization(w http.ResponseWriter, r *http.Request) {
	org, err := h.svc.Get(r.Context(), r.PathValue("organizationId"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeOK(w, org)
}

// patchOrganizationBody carries the caller alongside the owner change, the same
// shape the pairing and playbook writes use. See createPlaybookBody for why
// actorId lives on the wire type rather than the domain request.
type patchOrganizationBody struct {
	domain.PatchOrganizationRequest
	ActorID string `json:"actorId"`
}

// PatchOrganization serves PATCH /plg/organizations/{organizationId}.
//
// One of the four simple writes: a single statement, no precondition. The
// organisation's owner is the only field the portal writes at this level —
// lifecycle stage belongs to the pairing, not the customer.
func (h *PlgOrganizationHandler) PatchOrganization(w http.ResponseWriter, r *http.Request) {
	var body patchOrganizationBody
	if !decodeRequest(w, r, &body) {
		return
	}
	req := body.PatchOrganizationRequest
	req.ID = r.PathValue("organizationId")

	org, err := h.svc.Patch(r.Context(), req, body.ActorID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeOK(w, org)
}
