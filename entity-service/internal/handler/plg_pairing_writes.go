package handler

import (
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The pairing write handlers.
//
// Each one reads the actor from the body rather than from a header: a resolved
// `"user".id` that the BFF has already looked up. Putting it in the body keeps
// this service free of any notion of "the current user" — it has no
// session, no token and no header to trust, only an id a caller vouched for.

// acknowledgeBody is the body of POST /plg/registrations/{orgPlatformId}/acknowledge.
type acknowledgeBody struct {
	OwnerID *string `json:"ownerId"`
	ActorID string  `json:"actorId"`
}

// Acknowledge serves POST /plg/registrations/{orgPlatformId}/acknowledge. (W1)
func (h *PlgPairingHandler) Acknowledge(w http.ResponseWriter, r *http.Request) {
	var body acknowledgeBody
	if !decodeRequest(w, r, &body) {
		return
	}
	res, err := h.svc.Acknowledge(r.Context(), domain.AcknowledgeRequest{
		OrgPlatformID: r.PathValue("orgPlatformId"),
		OwnerID:       body.OwnerID,
	}, body.ActorID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	// RowsAffected 0 is a 200 here, not a 409. It means the precondition no
	// longer held; what that signifies is the BFF's call.
	writeOK(w, res)
}

// patchPairingBody is the body of PATCH /plg/pairings/{organizationId}/{productCode}.
type patchPairingBody struct {
	ExpectedStage  *domain.LifecycleStage `json:"expectedStage"`
	LifecycleStage *domain.LifecycleStage `json:"lifecycleStage"`
	// HealthState moves the second axis, independently of the stage. A request
	// may carry either, both or neither.
	HealthState            *domain.HealthState      `json:"healthState"`
	Reason                 *string                  `json:"reason"`
	SubscriptionTier       *domain.SubscriptionTier `json:"subscriptionTier"`
	TrialEndDate           *string                  `json:"trialEndDate"`
	ClearTrialEndDate      bool                     `json:"clearTrialEndDate"`
	TrialExtendedDate      *string                  `json:"trialExtendedDate"`
	ClearTrialExtendedDate bool                     `json:"clearTrialExtendedDate"`
	ActorID                string                   `json:"actorId"`
}

// PatchPairing serves PATCH /plg/organizations/{organizationId}/products/{productCode}. (W2)
func (h *PlgPairingHandler) PatchPairing(w http.ResponseWriter, r *http.Request) {
	var body patchPairingBody
	if !decodeRequest(w, r, &body) {
		return
	}
	res, err := h.svc.PatchPairing(r.Context(), domain.PatchOrgPlatformRequest{
		OrganizationID:         r.PathValue("organizationId"),
		ProductCode:            r.PathValue("productCode"),
		ExpectedStage:          body.ExpectedStage,
		LifecycleStage:         body.LifecycleStage,
		HealthState:            body.HealthState,
		Reason:                 body.Reason,
		SubscriptionTier:       body.SubscriptionTier,
		TrialEndDate:           body.TrialEndDate,
		ClearTrialEndDate:      body.ClearTrialEndDate,
		TrialExtendedDate:      body.TrialExtendedDate,
		ClearTrialExtendedDate: body.ClearTrialExtendedDate,
	}, body.ActorID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeOK(w, res)
}

// attachPlaybookBody is the body of POST …/playbook-runs.
type attachPlaybookBody struct {
	PlaybookID string `json:"playbookId"`
	ActorID    string `json:"actorId"`
}

// AttachPlaybook serves POST /plg/organizations/{organizationId}/products/{productCode}/playbook-runs. (W3)
func (h *PlgPairingHandler) AttachPlaybook(w http.ResponseWriter, r *http.Request) {
	var body attachPlaybookBody
	if !decodeRequest(w, r, &body) {
		return
	}
	if err := h.svc.AttachPlaybook(r.Context(), domain.AttachPlaybookRequest{
		OrganizationID: r.PathValue("organizationId"),
		ProductCode:    r.PathValue("productCode"),
		PlaybookID:     body.PlaybookID,
	}, body.ActorID); err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeOK(w, domain.WriteResult{RowsAffected: 1})
}

// DetachRun serves DELETE /plg/playbook-runs/{playbookRunId}. (W4)
func (h *PlgPairingHandler) DetachRun(w http.ResponseWriter, r *http.Request) {
	res, orgID, code, err := h.svc.DetachRun(r.Context(), r.PathValue("playbookRunId"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeOK(w, map[string]any{
		"rowsAffected": res.RowsAffected, "organizationId": orgID, "productCode": code,
	})
}

// patchRunTaskBody is the body of PATCH /plg/playbook-run-tasks/{taskId}.
type patchRunTaskBody struct {
	BoolValue    *bool     `json:"boolValue"`
	NumberValue  *float64  `json:"numberValue"`
	TextValue    *string   `json:"textValue"`
	CheckedCodes *[]string `json:"checkedCodes"`
	ClearValue   bool      `json:"clearValue"`
	ActorID      string    `json:"actorId"`
}

// PatchRunTask serves PATCH /plg/playbook-run-tasks/{taskId}. (W5)
func (h *PlgPairingHandler) PatchRunTask(w http.ResponseWriter, r *http.Request) {
	var body patchRunTaskBody
	if !decodeRequest(w, r, &body) {
		return
	}
	orgID, code, err := h.svc.PatchRunTask(r.Context(), domain.PatchRunTaskRequest{
		ID:           r.PathValue("taskId"),
		BoolValue:    body.BoolValue,
		NumberValue:  body.NumberValue,
		TextValue:    body.TextValue,
		CheckedCodes: body.CheckedCodes,
		ClearValue:   body.ClearValue,
	}, body.ActorID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeOK(w, map[string]any{
		"rowsAffected": 1, "organizationId": orgID, "productCode": code,
	})
}

// createNoteBody is the body of POST …/notes.
type createNoteBody struct {
	Body    string `json:"body"`
	ActorID string `json:"actorId"`
}

// CreateNote serves POST /plg/organizations/{organizationId}/products/{productCode}/notes. (S2)
func (h *PlgPairingHandler) CreateNote(w http.ResponseWriter, r *http.Request) {
	var body createNoteBody
	if !decodeRequest(w, r, &body) {
		return
	}
	if err := h.svc.CreateNote(r.Context(), domain.CreateNoteRequest{
		OrganizationID: r.PathValue("organizationId"),
		ProductCode:    r.PathValue("productCode"),
		Body:           body.Body,
	}, body.ActorID); err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeOK(w, domain.WriteResult{RowsAffected: 1})
}

// updateNoteBody is the body of PATCH /plg/notes/{noteId}.
type updateNoteBody struct {
	Body    string `json:"body"`
	ActorID string `json:"actorId"`
}

// UpdateNote serves PATCH /plg/notes/{noteId}. (W6)
func (h *PlgPairingHandler) UpdateNote(w http.ResponseWriter, r *http.Request) {
	var body updateNoteBody
	if !decodeRequest(w, r, &body) {
		return
	}
	res, orgID, code, err := h.svc.UpdateNote(r.Context(), domain.UpdateNoteRequest{
		ID:   r.PathValue("noteId"),
		Body: body.Body,
	}, body.ActorID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	// 0 means the caller is not the author. The BFF answers 403; this does not.
	writeOK(w, map[string]any{
		"rowsAffected": res.RowsAffected, "organizationId": orgID, "productCode": code,
	})
}

// GetRunTaskShape serves GET /plg/playbook-run-tasks/{taskId}/shape.
func (h *PlgPairingHandler) GetRunTaskShape(w http.ResponseWriter, r *http.Request) {
	shape, err := h.svc.RunTaskShape(r.Context(), r.PathValue("taskId"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeOK(w, shape)
}

// LocatePairing serves GET /plg/pairings/{orgPlatformId}/location.
func (h *PlgPairingHandler) LocatePairing(w http.ResponseWriter, r *http.Request) {
	loc, err := h.svc.LocatePairing(r.Context(), r.PathValue("orgPlatformId"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeOK(w, loc)
}
