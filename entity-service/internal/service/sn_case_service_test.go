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
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// newTestCaseClient spins up an httptest server that answers both the OAuth2
// token endpoint and the Choreo API path with apiHandler, and returns a
// Client wired to it. The server is closed automatically via t.Cleanup.
func newTestCaseClient(t *testing.T, apiHandler http.HandlerFunc) *integrationservice.Client {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "test-token", "expires_in": 3600})
	})
	mux.HandleFunc("/", apiHandler)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return integrationservice.New(srv.URL, integrationservice.ClientCredentialsConfig{
		TokenURL:     srv.URL + "/oauth2/token",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	})
}

// sysid32 pads/truncates a repeated hex rune to exactly 32 characters, the
// length ServiceNow sysids always have.
func sysid32(r byte) string {
	b := make([]byte, 32)
	for i := range b {
		b[i] = r
	}
	return string(b)
}

var (
	testWLCaseSysid  = sysid32('a')
	testProjectSysid = sysid32('b')
	testWatcherSysid = sysid32('c')
	testAccountSysid = sysid32('d')
	testCreTeamSysid = sysid32('e')
	testSreTeamSysid = sysid32('f')
)

const (
	testDeploymentUUID  = "22222222-2222-2222-2222-222222222222"
	testDeployedProdID  = "33333333-3333-3333-3333-333333333333"
	testRelatedCaseUUID = "44444444-4444-4444-4444-444444444444"
	testParentCaseUUID  = "55555555-5555-5555-5555-555555555555"
)

// testAutocloseHoldUntil is the hold-until date used by AutocloseHoldUntil test cases.
var testAutocloseHoldUntil = time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC)

// timePtr returns a pointer to the given time.Time.
func timePtr(t time.Time) *time.Time { return &t }

// TestSNCaseService_GetCaseByID_MapsWatchListAutoclosureAndTeams verifies the
// additive read-side wire-up for items 2 (watchers), 6 (autoclosureStep/autoclosureStateTime),
// and 10 (CRE/SRE team on account). AutoclosureStep/AutoclosureStateTime/CreTeam/SreTeam are
// Ballerina-blocked today (digiops-cs does not send them), but this test simulates a future
// response carrying them to prove the entity-service mapping code is ready once Ballerina adds
// the fields.
func TestSNCaseService_GetCaseByID_MapsWatchListAutoclosureAndTeams(t *testing.T) {
	body := `{
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
		"watchList": [
			{"id": "` + testWatcherSysid + `", "userName": "jdoe", "name": "Jane Doe", "email": "jane.doe@example.com"}
		],
		"account": {
			"id": "` + testAccountSysid + `",
			"name": "Account A",
			"type": "enterprise",
			"creTeam": {"id": "` + testCreTeamSysid + `", "name": "CRE Team A"},
			"sreTeam": {"id": "` + testSreTeamSysid + `", "name": "SRE Team A"}
		},
		"autoclosureStep": "ON_HOLD",
		"autoclosureStateTime": "2026-08-06 00:00:00"
	}`

	client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})

	svc := NewServiceNowCaseService(client, nil)

	cv, err := svc.GetCaseByID(contextWithUserIDToken("token"), sysidToUUID(testWLCaseSysid))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Item 2: watchers.
	if len(cv.WatchList) != 1 {
		t.Fatalf("expected 1 watcher, got %d", len(cv.WatchList))
	}
	gotWatcher := cv.WatchList[0]
	if gotWatcher.ID != sysidToUUID(testWatcherSysid) || gotWatcher.UserName != "jdoe" ||
		gotWatcher.Name != "Jane Doe" || gotWatcher.Email != "jane.doe@example.com" {
		t.Fatalf("unexpected watcher mapping: %+v", gotWatcher)
	}

	// Item 6: autoclosureStep/autoclosureStateTime (both read-only; the only write is the
	// derived autocloseHoldUntil variant).
	if cv.AutoclosureStep == nil || *cv.AutoclosureStep != "ON_HOLD" {
		t.Fatalf("expected autoclosureStep=ON_HOLD, got %+v", cv.AutoclosureStep)
	}
	wantStateTime, err := time.Parse(snCreatedOnLayout, "2026-08-06 00:00:00")
	if err != nil {
		t.Fatalf("parse want autoclosureStateTime: %v", err)
	}
	if cv.AutoclosureStateTime == nil || !cv.AutoclosureStateTime.Equal(wantStateTime) {
		t.Fatalf("expected autoclosureStateTime=%v, got %+v", wantStateTime, cv.AutoclosureStateTime)
	}

	// Item 10: CRE/SRE team on account.
	if cv.AccountDetails == nil {
		t.Fatalf("expected account details to be populated")
	}
	if cv.AccountDetails.CreTeam == nil || cv.AccountDetails.CreTeam.ID != sysidToUUID(testCreTeamSysid) ||
		cv.AccountDetails.CreTeam.Name != "CRE Team A" {
		t.Fatalf("unexpected creTeam mapping: %+v", cv.AccountDetails.CreTeam)
	}
	if cv.AccountDetails.SreTeam == nil || cv.AccountDetails.SreTeam.ID != sysidToUUID(testSreTeamSysid) ||
		cv.AccountDetails.SreTeam.Name != "SRE Team A" {
		t.Fatalf("unexpected sreTeam mapping: %+v", cv.AccountDetails.SreTeam)
	}
}

