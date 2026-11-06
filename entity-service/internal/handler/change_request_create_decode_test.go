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

// recordingCreateCRRepo is the PostgreSQL repository as far as a create is
// concerned: it records the request each create method is handed. Every other
// method is the nil embedded interface and panics if called.
type recordingCreateCRRepo struct {
	repository.ChangeRequestRepository
	got *domain.CreateChangeRequestRequest
}

func (r *recordingCreateCRRepo) CreateChangeRequest(_ context.Context, req domain.CreateChangeRequestRequest, _ string) (domain.CreateChangeRequestResponse, error) {
	r.got = &req
	return domain.CreateChangeRequestResponse{Message: "created"}, nil
}

func (r *recordingCreateCRRepo) CreateChangeRequestFromServiceNow(_ context.Context, req domain.CreateChangeRequestRequest, _, _, _ string) (domain.CreateChangeRequestResponse, error) {
	r.got = &req
	return domain.CreateChangeRequestResponse{Message: "created"}, nil
}

func (r *recordingCreateCRRepo) ValidateChangeRequestLinks(context.Context, domain.ChangeRequestLinkSelection) (domain.ChangeRequestLinkSet, error) {
	return domain.ChangeRequestLinkSet{}, nil
}

// recordingCreateMirror stands in for the previous system's change request
// service on the create that writes that system first. Only CreateChangeRequest
// is reachable.
type recordingCreateMirror struct {
	service.ChangeRequestService
	got *domain.CreateChangeRequestRequest
}

func (m *recordingCreateMirror) CreateChangeRequest(_ context.Context, req domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error) {
	m.got = &req
	resp := domain.CreateChangeRequestResponse{Message: "created"}
	resp.ChangeRequest.ID = "22222222-3333-4444-5555-666666666666"
	resp.ChangeRequest.Number = "CHG0001"
	resp.ChangeRequest.CreatedBy = "jane.doe@example.com"
	return resp, nil
}

// createChangeRequestOverHTTP sends body through the chain a real create takes
// up to the repository (and, on the create that writes the previous system
// first, up to that system): the x-user-id-token middleware, the handler's strict
// decoder and the change request service. It returns the response, the message
// and what the repository and the previous system received (nil when not called).
func createChangeRequestOverHTTP(t *testing.T, previousSystemFirst bool, body string) (*httptest.ResponseRecorder, string, *recordingCreateCRRepo, *recordingCreateMirror) {
	t.Helper()
	repo := &recordingCreateCRRepo{}
	mirror := &recordingCreateMirror{}
	svc := service.NewChangeRequestService(repo, nil)
	if previousSystemFirst {
		svc = service.NewChangeRequestServiceWithSNMirror(repo, nil, mirror)
	}
	h := NewChangeRequestHandler(svc)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /change-requests", h.CreateChangeRequest)

	payload, _ := json.Marshal(map[string]string{"email": "jane.doe@example.com"})
	token := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`)) + "." +
		base64.RawURLEncoding.EncodeToString(payload) + ".sig"

	r := httptest.NewRequest(http.MethodPost, "/change-requests", strings.NewReader(body))
	r.Header.Set("x-user-id-token", token)
	w := httptest.NewRecorder()
	middleware.UserIDToken(mux).ServeHTTP(w, r)
	var out struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w, out.Message, repo, mirror
}

// Priority, impact and risk are checked against the values this API sets on both
// create paths, before the repository or the previous system is called.
// planning is refused although change_request_priority_enum holds it.
func TestCreateChangeRequestOverHTTP_LevelsAreValidated(t *testing.T) {
	const priorityMsg = "priority must be one of critical, high, moderate or low"
	refused := map[string]string{
		`"priority": "planning"`: priorityMsg,
		`"priority": "PLANNING"`: priorityMsg,
		`"priority": "urgent"`:   priorityMsg,
		`"priority": " high "`:   priorityMsg,
		`"priority": ""`:         priorityMsg,
		`"impact": "severe"`:     "impact must be one of high, medium or low",
		`"impact": "moderate"`:   "impact must be one of high, medium or low",
		`"risk": "extreme"`:      "risk must be one of high, moderate or low",
		`"risk": "medium"`:       "risk must be one of high, moderate or low",
	}
	for _, previousSystemFirst := range []bool{false, true} {
		for field, want := range refused {
			w, msg, repo, mirror := createChangeRequestOverHTTP(t, previousSystemFirst,
				`{"subject": "s", "type": "normal", `+field+`}`)
			if w.Code != http.StatusBadRequest || msg != want {
				t.Errorf("previous system first %v, %s: %d %q, want 400 %q", previousSystemFirst, field, w.Code, msg, want)
			}
			if repo.got != nil || mirror.got != nil {
				t.Errorf("previous system first %v, %s: the repository (%v) or the previous system (%v) was called",
					previousSystemFirst, field, repo.got != nil, mirror.got != nil)
			}
		}
	}
}

func TestCreateChangeRequestOverHTTP_SettableLevelsAreCreated(t *testing.T) {
	for _, previousSystemFirst := range []bool{false, true} {
		for _, p := range []domain.ChangeRequestPriority{
			domain.ChangeRequestPriorityCritical, domain.ChangeRequestPriorityHigh,
			domain.ChangeRequestPriorityModerate, domain.ChangeRequestPriorityLow,
		} {
			w, msg, repo, mirror := createChangeRequestOverHTTP(t, previousSystemFirst,
				`{"subject": "s", "type": "normal", "priority": "`+string(p)+`", "impact": "medium", "risk": "moderate"}`)
			if w.Code != http.StatusCreated {
				t.Fatalf("previous system first %v, priority %s: %d %q, want 201", previousSystemFirst, p, w.Code, msg)
			}
			if repo.got == nil || repo.got.Priority == nil || *repo.got.Priority != p {
				t.Errorf("previous system first %v: the repository did not receive priority %s", previousSystemFirst, p)
			}
			if previousSystemFirst && (mirror.got == nil || mirror.got.Priority == nil || *mirror.got.Priority != p) {
				t.Errorf("the previous system did not receive priority %s", p)
			}
		}
	}
}
