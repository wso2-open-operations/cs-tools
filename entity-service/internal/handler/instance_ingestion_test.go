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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

type mockInstanceIngestionSvc struct {
	upsertMetaRes domain.InstanceMetaResponse
	upsertMetaErr error

	insertInfoRes domain.InstanceInfoResponse
	insertInfoErr error

	insertUsageRes domain.InstanceUsageIngestResponse
	insertUsageErr error

	importBatchRes domain.DailyUsageBatchResponse
	importBatchErr error

	getSecretRes domain.ProjectSecretKeyResponse
	getSecretErr error
	gotKey       string

	depSecretRes domain.DeploymentConsumptionSecretResponse
	depSecretErr error
	gotNumber    string
}

func (m *mockInstanceIngestionSvc) GetDeploymentConsumptionSecret(_ context.Context, number string) (domain.DeploymentConsumptionSecretResponse, error) {
	m.gotNumber = number
	if m.depSecretErr != nil {
		return domain.DeploymentConsumptionSecretResponse{}, m.depSecretErr
	}
	return m.depSecretRes, nil
}

func (m *mockInstanceIngestionSvc) UpsertInstanceMeta(_ context.Context, _ domain.InstanceMetaRequest) (domain.InstanceMetaResponse, error) {
	if m.upsertMetaErr != nil {
		return domain.InstanceMetaResponse{}, m.upsertMetaErr
	}
	return m.upsertMetaRes, nil
}

func (m *mockInstanceIngestionSvc) InsertInstanceInfo(_ context.Context, _ domain.InstanceInfoRequest) (domain.InstanceInfoResponse, error) {
	if m.insertInfoErr != nil {
		return domain.InstanceInfoResponse{}, m.insertInfoErr
	}
	return m.insertInfoRes, nil
}

func (m *mockInstanceIngestionSvc) InsertUsageStats(_ context.Context, _ domain.InstanceUsageIngestRequest) (domain.InstanceUsageIngestResponse, error) {
	if m.insertUsageErr != nil {
		return domain.InstanceUsageIngestResponse{}, m.insertUsageErr
	}
	return m.insertUsageRes, nil
}

func (m *mockInstanceIngestionSvc) ImportDailyUsagesBatch(_ context.Context, _ domain.DailyUsageBatchRequest) (domain.DailyUsageBatchResponse, error) {
	if m.importBatchErr != nil {
		return domain.DailyUsageBatchResponse{}, m.importBatchErr
	}
	return m.importBatchRes, nil
}

func (m *mockInstanceIngestionSvc) GetProjectSecretKey(_ context.Context, key string) (domain.ProjectSecretKeyResponse, error) {
	m.gotKey = key
	if m.getSecretErr != nil {
		return domain.ProjectSecretKeyResponse{}, m.getSecretErr
	}
	return m.getSecretRes, nil
}

