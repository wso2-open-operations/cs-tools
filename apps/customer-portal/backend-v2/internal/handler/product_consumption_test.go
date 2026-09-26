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
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/dto"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/productconsumption"
)

type mockProductConsumptionClient struct {
	importUsageResp productconsumption.ImportUsageResponse
	importUsageErr  error
}

func (m *mockProductConsumptionClient) ImportDeploymentUsage(_ context.Context, _ string, _ []byte) (productconsumption.ImportUsageResponse, error) {
	return m.importUsageResp, m.importUsageErr
}

type mockEntityLicenseClient struct {
	projectResp entity.ProjectDetailsView
	projectErr  error

	licenseResp entity.License
	licenseErr  error

	gotProjectID    string
	gotDeploymentID string
	gotEmail        string
}

func (m *mockEntityLicenseClient) GetProject(_ context.Context, id string) (entity.ProjectDetailsView, error) {
	return m.projectResp, m.projectErr
}

func (m *mockEntityLicenseClient) GetDeploymentLicense(_ context.Context, projectID, deploymentID, email string) (entity.License, error) {
	m.gotProjectID = projectID
	m.gotDeploymentID = deploymentID
	m.gotEmail = email
	return m.licenseResp, m.licenseErr
}

func TestGetDeploymentLicense_Unauthorized(t *testing.T) {
	h := NewProductConsumptionHandler(&mockProductConsumptionClient{}, &mockEntityLicenseClient{})

	req := httptest.NewRequest(http.MethodPost, "/projects/11111111-1111-1111-1111-111111111111/deployments/22222222-2222-2222-2222-222222222222/license", nil)
	rec := httptest.NewRecorder()

	h.GetDeploymentLicense(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", rec.Code)
	}
}

func TestGetDeploymentLicense_InvalidUUID(t *testing.T) {
	h := NewProductConsumptionHandler(&mockProductConsumptionClient{}, &mockEntityLicenseClient{})

	mux := http.NewServeMux()
	mux.HandleFunc("POST /projects/{projectId}/deployments/{deploymentId}/license", h.GetDeploymentLicense)

	rec := httptest.NewRecorder()
	req := authedRequest(http.MethodPost, "/projects/not-a-uuid/deployments/22222222-2222-2222-2222-222222222222/license", "")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for non-UUID project, got %d", rec.Code)
	}
}

func TestGetDeploymentLicense_GetProjectFails(t *testing.T) {
	mockEntity := &mockEntityLicenseClient{
		projectErr: apierror.NewUpstreamError(http.StatusNotFound, []byte("project not found")),
	}
	h := NewProductConsumptionHandler(&mockProductConsumptionClient{}, mockEntity)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /projects/{projectId}/deployments/{deploymentId}/license", h.GetDeploymentLicense)

	rec := httptest.NewRecorder()
	req := authedRequest(http.MethodPost, "/projects/11111111-1111-1111-1111-111111111111/deployments/22222222-2222-2222-2222-222222222222/license", "")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found when GetProject fails, got %d", rec.Code)
	}
}

func TestGetDeploymentLicense_Success(t *testing.T) {
	const projectID = "11111111-1111-1111-1111-111111111111"
	const deploymentID = "22222222-2222-2222-2222-222222222222"

	mockEntity := &mockEntityLicenseClient{
		projectResp: entity.ProjectDetailsView{ID: projectID, Name: "Test Project"},
		// usageDataPublishingUrl is signed by ServiceNow but was absent from the
		// struct this response used to be modelled with. It stands in here for
		// every field the portal does not name.
		licenseResp: entity.License{
			Signature: "signed-license-token",
			SubscriptionData: json.RawMessage(`{` +
				`"deploymentId":"` + deploymentID + `",` +
				`"deploymentName":"Production",` +
				`"subscriptionKey":"sub-key",` +
				`"clientId":"client-id",` +
				`"clientSecret":"client-secret",` +
				`"secrets":"secrets",` +
				`"usageDataPublishingUrl":"https://example.invalid/usage"}`),
		},
	}
	h := NewProductConsumptionHandler(&mockProductConsumptionClient{}, mockEntity)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /projects/{projectId}/deployments/{deploymentId}/license", h.GetDeploymentLicense)

	rec := httptest.NewRecorder()
	req := authedRequest(http.MethodPost, "/projects/"+projectID+"/deployments/"+deploymentID+"/license", "")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	if mockEntity.gotProjectID != projectID || mockEntity.gotDeploymentID != deploymentID {
		t.Errorf("expected entity call with project %s and deployment %s, got %s and %s",
			projectID, deploymentID, mockEntity.gotProjectID, mockEntity.gotDeploymentID)
	}

	var resp dto.LicenseResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Signature != "signed-license-token" {
		t.Errorf("got signature %q, want signed-license-token", resp.Signature)
	}
	// The portal must hand the customer exactly what ServiceNow signed. A field
	// dropped here breaks signature verification in the customer's product, and
	// usageDataPublishingUrl is also where that product publishes its usage.
	var sub map[string]any
	if err := json.Unmarshal(resp.SubscriptionData, &sub); err != nil {
		t.Fatalf("subscriptionData is not valid JSON: %v", err)
	}
	for k, want := range map[string]string{
		"deploymentId":           deploymentID,
		"clientSecret":           "client-secret",
		"secrets":                "secrets",
		"usageDataPublishingUrl": "https://example.invalid/usage",
	} {
		if sub[k] != want {
			t.Errorf("subscriptionData[%q] = %v, want %q", k, sub[k], want)
		}
	}
}

