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
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
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
// Ballerina-blocked today (the backing service does not send them), but this test simulates a
// future response carrying them to prove the entity-service mapping code is ready once
// Ballerina adds the fields.
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

	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

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

// TestSnParentCaseTypeToDomain covers the parent/related-case follow-up (tracked
// separately): a parent/related case reference's raw ServiceNow type maps to the
// public enum for every known sys_class_name-derived value, and an unmapped or absent
// raw value stays nil rather than leaking an unrecognised string onto the API surface.
func TestSnParentCaseTypeToDomain(t *testing.T) {
	tests := []struct {
		name string
		raw  *string
		want *string
	}{
		{name: "case", raw: strPtr("case"), want: strPtr("case")},
		{name: "incident", raw: strPtr("incident"), want: strPtr("incident")},
		{name: "change_request", raw: strPtr("change_request"), want: strPtr("change_request")},
		{name: "problem", raw: strPtr("problem"), want: strPtr("problem")},
		{name: "unrecognised value stays nil", raw: strPtr("some_future_sn_class"), want: nil},
		{name: "nil raw stays nil", raw: nil, want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := snParentCaseTypeToDomain(tt.raw)
			if (got == nil) != (tt.want == nil) {
				t.Fatalf("snParentCaseTypeToDomain(%v) = %v, want %v", tt.raw, got, tt.want)
			}
			if got != nil && *got != *tt.want {
				t.Fatalf("snParentCaseTypeToDomain(%v) = %q, want %q", tt.raw, *got, *tt.want)
			}
		})
	}
}

// TestSNCaseService_GetCaseByID_MapsParentCaseType verifies the parent/related-case
// follow-up (tracked separately) end to end: a GetCaseByID response carrying
// parentCase.type resolves to the matching domain.CaseNumberRef.Type for a known
// value, and stays nil for an unrecognised one -- never passing the raw ServiceNow
// string through unmapped.
func TestSNCaseService_GetCaseByID_MapsParentCaseType(t *testing.T) {
	newBody := func(parentType string) string {
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
			"parentCase": {"id": "` + testParentCaseUUID + `", "number": "INC0012345", "type": "` + parentType + `"}
		}`
	}

	t.Run("known type maps through", func(t *testing.T) {
		client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(newBody("incident")))
		})
		svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

		cv, err := svc.GetCaseByID(contextWithUserIDToken("token"), sysidToUUID(testWLCaseSysid))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cv.ParentCase == nil {
			t.Fatalf("expected parentCase to be populated")
		}
		if cv.ParentCase.Type == nil || *cv.ParentCase.Type != "incident" {
			t.Fatalf("expected parentCase.type=incident, got %+v", cv.ParentCase.Type)
		}
	})

	t.Run("unrecognised type stays nil", func(t *testing.T) {
		client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(newBody("some_future_sn_class")))
		})
		svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

		cv, err := svc.GetCaseByID(contextWithUserIDToken("token"), sysidToUUID(testWLCaseSysid))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cv.ParentCase == nil {
			t.Fatalf("expected parentCase to be populated (id/number still present)")
		}
		if cv.ParentCase.Type != nil {
			t.Fatalf("expected parentCase.type=nil for unrecognised SN value, got %q", *cv.ParentCase.Type)
		}
	})
}

// TestSNCaseService_GetCaseByID_MapsRelatedCaseType pins that relatedCase carries
// its record kind, exactly as parentCase does. The two references are the same
// shape and either can point at something other than a case, so a consumer must
// not be left guessing on one of them.
func TestSNCaseService_GetCaseByID_MapsRelatedCaseType(t *testing.T) {
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
		"relatedCase": {"id": "` + testParentCaseUUID + `", "number": "INC0012345", "type": "incident"}
	}`

	client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	cv, err := svc.GetCaseByID(contextWithUserIDToken("token"), sysidToUUID(testWLCaseSysid))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cv.RelatedCase == nil {
		t.Fatalf("expected relatedCase to be populated")
	}
	if cv.RelatedCase.Type == nil || *cv.RelatedCase.Type != "incident" {
		t.Fatalf("expected relatedCase.type=incident, got %+v", cv.RelatedCase.Type)
	}
	if cv.RelatedCase.Number != "INC0012345" {
		t.Fatalf("expected relatedCase.number to pass through, got %q", cv.RelatedCase.Number)
	}
}

// TestSNCaseService_GetCaseByID_NestsProductUnderDeployedProduct pins that the
// product catalogue entry hangs off the deployed product, and that a case naming
// a catalogue product with no deployed instance still returns it.
func TestSNCaseService_GetCaseByID_NestsProductUnderDeployedProduct(t *testing.T) {
	const (
		dpSysid   = "32e4c5e732e4c5e732e4c5e732e4c5e7"
		prodSysid = "4151bcd84151bcd84151bcd84151bcd8"
	)

	newBody := func(deployedProduct string) string {
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
			"deployedProduct": ` + deployedProduct + `,
			"product": {"id": "` + prodSysid + `", "name": "WSO2 API Manager 4.5.0"},
			"state": {"id": 1, "label": "Open"}
		}`
	}

	t.Run("both present", func(t *testing.T) {
		client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(newBody(`{"id": "` + dpSysid + `", "name": "WSO2 API Manager", "version": "4.5.0"}`)))
		})
		cv, err := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil).GetCaseByID(contextWithUserIDToken("token"), sysidToUUID(testWLCaseSysid))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		dp := cv.DeployedProductDetails
		if dp == nil || dp.ID == nil || *dp.ID != sysidToUUID(dpSysid) {
			t.Fatalf("deployedProduct.id not mapped: %+v", dp)
		}
		if dp.DisplayName == nil || *dp.DisplayName != "WSO2 API Manager 4.5.0" {
			t.Fatalf("deployedProduct.displayName not mapped: %+v", dp)
		}
		if dp.Product == nil || dp.Product.ID != sysidToUUID(prodSysid) || dp.Product.Name != "WSO2 API Manager 4.5.0" {
			t.Fatalf("deployedProduct.product not mapped: %+v", dp.Product)
		}
	})

	t.Run("product without a deployed product stays reachable", func(t *testing.T) {
		client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(newBody(`{"id": "", "name": "", "version": ""}`)))
		})
		cv, err := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil).GetCaseByID(contextWithUserIDToken("token"), sysidToUUID(testWLCaseSysid))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		dp := cv.DeployedProductDetails
		if dp == nil {
			t.Fatal("deployedProduct is nil: the catalogue product would be unreachable")
		}
		if dp.ID != nil || dp.DisplayName != nil {
			t.Errorf("deployedProduct id/displayName should be null with no deployed instance: %+v", dp)
		}
		if dp.Product == nil || dp.Product.ID != sysidToUUID(prodSysid) {
			t.Fatalf("deployedProduct.product not mapped: %+v", dp.Product)
		}
	})
}

// TestSNCaseService_GetCaseByID_BallerinaBlockedFieldsAbsent documents current reality:
// against a real, unmodified backing-service response with none of the blocked fields present,
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

	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

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
	svc := NewServiceNowCaseService(nil, nil, nil, noopSLAClockService{}, nil, "", nil)

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
			name: "autocloseHoldUntil",
			req:  domain.UpdateCaseRequest{ID: testDeploymentUUID, AutocloseHoldUntil: timePtr(testAutocloseHoldUntil)},
			// Date only: the integration service constrains autocloseHoldUntil to
			// YYYY-MM-DD, so a datetime fails payload binding upstream.
			wantPayload: map[string]any{"autocloseHoldUntil": testAutocloseHoldUntil.UTC().Format(snDateOnlyLayout)},
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

			svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)
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

// --- UpdateCase: type transfer ---

func TestSNCaseService_UpdateCase_TypeTransfer_ValidationErrors(t *testing.T) {
	strPtr := func(s string) *string { return &s }
	engagement := domain.EngagementTypeMigration
	paymentType := domain.EngagementPaymentTypePaid
	severity := domain.CaseSeverityHigh

	tests := []struct {
		name string
		req  domain.UpdateCaseRequest
	}{
		{
			name: "type contains an invalid value",
			req:  domain.UpdateCaseRequest{ID: testDeploymentUUID, Type: strPtr("hosting")},
		},
		{
			name: "engagement without engagementType",
			req:  domain.UpdateCaseRequest{ID: testDeploymentUUID, Type: strPtr("engagement")},
		},
		{
			name: "engagement without engagementPaymentType",
			req: domain.UpdateCaseRequest{
				ID: testDeploymentUUID, Type: strPtr("engagement"), EngagementType: &engagement,
			},
		},
		{
			name: "service_request without catalogId/catalogItemId",
			req:  domain.UpdateCaseRequest{ID: testDeploymentUUID, Type: strPtr("service_request")},
		},
		{
			name: "engagementType supplied without type",
			req:  domain.UpdateCaseRequest{ID: testDeploymentUUID, EngagementType: &engagement},
		},
		{
			name: "engagementPaymentType supplied without type",
			req:  domain.UpdateCaseRequest{ID: testDeploymentUUID, EngagementPaymentType: &paymentType},
		},
		{
			name: "catalogId supplied without type",
			req:  domain.UpdateCaseRequest{ID: testDeploymentUUID, CatalogID: strPtr(testDeploymentUUID)},
		},
		{
			name: "engagementType supplied with type \"case\"",
			req: domain.UpdateCaseRequest{
				ID: testDeploymentUUID, Type: strPtr("case"), EngagementType: &engagement,
			},
		},
		{
			name: "engagementPaymentType supplied with type \"case\"",
			req: domain.UpdateCaseRequest{
				ID: testDeploymentUUID, Type: strPtr("case"), EngagementPaymentType: &paymentType,
			},
		},
		{
			name: "catalogId supplied with type \"engagement\"",
			req: domain.UpdateCaseRequest{
				ID: testDeploymentUUID, Type: strPtr("engagement"), EngagementType: &engagement,
				EngagementPaymentType: &paymentType,
				CatalogID:             strPtr(testDeploymentUUID),
			},
		},
		{
			name: "engagementType supplied with type \"service_request\"",
			req: domain.UpdateCaseRequest{
				ID: testDeploymentUUID, Type: strPtr("service_request"), EngagementType: &engagement,
				CatalogID: strPtr(testDeploymentUUID), CatalogItemID: strPtr(testDeploymentUUID),
			},
		},
		{
			name: "engagementPaymentType supplied with type \"service_request\"",
			req: domain.UpdateCaseRequest{
				ID: testDeploymentUUID, Type: strPtr("service_request"), EngagementPaymentType: &paymentType,
				CatalogID: strPtr(testDeploymentUUID), CatalogItemID: strPtr(testDeploymentUUID),
			},
		},
		{
			name: "type combined with severity",
			req: domain.UpdateCaseRequest{
				ID: testDeploymentUUID, Type: strPtr("case"), Severity: &severity,
			},
		},
		{
			// A bare product/publicTicket with no fix-ETA date has nowhere to
			// go -- addPublicComment's own handling only runs when
			// addPublicComment itself is set, so without this rejection the
			// field would otherwise be silently dropped rather than erroring.
			name: "type combined with a bare product (no fix-ETA date, no addPublicComment)",
			req: domain.UpdateCaseRequest{
				ID: testDeploymentUUID, Type: strPtr("case"), Product: strPtr("API Manager"),
			},
		},
		{
			// Standalone (no other field at all) -- addPublicComment's own
			// handling never runs without addPublicComment itself present, so
			// product/publicTicket would otherwise be silently dropped rather
			// than erroring or being forwarded.
			name: "standalone product with no addPublicComment",
			req:  domain.UpdateCaseRequest{ID: testDeploymentUUID, Product: strPtr("API Manager")},
		},
		{
			name: "standalone publicTicket with no addPublicComment",
			req:  domain.UpdateCaseRequest{ID: testDeploymentUUID, PublicTicket: strPtr("GH-123")},
		},
	}

	svc := NewServiceNowCaseService(nil, nil, nil, noopSLAClockService{}, nil, "", nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.UpdateCase(contextWithUserIDToken("token"), tt.req)
			if _, ok := err.(*apierror.ValidationError); !ok {
				t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
			}
		})
	}
}

func TestSNCaseService_UpdateCase_TypeTransfer_Case(t *testing.T) {
	typ := "case"
	sev := domain.CaseSeverityLow
	issue := domain.CaseIssueTypeQuestion
	var gotBody map[string]any
	client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"message": "Case updated successfully.",
			"case": {
				"id": "` + testWLCaseSysid + `",
				"updatedOn": "2026-01-02 10:00:00",
				"updatedBy": "engineer@example.com",
				"type": {"id": "8d4b87bd1b18f010cb6898aebd4bcb59", "name": "Case"}
			}
		}`))
	})

	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)
	// severity and issueType are both mandatory for this target: the backing data source
	// selects Incident vs Query from the severity, and stores issue type on those records.
	resp, err := svc.UpdateCase(contextWithUserIDToken("token"), domain.UpdateCaseRequest{
		ID: testDeploymentUUID, Type: &typ, Severity: &sev, IssueType: &issue,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, ok := gotBody["type"]; !ok || got != "default_case" {
		t.Fatalf("expected type %q, got %v (body: %+v)", "default_case", got, gotBody)
	}
	if got, ok := gotBody["issueTypeKey"]; !ok || got != float64(4) {
		t.Fatalf("expected issueTypeKey 4 for %q, got %v (body: %+v)", issue, got, gotBody)
	}
	if _, ok := gotBody["severityKey"]; !ok {
		t.Fatalf("expected severityKey to be sent alongside type: %+v", gotBody)
	}
	for _, field := range []string{"engagementType", "engagementPaymentType", "catalogId", "catalogItemId", "variables"} {
		if _, ok := gotBody[field]; ok {
			t.Fatalf("unexpected extra field %q present in a type: \"case\" payload: %+v", field, gotBody)
		}
	}
	if resp.Case.Type != "case" {
		t.Fatalf("expected echoed type \"case\", got %q", resp.Case.Type)
	}
}

func TestSNCaseService_UpdateCase_TypeTransfer_CaseRequiresSeverityAndIssueType(t *testing.T) {
	typ := "case"
	sev := domain.CaseSeverityLow
	issue := domain.CaseIssueTypeQuestion
	client := newTestCaseClient(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("backing service must not be called for an incomplete transfer")
		w.WriteHeader(http.StatusOK)
	})
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	for name, req := range map[string]domain.UpdateCaseRequest{
		"missing both":      {ID: testDeploymentUUID, Type: &typ},
		"missing issueType": {ID: testDeploymentUUID, Type: &typ, Severity: &sev},
		"missing severity":  {ID: testDeploymentUUID, Type: &typ, IssueType: &issue},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := svc.UpdateCase(contextWithUserIDToken("token"), req); err == nil {
				t.Fatal("expected a validation error, got nil")
			}
		})
	}
}

