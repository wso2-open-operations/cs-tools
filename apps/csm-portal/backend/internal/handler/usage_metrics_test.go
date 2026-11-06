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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// mockUsageMetricsClient is a test double for usageMetricsClient.
type mockUsageMetricsClient struct {
	response []byte
	err      error

	lastSearch            string
	lastPayload           []byte
	lastDeployedProductID string
}

func (m *mockUsageMetricsClient) GetAllProjects(ctx context.Context, search string) ([]byte, error) {
	m.lastSearch = search
	return m.response, m.err
}
func (m *mockUsageMetricsClient) SearchInstanceMetrics(ctx context.Context, payload []byte) ([]byte, error) {
	m.lastPayload = payload
	return m.response, m.err
}
func (m *mockUsageMetricsClient) GetInstanceMetricsStats(ctx context.Context, payload []byte) ([]byte, error) {
	m.lastPayload = payload
	return m.response, m.err
}
func (m *mockUsageMetricsClient) SearchInstanceUsages(ctx context.Context, payload []byte) ([]byte, error) {
	m.lastPayload = payload
	return m.response, m.err
}
func (m *mockUsageMetricsClient) GetInstanceUsagesStats(ctx context.Context, payload []byte) ([]byte, error) {
	m.lastPayload = payload
	return m.response, m.err
}
func (m *mockUsageMetricsClient) SearchUsageMetricsDeployments(ctx context.Context, payload []byte) ([]byte, error) {
	m.lastPayload = payload
	return m.response, m.err
}
func (m *mockUsageMetricsClient) SearchUsageMetricsProjects(ctx context.Context, payload []byte) ([]byte, error) {
	m.lastPayload = payload
	return m.response, m.err
}
func (m *mockUsageMetricsClient) SearchUsageMetricsDeployedProducts(ctx context.Context, payload []byte) ([]byte, error) {
	m.lastPayload = payload
	return m.response, m.err
}
func (m *mockUsageMetricsClient) SearchUsageMetricsInstances(ctx context.Context, payload []byte) ([]byte, error) {
	m.lastPayload = payload
	return m.response, m.err
}
func (m *mockUsageMetricsClient) GetDeployedProductMetrics(ctx context.Context, deployedProductID string, payload []byte) ([]byte, error) {
	m.lastDeployedProductID = deployedProductID
	m.lastPayload = payload
	return m.response, m.err
}
func (m *mockUsageMetricsClient) GetDeployedProductUsageCounts(ctx context.Context, deployedProductID string, payload []byte) ([]byte, error) {
	m.lastDeployedProductID = deployedProductID
	m.lastPayload = payload
	return m.response, m.err
}

func TestUsageMetricsHandler_GetProjects_RequiresAuth(t *testing.T) {
	h := NewUsageMetricsHandler(&mockUsageMetricsClient{}, viewerAccessGuard)

	req := httptest.NewRequest(http.MethodGet, "/spl/usage-metrics/projects", nil)
	w := httptest.NewRecorder()
	h.GetProjects(w, req)

	assertStatus(t, w, http.StatusUnauthorized)
}

func TestUsageMetricsHandler_GetProjects_RequiresUsageMetricsPermission(t *testing.T) {
	h := NewUsageMetricsHandler(&mockUsageMetricsClient{}, viewerAccessGuard)

	// sales_solutions holds none of viewer/usage_metrics_viewer/cs_engineer/
	// admin, so it fails the direct PermUsageMetricsViewer check.
	req := httptest.NewRequest(http.MethodGet, "/usage-metrics/projects", nil)
	req = req.WithContext(middleware.WithUserInfo(req.Context(), &middleware.UserInfo{
		Email: "sales@example.com", UserID: "u-sales", Roles: []string{"test-sales-solutions"},
	}))
	w := httptest.NewRecorder()
	h.GetProjects(w, req)

	assertStatus(t, w, http.StatusForbidden)
}

