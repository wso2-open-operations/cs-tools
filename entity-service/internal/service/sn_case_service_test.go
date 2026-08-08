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
	"io"
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
		svc := NewServiceNowCaseService(client, nil)

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
		svc := NewServiceNowCaseService(client, nil)

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
	svc := NewServiceNowCaseService(client, nil)

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
		cv, err := NewServiceNowCaseService(client, nil).GetCaseByID(contextWithUserIDToken("token"), sysidToUUID(testWLCaseSysid))
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
		cv, err := NewServiceNowCaseService(client, nil).GetCaseByID(contextWithUserIDToken("token"), sysidToUUID(testWLCaseSysid))
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

// --- UpdateCase: field-count union (including the internal fix-ETA date variants) ---

func TestSNCaseService_UpdateCase_FieldCountValidation(t *testing.T) {
	svc := NewServiceNowCaseService(nil, nil)
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
	svc := NewServiceNowCaseService(client, nil)

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

	svc := NewServiceNowCaseService(client, nil)
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
			svc := NewServiceNowCaseService(client, nil)

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
	svc := NewServiceNowCaseService(client, nil)

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
	svc := NewServiceNowCaseService(client, nil)

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
	svc := NewServiceNowCaseService(client, nil)

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
	svc := NewServiceNowCaseService(client, nil)

	req := domain.SearchCasesRequest{
		Filters: domain.SearchCasesFilters{
			Filters: []domain.CaseFieldFilter{
				{Field: "tag", Op: "in", Values: []string{"patch"}},
				{Field: "tag", Op: "notIn", Values: []string{"beta"}},
				{Field: "assignedUserId", Op: "isEmpty"},
				{Field: "resolutionNotes", Op: "isEmpty"},
				{Field: "createdBy", Op: "eq", Values: []string{currentUserFilterPlaceholder}},
				{Field: "projectType", Op: "in", Values: []string{"Subscription", "Free Trial"}},
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
	svc := NewServiceNowCaseService(client, nil)

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

// TestSNCaseService_SearchCases_RejectsBadFilterFieldAndCombo proves invalid
// field names and invalid field/op combinations are rejected before ever
// reaching the backing service, not silently ignored or forwarded.
func TestSNCaseService_SearchCases_RejectsBadFilterFieldAndCombo(t *testing.T) {
	client := newTestSNClient(t, http.NewServeMux())
	svc := NewServiceNowCaseService(client, nil)
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
	svc := NewServiceNowCaseService(client, nil)
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
	svc := NewServiceNowCaseService(client, nil)
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
	svc := NewServiceNowCaseService(client, nil)
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

	tags, err := svc.SearchTags(contextWithUserIDToken("token"), "micro", 0)
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

func TestSNCaseService_SearchTags_ForwardsLimit(t *testing.T) {
	var gotLimit string
	mux := http.NewServeMux()
	mux.HandleFunc("/tags/search", func(w http.ResponseWriter, r *http.Request) {
		gotLimit = r.URL.Query().Get("limit")
		_ = json.NewEncoder(w).Encode(map[string]any{"tags": []map[string]any{}})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil)

	if _, err := svc.SearchTags(contextWithUserIDToken("token"), "micro", 5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotLimit != "5" {
		t.Fatalf("limit sent = %q, want 5", gotLimit)
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

	tags, err := svc.SearchTags(contextWithUserIDToken("token"), "", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tags) != 0 {
		t.Fatalf("expected 0 tags, got %d", len(tags))
	}
}

func TestCaseService_SearchTags_ServiceUnavailable(t *testing.T) {
	svc := &caseService{}

	if _, err := svc.SearchTags(contextWithUserIDToken("token"), "micro", 0); err == nil {
		t.Fatalf("expected error")
	} else if _, ok := err.(*apierror.ServiceUnavailableError); !ok {
		t.Fatalf("expected *apierror.ServiceUnavailableError, got %T: %v", err, err)
	}
}

// TestSNCaseService_GetCaseByID_MapsLinkedChangeRequests covers the reverse side of the
// service-request <-> change-request link. Upstream sends the list under `changeRequests`
// with 32-hex ids; the domain exposes it as `linkedChangeRequests` with canonical UUIDs.
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
			"changeRequests": ` + changeRequests + `
		}`
	}

	get := func(t *testing.T, changeRequests string) domain.CaseView {
		t.Helper()
		client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(newBody(changeRequests)))
		})
		svc := NewServiceNowCaseService(client, nil)

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
	svc := NewServiceNowCaseService(client, nil)

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
	svc := NewServiceNowCaseService(client, nil)

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