func TestSNCaseService_UpdateCase_TypeTransfer_IssueTypeRejectedForOtherTargets(t *testing.T) {
	issue := domain.CaseIssueTypeQuestion
	client := newTestCaseClient(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("backing service must not be called for a mismatched transfer")
		w.WriteHeader(http.StatusOK)
	})
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	for _, typ := range []string{"engagement", "security_report_analysis"} {
		t.Run(typ, func(t *testing.T) {
			target := typ
			if _, err := svc.UpdateCase(contextWithUserIDToken("token"), domain.UpdateCaseRequest{
				ID: testDeploymentUUID, Type: &target, IssueType: &issue,
			}); err == nil {
				t.Fatalf("expected issueType to be rejected for type %q", typ)
			}
		})
	}
}

func TestSNCaseService_UpdateCase_IssueTypeWithoutTypeRejected(t *testing.T) {
	issue := domain.CaseIssueTypeQuestion
	client := newTestCaseClient(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("backing service must not be called")
		w.WriteHeader(http.StatusOK)
	})
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)
	if _, err := svc.UpdateCase(contextWithUserIDToken("token"), domain.UpdateCaseRequest{
		ID: testDeploymentUUID, IssueType: &issue,
	}); err == nil {
		t.Fatal("expected issueType alone to be rejected")
	}
}

