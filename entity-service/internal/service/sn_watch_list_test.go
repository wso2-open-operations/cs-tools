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
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

const (
	testWatcherEmail1    = "jane.doe@example.com"
	testWatcherEmail2    = "john.doe@example.com"
	testWatcherUserName1 = "jane.doe"
	testWatcherUserName2 = "john.doe"
	// testUnknownWatcherUUID resolves to no user in watchListUserSearchStub.
	testUnknownWatcherUUID = "00000000-0000-0000-0000-000000000000"
)

// watchListUserSearchStub answers POST /users/search from a two-user directory,
// returning only the users whose ids the request asked for. Results come back in
// the directory's own order rather than the request's, so a caller that relies on
// the response order instead of re-ordering by id fails these tests.
func watchListUserSearchStub(t *testing.T) http.HandlerFunc {
	t.Helper()

	directory := map[string]map[string]any{
		testIncidentWatcherSysid1: {
			"id": testIncidentWatcherSysid1, "userName": testWatcherUserName1,
			"name": "Jane Doe", "email": testWatcherEmail1, "active": true,
		},
		testIncidentWatcherSysid2: {
			"id": testIncidentWatcherSysid2, "userName": testWatcherUserName2,
			"name": "John Doe", "email": testWatcherEmail2, "active": true,
		},
	}
	// Directory order, deliberately the reverse of the order the tests request.
	order := []string{testIncidentWatcherSysid2, testIncidentWatcherSysid1}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST on /users/search, got %s", r.Method)
		}
		var body struct {
			Filters struct {
				UserIDs []string `json:"userIds"`
			} `json:"filters"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode /users/search body: %v", err)
		}
		asked := make(map[string]struct{}, len(body.Filters.UserIDs))
		for _, id := range body.Filters.UserIDs {
			asked[id] = struct{}{}
		}

		users := []map[string]any{}
		for _, id := range order {
			if _, ok := asked[id]; ok {
				users = append(users, directory[id])
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"users": users, "totalRecords": len(users), "limit": len(users), "offset": 0,
		})
	}
}

// TestSNCaseService_CreateCase_WatchListResolvedToEmails verifies the case-create
// path resolves watch-list user ids to email addresses, in the caller's order --
// the backing service's create payload declares emails, so an id reaches it as an
// unresolvable value and the watcher is silently lost.
func TestSNCaseService_CreateCase_WatchListResolvedToEmails(t *testing.T) {
	var gotBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/users/search", watchListUserSearchStub(t))
	mux.HandleFunc("/cases", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
			"message": "Case created successfully",
			"case": {"id": "` + testWLCaseSysid + `", "number": "CS0000010", "createdBy": "engineer@example.com", "createdOn": "2026-01-02 10:00:00", "state": {"id": 1, "label": "Open"}}
		}`))
	})

	svc := NewServiceNowCaseService(newTestSNClient(t, mux), nil, nil, noopSLAClockService{}, nil, "", nil)

	req := domain.CreateCaseRequest{
		Type:                  "engagement",
		ProjectID:             testProjectUUID,
		DeploymentID:          testDeploymentUUID,
		DeployedProductID:     testDeployedProdID,
		Subject:               "Migration planning",
		Description:           "Plan the migration",
		EngagementType:        domain.EngagementTypeMigration,
		EngagementPaymentType: domain.EngagementPaymentTypePaid,
		WatchList:             []string{testIncidentWatcherUUID1, testIncidentWatcherUUID2},
	}

	if _, err := svc.CreateCase(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertWatchListPayload(t, gotBody, []string{testWatcherEmail1, testWatcherEmail2})
}

