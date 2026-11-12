package handler

import (
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// PlgPairingHandler serves the product tab and the registrations panel.
type PlgPairingHandler struct {
	svc service.PairingService
}

// NewPlgPairingHandler wires the handler over its service.
func NewPlgPairingHandler(svc service.PairingService) *PlgPairingHandler {
	return &PlgPairingHandler{svc: svc}
}

// GetPairing serves GET /plg/organizations/{organizationId}/products/{productCode}.
//
// Addressed as a product under an organisation: a pairing is the unit of work,
// not a resource with a life of its own. The response is the whole
// product tab — the pairing, its playbook runs with their tasks, the notes and
// the lifecycle history — because that is one screen and one round trip.
func (h *PlgPairingHandler) GetPairing(w http.ResponseWriter, r *http.Request) {
	detail, err := h.svc.Get(r.Context(),
		r.PathValue("organizationId"), r.PathValue("productCode"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeOK(w, detail)
}

// SearchRegistrations serves POST /plg/registrations/search.
//
// A registration is a pairing that has not been acknowledged. There is no flag:
// the filter is acknowledged_on IS NULL, which is why "new" cannot drift out of
// step with anything.
func (h *PlgPairingHandler) SearchRegistrations(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchRegistrationsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	result, err := h.svc.SearchRegistrations(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeOK(w, result)
}

// AggregateRegistrations serves POST /plg/registrations/aggregate: the
// registrations the search's filters match, counted per their
// organisation's country, in the AggregateResponse shape.
func (h *PlgPairingHandler) AggregateRegistrations(w http.ResponseWriter, r *http.Request) {
	var req domain.AggregateRegistrationsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	result, err := h.svc.AggregateRegistrations(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeOK(w, result)
}

// RegistrationVolumeTrend serves POST /plg/registrations/volume-trend: the
// registrations the search's filters match, counted per day, week or month
// of a window, in the case trend's response shape (one series, "Registered").
func (h *PlgPairingHandler) RegistrationVolumeTrend(w http.ResponseWriter, r *http.Request) {
	var req domain.RegistrationVolumeTrendRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	result, err := h.svc.RegistrationVolumeTrend(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeOK(w, result)
}