func TestSNCaseService_UpdateCase_TypeTransfer_Engagement(t *testing.T) {
	typ := "engagement"
	engagement := domain.EngagementTypeMigration
	paymentType := domain.EngagementPaymentTypePaid
	var gotBody map[string]any
	client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"message": "Case updated successfully.",
			"case": {"id": "` + testWLCaseSysid + `", "updatedOn": "2026-01-02 10:00:00", "updatedBy": "engineer@example.com"}
		}`))
	})

	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)
	_, err := svc.UpdateCase(contextWithUserIDToken("token"), domain.UpdateCaseRequest{
		ID: testDeploymentUUID, Type: &typ, EngagementType: &engagement, EngagementPaymentType: &paymentType,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := gotBody["type"]; got != "engagement" {
		t.Fatalf("expected type \"engagement\", got %v", got)
	}
	if got, ok := gotBody["engagementType"]; !ok || !jsonEqual(got, float64(1)) {
		t.Fatalf("expected engagementType 1 (migration), got %v (body: %+v)", got, gotBody)
	}
	if got, ok := gotBody["engagementPaymentType"]; !ok || !jsonEqual(got, float64(1)) {
		t.Fatalf("expected engagementPaymentType 1 (paid), got %v (body: %+v)", got, gotBody)
	}
}

func TestSNCaseService_UpdateCase_TypeTransfer_SecurityReportAnalysis(t *testing.T) {
	typ := "security_report_analysis"
	var gotBody map[string]any
	client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"message": "Case updated successfully.",
			"case": {"id": "` + testWLCaseSysid + `", "updatedOn": "2026-01-02 10:00:00", "updatedBy": "engineer@example.com"}
		}`))
	})

	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)
	_, err := svc.UpdateCase(contextWithUserIDToken("token"), domain.UpdateCaseRequest{
		ID: testDeploymentUUID, Type: &typ,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := gotBody["type"]; got != "security_report_analysis" {
		t.Fatalf("expected type \"security_report_analysis\", got %v", got)
	}
}

func TestSNCaseService_UpdateCase_TypeTransfer_ServiceRequest(t *testing.T) {
	typ := "service_request"
	catalogUUID := "44444444-4444-4444-4444-444444444444"
	catalogItemUUID := "55555555-5555-5555-5555-555555555555"
	variableUUID := "66666666-6666-6666-6666-666666666666"
	var gotBody map[string]any
	client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"message": "Case updated successfully.",
			"case": {"id": "` + testWLCaseSysid + `", "updatedOn": "2026-01-02 10:00:00", "updatedBy": "engineer@example.com"}
		}`))
	})

	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)
	_, err := svc.UpdateCase(contextWithUserIDToken("token"), domain.UpdateCaseRequest{
		ID:            testDeploymentUUID,
		Type:          &typ,
		CatalogID:     &catalogUUID,
		CatalogItemID: &catalogItemUUID,
		Variables:     []domain.Variable{{ID: variableUUID, Value: "Scaling for a launch"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := gotBody["type"]; got != "service_request" {
		t.Fatalf("expected type \"service_request\", got %v", got)
	}
	if got, ok := gotBody["catalogId"]; !ok || got != uuidToSysid(catalogUUID) {
		t.Fatalf("expected catalogId %q, got %v", uuidToSysid(catalogUUID), got)
	}
	if got, ok := gotBody["catalogItemId"]; !ok || got != uuidToSysid(catalogItemUUID) {
		t.Fatalf("expected catalogItemId %q, got %v", uuidToSysid(catalogItemUUID), got)
	}
	vars, ok := gotBody["variables"].([]any)
	if !ok || len(vars) != 1 {
		t.Fatalf("expected 1 variable, got %+v", gotBody["variables"])
	}
	v, ok := vars[0].(map[string]any)
	if !ok {
		t.Fatalf("expected variables[0] to be an object, got %+v", vars[0])
	}
	if got := v["id"]; got != uuidToSysid(variableUUID) {
		t.Fatalf("expected variables[0].id %q, got %v", uuidToSysid(variableUUID), got)
	}
	if got := v["value"]; got != "Scaling for a launch" {
		t.Fatalf("expected variables[0].value %q, got %v", "Scaling for a launch", got)
	}
}

func TestSNCaseService_UpdateCase_TypeTransfer_ServiceRequestRequiresVariables(t *testing.T) {
	typ := "service_request"
	catalogUUID := "44444444-4444-4444-4444-444444444444"
	catalogItemUUID := "55555555-5555-5555-5555-555555555555"
	client := newTestCaseClient(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("backing service must not be called when variables are missing")
		w.WriteHeader(http.StatusOK)
	})

	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)
	// The backing data source requires at least one variable for a service request, exactly as
	// it does at create time. A transfer with none would be rejected downstream, so reject it
	// here rather than spending the round-trip.
	if _, err := svc.UpdateCase(contextWithUserIDToken("token"), domain.UpdateCaseRequest{
		ID: testDeploymentUUID, Type: &typ, CatalogID: &catalogUUID, CatalogItemID: &catalogItemUUID,
	}); err == nil {
		t.Fatal("expected a validation error when variables are omitted")
	}
}

func TestSNCaseService_UpdateCase_TypeTransfer_SeverityRejectedForOtherTargets(t *testing.T) {
	sev := domain.CaseSeverityHigh
	engagement := domain.EngagementTypeMigration
	paymentType := domain.EngagementPaymentTypePaid
	catalogUUID := "44444444-4444-4444-4444-444444444444"
	catalogItemUUID := "55555555-5555-5555-5555-555555555555"
	variableUUID := "66666666-6666-6666-6666-666666666666"
	client := newTestCaseClient(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("backing service must not be called for a mismatched transfer")
		w.WriteHeader(http.StatusOK)
	})
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	tests := []struct {
		name string
		req  domain.UpdateCaseRequest
	}{
		{
			name: "engagement",
			req: domain.UpdateCaseRequest{
				ID: testDeploymentUUID, Type: strPtr("engagement"), EngagementType: &engagement,
				EngagementPaymentType: &paymentType, Severity: &sev,
			},
		},
		{
			name: "service_request",
			req: domain.UpdateCaseRequest{
				ID: testDeploymentUUID, Type: strPtr("service_request"),
				CatalogID: &catalogUUID, CatalogItemID: &catalogItemUUID,
				Variables: []domain.Variable{{ID: variableUUID, Value: "Scaling for a launch"}},
				Severity:  &sev,
			},
		},
		{
			name: "security_report_analysis",
			req: domain.UpdateCaseRequest{
				ID: testDeploymentUUID, Type: strPtr("security_report_analysis"), Severity: &sev,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := svc.UpdateCase(contextWithUserIDToken("token"), tt.req); err == nil {
				t.Fatalf("expected severity to be rejected for type %q", tt.name)
			}
		})
	}
}

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

// --- UpdateCase: field-count union (including the internal fix-ETA date variants) ---

func TestSNCaseService_UpdateCase_FieldCountValidation(t *testing.T) {
	svc := NewServiceNowCaseService(nil, nil, nil, noopSLAClockService{}, nil, "", nil)
	closed := domain.CaseStateClosed
	bestCase := "2026-08-01"

	tests := []struct {
		name string
		req  domain.UpdateCaseRequest
	}{
		{
			name: "no fields provided",
			req:  domain.UpdateCaseRequest{ID: testCaseUUID},
		},
		{
			name: "state and bestCaseFixEta both provided",
			req:  domain.UpdateCaseRequest{ID: testCaseUUID, State: &closed, BestCaseFixEta: &bestCase},
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

// --- UpdateCase: close no longer gated on open visible tasks ---
//
// The open-visible-task close-gate previously enforced here has been removed:
// it required two extra round trips (task search + per-task detail) to a
// dependency whose own pagination limit violated the entity-service's
// Pagination.limit constraint (max 50 vs the hardcoded 100 this gate sent),
// which broke every case close in production. That business rule belongs at
// the ServiceNow layer instead (see CaseUtils.patchCaseState's existing,
// zero-round-trip child-case-block-close pattern) -- tracked in
// tasks/active/2026-07-30-sn-close-gate-migration.md. This test guards
// against silently reintroducing the Go-side gate.

func TestSNCaseService_UpdateCase_Close_NoLongerCallsTaskSearch(t *testing.T) {
	taskSearchCalled := false
	patchCalled := false
	mux := http.NewServeMux()
	mux.HandleFunc("/cases/"+testCaseSysid+"/tasks/search", func(w http.ResponseWriter, r *http.Request) {
		taskSearchCalled = true
		_ = json.NewEncoder(w).Encode(map[string]any{"tasks": []map[string]any{}, "total": 0, "offset": 0, "limit": 100})
	})
	mux.HandleFunc("/cases/"+testCaseSysid, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			patchCalled = true
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"message": "ok", "case": map[string]any{"id": testCaseSysid, "updatedOn": "2026-01-01 00:00:00"}})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	closed := domain.CaseStateClosed
	if _, err := svc.UpdateCase(contextWithUserIDToken("token"), domain.UpdateCaseRequest{ID: testCaseUUID, State: &closed}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if taskSearchCalled {
		t.Fatalf("expected closing a case not to call /cases/{id}/tasks/search (close-gate removed from Go)")
	}
	if !patchCalled {
		t.Fatalf("expected PATCH /cases/{id} to be called")
	}
}

// --- Case tags ---

func TestSNCaseService_AddCaseTag_Validation(t *testing.T) {
	svc := NewServiceNowCaseService(nil, nil, nil, noopSLAClockService{}, nil, "", nil)

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
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

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
	svc := NewServiceNowCaseService(nil, nil, nil, noopSLAClockService{}, nil, "", nil)

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
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	if err := svc.RemoveCaseTag(contextWithUserIDToken("token"), testCaseUUID, testTagUUID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deleteCalled {
		t.Fatalf("expected DELETE /cases/{id}/tags/{tagId} to be called")
	}
}

// --- Internal-only fix-ETA estimates: best/most-likely/worst case ---

func TestSNCaseService_UpdateCase_FieldCountValidation_InternalFixEtaVariants(t *testing.T) {
	svc := NewServiceNowCaseService(nil, nil, nil, noopSLAClockService{}, nil, "", nil)
	closed := domain.CaseStateClosed
	bestCase := "2026-08-02"

	tests := []struct {
		name string
		req  domain.UpdateCaseRequest
	}{
		{
			name: "state and bestCaseFixEta both provided",
			req:  domain.UpdateCaseRequest{ID: testCaseUUID, State: &closed, BestCaseFixEta: &bestCase},
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

// TestSNCaseService_UpdateCase_CombinableFieldsCombineInSingleRequest verifies that the
// combinable-group fields (everything except state, severity, workState, watchList,
// assigneeEmail, and parentId) can be PATCHed together in one request and all land in a
// single payload sent to ServiceNow.
func TestSNCaseService_UpdateCase_CombinableFieldsCombineInSingleRequest(t *testing.T) {
	strPtr := func(s string) *string { return &s }
	bestCase := "2026-08-02"
	mostLikely := "2026-08-03"
	worstCase := "2026-08-04"

	var gotBody map[string]any
	client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"message": "Case updated successfully.",
			"case": {"id": "` + testWLCaseSysid + `", "updatedOn": "2026-01-02 10:00:00", "updatedBy": "engineer@example.com"}
		}`))
	})

	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)
	req := domain.UpdateCaseRequest{
		ID:               testDeploymentUUID,
		Subject:          strPtr("Updated subject"),
		Description:      strPtr("Updated description"),
		BestCaseFixEta:   &bestCase,
		MostLikelyFixEta: &mostLikely,
		WorstCaseFixEta:  &worstCase,
	}

	if _, err := svc.UpdateCase(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for field, want := range map[string]string{
		"title":            "Updated subject",
		"description":      "Updated description",
		"bestCaseFixEta":   bestCase,
		"mostLikelyFixEta": mostLikely,
		"worstCaseFixEta":  worstCase,
	} {
		got, ok := gotBody[field]
		if !ok {
			t.Fatalf("expected payload field %q to be present in %+v", field, gotBody)
		}
		if got != want {
			t.Fatalf("payload field %q: got %v, want %v", field, got, want)
		}
	}
}

func TestSNCaseService_UpdateCase_InternalFixEtaVariants_EachIndependentlySettable(t *testing.T) {
	tests := []struct {
		name    string
		req     func(v string) domain.UpdateCaseRequest
		bodyKey string
	}{
		{
			name: "bestCaseFixEta",
			req: func(v string) domain.UpdateCaseRequest {
				return domain.UpdateCaseRequest{ID: testCaseUUID, BestCaseFixEta: &v}
			},
			bodyKey: "bestCaseFixEta",
		},
		{
			name: "mostLikelyFixEta",
			req: func(v string) domain.UpdateCaseRequest {
				return domain.UpdateCaseRequest{ID: testCaseUUID, MostLikelyFixEta: &v}
			},
			bodyKey: "mostLikelyFixEta",
		},
		{
			name: "worstCaseFixEta",
			req: func(v string) domain.UpdateCaseRequest {
				return domain.UpdateCaseRequest{ID: testCaseUUID, WorstCaseFixEta: &v}
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
			svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

			value := "2026-03-01"
			_, err := svc.UpdateCase(contextWithUserIDToken("token"), tt.req(value))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got, _ := gotBody[tt.bodyKey].(string)
			want := "2026-03-01"
			if got != want {
				t.Fatalf("%s sent = %q, want %q", tt.bodyKey, got, want)
			}
		})
	}
}

