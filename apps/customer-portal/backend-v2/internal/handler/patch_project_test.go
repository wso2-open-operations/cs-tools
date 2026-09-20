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
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// fakeEntityProjectClient records the UpdateProjectRequest it received.
// entityProjectClient is embedded (nil) so only UpdateProject needs
// implementing; anything else would nil-panic, which these tests never do.
type fakeEntityProjectClient struct {
	entityProjectClient
	gotID  string
	gotReq entity.UpdateProjectRequest
	called bool
	err    error
}

func (f *fakeEntityProjectClient) UpdateProject(_ context.Context, id string, req entity.UpdateProjectRequest) (entity.UpdateProjectResponse, error) {
	f.called, f.gotID, f.gotReq = true, id, req
	if f.err != nil {
		return entity.UpdateProjectResponse{}, f.err
	}
	return entity.UpdateProjectResponse{
		Message: "updated",
		Project: entity.UpdatedProjectRef{ID: id, Name: "Acme", HasAgent: req.HasAgent, HasKbReferences: req.HasKbReferences},
	}, nil
}

// TestPatchProject_RouteExists is the regression test for the bug this fixes:
// the Ballerina backend served PATCH /projects/{id} and backend-v2 did not, so
// toggling the AI assistant returned 404 from the mux rather than reaching any
// handler. Registered through a real ServeMux with main.go's exact pattern.
func TestPatchProject_RouteExists(t *testing.T) {
	fake := &fakeEntityProjectClient{}
	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /projects/{id}", NewProjectHandler(fake).PatchProject)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, authedRequest(http.MethodPatch, "/projects/"+testProjectID, `{"hasAgent":true}`))

	if rec.Code == http.StatusNotFound {
		t.Fatal("PATCH /projects/{id} returned 404: the route is not registered")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body: %s", rec.Code, rec.Body.String())
	}
	if !fake.called {
		t.Fatal("handler did not call entity UpdateProject")
	}
	if fake.gotID != testProjectID {
		t.Errorf("project id = %q, want %q", fake.gotID, testProjectID)
	}
	if fake.gotReq.HasAgent == nil || !*fake.gotReq.HasAgent {
		t.Errorf("hasAgent = %v, want true", fake.gotReq.HasAgent)
	}
	if fake.gotReq.HasKbReferences != nil {
		t.Errorf("hasKbReferences = %v, want nil — an absent field must stay absent upstream", fake.gotReq.HasKbReferences)
	}
}

// TestPatchProject_ExactlyOneField pins the rule entity-service enforces and
// the Ballerina backend checked before calling it. The false cases matter most:
// with plain bools rather than pointers, "hasAgent":false would be
// indistinguishable from an absent field and every request would look like it
// set both.
func TestPatchProject_ExactlyOneField(t *testing.T) {
	tests := []struct {
		name, body string
		wantStatus int
		wantCalled bool
	}{
		{"one field, true", `{"hasAgent":true}`, http.StatusOK, true},
		{"one field, false", `{"hasAgent":false}`, http.StatusOK, true},
		{"the other field", `{"hasKbReferences":true}`, http.StatusOK, true},
		{"both fields", `{"hasAgent":true,"hasKbReferences":true}`, http.StatusBadRequest, false},
		{"both, one false", `{"hasAgent":false,"hasKbReferences":false}`, http.StatusBadRequest, false},
		{"no fields", `{}`, http.StatusBadRequest, false},
		{"malformed", `{"hasAgent":`, http.StatusBadRequest, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeEntityProjectClient{}
			rec := httptest.NewRecorder()
			req := authedRequest(http.MethodPatch, "/projects/"+testProjectID, tt.body)
			req.SetPathValue("id", testProjectID)

			NewProjectHandler(fake).PatchProject(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d. body: %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if fake.called != tt.wantCalled {
				t.Errorf("upstream called = %v, want %v", fake.called, tt.wantCalled)
			}
		})
	}
}

// TestPatchProject_ReturnsTheProject: the Ballerina resource returns
// response.project, not the whole {message, project} envelope, and the frontend
// reads it that way.
func TestPatchProject_ReturnsTheProject(t *testing.T) {
	fake := &fakeEntityProjectClient{}
	rec := httptest.NewRecorder()
	req := authedRequest(http.MethodPatch, "/projects/"+testProjectID, `{"hasKbReferences":true}`)
	req.SetPathValue("id", testProjectID)

	NewProjectHandler(fake).PatchProject(rec, req)

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response is not JSON: %v (%s)", err, rec.Body.String())
	}
	if _, wrapped := got["project"]; wrapped {
		t.Error("response is the {message, project} envelope; the frontend expects the project itself")
	}
	if got["id"] != testProjectID {
		t.Errorf("id = %v, want %q", got["id"], testProjectID)
	}
	if got["hasKbReferences"] != true {
		t.Errorf("hasKbReferences = %v, want true", got["hasKbReferences"])
	}
}

// TestPatchProject_RejectsNonUUID keeps the id check consistent with every
// other project route here.
func TestPatchProject_RejectsNonUUID(t *testing.T) {
	fake := &fakeEntityProjectClient{}
	rec := httptest.NewRecorder()
	req := authedRequest(http.MethodPatch, "/projects/not-a-uuid", `{"hasAgent":true}`)
	req.SetPathValue("id", "not-a-uuid")

	NewProjectHandler(fake).PatchProject(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	if fake.called {
		t.Error("upstream was called with a non-UUID project id")
	}
}

// TestPatchProject_RejectsUnknownFields: the Ballerina payload this replaces is
// a closed record (record {| ... |}), so it rejects a property it does not
// declare, and openapi.yaml says additionalProperties: false. Without a strict
// decode, a caller misspelling hasKbReferences would get 200 and no change.
func TestPatchProject_RejectsUnknownFields(t *testing.T) {
	tests := []struct{ name, body string }{
		{"a valid field plus an unknown one", `{"hasAgent":true,"bogus":1}`},
		{"a plausible misspelling", `{"hasKbReference":true}`},
		{"only an unknown field", `{"enabled":true}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeEntityProjectClient{}
			rec := httptest.NewRecorder()
			req := authedRequest(http.MethodPatch, "/projects/"+testProjectID, tt.body)
			req.SetPathValue("id", testProjectID)

			NewProjectHandler(fake).PatchProject(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400. body: %s", rec.Code, rec.Body.String())
			}
			if fake.called {
				t.Error("the project was updated despite an undocumented property in the request")
			}
		})
	}
}
