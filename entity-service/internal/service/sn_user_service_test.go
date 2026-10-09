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
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// testCallerSysid is the caller's sys_id used across the GetMe membership tests.
var testCallerSysid = sysid32('9')

// snUserMeJSON is a minimal upstream GET /users/me payload for the given
// caller sys_id and userType.
func snUserMeJSON(id, userType string) string {
	return `{
		"id": "` + id + `",
		"email": "agent@example.com",
		"lastName": "Agent",
		"userType": "` + userType + `",
		"roles": ["wso2_agent"]
	}`
}

// membershipsJSON builds a group-members/search response body with the given
// groupName as the caller's single membership, or an empty memberships list
// if groupName is "".
func membershipsJSON(userID, groupName string) string {
	if groupName == "" {
		return `{"memberships": [], "totalRecords": 0}`
	}
	return `{"memberships": [{"userId": "` + userID + `", "groupId": "` + sysid32('c') +
		`", "groupName": "` + groupName + `"}], "totalRecords": 1}`
}

// TestSNUserService_GetMe_ReturnsCallerGroups is the shape the team registry's
// move depends on: this service reports the caller's raw group membership and
// says nothing about teams, because the registry that names them is the
// caller's configuration now.
//
// The membership query must therefore be unfiltered -- filtering it by group
// name would need exactly the registry this service no longer has.
func TestSNUserService_GetMe_ReturnsCallerGroups(t *testing.T) {
	var capturedBody []byte

	mux := http.NewServeMux()
	mux.HandleFunc("/users/me", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(snUserMeJSON(testCallerSysid, "internal")))
	})
	mux.HandleFunc("/group-members/search", func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(membershipsJSON(testCallerSysid, "Alpha Team")))
	})

	svc := NewServiceNowUserService(newTestSNClient(t, mux))

	got, err := svc.GetMe(contextWithUserIDToken("token"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Groups) != 1 {
		t.Fatalf("Groups = %+v, want the caller's one membership", got.Groups)
	}
	if got.Groups[0].Name != "Alpha Team" {
		t.Fatalf("Groups[0].Name = %q, want \"Alpha Team\"", got.Groups[0].Name)
	}
	// Ids are converted to this platform's UUID form, like every other id.
	if got.Groups[0].ID != sysidToUUID(sysid32('c')) {
		t.Fatalf("Groups[0].ID = %q, want the UUID form of the upstream group id", got.Groups[0].ID)
	}

	var reqBody struct {
		Filters struct {
			GroupNames []string `json:"groupNames"`
			GroupIDs   []string `json:"groupIds"`
			UserID     string   `json:"userId"`
		} `json:"filters"`
	}
	if err := json.Unmarshal(capturedBody, &reqBody); err != nil {
		t.Fatalf("unmarshal captured request body: %v", err)
	}
	if len(reqBody.Filters.GroupNames) != 0 || len(reqBody.Filters.GroupIDs) != 0 {
		t.Fatalf("membership query was narrowed (groupNames=%v groupIds=%v); it must ask for every group the caller is in",
			reqBody.Filters.GroupNames, reqBody.Filters.GroupIDs)
	}
	if reqBody.Filters.UserID != testCallerSysid {
		t.Fatalf("userId = %q, want %q", reqBody.Filters.UserID, testCallerSysid)
	}
}

// TestSNUserService_GetMe_NoMemberships: a caller in no group at all still gets
// their identity, with an empty (never null) groups list.
func TestSNUserService_GetMe_NoMemberships(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/users/me", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(snUserMeJSON(testCallerSysid, "internal")))
	})
	mux.HandleFunc("/group-members/search", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(membershipsJSON(testCallerSysid, "")))
	})

	svc := NewServiceNowUserService(newTestSNClient(t, mux))

	got, err := svc.GetMe(contextWithUserIDToken("token"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Groups == nil || len(got.Groups) != 0 {
		t.Fatalf("Groups = %+v, want an empty non-nil slice", got.Groups)
	}
}

// TestSNUserService_GetMe_GroupMembershipCallErrors_IdentityStillReturned
// verifies that a downstream failure on the group-members/search call never
// fails the overall /users/me response -- identity/roles must still come back,
// with groups simply empty.
func TestSNUserService_GetMe_GroupMembershipCallErrors_IdentityStillReturned(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/users/me", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(snUserMeJSON(testCallerSysid, "internal")))
	})
	mux.HandleFunc("/group-members/search", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	svc := NewServiceNowUserService(newTestSNClient(t, mux))

	got, err := svc.GetMe(contextWithUserIDToken("token"))
	if err != nil {
		t.Fatalf("unexpected error: %v (identity must still be returned)", err)
	}
	if got.Email != "agent@example.com" {
		t.Fatalf("Email = %q, want agent@example.com even though group lookup failed", got.Email)
	}
	if len(got.Groups) != 0 {
		t.Fatalf("Groups = %+v, want empty when the membership call errors", got.Groups)
	}
}

