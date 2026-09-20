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

package productconsumption

import (
	"context"
	"fmt"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/apierror"
)

// LicenseDownloadRequest is the input for ProcessLicenseDownload.
type LicenseDownloadRequest struct {
	Email        string
	DeploymentID string
	ProjectID    string
}

// ProcessLicenseDownload drives the upstream product-consumption service's
// per-project provisioning state machine to completion and returns the
// resulting deployment license. Mirrors
// apps/customer-portal/backend's product_consumption_subscription:processLicenseDownload
// exactly, including resuming from whatever step a project's state is
// already at (each step below only runs if the project hasn't completed it
// yet) — this is not a one-shot call, it can make up to 5 sequential
// requests to the upstream service, several of which have side effects
// (creating a WSO2 API Manager application, subscribing it, generating
// credentials) that must not be repeated once done.
func (c *Client) ProcessLicenseDownload(ctx context.Context, req LicenseDownloadRequest) (License, error) {
	statusRes, err := c.getConsumptionStatus(ctx, req.ProjectID, ConsumptionStatusRequest{
		Email: req.Email,
		// In the body as well as the path: the service matches on it either
		// way, and a dashed id matches nothing.
		DeploymentID: uuidToSysID(req.DeploymentID),
	})
	if err != nil {
		return License{}, fmt.Errorf("productconsumption: get consumption status: %w", err)
	}

	status := consumptionStatus(statusRes.Result.Status)
	applicationID := statusRes.Result.ApplicationID

	if status == statusPending {
		if statusRes.Result.Name == nil || statusRes.Result.Description == nil {
			return License{}, apierror.NewDataError("application is PENDING but the licensing service supplied no name/description for project %s", req.ProjectID)
		}
		app, err := c.createApplication(ctx, ApplicationCreateRequest{
			Name:        *statusRes.Result.Name,
			Description: *statusRes.Result.Description,
		})
		if err != nil {
			return License{}, fmt.Errorf("productconsumption: create application: %w", err)
		}
		applicationID = &app.ApplicationID

		if _, err := c.updateProjectStatus(ctx, req.ProjectID, UpdateProjectStatusRequest{
			Status:        int(statusCreated),
			ApplicationID: applicationID,
		}); err != nil {
			return License{}, fmt.Errorf("productconsumption: update project status to created: %w", err)
		}
		status = statusCreated
	}

	if applicationID == nil {
		return License{}, apierror.NewDataError("no application id for project %s after reaching status %d", req.ProjectID, status)
	}

	if status == statusCreated {
		if _, err := c.subscribeApplication(ctx, *applicationID); err != nil {
			return License{}, fmt.Errorf("productconsumption: subscribe application: %w", err)
		}
		if _, err := c.updateProjectStatus(ctx, req.ProjectID, UpdateProjectStatusRequest{
			Status: int(statusSubscribed),
		}); err != nil {
			return License{}, fmt.Errorf("productconsumption: update project status to subscribed: %w", err)
		}
		status = statusSubscribed
	}

	if status == statusSubscribed {
		creds, err := c.generateCredentials(ctx, *applicationID)
		if err != nil {
			return License{}, fmt.Errorf("productconsumption: generate credentials: %w", err)
		}
		if _, err := c.updateProjectStatus(ctx, req.ProjectID, UpdateProjectStatusRequest{
			Status:         int(statusGeneratedCredentials),
			ConsumerKey:    &creds.ConsumerKey,
			ConsumerSecret: &creds.ConsumerSecret,
		}); err != nil {
			return License{}, fmt.Errorf("productconsumption: update project status to generated-credentials: %w", err)
		}
		status = statusGeneratedCredentials
	}

	if status == statusGeneratedCredentials {
		keys, err := c.generateSecretKeys(ctx)
		if err != nil {
			return License{}, fmt.Errorf("productconsumption: generate secret keys: %w", err)
		}
		if _, err := c.updateProjectStatus(ctx, req.ProjectID, UpdateProjectStatusRequest{
			Status:             int(statusGeneratedSecretKeys),
			PrimarySecretKey:   &keys.PrimarySecretKey,
			SecondarySecretKey: &keys.SecondarySecretKey,
		}); err != nil {
			return License{}, fmt.Errorf("productconsumption: update project status to generated-secret-keys: %w", err)
		}
		status = statusGeneratedSecretKeys
	}

	if status == statusGeneratedSecretKeys {
		license, err := c.getDeploymentLicense(ctx, req.ProjectID, req.DeploymentID, DeploymentLicenseRequest{Email: req.Email})
		if err != nil {
			return License{}, fmt.Errorf("productconsumption: get deployment license: %w", err)
		}
		return license.Result.License, nil
	}

	return License{}, apierror.NewDataError("application status %d is outside the set this flow handles", status)
}

// getConsumptionStatus calls POST /projects/{projectId}/consumption/status.
func (c *Client) getConsumptionStatus(ctx context.Context, projectID string, req ConsumptionStatusRequest) (ConsumptionResult, error) {
	var out ConsumptionResult
	err := c.postJSON(ctx, fmt.Sprintf("/projects/%s/consumption/status", pathEscape(uuidToSysID(projectID))), req, &out)
	return out, err
}

// createApplication calls POST /applications.
func (c *Client) createApplication(ctx context.Context, req ApplicationCreateRequest) (ApplicationCreateResponse, error) {
	var out ApplicationCreateResponse
	err := c.postJSON(ctx, "/applications", req, &out)
	return out, err
}

// updateProjectStatus calls PATCH /projects/{projectId} — the
// product-consumption service's own per-project state record.
func (c *Client) updateProjectStatus(ctx context.Context, projectID string, req UpdateProjectStatusRequest) (ConsumptionResult, error) {
	var out ConsumptionResult
	err := c.patchJSON(ctx, fmt.Sprintf("/projects/%s", pathEscape(uuidToSysID(projectID))), req, &out)
	return out, err
}

// subscribeApplication calls POST /applications/{applicationId}/subscribe.
// The request body is the bare applicationId string, as the upstream
// endpoint expects.
func (c *Client) subscribeApplication(ctx context.Context, applicationID string) (ApplicationSubscriptionResponse, error) {
	var out ApplicationSubscriptionResponse
	err := c.postText(ctx, fmt.Sprintf("/applications/%s/subscribe", pathEscape(applicationID)), applicationID, &out)
	return out, err
}

// generateCredentials calls POST /applications/{applicationId}/generate-credentials.
func (c *Client) generateCredentials(ctx context.Context, applicationID string) (ApplicationKeyGenerationResponse, error) {
	var out ApplicationKeyGenerationResponse
	err := c.postJSON(ctx, fmt.Sprintf("/applications/%s/generate-credentials", pathEscape(applicationID)), struct{}{}, &out)
	return out, err
}

// generateSecretKeys calls POST /generate-secret-keys.
func (c *Client) generateSecretKeys(ctx context.Context) (SecretKeysResponse, error) {
	var out SecretKeysResponse
	err := c.postJSON(ctx, "/generate-secret-keys", struct{}{}, &out)
	return out, err
}

// getDeploymentLicense calls POST /projects/{projectId}/deployments/{deploymentId}/license.
func (c *Client) getDeploymentLicense(ctx context.Context, projectID, deploymentID string, req DeploymentLicenseRequest) (LicenseResponse, error) {
	var out LicenseResponse
	path := fmt.Sprintf("/projects/%s/deployments/%s/license", pathEscape(uuidToSysID(projectID)), pathEscape(uuidToSysID(deploymentID)))
	err := c.postJSON(ctx, path, req, &out)
	return out, err
}
