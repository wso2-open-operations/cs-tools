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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/choreosubscription"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// unrestrictedAccess is a service.AccessService stub for the one test here
// that builds a real ProjectConsumptionService: it exercises the handler's
// licence path end to end against a fake Choreo, not the scoping rules (those
// are the service package's own tests).
type unrestrictedAccess struct{}

func (unrestrictedAccess) ResolveScope(context.Context) (service.AccessScope, error) {
	return service.AccessScope{Unrestricted: true}, nil
}

type stubProjectConsumptionService struct {
	service.ProjectConsumptionService

	getResp domain.ProjectConsumptionView
	getErr  error

	updateResp domain.UpdateProjectConsumptionResponse
	updateErr  error

	licenseResp domain.License
	licenseErr  error
}

func (s *stubProjectConsumptionService) GetProjectConsumption(_ context.Context, _ string) (domain.ProjectConsumptionView, error) {
	return s.getResp, s.getErr
}

func (s *stubProjectConsumptionService) UpdateProjectConsumption(_ context.Context, _ string, _ domain.UpdateProjectConsumptionRequest) (domain.UpdateProjectConsumptionResponse, error) {
	return s.updateResp, s.updateErr
}

func (s *stubProjectConsumptionService) ProcessLicenseDownload(_ context.Context, _, _, _ string) (domain.License, error) {
	return s.licenseResp, s.licenseErr
}