// Regression: Usage Metrics used to be reachable only by a caller who ALSO
// held the viewer role (PermViewerAccess layered underneath), regardless of
// also being cs_engineer/admin/usage_metrics_viewer -- reported live once
// Support Portal Lite's separate app/nav was folded into the main portal, a
// plain cs_engineer (no viewer role at all) could not open this page. Fixed
// by registering these routes directly on PermUsageMetricsViewer instead of
// layering on PermViewerAccess.
func TestUsageMetricsHandler_GetProjects_CsEngineerWithoutViewerCanReach(t *testing.T) {
	client := &mockUsageMetricsClient{response: []byte(`[]`)}
	h := NewUsageMetricsHandler(client, viewerAccessGuard)

	req := httptest.NewRequest(http.MethodGet, "/usage-metrics/projects", nil)
	req = req.WithContext(middleware.WithUserInfo(req.Context(), &middleware.UserInfo{
		Email: "engineer@example.com", UserID: "u-eng", Roles: []string{"test-cs-engineer"},
	}))
	w := httptest.NewRecorder()
	h.GetProjects(w, req)

	assertStatus(t, w, http.StatusOK)
}

// A plain viewer (no cs_engineer/admin/usage_metrics_viewer) must NOT reach
// this domain at all -- confirmed live, correcting an earlier pass that
// mistakenly granted Viewer this permission. Unlike its ex-SPL siblings
// (Customer Health), Usage Metrics is not part of what a
// Viewer-only account gets.
func TestUsageMetricsHandler_GetProjects_PlainViewerIsDenied(t *testing.T) {
	h := NewUsageMetricsHandler(&mockUsageMetricsClient{}, viewerAccessGuard)

	req := httptest.NewRequest(http.MethodGet, "/usage-metrics/projects", nil)
	req = req.WithContext(middleware.WithUserInfo(req.Context(), &middleware.UserInfo{
		Email: "viewer@example.com", UserID: "u-viewer", Roles: []string{"test-viewer"},
	}))
	w := httptest.NewRecorder()
	h.GetProjects(w, req)

	assertStatus(t, w, http.StatusForbidden)
}

// A viewer who ALSO holds usage_metrics_viewer, cs_engineer or admin does
// reach it, same as holding that role alone.
func TestUsageMetricsHandler_GetProjects_ViewerWithQualifyingRoleHasAccess(t *testing.T) {
	client := &mockUsageMetricsClient{response: []byte(`[]`)}
	h := NewUsageMetricsHandler(client, viewerAccessGuard)

	req := httptest.NewRequest(http.MethodGet, "/usage-metrics/projects", nil)
	req = req.WithContext(middleware.WithUserInfo(req.Context(), &middleware.UserInfo{
		Email: "viewer@example.com", UserID: "u-viewer", Roles: []string{"test-viewer", "test-usage-metrics-viewer"},
	}))
	w := httptest.NewRecorder()
	h.GetProjects(w, req)

	assertStatus(t, w, http.StatusOK)
}

func TestUsageMetricsHandler_GetProjects_Success(t *testing.T) {
	client := &mockUsageMetricsClient{response: []byte(`[{"sys_id":"1"}]`)}
	h := NewUsageMetricsHandler(client, viewerAccessGuard)

	req := withUser(httptest.NewRequest(http.MethodGet, "/spl/usage-metrics/projects?search=Acme", nil))
	w := httptest.NewRecorder()
	h.GetProjects(w, req)

	assertStatus(t, w, http.StatusOK)
	if client.lastSearch != "Acme" {
		t.Errorf("client received search = %q, want %q", client.lastSearch, "Acme")
	}
	if w.Body.String() != `[{"sys_id":"1"}]` {
		t.Errorf("body = %s, want raw passthrough", w.Body.String())
	}
}

func TestUsageMetricsHandler_GetProjects_MapsUpstreamError(t *testing.T) {
	client := &mockUsageMetricsClient{err: &apierror.Error{StatusCode: http.StatusServiceUnavailable, Body: "down"}}
	h := NewUsageMetricsHandler(client, viewerAccessGuard)

	req := withUser(httptest.NewRequest(http.MethodGet, "/spl/usage-metrics/projects", nil))
	w := httptest.NewRecorder()
	h.GetProjects(w, req)

	assertStatus(t, w, http.StatusServiceUnavailable)
}

