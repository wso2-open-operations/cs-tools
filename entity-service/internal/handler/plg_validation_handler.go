package handler

import (
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// PlgValidationHandler serves the plg_registration_validation job.
type PlgValidationHandler struct {
	svc service.ValidationService
}

// NewPlgValidationHandler wires the handler over its service.
func NewPlgValidationHandler(svc service.ValidationService) *PlgValidationHandler {
	return &PlgValidationHandler{svc: svc}
}

// Queue serves POST /plg/registrations/validation-queue.
func (h *PlgValidationHandler) Queue(w http.ResponseWriter, r *http.Request) {
	var req domain.ValidationQueueRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	res, err := h.svc.Queue(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeStatus(w, http.StatusOK, res)
}

// Apply serves PUT /plg/pairings/{orgPlatformId}/registration-validation.
func (h *PlgValidationHandler) Apply(w http.ResponseWriter, r *http.Request) {
	var req domain.ApplyValidationRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.OrgPlatformID = r.PathValue("orgPlatformId")
	res, err := h.svc.Apply(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeStatus(w, http.StatusOK, res)
}
