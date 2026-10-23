// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// slaDefinitionLegalNextStatus is the status transition graph (R14): a
// draft may be published or retired directly (abandoned with no
// publication); a published definition may only be retired, never moved
// back to draft -- unpublishing a definition that may already have live
// sla_instance rows tracking against it is a bigger decision than this
// admin screen makes silently, so it is deliberately not offered. retired
// is final, mirroring R34's "final states are final" for an instance.
var slaDefinitionLegalNextStatus = map[string]map[string]bool{
	"draft":     {"published": true, "retired": true},
	"published": {"retired": true},
	"retired":   {},
}

const (
	slaThresholdMin = 1
	slaThresholdMax = 1000
)

// SLADefinitionService is the admin-facing surface of sla_definition
// (section 6.1/6.1.1/6.1.2). It validates a predicate's shape (well-formed
// JSON, non-empty where required) but not yet a predicate's fields/
// operators against the enabled field catalog (R5, R7) -- that validator is
// a separate, larger piece (a recursive walk of the "all"/"any"/"not" tree
// against SLAFieldRegistryRepository) not yet built; flagged here rather
// than silently skipped.
type SLADefinitionService interface {
	Search(ctx context.Context, req domain.SearchSLADefinitionsRequest) (domain.SearchSLADefinitionsResponse, error)
	Get(ctx context.Context, id string) (domain.SLADefinition, error)
	Create(ctx context.Context, req domain.UpsertSLADefinitionRequest) (domain.SLADefinition, error)
	Update(ctx context.Context, id string, req domain.UpsertSLADefinitionRequest) (domain.SLADefinition, error)
	SetStatus(ctx context.Context, id string, req domain.SetSLADefinitionStatusRequest) (domain.SLADefinition, error)
}

type slaDefinitionService struct {
	repo       repository.SLADefinitionRepository
	access     AccessService
	adminRoles []string
}

// NewSLADefinitionService constructs an SLADefinitionService. adminRoles is
// AUTH_ADMIN_ROLES.
func NewSLADefinitionService(repo repository.SLADefinitionRepository, access AccessService, adminRoles []string) SLADefinitionService {
	return &slaDefinitionService{repo: repo, access: access, adminRoles: adminRoles}
}

const (
	slaDefinitionDefaultLimit = 20
	slaDefinitionMaxLimit     = 50
)

func (s *slaDefinitionService) Search(ctx context.Context, req domain.SearchSLADefinitionsRequest) (domain.SearchSLADefinitionsResponse, error) {
	if err := requireSLAAdmin(ctx, s.access, s.adminRoles); err != nil {
		return domain.SearchSLADefinitionsResponse{}, err
	}
	if req.Pagination.Limit <= 0 {
		req.Pagination.Limit = slaDefinitionDefaultLimit
	}
	if req.Pagination.Limit > slaDefinitionMaxLimit {
		return domain.SearchSLADefinitionsResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("limit cannot exceed %d", slaDefinitionMaxLimit)}
	}
	if req.Pagination.Offset < 0 {
		req.Pagination.Offset = 0
	}
	defs, total, err := s.repo.Search(ctx, req.WorkItemType, req.Status, req.Pagination.Limit, req.Pagination.Offset)
	if err != nil {
		return domain.SearchSLADefinitionsResponse{}, err
	}
	return domain.SearchSLADefinitionsResponse{Definitions: defs, Total: total, Limit: req.Pagination.Limit, Offset: req.Pagination.Offset}, nil
}

func (s *slaDefinitionService) Get(ctx context.Context, id string) (domain.SLADefinition, error) {
	if err := requireSLAAdmin(ctx, s.access, s.adminRoles); err != nil {
		return domain.SLADefinition{}, err
	}
	if id == "" {
		return domain.SLADefinition{}, &apierror.ValidationError{Msg: "definition id is required"}
	}
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.SLADefinition{}, err
	}
	return s.repo.Get(ctx, id)
}