func TestSNCaseService_UpdateCase_InternalFixEtaVariants_RejectsMalformedDate(t *testing.T) {
	tests := []struct {
		name string
		req  domain.UpdateCaseRequest
	}{
		{
			name: "bestCaseFixEta not YYYY-MM-DD",
			req:  domain.UpdateCaseRequest{ID: testCaseUUID, BestCaseFixEta: strPtr("2026-08-01T00:00:00Z")},
		},
		{
			name: "mostLikelyFixEta not a date",
			req:  domain.UpdateCaseRequest{ID: testCaseUUID, MostLikelyFixEta: strPtr("not-a-date")},
		},
		{
			name: "worstCaseFixEta empty string",
			req:  domain.UpdateCaseRequest{ID: testCaseUUID, WorstCaseFixEta: strPtr("")},
		},
	}

	svc := NewServiceNowCaseService(nil, nil, nil, noopSLAClockService{}, nil, "", nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.UpdateCase(contextWithUserIDToken("token"), tt.req)
			if _, ok := err.(*apierror.ValidationError); !ok {
				t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
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
			"bestCaseFixEta":   "2026-02-10",
			"mostLikelyFixEta": "2026-02-15",
			"worstCaseFixEta":  "2026-02-20",
		})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	cv, err := svc.GetCaseByID(contextWithUserIDToken("token"), testCaseUUID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cv.BestCaseFixEta == nil || *cv.BestCaseFixEta != "2026-02-10" {
		t.Fatalf("BestCaseFixEta = %v, want 2026-02-10", cv.BestCaseFixEta)
	}
	if cv.MostLikelyFixEta == nil || *cv.MostLikelyFixEta != "2026-02-15" {
		t.Fatalf("MostLikelyFixEta = %v, want 2026-02-15", cv.MostLikelyFixEta)
	}
	if cv.WorstCaseFixEta == nil || *cv.WorstCaseFixEta != "2026-02-20" {
		t.Fatalf("WorstCaseFixEta = %v, want 2026-02-20", cv.WorstCaseFixEta)
	}
}

func TestSNCaseService_UpdateCase_EchoesInternalFixEtaFieldsBack(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/cases/"+testCaseSysid, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": "ok",
			"case": map[string]any{
				"id": testCaseSysid, "updatedOn": "2026-01-01 00:00:00",
				"bestCaseFixEta":   "2026-02-10",
				"mostLikelyFixEta": "2026-02-15",
				"worstCaseFixEta":  "2026-02-20",
			},
		})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	bestCase := "2026-02-10"
	resp, err := svc.UpdateCase(contextWithUserIDToken("token"), domain.UpdateCaseRequest{ID: testCaseUUID, BestCaseFixEta: &bestCase})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Case.BestCaseFixEta == nil || *resp.Case.BestCaseFixEta != "2026-02-10" {
		t.Fatalf("BestCaseFixEta = %v, want 2026-02-10", resp.Case.BestCaseFixEta)
	}
	if resp.Case.MostLikelyFixEta == nil || *resp.Case.MostLikelyFixEta != "2026-02-15" {
		t.Fatalf("MostLikelyFixEta = %v, want 2026-02-15", resp.Case.MostLikelyFixEta)
	}
	if resp.Case.WorstCaseFixEta == nil || *resp.Case.WorstCaseFixEta != "2026-02-20" {
		t.Fatalf("WorstCaseFixEta = %v, want 2026-02-20", resp.Case.WorstCaseFixEta)
	}
}

// --- SearchTags ---

