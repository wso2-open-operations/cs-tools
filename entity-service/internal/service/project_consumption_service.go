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

// Package service is declared in interfaces.go.
package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// consumptionApplicationDescription is the description given to the Choreo
// application created for a project.
//
// The ServiceNow path returns a name and description chosen inside ServiceNow;
// on the Postgres path there is no such stored value, so it is derived here
// from the project itself. Keep it derived rather than caller-supplied: the
// application name is the only thing tying a Choreo application back to a
// project, and letting a caller pass an arbitrary one would make the mapping
// unverifiable.
const consumptionApplicationDescription = "Product consumption tracking application for project %s (%s)"

type projectConsumptionService struct {
	repo repository.ProjectConsumptionRepository
}

// NewProjectConsumptionService constructs a ProjectConsumptionService backed by
// the given repository.
func NewProjectConsumptionService(repo repository.ProjectConsumptionRepository) ProjectConsumptionService {
	return &projectConsumptionService{repo: repo}
}

// GetProjectConsumption implements ProjectConsumptionService.
func (s *projectConsumptionService) GetProjectConsumption(ctx context.Context, projectID string) (domain.ProjectConsumptionView, error) {
	if err := validateUUIDs("projectId", []string{projectID}); err != nil {
		return domain.ProjectConsumptionView{}, err
	}

	state, name, key, err := s.repo.Get(ctx, projectID)
	if err != nil {
		return domain.ProjectConsumptionView{}, err
	}
	return toProjectConsumptionView(state, name, key), nil
}

// UpdateProjectConsumption implements ProjectConsumptionService.
//
// The status may only move forward. The provisioning flow is resumable — a
// caller reads the current status and runs the steps above it — so an
// out-of-order write is always either a stale retry or a bug, and applying it
// would re-run side-effecting steps: the Choreo application would be created a
// second time for a customer who already has one. A status that is not ahead of
// what is stored is therefore a no-op that returns the current state, not an
// error, so that a retrying caller converges instead of failing.
func (s *projectConsumptionService) UpdateProjectConsumption(ctx context.Context, projectID string, req domain.UpdateProjectConsumptionRequest) (domain.UpdateProjectConsumptionResponse, error) {
	if err := validateUUIDs("projectId", []string{projectID}); err != nil {
		return domain.UpdateProjectConsumptionResponse{}, err
	}

	status := domain.ConsumptionStatus(req.Status)
	if !status.Valid() {
		return domain.UpdateProjectConsumptionResponse{}, &apierror.ValidationError{
			Msg: fmt.Sprintf("status must be between %d and %d", domain.ConsumptionStatusPending, domain.ConsumptionStatusGeneratedSecretKeys),
		}
	}
	if err := validateConsumptionArtefacts(status, req); err != nil {
		return domain.UpdateProjectConsumptionResponse{}, err
	}

	next := domain.ProjectConsumption{
		Status:              status,
		ChoreoApplicationID: req.ApplicationID,
		ConsumerKey:         req.ConsumerKey,
		ConsumerSecret:      req.ConsumerSecret,
		PrimarySecretKey:    req.PrimarySecretKey,
		SecondarySecretKey:  req.SecondarySecretKey,
	}

	_, err := s.repo.Upsert(ctx, projectID, next)
	switch {
	case errors.Is(err, repository.ErrConsumptionStatusStale):
		// Another caller already advanced past this step. Report what is
		// actually stored so the caller can resume from there.
		view, getErr := s.GetProjectConsumption(ctx, projectID)
		if getErr != nil {
			return domain.UpdateProjectConsumptionResponse{}, getErr
		}
		return domain.UpdateProjectConsumptionResponse{
			Message: "project consumption status already at or beyond the requested status",
			Result:  view,
		}, nil
	case err != nil:
		return domain.UpdateProjectConsumptionResponse{}, err
	}

	// Re-read rather than mapping the write's own RETURNING row: the response
	// carries the project's name and key, which the write does not select, and
	// this keeps a single definition of the view's shape.
	view, err := s.GetProjectConsumption(ctx, projectID)
	if err != nil {
		return domain.UpdateProjectConsumptionResponse{}, err
	}
	return domain.UpdateProjectConsumptionResponse{
		Message: "project consumption updated",
		Result:  view,
	}, nil
}

// validateConsumptionArtefacts checks that a caller recording a step has
// actually supplied that step's output.
//
// The same rules exist as CHECK constraints on the table. They are repeated
// here so the caller gets a specific 400 naming the missing field instead of a
// constraint violation, and so the rule is visible at the layer that owns the
// state machine's meaning.
func validateConsumptionArtefacts(status domain.ConsumptionStatus, req domain.UpdateProjectConsumptionRequest) error {
	missing := func(field string) error {
		return &apierror.ValidationError{Msg: fmt.Sprintf("%s is required when status is %d", field, status)}
	}

	switch status {
	case domain.ConsumptionStatusCreated:
		if isBlank(req.ApplicationID) {
			return missing("applicationId")
		}
	case domain.ConsumptionStatusGeneratedCredentials:
		if isBlank(req.ConsumerKey) {
			return missing("consumerKey")
		}
		if isBlank(req.ConsumerSecret) {
			return missing("consumerSecret")
		}
	case domain.ConsumptionStatusGeneratedSecretKeys:
		if isBlank(req.PrimarySecretKey) {
			return missing("primarySecretKey")
		}
		if isBlank(req.SecondarySecretKey) {
			return missing("secondarySecretKey")
		}
	}
	return nil
}

// isBlank reports whether an optional string field was omitted or supplied
// empty. An empty credential is never meaningful, so the two are treated alike.
func isBlank(v *string) bool {
	return v == nil || *v == ""
}

// toProjectConsumptionView maps stored state to the read response, dropping
// every secret value — see domain.ProjectConsumptionView's doc comment.
func toProjectConsumptionView(state domain.ProjectConsumption, name, key string) domain.ProjectConsumptionView {
	return domain.ProjectConsumptionView{
		ProjectID:         state.ProjectID,
		Status:            int16(state.Status),
		ApplicationID:     state.ChoreoApplicationID,
		ConsumerKey:       state.ConsumerKey,
		Name:              key,
		Description:       fmt.Sprintf(consumptionApplicationDescription, name, key),
		HasConsumerSecret: state.ConsumerSecret != nil,
		HasSecretKeys:     state.PrimarySecretKey != nil && state.SecondarySecretKey != nil,
		UpdatedOn:         state.UpdatedOn,
	}
}
