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
	"time"

	"golang.org/x/sync/singleflight"

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
	access       AccessService
	dualWrite    bool
	sf           singleflight.Group
}

// NewProjectConsumptionService constructs a ProjectConsumptionService backed by
// the given repository, Choreo subscription client, and dual-write setting.
//
// access scopes every method to the caller. Each of them takes the project id
// straight from the request path, and between them they read provisioning
// state, overwrite stored credentials and drive an upstream that mints real
// Choreo applications — so none may run on a project the caller cannot see.
func NewProjectConsumptionService(
	repo repository.ProjectConsumptionRepository,
	choreoClient choreosubscription.Client,
	access AccessService,
	dualWrite bool,
) ProjectConsumptionService {
	return &projectConsumptionService{
		repo:         repo,
		choreoClient: choreoClient,
		access:       access,
		dualWrite:    dualWrite,
	}
}

// GetProjectConsumption implements ProjectConsumptionService.
func (s *projectConsumptionService) GetProjectConsumption(ctx context.Context, projectID string) (domain.ProjectConsumptionView, error) {
	if err := validateUUIDs("projectId", []string{projectID}); err != nil {
		return domain.ProjectConsumptionView{}, err
	}
	if _, err := authorizeProject(ctx, s.access, projectID); err != nil {
		return domain.ProjectConsumptionView{}, err
	}
	return s.readConsumption(ctx, projectID)
}

