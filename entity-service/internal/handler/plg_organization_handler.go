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

// PatchOrganization serves PATCH /plg/organizations/{organizationId}.
//
// One of the four simple writes: a single statement, no precondition. The
// organisation's owner is the only field the portal writes at this level —
// lifecycle stage belongs to the pairing, not the customer.
func (h *PlgOrganizationHandler) PatchOrganization(w http.ResponseWriter, r *http.Request) {
	var req domain.PatchOrganizationRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.ID = r.PathValue("organizationId")

	org, err := h.svc.Patch(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeOK(w, org)
}
