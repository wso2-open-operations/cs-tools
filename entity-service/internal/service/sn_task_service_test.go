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

package service

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// --- CreateCaseTask ---

func TestSNTaskService_CreateCaseTask_Validation(t *testing.T) {
	svc := NewServiceNowTaskService(nil)

	tests := []struct {
		name   string
		caseID string
		req    domain.CreateCaseTaskRequest
	}{
		{
			name:   "invalid case id",
			caseID: "not-a-uuid",
			req:    domain.CreateCaseTaskRequest{Subject: "follow up"},
		},
		{
			name:   "empty subject",
			caseID: testCaseUUID,
			req:    domain.CreateCaseTaskRequest{Subject: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateCaseTask(contextWithUserIDToken("token"), tt.caseID, tt.req)
			if _, ok := err.(*apierror.ValidationError); !ok {
				t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
			}
		})
	}
}

func TestSNTaskService_CreateCaseTask_GatedUnavailable(t *testing.T) {
	svc := NewServiceNowTaskService(nil)
	_, err := svc.CreateCaseTask(contextWithUserIDToken("token"), testCaseUUID, domain.CreateCaseTaskRequest{Subject: "follow up"})
	if _, ok := err.(*apierror.ServiceUnavailableError); !ok {
		t.Fatalf("expected *apierror.ServiceUnavailableError while taskWritesUnavailable is true, got %T: %v", err, err)
	}
}

// TestSNTaskService_CreateCaseTask_Success exercises the send logic that's
// ready to go the moment the downstream endpoint ships -- it temporarily
// disables the taskWritesUnavailable gate to prove that logic still works.
func TestSNTaskService_CreateCaseTask_Success(t *testing.T) {
	taskWritesUnavailable = false
	defer func() { taskWritesUnavailable = true }()

	var gotBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/cases/"+testCaseSysid+"/tasks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": "ok",
			"task": map[string]any{
				"id": testTaskSysid, "subject": "follow up", "state": "OPEN",
				"visibleToCustomer": true,
			},
		})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowTaskService(client)

	due := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	email := "eng@example.com"
	visible := true
	detail, err := svc.CreateCaseTask(contextWithUserIDToken("token"), testCaseUUID, domain.CreateCaseTaskRequest{
		Subject: "follow up", DueDate: &due, AssignedToEmail: &email, VisibleToCustomer: &visible,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detail.Subject != "follow up" {
		t.Fatalf("Subject = %q, want %q", detail.Subject, "follow up")
	}
	if !detail.VisibleToCustomer {
		t.Fatalf("VisibleToCustomer = false, want true")
	}
	if gotBody["dueDate"] != "2026-08-01 00:00:00" {
		t.Fatalf("dueDate sent = %v, want 2026-08-01 00:00:00", gotBody["dueDate"])
	}
	if gotBody["assignedToEmail"] != email {
		t.Fatalf("assignedToEmail sent = %v, want %v", gotBody["assignedToEmail"], email)
	}
}

// --- UpdateTask ---

func TestSNTaskService_UpdateTask_FieldCountValidation(t *testing.T) {
	svc := NewServiceNowTaskService(nil)
	email := "eng@example.com"
	state := "CLOSED"

	tests := []struct {
		name string
		req  domain.UpdateTaskRequest
	}{
		{name: "no fields", req: domain.UpdateTaskRequest{ID: testCaseUUID}},
		{name: "two fields", req: domain.UpdateTaskRequest{ID: testCaseUUID, State: &state, AssignedToEmail: &email}},
		{name: "empty state", req: domain.UpdateTaskRequest{ID: testCaseUUID, State: strPtr("")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.UpdateTask(contextWithUserIDToken("token"), tt.req.ID, tt.req)
			if _, ok := err.(*apierror.ValidationError); !ok {
				t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
			}
		})
	}
}

func TestSNTaskService_UpdateTask_GatedUnavailable(t *testing.T) {
	svc := NewServiceNowTaskService(nil)
	state := "CLOSED"
	_, err := svc.UpdateTask(contextWithUserIDToken("token"), "33333333-3333-3333-3333-333333333333", domain.UpdateTaskRequest{State: &state})
	if _, ok := err.(*apierror.ServiceUnavailableError); !ok {
		t.Fatalf("expected *apierror.ServiceUnavailableError while taskWritesUnavailable is true, got %T: %v", err, err)
	}
}

// TestSNTaskService_UpdateTask_Success exercises the send logic that's ready
// to go the moment the downstream endpoint ships -- it temporarily disables
// the taskWritesUnavailable gate to prove that logic still works.
func TestSNTaskService_UpdateTask_Success(t *testing.T) {
	taskWritesUnavailable = false
	defer func() { taskWritesUnavailable = true }()

	var gotBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/"+testTaskSysid, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("method = %s, want PATCH", r.Method)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": "ok",
			"task": map[string]any{
				"id": testTaskSysid, "subject": "follow up", "state": "CLOSED",
				"visibleToCustomer": true,
			},
		})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowTaskService(client)

	taskUUID := "33333333-3333-3333-3333-333333333333"
	state := "CLOSED"
	detail, err := svc.UpdateTask(contextWithUserIDToken("token"), taskUUID, domain.UpdateTaskRequest{State: &state})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detail.State == nil || *detail.State != "CLOSED" {
		t.Fatalf("State = %v, want CLOSED", detail.State)
	}
	if gotBody["state"] != "CLOSED" {
		t.Fatalf("state sent = %v, want CLOSED", gotBody["state"])
	}
}