func TestSNCaseService_SearchCases_EmptyTypesFilterSendsNoTypeRestriction(t *testing.T) {
	// An empty/omitted Types filter must mean "search every case type" -- SN's
	// own case search already treats an empty caseTypes list this way. Guards
	// against reintroducing a default like ["default_case"], which silently
	// excluded service_request (and every other non-"case" type) from any
	// caller searching across all types, e.g. the "does this engineer already
	// have another ongoing work item" pre-check.
	var gotBody struct {
		Filters struct {
			CaseTypes *[]string `json:"caseTypes"`
		} `json:"filters"`
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/cases/search", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"cases": []map[string]any{}, "total": 0, "offset": 0, "limit": 20})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	if _, err := svc.SearchCases(contextWithUserIDToken("token"), domain.SearchCasesRequest{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Decode caseTypes as *[]string (not []string) so this distinguishes the
	// actual wire contract -- Go always sends an explicit empty array, never
	// omits the field or sends null -- from a regression that would omit it.
	if gotBody.Filters.CaseTypes == nil {
		t.Fatalf("expected caseTypes to be sent as an explicit empty array, got the field omitted/null")
	}
	if len(*gotBody.Filters.CaseTypes) != 0 {
		t.Fatalf("expected no caseTypes restriction sent when Types filter is empty, got %v", *gotBody.Filters.CaseTypes)
	}
}

// TestSNCaseService_SearchCases_HostingCaseTypesTranslate verifies the three
// SaaS/SRE-specific case types (hosting, hosting_query, hosting_task) are
// accepted by search and translated to their own SN wire values, not
// silently dropped by snCaseTypeMap the way they were before it carried
// entries for them.
func TestSNCaseService_SearchCases_HostingCaseTypesTranslate(t *testing.T) {
	var gotBody struct {
		Filters struct {
			CaseTypes []string `json:"caseTypes"`
		} `json:"filters"`
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/cases/search", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"cases": []map[string]any{}, "total": 0, "offset": 0, "limit": 20})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	req := domain.SearchCasesRequest{
		Filters: domain.SearchCasesFilters{
			Filters: []domain.CaseFieldFilter{
				{Field: "type", Op: "in", Values: []string{"hosting", "hosting_query", "hosting_task"}},
			},
		},
	}
	if _, err := svc.SearchCases(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"hosting", "hosting_query", "hosting_task"}
	if len(gotBody.Filters.CaseTypes) != len(want) {
		t.Fatalf("caseTypes = %v, want %v", gotBody.Filters.CaseTypes, want)
	}
	for i, w := range want {
		if gotBody.Filters.CaseTypes[i] != w {
			t.Fatalf("caseTypes[%d] = %q, want %q (hosting types must not be dropped)", i, gotBody.Filters.CaseTypes[i], w)
		}
	}
}

// TestSNCaseService_SearchCases_GenericFiltersTranslateToSNPayload proves the
// generic filters array (the new public contract) still produces the exact
// same named-field Ballerina payload SearchCases has always sent, just fed by
// ParseCaseFieldFilters + buildSNCaseFilters instead of directly by named
// request struct fields.
func TestSNCaseService_SearchCases_GenericFiltersTranslateToSNPayload(t *testing.T) {
	var gotBody snCaseSearchPayload
	mux := http.NewServeMux()
	mux.HandleFunc("/cases/search", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"cases": []map[string]any{}, "total": 0, "offset": 0, "limit": 20})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	req := domain.SearchCasesRequest{
		Filters: domain.SearchCasesFilters{
			Filters: []domain.CaseFieldFilter{
				{Field: "tag", Op: "in", Values: []string{"patch"}},
				{Field: "tag", Op: "notIn", Values: []string{"beta"}},
				{Field: "assignedUserId", Op: "isEmpty"},
				{Field: "resolutionNotes", Op: "isEmpty"},
				{Field: "createdBy", Op: "eq", Values: []string{currentUserFilterPlaceholder}},
				{Field: "projectType", Op: "in", Values: []string{"Subscription", "Free Trial"}},
				{Field: "slaBreached", Op: "eq", Values: []string{"true"}},
				{Field: "accountEscalationActive", Op: "eq", Values: []string{"true"}},
			},
		},
	}

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	if _, err := svc.SearchCases(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(gotBody.Filters.Tags) != 1 || gotBody.Filters.Tags[0] != "patch" {
		t.Fatalf("Tags = %v", gotBody.Filters.Tags)
	}
	if len(gotBody.Filters.ExcludeTags) != 1 || gotBody.Filters.ExcludeTags[0] != "beta" {
		t.Fatalf("ExcludeTags = %v", gotBody.Filters.ExcludeTags)
	}
	if !gotBody.Filters.Unassigned {
		t.Fatalf("expected Unassigned = true")
	}
	if !gotBody.Filters.ResolutionNotesEmpty {
		t.Fatalf("expected ResolutionNotesEmpty = true")
	}
	if !gotBody.Filters.CreatedByMe {
		t.Fatalf("expected CreatedByMe = true (forwarded as a flag, not resolved into CreatedBy)")
	}
	if len(gotBody.Filters.CreatedBy) != 0 {
		t.Fatalf("expected CreatedBy to stay empty for the current-user placeholder, got %v", gotBody.Filters.CreatedBy)
	}
	// projectType values are project-type NAMES passed through verbatim -- no
	// UUID validation, no id conversion (mirrors the product filter).
	if len(gotBody.Filters.ProjectTypeNames) != 2 ||
		gotBody.Filters.ProjectTypeNames[0] != "Subscription" ||
		gotBody.Filters.ProjectTypeNames[1] != "Free Trial" {
		t.Fatalf("ProjectTypeNames = %v, want [Subscription, Free Trial] passed through unchanged", gotBody.Filters.ProjectTypeNames)
	}
	if gotBody.Filters.SlaBreached == nil || !*gotBody.Filters.SlaBreached {
		t.Fatalf("SlaBreached = %v, want pointer to true", gotBody.Filters.SlaBreached)
	}
	if gotBody.Filters.AccountEscalationActive == nil || !*gotBody.Filters.AccountEscalationActive {
		t.Fatalf("AccountEscalationActive = %v, want pointer to true", gotBody.Filters.AccountEscalationActive)
	}
}

// TestSNCaseService_SearchCases_SLABreachedAndAccountEscalationTravelOnTheirOwnWireKeys
// pins the exact JSON wire keys for the two new plain-boolean filters --
// "slaBreached" and "accountEscalationActive" -- distinct from the existing
// case-level "isEscalated" key, and proves an explicit false is still sent on
// the wire (not dropped by omitempty, since both fields are *bool).
func TestSNCaseService_SearchCases_SLABreachedAndAccountEscalationTravelOnTheirOwnWireKeys(t *testing.T) {
	var rawBody []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/cases/search", func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		rawBody = b
		_ = json.NewEncoder(w).Encode(map[string]any{"cases": []map[string]any{}, "total": 0, "offset": 0, "limit": 20})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	req := domain.SearchCasesRequest{
		Filters: domain.SearchCasesFilters{
			Filters: []domain.CaseFieldFilter{
				{Field: "slaBreached", Op: "eq", Values: []string{"false"}},
				{Field: "accountEscalationActive", Op: "eq", Values: []string{"false"}},
			},
		},
	}
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	if _, err := svc.SearchCases(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var envelope struct {
		Filters map[string]json.RawMessage `json:"filters"`
	}
	if err := json.Unmarshal(rawBody, &envelope); err != nil {
		t.Fatalf("unmarshal request body: %v", err)
	}
	slaBreachedRaw, ok := envelope.Filters["slaBreached"]
	if !ok {
		t.Fatalf("expected \"slaBreached\" key on the wire even for an explicit false, filters = %v", envelope.Filters)
	}
	if string(slaBreachedRaw) != "false" {
		t.Fatalf("slaBreached = %s, want false", slaBreachedRaw)
	}
	accountEscalationRaw, ok := envelope.Filters["accountEscalationActive"]
	if !ok {
		t.Fatalf("expected \"accountEscalationActive\" key on the wire even for an explicit false, filters = %v", envelope.Filters)
	}
	if string(accountEscalationRaw) != "false" {
		t.Fatalf("accountEscalationActive = %s, want false", accountEscalationRaw)
	}
	if _, ok := envelope.Filters["isEscalated"]; ok {
		t.Fatalf("expected no \"isEscalated\" key: accountEscalationActive must not be conflated with the case-level escalation filter")
	}
}

// TestSNCaseService_SearchCases_CreTeamAndSreTeamFiltersTranslateToSysidsOnTheirWireKeys
// pins both team filters' wire form: creTeam (the renamed former
// integrationCsTeam filter) must still travel under the wire key
// "integrationCsTeamIds" -- the Ballerina/SN contract's key, unchanged by the
// Go-side rename -- and the new sreTeam filter must travel under "sreTeamIds",
// both as sysids converted from the UUIDs the filter DSL accepts.
func TestSNCaseService_SearchCases_CreTeamAndSreTeamFiltersTranslateToSysidsOnTheirWireKeys(t *testing.T) {
	var rawBody []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/cases/search", func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		rawBody = b
		_ = json.NewEncoder(w).Encode(map[string]any{"cases": []map[string]any{}, "total": 0, "offset": 0, "limit": 20})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	creUUID := sysidToUUID(testCreTeamSysid)
	sreUUID := sysidToUUID(testSreTeamSysid)

	req := domain.SearchCasesRequest{
		Filters: domain.SearchCasesFilters{
			Filters: []domain.CaseFieldFilter{
				{Field: "creTeam", Op: "in", Values: []string{creUUID}},
				{Field: "sreTeam", Op: "in", Values: []string{sreUUID}},
			},
		},
	}
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	if _, err := svc.SearchCases(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var envelope struct {
		Filters map[string]json.RawMessage `json:"filters"`
	}
	if err := json.Unmarshal(rawBody, &envelope); err != nil {
		t.Fatalf("decode request body: %v; raw: %s", err, rawBody)
	}

	gotCre, ok := envelope.Filters["integrationCsTeamIds"]
	if !ok {
		t.Fatalf("filters has no \"integrationCsTeamIds\" key; keys sent: %v", filterKeys(envelope.Filters))
	}
	if string(gotCre) != `["`+testCreTeamSysid+`"]` {
		t.Fatalf("integrationCsTeamIds = %s, want [%q]", gotCre, testCreTeamSysid)
	}

	gotSre, ok := envelope.Filters["sreTeamIds"]
	if !ok {
		t.Fatalf("filters has no \"sreTeamIds\" key; keys sent: %v", filterKeys(envelope.Filters))
	}
	if string(gotSre) != `["`+testSreTeamSysid+`"]` {
		t.Fatalf("sreTeamIds = %s, want [%q]", gotSre, testSreTeamSysid)
	}

	if _, ok := envelope.Filters["creTeamIds"]; ok {
		t.Fatal("filters carries a \"creTeamIds\" key -- the wire key must stay integrationCsTeamIds")
	}
}

// TestSNCaseService_SearchCases_ProjectTypeGoesOutAsNamesOnItsOwnKey pins the
// projectType filter's wire form against the raw request body. The values are
// readable project-type names and must travel untouched -- no id validation,
// no id conversion -- under "projectTypes". They must not go out under the
// retired id-based key: that one is declared as a fixed-width hex id array
// upstream, so a name sent through it fails request validation outright.
func TestSNCaseService_SearchCases_ProjectTypeGoesOutAsNamesOnItsOwnKey(t *testing.T) {
	var rawBody []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/cases/search", func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		rawBody = b
		_ = json.NewEncoder(w).Encode(map[string]any{"cases": []map[string]any{}, "total": 0, "offset": 0, "limit": 20})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	req := domain.SearchCasesRequest{
		Filters: domain.SearchCasesFilters{
			Filters: []domain.CaseFieldFilter{
				{Field: "projectType", Op: "in", Values: []string{"Subscription", "Free Trial"}},
			},
		},
	}
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	if _, err := svc.SearchCases(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var envelope struct {
		Filters map[string]json.RawMessage `json:"filters"`
	}
	if err := json.Unmarshal(rawBody, &envelope); err != nil {
		t.Fatalf("decode request body: %v; raw: %s", err, rawBody)
	}
	got, ok := envelope.Filters["projectTypes"]
	if !ok {
		t.Fatalf("filters has no \"projectTypes\" key; keys sent: %v", filterKeys(envelope.Filters))
	}
	if string(got) != `["Subscription","Free Trial"]` {
		t.Fatalf("projectTypes = %s, want the names passed through unchanged", got)
	}
	if _, ok := envelope.Filters["projectTypeIds"]; ok {
		t.Fatal("filters still carries the retired id-based project-type key")
	}
}

// TestSNCaseService_SearchCases_StateNotInTranslatesToExcludeStateKeys pins
// both halves of the state notIn filter's wire form: the key name and the
// value representation. Both matter because the backing data source drops a
// filter key it does not recognize instead of rejecting it, so a wrong key --
// or the right key carrying state names where numeric keys are expected --
// would silently widen the result set rather than fail. The assertions run
// against the raw request body, not the decoded struct, since decoding through
// the same struct tags would agree with any name they happened to carry.
func TestSNCaseService_SearchCases_StateNotInTranslatesToExcludeStateKeys(t *testing.T) {
	var rawBody []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/cases/search", func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		rawBody = b
		_ = json.NewEncoder(w).Encode(map[string]any{"cases": []map[string]any{}, "total": 0, "offset": 0, "limit": 20})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	req := domain.SearchCasesRequest{
		Filters: domain.SearchCasesFilters{
			Filters: []domain.CaseFieldFilter{
				{Field: "state", Op: "in", Values: []string{"open"}},
				{Field: "state", Op: "notIn", Values: []string{"awaiting_info", "solution_proposed", "closed"}},
			},
		},
	}

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	if _, err := svc.SearchCases(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var envelope struct {
		Filters map[string]json.RawMessage `json:"filters"`
	}
	if err := json.Unmarshal(rawBody, &envelope); err != nil {
		t.Fatalf("decode request body: %v; raw: %s", err, rawBody)
	}
	got, ok := envelope.Filters["excludeStateKeys"]
	if !ok {
		t.Fatalf("filters has no \"excludeStateKeys\" key; keys sent: %v", filterKeys(envelope.Filters))
	}
	// awaiting_info=18, solution_proposed=6, closed=3 -- the same numeric keys
	// the positive stateKeys list is built from.
	if string(got) != "[18,6,3]" {
		t.Fatalf("excludeStateKeys = %s, want [18,6,3] (numeric state keys, not names)", got)
	}
	// The positive list is unaffected: notIn must never be folded into it.
	if stateKeys, ok := envelope.Filters["stateKeys"]; !ok || string(stateKeys) != "[1]" {
		t.Fatalf("stateKeys = %s (present=%v), want [1]", stateKeys, ok)
	}
	// The pre-rename key must not appear at all.
	if _, ok := envelope.Filters["excludeStates"]; ok {
		t.Fatal("filters carries \"excludeStates\"; the wire key is \"excludeStateKeys\"")
	}
}

// TestSNCaseService_SearchCases_StateNotInOmittedWhenUnused guards the
// inertness of the filter: a request that does not ask for an exclusion must
// send no exclusion key, so nothing changes for existing callers while the
// downstream layers do not yet honor it.
func TestSNCaseService_SearchCases_StateNotInOmittedWhenUnused(t *testing.T) {
	var rawBody []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/cases/search", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		rawBody = b
		_ = json.NewEncoder(w).Encode(map[string]any{"cases": []map[string]any{}, "total": 0, "offset": 0, "limit": 20})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	req := domain.SearchCasesRequest{
		Filters: domain.SearchCasesFilters{
			Filters: []domain.CaseFieldFilter{{Field: "state", Op: "in", Values: []string{"open"}}},
		},
	}
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	if _, err := svc.SearchCases(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bytes.Contains(rawBody, []byte("excludeStateKeys")) {
		t.Fatalf("excludeStateKeys must be omitted when no notIn filter was given; body: %s", rawBody)
	}
}

// TestSNCaseService_SearchCases_StateNotInRejectsUnknownValue guards against
// an unrecognized state silently vanishing in the domain-to-key conversion,
// which for an exclusion would widen the result set.
func TestSNCaseService_SearchCases_StateNotInRejectsUnknownValue(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/cases/search", func(w http.ResponseWriter, r *http.Request) {
		t.Error("upstream must not be called for an invalid filter value")
		_ = json.NewEncoder(w).Encode(map[string]any{"cases": []map[string]any{}, "total": 0, "offset": 0, "limit": 20})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	req := domain.SearchCasesRequest{
		Filters: domain.SearchCasesFilters{
			Filters: []domain.CaseFieldFilter{{Field: "state", Op: "notIn", Values: []string{"not_a_state"}}},
		},
	}
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	_, err := svc.SearchCases(ctx, req)
	if err == nil {
		t.Fatal("expected a validation error for an unknown state (notIn) value")
	}
	if !strings.Contains(err.Error(), "not_a_state") {
		t.Fatalf("error = %v, want it to name the offending value", err)
	}
}

// filterKeys returns the keys of a decoded filters object, for test failure
// messages.
func filterKeys(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// TestSNCaseService_SearchCases_AnyOfKeepsSNOrGroupsWireFormat is the guard
// against the single most dangerous regression this rename could cause. The
// PUBLIC contract's cross-field-OR key was renamed filters.orGroups ->
// filters.anyOf (and each branch became an object with its own filters
// array), but the ServiceNow WIRE format must not move: CaseUtils' Script
// Include reads "orGroups" and silently ignores JSON keys it does not
// recognise, so a renamed wire key returns an UNFILTERED count with no error
// anywhere. This test therefore asserts on the raw outgoing JSON -- not a
// typed decode, which would happily re-map a renamed key -- that the body
// still carries "orGroups", still does NOT carry "anyOf", and that each
// branch is still a flat named-field object (no nested "filters" array).
func TestSNCaseService_SearchCases_AnyOfKeepsSNOrGroupsWireFormat(t *testing.T) {
	var rawBody []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/cases/search", func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		rawBody = b
		_ = json.NewEncoder(w).Encode(map[string]any{"cases": []map[string]any{}, "total": 0, "offset": 0, "limit": 20})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	req := domain.SearchCasesRequest{
		Filters: domain.SearchCasesFilters{
			Filters: []domain.CaseFieldFilter{
				{Field: "state", Op: "in", Values: []string{"open"}},
			},
			AnyOf: []domain.CaseFilterBranch{
				{Filters: []domain.CaseFieldFilter{
					{Field: "severity", Op: "in", Values: []string{"catastrophic"}},
					{Field: "workState", Op: "in", Values: []string{"ongoing"}},
				}},
				{Filters: []domain.CaseFieldFilter{
					{Field: "escalationLevel", Op: "in", Values: []string{"3"}},
				}},
			},
		},
	}

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	if _, err := svc.SearchCases(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(rawBody, &body); err != nil {
		t.Fatalf("unmarshal raw request body: %v (body=%s)", err, rawBody)
	}
	filters, ok := body["filters"].(map[string]any)
	if !ok {
		t.Fatalf("request body has no filters object: %s", rawBody)
	}
	if _, bad := filters["anyOf"]; bad {
		t.Fatalf(`SN payload must NOT carry the public-API key "anyOf": %s`, rawBody)
	}
	groups, ok := filters["orGroups"].([]any)
	if !ok {
		t.Fatalf(`SN payload lost the "orGroups" wire key (ServiceNow would silently return an unfiltered count): %s`, rawBody)
	}
	if len(groups) != 2 {
		t.Fatalf("orGroups length = %d, want 2: %s", len(groups), rawBody)
	}

	first, ok := groups[0].(map[string]any)
	if !ok {
		t.Fatalf("orGroups[0] is not an object: %s", rawBody)
	}
	if _, nested := first["filters"]; nested {
		t.Fatalf(`orGroups[0] must stay a flat named-field object, not the public API's {"filters": [...]} branch shape: %s`, rawBody)
	}
	// severity "catastrophic" -> severityKeys, workState "in_progress" ->
	// workStateKeys: the exact named-field branch shape CaseUtils reads.
	if _, ok := first["severityKeys"].([]any); !ok {
		t.Fatalf("orGroups[0].severityKeys missing: %s", rawBody)
	}
	if _, ok := first["workStateKeys"].([]any); !ok {
		t.Fatalf("orGroups[0].workStateKeys missing: %s", rawBody)
	}

	second, ok := groups[1].(map[string]any)
	if !ok {
		t.Fatalf("orGroups[1] is not an object: %s", rawBody)
	}
	levels, ok := second["escalationLevel"].([]any)
	if !ok || len(levels) != 1 || levels[0] != "3" {
		t.Fatalf("orGroups[1].escalationLevel = %v, want [\"3\"]: %s", second["escalationLevel"], rawBody)
	}
}

// TestSNCaseService_SearchCases_AnyOfBranchTagsFlowToSNOrGroups proves tag /
// excludeTags are now accepted inside an anyOf branch (the Go-side rejection
// was removed once ServiceNow's CaseUtils gained orGroups[].tags/excludeTags
// support) and that the values reach the SN payload's "orGroups" entries
// under the same "tags"/"excludeTags" keys the top-level filters object uses,
// nested one level into the branch object.
func TestSNCaseService_SearchCases_AnyOfBranchTagsFlowToSNOrGroups(t *testing.T) {
	var rawBody []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/cases/search", func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		rawBody = b
		_ = json.NewEncoder(w).Encode(map[string]any{"cases": []map[string]any{}, "total": 0, "offset": 0, "limit": 20})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	req := domain.SearchCasesRequest{
		Filters: domain.SearchCasesFilters{
			AnyOf: []domain.CaseFilterBranch{
				{Filters: []domain.CaseFieldFilter{
					{Field: "type", Op: "in", Values: []string{"engagement"}},
					{Field: "tag", Op: "in", Values: []string{"migration"}},
				}},
				{Filters: []domain.CaseFieldFilter{
					{Field: "type", Op: "in", Values: []string{"security_report_analysis"}},
					{Field: "tag", Op: "in", Values: []string{"s_migration"}},
					{Field: "tag", Op: "notIn", Values: []string{"archived"}},
				}},
			},
		},
	}

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	if _, err := svc.SearchCases(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v (tag/excludeTags must be accepted inside an anyOf branch)", err)
	}

	var body map[string]any
	if err := json.Unmarshal(rawBody, &body); err != nil {
		t.Fatalf("unmarshal raw request body: %v (body=%s)", err, rawBody)
	}
	filters, ok := body["filters"].(map[string]any)
	if !ok {
		t.Fatalf("request body has no filters object: %s", rawBody)
	}
	groups, ok := filters["orGroups"].([]any)
	if !ok || len(groups) != 2 {
		t.Fatalf("orGroups = %v, want 2 entries: %s", filters["orGroups"], rawBody)
	}

	first, ok := groups[0].(map[string]any)
	if !ok {
		t.Fatalf("orGroups[0] is not an object: %s", rawBody)
	}
	tags, ok := first["tags"].([]any)
	if !ok || len(tags) != 1 || tags[0] != "migration" {
		t.Fatalf("orGroups[0].tags = %v, want [\"migration\"]: %s", first["tags"], rawBody)
	}

	second, ok := groups[1].(map[string]any)
	if !ok {
		t.Fatalf("orGroups[1] is not an object: %s", rawBody)
	}
	sTags, ok := second["tags"].([]any)
	if !ok || len(sTags) != 1 || sTags[0] != "s_migration" {
		t.Fatalf("orGroups[1].tags = %v, want [\"s_migration\"]: %s", second["tags"], rawBody)
	}
	excludeTags, ok := second["excludeTags"].([]any)
	if !ok || len(excludeTags) != 1 || excludeTags[0] != "archived" {
		t.Fatalf("orGroups[1].excludeTags = %v, want [\"archived\"]: %s", second["excludeTags"], rawBody)
	}
}

// TestSNCaseService_SearchCases_RejectsBadFilterFieldAndCombo proves invalid
// field names and invalid field/op combinations are rejected before ever
// reaching the backing service, not silently ignored or forwarded.
func TestSNCaseService_SearchCases_RejectsBadFilterFieldAndCombo(t *testing.T) {
	client := newTestSNClient(t, http.NewServeMux())
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	t.Run("bad field name", func(t *testing.T) {
		req := domain.SearchCasesRequest{Filters: domain.SearchCasesFilters{
			Filters: []domain.CaseFieldFilter{{Field: "bogusField", Op: "in", Values: []string{"x"}}},
		}}
		_, err := svc.SearchCases(ctx, req)
		if _, ok := err.(*apierror.ValidationError); !ok {
			t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
		}
	})

	t.Run("bad field+op combo", func(t *testing.T) {
		req := domain.SearchCasesRequest{Filters: domain.SearchCasesFilters{
			Filters: []domain.CaseFieldFilter{{Field: "type", Op: "gte", Values: []string{"case"}}},
		}}
		_, err := svc.SearchCases(ctx, req)
		if _, ok := err.(*apierror.ValidationError); !ok {
			t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
		}
	})
}

// TestSNCaseService_SearchCases_RejectsUnrecognizedEnumValues proves that
// State/Severity/IssueType/EngagementType filter values not present in the
// domain's validXxx maps are rejected outright, rather than being silently
// dropped by domainStatesToSNIDs/domainSeveritiesToSNIDs/domainIssueTypesToSNIDs/
// domainEngagementTypesToSNIDs (which skip unrecognized values, producing an
// empty key slice that omitempty then drops from the SN payload entirely --
// previously this widened the result set instead of erroring).
func TestSNCaseService_SearchCases_RejectsUnrecognizedEnumValues(t *testing.T) {
	client := newTestSNClient(t, http.NewServeMux())
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	cases := []struct {
		name   string
		filter domain.CaseFieldFilter
	}{
		{name: "state", filter: domain.CaseFieldFilter{Field: "state", Op: "in", Values: []string{"bogus_state"}}},
		{name: "severity", filter: domain.CaseFieldFilter{Field: "severity", Op: "in", Values: []string{"bogus_severity"}}},
		{name: "issueType", filter: domain.CaseFieldFilter{Field: "issueType", Op: "in", Values: []string{"bogus_issue_type"}}},
		{name: "engagementType", filter: domain.CaseFieldFilter{Field: "engagementType", Op: "in", Values: []string{"bogus_engagement_type"}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := domain.SearchCasesRequest{Filters: domain.SearchCasesFilters{
				Filters: []domain.CaseFieldFilter{tc.filter},
			}}
			_, err := svc.SearchCases(ctx, req)
			if _, ok := err.(*apierror.ValidationError); !ok {
				t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
			}
		})
	}
}

// TestSNCaseService_SearchCases_AcceptsAllPreviouslyValidEnumValues proves the
// new enum validation added alongside TestSNCaseService_SearchCases_RejectsUnrecognizedEnumValues
// does not newly reject any value that was previously forwarded to SN, for
// each of State/Severity/IssueType/EngagementType.
func TestSNCaseService_SearchCases_AcceptsAllPreviouslyValidEnumValues(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/cases/search", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"cases": []map[string]any{}, "total": 0, "offset": 0, "limit": 20})
	})
	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	allStates := make([]string, 0, len(validCaseState))
	for s := range validCaseState {
		allStates = append(allStates, string(s))
	}
	allSeverities := make([]string, 0, len(validCaseSeverity))
	for s := range validCaseSeverity {
		allSeverities = append(allSeverities, string(s))
	}
	allIssueTypes := make([]string, 0, len(validCaseIssueType))
	for s := range validCaseIssueType {
		allIssueTypes = append(allIssueTypes, string(s))
	}
	allEngagementTypes := make([]string, 0, len(validEngagementType))
	for s := range validEngagementType {
		allEngagementTypes = append(allEngagementTypes, string(s))
	}

	req := domain.SearchCasesRequest{Filters: domain.SearchCasesFilters{
		Filters: []domain.CaseFieldFilter{
			{Field: "state", Op: "in", Values: allStates},
			{Field: "severity", Op: "in", Values: allSeverities},
			{Field: "issueType", Op: "in", Values: allIssueTypes},
			{Field: "engagementType", Op: "in", Values: allEngagementTypes},
		},
	}}
	if _, err := svc.SearchCases(ctx, req); err != nil {
		t.Fatalf("unexpected error for previously-valid enum values: %v", err)
	}
}

// TestSNCaseService_SearchCases_PopulatesUpdatedOn proves SearchCases carries
// a real, non-empty updatedOn distinct from createdOn when the SN response
// supplies one, fixing the case-list "Updated" column always showing the
// created date.
func TestSNCaseService_SearchCases_PopulatesUpdatedOn(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/cases/search", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"cases": []map[string]any{
				{
					"id":              "case-sys-id",
					"internalId":      "INT-1",
					"number":          "CS0001",
					"title":           "t",
					"description":     "d",
					"createdOn":       "2026-01-01 00:00:00",
					"updatedOn":       "2026-01-15 12:30:00",
					"createdBy":       "jane.doe@example.com",
					"project":         map[string]any{"id": "proj-sys-id", "name": "Proj"},
					"deployment":      map[string]any{"id": "", "name": ""},
					"deployedProduct": map[string]any{"id": "", "name": "", "product": map[string]any{"id": "", "name": ""}},
				},
			},
			"total": 1, "offset": 0, "limit": 20,
		})
	})
	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	resp, err := svc.SearchCases(ctx, domain.SearchCasesRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Cases) != 1 {
		t.Fatalf("expected 1 case, got %d", len(resp.Cases))
	}
	got := resp.Cases[0]
	if got.UpdatedOn == "" {
		t.Fatalf("expected non-empty UpdatedOn")
	}
	if got.UpdatedOn == got.CreatedOn {
		t.Fatalf("expected UpdatedOn %q to differ from CreatedOn %q", got.UpdatedOn, got.CreatedOn)
	}
	if got.UpdatedOn != "2026-01-15 12:30:00" {
		t.Fatalf("UpdatedOn = %q, want the SN updatedOn field value", got.UpdatedOn)
	}
}

// TestSNCaseService_SearchCases_SetsIncludeExtendedFields proves SearchCases
// always opts every outbound search request into the account/project-key/
// fix-ETA fields on search rows -- the backing service only returns them when
// this flag is explicitly set true, and defaults to leaving them off (the
// customer-portal's own search calls never set it, so the flag is what keeps
// this call additive rather than a shared-contract change).
func TestSNCaseService_SearchCases_SetsIncludeExtendedFields(t *testing.T) {
	var gotBody snCaseSearchPayload
	mux := http.NewServeMux()
	mux.HandleFunc("/cases/search", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"cases": []map[string]any{}, "total": 0, "offset": 0, "limit": 20})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	if _, err := svc.SearchCases(contextWithUserIDToken("token"), domain.SearchCasesRequest{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !gotBody.IncludeExtendedFields {
		t.Fatalf("expected includeExtendedFields to be sent as true on every search request")
	}
}

// TestSNCaseService_SearchCases_MapsExtendedFieldsWhenPresent proves that when
// the mocked SN response includes the account reference, project key, and the
// three fix-ETA fields on a row, SearchCases maps every one of them through to
// the returned SearchCaseView.
func TestSNCaseService_SearchCases_MapsExtendedFieldsWhenPresent(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/cases/search", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"cases": []map[string]any{
				{
					"id":               "case-sys-id",
					"internalId":       "INT-1",
					"number":           "CS0001",
					"title":            "t",
					"description":      "d",
					"createdOn":        "2026-01-01 00:00:00",
					"createdBy":        "jane.doe@example.com",
					"project":          map[string]any{"id": "proj-sys-id", "name": "Proj", "key": "TESTQUERYSUB"},
					"deployment":       map[string]any{"id": "", "name": ""},
					"deployedProduct":  map[string]any{"id": "", "name": "", "product": map[string]any{"id": "", "name": ""}},
					"account":          map[string]any{"id": "acct-sys-id", "name": "Acme Corp", "type": "premium"},
					"bestCaseFixEta":   "2026-04-15",
					"mostLikelyFixEta": "2026-04-22",
					"worstCaseFixEta":  "2026-04-29",
				},
			},
			"total": 1, "offset": 0, "limit": 20,
		})
	})
	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	resp, err := svc.SearchCases(ctx, domain.SearchCasesRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Cases) != 1 {
		t.Fatalf("expected 1 case, got %d", len(resp.Cases))
	}
	got := resp.Cases[0]

	if got.ProjectKey == nil || *got.ProjectKey != "TESTQUERYSUB" {
		t.Fatalf("ProjectKey = %v, want \"TESTQUERYSUB\"", got.ProjectKey)
	}
	if got.AccountDetails == nil {
		t.Fatalf("expected AccountDetails to be populated")
	}
	if got.AccountDetails.Name != "Acme Corp" || got.AccountDetails.Type != "premium" {
		t.Fatalf("AccountDetails = %+v, want Name=Acme Corp Type=premium", got.AccountDetails)
	}
	if got.BestCaseFixEta == nil || *got.BestCaseFixEta != "2026-04-15" {
		t.Fatalf("BestCaseFixEta = %v, want \"2026-04-15\"", got.BestCaseFixEta)
	}
	if got.MostLikelyFixEta == nil || *got.MostLikelyFixEta != "2026-04-22" {
		t.Fatalf("MostLikelyFixEta = %v, want \"2026-04-22\"", got.MostLikelyFixEta)
	}
	if got.WorstCaseFixEta == nil || *got.WorstCaseFixEta != "2026-04-29" {
		t.Fatalf("WorstCaseFixEta = %v, want \"2026-04-29\"", got.WorstCaseFixEta)
	}
}

