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
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/entity"
)

const (
	skillTestID  = "11111111-1111-4111-8111-111111111111"
	skillTestLvl = "22222222-2222-4222-8222-222222222222"
)

// skillUpstream is a stub entity-service: it records the last skills call and
// answers with the configured status and body.
type skillUpstream struct {
	method, path, rawQuery, body, userToken, correlation, contentType string
	status                                                            int
	respBody                                                          string
}

func newSkillStack(t *testing.T) (*SkillHandler, *skillUpstream) {
	t.Helper()
	up := &skillUpstream{status: http.StatusOK, respBody: `{}`}
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`))
	})
	mux.HandleFunc("/skills", up.serve)
	mux.HandleFunc("/skills/", up.serve)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := entity.NewCustomerEntityClient(entity.CustomerEntityConfig{
		BaseURL: srv.URL, TokenURL: srv.URL + "/token", ClientID: "test-client", ClientSecret: "test-secret",
	})
	return NewSkillHandler(client), up
}

func (u *skillUpstream) serve(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	u.method, u.path, u.rawQuery, u.body = r.Method, r.URL.Path, r.URL.RawQuery, string(b)
	u.userToken = r.Header.Get("x-user-id-token")
	u.correlation = r.Header.Get("X-CSM-Correlation-ID")
	u.contentType = r.Header.Get("Content-Type")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(u.status)
	if u.status != http.StatusNoContent {
		_, _ = w.Write([]byte(u.respBody))
	}
}

// skillMux registers the same patterns as cmd/server/main.go; ServeMux panics
// on a conflicting pair, so building it proves the literal and wildcard
// /skills routes can coexist.
func skillMux(h *SkillHandler) *http.ServeMux {
	m := http.NewServeMux()
	m.HandleFunc("GET /skills/levels", h.GetSkillLevels)
	m.HandleFunc("GET /skills/categories", h.GetSkillCategories)
	m.HandleFunc("POST /skills/search", h.SearchSkills)
	m.HandleFunc("GET /skills/me", h.GetMySkills)
	m.HandleFunc("GET /skills/me/history", h.GetMySkillHistory)
	m.HandleFunc("POST /skills/users/search", h.SearchUserSkills)
	m.HandleFunc("GET /skills/{id}", h.GetSkill)
	m.HandleFunc("POST /skills", h.CreateSkill)
	m.HandleFunc("PATCH /skills/{id}", h.UpdateSkill)
	m.HandleFunc("POST /skills/me", h.AddMySkill)
	m.HandleFunc("PUT /skills/me/{skillId}", h.SetMySkillLevel)
	m.HandleFunc("DELETE /skills/me/{skillId}", h.RemoveMySkill)
	return m
}

func TestSkills_EachRouteHitsTheRightUpstreamCall(t *testing.T) {
	h, up := newSkillStack(t)
	mux := skillMux(h)
	cases := []struct {
		name, method, target, body string
		upstreamStatus             int
		wantStatus                 int
		wantPath, wantQuery        string
		wantBody                   string
	}{
		{"levels", "GET", "/skills/levels", "", 200, 200, "/skills/levels", "", ""},
		{"categories", "GET", "/skills/categories", "", 200, 200, "/skills/categories", "", ""},
		{"search", "POST", "/skills/search", `{"searchQuery":"go","pagination":{"limit":10,"offset":0}}`, 200, 200, "/skills/search", "", `{"searchQuery":"go","pagination":{"limit":10,"offset":0}}`},
		{"get", "GET", "/skills/" + skillTestID, "", 200, 200, "/skills/" + skillTestID, "", ""},
		{"create", "POST", "/skills", `{"name":"Go","levelTypeId":"` + skillTestLvl + `"}`, 201, 201, "/skills", "", `{"name":"Go","levelTypeId":"` + skillTestLvl + `"}`},
		{"update", "PATCH", "/skills/" + skillTestID, `{"active":false}`, 200, 200, "/skills/" + skillTestID, "", `{"active":false}`},
		{"mine", "GET", "/skills/me", "", 200, 200, "/skills/me", "", ""},
		{"add mine", "POST", "/skills/me", `{"skillId":"` + skillTestID + `","levelId":"` + skillTestLvl + `"}`, 201, 201, "/skills/me", "", `{"skillId":"` + skillTestID + `","levelId":"` + skillTestLvl + `"}`},
		{"set mine", "PUT", "/skills/me/" + skillTestID, `{"levelId":"` + skillTestLvl + `"}`, 200, 200, "/skills/me/" + skillTestID, "", `{"levelId":"` + skillTestLvl + `"}`},
		{"remove mine", "DELETE", "/skills/me/" + skillTestID, "", 204, 204, "/skills/me/" + skillTestID, "", ""},
		{"history", "GET", "/skills/me/history?skillId=" + skillTestID + "&limit=5&offset=10", "", 200, 200, "/skills/me/history", "limit=5&offset=10&skillId=" + skillTestID, ""},
		{"history bare", "GET", "/skills/me/history", "", 200, 200, "/skills/me/history", "", ""},
		{"user search", "POST", "/skills/users/search", `{"teamKeys":["apollo"],"minLevelValue":50}`, 200, 200, "/skills/users/search", "", `{"teamKeys":["apollo"],"minLevelValue":50}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			*up = skillUpstream{status: c.upstreamStatus, respBody: `{"ok":true}`}
			var body io.Reader
			if c.body != "" {
				body = strings.NewReader(c.body)
			}
			r := httptest.NewRequest(c.method, c.target, body)
			r = withUser(r.WithContext(entity.WithUserIDToken(r.Context(), "caller-token")))
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)

			if w.Code != c.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", w.Code, c.wantStatus, w.Body.String())
			}
			if up.method != c.method || up.path != c.wantPath || up.rawQuery != c.wantQuery {
				t.Errorf("upstream = %s %s?%s, want %s %s?%s", up.method, up.path, up.rawQuery, c.method, c.wantPath, c.wantQuery)
			}
			if up.body != c.wantBody {
				t.Errorf("upstream body = %q, want %q", up.body, c.wantBody)
			}
			if up.userToken != "caller-token" {
				t.Errorf("x-user-id-token = %q, want the caller's token forwarded", up.userToken)
			}
			if c.wantStatus != 204 && w.Body.String() != `{"ok":true}` {
				t.Errorf("response body = %q, want upstream's unchanged", w.Body.String())
			}
		})
	}
}