func TestUsageMetricsHandler_SearchInstanceMetrics_ForwardsBody(t *testing.T) {
	client := &mockUsageMetricsClient{response: []byte(`{"ok":true}`)}
	h := NewUsageMetricsHandler(client, viewerAccessGuard)

	body := `{"projectIds":["p1"]}`
	req := withUser(httptest.NewRequest(http.MethodPost, "/spl/usage-metrics/instances/metrics/search", strings.NewReader(body)))
	w := httptest.NewRecorder()
	h.SearchInstanceMetrics(w, req)

	assertStatus(t, w, http.StatusOK)
	if string(client.lastPayload) != body {
		t.Errorf("forwarded payload = %s, want %s", client.lastPayload, body)
	}
}

func TestUsageMetricsHandler_SearchInstanceMetrics_RejectsInvalidJSON(t *testing.T) {
	client := &mockUsageMetricsClient{}
	h := NewUsageMetricsHandler(client, viewerAccessGuard)

	req := withUser(httptest.NewRequest(http.MethodPost, "/spl/usage-metrics/instances/metrics/search", strings.NewReader("not json")))
	w := httptest.NewRecorder()
	h.SearchInstanceMetrics(w, req)

	assertStatus(t, w, http.StatusBadRequest)
	if client.lastPayload != nil {
		t.Error("upstream client should not have been called for invalid JSON")
	}
}

func TestUsageMetricsHandler_GetDeployedProductMetrics_RejectsInvalidDateRange(t *testing.T) {
	client := &mockUsageMetricsClient{response: []byte(`{}`)}
	h := NewUsageMetricsHandler(client, viewerAccessGuard)

	body := `{"deploymentId":"d1","startDate":"2026-06-01","endDate":"2026-01-01"}`
	req := withUser(httptest.NewRequest(http.MethodPost, "/spl/usage-metrics/deployed-products/dp-1/metrics/search", strings.NewReader(body)))
	req.SetPathValue("id", "dp-1")
	w := httptest.NewRecorder()
	h.GetDeployedProductMetrics(w, req)

	assertStatus(t, w, http.StatusBadRequest)
	if client.lastPayload != nil {
		t.Error("upstream client should not have been called for an invalid date range")
	}
}

func TestUsageMetricsHandler_GetDeployedProductMetrics_Success(t *testing.T) {
	client := &mockUsageMetricsClient{response: []byte(`{"summary":{}}`)}
	h := NewUsageMetricsHandler(client, viewerAccessGuard)

	body := `{"deploymentId":"d1","startDate":"2026-01-01","endDate":"2026-01-31"}`
	req := withUser(httptest.NewRequest(http.MethodPost, "/spl/usage-metrics/deployed-products/dp-1/metrics/search", strings.NewReader(body)))
	req.SetPathValue("id", "dp-1")
	w := httptest.NewRecorder()
	h.GetDeployedProductMetrics(w, req)

	assertStatus(t, w, http.StatusOK)
	if client.lastDeployedProductID != "dp-1" {
		t.Errorf("deployedProductID = %q, want %q", client.lastDeployedProductID, "dp-1")
	}
	if string(client.lastPayload) != body {
		t.Errorf("forwarded payload = %s, want the original raw body %s", client.lastPayload, body)
	}
}

func TestUsageMetricsHandler_GetDeployedProductMetrics_RequiresID(t *testing.T) {
	client := &mockUsageMetricsClient{}
	h := NewUsageMetricsHandler(client, viewerAccessGuard)

	body := `{"deploymentId":"d1","startDate":"2026-01-01","endDate":"2026-01-31"}`
	req := withUser(httptest.NewRequest(http.MethodPost, "/spl/usage-metrics/deployed-products//metrics/search", strings.NewReader(body)))
	w := httptest.NewRecorder()
	h.GetDeployedProductMetrics(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestValidateMetricsDateRange(t *testing.T) {
	tests := []struct {
		name      string
		startDate string
		endDate   string
		wantErr   bool
	}{
		{"valid same-day range", "2026-01-01", "2026-01-01", false},
		{"valid range within limit", "2026-01-01", "2026-06-01", false},
		{"end before start", "2026-06-01", "2026-01-01", true},
		{"malformed start date", "not-a-date", "2026-01-01", true},
		{"malformed end date", "2026-01-01", "not-a-date", true},
		{"range exceeds 366 days", "2025-01-01", "2026-06-01", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMetricsDateRange(tt.startDate, tt.endDate)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateMetricsDateRange(%q, %q) error = %v, wantErr %v", tt.startDate, tt.endDate, err, tt.wantErr)
			}
		})
	}
}
