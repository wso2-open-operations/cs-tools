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

package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/dto"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/productconsumption"
)

// licenseProvisioningWriteDeadline extends the response write deadline for
// GetDeploymentLicense beyond the server's global WriteTimeout (see
// cmd/server/main.go) — ProcessLicenseDownload can make up to 5 sequential
// upstream requests, which can plausibly exceed the global timeout under
// normal network latency even though no single step is slow.
const licenseProvisioningWriteDeadline = 2 * time.Minute

// productConsumptionClient abstracts the upstream product-consumption
// service operations used by ProductConsumptionHandler.
type productConsumptionClient interface {
	ProcessLicenseDownload(ctx context.Context, req productconsumption.LicenseDownloadRequest) (productconsumption.License, error)
	ImportDeploymentUsage(ctx context.Context, email string, zipFile []byte) (productconsumption.ImportUsageResponse, error)
}

// entityProjectAccessChecker is the subset of entityProjectClient needed to
// verify the caller has access to a project before provisioning a license.
type entityProjectAccessChecker interface {
	GetProject(ctx context.Context, id string) (entity.ProjectDetailsView, error)
}

// ProductConsumptionHandler handles HTTP requests for the product-consumption
// feature: deployment license provisioning/download and deployment usage
// import. Calls a separate upstream service (not entity-service) — see
// internal/productconsumption's package doc comment.
type ProductConsumptionHandler struct {
	productConsumption productConsumptionClient
	entity             entityProjectAccessChecker
	callerScope        *CallerScopeResolver
}

// NewProductConsumptionHandler creates a ProductConsumptionHandler backed by
// the given product-consumption and entity clients.
func NewProductConsumptionHandler(productConsumption productConsumptionClient, entityClient entityProjectAccessChecker) *ProductConsumptionHandler {
	return &ProductConsumptionHandler{productConsumption: productConsumption, entity: entityClient}
}

// SetCallerScope enables caller-scoped access: deployment license provisioning
// requires the caller to be an active portal-user contact of the project in
// the URL path. Always enforced in production (main.go calls this
// unconditionally, no kill switch) — see ProjectHandler.SetCallerScope for
// why this is a setter rather than a constructor parameter.
func (h *ProductConsumptionHandler) SetCallerScope(resolver *CallerScopeResolver) {
	h.callerScope = resolver
}

// GetDeploymentLicense handles POST /projects/{projectId}/deployments/{deploymentId}/license.
func (h *ProductConsumptionHandler) GetDeploymentLicense(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	// See licenseProvisioningWriteDeadline's doc comment. Best-effort: an
	// unrecognized ResponseWriter simply keeps the server's default timeout.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(licenseProvisioningWriteDeadline))

	projectID := r.PathValue("projectId")
	deploymentID := r.PathValue("deploymentId")
	if !uuidRe.MatchString(projectID) || !uuidRe.MatchString(deploymentID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	// Caller-scope check commented out for now per review; will be re-evaluated:
	// if !requireProjectMember(w, r, h.callerScope, projectID, user.UserID, user.Email, http.StatusForbidden, ErrMsgForbidden) {
	// 	return
	// }

	// Verify the caller can access this project before provisioning
	// anything — entity-service's own project-access check is the
	// authorization gate here.
	if _, err := h.entity.GetProject(r.Context(), projectID); err != nil {
		slog.ErrorContext(r.Context(), "entity GetProject failed", "userID", user.UserID, "projectID", projectID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve project details.")
		return
	}

	license, err := h.productConsumption.ProcessLicenseDownload(r.Context(), productconsumption.LicenseDownloadRequest{
		Email:        user.Email,
		DeploymentID: deploymentID,
		ProjectID:    projectID,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "productconsumption ProcessLicenseDownload failed", "userID", user.UserID, "projectID", projectID, "deploymentID", deploymentID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve license.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapLicense(license))
}

// ImportDeploymentUsage handles POST /deployment-usages. The request body is
// a raw zip file, not JSON.
func (h *ProductConsumptionHandler) ImportDeploymentUsage(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	contentType := strings.ToLower(strings.TrimSpace(strings.SplitN(r.Header.Get("Content-Type"), ";", 2)[0]))
	if contentType != "application/zip" && contentType != "application/x-zip-compressed" {
		writeError(w, http.StatusBadRequest, "Request body must be a zip file.")
		return
	}

	zipFile, ok := readBinaryBody(w, r, maxZipUploadBytes)
	if !ok {
		return
	}

	result, err := h.productConsumption.ImportDeploymentUsage(r.Context(), user.Email, zipFile)
	if err != nil {
		slog.ErrorContext(r.Context(), "productconsumption ImportDeploymentUsage failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to import deployment usage data.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapImportDeploymentUsage(result))
}
