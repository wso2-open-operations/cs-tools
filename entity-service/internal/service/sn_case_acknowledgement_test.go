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
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
)

// Case acknowledgement: a first-write-wins claim that an engineer has picked the
// case up, distinct from assignment. These cover the two directions the adapter is
// responsible for -- reading acknowledgedBy off the case detail, and the
// acknowledge write plus its echo -- and the guard rails around the write.

func TestSNCaseService_GetCaseByID_MapsAcknowledgedBy(t *testing.T) {
	ackSysid := sysid32('9')
	newBody := func(acknowledgedBy string) string {
		return `{
			"id": "` + testWLCaseSysid + `",
			"internalId": "WSO2-001",
			"number": "CS0001001",
			"title": "Case subject",
			"description": "Case description",
			"createdOn": "2026-01-01 10:00:00",
			"updatedOn": "2026-01-02 10:00:00",
			"createdBy": "reporter@example.com",
			"project": {"id": "` + testProjectSysid + `", "name": "Project A"},
			"deployment": {"id": "", "name": ""},
			"deployedProduct": {"id": "", "name": "", "version": ""},
			"state": {"id": 1, "label": "Open"},
			"acknowledgedBy": ` + acknowledgedBy + `
		}`
	}

	t.Run("populated acknowledgement maps through with the id converted to a platform UUID", func(t *testing.T) {
		client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(newBody(`{"id": "` + ackSysid + `", "name": "Jane Doe", "email": "jane.doe@example.com"}`)))
		})
		svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

		cv, err := svc.GetCaseByID(contextWithUserIDToken("token"), sysidToUUID(testWLCaseSysid))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cv.AcknowledgedBy == nil {
			t.Fatal("AcknowledgedBy = nil, want a populated reference")
		}
		if got, want := cv.AcknowledgedBy.ID, sysidToUUID(ackSysid); got != want {
			t.Fatalf("AcknowledgedBy.ID = %q, want the platform UUID %q", got, want)
		}
		if cv.AcknowledgedBy.Name != "Jane Doe" {
			t.Fatalf("AcknowledgedBy.Name = %q, want %q", cv.AcknowledgedBy.Name, "Jane Doe")
		}
		if cv.AcknowledgedBy.Email == nil || *cv.AcknowledgedBy.Email != "jane.doe@example.com" {
			t.Fatalf("AcknowledgedBy.Email = %v, want jane.doe@example.com", cv.AcknowledgedBy.Email)
		}
	})

	t.Run("null acknowledgement stays nil rather than an empty reference", func(t *testing.T) {
		// A zero-valued AssignedEngineerRef would read to a consumer as "acknowledged
		// by someone with no name", which is the opposite of the truth. It must be nil.
		client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(newBody(`null`)))
		})
		svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

		cv, err := svc.GetCaseByID(contextWithUserIDToken("token"), sysidToUUID(testWLCaseSysid))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cv.AcknowledgedBy != nil {
			t.Fatalf("AcknowledgedBy = %+v, want nil", cv.AcknowledgedBy)
		}
	})
}

func TestSNCaseService_UpdateCase_AcknowledgeSendsTrueAndEchoesBack(t *testing.T) {
	ackSysid := sysid32('9')
	var gotBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/cases/"+testCaseSysid, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": "Case acknowledged successfully",
			"case": map[string]any{
				"id": testCaseSysid, "updatedOn": "2026-01-01 00:00:00",
				"number":              "CS0001001",
				"alreadyAcknowledged": false,
				"acknowledgedBy": map[string]any{
					"id": ackSysid, "name": "Jane Doe", "email": "jane.doe@example.com",
				},
			},
		})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	acknowledge := true
	resp, err := svc.UpdateCase(contextWithUserIDToken("token"), domain.UpdateCaseRequest{ID: testCaseUUID, Acknowledge: &acknowledge})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The acknowledge flag must reach the upstream payload -- an acknowledge request
	// that arrives as an empty body would silently do nothing.
	if gotBody["acknowledge"] != true {
		t.Fatalf("upstream payload acknowledge = %v, want true", gotBody["acknowledge"])
	}
	if len(gotBody) != 1 {
		t.Fatalf("upstream payload = %v, want acknowledge alone (it is mutually exclusive with every other field)", gotBody)
	}
	if resp.Case.Number != "CS0001001" {
		t.Fatalf("Number = %q, want CS0001001", resp.Case.Number)
	}
	if resp.Case.AlreadyAcknowledged == nil || *resp.Case.AlreadyAcknowledged {
		t.Fatalf("AlreadyAcknowledged = %v, want a pointer to false", resp.Case.AlreadyAcknowledged)
	}
	if resp.Case.AcknowledgedBy == nil {
		t.Fatal("AcknowledgedBy = nil, want the new acknowledger")
	}
	if got, want := resp.Case.AcknowledgedBy.ID, sysidToUUID(ackSysid); got != want {
		t.Fatalf("AcknowledgedBy.ID = %q, want the platform UUID %q", got, want)
	}
}

