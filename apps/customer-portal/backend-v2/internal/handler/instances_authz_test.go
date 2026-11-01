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

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
)

// fakeInstanceEntityClient records whether the project-access gate (GetProject)
// and each instance read were called, so a test can prove the gate runs before
// the read — and blocks it — on the project-scoped routes.
type fakeInstanceEntityClient struct {
	getProjectErr   error
	getProjectCalls int
	searchCalls     int
}

func (f *fakeInstanceEntityClient) GetProject(context.Context, string) (entity.ProjectDetailsView, error) {
	f.getProjectCalls++
	return entity.ProjectDetailsView{}, f.getProjectErr
}

func (f *fakeInstanceEntityClient) SearchInstances(context.Context, entity.SearchInstancesRequest) (entity.SearchInstancesResponse, error) {
	f.searchCalls++
	return entity.SearchInstancesResponse{}, nil
}

func (f *fakeInstanceEntityClient) SearchInstanceMetrics(context.Context, entity.InstanceMetricsRequest) (entity.InstanceMetricsResponse, error) {
	f.searchCalls++
	return entity.InstanceMetricsResponse{}, nil
}

func (f *fakeInstanceEntityClient) SearchInstanceUsage(context.Context, entity.InstanceUsageRequest) (entity.InstanceUsageResponse, error) {
	f.searchCalls++
	return entity.InstanceUsageResponse{}, nil
}

func (f *fakeInstanceEntityClient) SearchInstanceMetricsStats(context.Context, entity.InstanceMetricsStatsRequest) (entity.InstanceMetricsStatsResponse, error) {
	f.searchCalls++
	return entity.InstanceMetricsStatsResponse{}, nil
}

func (f *fakeInstanceEntityClient) SearchInstanceUsageStats(context.Context, entity.InstanceUsageStatsRequest) (entity.InstanceUsageStatsResponse, error) {
	f.searchCalls++
	return entity.InstanceUsageStatsResponse{}, nil
}

func authedInstanceRequest(target string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(`{"startDate":"2026-09-01","endDate":"2026-10-28"}`))
	req.Header.Set("Content-Type", "application/json")
	return req.WithContext(middleware.WithUserInfo(req.Context(), &middleware.UserInfo{
		UserID: "user-1",
		Email:  "customer@example.com",
	}))
}

const (
	projectAUUID    = "11111111-1111-1111-1111-111111111111"
	deploymentUUID  = "22222222-2222-2222-2222-222222222222"
	deployedProdID  = "33333333-3333-3333-3333-333333333333"
	forbiddenReason = "secret-project-name-xyz"
)

// Every project-scoped instance route calls GetProject as an access check
// before the read, and does not run the read when entity-service refuses it.
func TestProjectInstanceRoutes_RefuseWhenProjectAccessDenied(t *testing.T) {
	cases := []struct {
		name   string
		invoke func(h *InstanceHandler, w http.ResponseWriter, r *http.Request)
	}{
		{"instances", func(h *InstanceHandler, w http.ResponseWriter, r *http.Request) { h.SearchProjectInstances(w, r) }},
		{"metrics", func(h *InstanceHandler, w http.ResponseWriter, r *http.Request) { h.SearchProjectInstanceMetrics(w, r) }},
		{"usages", func(h *InstanceHandler, w http.ResponseWriter, r *http.Request) { h.SearchProjectInstanceUsage(w, r) }},
		{"metrics-stats", func(h *InstanceHandler, w http.ResponseWriter, r *http.Request) {
			h.SearchProjectInstanceMetricsStats(w, r)
		}},
		{"usages-stats", func(h *InstanceHandler, w http.ResponseWriter, r *http.Request) {
			h.SearchProjectInstanceUsageStats(w, r)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			entityClient := &fakeInstanceEntityClient{
				getProjectErr: apierror.NewUpstreamError(http.StatusForbidden, []byte(`{"message":"`+forbiddenReason+`"}`)),
			}
			h := NewInstanceHandler(entityClient)
			req := authedInstanceRequest("/projects/" + projectAUUID + "/instances/search")
			req.SetPathValue("id", projectAUUID)
			rec := httptest.NewRecorder()

			c.invoke(h, rec, req)

			if rec.Code != http.StatusForbidden {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
			}
			if entityClient.getProjectCalls != 1 {
				t.Errorf("GetProject calls = %d, want 1", entityClient.getProjectCalls)
			}
			if entityClient.searchCalls != 0 {
				t.Errorf("instance read ran despite denied access: searchCalls = %d, want 0", entityClient.searchCalls)
			}
		})
	}
}

