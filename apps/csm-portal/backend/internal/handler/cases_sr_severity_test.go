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
)

// The case routes forward bodies and responses as raw JSON. These tests pin
// that a service request's internal severity, and the machine-readable
// warnings array a type transfer can return, cross the BFF unchanged in both
// directions, so a future typed DTO cannot silently drop them.

const srSeverityCaseID = "11111111-1111-1111-1111-111111111111"

func TestCreateCase_ServiceRequestSeverityPassesThrough(t *testing.T) {
	const reqBody = `{"type":"service_request","projectId":"p","deploymentId":"d","deployedProductId":"dp","catalogId":"c","catalogItemId":"ci","variables":{"a":"b"},"severity":"high"}`
	var forwarded map[string]any
	h := NewCaseHandler(&mockEntityCaseClient{
		createCaseFn: func(_ context.Context, body []byte) ([]byte, error) {
			if err := json.Unmarshal(body, &forwarded); err != nil {
				t.Fatalf("forwarded body not JSON: %v", err)
			}
			return []byte(`{"id":"` + srSeverityCaseID + `","severity":"high"}`), nil
		},
	})
	r := withUser(httptest.NewRequest(http.MethodPost, "/cases", strings.NewReader(reqBody)))
	w := httptest.NewRecorder()
	h.CreateCase(w, r)
	assertStatus(t, w, http.StatusCreated)
	if forwarded["severity"] != "high" {
		t.Errorf("forwarded severity = %v, want high", forwarded["severity"])
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["severity"] != "high" {
		t.Errorf("response severity = %v, want high", resp["severity"])
	}
}

func TestGetCase_ServiceRequestSeverityPassesThrough(t *testing.T) {
	h := NewCaseHandler(&mockEntityCaseClient{
		getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
			return []byte(`{"id":"` + srSeverityCaseID + `","type":"service_request","state":"open","severity":"low"}`), nil
		},
	})
	r := withUser(httptest.NewRequest(http.MethodGet, "/cases/"+srSeverityCaseID, nil))
	r.SetPathValue("id", srSeverityCaseID)
	w := httptest.NewRecorder()
	h.GetCase(w, r)
	assertStatus(t, w, http.StatusOK)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["severity"] != "low" {
		t.Errorf("severity = %v, want low", resp["severity"])
	}
}

func TestSearchCases_ServiceRequestSeverityPassesThrough(t *testing.T) {
	h := NewCaseHandler(&mockEntityCaseClient{
		searchCasesFn: func(_ context.Context, _ []byte) ([]byte, error) {
			return []byte(`{"cases":[{"id":"` + srSeverityCaseID + `","type":"service_request","severity":"medium"}],"totalRecords":1}`), nil
		},
	})
	r := withUser(httptest.NewRequest(http.MethodPost, "/cases/search", strings.NewReader(`{"filters":{"type":["service_request"]}}`)))
	w := httptest.NewRecorder()
	h.SearchCases(w, r)
	assertStatus(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"severity":"medium"`) {
		t.Errorf("search response lost severity: %s", w.Body.String())
	}
}

func TestPatchCase_SeverityAndTransferWarningPassThrough(t *testing.T) {
	const upstream = `{"message":"ok","case":{"id":"` + srSeverityCaseID + `","severity":"critical"},"warnings":[{"code":"SEVERITY_DOWNGRADED","message":"stored one level lower","from":"catastrophic","to":"critical"}]}`

	cases := []struct {
		name string
		body string
	}{
		{"update severity on a service request", `{"severity":"medium"}`},
		{"transfer to service request", `{"type":"service_request","catalogId":"c","catalogItemId":"ci","variables":{"a":"b"}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var forwarded string
			h := NewCaseHandler(&mockEntityCaseClient{
				patchCaseFn: func(_ context.Context, id string, body []byte) ([]byte, error) {
					forwarded = string(body)
					return []byte(upstream), nil
				},
			})
			r := withUser(httptest.NewRequest(http.MethodPatch, "/cases/"+srSeverityCaseID, strings.NewReader(tc.body)))
			r.SetPathValue("id", srSeverityCaseID)
			w := httptest.NewRecorder()
			h.PatchCase(w, r)
			assertStatus(t, w, http.StatusOK)
			if forwarded != tc.body {
				t.Errorf("forwarded body = %s, want %s", forwarded, tc.body)
			}
			var got, want map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			_ = json.Unmarshal([]byte(upstream), &want)
			gw, _ := json.Marshal(got["warnings"])
			ww, _ := json.Marshal(want["warnings"])
			if string(gw) != string(ww) {
				t.Errorf("warnings = %s, want %s", gw, ww)
			}
			if got["case"].(map[string]any)["severity"] != "critical" {
				t.Errorf("case.severity lost: %v", got["case"])
			}
		})
	}
}