func TestSNCaseService_UpdateCase_AcknowledgeAlreadyAcknowledgedIsNotAnError(t *testing.T) {
	// Re-acknowledging is a no-op that succeeds: the caller needs to render "already
	// acknowledged by X", which it cannot do if this surfaces as an error.
	ackSysid := sysid32('9')
	mux := http.NewServeMux()
	mux.HandleFunc("/cases/"+testCaseSysid, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": "Case is already acknowledged",
			"case": map[string]any{
				"id": testCaseSysid, "updatedOn": "2026-01-01 00:00:00",
				"number":              "CS0001001",
				"alreadyAcknowledged": true,
				"acknowledgedBy": map[string]any{
					"id": ackSysid, "name": "Jane Doe", "email": "jane.doe@example.com",
				},
			},
		})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	acknowledge := true
	resp, err := svc.UpdateCase(contextWithUserIDToken("token"), domain.UpdateCaseRequest{ID: testCaseUUID, Acknowledge: &acknowledge})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Case.AlreadyAcknowledged == nil || !*resp.Case.AlreadyAcknowledged {
		t.Fatalf("AlreadyAcknowledged = %v, want a pointer to true", resp.Case.AlreadyAcknowledged)
	}
	if resp.Case.AcknowledgedBy == nil || resp.Case.AcknowledgedBy.Name != "Jane Doe" {
		t.Fatalf("AcknowledgedBy = %+v, want the existing acknowledger", resp.Case.AcknowledgedBy)
	}
}

// TestSNCaseService_UpdateCase_AcknowledgePublishesCaseAcknowledged verifies
// the happy path: a first-time acknowledge publishes case.acknowledged with
// the acknowledger's name, enriched via a follow-up GetCaseByID for
// CaseNumber/WSO2CaseID/Severity — none of which the acknowledge PATCH
// response itself carries.
func TestSNCaseService_UpdateCase_AcknowledgePublishesCaseAcknowledged(t *testing.T) {
	ackSysid := sysid32('9')
	mux := http.NewServeMux()
	mux.HandleFunc("/cases/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPatch {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"message": "Case acknowledged successfully",
				"case": map[string]any{
					"id": testCaseSysid, "updatedOn": "2026-01-01 00:00:00",
					"number":              "CS0001001",
					"alreadyAcknowledged": false,
					"acknowledgedBy": map[string]any{
						"id": ackSysid, "name": "Jane Doe", "email": "jane.doe@example.com",
					},
				},
			})
			return
		}
		if strings.HasSuffix(r.URL.Path, "/tags") {
			_, _ = w.Write([]byte(`{"tags":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{
			"id": "` + testCaseSysid + `",
			"internalId": "WSO2-050",
			"number": "CS0001001",
			"title": "Case subject",
			"description": "d",
			"createdOn": "2026-01-01 10:00:00",
			"createdBy": "reporter@example.com",
			"project": {"id": "` + testProjectSysid + `", "name": "Project A"},
			"deployment": {"id": "", "name": ""},
			"deployedProduct": {"id": "", "name": "", "version": ""},
			"product": {"id": "` + sysid32('7') + `", "name": "WSO2 API Manager"},
			"severity": {"id": 1, "label": "1 - Critical"},
			"state": {"id": 1, "label": "Open"},
			"account": {"id": "` + testProjectSysid + `", "name": "Account A", "type": "customer", "creTeam": {"id": "` + sysid32('8') + `", "name": "Team Nova"}}
		}`))
	})

	client := newTestSNClient(t, mux)
	publisher := &mockEventPublisher{}
	svc := NewServiceNowCaseService(client, nil, publisher, noopSLAClockService{}, nil, "", nil)

	acknowledge := true
	if _, err := svc.UpdateCase(contextWithUserIDToken("token"), domain.UpdateCaseRequest{ID: testCaseUUID, Acknowledge: &acknowledge}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(publisher.calls) != 1 {
		t.Fatalf("expected 1 publish call, got %d", len(publisher.calls))
	}
	call := publisher.calls[0]
	if call.eventType != events.TypeCaseAcknowledged {
		t.Errorf("eventType = %q, want %q", call.eventType, events.TypeCaseAcknowledged)
	}
	if call.entityID != testCaseUUID {
		t.Errorf("entityID = %q, want %q", call.entityID, testCaseUUID)
	}

	var payload events.CaseAcknowledgedPayload
	if err := json.Unmarshal(call.payload, &payload); err != nil {
		t.Fatalf("decode published payload: %v", err)
	}
	if payload.AcknowledgerName != "Jane Doe" {
		t.Errorf("acknowledgerName = %q, want %q", payload.AcknowledgerName, "Jane Doe")
	}
	if payload.CaseNumber != "CS0001001" {
		t.Errorf("caseNumber = %q, want %q", payload.CaseNumber, "CS0001001")
	}
	if payload.WSO2CaseID != "WSO2-050" {
		t.Errorf("wso2CaseId = %q, want %q", payload.WSO2CaseID, "WSO2-050")
	}
	if payload.Severity != "CRITICAL" {
		t.Errorf("severity = %q, want %q", payload.Severity, "CRITICAL")
	}
	if payload.Team != "Team Nova" {
		t.Errorf("team = %q, want %q", payload.Team, "Team Nova")
	}
	if payload.Product != "WSO2 API Manager" {
		t.Errorf("product = %q, want %q", payload.Product, "WSO2 API Manager")
	}
}

