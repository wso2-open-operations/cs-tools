package handler

import (
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// createPlaybookBody and patchPlaybookBody carry the caller alongside the
// request the service already understood.
//
// Embedded rather than added to the domain types, matching
// plg_pairing_writes.go: actorId is a wire concern between the BFF and this
// service -- the BFF resolves who the caller is, because this service has no
// notion of a current user -- while the domain request is the same whoever
// made it.
//
// They have to be declared fields rather than tolerated extras: decodeRequest
// sets DisallowUnknownFields, so a body carrying actorId against a struct
// without it is a 400, not a silently dropped field.
type createPlaybookBody struct {
	domain.CreatePlaybookRequest
	ActorID string `json:"actorId"`
}

type patchPlaybookBody struct {
	domain.PatchPlaybookRequest
	ActorID string `json:"actorId"`
}

type replaceTasksBody struct {
	domain.ReplacePlaybookTasksRequest
	ActorID string `json:"actorId"`
}

// CreatePlaybook serves POST /plg/products/{productCode}/playbooks. (W7)
//
// Atomic: the playbook and its tasks land together. The task list arrives
// already normalised — the BFF has injected the bookends and validated the
// codes before this is called.
func (h *PlgPlaybookHandler) CreatePlaybook(w http.ResponseWriter, r *http.Request) {
	var body createPlaybookBody
	if !decodeRequest(w, r, &body) {
		return
	}
	req := body.CreatePlaybookRequest
	req.ProductCode = r.PathValue("productCode")
	res, err := h.svc.Create(r.Context(), req, body.ActorID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeOK(w, res)
}

// PatchPlaybook serves PATCH /plg/playbooks/{playbookId}. (S3)
func (h *PlgPlaybookHandler) PatchPlaybook(w http.ResponseWriter, r *http.Request) {
	var body patchPlaybookBody
	if !decodeRequest(w, r, &body) {
		return
	}
	req := body.PatchPlaybookRequest
	req.ID = r.PathValue("playbookId")
	res, err := h.svc.Patch(r.Context(), req, body.ActorID)
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
	var body replaceTasksBody
	if !decodeRequest(w, r, &body) {
		return
	}
	req := body.ReplacePlaybookTasksRequest
	req.PlaybookID = r.PathValue("playbookId")
	res, err := h.svc.ReplaceTasks(r.Context(), req, body.ActorID)
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
