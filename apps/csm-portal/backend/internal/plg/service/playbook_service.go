package service

import (
	"context"
	"strconv"
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/domain"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/repository"
)

// PlaybookService owns playbook templates — the Playbook Manager.
//
// The manager is meant to be visible only to a specific role. Roles are
// delegated to the CRM portal at user signup, so there is nothing to gate on
// here; the check belongs in whatever validates the caller's token.
type PlaybookService interface {
	ListAll(ctx context.Context) ([]domain.Playbook, error)
	ListByProduct(ctx context.Context, productCode string) ([]domain.Playbook, error)
	Get(ctx context.Context, id string) (*domain.Playbook, error)
	Create(ctx context.Context, req domain.CreatePlaybookRequest) (*domain.Playbook, error)
	Patch(ctx context.Context, req domain.PatchPlaybookRequest) (*domain.Playbook, error)
	ReplaceTasks(ctx context.Context, req domain.ReplacePlaybookTasksRequest) (*domain.Playbook, error)
	Delete(ctx context.Context, id string) error
}

type playbookService struct{ repo repository.PlaybookRepository }

// NewPlaybookService wires a PlaybookService over its repository.
func NewPlaybookService(repo repository.PlaybookRepository) PlaybookService {
	return &playbookService{repo: repo}
}

func (s *playbookService) ListAll(ctx context.Context) ([]domain.Playbook, error) {
	return s.repo.ListAll(ctx)
}

func (s *playbookService) ListByProduct(ctx context.Context, productCode string) ([]domain.Playbook, error) {
	if strings.TrimSpace(productCode) == "" {
		return nil, apierror.Validation("product is required")
	}
	return s.repo.ListByProduct(ctx, productCode)
}

func (s *playbookService) Get(ctx context.Context, id string) (*domain.Playbook, error) {
	if err := validateUUID("playbookId", id); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, id)
}

func (s *playbookService) Create(ctx context.Context, req domain.CreatePlaybookRequest) (*domain.Playbook, error) {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return nil, apierror.Validation("name is required")
	}
	if !domain.ValidLifecycleStage[req.LifecycleStage] {
		return nil, apierror.Validation("lifecycleStage is required and must be one of the six stages")
	}
	// The authoring sequence is product, then kind, then stage. A progressive
	// playbook carries the pairing to the next stage; a recovery one restores
	// its health in place; a sustaining one holds it steady. None needs a
	// destination named, because none has a choice of one.
	if !domain.ValidPlaybookType[req.PlaybookType] {
		return nil, apierror.Validation(
			"playbookType is required and must be PROGRESSIVE, RECOVERY or SUSTAINING")
	}
	if req.LifecycleStage == domain.StageAbandoned {
		return nil, apierror.Validation(
			"a playbook cannot sit at ABANDONED: nothing progresses out of it and a pairing there is gone")
	}
	req.Description = trimmedOrNil(req.Description)

	tasks, err := withBookends(req.Tasks)
	if err != nil {
		return nil, err
	}
	req.Tasks = tasks

	id, err := s.repo.Create(ctx, req)
	if err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, id)
}

func (s *playbookService) Patch(ctx context.Context, req domain.PatchPlaybookRequest) (*domain.Playbook, error) {
	if err := validateUUID("playbookId", req.ID); err != nil {
		return nil, err
	}
	if err := validateEnum("lifecycleStage", req.LifecycleStage, domain.ValidLifecycleStage); err != nil {
		return nil, err
	}
	if err := validateEnum("playbookType", req.PlaybookType, domain.ValidPlaybookType); err != nil {
		return nil, err
	}
	if req.Name == nil && req.Description == nil && req.LifecycleStage == nil &&
		req.PlaybookType == nil && req.Active == nil {
		return nil, apierror.Validation("at least one field must be supplied")
	}
	// A stage move is a stage move: nothing else has to be supplied alongside
	// it, because a playbook names no destination to keep consistent with it.
	if req.LifecycleStage != nil && *req.LifecycleStage == domain.StageAbandoned {
		return nil, apierror.Validation("a playbook cannot be moved to ABANDONED")
	}
	if req.Name != nil && strings.TrimSpace(*req.Name) == "" {
		return nil, apierror.Validation("name must not be empty")
	}

	if err := s.repo.Patch(ctx, req); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, req.ID)
}

func (s *playbookService) ReplaceTasks(ctx context.Context, req domain.ReplacePlaybookTasksRequest) (*domain.Playbook, error) {
	if err := validateUUID("playbookId", req.PlaybookID); err != nil {
		return nil, err
	}
	tasks, err := withBookends(req.Tasks)
	if err != nil {
		return nil, err
	}
	req.Tasks = tasks

	if err := s.repo.ReplaceTasks(ctx, req); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, req.PlaybookID)
}

