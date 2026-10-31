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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// noAutocloseRepo fails the test if the service reaches the repository: every
// request in these tests must be refused before that.
type noAutocloseRepo struct{ t *testing.T }

func (r noAutocloseRepo) ListIncidentsDueForAutoclose(context.Context, int, int) ([]repository.IncidentAutocloseCandidate, error) {
	r.t.Fatal("repository reached")
	return nil, nil
}

func (r noAutocloseRepo) AutocloseIncident(context.Context, string, int, string) (bool, error) {
	r.t.Fatal("repository reached")
	return false, nil
}

func (r noAutocloseRepo) RecordIncidentFieldChanges(context.Context, string, []repository.IncidentFieldChange, string) error {
	r.t.Fatal("repository reached")
	return nil
}

// TestIncidentAutocloseHandler_RefusesBadBodies: an unknown field, a missing
// resolvedForDays or one outside 1..3650 and a limit over 1000 are 400s, and
// nothing is selected.
func TestIncidentAutocloseHandler_RefusesBadBodies(t *testing.T) {
	h := NewIncidentAutocloseHandler(service.NewIncidentAutocloseService(noAutocloseRepo{t}))
	for name, body := range map[string]string{
		"unknown field":     `{"resolvedForDays":14,"days":14}`,
		"missing days":      `{"limit":10}`,
		"zero days":         `{"resolvedForDays":0}`,
		"negative days":     `{"resolvedForDays":-1}`,
		"days over 3650":    `{"resolvedForDays":3651}`,
		"days past int32":   `{"resolvedForDays":3000000000}`,
		"limit over 1000":   `{"resolvedForDays":14,"limit":1001}`,
		"negative limit":    `{"resolvedForDays":14,"limit":-5}`,
		"days not a number": `{"resolvedForDays":"14"}`,
		"trailing data":     `{"resolvedForDays":14} {}`,
	} {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			h.Autoclose(w, httptest.NewRequest(http.MethodPost, "/incidents/autoclose", strings.NewReader(body)))
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (%s)", w.Code, w.Body.String())
			}
		})
	}
}

// TestIncidentAutocloseHandler_Unavailable: the ServiceNow stand-in answers 503
// (writeServiceError logs the reason and returns a generic message).
func TestIncidentAutocloseHandler_Unavailable(t *testing.T) {
	h := NewIncidentAutocloseHandler(service.NewUnavailableIncidentAutocloseService("needs postgres"))
	w := httptest.NewRecorder()
	h.Autoclose(w, httptest.NewRequest(http.MethodPost, "/incidents/autoclose", strings.NewReader(`{"resolvedForDays":14}`)))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d %s, want 503", w.Code, w.Body.String())
	}
}

type stubAutocloseRepo struct {
	due []repository.IncidentAutocloseCandidate
}

func (r stubAutocloseRepo) ListIncidentsDueForAutoclose(context.Context, int, int) ([]repository.IncidentAutocloseCandidate, error) {
	return r.due, nil
}
func (stubAutocloseRepo) AutocloseIncident(context.Context, string, int, string) (bool, error) {
	return true, nil
}
func (stubAutocloseRepo) RecordIncidentFieldChanges(context.Context, string, []repository.IncidentFieldChange, string) error {
	return nil
}

// TestIncidentAutocloseHandler_ResponseShape: 200 with dryRun, closed, failed
// (an empty array, not null) and more.
func TestIncidentAutocloseHandler_ResponseShape(t *testing.T) {
	h := NewIncidentAutocloseHandler(service.NewIncidentAutocloseService(stubAutocloseRepo{
		due: []repository.IncidentAutocloseCandidate{{ID: "a0000000-0000-0000-0000-000000000001", Number: "INC1"}},
	}))
	w := httptest.NewRecorder()
	h.Autoclose(w, httptest.NewRequest(http.MethodPost, "/incidents/autoclose", strings.NewReader(`{"resolvedForDays":14,"limit":1}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", w.Code, w.Body.String())
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"dryRun", "closed", "failed", "more"} {
		if _, ok := raw[k]; !ok {
			t.Errorf("response has no %q: %s", k, w.Body.String())
		}
	}
	if string(raw["failed"]) != "[]" || string(raw["more"]) != "true" {
		t.Errorf("failed = %s, more = %s; want [] and true", raw["failed"], raw["more"])
	}
	var resp domain.IncidentAutocloseResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Closed) != 1 || resp.Closed[0].Number != "INC1" || resp.Closed[0].ResolvedOn != nil {
		t.Errorf("closed = %+v", resp.Closed)
	}
}