// TestSNCaseService_UpdateCase_AcknowledgeAlreadyAcknowledgedSkipsPublish
// verifies a repeat acknowledge (AlreadyAcknowledged: true) never publishes
// case.acknowledged — it's a no-op as far as the case is concerned, and
// publishing would send a false "just acknowledged" Chat alert.
func TestSNCaseService_UpdateCase_AcknowledgeAlreadyAcknowledgedSkipsPublish(t *testing.T) {
	ackSysid := sysid32('9')
	mux := http.NewServeMux()
	mux.HandleFunc("/cases/"+testCaseSysid, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": "Case is already acknowledged",
			"case": map[string]any{
				"id": testCaseSysid, "updatedOn": "2026-01-01 00:00:00",
				"number":              "CS0001001",
				"alreadyAcknowledged": true,
				"acknowledgedBy": map[string]any{
					"id": ackSysid, "name": "Jane Doe", "email": "jane.doe@example.com",
				},
			},
		})
	})

	client := newTestSNClient(t, mux)
	publisher := &mockEventPublisher{}
	svc := NewServiceNowCaseService(client, nil, publisher, noopSLAClockService{}, nil, "", nil)

	acknowledge := true
	if _, err := svc.UpdateCase(contextWithUserIDToken("token"), domain.UpdateCaseRequest{ID: testCaseUUID, Acknowledge: &acknowledge}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(publisher.calls) != 0 {
		t.Fatalf("expected no publish call for an already-acknowledged case, got %d", len(publisher.calls))
	}
}

func TestSNCaseService_UpdateCase_AcknowledgeRejectsFalseAndCombinations(t *testing.T) {
	client := newTestSNClient(t, http.NewServeMux())
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	acknowledgeTrue, acknowledgeFalse := true, false
	subject := "new subject"
	state := domain.CaseStateWorkInProgress

	tests := []struct {
		name string
		req  domain.UpdateCaseRequest
	}{
		{
			// There is no unacknowledge. Accepting false and forwarding it would let a
			// caller believe it cleared the field.
			name: "acknowledge=false",
			req:  domain.UpdateCaseRequest{ID: testCaseUUID, Acknowledge: &acknowledgeFalse},
		},
		{
			name: "acknowledge combined with a combinable field",
			req:  domain.UpdateCaseRequest{ID: testCaseUUID, Acknowledge: &acknowledgeTrue, Subject: &subject},
		},
		{
			name: "acknowledge combined with another exclusive field",
			req:  domain.UpdateCaseRequest{ID: testCaseUUID, Acknowledge: &acknowledgeTrue, State: &state},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.UpdateCase(contextWithUserIDToken("token"), tc.req)
			if err == nil {
				t.Fatal("expected a validation error, got nil")
			}
			var verr *apierror.ValidationError
			if !errors.As(err, &verr) {
				t.Fatalf("error = %T (%v), want *apierror.ValidationError", err, err)
			}
		})
	}
}
