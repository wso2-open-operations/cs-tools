package handler

import (
	"encoding/json"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// PlgReferenceHandler serves PLG's reference data.
type PlgReferenceHandler struct {
	svc service.ReferenceService
}

// NewPlgReferenceHandler wires the handler over its service.
func NewPlgReferenceHandler(svc service.ReferenceService) *PlgReferenceHandler {
	return &PlgReferenceHandler{svc: svc}
}

// ListProducts serves GET /plg/products.
//
// A GET rather than a POST …/search, unlike most of this slice: there are five
// rows, there is nothing to filter, and entity-service reserves the search
// shape for things that take filters.
func (h *PlgReferenceHandler) ListProducts(w http.ResponseWriter, r *http.Request) {
	products, err := h.svc.ListProducts(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeOK(w, map[string]any{"products": products})
}

// Lifecycle serves GET /plg/lifecycle — the nine stages and the seven paths.
func (h *PlgReferenceHandler) Lifecycle(w http.ResponseWriter, r *http.Request) {
	cat, err := h.svc.LifecycleCatalogue(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeOK(w, cat)
}

// writeOK encodes a 200 response.
//
// Declared here rather than in the copied decode.go, which has no success
// writer — entity-service's handlers set the header and encode inline, three
// lines at a time. One helper is the same thing said once.
func writeOK(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(body)
}

// writeStatus encodes a response at a status other than 200.
func writeStatus(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
