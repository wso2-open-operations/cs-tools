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

const (
	testCaseUUID  = "11111111-1111-1111-1111-111111111111"
	testCaseSysid = "11111111111111111111111111111111"
	testTagUUID   = "22222222-2222-2222-2222-222222222222"
	testTagSysid  = "22222222222222222222222222222222"
	testTaskSysid = "33333333333333333333333333333333"
)

// --- UpdateCase: field-count union (including the new fixEta variant) ---

func TestSNCaseService_UpdateCase_FieldCountValidation(t *testing.T) {
	svc := NewServiceNowCaseService(nil, nil)
	closed := domain.CaseStateClosed
	fixEta := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		req  domain.UpdateCaseRequest
	}{
		{
			name: "no fields provided",
			req:  domain.UpdateCaseRequest{ID: testCaseUUID},
		},
		{
			name: "state and fixEta both provided",
			req:  domain.UpdateCaseRequest{ID: testCaseUUID, State: &closed, FixEta: &fixEta},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.UpdateCase(contextWithUserIDToken("token"), tt.req)
			if _, ok := err.(*apierror.ValidationError); !ok {
				t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
			}
		})
	}
}

// --- UpdateCase: close-gate ---

func TestSNCaseService_UpdateCase_CloseGate_BlocksOnOpenVisibleTask(t *testing.T) {
	patchCalled := false
	mux := http.NewServeMux()
	mux.HandleFunc("/cases/"+testCaseSysid+"/tasks/search", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tasks": []map[string]any{
				{"id": testTaskSysid, "subject": "follow up", "state": "OPEN"},
			},
			"total": 1, "offset": 0, "limit": 100,
		})
	})
	mux.HandleFunc("/tasks/"+testTaskSysid, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": testTaskSysid, "subject": "follow up", "visibleToCustomer": true,
		})
	})
	mux.HandleFunc("/cases/"+testCaseSysid, func(w http.ResponseWriter, r *http.Request) {
		patchCalled = true
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"message": "ok", "case": map[string]any{"id": testCaseSysid, "updatedOn": "2026-01-01 00:00:00"}})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil)

	closed := domain.CaseStateClosed
	_, err := svc.UpdateCase(contextWithUserIDToken("token"), domain.UpdateCaseRequest{ID: testCaseUUID, State: &closed})
	if _, ok := err.(*apierror.ValidationError); !ok {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
	if patchCalled {
		t.Fatalf("expected PATCH /cases/{id} not to be called when an open visible task blocks the close")
	}
}

func TestSNCaseService_UpdateCase_CloseGate_AllowsWhenNoVisibleOpenTask(t *testing.T) {
	patchCalled := false
	mux := http.NewServeMux()
	mux.HandleFunc("/cases/"+testCaseSysid+"/tasks/search", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tasks": []map[string]any{
				{"id": testTaskSysid, "subject": "internal note", "state": "OPEN"},
			},
			"total": 1, "offset": 0, "limit": 100,
		})
	})
	mux.HandleFunc("/tasks/"+testTaskSysid, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": testTaskSysid, "subject": "internal note", "visibleToCustomer": false,
		})
	})
	mux.HandleFunc("/cases/"+testCaseSysid, func(w http.ResponseWriter, r *http.Request) {
		patchCalled = true
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"message": "ok", "case": map[string]any{"id": testCaseSysid, "updatedOn": "2026-01-01 00:00:00"}})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil)

	closed := domain.CaseStateClosed
	_, err := svc.UpdateCase(contextWithUserIDToken("token"), domain.UpdateCaseRequest{ID: testCaseUUID, State: &closed})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !patchCalled {
		t.Fatalf("expected PATCH /cases/{id} to be called when no visible open task blocks the close")
	}
}

// --- FixEta read/write mapping ---