// TestSNUserService_GetMe_ExternalUser_SkipsGroupLookup verifies that GetMe
// never calls group-members/search for an external (customer) caller -- the
// upstream ACL rejects that lookup for them, exactly as it does on GetUser.
func TestSNUserService_GetMe_ExternalUser_SkipsGroupLookup(t *testing.T) {
	groupLookupCalled := false

	mux := http.NewServeMux()
	mux.HandleFunc("/users/me", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(snUserMeJSON(testCallerSysid, "external")))
	})
	mux.HandleFunc("/group-members/search", func(w http.ResponseWriter, _ *http.Request) {
		groupLookupCalled = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(membershipsJSON(testCallerSysid, "")))
	})

	svc := NewServiceNowUserService(newTestSNClient(t, mux))

	got, err := svc.GetMe(contextWithUserIDToken("token"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if groupLookupCalled {
		t.Fatalf("group-members/search was called for an external caller; it must be skipped")
	}
	if got.Groups == nil || len(got.Groups) != 0 {
		t.Fatalf("Groups = %+v, want an empty non-nil slice", got.Groups)
	}
	if got.UserType != domain.UserTypeExternal {
		t.Fatalf("UserType = %q, want %q", got.UserType, domain.UserTypeExternal)
	}
}

// TestSNUserService_GetMe_InternalUser_CallsGroupLookup verifies the existing
// behaviour is unchanged for internal (staff) callers: the group membership
// lookup still runs and populates Groups.
func TestSNUserService_GetMe_InternalUser_CallsGroupLookup(t *testing.T) {
	groupLookupCalled := false

	mux := http.NewServeMux()
	mux.HandleFunc("/users/me", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(snUserMeJSON(testCallerSysid, "internal")))
	})
	mux.HandleFunc("/group-members/search", func(w http.ResponseWriter, _ *http.Request) {
		groupLookupCalled = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(membershipsJSON(testCallerSysid, "Alpha Team")))
	})

	svc := NewServiceNowUserService(newTestSNClient(t, mux))

	got, err := svc.GetMe(contextWithUserIDToken("token"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !groupLookupCalled {
		t.Fatalf("group-members/search was not called for an internal caller; it must still run")
	}
	if len(got.Groups) != 1 || got.Groups[0].Name != "Alpha Team" {
		t.Fatalf("Groups = %+v, want the caller's one membership", got.Groups)
	}
	if got.UserType != domain.UserTypeInternal {
		t.Fatalf("UserType = %q, want %q", got.UserType, domain.UserTypeInternal)
	}
}

// snUserSearchJSON builds a minimal upstream POST /users/search response with
// a single user of the given sys_id and userType.
func snUserSearchJSON(id, userType string) string {
	return `{
		"users": [{
			"id": "` + id + `",
			"userName": "customer1",
			"name": "Customer One",
			"email": "customer@example.com",
			"userType": "` + userType + `",
			"active": true,
			"lockedOut": false,
			"createdOn": "2026-01-01 00:00:00",
			"updatedOn": "2026-01-01 00:00:00",
			"roles": []
		}],
		"totalRecords": 1,
		"offset": 0,
		"limit": 1
	}`
}

// TestSNUserService_GetUser_ExternalUser_SkipsGroupLookup verifies that GetUser
// never calls group-members/search for an external (customer) user -- the
// upstream ACL rejects that lookup for them, so making it wastes a call and
// logs a spurious failure on every request.
func TestSNUserService_GetUser_ExternalUser_SkipsGroupLookup(t *testing.T) {
	targetSysid := sysid32('5')
	groupLookupCalled := false

	mux := http.NewServeMux()
	mux.HandleFunc("/users/search", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(snUserSearchJSON(targetSysid, "external")))
	})
	mux.HandleFunc("/group-members/search", func(w http.ResponseWriter, _ *http.Request) {
		groupLookupCalled = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(membershipsJSON(targetSysid, "")))
	})
	mux.HandleFunc("/project-contacts/search", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"contacts": [], "totalRecords": 0}`))
	})

	svc := NewServiceNowUserService(newTestSNClient(t, mux))

	got, err := svc.GetUser(contextWithUserIDToken("token"), sysidToUUID(targetSysid))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if groupLookupCalled {
		t.Fatalf("group-members/search was called for an external user; it must be skipped")
	}
	if got.Groups == nil || len(got.Groups) != 0 {
		t.Fatalf("Groups = %+v, want an empty non-nil slice", got.Groups)
	}
}

// TestSNUserService_GetUser_InternalUser_CallsGroupLookup verifies the
// existing behaviour is unchanged for internal (staff) users: the group
// membership lookup still runs and populates Groups.
func TestSNUserService_GetUser_InternalUser_CallsGroupLookup(t *testing.T) {
	targetSysid := sysid32('6')
	groupLookupCalled := false

	mux := http.NewServeMux()
	mux.HandleFunc("/users/search", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(snUserSearchJSON(targetSysid, "internal")))
	})
	mux.HandleFunc("/group-members/search", func(w http.ResponseWriter, _ *http.Request) {
		groupLookupCalled = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(membershipsJSON(targetSysid, "Alpha Team")))
	})

	svc := NewServiceNowUserService(newTestSNClient(t, mux))

	got, err := svc.GetUser(contextWithUserIDToken("token"), sysidToUUID(targetSysid))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !groupLookupCalled {
		t.Fatalf("group-members/search was not called for an internal user; it must still run")
	}
	if len(got.Groups) != 1 || got.Groups[0].Name != "Alpha Team" {
		t.Fatalf("Groups = %+v, want the caller's one membership", got.Groups)
	}
}

// TestGetUserMeResponse_GroupsFieldShape locks in the exact field names the
// portal backend decodes to resolve the caller's team. A rename here silently
// costs every caller their team.
func TestGetUserMeResponse_GroupsFieldShape(t *testing.T) {
	resp := domain.GetUserMeResponse{
		ID:     "u1",
		Email:  "agent@example.com",
		Roles:  []string{},
		Groups: []domain.UserGroupRef{{ID: "g1", Name: "Alpha Team"}},
	}
	encoded, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if want := `"groups":[{"id":"g1","name":"Alpha Team"}]`; !strings.Contains(string(encoded), want) {
		t.Fatalf("encoded = %s, want it to contain %s", encoded, want)
	}
}

// TestSNUserService_PatchMe_PhoneIgnored: phone is accepted but not stored on
// this data source. A phone-only request is a no-op 200 that never reaches the
// upstream; with timeZone also present, only timeZone is forwarded.
func TestSNUserService_PatchMe_PhoneIgnored(t *testing.T) {
	phone := "+15555550123"

	t.Run("phone only is a no-op", func(t *testing.T) {
		calls := 0
		mux := http.NewServeMux()
		mux.HandleFunc("/users/me", func(w http.ResponseWriter, _ *http.Request) { calls++ })
		svc := NewServiceNowUserService(newTestSNClient(t, mux))

		resp, err := svc.PatchMe(contextWithUserIDToken("token"), domain.PatchUserMeRequest{Phone: &phone})
		if err != nil {
			t.Fatalf("PatchMe returned error: %v", err)
		}
		if resp.Message == "" || resp.User.ID != "" {
			t.Errorf("response = %+v, want a message and an empty user", resp)
		}
		if calls != 0 {
			t.Errorf("upstream called %d times, want 0", calls)
		}
	})

	t.Run("timeZone with phone applies timeZone only", func(t *testing.T) {
		var body []byte
		mux := http.NewServeMux()
		mux.HandleFunc("/users/me", func(w http.ResponseWriter, r *http.Request) {
			body, _ = io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"message":"ok","user":{"id":"` + testCallerSysid + `","updatedBy":"a","updatedOn":"b"}}`))
		})
		svc := NewServiceNowUserService(newTestSNClient(t, mux))

		resp, err := svc.PatchMe(contextWithUserIDToken("token"), domain.PatchUserMeRequest{TimeZone: "Asia/Colombo", Phone: &phone})
		if err != nil {
			t.Fatalf("PatchMe returned error: %v", err)
		}
		if resp.Message != "ok" {
			t.Errorf("Message = %q, want ok", resp.Message)
		}
		if strings.Contains(string(body), "phone") || !strings.Contains(string(body), "Asia/Colombo") {
			t.Errorf("forwarded body = %s, want timeZone only", body)
		}
	})

	t.Run("empty request still rejected", func(t *testing.T) {
		svc := NewServiceNowUserService(newTestSNClient(t, http.NewServeMux()))
		if _, err := svc.PatchMe(contextWithUserIDToken("token"), domain.PatchUserMeRequest{}); err == nil {
			t.Fatal("want a validation error")
		}
	})
}
