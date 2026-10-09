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

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const createIncidentGroupBase = `{"callerId":"11111111-1111-1111-1111-111111111111","category":"SECURITY","serviceId":"22222222-2222-2222-2222-222222222222","impact":"HIGH","urgency":"HIGH","subject":"Something broke"`

// assignmentGroupId is optional on create: a UUID (or a blank/null value) is
// forwarded in the body unchanged; anything else is a 400 with no upstream call.
func TestCreateIncident_AssignmentGroupID(t *testing.T) {
	for name, tc := range map[string]struct {
		extra    string
		wantCode int
	}{
		"a UUID is forwarded": {`,"assignmentGroupId":"33333333-3333-4333-8333-333333333333"`, http.StatusCreated},
		"null is not sent":    {`,"assignmentGroupId":null`, http.StatusCreated},
		"blank is not sent":   {`,"assignmentGroupId":""`, http.StatusCreated},
		"a name is refused":   {`,"assignmentGroupId":"Network"`, http.StatusBadRequest},
		"a number is refused": {`,"assignmentGroupId":42`, http.StatusBadRequest},
	} {
		t.Run(name, func(t *testing.T) {
			body := createIncidentGroupBase + tc.extra + `}`
			var forwarded []byte
			h := NewIncidentHandler(&mockEntityIncidentClient{
				createIncidentFn: func(_ context.Context, b []byte) ([]byte, error) {
					forwarded = b
					return []byte(`{"message":"incident created"}`), nil
				},
			})
			w := httptest.NewRecorder()
			h.CreateIncident(w, withUser(httptest.NewRequest(http.MethodPost, "/incidents", strings.NewReader(body))))
			assertStatus(t, w, tc.wantCode)
			if tc.wantCode == http.StatusCreated && string(forwarded) != body {
				t.Errorf("forwarded %s, want the body unchanged: %s", forwarded, body)
			}
			if tc.wantCode == http.StatusBadRequest && forwarded != nil {
				t.Errorf("the entity service was called for a refused body: %s", forwarded)
			}
		})
	}
}

// filters.supportGroupsOnly reaches the entity service: the body is forwarded
// byte for byte, never re-marshalled through a struct that could drop it.
func TestSearchGroups_ForwardsSupportGroupsOnly(t *testing.T) {
	const body = `{"filters":{"searchQuery":"sre","supportGroupsOnly":true},"pagination":{"limit":20,"offset":0}}`
	var forwarded []byte
	h := NewGroupHandler(&mockEntityGroupClient{
		searchGroupsFn: func(_ context.Context, b []byte) ([]byte, error) {
			forwarded = b
			return []byte(`{"groups":[],"total":0,"limit":20,"offset":0}`), nil
		},
	})
	w := httptest.NewRecorder()
	h.SearchGroups(w, withUser(httptest.NewRequest(http.MethodPost, "/groups/search", strings.NewReader(body))))
	assertStatus(t, w, http.StatusOK)
	if string(forwarded) != body {
		t.Errorf("forwarded %s, want %s", forwarded, body)
	}
}

func TestGetIncidentCreateDefaults(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{})
		w := httptest.NewRecorder()
		h.GetIncidentCreateDefaults(w, httptest.NewRequest(http.MethodGet, "/incidents/create-defaults", nil))
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("passes the entity service's answer through", func(t *testing.T) {
		const body = `{"defaultServiceId":"44444444-4444-4444-8444-444444444444","defaultGroup":{"id":"55555555-5555-4555-8555-555555555555","name":"SRE"}}`
		h := NewIncidentHandler(&mockEntityIncidentClient{
			getIncidentCreateDefaultsFn: func(context.Context) ([]byte, error) { return []byte(body), nil },
		})
		w := httptest.NewRecorder()
		h.GetIncidentCreateDefaults(w, withUser(httptest.NewRequest(http.MethodGet, "/incidents/create-defaults", nil)))
		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if got := strings.TrimSpace(w.Body.String()); got != body {
			t.Errorf("body = %s, want %s", got, body)
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to load the incident defaults.") {
			t.Run(tc.name, func(t *testing.T) {
				h := NewIncidentHandler(&mockEntityIncidentClient{
					getIncidentCreateDefaultsFn: func(context.Context) ([]byte, error) { return nil, tc.err },
				})
				w := httptest.NewRecorder()
				h.GetIncidentCreateDefaults(w, withUser(httptest.NewRequest(http.MethodGet, "/incidents/create-defaults", nil)))
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
			})
		}
	})
}

// The refused-group 400 keeps its status, message and errorCode; every other
// upstream 400 (whose message may quote database details) stays generic.
func TestCreateIncident_RefusedGroupPassesThrough(t *testing.T) {
	body := createIncidentGroupBase + `,"assignmentGroupId":"33333333-3333-4333-8333-333333333333"}`
	for name, tc := range map[string]struct {
		upstream string
		want     string
	}{
		"refused group": {
			upstream: `{"code":400,"message":"assignmentGroupId must be the active support group of a service","errorCode":"incident_assignment_group_not_allowed"}`,
			want:     `{"message":"assignmentGroupId must be the active support group of a service","errorCode":"incident_assignment_group_not_allowed"}`,
		},
		"other 400": {
			upstream: `{"code":400,"message":"one or more referenced IDs do not exist: Key (caller_id)=(x) is not present in table \"user\"."}`,
			want:     `{"message":"Failed to create incident."}`,
		},
		"other code": {
			upstream: `{"code":400,"message":"secret detail","errorCode":"something_else"}`,
			want:     `{"message":"Failed to create incident."}`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			h := NewIncidentHandler(&mockEntityIncidentClient{
				createIncidentFn: func(context.Context, []byte) ([]byte, error) {
					return nil, &apierror.Error{StatusCode: http.StatusBadRequest, Body: tc.upstream}
				},
			})
			w := httptest.NewRecorder()
			h.CreateIncident(w, withUser(httptest.NewRequest(http.MethodPost, "/incidents", strings.NewReader(body))))
			assertStatus(t, w, http.StatusBadRequest)
			if got := strings.TrimSpace(w.Body.String()); got != tc.want {
				t.Errorf("body = %s, want %s", got, tc.want)
			}
		})
	}
}