// TestSNCaseService_UpdateCase_WatchListResolvedToEmails mirrors the create-path
// coverage for PATCH /cases/{id}, whose payload declares emails as well.
func TestSNCaseService_UpdateCase_WatchListResolvedToEmails(t *testing.T) {
	var gotBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/users/search", watchListUserSearchStub(t))
	mux.HandleFunc("/cases/"+testWLCaseSysid, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"message": "Case updated successfully",
			"case": {"id": "` + testWLCaseSysid + `", "updatedOn": "2026-01-03 10:00:00", "updatedBy": "engineer@example.com"}
		}`))
	})

	svc := NewServiceNowCaseService(newTestSNClient(t, mux), nil, nil, noopSLAClockService{}, nil, "", nil)

	watchList := []string{testIncidentWatcherUUID1, testIncidentWatcherUUID2}
	_, err := svc.UpdateCase(contextWithUserIDToken("token"), domain.UpdateCaseRequest{
		ID:        sysidToUUID(testWLCaseSysid),
		WatchList: &watchList,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertWatchListPayload(t, gotBody, []string{testWatcherEmail1, testWatcherEmail2})
}

// TestSNIncidentService_UpdateIncident_WatchListClearedByEmptyList verifies an
// explicitly empty watch list still reaches the backing service as an empty list.
// The incident-update payload replaces the whole watch list, so an empty list is
// the only way to clear it; a length guard on the outgoing field would turn "clear
// the watch list" into a silent no-op.
func TestSNIncidentService_UpdateIncident_WatchListClearedByEmptyList(t *testing.T) {
	var gotBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/users/search", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("clearing the watch list must not trigger a user lookup")
	})
	mux.HandleFunc("/incidents/"+testIncidentSysid, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"message": "Incident updated successfully.",
			"incident": {"id": "` + testIncidentSysid + `", "number": "INC0001", "createdOn": "2026-01-01 00:00:00", "createdBy": "engineer@example.com"}
		}`))
	})

	svc := NewServiceNowIncidentService(newTestSNClient(t, mux), nil)

	watchList := []string{}
	_, err := svc.UpdateIncident(contextWithUserIDToken("token"), domain.UpdateIncidentRequest{
		ID:        testIncidentUUID,
		WatchList: &watchList,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, present := gotBody["watchList"]
	if !present {
		t.Fatalf("watchList absent from payload %+v, want an empty list so the watch list is cleared", gotBody)
	}
	list, ok := got.([]any)
	if !ok || len(list) != 0 {
		t.Fatalf("watchList = %#v, want an empty list", got)
	}
}

// TestSNCaseService_UpdateCase_WatchListAbsentVsEmpty covers the three states the
// case-update watch list can be in. UpdateCaseRequest.WatchList is a pointer so
// "not mentioned" and "explicitly empty" stay distinguishable: only the former may
// leave the field out of the outgoing payload, and only the latter clears the list.
func TestSNCaseService_UpdateCase_WatchListAbsentVsEmpty(t *testing.T) {
	emptyWatchList := []string{}
	populatedWatchList := []string{testIncidentWatcherUUID1, testIncidentWatcherUUID2}
	assignee := "jane.doe@example.com"

	tests := []struct {
		name           string
		req            domain.UpdateCaseRequest
		wantPresent    bool
		wantWatchList  []string
		wantUserLookup bool
	}{
		{
			name: "absent watch list is not sent",
			req: domain.UpdateCaseRequest{
				ID:            sysidToUUID(testWLCaseSysid),
				AssigneeEmail: &assignee,
			},
			wantPresent: false,
		},
		{
			name: "explicitly empty watch list is sent as an empty list",
			req: domain.UpdateCaseRequest{
				ID:        sysidToUUID(testWLCaseSysid),
				WatchList: &emptyWatchList,
			},
			wantPresent:   true,
			wantWatchList: []string{},
		},
		{
			name: "populated watch list still resolves to identities in order",
			req: domain.UpdateCaseRequest{
				ID:        sysidToUUID(testWLCaseSysid),
				WatchList: &populatedWatchList,
			},
			wantPresent:    true,
			wantWatchList:  []string{testWatcherEmail1, testWatcherEmail2},
			wantUserLookup: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotBody map[string]any
			userLookupCalled := false
			mux := http.NewServeMux()
			mux.HandleFunc("/users/search", func(w http.ResponseWriter, r *http.Request) {
				userLookupCalled = true
				watchListUserSearchStub(t)(w, r)
			})
			mux.HandleFunc("/cases/"+testWLCaseSysid, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPatch {
					t.Fatalf("expected PATCH, got %s", r.Method)
				}
				if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
					t.Fatalf("decode request body: %v", err)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{
					"message": "Case updated successfully",
					"case": {"id": "` + testWLCaseSysid + `", "updatedOn": "2026-01-03 10:00:00", "updatedBy": "engineer@example.com"}
				}`))
			})

			svc := NewServiceNowCaseService(newTestSNClient(t, mux), nil, nil, noopSLAClockService{}, nil, "", nil)
			if _, err := svc.UpdateCase(contextWithUserIDToken("token"), tt.req); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got, present := gotBody["watchList"]
			if present != tt.wantPresent {
				t.Fatalf("watchList present = %v, want %v (payload %+v)", present, tt.wantPresent, gotBody)
			}
			if tt.wantPresent {
				list, ok := got.([]any)
				if !ok {
					t.Fatalf("watchList = %#v, want an array", got)
				}
				if len(list) != len(tt.wantWatchList) {
					t.Fatalf("watchList length: got %d, want %d", len(list), len(tt.wantWatchList))
				}
				for i, w := range tt.wantWatchList {
					if list[i] != w {
						t.Fatalf("watchList[%d]: got %v, want %q", i, list[i], w)
					}
				}
			}
			if userLookupCalled != tt.wantUserLookup {
				t.Fatalf("user lookup called = %v, want %v", userLookupCalled, tt.wantUserLookup)
			}
		})
	}
}

// TestSNCaseService_UpdateCase_EmptyWatchListFieldAccounting verifies an explicitly
// empty watch list participates in the request's field accounting exactly as a
// populated one does: it satisfies the at-least-one-field rule on its own, and it
// is still subject to the rule that keeps the exclusive fields out of any
// combination.
func TestSNCaseService_UpdateCase_EmptyWatchListFieldAccounting(t *testing.T) {
	emptyWatchList := []string{}
	populatedWatchList := []string{testIncidentWatcherUUID1}
	assignee := "jane.doe@example.com"
	relatedCase := testRelatedCaseUUID

	tests := []struct {
		name    string
		req     domain.UpdateCaseRequest
		wantErr bool
	}{
		{
			name: "empty watch list alone satisfies the at-least-one-field rule",
			req: domain.UpdateCaseRequest{
				ID:        sysidToUUID(testWLCaseSysid),
				WatchList: &emptyWatchList,
			},
		},
		{
			name: "empty watch list cannot be combined with another exclusive field",
			req: domain.UpdateCaseRequest{
				ID:            sysidToUUID(testWLCaseSysid),
				WatchList:     &emptyWatchList,
				AssigneeEmail: &assignee,
			},
			wantErr: true,
		},
		{
			name: "empty watch list cannot be combined with a combinable field",
			req: domain.UpdateCaseRequest{
				ID:            sysidToUUID(testWLCaseSysid),
				WatchList:     &emptyWatchList,
				RelatedCaseID: &relatedCase,
			},
			wantErr: true,
		},
		{
			name: "populated watch list is rejected in the same combination",
			req: domain.UpdateCaseRequest{
				ID:            sysidToUUID(testWLCaseSysid),
				WatchList:     &populatedWatchList,
				RelatedCaseID: &relatedCase,
			},
			wantErr: true,
		},
		{
			name:    "no field at all is still rejected",
			req:     domain.UpdateCaseRequest{ID: sysidToUUID(testWLCaseSysid)},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/users/search", watchListUserSearchStub(t))
			mux.HandleFunc("/cases/"+testWLCaseSysid, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{
					"message": "Case updated successfully",
					"case": {"id": "` + testWLCaseSysid + `", "updatedOn": "2026-01-03 10:00:00", "updatedBy": "engineer@example.com"}
				}`))
			})

			svc := NewServiceNowCaseService(newTestSNClient(t, mux), nil, nil, noopSLAClockService{}, nil, "", nil)
			_, err := svc.UpdateCase(contextWithUserIDToken("token"), tt.req)
			if tt.wantErr {
				if _, ok := err.(*apierror.ValidationError); !ok {
					t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// TestWatchListResolution_UnknownUserID verifies that a well-formed id matching no
// user is a validation error on every watch-list path, rather than a watcher that
// vanishes from an otherwise successful write.
func TestWatchListResolution_UnknownUserID(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/users/search", watchListUserSearchStub(t))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected write to %s: an unresolvable watcher must fail before the write", r.URL.Path)
	})
	client := newTestSNClient(t, mux)

	caseSvc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)
	incidentSvc := NewServiceNowIncidentService(client, nil)
	unknown := []string{testIncidentWatcherUUID1, testUnknownWatcherUUID}

	tests := []struct {
		name string
		call func() error
	}{
		{
			name: "case create",
			call: func() error {
				_, err := caseSvc.CreateCase(contextWithUserIDToken("token"), domain.CreateCaseRequest{
					Type:                  "engagement",
					ProjectID:             testProjectUUID,
					DeploymentID:          testDeploymentUUID,
					DeployedProductID:     testDeployedProdID,
					Subject:               "Migration planning",
					Description:           "Plan the migration",
					EngagementType:        domain.EngagementTypeMigration,
					EngagementPaymentType: domain.EngagementPaymentTypePaid,
					WatchList:             unknown,
				})
				return err
			},
		},
		{
			name: "case update",
			call: func() error {
				watchList := unknown
				_, err := caseSvc.UpdateCase(contextWithUserIDToken("token"), domain.UpdateCaseRequest{
					ID:        sysidToUUID(testWLCaseSysid),
					WatchList: &watchList,
				})
				return err
			},
		},
		{
			name: "incident create",
			call: func() error {
				req := validCreateIncidentRequest()
				req.WatchList = unknown
				_, err := incidentSvc.CreateIncident(contextWithUserIDToken("token"), req)
				return err
			},
		},
		{
			name: "incident update",
			call: func() error {
				watchList := unknown
				_, err := incidentSvc.UpdateIncident(contextWithUserIDToken("token"), domain.UpdateIncidentRequest{
					ID:        testIncidentUUID,
					WatchList: &watchList,
				})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()
			verr, ok := err.(*apierror.ValidationError)
			if !ok {
				t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
			}
			if !strings.Contains(verr.Msg, testUnknownWatcherUUID) {
				t.Fatalf("error %q does not name the offending id %q", verr.Msg, testUnknownWatcherUUID)
			}
		})
	}
}

// assertWatchListPayload checks the outgoing payload's watchList against want,
// order included.
func assertWatchListPayload(t *testing.T, body map[string]any, want []string) {
	t.Helper()

	got, ok := body["watchList"].([]any)
	if !ok {
		t.Fatalf("expected watchList array in payload, got %+v", body["watchList"])
	}
	if len(got) != len(want) {
		t.Fatalf("watchList length: got %d, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i] != w {
			t.Fatalf("watchList[%d]: got %v, want %q", i, got[i], w)
		}
	}
}
