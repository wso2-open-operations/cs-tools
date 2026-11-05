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
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

const testDeploymentID = "22222222-2222-2222-2222-222222222222"

// attachFakeEntity records what the handler sent upstream.
type attachFakeEntity struct {
	entityDeploymentClient
	gotSearch entity.SearchAttachmentsRequest
}

func (f *attachFakeEntity) SearchAttachments(_ context.Context, req entity.SearchAttachmentsRequest) (entity.SearchAttachmentsResponse, error) {
	f.gotSearch = req
	return entity.SearchAttachmentsResponse{Total: 0, Limit: req.Pagination.Limit, Offset: req.Pagination.Offset}, nil
}

// TestSearchDeploymentAttachments_ScopesToPathDeployment checks the route reaches
// the handler with deploymentId populated, and that the upstream search is scoped
// by the path value and forced to referenceType=deployment — the deployment must
// never come from a query parameter or body.
func TestSearchDeploymentAttachments_ScopesToPathDeployment(t *testing.T) {
	fake := &attachFakeEntity{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /deployments/{deploymentId}/attachments", NewDeploymentHandler(fake).SearchDeploymentAttachments)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, authedRequest(http.MethodGet, "/deployments/"+testDeploymentID+"/attachments?limit=25&offset=50", ""))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	if fake.gotSearch.ReferenceID != testDeploymentID {
		t.Errorf("ReferenceID = %q, want the path deploymentId %q", fake.gotSearch.ReferenceID, testDeploymentID)
	}
	if fake.gotSearch.ReferenceType != entity.ReferenceTypeDeployment {
		t.Errorf("ReferenceType = %q, want %q", fake.gotSearch.ReferenceType, entity.ReferenceTypeDeployment)
	}
	if fake.gotSearch.Pagination.Limit != 25 || fake.gotSearch.Pagination.Offset != 50 {
		t.Errorf("pagination = %+v, want limit 25 offset 50 from the query string", fake.gotSearch.Pagination)
	}
}

// TestSearchDeploymentAttachments_RejectsNonUUID guards the path-param check.
func TestSearchDeploymentAttachments_RejectsNonUUID(t *testing.T) {
	fake := &attachFakeEntity{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /deployments/{deploymentId}/attachments", NewDeploymentHandler(fake).SearchDeploymentAttachments)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, authedRequest(http.MethodGet, "/deployments/not-a-uuid/attachments", ""))

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
	if fake.gotSearch.ReferenceID != "" {
		t.Error("upstream was called despite an invalid deploymentId")
	}
}
