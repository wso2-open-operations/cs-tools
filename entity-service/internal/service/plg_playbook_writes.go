package service

import (
	"context"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The playbook-template write surface (contract W7, W8, S3, S4).
//
// NO BOOKEND LOGIC HERE. Injecting the two reserved bookend tasks, refusing to
// remove or retype them, normalising task codes and validating checklist
// options are all in the BFF: they are PLG's notion of a
// playbook, not a property of the tables.
//
// What is left is what the data owner legitimately enforces: ids are UUIDs,
// enums are known, and the schema's own constraints (fk_plg_playbook_path,
// chk_plg_playbook_task_options, uq_plg_playbook_task_seq) do the rest. A
// playbook aimed at a stage that carries none is refused by Postgres, not by a
// rule written twice.

// PlaybookWriter is the write half of the playbook templates.
type PlaybookWriter interface {
	Create(ctx context.Context, req domain.CreatePlaybookRequest) (domain.CreatePlaybookResult, error)
	Patch(ctx context.Context, req domain.PatchPlaybookRequest) (domain.WriteResult, error)
	ReplaceTasks(ctx context.Context, req domain.ReplacePlaybookTasksRequest) (domain.WriteResult, error)
	Delete(ctx context.Context, id string) (domain.WriteResult, error)
}

func (s *playbookService) Create(ctx context.Context, req domain.CreatePlaybookRequest) (domain.CreatePlaybookResult, error) {
	if !domain.ValidLifecycleStage[req.LifecycleStage] {
		return domain.CreatePlaybookResult{}, invalidEnum("lifecycleStage", string(req.LifecycleStage))
	}
	if !domain.ValidPlaybookType[req.PlaybookType] {
		return domain.CreatePlaybookResult{}, invalidEnum("playbookType", string(req.PlaybookType))
	}
	// Nothing progresses out of ABANDONED and a pairing there is gone, so a
	// playbook of either kind could never run. The schema refuses it too; this
	// is the readable half.
	if req.LifecycleStage == domain.StageAbandoned {
		return domain.CreatePlaybookResult{}, invalidField(
			"a playbook cannot sit at ABANDONED: nothing progresses out of it and a pairing there is gone")
	}
	for i := range req.Tasks {
		if !domain.ValidTaskValueType[req.Tasks[i].ValueType] {
			return domain.CreatePlaybookResult{}, invalidEnum("tasks.valueType", string(req.Tasks[i].ValueType))
		}
	}
	id, err := s.repo.Create(ctx, req)
	if err != nil {
		return domain.CreatePlaybookResult{}, err
	}
	return domain.CreatePlaybookResult{PlaybookID: id}, nil
}

func (s *playbookService) Patch(ctx context.Context, req domain.PatchPlaybookRequest) (domain.WriteResult, error) {
	if err := validateUUID("playbookId", req.ID); err != nil {
		return domain.WriteResult{}, err
	}
	if err := s.repo.Patch(ctx, req); err != nil {
		return domain.WriteResult{}, err
	}
	return domain.WriteResult{RowsAffected: 1}, nil
}

func (s *playbookService) ReplaceTasks(ctx context.Context, req domain.ReplacePlaybookTasksRequest) (domain.WriteResult, error) {
	if err := validateUUID("playbookId", req.PlaybookID); err != nil {
		return domain.WriteResult{}, err
	}
	for i := range req.Tasks {
		if !domain.ValidTaskValueType[req.Tasks[i].ValueType] {
			return domain.WriteResult{}, invalidEnum("tasks.valueType", string(req.Tasks[i].ValueType))
		}
	}
	if err := s.repo.ReplaceTasks(ctx, req); err != nil {
		return domain.WriteResult{}, err
	}
	return domain.WriteResult{RowsAffected: len(req.Tasks)}, nil
}

func (s *playbookService) Delete(ctx context.Context, id string) (domain.WriteResult, error) {
	if err := validateUUID("playbookId", id); err != nil {
		return domain.WriteResult{}, err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return domain.WriteResult{}, err
	}
	return domain.WriteResult{RowsAffected: 1}, nil
}