// readConsumption is GetProjectConsumption without the authorization check, for
// callers in this file that have already made it. Kept separate so re-reading
// the state after a write does not re-resolve the caller's scope, which for a
// user-token caller is two more queries per request.
func (s *projectConsumptionService) readConsumption(ctx context.Context, projectID string) (domain.ProjectConsumptionView, error) {
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
	if _, err := authorizeProject(ctx, s.access, projectID); err != nil {
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
		view, getErr := s.readConsumption(ctx, projectID)
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
	view, err := s.readConsumption(ctx, projectID)
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
// This is the ONLY enforcement of these rules. They were once CHECK
// constraints as well, on the superseded project_consumption table; that table
// is gone and the state now lives on project, which carries no CHECK
// constraint for any of them. So do not remove this validation on the
// assumption that the database will catch it -- nothing will, and a step would
// record as complete with its own output missing.
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
	if err := validateEmail(email); err != nil {
		return domain.License{}, err
	}
	if _, err := authorizeProject(ctx, s.access, projectID); err != nil {
		return domain.License{}, err
	}
	if s.choreoClient == nil {
		return domain.License{}, errors.New("choreo subscription client not configured")
	}

	// Synchronize project provisioning transitions per projectID using singleflight
	// so concurrent requests cannot race on status checks or create duplicate Choreo applications.
	ch := s.sf.DoChan(projectID, func() (any, error) {
		provisionCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
		defer cancel()
		return nil, s.ensureProjectProvisioned(provisionCtx, projectID, deploymentID, email)
	})

	select {
	case <-ctx.Done():
		return domain.License{}, ctx.Err()
	case res := <-ch:
		if res.Err != nil {
			return domain.License{}, res.Err
		}
	}

	return s.downloadDeploymentLicense(ctx, projectID, deploymentID, email)
}

// ensureProjectProvisioned advances a project through the four setup steps,
// one status at a time (1 -> 2 -> 3 -> 4 -> 5), until it reaches
// ConsumptionStatusGeneratedSecretKeys: create the application (2), subscribe
// it (3), generate credentials (4), generate secret keys (5). No status is
// skipped -- each step records its own outcome before the next begins, which
// is what makes the sequence resumable.
//
// Idempotent and safe to call repeatedly: it resumes from the stored status,
// so a project already at 5 does no upstream work.
func (s *projectConsumptionService) ensureProjectProvisioned(ctx context.Context, projectID, deploymentID, email string) error {
	statusRes, err := s.choreoClient.GetConsumptionStatus(ctx, projectID, choreosubscription.ConsumptionStatusRequest{
		Email:        email,
		DeploymentID: deploymentID,
	})
	if err != nil {
		return fmt.Errorf("choreosubscription: get consumption status: %w", err)
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
			return fmt.Errorf("application is PENDING but the licensing service supplied no name/description for project %s", projectID)
		}
		app, err := s.choreoClient.CreateApplication(ctx, choreosubscription.ApplicationCreateRequest{
			Name:        *statusRes.Result.Name,
			Description: *statusRes.Result.Description,
		})
		if err != nil {
			return fmt.Errorf("choreosubscription: create application: %w", err)
		}
		applicationID = &app.ApplicationID

		if _, err := s.choreoClient.UpdateProjectStatus(ctx, projectID, choreosubscription.UpdateProjectStatusRequest{
			Status:        int(domain.ConsumptionStatusCreated),
			ApplicationID: applicationID,
		}); err != nil {
			return fmt.Errorf("choreosubscription: update project status to created: %w", err)
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
		return fmt.Errorf("no application id for project %s after reaching status %d", projectID, status)
	}

	if status == int(domain.ConsumptionStatusCreated) {
		if _, err := s.choreoClient.SubscribeApplication(ctx, *applicationID); err != nil {
			return fmt.Errorf("choreosubscription: subscribe application: %w", err)
		}
		if _, err := s.choreoClient.UpdateProjectStatus(ctx, projectID, choreosubscription.UpdateProjectStatusRequest{
			Status: int(domain.ConsumptionStatusSubscribed),
		}); err != nil {
			return fmt.Errorf("choreosubscription: update project status to subscribed: %w", err)
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
			return fmt.Errorf("choreosubscription: generate credentials: %w", err)
		}
		if _, err := s.choreoClient.UpdateProjectStatus(ctx, projectID, choreosubscription.UpdateProjectStatusRequest{
			Status:         int(domain.ConsumptionStatusGeneratedCredentials),
			ConsumerKey:    &creds.ConsumerKey,
			ConsumerSecret: &creds.ConsumerSecret,
		}); err != nil {
			return fmt.Errorf("choreosubscription: update project status to generated-credentials: %w", err)
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
			return fmt.Errorf("choreosubscription: generate secret keys: %w", err)
		}
		if _, err := s.choreoClient.UpdateProjectStatus(ctx, projectID, choreosubscription.UpdateProjectStatusRequest{
			Status:             int(domain.ConsumptionStatusGeneratedSecretKeys),
			PrimarySecretKey:   &keys.PrimarySecretKey,
			SecondarySecretKey: &keys.SecondarySecretKey,
		}); err != nil {
			return fmt.Errorf("choreosubscription: update project status to generated-secret-keys: %w", err)
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

	if status != int(domain.ConsumptionStatusGeneratedSecretKeys) {
		return fmt.Errorf("application status %d is outside the set this flow handles", status)
	}

	return nil
}

// downloadDeploymentLicense issues the signed deployment license for a fully provisioned project (status 5).
func (s *projectConsumptionService) downloadDeploymentLicense(ctx context.Context, projectID, deploymentID, email string) (domain.License, error) {
	req := domain.DeploymentLicenseRequest{
		Email: email,
	}
	// The signing context is only safe to send when the Postgres mirror is
	// authoritative for this project. GetSigningContext reads credentials
	// by project id alone and does not look at status, so a mirror that is
	// disabled, stale or diverged would otherwise hand the licensing
	// operation keys that do not match the ones ServiceNow holds -- and it
	// would sign with them rather than fail. Two conditions gate it:
	// dual-write is on, so this service is actually maintaining the mirror;
	// and the mirror has itself reached step 5, matching the ServiceNow
	// status this branch was entered on. The status is re-read here rather
	// than reused from the divergence check above, because the sequence
	// advances the mirror during this same call.
	//
	// Failing either check is not an error: the direct download is the
	// pre-existing path and still works.
	if s.dualWrite && s.repo != nil {
		pgState, _, _, getErr := s.repo.Get(ctx, projectID)
		switch {
		case getErr != nil:
			slog.WarnContext(ctx, "could not read the postgres mirror for licence signing; falling back to direct download",
				"projectId", projectID,
				"deploymentId", deploymentID,
				"err", getErr,
			)
		case pgState.Status != domain.ConsumptionStatusGeneratedSecretKeys:
			slog.WarnContext(ctx, "postgres mirror is not at the generated-secret-keys step; falling back to direct download",
				"projectId", projectID,
				"deploymentId", deploymentID,
				"postgresStatus", int(pgState.Status),
			)
		default:
			signingCtx, err := s.repo.GetSigningContext(ctx, projectID, deploymentID)
			if err != nil {
				slog.WarnContext(ctx, "failed to load signing context for licence download; falling back to direct download",
					"projectId", projectID,
					"deploymentId", deploymentID,
					"err", err,
				)
			} else if signingCtx != nil && signingCtx.PrimarySecretKey != "" {
				req.SigningContext = signingCtx
			}
		}
	}

	license, err := s.choreoClient.GetDeploymentLicense(ctx, projectID, deploymentID, req)
	if err != nil {
		return domain.License{}, fmt.Errorf("choreosubscription: get deployment license: %w", err)
	}
	return license, nil
}
