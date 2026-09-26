package repository

import (
	"encoding/json"
	"fmt"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// optionsArg renders a task's checklist options for storage.
//
// nil rather than an empty array for a task that has none: the CHECK constraint
// requires the column to be NULL on every type but CHECKLIST, and "[]" is not
// NULL.
func optionsArg(opts []domain.ChecklistOption) (any, error) {
	if len(opts) == 0 {
		return nil, nil
	}
	encoded, err := json.Marshal(opts)
	if err != nil {
		return nil, fmt.Errorf("encode checklist options: %w", err)
	}
	return string(encoded), nil
}

// scanOptions decodes the stored options, tolerating NULL.
func scanOptions(raw []byte) ([]domain.ChecklistOption, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var opts []domain.ChecklistOption
	if err := json.Unmarshal(raw, &opts); err != nil {
		return nil, fmt.Errorf("decode checklist options: %w", err)
	}
	return opts, nil
}
