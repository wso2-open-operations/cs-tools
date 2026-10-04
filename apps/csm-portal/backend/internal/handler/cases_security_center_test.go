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

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// Tests for the Security Center scoping applied to case reads for a caller
// without PermViewSecurityCenter (scopeCaseSearchBody, caseViewIsSecurityReport,
// requireCaseVisibleToCaller).

var testViewerUser = &middleware.UserInfo{Email: "viewer@example.com", UserID: "viewer-1", Roles: []string{"test-viewer"}}

func withViewerUser(r *http.Request) *http.Request {
	return r.WithContext(middleware.WithUserInfo(r.Context(), testViewerUser))
}

func guardedCaseHandler(client *mockEntityCaseClient) *CaseHandler {
	return NewCaseHandler(client).WithAccessGuard(NewAccessGuard(testAccessConfig()))
}

// topLevelTypeFilter returns the values of the top-level type predicate in a
// forwarded search body, or nil when there is none.
func topLevelTypeFilter(t *testing.T, body []byte) []string {
	t.Helper()
	var req struct {
		Filters struct {
			Filters []struct {
				Field  string   `json:"field"`
				Op     string   `json:"op"`
				Values []string `json:"values"`
			} `json:"filters"`
		} `json:"filters"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("forwarded body is not JSON: %v (%s)", err, body)
	}
	for _, f := range req.Filters.Filters {
		if f.Field == "type" {
			if f.Op != "in" {
				t.Fatalf("type predicate op = %q, want in", f.Op)
			}
			return f.Values
		}
	}
	return nil
}

func assertScopedToNonSecurityTypes(t *testing.T, body []byte) {
	t.Helper()
	got := topLevelTypeFilter(t, body)
	if len(got) != len(nonSecurityCaseTypes) {
		t.Fatalf("injected type filter = %v, want %v (body %s)", got, nonSecurityCaseTypes, body)
	}
	for i := range got {
		if got[i] != nonSecurityCaseTypes[i] {
			t.Fatalf("injected type filter = %v, want %v", got, nonSecurityCaseTypes)
		}
	}
	for _, v := range got {
		if v == securityReportCaseType {
			t.Fatalf("injected type filter names the restricted type: %v", got)
		}
	}
}

func TestScopeCaseSearchBody(t *testing.T) {
	cases := []struct {
		name       string
		in         string
		wantErr    error
		wantInject bool
		wantSame   bool
	}{
		{name: "empty object", in: `{}`, wantInject: true},
		{name: "null body", in: `null`, wantInject: true},
		{name: "legacy flat filters only", in: `{"filters":{"states":["open"]},"pagination":{"limit":10}}`, wantInject: true},
		{name: "null filters", in: `{"filters":null}`, wantInject: true},
		{name: "other predicates present", in: `{"filters":{"filters":[{"field":"state","op":"in","values":["open"]}]}}`, wantInject: true},
		{name: "type filter only inside anyOf branch", in: `{"filters":{"anyOf":[{"filters":[{"field":"type","op":"in","values":["case"]}]}]}}`, wantInject: true},
		{name: "top-level type filter naming other types", in: `{"filters":{"filters":[{"field":"type","op":"in","values":["case","engagement"]}]}}`, wantSame: true},
		{name: "top-level type names restricted", in: `{"filters":{"filters":[{"field":"type","op":"in","values":["case","security_report_analysis"]}]}}`, wantErr: errSecurityReportsRestricted},
		{name: "branch type names restricted", in: `{"filters":{"anyOf":[{"filters":[{"field":"type","op":"in","values":["security_report_analysis"]}]}]}}`, wantErr: errSecurityReportsRestricted},
		{name: "filters.filters not an array", in: `{"filters":{"filters":"x"}}`, wantErr: errBadShape},
		{name: "anyOf not an array", in: `{"filters":{"anyOf":{}}}`, wantErr: errBadShape},
		{name: "top-level body is an array", in: `[]`, wantErr: errBadShape},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := scopeCaseSearchBody([]byte(tc.in))
			switch {
			case tc.wantErr == errSecurityReportsRestricted:
				if err != errSecurityReportsRestricted {
					t.Fatalf("err = %v, want errSecurityReportsRestricted", err)
				}
				return
			case tc.wantErr == errBadShape:
				if err == nil || err == errSecurityReportsRestricted {
					t.Fatalf("err = %v, want a decode error", err)
				}
				return
			case err != nil:
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantSame && string(out) != tc.in {
				t.Fatalf("body rewritten: %s", out)
			}
			if tc.wantInject {
				assertScopedToNonSecurityTypes(t, out)
			}
		})
	}

	t.Run("carries every other field through untouched", func(t *testing.T) {
		in := `{"filters":{"states":["open"],"anyOf":[{"filters":[{"field":"severity","op":"in","values":["S1"]}]}],"filters":[{"field":"state","op":"in","values":["open"]}]},"pagination":{"limit":10,"offset":20},"sortBy":{"field":"createdOn","order":"desc"},"searchQuery":"abc 12345678901234567890"}`
		out, err := scopeCaseSearchBody([]byte(in))
		if err != nil {
			t.Fatal(err)
		}
		var got map[string]json.RawMessage
		if err := json.Unmarshal(out, &got); err != nil {
			t.Fatal(err)
		}
		if string(got["pagination"]) != `{"limit":10,"offset":20}` || string(got["sortBy"]) != `{"field":"createdOn","order":"desc"}` || string(got["searchQuery"]) != `"abc 12345678901234567890"` {
			t.Fatalf("sibling fields rewritten: %s", out)
		}
		var filters map[string]json.RawMessage
		if err := json.Unmarshal(got["filters"], &filters); err != nil {
			t.Fatal(err)
		}
		if string(filters["states"]) != `["open"]` || string(filters["anyOf"]) != `[{"filters":[{"field":"severity","op":"in","values":["S1"]}]}]` {
			t.Fatalf("filter siblings rewritten: %s", got["filters"])
		}
		var top []json.RawMessage
		if err := json.Unmarshal(filters["filters"], &top); err != nil {
			t.Fatal(err)
		}
		if len(top) != 2 || string(top[0]) != `{"field":"state","op":"in","values":["open"]}` {
			t.Fatalf("existing predicates not preserved in order: %s", filters["filters"])
		}
	})
}

// errBadShape is a marker for the table above: any non-nil error that is not
// errSecurityReportsRestricted.
var errBadShape = context.Canceled

func TestSearchCasesSecurityCenterScoping(t *testing.T) {
	t.Run("viewer without a type filter gets the non-security allow-list injected", func(t *testing.T) {
		var captured []byte
		h := guardedCaseHandler(&mockEntityCaseClient{searchCasesFn: func(_ context.Context, body []byte) ([]byte, error) {
			captured = body
			return []byte(`{"cases":[],"total":0}`), nil
		}})
		r := withViewerUser(httptest.NewRequest(http.MethodPost, "/cases/search", strings.NewReader(`{"filters":{"states":["open"]},"pagination":{"limit":10,"offset":0}}`)))
		w := httptest.NewRecorder()
		h.SearchCases(w, r)
		assertStatus(t, w, http.StatusOK)
		assertScopedToNonSecurityTypes(t, captured)
	})

	t.Run("handler without a guard fails closed and injects too", func(t *testing.T) {
		var captured []byte
		h := NewCaseHandler(&mockEntityCaseClient{searchCasesFn: func(_ context.Context, body []byte) ([]byte, error) {
			captured = body
			return []byte(`{"cases":[],"total":0}`), nil
		}})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/search", strings.NewReader(`{}`)))
		w := httptest.NewRecorder()
		h.SearchCases(w, r)
		assertStatus(t, w, http.StatusOK)
		assertScopedToNonSecurityTypes(t, captured)
	})

	t.Run("viewer with an explicit non-security type filter is forwarded verbatim", func(t *testing.T) {
		var captured []byte
		h := guardedCaseHandler(&mockEntityCaseClient{searchCasesFn: func(_ context.Context, body []byte) ([]byte, error) {
			captured = body
			return []byte(`{"cases":[],"total":0}`), nil
		}})
		const reqBody = `{"filters":{"filters":[{"field":"type","op":"in","values":["engagement"]}]}}`
		r := withViewerUser(httptest.NewRequest(http.MethodPost, "/cases/search", strings.NewReader(reqBody)))
		w := httptest.NewRecorder()
		h.SearchCases(w, r)
		assertStatus(t, w, http.StatusOK)
		if string(captured) != reqBody {
			t.Fatalf("body rewritten: %s", captured)
		}
	})

	t.Run("viewer with a malformed filters array gets 400 without an upstream call", func(t *testing.T) {
		called := false
		h := guardedCaseHandler(&mockEntityCaseClient{searchCasesFn: func(context.Context, []byte) ([]byte, error) {
			called = true
			return []byte(`{}`), nil
		}})
		r := withViewerUser(httptest.NewRequest(http.MethodPost, "/cases/search", strings.NewReader(`{"filters":{"filters":{"field":"type"}}}`)))
		w := httptest.NewRecorder()
		h.SearchCases(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		if called {
			t.Fatal("upstream must not be called")
		}
	})

	t.Run("cs engineer body is never rewritten", func(t *testing.T) {
		var captured []byte
		h := guardedCaseHandler(&mockEntityCaseClient{searchCasesFn: func(_ context.Context, body []byte) ([]byte, error) {
			captured = body
			return []byte(`{"cases":[],"total":0}`), nil
		}})
		const reqBody = `{"filters":{"filters":{"not":"an array"}}}`
		r := withCsEngineerUser(httptest.NewRequest(http.MethodPost, "/cases/search", strings.NewReader(reqBody)))
		w := httptest.NewRecorder()
		h.SearchCases(w, r)
		assertStatus(t, w, http.StatusOK)
		if string(captured) != reqBody {
			t.Fatalf("holder body rewritten: %s", captured)
		}
	})
}

func TestAggregateCasesSecurityCenterScoping(t *testing.T) {
	t.Run("viewer aggregate gets the allow-list injected", func(t *testing.T) {
		var captured []byte
		h := guardedCaseHandler(&mockEntityCaseClient{aggregateCasesFn: func(_ context.Context, body []byte) ([]byte, error) {
			captured = body
			return []byte(`{"groups":[],"othersCount":0,"totalRecords":0}`), nil
		}})
		r := withViewerUser(httptest.NewRequest(http.MethodPost, "/cases/aggregate", strings.NewReader(`{"filters":{},"groupBy":"type","maxGroups":12}`)))
		w := httptest.NewRecorder()
		h.AggregateCases(w, r)
		assertStatus(t, w, http.StatusOK)
		assertScopedToNonSecurityTypes(t, captured)
		var req map[string]json.RawMessage
		_ = json.Unmarshal(captured, &req)
		if string(req["groupBy"]) != `"type"` || string(req["maxGroups"]) != `12` {
			t.Fatalf("aggregate fields rewritten: %s", captured)
		}
	})

	t.Run("viewer aggregate naming the restricted type is refused", func(t *testing.T) {
		called := false
		h := guardedCaseHandler(&mockEntityCaseClient{aggregateCasesFn: func(context.Context, []byte) ([]byte, error) {
			called = true
			return []byte(`{}`), nil
		}})
		r := withViewerUser(httptest.NewRequest(http.MethodPost, "/cases/aggregate", strings.NewReader(`{"filters":{"filters":[{"field":"type","op":"in","values":["security_report_analysis"]}]},"groupBy":"state"}`)))
		w := httptest.NewRecorder()
		h.AggregateCases(w, r)
		assertStatus(t, w, http.StatusForbidden)
		assertErrorMessage(t, w, ErrMsgForbidden)
		if called {
			t.Fatal("upstream must not be called")
		}
	})
}

func TestGetCaseSecurityCenterScoping(t *testing.T) {
	const caseID = "11111111-1111-1111-1111-111111111111"
	securityCase := `{"id":"` + caseID + `","type":"security_report_analysis","state":"open"}`
	plainCase := `{"id":"` + caseID + `","type":"case","state":"open"}`

	t.Run("viewer gets 403 for a security-report case", func(t *testing.T) {
		h := guardedCaseHandler(&mockEntityCaseClient{getCaseFn: func(context.Context, string) ([]byte, error) { return []byte(securityCase), nil }})
		r := withViewerUser(httptest.NewRequest(http.MethodGet, "/cases/"+caseID, nil))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.GetCase(w, r)
		assertStatus(t, w, http.StatusForbidden)
		assertErrorMessage(t, w, ErrMsgForbidden)
	})

	t.Run("viewer still reads an ordinary case", func(t *testing.T) {
		h := guardedCaseHandler(&mockEntityCaseClient{getCaseFn: func(context.Context, string) ([]byte, error) { return []byte(plainCase), nil }})
		r := withViewerUser(httptest.NewRequest(http.MethodGet, "/cases/"+caseID, nil))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.GetCase(w, r)
		assertStatus(t, w, http.StatusOK)
	})

	t.Run("cs engineer reads a security-report case", func(t *testing.T) {
		h := guardedCaseHandler(&mockEntityCaseClient{getCaseFn: func(context.Context, string) ([]byte, error) { return []byte(securityCase), nil }})
		r := withCsEngineerUser(httptest.NewRequest(http.MethodGet, "/cases/"+caseID, nil))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.GetCase(w, r)
		assertStatus(t, w, http.StatusOK)
	})
}

func TestCaseSubResourceSecurityCenterScoping(t *testing.T) {
	const caseID = "11111111-1111-1111-1111-111111111111"
	securityCase := `{"id":"` + caseID + `","type":"security_report_analysis"}`
	plainCase := `{"id":"` + caseID + `","type":"case"}`

	type subRoute struct {
		name string
		call func(h *CaseHandler, r *http.Request, w http.ResponseWriter)
		req  func() *http.Request
	}
	routes := []subRoute{
		{
			name: "comments search",
			call: func(h *CaseHandler, r *http.Request, w http.ResponseWriter) { h.SearchCaseComments(w, r) },
			req: func() *http.Request {
				r := httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/comments/search", strings.NewReader(`{}`))
				r.SetPathValue("id", caseID)
				return r
			},
		},
		{
			name: "activities search",
			call: func(h *CaseHandler, r *http.Request, w http.ResponseWriter) { h.SearchCaseActivities(w, r) },
			req: func() *http.Request {
				r := httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/activities/search", strings.NewReader(`{}`))
				r.SetPathValue("id", caseID)
				return r
			},
		},
		{
			name: "escalations list",
			call: func(h *CaseHandler, r *http.Request, w http.ResponseWriter) { h.GetCaseEscalations(w, r) },
			req: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/cases/"+caseID+"/escalations", nil)
				r.SetPathValue("id", caseID)
				return r
			},
		},
	}

	newClient := func(caseBody string, caseErr error, reached *bool) *mockEntityCaseClient {
		return &mockEntityCaseClient{
			getCaseFn: func(context.Context, string) ([]byte, error) {
				if caseErr != nil {
					return nil, caseErr
				}
				return []byte(caseBody), nil
			},
			searchCommentsFn: func(context.Context, []byte) ([]byte, error) { *reached = true; return []byte(`{"comments":[]}`), nil },
			searchCaseActivitiesFn: func(context.Context, string, []byte) ([]byte, error) {
				*reached = true
				return []byte(`{"activities":[]}`), nil
			},
			searchCaseEscalationsFn: func(context.Context, string) ([]byte, error) { *reached = true; return []byte(`[]`), nil },
		}
	}

	for _, rt := range routes {
		t.Run(rt.name, func(t *testing.T) {
			t.Run("viewer is refused for a security-report case", func(t *testing.T) {
				reached := false
				h := guardedCaseHandler(newClient(securityCase, nil, &reached))
				w := httptest.NewRecorder()
				rt.call(h, withViewerUser(rt.req()), w)
				assertStatus(t, w, http.StatusForbidden)
				if reached {
					t.Fatal("sub-resource upstream call must not run")
				}
			})
			t.Run("viewer reads an ordinary case's sub-resource", func(t *testing.T) {
				reached := false
				h := guardedCaseHandler(newClient(plainCase, nil, &reached))
				w := httptest.NewRecorder()
				rt.call(h, withViewerUser(rt.req()), w)
				assertStatus(t, w, http.StatusOK)
				if !reached {
					t.Fatal("sub-resource upstream call expected")
				}
			})
			t.Run("an unknown case is still 404 for a viewer", func(t *testing.T) {
				reached := false
				h := guardedCaseHandler(newClient("", &apierror.Error{StatusCode: http.StatusNotFound}, &reached))
				w := httptest.NewRecorder()
				rt.call(h, withViewerUser(rt.req()), w)
				assertStatus(t, w, http.StatusNotFound)
				assertErrorMessage(t, w, ErrMsgNotFound)
			})
			t.Run("cs engineer skips the case load", func(t *testing.T) {
				reached := false
				client := newClient(securityCase, nil, &reached)
				loaded := false
				client.getCaseFn = func(context.Context, string) ([]byte, error) { loaded = true; return []byte(securityCase), nil }
				h := guardedCaseHandler(client)
				w := httptest.NewRecorder()
				rt.call(h, withCsEngineerUser(rt.req()), w)
				assertStatus(t, w, http.StatusOK)
				if loaded {
					t.Fatal("holder must not pay for a case load")
				}
			})
		})
	}
}
