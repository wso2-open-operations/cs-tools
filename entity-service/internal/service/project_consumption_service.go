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
	"log/slog"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/choreosubscription"
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
	repo         repository.ProjectConsumptionRepository
	choreoClient choreosubscription.Client
	dualWrite    bool
}

// NewProjectConsumptionService constructs a ProjectConsumptionService backed by
// the given repository, Choreo subscription client, and dual-write setting.
func NewProjectConsumptionService(
	repo repository.ProjectConsumptionRepository,
	choreoClient choreosubscription.Client,
	dualWrite bool,
) ProjectConsumptionService {
	return &projectConsumptionService{
		repo:         repo,
		choreoClient: choreoClient,
		dualWrite:    dualWrite,
	}
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

// ProcessLicenseDownload implements ProjectConsumptionService.
//
// Drives the upstream Choreo subscription operation's 5-step resumable
// state machine to issue a signed deployment license. ServiceNow mutation
// is fatal; Postgres dual-write is non-fatal (logged on failure) so
// license issuance cannot fail due to secondary store transient errors,
// while ServiceNow never falls behind.
func (s *projectConsumptionService) ProcessLicenseDownload(ctx context.Context, projectID, deploymentID, email string) (domain.License, error) {
	if err := validateUUIDs("projectId", []string{projectID}); err != nil {
		return domain.License{}, err
	}
	if err := validateUUIDs("deploymentId", []string{deploymentID}); err != nil {
		return domain.License{}, err
	}
	if email == "" {
		return domain.License{}, &apierror.ValidationError{Msg: "email is required"}
	}
	if s.choreoClient == nil {
		return domain.License{}, errors.New("choreo subscription client not configured")
	}

	statusRes, err := s.choreoClient.GetConsumptionStatus(ctx, projectID, choreosubscription.ConsumptionStatusRequest{
		Email:        email,
		DeploymentID: deploymentID,
	})
	if err != nil {
		return domain.License{}, fmt.Errorf("choreosubscription: get consumption status: %w", err)
	}

	if s.repo != nil {
		pgState, _, _, getErr := s.repo.Get(ctx, projectID)
		if getErr == nil {
			if int(pgState.Status) != int(statusRes.Result.Status) {
				slog.WarnContext(ctx, "project consumption status diverged between stores",
					"projectId", projectID,
					"serviceNowStatus", int(statusRes.Result.Status),
					"postgresStatus", int(pgState.Status),
				)
			}
		}
	}

	status := int(statusRes.Result.Status)
	applicationID := statusRes.Result.ApplicationID

	if status == int(domain.ConsumptionStatusPending) {
		if statusRes.Result.Name == nil || statusRes.Result.Description == nil {
			return domain.License{}, fmt.Errorf("application is PENDING but the licensing service supplied no name/description for project %s", projectID)
		}
		app, err := s.choreoClient.CreateApplication(ctx, choreosubscription.ApplicationCreateRequest{
			Name:        *statusRes.Result.Name,
			Description: *statusRes.Result.Description,
		})
		if err != nil {
			return domain.License{}, fmt.Errorf("choreosubscription: create application: %w", err)
		}
		applicationID = &app.ApplicationID

		if _, err := s.choreoClient.UpdateProjectStatus(ctx, projectID, choreosubscription.UpdateProjectStatusRequest{
			Status:        int(domain.ConsumptionStatusCreated),
			ApplicationID: applicationID,
		}); err != nil {
			return domain.License{}, fmt.Errorf("choreosubscription: update project status to created: %w", err)
		}

		if s.dualWrite && s.repo != nil {
			if _, err := s.repo.Upsert(ctx, projectID, domain.ProjectConsumption{
				Status:              domain.ConsumptionStatusCreated,
				ChoreoApplicationID: applicationID,
			}); err != nil {
				slog.ErrorContext(ctx, "failed to dual-write project consumption to postgres",
					"projectId", projectID,
					"status", domain.ConsumptionStatusCreated,
					"err", err,
				)
			}
		}
		status = int(domain.ConsumptionStatusCreated)
	}

	if applicationID == nil {
		return domain.License{}, fmt.Errorf("no application id for project %s after reaching status %d", projectID, status)
	}

	if status == int(domain.ConsumptionStatusCreated) {
		if _, err := s.choreoClient.SubscribeApplication(ctx, *applicationID); err != nil {
			return domain.License{}, fmt.Errorf("choreosubscription: subscribe application: %w", err)
		}
		if _, err := s.choreoClient.UpdateProjectStatus(ctx, projectID, choreosubscription.UpdateProjectStatusRequest{
			Status: int(domain.ConsumptionStatusSubscribed),
		}); err != nil {
			return domain.License{}, fmt.Errorf("choreosubscription: update project status to subscribed: %w", err)
		}

		if s.dualWrite && s.repo != nil {
			if _, err := s.repo.Upsert(ctx, projectID, domain.ProjectConsumption{
				Status: domain.ConsumptionStatusSubscribed,
			}); err != nil {
				slog.ErrorContext(ctx, "failed to dual-write project consumption to postgres",
					"projectId", projectID,
					"status", domain.ConsumptionStatusSubscribed,
					"err", err,
				)
			}
		}
		status = int(domain.ConsumptionStatusSubscribed)
	}

	if status == int(domain.ConsumptionStatusSubscribed) {
		creds, err := s.choreoClient.GenerateCredentials(ctx, *applicationID)
		if err != nil {
			return domain.License{}, fmt.Errorf("choreosubscription: generate credentials: %w", err)
		}
		if _, err := s.choreoClient.UpdateProjectStatus(ctx, projectID, choreosubscription.UpdateProjectStatusRequest{
			Status:         int(domain.ConsumptionStatusGeneratedCredentials),
			ConsumerKey:    &creds.ConsumerKey,
			ConsumerSecret: &creds.ConsumerSecret,
		}); err != nil {
			return domain.License{}, fmt.Errorf("choreosubscription: update project status to generated-credentials: %w", err)
		}

		if s.dualWrite && s.repo != nil {
			if _, err := s.repo.Upsert(ctx, projectID, domain.ProjectConsumption{
				Status:         domain.ConsumptionStatusGeneratedCredentials,
				ConsumerKey:    &creds.ConsumerKey,
				ConsumerSecret: &creds.ConsumerSecret,
			}); err != nil {
				slog.ErrorContext(ctx, "failed to dual-write project consumption to postgres",
					"projectId", projectID,
					"status", domain.ConsumptionStatusGeneratedCredentials,
					"err", err,
				)
			}
		}
		status = int(domain.ConsumptionStatusGeneratedCredentials)
	}

	if status == int(domain.ConsumptionStatusGeneratedCredentials) {
		keys, err := s.choreoClient.GenerateSecretKeys(ctx)
		if err != nil {
			return domain.License{}, fmt.Errorf("choreosubscription: generate secret keys: %w", err)
		}
		if _, err := s.choreoClient.UpdateProjectStatus(ctx, projectID, choreosubscription.UpdateProjectStatusRequest{
			Status:             int(domain.ConsumptionStatusGeneratedSecretKeys),
			PrimarySecretKey:   &keys.PrimarySecretKey,
			SecondarySecretKey: &keys.SecondarySecretKey,
		}); err != nil {
			return domain.License{}, fmt.Errorf("choreosubscription: update project status to generated-secret-keys: %w", err)
		}

		if s.dualWrite && s.repo != nil {
			if _, err := s.repo.Upsert(ctx, projectID, domain.ProjectConsumption{
				Status:             domain.ConsumptionStatusGeneratedSecretKeys,
				PrimarySecretKey:   &keys.PrimarySecretKey,
				SecondarySecretKey: &keys.SecondarySecretKey,
			}); err != nil {
				slog.ErrorContext(ctx, "failed to dual-write project consumption to postgres",
					"projectId", projectID,
					"status", domain.ConsumptionStatusGeneratedSecretKeys,
					"err", err,
				)
			}
		}
		status = int(domain.ConsumptionStatusGeneratedSecretKeys)
	}

	if status == int(domain.ConsumptionStatusGeneratedSecretKeys) {
		license, err := s.choreoClient.GetDeploymentLicense(ctx, projectID, deploymentID, domain.DeploymentLicenseRequest{
			Email: email,
		})
		if err != nil {
			return domain.License{}, fmt.Errorf("choreosubscription: get deployment license: %w", err)
		}
		return license, nil
	}

	return domain.License{}, fmt.Errorf("application status %d is outside the set this flow handles", status)
}