func TestSNCaseService_GetCaseByID_MapsFixEta(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/cases/"+testCaseSysid, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": testCaseSysid, "internalId": "INT-1", "number": "CS0001",
			"title": "t", "description": "d",
			"createdOn": "2026-01-01 00:00:00", "createdBy": "a@example.com",
			"project":         map[string]any{"id": "", "name": ""},
			"deployment":      map[string]any{"id": "", "name": ""},
			"deployedProduct": map[string]any{"id": "", "name": "", "version": ""},
			"fixEta":          "2026-02-15 10:30:00",
		})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil)

	cv, err := svc.GetCaseByID(contextWithUserIDToken("token"), testCaseUUID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cv.FixEta == nil {
		t.Fatalf("expected FixEta to be populated, got nil")
	}
	want := time.Date(2026, 2, 15, 10, 30, 0, 0, time.UTC)
	if !cv.FixEta.Equal(want) {
		t.Fatalf("FixEta = %v, want %v", cv.FixEta, want)
	}
}

func TestSNCaseService_UpdateCase_FixEta_SendsFormattedDate(t *testing.T) {
	var gotBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/cases/"+testCaseSysid, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{"message": "ok", "case": map[string]any{"id": testCaseSysid, "updatedOn": "2026-01-01 00:00:00"}})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil)

	fixEta := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	_, err := svc.UpdateCase(contextWithUserIDToken("token"), domain.UpdateCaseRequest{ID: testCaseUUID, FixEta: &fixEta})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ := gotBody["fixEta"].(string)
	want := "2026-03-01 12:00:00"
	if got != want {
		t.Fatalf("fixEta sent = %q, want %q", got, want)
	}
}

// --- Case tags ---

func TestSNCaseService_AddCaseTag_Validation(t *testing.T) {
	svc := NewServiceNowCaseService(nil, nil)

	if _, err := svc.AddCaseTag(contextWithUserIDToken("token"), "not-a-uuid", "micro-gw"); err == nil {
		t.Fatalf("expected error for invalid case id")
	} else if _, ok := err.(*apierror.ValidationError); !ok {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}

	if _, err := svc.AddCaseTag(contextWithUserIDToken("token"), testCaseUUID, "   "); err == nil {
		t.Fatalf("expected error for empty label")
	} else if _, ok := err.(*apierror.ValidationError); !ok {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

func TestSNCaseService_AddCaseTag_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/cases/"+testCaseSysid+"/tags", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["label"] != "micro-gw" {
			t.Fatalf("label sent = %v, want micro-gw", body["label"])
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": "ok",
			"tag":     map[string]any{"id": testTagSysid, "label": "micro-gw", "color": "#f97316"},
		})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil)

	tag, err := svc.AddCaseTag(contextWithUserIDToken("token"), testCaseUUID, "micro-gw")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tag.ID != testTagUUID {
		t.Fatalf("tag.ID = %q, want %q", tag.ID, testTagUUID)
	}
	if tag.Label != "micro-gw" {
		t.Fatalf("tag.Label = %q, want micro-gw", tag.Label)
	}
	if tag.Color == nil || *tag.Color != "#f97316" {
		t.Fatalf("tag.Color = %v, want #f97316", tag.Color)
	}
}

func TestSNCaseService_RemoveCaseTag_Validation(t *testing.T) {
	svc := NewServiceNowCaseService(nil, nil)

	if err := svc.RemoveCaseTag(contextWithUserIDToken("token"), "not-a-uuid", testTagUUID); err == nil {
		t.Fatalf("expected error for invalid case id")
	} else if _, ok := err.(*apierror.ValidationError); !ok {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}

	if err := svc.RemoveCaseTag(contextWithUserIDToken("token"), testCaseUUID, "not-a-uuid"); err == nil {
		t.Fatalf("expected error for invalid tag id")
	} else if _, ok := err.(*apierror.ValidationError); !ok {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

func TestSNCaseService_RemoveCaseTag_Success(t *testing.T) {
	deleteCalled := false
	mux := http.NewServeMux()
	mux.HandleFunc("/cases/"+testCaseSysid+"/tags/"+testTagSysid, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s, want DELETE", r.Method)
		}
		deleteCalled = true
		_ = json.NewEncoder(w).Encode(map[string]any{"message": "ok"})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil)

	if err := svc.RemoveCaseTag(contextWithUserIDToken("token"), testCaseUUID, testTagUUID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deleteCalled {
		t.Fatalf("expected DELETE /cases/{id}/tags/{tagId} to be called")
	}
}