// TestSNCaseService_GetCaseByID_BallerinaBlockedFieldsAbsent documents current reality:
// against a real (unmodified) digiops-cs response with none of the blocked fields present,
// AutoclosureStep/AutoclosureStateTime/CreTeam/SreTeam all stay nil rather than zero-valuing.
func TestSNCaseService_GetCaseByID_BallerinaBlockedFieldsAbsent(t *testing.T) {
	body := `{
		"id": "` + testWLCaseSysid + `",
		"internalId": "WSO2-001",
		"number": "CS0001001",
		"title": "Case subject",
		"description": "Case description",
		"createdOn": "2026-01-01 10:00:00",
		"updatedOn": null,
		"createdBy": "reporter@example.com",
		"project": {"id": "` + testProjectSysid + `", "name": "Project A"},
		"deployment": {"id": "", "name": ""},
		"deployedProduct": {"id": "", "name": "", "version": ""},
		"state": {"id": 1, "label": "Open"},
		"account": {"id": "` + testAccountSysid + `", "name": "Account A", "type": "enterprise"}
	}`

	client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})

	svc := NewServiceNowCaseService(client, nil)

	cv, err := svc.GetCaseByID(contextWithUserIDToken("token"), sysidToUUID(testWLCaseSysid))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cv.AutoclosureStep != nil {
		t.Fatalf("expected autoclosureStep nil (Ballerina-blocked), got %+v", cv.AutoclosureStep)
	}
	if cv.AutoclosureStateTime != nil {
		t.Fatalf("expected autoclosureStateTime nil (Ballerina-blocked), got %+v", cv.AutoclosureStateTime)
	}
	if cv.AccountDetails == nil {
		t.Fatalf("expected account details to be populated")
	}
	if cv.AccountDetails.CreTeam != nil || cv.AccountDetails.SreTeam != nil {
		t.Fatalf("expected creTeam/sreTeam nil (Ballerina-blocked), got %+v / %+v",
			cv.AccountDetails.CreTeam, cv.AccountDetails.SreTeam)
	}
	if len(cv.WatchList) != 0 {
		t.Fatalf("expected no watchers, got %+v", cv.WatchList)
	}
}

// TestSNCaseService_UpdateCase_ExactlyOneFieldValidation exercises the exactly-one-field
// union for every new PATCH variant (items 6, 7, 9) alongside the pre-existing ones, using
// a nil client so every case must fail validation before any network call is attempted.
func TestSNCaseService_UpdateCase_ExactlyOneFieldValidation(t *testing.T) {
	strPtr := func(s string) *string { return &s }
	emptyStr := ""

	tests := []struct {
		name string
		req  domain.UpdateCaseRequest
	}{
		{
			name: "no fields provided",
			req:  domain.UpdateCaseRequest{ID: testDeploymentUUID},
		},
		{
			name: "two new fields provided at once",
			req: domain.UpdateCaseRequest{
				ID:                 testDeploymentUUID,
				Subject:            strPtr("New subject"),
				AutocloseHoldUntil: timePtr(testAutocloseHoldUntil),
			},
		},
		{
			name: "new field mixed with a pre-existing field",
			req: domain.UpdateCaseRequest{
				ID:            testDeploymentUUID,
				DeploymentID:  strPtr(testDeploymentUUID),
				AssigneeEmail: strPtr("engineer@example.com"),
			},
		},
		{
			name: "relatedCaseId invalid uuid",
			req: domain.UpdateCaseRequest{
				ID:            testDeploymentUUID,
				RelatedCaseID: strPtr("not-a-uuid"),
			},
		},
		{
			name: "deploymentId invalid uuid",
			req: domain.UpdateCaseRequest{
				ID:           testDeploymentUUID,
				DeploymentID: strPtr("not-a-uuid"),
			},
		},
		{
			name: "deployedProductId invalid uuid",
			req: domain.UpdateCaseRequest{
				ID:                testDeploymentUUID,
				DeployedProductID: strPtr("not-a-uuid"),
			},
		},
		{
			name: "subject empty string rejected",
			req: domain.UpdateCaseRequest{
				ID:      testDeploymentUUID,
				Subject: &emptyStr,
			},
		},
		{
			name: "description empty string rejected",
			req: domain.UpdateCaseRequest{
				ID:          testDeploymentUUID,
				Description: &emptyStr,
			},
		},
	}

	// client is intentionally nil: every case must fail validation before touching it.
	svc := NewServiceNowCaseService(nil, nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.UpdateCase(contextWithUserIDToken("token"), tt.req)
			if _, ok := err.(*apierror.ValidationError); !ok {
				t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
			}
		})
	}
}