func TestSkills_UpstreamRefusalKeepsItsMessage(t *testing.T) {
	h, up := newSkillStack(t)
	mux := skillMux(h)
	for _, status := range []int{400, 403, 404, 409} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			*up = skillUpstream{status: status, respBody: `{"code":` + strconv.Itoa(status) + `,"message":"Reason ` + http.StatusText(status) + ` for the caller.","errorCode":"skill_reason"}`}
			r := withUser(httptest.NewRequest("POST", "/skills", strings.NewReader(`{"name":"Go"}`)))
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)
			if w.Code != status {
				t.Fatalf("status = %d, want %d", w.Code, status)
			}
			if !strings.Contains(w.Body.String(), "Reason "+http.StatusText(status)+" for the caller.") {
				t.Errorf("body %q lost the upstream message", w.Body.String())
			}
			if !strings.Contains(w.Body.String(), `"errorCode":"skill_reason"`) {
				t.Errorf("body %q lost the upstream errorCode", w.Body.String())
			}
		})
	}
}

func TestSkills_UpstreamServerErrorStaysGeneric(t *testing.T) {
	h, up := newSkillStack(t)
	*up = skillUpstream{status: 500, respBody: `{"code":500,"message":"pq: relation \"skill\" broke"}`}
	w := httptest.NewRecorder()
	skillMux(h).ServeHTTP(w, withUser(httptest.NewRequest("GET", "/skills/me", nil)))
	if w.Code != 500 || strings.Contains(w.Body.String(), "pq:") {
		t.Fatalf("got %d %q, want a generic 500", w.Code, w.Body.String())
	}
}

func TestSkills_RejectsBadInputWithoutCallingUpstream(t *testing.T) {
	h, up := newSkillStack(t)
	mux := skillMux(h)
	cases := []struct{ name, method, target, body string }{
		{"bad id on get", "GET", "/skills/not-a-uuid", ""},
		{"bad id on patch", "PATCH", "/skills/not-a-uuid", `{}`},
		{"bad id on put", "PUT", "/skills/me/not-a-uuid", `{}`},
		{"bad id on delete", "DELETE", "/skills/me/not-a-uuid", ""},
		{"bad history skill id", "GET", "/skills/me/history?skillId=nope", ""},
		{"malformed json", "POST", "/skills/search", `{not json`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			*up = skillUpstream{status: 200, respBody: `{}`}
			var body io.Reader
			if c.body != "" {
				body = strings.NewReader(c.body)
			}
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, withUser(httptest.NewRequest(c.method, c.target, body)))
			if w.Code != 400 {
				t.Errorf("status = %d, want 400", w.Code)
			}
			if up.method != "" {
				t.Errorf("upstream was called (%s %s)", up.method, up.path)
			}
		})
	}
}

func TestSkills_NoUserIs401(t *testing.T) {
	h, up := newSkillStack(t)
	w := httptest.NewRecorder()
	skillMux(h).ServeHTTP(w, httptest.NewRequest("GET", "/skills/me", nil))
	if w.Code != 401 || up.method != "" {
		t.Fatalf("got %d (upstream called: %q), want 401 and no call", w.Code, up.method)
	}
}

