package handler

import (
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// CreatePlaybook serves POST /plg/products/{productCode}/playbooks. (W7)
//
// Atomic: the playbook and its tasks land together. The task list arrives
// already normalised — the BFF has injected the bookends and validated the
// codes before this is called.
func (h *PlgPlaybookHandler) CreatePlaybook(w http.ResponseWriter, r *http.Request) {
	var req domain.CreatePlaybookRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.ProductCode = r.PathValue("productCode")
	res, err := h.svc.Create(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeOK(w, res)
}

// PatchPlaybook serves PATCH /plg/playbooks/{playbookId}. (S3)
func (h *PlgPlaybookHandler) PatchPlaybook(w http.ResponseWriter, r *http.Request) {
	var req domain.PatchPlaybookRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.ID = r.PathValue("playbookId")
	res, err := h.svc.Patch(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeOK(w, res)
}

// ReplacePlaybookTasks serves PUT /plg/playbooks/{playbookId}/tasks. (W8)
//
// The whole set is replaced in one transaction. uq_plg_playbook_task_seq is
// DEFERRABLE INITIALLY DEFERRED, which is what lets a reorder happen without
// tripping over itself mid-transaction.
func (h *PlgPlaybookHandler) ReplacePlaybookTasks(w http.ResponseWriter, r *http.Request) {
	var req domain.ReplacePlaybookTasksRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.PlaybookID = r.PathValue("playbookId")
	res, err := h.svc.ReplaceTasks(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeOK(w, res)
}

// DeletePlaybook serves DELETE /plg/playbooks/{playbookId}. (S4)
func (h *PlgPlaybookHandler) DeletePlaybook(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.Delete(r.Context(), r.PathValue("playbookId"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeOK(w, res)
}
