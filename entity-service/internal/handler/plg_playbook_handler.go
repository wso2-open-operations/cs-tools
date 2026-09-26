package handler

import (
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// PlgPlaybookHandler serves the playbook templates.
type PlgPlaybookHandler struct {
	svc service.PlaybookService
}

// NewPlgPlaybookHandler wires the handler over its service.
func NewPlgPlaybookHandler(svc service.PlaybookService) *PlgPlaybookHandler {
	return &PlgPlaybookHandler{svc: svc}
}

// ListPlaybooks serves GET /plg/playbooks?product=CODE.
//
// A GET with one optional filter, where the contract originally sketched a POST
// search. One optional scalar does belong in a query string, and keeping it a
// GET means the playbook list stays cacheable.
func (h *PlgPlaybookHandler) ListPlaybooks(w http.ResponseWriter, r *http.Request) {
	playbooks, err := h.svc.List(r.Context(), r.URL.Query().Get("product"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeOK(w, map[string]any{"playbooks": playbooks})
}

// GetPlaybook serves GET /plg/playbooks/{playbookId}.
//
// NOTE: nothing calls this. The playbook editor takes everything from the list
// endpoint instead. It is listed in plg-docs/ENTITY-SERVICE-CONTRACT.md as
// needing either a caller or deletion.
func (h *PlgPlaybookHandler) GetPlaybook(w http.ResponseWriter, r *http.Request) {
	playbook, err := h.svc.Get(r.Context(), r.PathValue("playbookId"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeOK(w, playbook)
}