func validateSLADefinitionRequest(req domain.UpsertSLADefinitionRequest) error {
	if req.Name == "" {
		return &apierror.ValidationError{Msg: "name is required"}
	}
	if req.WorkItemType == "" {
		return &apierror.ValidationError{Msg: "workItemType is required"}
	}
	if req.CalendarID == "" {
		return &apierror.ValidationError{Msg: "calendarId is required"}
	}
	if err := validateUUIDs("calendarId", []string{req.CalendarID}); err != nil {
		return err
	}
	switch req.TimezoneSource {
	case "definition":
		if req.DefinitionTimezone == nil || *req.DefinitionTimezone == "" {
			return &apierror.ValidationError{Msg: "definitionTimezone is required when timezoneSource is \"definition\""}
		}
	case "account":
		if req.AccountTimezoneFallback == nil || *req.AccountTimezoneFallback == "" {
			return &apierror.ValidationError{Msg: "accountTimezoneFallback is required when timezoneSource is \"account\""}
		}
	default:
		return &apierror.ValidationError{Msg: `timezoneSource must be "definition" or "account"`}
	}
	switch req.StartMode {
	case "now", "retroactive", "carry_over":
	default:
		return &apierror.ValidationError{Msg: `startMode must be "now", "retroactive" or "carry_over"`}
	}
	if req.TargetLabel == "" {
		return &apierror.ValidationError{Msg: "targetLabel is required"}
	}
	if req.DurationSeconds <= 0 {
		return &apierror.ValidationError{Msg: "durationSeconds must be greater than 0"}
	}
	if repository.IsJSONNull(req.StartCondition) || !json.Valid(req.StartCondition) {
		return &apierror.ValidationError{Msg: "startCondition is required and must be well-formed JSON"}
	}
	for name, raw := range map[string]json.RawMessage{
		"pauseCondition": req.PauseCondition, "stopCondition": req.StopCondition, "cancelCondition": req.CancelCondition,
	} {
		if len(raw) > 0 && !json.Valid(raw) {
			return &apierror.ValidationError{Msg: name + " must be well-formed JSON"}
		}
	}
	if err := validateSLAThresholds(req.ThresholdPercentages); err != nil {
		return err
	}
	if err := validateUUIDs("alsoCompletesDefinitionIds", req.AlsoCompletesDefinitionIDs); err != nil {
		return err
	}
	return nil
}

// validateSLAThresholds enforces R39: every value greater than 0, unique
// within the definition, and the set always includes 100 (the breach
// threshold).
func validateSLAThresholds(percentages []int) error {
	if len(percentages) == 0 {
		return &apierror.ValidationError{Msg: "thresholdPercentages must include at least 100"}
	}
	seen := map[int]bool{}
	hasHundred := false
	for _, p := range percentages {
		if p <= slaThresholdMin-1 || p > slaThresholdMax {
			return &apierror.ValidationError{Msg: fmt.Sprintf("thresholdPercentages must be greater than 0 (and no more than %d)", slaThresholdMax)}
		}
		if seen[p] {
			return &apierror.ValidationError{Msg: "thresholdPercentages must be unique"}
		}
		seen[p] = true
		if p == 100 {
			hasHundred = true
		}
	}
	if !hasHundred {
		return &apierror.ValidationError{Msg: "thresholdPercentages must include 100 (the breach threshold)"}
	}
	return nil
}

func (s *slaDefinitionService) Create(ctx context.Context, req domain.UpsertSLADefinitionRequest) (domain.SLADefinition, error) {
	if err := requireSLAAdmin(ctx, s.access, s.adminRoles); err != nil {
		return domain.SLADefinition{}, err
	}
	if err := validateSLADefinitionRequest(req); err != nil {
		return domain.SLADefinition{}, err
	}
	return s.repo.Create(ctx, req, slaActorEmail(ctx))
}

func (s *slaDefinitionService) Update(ctx context.Context, id string, req domain.UpsertSLADefinitionRequest) (domain.SLADefinition, error) {
	if err := requireSLAAdmin(ctx, s.access, s.adminRoles); err != nil {
		return domain.SLADefinition{}, err
	}
	if id == "" {
		return domain.SLADefinition{}, &apierror.ValidationError{Msg: "definition id is required"}
	}
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.SLADefinition{}, err
	}
	if err := validateSLADefinitionRequest(req); err != nil {
		return domain.SLADefinition{}, err
	}
	return s.repo.Update(ctx, id, req, slaActorEmail(ctx))
}

func (s *slaDefinitionService) SetStatus(ctx context.Context, id string, req domain.SetSLADefinitionStatusRequest) (domain.SLADefinition, error) {
	if err := requireSLAAdmin(ctx, s.access, s.adminRoles); err != nil {
		return domain.SLADefinition{}, err
	}
	if id == "" {
		return domain.SLADefinition{}, &apierror.ValidationError{Msg: "definition id is required"}
	}
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.SLADefinition{}, err
	}
	current, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.SLADefinition{}, err
	}
	legal, ok := slaDefinitionLegalNextStatus[current.Status]
	if !ok || !legal[req.Status] {
		return domain.SLADefinition{}, &apierror.ConflictError{Msg: fmt.Sprintf("cannot move an sla definition from %q to %q", current.Status, req.Status)}
	}
	// current.Status is passed as the expected status so the repository's own
	// UPDATE is conditioned on it (WHERE status = expected), not just this
	// read -- otherwise two concurrent requests (e.g. "Retire" and "Publish")
	// could both read "draft", both pass the check above, and the second
	// write would win unconditionally: a "retired" definition silently moved
	// back to "published", breaking "retired is final" (R34's same rule for
	// an instance). A row that no longer matches is a genuine race, reported
	// as a conflict rather than silently overwritten.
	return s.repo.SetStatus(ctx, id, current.Status, req.Status, slaActorEmail(ctx))
}
