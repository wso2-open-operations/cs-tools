package service

import (
	"context"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The pairing write surface.
//
// Every method here returns what the write did, never what it means. A zero
// RowsAffected is returned as a zero RowsAffected — not a 409, not a 403 —
// because which of those it is depends on a PLG rule, and PLG's rules live in
// the BFF. See plg-docs/ENTITY-SERVICE-CONTRACT.md.
//
// What this layer does do is validate: ids are UUIDs, enums are known. That is
// entity-service's own precedent for every entity it owns, and it is the reason
// a malformed request fails here with a named field rather than in Postgres
// with a cast error.

// PairingWriter is the write half of the product tab.
type PairingWriter interface {
	Acknowledge(ctx context.Context, req domain.AcknowledgeRequest, actorID string) (domain.AcknowledgeResult, error)
	PatchPairing(ctx context.Context, req domain.PatchOrgPlatformRequest, actorID string) (domain.PatchPairingResult, error)
	AttachPlaybook(ctx context.Context, req domain.AttachPlaybookRequest, actorID string) error
	DetachRun(ctx context.Context, runID string) (domain.WriteResult, string, string, error)
	PatchRunTask(ctx context.Context, req domain.PatchRunTaskRequest, actorID string) (string, string, error)
	CreateNote(ctx context.Context, req domain.CreateNoteRequest, actorID string) error
	UpdateNote(ctx context.Context, req domain.UpdateNoteRequest, actorID string) (domain.WriteResult, string, string, error)
	RunTaskShape(ctx context.Context, taskID string) (domain.RunTaskShape, error)
	LocatePairing(ctx context.Context, orgPlatformID string) (domain.PairingLocation, error)
}

// validateActor checks the caller id every attributed write carries.
//
// Worth its own helper because it is the one field every write here has in
// common, and because an actor that is not a UUID must never reach the
// database as one.
func validateActor(actorID string) error {
	return validateUUID("actorId", actorID)
}

func (s *pairingService) Acknowledge(ctx context.Context, req domain.AcknowledgeRequest, actorID string) (domain.AcknowledgeResult, error) {
	if err := validateUUID("orgPlatformId", req.OrgPlatformID); err != nil {
		return domain.AcknowledgeResult{}, err
	}
	if err := validateActor(actorID); err != nil {
		return domain.AcknowledgeResult{}, err
	}
	if req.OwnerID != nil && *req.OwnerID != "" {
		if err := validateUUID("ownerId", *req.OwnerID); err != nil {
			return domain.AcknowledgeResult{}, err
		}
	}
	return s.repo.Acknowledge(ctx, req, actorID)
}

func (s *pairingService) PatchPairing(ctx context.Context, req domain.PatchOrgPlatformRequest, actorID string) (domain.PatchPairingResult, error) {
	if err := validateUUID("organizationId", req.OrganizationID); err != nil {
		return domain.PatchPairingResult{}, err
	}
	if err := validateActor(actorID); err != nil {
		return domain.PatchPairingResult{}, err
	}
	if req.LifecycleStage != nil && !domain.ValidLifecycleStage[*req.LifecycleStage] {
		return domain.PatchPairingResult{}, invalidEnum("lifecycleStage", string(*req.LifecycleStage))
	}
	if req.ExpectedStage != nil && !domain.ValidLifecycleStage[*req.ExpectedStage] {
		return domain.PatchPairingResult{}, invalidEnum("expectedStage", string(*req.ExpectedStage))
	}
	if req.HealthState != nil && !domain.ValidHealthState[*req.HealthState] {
		return domain.PatchPairingResult{}, invalidEnum("healthState", string(*req.HealthState))
	}
	// The movement rule, checked here so the caller gets a sentence rather than
	// a trigger's message. The trigger is still the authority — see
	// plg_check_stage_move() — and if the two ever disagree it wins.
	if req.LifecycleStage != nil && req.ExpectedStage != nil &&
		!domain.CanMoveStage(*req.ExpectedStage, *req.LifecycleStage) {
		if *req.ExpectedStage == domain.StageAbandoned {
			return domain.PatchPairingResult{}, invalidField("a pairing cannot leave ABANDONED")
		}
		return domain.PatchPairingResult{}, invalidField(
			"a pairing moves forward only: " + string(*req.LifecycleStage) +
				" cannot follow " + string(*req.ExpectedStage))
	}
	if req.SubscriptionTier != nil && !domain.ValidSubscriptionTier[*req.SubscriptionTier] {
		return domain.PatchPairingResult{}, invalidEnum("subscriptionTier", string(*req.SubscriptionTier))
	}
	if err := parseDate("trialEndDate", req.TrialEndDate); err != nil {
		return domain.PatchPairingResult{}, err
	}
	if err := parseDate("trialExtendedDate", req.TrialExtendedDate); err != nil {
		return domain.PatchPairingResult{}, err
	}
	// Every tracked axis carries a reason. Checked here as well as in the BFF
	// because the slice is reachable by any caller with a network route to it,
	// and because the alternative is a NOT NULL violation surfacing as a 500 —
	// a true statement about the database and a useless one about the request.
	if (req.LifecycleStage != nil || req.HealthState != nil || req.SubscriptionTier != nil) &&
		(req.Reason == nil || strings.TrimSpace(*req.Reason) == "") {
		return domain.PatchPairingResult{}, invalidField(
			"a lifecycle, health or subscription change needs a reason")
	}
	return s.repo.Patch(ctx, req, actorID)
}

func (s *pairingService) AttachPlaybook(ctx context.Context, req domain.AttachPlaybookRequest, actorID string) error {
	if err := validateUUID("organizationId", req.OrganizationID); err != nil {
		return err
	}
	if err := validateUUID("playbookId", req.PlaybookID); err != nil {
		return err
	}
	if err := validateActor(actorID); err != nil {
		return err
	}
	return s.repo.AttachPlaybook(ctx, req, actorID)
}

func (s *pairingService) DetachRun(ctx context.Context, runID string) (domain.WriteResult, string, string, error) {
	if err := validateUUID("playbookRunId", runID); err != nil {
		return domain.WriteResult{}, "", "", err
	}
	return s.repo.DetachRun(ctx, runID)
}

func (s *pairingService) PatchRunTask(ctx context.Context, req domain.PatchRunTaskRequest, actorID string) (string, string, error) {
	if err := validateUUID("taskId", req.ID); err != nil {
		return "", "", err
	}
	if err := validateActor(actorID); err != nil {
		return "", "", err
	}
	// No precondition: recording a value is last-writer-wins. The
	// chk_plg_run_task_value_type constraint rejects a value that does not match
	// the task's type, and is_completed recomputes itself.
	return s.repo.PatchRunTask(ctx, req, actorID)
}

func (s *pairingService) CreateNote(ctx context.Context, req domain.CreateNoteRequest, actorID string) error {
	if err := validateUUID("organizationId", req.OrganizationID); err != nil {
		return err
	}
	if err := validateActor(actorID); err != nil {
		return err
	}
	if req.Body == "" {
		return invalidField("body must not be empty")
	}
	return s.repo.CreateNote(ctx, req, actorID)
}

func (s *pairingService) UpdateNote(ctx context.Context, req domain.UpdateNoteRequest, actorID string) (domain.WriteResult, string, string, error) {
	if err := validateUUID("noteId", req.ID); err != nil {
		return domain.WriteResult{}, "", "", err
	}
	if err := validateActor(actorID); err != nil {
		return domain.WriteResult{}, "", "", err
	}
	if req.Body == "" {
		return domain.WriteResult{}, "", "", invalidField("body must not be empty")
	}
	return s.repo.UpdateNote(ctx, req, actorID)
}

// RunTaskShape returns a task instance's declared type and options.
func (s *pairingService) RunTaskShape(ctx context.Context, taskID string) (domain.RunTaskShape, error) {
	if err := validateUUID("taskId", taskID); err != nil {
		return domain.RunTaskShape{}, err
	}
	return s.repo.RunTaskShape(ctx, taskID)
}

// LocatePairing resolves a pairing id to the organisation and product it is under.
func (s *pairingService) LocatePairing(ctx context.Context, orgPlatformID string) (domain.PairingLocation, error) {
	if err := validateUUID("orgPlatformId", orgPlatformID); err != nil {
		return domain.PairingLocation{}, err
	}
	orgID, code, err := s.repo.LocatePairing(ctx, orgPlatformID)
	if err != nil {
		return domain.PairingLocation{}, err
	}
	return domain.PairingLocation{OrganizationID: orgID, ProductCode: code}, nil
}