func TestInstanceIngestionHandler(t *testing.T) {
	t.Run("UpsertInstanceMeta 200 OK", func(t *testing.T) {
		svc := &mockInstanceIngestionSvc{
			upsertMetaRes: domain.InstanceMetaResponse{ID: "node-uuid-123"},
		}
		h := NewInstanceIngestionHandler(svc)

		reqBody := `{"nodeId":"node-1","projectKey":"PRJ","deploymentNumber":"DEP1","product":"MI","version":"4.0.0"}`
		req := httptest.NewRequest(http.MethodPost, "/instances/meta", strings.NewReader(reqBody))
		w := httptest.NewRecorder()

		h.UpsertInstanceMeta(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var resp domain.InstanceMetaResponse
		_ = json.NewDecoder(w.Body).Decode(&resp)
		if resp.ID != "node-uuid-123" {
			t.Errorf("expected node-uuid-123, got %q", resp.ID)
		}
	})

	t.Run("UpsertInstanceMeta 400 Validation Error", func(t *testing.T) {
		svc := &mockInstanceIngestionSvc{
			upsertMetaErr: &apierror.ValidationError{Msg: "nodeId is required"},
		}
		h := NewInstanceIngestionHandler(svc)

		req := httptest.NewRequest(http.MethodPost, "/instances/meta", strings.NewReader(`{}`))
		w := httptest.NewRecorder()

		h.UpsertInstanceMeta(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("InsertInstanceInfo 200 OK", func(t *testing.T) {
		svc := &mockInstanceIngestionSvc{
			insertInfoRes: domain.InstanceInfoResponse{ID: "info-uuid-123"},
		}
		h := NewInstanceIngestionHandler(svc)

		reqBody := `{"nodeId":"node-1","jdkVersion":"17","coreCount":8}`
		req := httptest.NewRequest(http.MethodPost, "/instances/info", strings.NewReader(reqBody))
		w := httptest.NewRecorder()

		h.InsertInstanceInfo(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}
	})

	t.Run("InsertUsageStats 200 OK", func(t *testing.T) {
		svc := &mockInstanceIngestionSvc{
			insertUsageRes: domain.InstanceUsageIngestResponse{Status: "SUCCESS"},
		}
		h := NewInstanceIngestionHandler(svc)

		reqBody := `{"nodeId":"node-1","countType":"CORES","count":16}`
		req := httptest.NewRequest(http.MethodPost, "/instances/usage-stats", strings.NewReader(reqBody))
		w := httptest.NewRecorder()

		h.InsertUsageStats(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}
	})

	t.Run("ImportDailyUsagesBatch 200 OK", func(t *testing.T) {
		svc := &mockInstanceIngestionSvc{
			importBatchRes: domain.DailyUsageBatchResponse{ImportedCount: 3, Status: "SUCCESS"},
		}
		h := NewInstanceIngestionHandler(svc)

		reqBody := `{"nodeId":"node-1","usages":[{"date":"2026-10-15","type":"CORES","value":8}]}`
		req := httptest.NewRequest(http.MethodPost, "/instances/daily-usages/batch", strings.NewReader(reqBody))
		w := httptest.NewRecorder()

		h.ImportDailyUsagesBatch(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}
		var resp domain.DailyUsageBatchResponse
		_ = json.NewDecoder(w.Body).Decode(&resp)
		if resp.ImportedCount != 3 {
			t.Errorf("expected 3, got %d", resp.ImportedCount)
		}
	})

	t.Run("GetProjectSecretKey 200 OK", func(t *testing.T) {
		svc := &mockInstanceIngestionSvc{
			getSecretRes: domain.ProjectSecretKeyResponse{SecretKey: "secret-abc"},
		}
		h := NewInstanceIngestionHandler(svc)

		req := httptest.NewRequest(http.MethodGet, "/projects/CUPRSUBDEV/secret-key", nil)
		req.SetPathValue("key", "CUPRSUBDEV")
		w := httptest.NewRecorder()

		h.GetProjectSecretKey(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}
		if svc.gotKey != "CUPRSUBDEV" {
			t.Errorf("service got key %q, want CUPRSUBDEV", svc.gotKey)
		}
		if got := w.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("Cache-Control = %q, want no-store: the body is a credential", got)
		}
		var resp domain.ProjectSecretKeyResponse
		_ = json.NewDecoder(w.Body).Decode(&resp)
		if resp.SecretKey != "secret-abc" {
			t.Errorf("expected 'secret-abc', got %q", resp.SecretKey)
		}
	})

	t.Run("GetProjectSecretKey 404 when the project has no key", func(t *testing.T) {
		svc := &mockInstanceIngestionSvc{
			getSecretErr: &apierror.NotFoundError{Msg: `project "X" has no product consumption secret key`},
		}
		h := NewInstanceIngestionHandler(svc)

		req := httptest.NewRequest(http.MethodGet, "/projects/X/secret-key", nil)
		req.SetPathValue("key", "X")
		w := httptest.NewRecorder()

		h.GetProjectSecretKey(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", w.Code)
		}
	})

	t.Run("GetDeploymentConsumptionSecret 200 OK", func(t *testing.T) {
		svc := &mockInstanceIngestionSvc{depSecretRes: domain.DeploymentConsumptionSecretResponse{
			DeploymentNumber: "DEP1", ProjectID: "p-1", ProjectKey: "PROJ", SecretKey: "abc",
		}}
		h := NewInstanceIngestionHandler(svc)
		req := httptest.NewRequest(http.MethodGet, "/deployments/by-number/DEP1/consumption-secret", nil)
		req.SetPathValue("number", "DEP1")
		w := httptest.NewRecorder()

		h.GetDeploymentConsumptionSecret(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if svc.gotNumber != "DEP1" {
			t.Errorf("service got number %q", svc.gotNumber)
		}
		if got := w.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("Cache-Control = %q, want no-store", got)
		}
		var resp domain.DeploymentConsumptionSecretResponse
		_ = json.NewDecoder(w.Body).Decode(&resp)
		if resp.ProjectKey != "PROJ" || resp.SecretKey != "abc" || resp.ProjectID != "p-1" {
			t.Errorf("response = %+v", resp)
		}
	})

	t.Run("GetDeploymentConsumptionSecret 404", func(t *testing.T) {
		svc := &mockInstanceIngestionSvc{depSecretErr: &apierror.NotFoundError{Msg: "deployment not found"}}
		h := NewInstanceIngestionHandler(svc)
		req := httptest.NewRequest(http.MethodGet, "/deployments/by-number/X/consumption-secret", nil)
		req.SetPathValue("number", "X")
		w := httptest.NewRecorder()

		h.GetDeploymentConsumptionSecret(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", w.Code)
		}
	})
}