func TestSkills_ManagementRoutesHitTheRightUpstreamCall(t *testing.T) {
	h, up := newSkillStack(t)
	m := http.NewServeMux()
	m.HandleFunc("GET /skills/me/capabilities", h.GetMySkillCapabilities)
	m.HandleFunc("POST /skills/categories", h.CreateSkillCategory)
	m.HandleFunc("PATCH /skills/categories/{id}", h.UpdateSkillCategory)
	m.HandleFunc("POST /skills/requests/check", h.CheckSkillRequest)
	m.HandleFunc("POST /skills/requests", h.CreateSkillRequest)
	m.HandleFunc("GET /skills/requests/mine", h.ListMySkillRequests)
	m.HandleFunc("POST /skills/requests/search", h.SearchSkillRequests)
	m.HandleFunc("POST /skills/requests/{id}/withdraw", h.WithdrawSkillRequest)
	m.HandleFunc("POST /skills/requests/{id}/approve", h.ApproveSkillRequest)
	m.HandleFunc("POST /skills/requests/{id}/reject", h.RejectSkillRequest)
	m.HandleFunc("POST /skills/directory/search", h.SearchSkillDirectory)
	m.HandleFunc("GET /skills/users/{userId}", h.GetUserSkillProfile)
	m.HandleFunc("GET /skills/users/{userId}/history", h.GetUserSkillHistory)

	cases := []struct {
		method, target, body, wantPath, wantQuery string
		upstream, want                            int
	}{
		{"GET", "/skills/me/capabilities", "", "/skills/me/capabilities", "", 200, 200},
		{"POST", "/skills/categories", `{"name":"Cloud"}`, "/skills/categories", "", 201, 201},
		{"PATCH", "/skills/categories/" + skillTestID, `{"active":false}`, "/skills/categories/" + skillTestID, "", 200, 200},
		{"POST", "/skills/requests/check", `{"name":"go"}`, "/skills/requests/check", "", 200, 200},
		{"POST", "/skills/requests", `{"name":"Go","levelId":"` + skillTestLvl + `"}`, "/skills/requests", "", 201, 201},
		{"GET", "/skills/requests/mine", "", "/skills/requests/mine", "", 200, 200},
		{"POST", "/skills/requests/search", `{}`, "/skills/requests/search", "", 200, 200},
		{"POST", "/skills/requests/" + skillTestID + "/withdraw", "", "/skills/requests/" + skillTestID + "/withdraw", "", 200, 200},
		{"POST", "/skills/requests/" + skillTestID + "/approve", `{"mode":"create"}`, "/skills/requests/" + skillTestID + "/approve", "", 200, 200},
		{"POST", "/skills/requests/" + skillTestID + "/reject", `{"reason":"no"}`, "/skills/requests/" + skillTestID + "/reject", "", 409, 409},
		{"POST", "/skills/directory/search", `{"match":"ALL"}`, "/skills/directory/search", "", 403, 403},
		{"GET", "/skills/users/" + skillTestID, "", "/skills/users/" + skillTestID, "", 200, 200},
		{"GET", "/skills/users/" + skillTestID + "/history?skillId=" + skillTestLvl + "&limit=5", "", "/skills/users/" + skillTestID + "/history", "limit=5&skillId=" + skillTestLvl, 200, 200},
	}
	for _, c := range cases {
		up.method, up.path, up.rawQuery, up.body = "", "", "", ""
		up.status, up.respBody = c.upstream, `{"message":"upstream says no"}`
		if c.upstream < 400 {
			up.respBody = `{}`
		}
		rec := httptest.NewRecorder()
		var body io.Reader
		if c.body != "" {
			body = strings.NewReader(c.body)
		}
		m.ServeHTTP(rec, withUser(httptest.NewRequest(c.method, c.target, body)))
		if rec.Code != c.want {
			t.Fatalf("%s %s: status %d, want %d (%s)", c.method, c.target, rec.Code, c.want, rec.Body.String())
		}
		if up.path != c.wantPath || up.rawQuery != c.wantQuery || up.method != c.method {
			t.Fatalf("%s %s reached %s %s?%s", c.method, c.target, up.method, up.path, up.rawQuery)
		}
		if c.body != "" && up.body != c.body {
			t.Fatalf("%s %s forwarded body %q", c.method, c.target, up.body)
		}
		if c.upstream == 409 && !strings.Contains(rec.Body.String(), "upstream says no") {
			t.Fatalf("a 409's reason was not kept: %s", rec.Body.String())
		}
	}
	// A malformed id never reaches the entity service.
	up.path = ""
	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, withUser(httptest.NewRequest("GET", "/skills/users/not-a-uuid/history", nil)))
	if rec.Code != 400 || up.path != "" {
		t.Fatalf("malformed id: %d, upstream %q", rec.Code, up.path)
	}
}