// TestSNCaseService_UpdateCase_NewSingleFieldVariants verifies each new single-field PATCH
// variant (items 6, 7, 9) builds the expected snUpdateCasePayload and round-trips a
// successful response.
func TestSNCaseService_UpdateCase_NewSingleFieldVariants(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	tests := []struct {
		name        string
		req         domain.UpdateCaseRequest
		wantPayload map[string]any
	}{
		{
			name:        "autocloseHoldUntil",
			req:         domain.UpdateCaseRequest{ID: testDeploymentUUID, AutocloseHoldUntil: timePtr(testAutocloseHoldUntil)},
			wantPayload: map[string]any{"autocloseHoldUntil": testAutocloseHoldUntil.Format(snCreatedOnLayout)},
		},
		{
			name:        "subject",
			req:         domain.UpdateCaseRequest{ID: testDeploymentUUID, Subject: strPtr("Updated subject")},
			wantPayload: map[string]any{"title": "Updated subject"},
		},
		{
			name:        "description",
			req:         domain.UpdateCaseRequest{ID: testDeploymentUUID, Description: strPtr("Updated description")},
			wantPayload: map[string]any{"description": "Updated description"},
		},
		{
			name:        "deploymentId",
			req:         domain.UpdateCaseRequest{ID: testDeploymentUUID, DeploymentID: strPtr(testDeploymentUUID)},
			wantPayload: map[string]any{"deploymentId": uuidToSysid(testDeploymentUUID)},
		},
		{
			name:        "deployedProductId",
			req:         domain.UpdateCaseRequest{ID: testDeploymentUUID, DeployedProductID: strPtr(testDeployedProdID)},
			wantPayload: map[string]any{"deployedProductId": uuidToSysid(testDeployedProdID)},
		},
		{
			name:        "relatedCaseId",
			req:         domain.UpdateCaseRequest{ID: testDeploymentUUID, RelatedCaseID: strPtr(testRelatedCaseUUID)},
			wantPayload: map[string]any{"relatedCaseId": uuidToSysid(testRelatedCaseUUID)},
		},
		{
			name:        "parentCaseId (already-wired native parent field)",
			req:         domain.UpdateCaseRequest{ID: testDeploymentUUID, ParentID: strPtr(testParentCaseUUID)},
			wantPayload: map[string]any{"parentId": uuidToSysid(testParentCaseUUID)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotBody map[string]any
			client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPatch {
					t.Fatalf("expected PATCH, got %s", r.Method)
				}
				if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
					t.Fatalf("decode request body: %v", err)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{
					"message": "Case updated successfully.",
					"case": {"id": "` + testWLCaseSysid + `", "updatedOn": "2026-01-02 10:00:00", "updatedBy": "engineer@example.com"}
				}`))
			})

			svc := NewServiceNowCaseService(client, nil)
			resp, err := svc.UpdateCase(contextWithUserIDToken("token"), tt.req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.Case.ID != sysidToUUID(testWLCaseSysid) {
				t.Fatalf("unexpected response case id: %s", resp.Case.ID)
			}

			for field, want := range tt.wantPayload {
				got, ok := gotBody[field]
				if !ok {
					t.Fatalf("expected payload field %q to be present in %+v", field, gotBody)
				}
				if fmt := jsonEqual(got, want); !fmt {
					t.Fatalf("payload field %q: got %v, want %v", field, got, want)
				}
			}
			// Every payload in this table must be a true single-field PATCH: no other
			// recognised update field should be set alongside it.
			for _, field := range []string{
				"stateKey", "severityKey", "workStateKey", "watchList", "assigneeEmail",
				"resolutionCode", "cause", "closeNotes",
			} {
				if _, ok := gotBody[field]; ok {
					t.Fatalf("unexpected extra field %q present in single-field payload: %+v", field, gotBody)
				}
			}
		})
	}
}

// jsonEqual compares two decoded-JSON values (bool/string/number) for equality.
func jsonEqual(got, want any) bool {
	switch w := want.(type) {
	case bool:
		g, ok := got.(bool)
		return ok && g == w
	case string:
		g, ok := got.(string)
		return ok && g == w
	default:
		return got == want
	}
}

// --- Below: tests from entity-tasks-fixeta-tags (items 1, 3, 8) ---

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

// --- Internal-only fix-ETA estimates: best/most-likely/worst case ---

func TestSNCaseService_UpdateCase_FieldCountValidation_InternalFixEtaVariants(t *testing.T) {
	svc := NewServiceNowCaseService(nil, nil)
	closed := domain.CaseStateClosed
	fixEta := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	bestCase := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	mostLikely := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	worstCase := time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		req  domain.UpdateCaseRequest
	}{
		{
			name: "state and bestCaseFixEta both provided",
			req:  domain.UpdateCaseRequest{ID: testCaseUUID, State: &closed, BestCaseFixEta: &bestCase},
		},
		{
			name: "fixEta and mostLikelyFixEta both provided",
			req:  domain.UpdateCaseRequest{ID: testCaseUUID, FixEta: &fixEta, MostLikelyFixEta: &mostLikely},
		},
		{
			name: "bestCaseFixEta and worstCaseFixEta both provided",
			req:  domain.UpdateCaseRequest{ID: testCaseUUID, BestCaseFixEta: &bestCase, WorstCaseFixEta: &worstCase},
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

func TestSNCaseService_UpdateCase_InternalFixEtaVariants_EachIndependentlySettable(t *testing.T) {
	tests := []struct {
		name    string
		req     func(t time.Time) domain.UpdateCaseRequest
		bodyKey string
	}{
		{
			name: "bestCaseFixEta",
			req: func(t time.Time) domain.UpdateCaseRequest {
				return domain.UpdateCaseRequest{ID: testCaseUUID, BestCaseFixEta: &t}
			},
			bodyKey: "bestCaseFixEta",
		},
		{
			name: "mostLikelyFixEta",
			req: func(t time.Time) domain.UpdateCaseRequest {
				return domain.UpdateCaseRequest{ID: testCaseUUID, MostLikelyFixEta: &t}
			},
			bodyKey: "mostLikelyFixEta",
		},
		{
			name: "worstCaseFixEta",
			req: func(t time.Time) domain.UpdateCaseRequest {
				return domain.UpdateCaseRequest{ID: testCaseUUID, WorstCaseFixEta: &t}
			},
			bodyKey: "worstCaseFixEta",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotBody map[string]any
			mux := http.NewServeMux()
			mux.HandleFunc("/cases/"+testCaseSysid, func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewDecoder(r.Body).Decode(&gotBody)
				_ = json.NewEncoder(w).Encode(map[string]any{"message": "ok", "case": map[string]any{"id": testCaseSysid, "updatedOn": "2026-01-01 00:00:00"}})
			})

			client := newTestSNClient(t, mux)
			svc := NewServiceNowCaseService(client, nil)

			value := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
			_, err := svc.UpdateCase(contextWithUserIDToken("token"), tt.req(value))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got, _ := gotBody[tt.bodyKey].(string)
			want := "2026-03-01 12:00:00"
			if got != want {
				t.Fatalf("%s sent = %q, want %q", tt.bodyKey, got, want)
			}
		})
	}
}

func TestSNCaseService_GetCaseByID_MapsInternalFixEtaFields(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/cases/"+testCaseSysid, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": testCaseSysid, "internalId": "INT-1", "number": "CS0001",
			"title": "t", "description": "d",
			"createdOn": "2026-01-01 00:00:00", "createdBy": "a@example.com",
			"project":          map[string]any{"id": "", "name": ""},
			"deployment":       map[string]any{"id": "", "name": ""},
			"deployedProduct":  map[string]any{"id": "", "name": "", "version": ""},
			"bestCaseFixEta":   "2026-02-10 00:00:00",
			"mostLikelyFixEta": "2026-02-15 00:00:00",
			"worstCaseFixEta":  "2026-02-20 00:00:00",
		})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil)

	cv, err := svc.GetCaseByID(contextWithUserIDToken("token"), testCaseUUID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantBestCase := time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC)
	if cv.BestCaseFixEta == nil || !cv.BestCaseFixEta.Equal(wantBestCase) {
		t.Fatalf("BestCaseFixEta = %v, want %v", cv.BestCaseFixEta, wantBestCase)
	}
	wantMostLikely := time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)
	if cv.MostLikelyFixEta == nil || !cv.MostLikelyFixEta.Equal(wantMostLikely) {
		t.Fatalf("MostLikelyFixEta = %v, want %v", cv.MostLikelyFixEta, wantMostLikely)
	}
	wantWorstCase := time.Date(2026, 2, 20, 0, 0, 0, 0, time.UTC)
	if cv.WorstCaseFixEta == nil || !cv.WorstCaseFixEta.Equal(wantWorstCase) {
		t.Fatalf("WorstCaseFixEta = %v, want %v", cv.WorstCaseFixEta, wantWorstCase)
	}
}

func TestSNCaseService_UpdateCase_EchoesInternalFixEtaFieldsBack(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/cases/"+testCaseSysid, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": "ok",
			"case": map[string]any{
				"id": testCaseSysid, "updatedOn": "2026-01-01 00:00:00",
				"bestCaseFixEta":   "2026-02-10 00:00:00",
				"mostLikelyFixEta": "2026-02-15 00:00:00",
				"worstCaseFixEta":  "2026-02-20 00:00:00",
			},
		})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil)

	bestCase := time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC)
	resp, err := svc.UpdateCase(contextWithUserIDToken("token"), domain.UpdateCaseRequest{ID: testCaseUUID, BestCaseFixEta: &bestCase})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantBestCase := time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC)
	if resp.Case.BestCaseFixEta == nil || !resp.Case.BestCaseFixEta.Equal(wantBestCase) {
		t.Fatalf("BestCaseFixEta = %v, want %v", resp.Case.BestCaseFixEta, wantBestCase)
	}
	wantMostLikely := time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)
	if resp.Case.MostLikelyFixEta == nil || !resp.Case.MostLikelyFixEta.Equal(wantMostLikely) {
		t.Fatalf("MostLikelyFixEta = %v, want %v", resp.Case.MostLikelyFixEta, wantMostLikely)
	}
	wantWorstCase := time.Date(2026, 2, 20, 0, 0, 0, 0, time.UTC)
	if resp.Case.WorstCaseFixEta == nil || !resp.Case.WorstCaseFixEta.Equal(wantWorstCase) {
		t.Fatalf("WorstCaseFixEta = %v, want %v", resp.Case.WorstCaseFixEta, wantWorstCase)
	}
}

// --- SearchTags ---

func TestSNCaseService_SearchTags_Success(t *testing.T) {
	var gotQuery string
	mux := http.NewServeMux()
	mux.HandleFunc("/tags/search", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("q")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tags": []map[string]any{
				{"id": testTagSysid, "label": "micro-gw", "color": "#f97316"},
			},
		})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil)

	tags, err := svc.SearchTags(contextWithUserIDToken("token"), "micro")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotQuery != "micro" {
		t.Fatalf("q sent = %q, want micro", gotQuery)
	}
	if len(tags) != 1 {
		t.Fatalf("expected 1 tag, got %d", len(tags))
	}
	if tags[0].ID != testTagUUID {
		t.Fatalf("tag.ID = %q, want %q", tags[0].ID, testTagUUID)
	}
	if tags[0].Label != "micro-gw" {
		t.Fatalf("tag.Label = %q, want micro-gw", tags[0].Label)
	}
	if tags[0].Color == nil || *tags[0].Color != "#f97316" {
		t.Fatalf("tag.Color = %v, want #f97316", tags[0].Color)
	}
}

func TestSNCaseService_SearchTags_EmptyQuery(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tags/search", func(w http.ResponseWriter, r *http.Request) {
		if q := r.URL.Query().Get("q"); q != "" {
			t.Fatalf("expected no q param, got %q", q)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"tags": []map[string]any{}})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil)

	tags, err := svc.SearchTags(contextWithUserIDToken("token"), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tags) != 0 {
		t.Fatalf("expected 0 tags, got %d", len(tags))
	}
}

func TestCaseService_SearchTags_ServiceUnavailable(t *testing.T) {
	svc := &caseService{}

	if _, err := svc.SearchTags(contextWithUserIDToken("token"), "micro"); err == nil {
		t.Fatalf("expected error")
	} else if _, ok := err.(*apierror.ServiceUnavailableError); !ok {
		t.Fatalf("expected *apierror.ServiceUnavailableError, got %T: %v", err, err)
	}
}