// TestSNCaseService_SearchCases_ExtendedFieldsAbsentDoNotPanic proves that
// when a mocked SN response omits the account/project-key/fix-ETA fields
// entirely (simulating the backing service ignoring includeExtendedFields, or
// an older response shape), SearchCases does not panic or error -- the new
// fields come through as nil, matching this file's existing nil-handling
// convention for every other optional reference.
func TestSNCaseService_SearchCases_ExtendedFieldsAbsentDoNotPanic(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/cases/search", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"cases": []map[string]any{
				{
					"id":              "case-sys-id",
					"internalId":      "INT-1",
					"number":          "CS0001",
					"title":           "t",
					"description":     "d",
					"createdOn":       "2026-01-01 00:00:00",
					"createdBy":       "jane.doe@example.com",
					"project":         map[string]any{"id": "proj-sys-id", "name": "Proj"},
					"deployment":      map[string]any{"id": "", "name": ""},
					"deployedProduct": map[string]any{"id": "", "name": "", "product": map[string]any{"id": "", "name": ""}},
				},
			},
			"total": 1, "offset": 0, "limit": 20,
		})
	})
	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	resp, err := svc.SearchCases(ctx, domain.SearchCasesRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Cases) != 1 {
		t.Fatalf("expected 1 case, got %d", len(resp.Cases))
	}
	got := resp.Cases[0]

	if got.ProjectKey != nil {
		t.Fatalf("ProjectKey = %v, want nil", got.ProjectKey)
	}
	if got.AccountDetails != nil {
		t.Fatalf("AccountDetails = %v, want nil", got.AccountDetails)
	}
	if got.BestCaseFixEta != nil {
		t.Fatalf("BestCaseFixEta = %v, want nil", got.BestCaseFixEta)
	}
	if got.MostLikelyFixEta != nil {
		t.Fatalf("MostLikelyFixEta = %v, want nil", got.MostLikelyFixEta)
	}
	if got.WorstCaseFixEta != nil {
		t.Fatalf("WorstCaseFixEta = %v, want nil", got.WorstCaseFixEta)
	}
}