func TestGetDeploymentLicense_EntityLicenseFails(t *testing.T) {
	const projectID = "11111111-1111-1111-1111-111111111111"
	const deploymentID = "22222222-2222-2222-2222-222222222222"

	mockEntity := &mockEntityLicenseClient{
		projectResp: entity.ProjectDetailsView{ID: projectID, Name: "Test Project"},
		licenseErr:  errors.New("downstream 500"),
	}
	h := NewProductConsumptionHandler(&mockProductConsumptionClient{}, mockEntity)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /projects/{projectId}/deployments/{deploymentId}/license", h.GetDeploymentLicense)

	rec := httptest.NewRecorder()
	req := authedRequest(http.MethodPost, "/projects/"+projectID+"/deployments/"+deploymentID+"/license", "")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 Internal Server Error when entity GetDeploymentLicense fails, got %d", rec.Code)
	}
}

// TestGetDeploymentLicense_E2E_EntityClientHttp tests that ProductConsumptionHandler
// forwards requests over real HTTP using entity.Client to entity-service and returns
// the mapped license.
func TestGetDeploymentLicense_E2E_EntityClientHttp(t *testing.T) {
	const projectID = "11111111-1111-1111-1111-111111111111"
	const deploymentID = "22222222-2222-2222-2222-222222222222"

	var receivedPath string
	var receivedEmail string

	mockEntityService := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		if r.Method == http.MethodPost && r.URL.Path == "/token" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "mock-token",
				"token_type":   "Bearer",
				"expires_in":   3600,
			})
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/projects/"+projectID {
			_ = json.NewEncoder(w).Encode(entity.ProjectDetailsView{ID: projectID, Name: "Test Project"})
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/projects/"+projectID+"/deployments/"+deploymentID+"/license" {
			var body entity.DeploymentLicenseRequest
			_ = json.NewDecoder(r.Body).Decode(&body)
			receivedEmail = body.Email
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"signature": "e2e-token-from-entity-service",
				"subscriptionData": map[string]any{
					"deploymentId":           deploymentID,
					"deploymentName":         "Production-E2E",
					"subscriptionKey":        "sub-key-123",
					"clientId":               "client-123",
					"clientSecret":           "secret-123",
					"secrets":                "sec-123",
					"usageDataPublishingUrl": "https://example.invalid/usage",
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer mockEntityService.Close()

	realEntityClient := entity.NewClient(entity.Config{
		BaseURL:      mockEntityService.URL,
		TokenURL:     mockEntityService.URL + "/token",
		ClientID:     "mock-client",
		ClientSecret: "mock-secret",
	})

	h := NewProductConsumptionHandler(&mockProductConsumptionClient{}, realEntityClient)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /projects/{projectId}/deployments/{deploymentId}/license", h.GetDeploymentLicense)

	rec := httptest.NewRecorder()
	req := authedRequest(http.MethodPost, "/projects/"+projectID+"/deployments/"+deploymentID+"/license", "")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	if receivedPath != "/projects/"+projectID+"/deployments/"+deploymentID+"/license" {
		t.Errorf("entity service received path %q, want %q", receivedPath, "/projects/"+projectID+"/deployments/"+deploymentID+"/license")
	}
	if receivedEmail == "" {
		t.Errorf("entity service received empty email, want authenticated user email")
	}

	var resp dto.LicenseResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Signature != "e2e-token-from-entity-service" {
		t.Errorf("got signature %q, want e2e-token-from-entity-service", resp.Signature)
	}
	// End to end, entity-service → portal → customer: the signed payload must
	// arrive byte-complete, unmodelled fields included.
	var sub map[string]any
	if err := json.Unmarshal(resp.SubscriptionData, &sub); err != nil {
		t.Fatalf("subscriptionData is not valid JSON: %v", err)
	}
	for k, want := range map[string]string{
		"deploymentName":         "Production-E2E",
		"secrets":                "sec-123",
		"usageDataPublishingUrl": "https://example.invalid/usage",
	} {
		if sub[k] != want {
			t.Errorf("subscriptionData[%q] = %v, want %q", k, sub[k], want)
		}
	}
}
