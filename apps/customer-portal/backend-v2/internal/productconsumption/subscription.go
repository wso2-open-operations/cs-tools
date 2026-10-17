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
	"bytes"
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

// ProcessLicenseDownload delegates deployment license provisioning to the
// upstream product-consumption microservice in a single 1-shot call.
// The upstream microservice encapsulates WSO2 API Manager application creation,
// tier subscription, credential generation, and offline/online HMAC signing.
func (c *Client) ProcessLicenseDownload(ctx context.Context, req LicenseDownloadRequest) (License, error) {
	license, err := c.getDeploymentLicense(ctx, req.ProjectID, req.DeploymentID, DeploymentLicenseRequest{Email: req.Email})
	if err != nil {
		return License{}, fmt.Errorf("productconsumption: get deployment license: %w", err)
	}
	return license.Result.License, nil
}

// getDeploymentLicense calls POST /projects/{projectId}/deployments/{deploymentId}/license.
//
// A refusal arrives as a 200 carrying success:false, and a malformed
// subscriptionData or a missing signature are both possible even on
// success:true — any of these would otherwise reach ProcessLicenseDownload's
// caller as a 200 with an empty or unusable license and no error. A license
// with no signature is unverifiable by the customer's product, the same
// failure shape as no subscriptionData at all.
func (c *Client) getDeploymentLicense(ctx context.Context, projectID, deploymentID string, req DeploymentLicenseRequest) (LicenseResponse, error) {
	var out LicenseResponse
	path := fmt.Sprintf("/projects/%s/deployments/%s/license", pathEscape(uuidToSysID(projectID)), pathEscape(uuidToSysID(deploymentID)))
	if err := c.postJSON(ctx, path, req, &out); err != nil {
		return LicenseResponse{}, err
	}
	if !out.Result.Success {
		return LicenseResponse{}, apierror.NewDataError("licensing service did not issue a license for project %s", projectID)
	}
	data := bytes.TrimSpace(out.Result.License.SubscriptionData)
	if len(data) == 0 || data[0] != '{' {
		return LicenseResponse{}, apierror.NewDataError("licensing service returned no subscription data for project %s", projectID)
	}
	if out.Result.License.Signature == "" {
		return LicenseResponse{}, apierror.NewDataError("licensing service returned no signature for project %s", projectID)
	}
	return out, nil
}