// TestSNCaseService_SearchTags_Success pins the upstream wire format: tag search is a POST
// with the query nested under `filters.searchQuery`, not a GET with a `q` param. The body is
// asserted as raw JSON on purpose — decoding it through a struct with the same tags would
// agree with whatever key the payload happens to carry, which is how an earlier wire-format
// bug got through.
func TestSNCaseService_SearchTags_Success(t *testing.T) {
	var gotMethod string
	var gotBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/tags/search", func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tags": []map[string]any{
				{"id": testTagSysid, "label": "micro-gw", "color": "#f97316"},
			},
		})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	tags, err := svc.SearchTags(contextWithUserIDToken("token"), domain.SearchTagsRequest{
		Filters: domain.SearchTagsFilters{SearchQuery: "micro"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", gotMethod)
	}
	filters, ok := gotBody["filters"].(map[string]any)
	if !ok {
		t.Fatalf("body has no filters object: %#v", gotBody)
	}
	if filters["searchQuery"] != "micro" {
		t.Fatalf("filters.searchQuery = %v, want micro", filters["searchQuery"])
	}
	if _, present := gotBody["q"]; present {
		t.Fatalf("body must not carry the legacy q key: %#v", gotBody)
	}
	if _, present := gotBody["limit"]; present {
		t.Fatalf("limit must be omitted when unset: %#v", gotBody)
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

func TestSNCaseService_SearchTags_ForwardsLimit(t *testing.T) {
	var rawBody []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/tags/search", func(w http.ResponseWriter, r *http.Request) {
		rawBody, _ = io.ReadAll(r.Body)
		_ = json.NewEncoder(w).Encode(map[string]any{"tags": []map[string]any{}})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	if _, err := svc.SearchTags(contextWithUserIDToken("token"), domain.SearchTagsRequest{
		Filters: domain.SearchTagsFilters{SearchQuery: "micro"},
		Limit:   5,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := `{"filters":{"searchQuery":"micro"},"limit":5}`; strings.TrimSpace(string(rawBody)) != want {
		t.Fatalf("raw body = %s, want %s", rawBody, want)
	}
}

func TestSNCaseService_SearchTags_EmptyQuery(t *testing.T) {
	var rawBody []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/tags/search", func(w http.ResponseWriter, r *http.Request) {
		rawBody, _ = io.ReadAll(r.Body)
		_ = json.NewEncoder(w).Encode(map[string]any{"tags": []map[string]any{}})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	tags, err := svc.SearchTags(contextWithUserIDToken("token"), domain.SearchTagsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := `{"filters":{}}`; strings.TrimSpace(string(rawBody)) != want {
		t.Fatalf("raw body = %s, want %s", rawBody, want)
	}
	if len(tags) != 0 {
		t.Fatalf("expected 0 tags, got %d", len(tags))
	}
}

// TestSNCaseService_SearchTags_NeverSendsCaseID guards the deliberate decision not to expose
// the case-scoped variant upward: the upstream contract accepts filters.caseId, but nothing
// above the entity service consumes it, so the entity service must never emit it.
func TestSNCaseService_SearchTags_NeverSendsCaseID(t *testing.T) {
	var gotBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/tags/search", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{"tags": []map[string]any{}})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	if _, err := svc.SearchTags(contextWithUserIDToken("token"), domain.SearchTagsRequest{
		Filters: domain.SearchTagsFilters{SearchQuery: "micro"},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	filters, _ := gotBody["filters"].(map[string]any)
	if _, present := filters["caseId"]; present {
		t.Fatalf("filters must not carry caseId: %#v", filters)
	}
}

func TestSNCaseService_SearchTags_QueryTooLong(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tags/search", func(w http.ResponseWriter, r *http.Request) {
		t.Error("upstream must not be called for an over-long query")
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	_, err := svc.SearchTags(contextWithUserIDToken("token"), domain.SearchTagsRequest{
		Filters: domain.SearchTagsFilters{SearchQuery: strings.Repeat("a", 201)},
	})
	if _, ok := err.(*apierror.ValidationError); !ok {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

func TestCaseService_SearchTags_ServiceUnavailable(t *testing.T) {
	svc := &caseService{}

	if _, err := svc.SearchTags(contextWithUserIDToken("token"), domain.SearchTagsRequest{
		Filters: domain.SearchTagsFilters{SearchQuery: "micro"},
	}); err == nil {
		t.Fatalf("expected error")
	} else if _, ok := err.(*apierror.ServiceUnavailableError); !ok {
		t.Fatalf("expected *apierror.ServiceUnavailableError, got %T: %v", err, err)
	}
}

// TestSNCaseService_GetCaseByID_MapsLinkedChangeRequests covers the reverse side of the
// service-request <-> change-request link. Upstream sends the list under `changeRequestsAll`
// (unfiltered by change-request state, unlike the older `changeRequests` field) with 32-hex
// ids; the domain exposes it as `linkedChangeRequests` with canonical UUIDs.
//
// The cardinality cases matter: a service request can have several change requests (one per
// environment the change is promoted to), so a single-value mapping would look correct
// against a record that happens to have exactly one and be wrong in production.
func TestSNCaseService_GetCaseByID_MapsLinkedChangeRequests(t *testing.T) {
	crSysidA := sysid32('1')
	crSysidB := sysid32('2')

	newBody := func(changeRequests string) string {
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
			"changeRequestsAll": ` + changeRequests + `
		}`
	}

	get := func(t *testing.T, changeRequests string) domain.CaseView {
		t.Helper()
		client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(newBody(changeRequests)))
		})
		svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

		cv, err := svc.GetCaseByID(contextWithUserIDToken("token"), sysidToUUID(testWLCaseSysid))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return cv
	}

	t.Run("null stays empty", func(t *testing.T) {
		cv := get(t, "null")
		if len(cv.LinkedChangeRequests) != 0 {
			t.Fatalf("expected no linked change requests, got %+v", cv.LinkedChangeRequests)
		}
	})

	t.Run("single entry maps with a canonical UUID", func(t *testing.T) {
		cv := get(t, `[{"id": "`+crSysidA+`", "number": "CHG0000001", "name": "Promote to dev"}]`)
		if len(cv.LinkedChangeRequests) != 1 {
			t.Fatalf("expected 1 linked change request, got %d", len(cv.LinkedChangeRequests))
		}
		got := cv.LinkedChangeRequests[0]
		if got.ID != sysidToUUID(crSysidA) {
			t.Fatalf("expected id %q, got %q", sysidToUUID(crSysidA), got.ID)
		}
		if got.Number != "CHG0000001" || got.Name == nil || *got.Name != "Promote to dev" {
			t.Fatalf("unexpected mapping: %+v", got)
		}
	})

	t.Run("several entries all map, order preserved", func(t *testing.T) {
		cv := get(t, `[
			{"id": "`+crSysidA+`", "number": "CHG0000001", "name": "Promote to dev"},
			{"id": "`+crSysidB+`", "number": "CHG0000002", "name": ""}
		]`)
		if len(cv.LinkedChangeRequests) != 2 {
			t.Fatalf("expected 2 linked change requests, got %d", len(cv.LinkedChangeRequests))
		}
		if cv.LinkedChangeRequests[0].Number != "CHG0000001" || cv.LinkedChangeRequests[1].Number != "CHG0000002" {
			t.Fatalf("order not preserved: %+v", cv.LinkedChangeRequests)
		}
		if cv.LinkedChangeRequests[1].ID != sysidToUUID(crSysidB) {
			t.Fatalf("expected id %q, got %q", sysidToUUID(crSysidB), cv.LinkedChangeRequests[1].ID)
		}
		// An absent upstream subject must surface as nil, not "": the two are
		// otherwise indistinguishable to a caller.
		if cv.LinkedChangeRequests[1].Name != nil {
			t.Fatalf("expected an empty upstream name to map to nil, got %q", *cv.LinkedChangeRequests[1].Name)
		}
	})
}

// caseDetailBody is a minimal, valid GET /cases/{id} response body for the given sysid,
// used by the tags-population tests below where only the tags side-fetch is under test.
func caseDetailBody(caseSysid string) string {
	return `{
		"id": "` + caseSysid + `",
		"internalId": "WSO2-001",
		"number": "CS0001001",
		"title": "Case subject",
		"description": "Case description",
		"createdOn": "2026-01-01 10:00:00",
		"project": {"id": "` + testProjectSysid + `", "name": "Project A"},
		"deployment": {"id": "", "name": ""},
		"deployedProduct": {"id": "", "name": "", "version": ""},
		"state": {"id": 1, "label": "Open"}
	}`
}

// TestSNCaseService_GetCaseByID_PopulatesTags verifies GetCaseByID fetches the case's
// current tags from the case-scoped GET /cases/{id}/tags resource and maps them onto
// CaseView.Tags, the same way SearchTags/AddCaseTag map the shared snTag shape.
func TestSNCaseService_GetCaseByID_PopulatesTags(t *testing.T) {
	var gotTagsPath string
	mux := http.NewServeMux()
	mux.HandleFunc("/cases/"+testCaseSysid, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(caseDetailBody(testCaseSysid)))
	})
	mux.HandleFunc("/cases/"+testCaseSysid+"/tags", func(w http.ResponseWriter, r *http.Request) {
		gotTagsPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tags": []map[string]any{
				{"id": testTagSysid, "label": "micro-gw", "color": "#f97316"},
			},
		})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	cv, err := svc.GetCaseByID(contextWithUserIDToken("token"), testCaseUUID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotTagsPath != "/cases/"+testCaseSysid+"/tags" {
		t.Fatalf("tags endpoint not called, got path %q", gotTagsPath)
	}
	if len(cv.Tags) != 1 {
		t.Fatalf("expected 1 tag, got %d: %+v", len(cv.Tags), cv.Tags)
	}
	if cv.Tags[0].ID != testTagUUID || cv.Tags[0].Label != "micro-gw" {
		t.Fatalf("unexpected tag mapping: %+v", cv.Tags[0])
	}
	if cv.Tags[0].Color == nil || *cv.Tags[0].Color != "#f97316" {
		t.Fatalf("tag.Color = %v, want #f97316", cv.Tags[0].Color)
	}
}

// TestSNCaseService_GetCaseByID_TagsFetchFailureDoesNotFailRead verifies that a failing
// tags lookup is soft-failed: the case detail read still succeeds (matching this file's
// established soft-fail convention for supplementary side-fetches, e.g.
// resolveUserGroups/resolveProjectAccess in sn_user_service.go), with CaseView.Tags left
// nil rather than the whole GetCaseByID call returning an error.
func TestSNCaseService_GetCaseByID_TagsFetchFailureDoesNotFailRead(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/cases/"+testCaseSysid, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(caseDetailBody(testCaseSysid)))
	})
	mux.HandleFunc("/cases/"+testCaseSysid+"/tags", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message": "internal error"}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	cv, err := svc.GetCaseByID(contextWithUserIDToken("token"), testCaseUUID)
	if err != nil {
		t.Fatalf("expected GetCaseByID to succeed despite the tags-lookup failure, got: %v", err)
	}
	if cv.Tags != nil {
		t.Fatalf("expected Tags to stay nil on a fetch failure, got %+v", cv.Tags)
	}
	if cv.Number != "CS0001001" {
		t.Fatalf("expected the rest of the case detail to still be populated, got %+v", cv)
	}
}
