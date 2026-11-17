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
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

const patchCRTestID = "11111111-2222-3333-4444-555555555555"

// recordingCRRepo is the PostgreSQL repository as far as PATCH is concerned:
// it records the request the service hands it. Every other method is the nil
// embedded interface and panics if called.
type recordingCRRepo struct {
	repository.ChangeRequestRepository
	got    *domain.PatchChangeRequestRequest
	called bool
}

func (r *recordingCRRepo) PatchChangeRequest(_ context.Context, id string, req domain.PatchChangeRequestRequest, _ string) (domain.ChangeRequest, error) {
	r.called = true
	r.got = &req
	return domain.ChangeRequest{SearchChangeRequestView: domain.SearchChangeRequestView{ID: id}}, nil
}

// patchChangeRequestOverHTTP sends body through the same chain a real request
// takes up to the repository: the x-user-id-token middleware, the handler's
// strict decoder (decodeRequest), and the PostgreSQL change request service.
func patchChangeRequestOverHTTP(t *testing.T, body string) (*httptest.ResponseRecorder, *recordingCRRepo) {
	t.Helper()
	repo := &recordingCRRepo{}
	h := NewChangeRequestHandler(service.NewChangeRequestService(repo, nil))
	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /change-requests/{id}", h.PatchChangeRequest)

	payload, _ := json.Marshal(map[string]string{"email": "jane.doe@example.com"})
	token := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`)) + "." +
		base64.RawURLEncoding.EncodeToString(payload) + ".sig"

	r := httptest.NewRequest(http.MethodPatch, "/change-requests/"+patchCRTestID, strings.NewReader(body))
	r.Header.Set("x-user-id-token", token)
	w := httptest.NewRecorder()
	middleware.UserIDToken(mux).ServeHTTP(w, r)
	return w, repo
}

// An explicit null reaches the repository as "clear", and a body whose only
// field is a null is a PATCH.
func TestPatchChangeRequestOverHTTP_ExplicitNullClears(t *testing.T) {
	for _, field := range []string{
		"implementationPlan", "priority", "category", "requestedById",
		"affectedServicesText", "affectedComponentsText", "rollbackDurationText",
	} {
		t.Run(field, func(t *testing.T) {
			w, repo := patchChangeRequestOverHTTP(t, `{"`+field+`": null}`)
			if w.Code != http.StatusOK {
				t.Fatalf(`{"%s": null}: status %d, body %s`, field, w.Code, w.Body.String())
			}
			if !repo.called {
				t.Fatal("the repository was never reached")
			}
			var cleared bool
			switch field {
			case "implementationPlan":
				cleared = repo.got.ImplementationPlan != nil && *repo.got.ImplementationPlan == nil
			case "priority":
				cleared = repo.got.Priority != nil && *repo.got.Priority == nil
			case "category":
				cleared = repo.got.Category != nil && *repo.got.Category == nil
			case "requestedById":
				cleared = repo.got.RequestedByID != nil && *repo.got.RequestedByID == nil
			case "affectedServicesText":
				cleared = repo.got.AffectedServicesText != nil && *repo.got.AffectedServicesText == nil
			case "affectedComponentsText":
				cleared = repo.got.AffectedComponentsText != nil && *repo.got.AffectedComponentsText == nil
			case "rollbackDurationText":
				cleared = repo.got.RollbackDurationText != nil && *repo.got.RollbackDurationText == nil
			}
			if !cleared {
				t.Fatalf("the repository did not receive %s as an explicit null: %+v", field, *repo.got)
			}
		})
	}
}

// customerGroupId and environmentIds are no longer accepted, null included.
func TestPatchChangeRequestOverHTTP_RemovedFieldsRefuseNull(t *testing.T) {
	for body, want := range map[string]string{
		`{"customerGroupId": null, "title": "t"}`: "customerGroupId is no longer accepted",
		`{"environmentIds": null, "title": "t"}`:  "environmentIds is no longer supported",
	} {
		w, repo := patchChangeRequestOverHTTP(t, body)
		if w.Code != http.StatusBadRequest || repo.called || !strings.Contains(w.Body.String(), want) {
			t.Errorf("%s: status %d %s (repository reached: %v), want a 400 saying %q before the repository", body, w.Code, w.Body.String(), repo.called, want)
		}
	}
}

func TestPatchChangeRequestOverHTTP_UnknownFieldStillRefused(t *testing.T) {
	w, repo := patchChangeRequestOverHTTP(t, `{"implementationPlan": null, "bogus": 1}`)
	if w.Code != http.StatusBadRequest || repo.called {
		t.Fatalf("status %d (repository reached: %v), want 400", w.Code, repo.called)
	}
	if !strings.Contains(w.Body.String(), `unknown field \"bogus\"`) {
		t.Fatalf("body %s does not name the unknown field", w.Body.String())
	}
}

// A PATCH that only sets the service (or only the service offering) is a PATCH.
func TestPatchChangeRequestOverHTTP_ServiceAloneIsAPatch(t *testing.T) {
	const svcID = "66666666-7777-8888-9999-000000000000"
	for name, body := range map[string]string{
		"serviceId":         `{"serviceId": "` + svcID + `"}`,
		"serviceOfferingId": `{"serviceOfferingId": "` + svcID + `"}`,
	} {
		t.Run(name, func(t *testing.T) {
			w, repo := patchChangeRequestOverHTTP(t, body)
			if w.Code != http.StatusOK || !repo.called {
				t.Fatalf("status %d (repository reached: %v), body %s", w.Code, repo.called, w.Body.String())
			}
			if (name == "serviceId" && (repo.got.ServiceID == nil || *repo.got.ServiceID != svcID)) ||
				(name == "serviceOfferingId" && (repo.got.ServiceOfferingID == nil || *repo.got.ServiceOfferingID != svcID)) {
				t.Fatalf("the repository did not receive %s: %+v", name, *repo.got)
			}
		})
	}
}

// An unknown priority is a 400 before the repository, and so is planning, which
// the enum column holds for migrated rows but no write sets.
func TestPatchChangeRequestOverHTTP_PriorityIsValidated(t *testing.T) {
	const want = `"priority must be one of critical, high, moderate or low"`
	for _, p := range []string{"planning", "PLANNING", "urgent", " high ", ""} {
		w, repo := patchChangeRequestOverHTTP(t, `{"priority": "`+p+`"}`)
		if w.Code != http.StatusBadRequest || repo.called || !strings.Contains(w.Body.String(), want) {
			t.Errorf("priority %q: status %d %s (repository reached: %v), want 400 %s", p, w.Code, w.Body.String(), repo.called, want)
		}
	}
	for _, p := range []string{"critical", "high", "moderate", "low"} {
		w, repo := patchChangeRequestOverHTTP(t, `{"priority": "`+p+`"}`)
		if w.Code != http.StatusOK || !repo.called {
			t.Errorf("priority %q: status %d (repository reached: %v), want 200", p, w.Code, repo.called)
		}
	}
}
