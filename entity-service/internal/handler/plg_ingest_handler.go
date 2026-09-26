package handler

import (
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// PlgIngestHandler lands registrations and records what could not be landed.
type PlgIngestHandler struct {
	svc service.IngestService
}

// NewPlgIngestHandler wires the handler over its service.
func NewPlgIngestHandler(svc service.IngestService) *PlgIngestHandler {
	return &PlgIngestHandler{svc: svc}
}

// Register serves POST /plg/registrations/ingest. (I1)
//
// 202, not 201: each registration lands in its own transaction and the batch
// reports per-record outcomes, so "accepted" is the honest status even when one
// of them failed. The caller reads the results array to find out.
func (h *PlgIngestHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req domain.IngestRegistrationsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	res, err := h.svc.Register(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeStatus(w, http.StatusAccepted, res)
}

// RecordFailure serves POST /plg/ingest-failures. (I2)
//
// The queue deletes on consume — no ack, no redelivery — so an event that
// cannot be turned into a registration has nowhere left to exist. A non-empty
// plg_ingest_failure means registrations arrived and were not recorded.
func (h *PlgIngestHandler) RecordFailure(w http.ResponseWriter, r *http.Request) {
	var req domain.RecordIngestFailureRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	res, err := h.svc.RecordFailure(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeStatus(w, http.StatusCreated, res)
}