// When the caller may access the project, the gate passes and the read runs.
func TestProjectInstanceRoute_AllowsWhenProjectAccessGranted(t *testing.T) {
	entityClient := &fakeInstanceEntityClient{} // GetProject returns no error
	h := NewInstanceHandler(entityClient)
	req := authedInstanceRequest("/projects/" + projectAUUID + "/instances/usages/search")
	req.SetPathValue("id", projectAUUID)
	rec := httptest.NewRecorder()

	h.SearchProjectInstanceUsage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if entityClient.getProjectCalls != 1 {
		t.Errorf("GetProject calls = %d, want 1", entityClient.getProjectCalls)
	}
	if entityClient.searchCalls != 1 {
		t.Errorf("searchCalls = %d, want 1", entityClient.searchCalls)
	}
}

// The deployment- and deployed-product-scoped routes rely on entity-service's
// own scoping, not on a project-membership check, so they must NOT
// call GetProject — doing so would be both wrong (no project in scope) and a
// needless round trip.
func TestNonProjectInstanceRoutes_DoNotCallProjectGate(t *testing.T) {
	for _, c := range []struct {
		name   string
		path   string
		pathID string
		invoke func(h *InstanceHandler, w http.ResponseWriter, r *http.Request)
	}{
		{"deployment", deploymentUUID, deploymentUUID, func(h *InstanceHandler, w http.ResponseWriter, r *http.Request) {
			h.SearchDeploymentInstanceUsage(w, r)
		}},
		{"deployed-product", deployedProdID, deployedProdID, func(h *InstanceHandler, w http.ResponseWriter, r *http.Request) {
			h.SearchDeployedProductInstanceUsage(w, r)
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			entityClient := &fakeInstanceEntityClient{}
			h := NewInstanceHandler(entityClient)
			req := authedInstanceRequest("/x/" + c.path + "/instances/usages/search")
			req.SetPathValue("id", c.pathID)
			rec := httptest.NewRecorder()

			c.invoke(h, rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
			}
			if entityClient.getProjectCalls != 0 {
				t.Errorf("GetProject called on a non-project route: calls = %d, want 0", entityClient.getProjectCalls)
			}
			if entityClient.searchCalls != 1 {
				t.Errorf("searchCalls = %d, want 1", entityClient.searchCalls)
			}
		})
	}
}

// A denied project access is reported without leaking the upstream reason text
// (403 uses the fixed forbidden message, as mapUpstreamError does elsewhere).
func TestProjectInstanceRoute_DeniedResponseHidesUpstreamReason(t *testing.T) {
	entityClient := &fakeInstanceEntityClient{
		getProjectErr: apierror.NewUpstreamError(http.StatusForbidden, []byte(`{"message":"`+forbiddenReason+`"}`)),
	}
	h := NewInstanceHandler(entityClient)
	req := authedInstanceRequest("/projects/" + projectAUUID + "/instances/search")
	req.SetPathValue("id", projectAUUID)
	rec := httptest.NewRecorder()

	h.SearchProjectInstances(rec, req)

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["message"] != ErrMsgForbidden {
		t.Errorf("message = %v, want the fixed forbidden message", body["message"])
	}
	if strings.Contains(rec.Body.String(), forbiddenReason) {
		t.Errorf("response leaked upstream reason text: %s", rec.Body.String())
	}
}