func (s *playbookService) Delete(ctx context.Context, id string) error {
	if err := validateUUID("playbookId", id); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

// withBookends normalises a submitted task list so it always opens with
// "Initiate playbook" and ends with "Close playbook".
//
// The two are structural rather than editorial: completing the first is what
// makes a run active and completing the last is what closes it, which is what
// the work queue counts. So they are injected when absent and forced back into
// position when an editor moves them, rather than rejected — an editor never has
// to think about them and cannot drag them out of place. Their type is likewise
// forced to BOOLEAN, which the database also enforces with a CHECK.
func withBookends(tasks []domain.PlaybookTaskInput) ([]domain.PlaybookTaskInput, error) {
	middle := make([]domain.PlaybookTaskInput, 0, len(tasks)+2)
	var initiate, closing *domain.PlaybookTaskInput

	for i := range tasks {
		t := tasks[i]
		switch strings.ToUpper(strings.TrimSpace(t.Code)) {
		case domain.TaskCodeInitiate:
			initiate = &tasks[i]
		case domain.TaskCodeClose:
			closing = &tasks[i]
		default:
			middle = append(middle, t)
		}
	}

	if initiate == nil {
		initiate = &domain.PlaybookTaskInput{
			Code: domain.TaskCodeInitiate, Name: "Initiate playbook",
			Description: ptr("Marks the playbook as started. Everything after this is live work."),
		}
	}
	if closing == nil {
		closing = &domain.PlaybookTaskInput{
			Code: domain.TaskCodeClose, Name: "Close playbook",
			Description: ptr("Marks the playbook finished. No further tasks are expected."),
		}
	}
	initiate.Code, closing.Code = domain.TaskCodeInitiate, domain.TaskCodeClose
	initiate.ValueType, closing.ValueType = domain.ValueBoolean, domain.ValueBoolean
	initiate.Options, closing.Options = nil, nil

	out := make([]domain.PlaybookTaskInput, 0, len(middle)+2)
	out = append(out, *initiate)
	out = append(out, middle...)
	out = append(out, *closing)

	if err := validateTaskInputs(out); err != nil {
		return nil, err
	}
	return out, nil
}

// validateTaskInputs normalises codes and rejects duplicates. A playbook with no
// middle tasks is allowed — the two bookends alone are a valid playbook.
func validateTaskInputs(tasks []domain.PlaybookTaskInput) error {
	seen := make(map[string]bool, len(tasks))
	for i := range tasks {
		t := &tasks[i]
		t.Code = strings.ToUpper(strings.TrimSpace(t.Code))
		t.Name = strings.TrimSpace(t.Name)
		if t.Name == "" {
			return apierror.Validation("every task needs a name")
		}
		if t.Code == "" {
			t.Code = deriveCode(t.Name)
		}
		if !codeRe.MatchString(t.Code) {
			return apierror.Validation("task code " + t.Code +
				" must be UPPER_SNAKE_CASE, 2-64 characters, starting with a letter")
		}
		if seen[t.Code] {
			return apierror.Validation("duplicate task code: " + t.Code)
		}
		seen[t.Code] = true
		t.Description = trimmedOrNil(t.Description)
		if t.ValueType == "" {
			// Omitted means a tick box, which is what most tasks are.
			t.ValueType = domain.ValueBoolean
		}
		if !domain.ValidTaskValueType[t.ValueType] {
			// The list is built from the enum rather than written out, because a
			// hand-written one drifts: this message omitted SINGLE_SELECT for as
			// long as SINGLE_SELECT existed.
			return apierror.Validation("task " + t.Code + " has an invalid valueType: " +
				string(t.ValueType) + " (expected one of: " + valueTypeList() + ")")
		}
		if err := validateOptions(t); err != nil {
			return err
		}
	}
	return nil
}

// validateOptions checks a checklist's reasons, and that nothing else carries any.
//
// The database enforces the same rules, so this exists to say what is wrong in
// words rather than let a constraint name surface.
func validateOptions(t *domain.PlaybookTaskInput) error {
	// Both list types carry options, and neither anything else. CHECKLIST and
	// SINGLE_SELECT differ in how many answers may be chosen, not in how the
	// answers are authored, so the validation below is shared.
	if !domain.TypeNeedsOptions(t.ValueType) {
		if len(t.Options) > 0 {
			return apierror.Validation("task " + t.Code + " is a " + string(t.ValueType) +
				" task, so it cannot have a list of options")
		}
		t.Options = nil
		return nil
	}

	if len(t.Options) < domain.MinChecklistOptions {
		return apierror.Validation("task " + t.Code +
			" offers a list, so it needs at least " +
			strconv.Itoa(domain.MinChecklistOptions) + " options to choose between")
	}

	seen := make(map[string]bool, len(t.Options))
	for i := range t.Options {
		opt := &t.Options[i]
		opt.Label = strings.TrimSpace(opt.Label)
		opt.Code = strings.ToUpper(strings.TrimSpace(opt.Code))
		if opt.Label == "" {
			return apierror.Validation("every option on task " + t.Code + " needs a label")
		}
		if opt.Code == "" {
			opt.Code = deriveCode(opt.Label)
		}
		if !codeRe.MatchString(opt.Code) {
			return apierror.Validation("option code " + opt.Code + " on task " + t.Code +
				" must be UPPER_SNAKE_CASE, 2-64 characters, starting with a letter")
		}
		if seen[opt.Code] {
			return apierror.Validation("task " + t.Code + " has two options coded " + opt.Code)
		}
		seen[opt.Code] = true
	}
	return nil
}

// valueTypeList renders the accepted task value types for an error message,
// from the enum's own order so it cannot fall out of step with it.
func valueTypeList() string {
	names := make([]string, 0, len(domain.TaskValueTypeOrder))
	for _, t := range domain.TaskValueTypeOrder {
		names = append(names, string(t))
	}
	return strings.Join(names, ", ")
}