func TestGetDeploymentLicense_Success(t *testing.T) {
	stub := &stubProjectConsumptionService{
		licenseResp: domain.License{
			Signature:        "test-signature",
			SubscriptionData: json.RawMessage(`{"deploymentId":"dep-1","deploymentName":"Production","subscriptionKey":"sub-key","usageDataPublishingUrl":"https://example.invalid/usage"}`),
		},
	}
	h := NewProjectConsumptionHandler(stub)

	body, _ := json.Marshal(domain.DeploymentLicenseRequest{Email: "user@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/projects/p-1/deployments/d-1/license", bytes.NewReader(body))
	req.SetPathValue("id", "6fa0b42d-1bfa-4a69-a002-c9d3604bcb77")
	req.SetPathValue("deploymentId", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()

	h.GetDeploymentLicense(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.License
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Signature != "test-signature" {
		t.Errorf("got signature %q, want test-signature", resp.Signature)
	}
}

func TestGetDeploymentLicense_InvalidJSON(t *testing.T) {
	stub := &stubProjectConsumptionService{}
	h := NewProjectConsumptionHandler(stub)

	req := httptest.NewRequest(http.MethodPost, "/projects/p-1/deployments/d-1/license", bytes.NewReader([]byte("not json")))
	rec := httptest.NewRecorder()

	h.GetDeploymentLicense(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
	}
}

func TestGetDeploymentLicense_ValidationError(t *testing.T) {
	stub := &stubProjectConsumptionService{
		licenseErr: &apierror.ValidationError{Msg: "email is required"},
	}
	h := NewProjectConsumptionHandler(stub)

	body, _ := json.Marshal(domain.DeploymentLicenseRequest{})
	req := httptest.NewRequest(http.MethodPost, "/projects/p-1/deployments/d-1/license", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.GetDeploymentLicense(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
	}
}

func TestGetDeploymentLicense_InternalError(t *testing.T) {
	stub := &stubProjectConsumptionService{
		licenseErr: errors.New("upstream failed"),
	}
	h := NewProjectConsumptionHandler(stub)

	body, _ := json.Marshal(domain.DeploymentLicenseRequest{Email: "user@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/projects/p-1/deployments/d-1/license", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.GetDeploymentLicense(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 Internal Server Error, got %d", rec.Code)
	}
}

func TestGetProjectConsumption_Success(t *testing.T) {
	stub := &stubProjectConsumptionService{
		getResp: domain.ProjectConsumptionView{
			ProjectID: "6fa0b42d-1bfa-4a69-a002-c9d3604bcb77",
			Status:    1,
		},
	}
	h := NewProjectConsumptionHandler(stub)

	req := httptest.NewRequest(http.MethodGet, "/projects/6fa0b42d-1bfa-4a69-a002-c9d3604bcb77/consumption", nil)
	req.SetPathValue("id", "6fa0b42d-1bfa-4a69-a002-c9d3604bcb77")
	rec := httptest.NewRecorder()

	h.GetProjectConsumption(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
}

func TestUpdateProjectConsumption_Success(t *testing.T) {
	stub := &stubProjectConsumptionService{
		updateResp: domain.UpdateProjectConsumptionResponse{
			Message: "project consumption updated",
		},
	}
	h := NewProjectConsumptionHandler(stub)

	body, _ := json.Marshal(domain.UpdateProjectConsumptionRequest{Status: 2})
	req := httptest.NewRequest(http.MethodPatch, "/projects/6fa0b42d-1bfa-4a69-a002-c9d3604bcb77/consumption", bytes.NewReader(body))
	req.SetPathValue("id", "6fa0b42d-1bfa-4a69-a002-c9d3604bcb77")
	rec := httptest.NewRecorder()

	h.UpdateProjectConsumption(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
}

// TestGetDeploymentLicense_E2E_ChoreoFlow spins up an httptest.Server representing
// the upstream Choreo subscription operation and an httptest.Server representing
// entity-service, testing the complete 5-step license issuance flow over real HTTP.
func TestGetDeploymentLicense_E2E_ChoreoFlow(t *testing.T) {
	var steps []string
	choreoMux := http.NewServeMux()

	// The operation sits behind the Choreo gateway, so the client fetches a
	// client-credentials token before the first call and reuses it thereafter.
	choreoMux.HandleFunc("POST /oauth2/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-token","expires_in":3600}`))
	})

	choreoMux.HandleFunc("POST /projects/{id}/consumption/status", func(w http.ResponseWriter, r *http.Request) {
		steps = append(steps, "status")
		name := "TestApp"
		desc := "TestDescription"
		_ = json.NewEncoder(w).Encode(choreosubscription.ConsumptionResult{
			Result: choreosubscription.ConsumptionData{
				Status:      1,
				Name:        &name,
				Description: &desc,
			},
		})
	})

	choreoMux.HandleFunc("POST /applications", func(w http.ResponseWriter, r *http.Request) {
		steps = append(steps, "create_app")
		_ = json.NewEncoder(w).Encode(choreosubscription.ApplicationCreateResponse{
			Name:          "TestApp",
			ApplicationID: "app-12345",
		})
	})

	choreoMux.HandleFunc("POST /applications/{id}/subscribe", func(w http.ResponseWriter, r *http.Request) {
		steps = append(steps, "subscribe")
		_ = json.NewEncoder(w).Encode(choreosubscription.ApplicationSubscriptionResponse{
			ApplicationID:  "app-12345",
			SubscriptionID: "sub-12345",
			APIID:          "api-12345",
		})
	})

	choreoMux.HandleFunc("POST /applications/{id}/generate-credentials", func(w http.ResponseWriter, r *http.Request) {
		steps = append(steps, "generate_credentials")
		_ = json.NewEncoder(w).Encode(choreosubscription.ApplicationKeyGenerationResponse{
			ConsumerKey:    "ck-999",
			ConsumerSecret: "cs-999",
		})
	})

	choreoMux.HandleFunc("POST /generate-secret-keys", func(w http.ResponseWriter, r *http.Request) {
		steps = append(steps, "generate_secret_keys")
		_ = json.NewEncoder(w).Encode(choreosubscription.SecretKeysResponse{
			PrimarySecretKey:   "pk-999",
			SecondarySecretKey: "sk-999",
		})
	})

	choreoMux.HandleFunc("PATCH /projects/{id}", func(w http.ResponseWriter, r *http.Request) {
		var req choreosubscription.UpdateProjectStatusRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		steps = append(steps, fmt.Sprintf("update_status_%d", req.Status))
		_ = json.NewEncoder(w).Encode(choreosubscription.ConsumptionResult{})
	})

	choreoMux.HandleFunc("POST /projects/{id}/deployments/{deploymentId}/license", func(w http.ResponseWriter, r *http.Request) {
		steps = append(steps, "license")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": map[string]any{
				"success": true,
				// usageDataPublishingUrl stands in for the signed fields this
				// service does not model; it must reach the caller untouched.
				"license": map[string]any{
					"signature": "valid-choreo-hmac-signature",
					"subscriptionData": map[string]any{
						"deploymentId":           r.PathValue("deploymentId"),
						"deploymentName":         "Production-E2E",
						"subscriptionKey":        "sub-prod-key",
						"clientId":               "ck-999",
						"clientSecret":           "cs-999",
						"secrets":                "sec-999",
						"usageDataPublishingUrl": "https://example.invalid/usage",
					},
				},
			},
		})
	})

	choreoServer := httptest.NewServer(choreoMux)
	defer choreoServer.Close()

	choreoClient, err := choreosubscription.NewClient(choreosubscription.Config{
		BaseURL: choreoServer.URL,
		Creds: choreosubscription.ClientCredentialsConfig{
			TokenURL:     choreoServer.URL + "/oauth2/token",
			ClientID:     "test-id",
			ClientSecret: "test-secret",
		},
		HTTPClient: choreoServer.Client(),
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	svc := service.NewProjectConsumptionService(nil, choreoClient, unrestrictedAccess{}, false)
	h := NewProjectConsumptionHandler(svc)

	entityMux := http.NewServeMux()
	entityMux.HandleFunc("POST /projects/{id}/deployments/{deploymentId}/license", h.GetDeploymentLicense)
	entityServer := httptest.NewServer(entityMux)
	defer entityServer.Close()

	projectID := "6fa0b42d-1bfa-4a69-a002-c9d3604bcb77"
	deploymentID := "11111111-1111-1111-1111-111111111111"

	reqBody, _ := json.Marshal(domain.DeploymentLicenseRequest{Email: "admin@wso2.com"})
	res, err := http.Post(
		fmt.Sprintf("%s/projects/%s/deployments/%s/license", entityServer.URL, projectID, deploymentID),
		"application/json",
		bytes.NewReader(reqBody),
	)
	if err != nil {
		t.Fatalf("POST license failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.StatusCode)
	}

	var lic domain.License
	if err := json.NewDecoder(res.Body).Decode(&lic); err != nil {
		t.Fatalf("decode license: %v", err)
	}

	if lic.Signature != "valid-choreo-hmac-signature" {
		t.Errorf("got signature %q, want valid-choreo-hmac-signature", lic.Signature)
	}
	// Every field ServiceNow signed must arrive intact — including the ones no
	// struct in this service names, since the customer's product recomputes the
	// canonical string over all of them to verify the signature.
	var sub map[string]any
	if err := json.Unmarshal(lic.SubscriptionData, &sub); err != nil {
		t.Fatalf("subscriptionData is not valid JSON: %v", err)
	}
	for k, want := range map[string]string{
		"deploymentName":         "Production-E2E",
		"clientId":               "ck-999",
		"secrets":                "sec-999",
		"usageDataPublishingUrl": "https://example.invalid/usage",
	} {
		if sub[k] != want {
			t.Errorf("subscriptionData[%q] = %v, want %q", k, sub[k], want)
		}
	}

	expectedSteps := []string{
		"status",
		"create_app",
		"update_status_2",
		"subscribe",
		"update_status_3",
		"generate_credentials",
		"update_status_4",
		"generate_secret_keys",
		"update_status_5",
		"license",
	}

	if len(steps) != len(expectedSteps) {
		t.Fatalf("got steps %v, want %v", steps, expectedSteps)
	}
	for i, step := range steps {
		if step != expectedSteps[i] {
			t.Errorf("step %d: got %s, want %s", i, step, expectedSteps[i])
		}
	}
}
