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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The author sysid and its canonical UUID form. Every id this service emits is a
// UUID, so the raw 32-hex form must never reach the response: a consumer takes
// the id straight to GET /users/{id}.
const (
	testAuthorSysid = "92f8bc293b39cf503e1e088aa4e45a5f"
	testAuthorUUID  = "92f8bc29-3b39-cf50-3e1e-088aa4e45a5f"
)

func userRefID(t *testing.T, ref *domain.UserReference) string {
	t.Helper()
	if ref == nil {
		t.Fatal("user reference is nil, want a populated reference")
	}
	if ref.ID == nil {
		return ""
	}
	return *ref.ID
}

// TestCommentUserReferenceFromUpstreamID covers the one site where the backing
// data source hands over a real user id, and pins the conversion: the raw sysid
// must come back out in UUID form, not verbatim.
func TestCommentUserReferenceFromUpstreamID(t *testing.T) {
	t.Parallel()

	body := `{"comments":[{
		"id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"referenceId": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"content": "a comment",
		"type": "comments",
		"createdOn": "2026-01-01 10:00:00",
		"createdBy": "jane.doe@example.com",
		"createdByFirstName": "Jane",
		"createdByLastName": "Doe",
		"createdByFullName": "Jane Doe",
		"createdByUser": {"id": "` + testAuthorSysid + `", "email": "jane.doe@example.com", "name": "Jane Doe"}
	}],"totalRecords":1}`

	client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})

	resp, err := NewServiceNowCommentService(client).SearchComments(contextWithUserIDToken("token"), domain.SearchCommentsRequest{
		ReferenceID:   sysidToUUID("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"),
		ReferenceType: domain.ReferenceTypeConversation,
		Pagination:    domain.Pagination{Limit: 10},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Comments) != 1 {
		t.Fatalf("got %d comments, want 1", len(resp.Comments))
	}
	got := resp.Comments[0].CreatedBy
	if id := userRefID(t, got); id != testAuthorUUID {
		t.Errorf("createdBy.id = %q, want the converted UUID %q", id, testAuthorUUID)
	}
	if got.Email != "jane.doe@example.com" || got.Name != "Jane Doe" {
		t.Errorf("createdBy = %+v, want email/name of the author", got)
	}
}

// TestCaseCommentUserReferenceUpstreamVariants pins the three upstream shapes on
// the case-comment path: the user object present, explicitly null, and absent
// altogether. The last one is the backward-compatibility guarantee — this
// service must be deployable before the data source grows the field.
func TestCaseCommentUserReferenceUpstreamVariants(t *testing.T) {
	t.Parallel()

	const caseSysid = "cccccccccccccccccccccccccccccccc"

	comment := func(extra string) string {
		return `{"comments":[{
			"id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"referenceId": "` + caseSysid + `",
			"content": "a comment",
			"type": "comments",
			"createdOn": "2026-01-01 10:00:00",
			"createdBy": "jane.doe@example.com",
			"createdByFirstName": "Jane",
			"createdByLastName": "Doe",
			"createdByFullName": "Jane Doe"` + extra + `
		}],"totalRecords":1}`
	}

	tests := []struct {
		name      string
		body      string
		wantID    string
		wantEmail string
		wantName  string
	}{
		{
			name:      "user object supplied",
			body:      comment(`,"createdByUser": {"id": "` + testAuthorSysid + `", "email": "jane.doe@example.com", "name": "Jane Doe"}`),
			wantID:    testAuthorUUID,
			wantEmail: "jane.doe@example.com",
			wantName:  "Jane Doe",
		},
		{
			// A non-user author (an automation account) resolves to no user
			// record upstream. The reference is still emitted so a consumer can
			// render the author; only the id is null.
			name:      "user object explicitly null",
			body:      comment(`,"createdByUser": null`),
			wantID:    "",
			wantEmail: "jane.doe@example.com",
			wantName:  "Jane Doe",
		},
		{
			// The data source has not been updated yet: no key at all. Must not
			// panic and must not invent an id.
			name:      "user object absent",
			body:      comment(``),
			wantID:    "",
			wantEmail: "jane.doe@example.com",
			wantName:  "Jane Doe",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			})
			svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

			resp, err := svc.SearchCaseComments(contextWithUserIDToken("token"), domain.SearchCaseCommentsRequest{
				CaseID:     sysidToUUID(caseSysid),
				Pagination: domain.Pagination{Limit: 10},
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(resp.Comments) != 1 {
				t.Fatalf("got %d comments, want 1", len(resp.Comments))
			}
			got := resp.Comments[0].CreatedBy
			if id := userRefID(t, got); id != tc.wantID {
				t.Errorf("createdBy.id = %q, want %q", id, tc.wantID)
			}
			if got.Email != tc.wantEmail || got.Name != tc.wantName {
				t.Errorf("createdBy = %+v, want email %q name %q", got, tc.wantEmail, tc.wantName)
			}
		})
	}
}

// TestAttachmentUserReference covers the attachment row, the second site where
// the data source supplies a user id, plus the absent-field fallback.
func TestAttachmentUserReference(t *testing.T) {
	t.Parallel()

	const caseSysid = "cccccccccccccccccccccccccccccccc"

	attachment := func(extra string) string {
		return `{"attachments":[{
			"id": "dddddddddddddddddddddddddddddddd",
			"referenceId": "` + caseSysid + `",
			"name": "log.txt",
			"type": "text/plain",
			"sizeBytes": 12,
			"createdBy": "jane.doe@example.com",
			"createdByFullName": "Jane Doe",
			"createdOn": "2026-01-01 10:00:00"` + extra + `
		}],"totalRecords":1}`
	}

	for _, tc := range []struct {
		name   string
		body   string
		wantID string
	}{
		{"user object supplied", attachment(`,"createdByUser": {"id": "` + testAuthorSysid + `", "email": "jane.doe@example.com", "name": "Jane Doe"}`), testAuthorUUID},
		{"user object absent", attachment(``), ""},
		{"user object with empty id", attachment(`,"createdByUser": {"id": "", "email": "jane.doe@example.com", "name": "Jane Doe"}`), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			})
			svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

			resp, err := svc.SearchCaseAttachments(contextWithUserIDToken("token"), domain.SearchAttachmentsRequest{
				ReferenceID:   sysidToUUID(caseSysid),
				ReferenceType: domain.ReferenceTypeCase,
				Pagination:    domain.Pagination{Limit: 10},
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(resp.Attachments) != 1 {
				t.Fatalf("got %d attachments, want 1", len(resp.Attachments))
			}
			got := resp.Attachments[0].CreatedBy
			if id := userRefID(t, got); id != tc.wantID {
				t.Errorf("createdBy.id = %q, want %q", id, tc.wantID)
			}
			if got.Email != "jane.doe@example.com" || got.Name != "Jane Doe" {
				t.Errorf("createdBy = %+v, want the uploader's email and name", got)
			}
		})
	}
}

// TestCaseViewUserReferences covers the sites with no upstream user id (case
// creator, watchers) alongside the assignee, whose id already arrives with the
// response, and asserts the wire format: an id must serialize as an explicit
// null, never as a missing key.
func TestCaseViewUserReferences(t *testing.T) {
	t.Parallel()

	const (
		caseSysid     = "cccccccccccccccccccccccccccccccc"
		projectSysid  = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
		assigneeSysid = "11112222333344445555666677778888"
		assigneeUUID  = "11112222-3333-4444-5555-666677778888"
		watcherSysid  = "99998888777766665555444433332222"
	)

	body := `{
		"id": "` + caseSysid + `",
		"internalId": "CS0001",
		"number": "CS0001",
		"title": "A case",
		"description": "d",
		"createdOn": "2026-01-01 10:00:00",
		"createdBy": "jane.doe@example.com",
		"createdByFullName": "Jane Doe",
		"project": {"id": "` + projectSysid + `", "name": "Project A"},
		"deployment": {"id": "", "name": ""},
		"deployedProduct": {"id": "", "name": "", "version": ""},
		"state": {"id": 1, "label": "Open"},
		"assignedEngineer": {"id": "` + assigneeSysid + `", "name": "John Roe", "email": "john.roe@example.com"},
		"watchList": [
			{"id": "` + watcherSysid + `", "userName": "jdoe", "name": "Jane Doe", "email": "jane.doe@example.com"}
		]
	}`

	client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	cv, err := svc.GetCaseByID(contextWithUserIDToken("token"), sysidToUUID(caseSysid))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Case creator: the response carries only an email and a full name, so the
	// id is null by design and the consumer resolves it from the email.
	creator := cv.CreatedBy
	if creator == nil {
		t.Fatal("createdBy is nil, want a reference with a null id")
	}
	if creator.ID != nil {
		t.Errorf("createdBy.id = %q, want null: there is no upstream user id for the creator", *creator.ID)
	}
	if creator.Email != "jane.doe@example.com" || creator.Name != "Jane Doe" {
		t.Errorf("createdBy = %+v, want the creator's email and name", creator)
	}

	// Assignee: the assignee's own id already arrives with the response, so it
	// is populated -- in converted UUID form.
	if id := userRefID(t, cv.AssignedEngineer); id != assigneeUUID {
		t.Errorf("assignedEngineer.id = %q, want %q", id, assigneeUUID)
	}
	if cv.AssignedEngineer.Email != "john.roe@example.com" || cv.AssignedEngineer.Name != "John Roe" {
		t.Errorf("assignedEngineer = %+v, want the assignee's email and name", cv.AssignedEngineer)
	}

	// Watcher: a watch-list entry is not guaranteed to be a user record, so its
	// id stays null.
	if len(cv.WatchList) != 1 {
		t.Fatalf("got %d watchers, want 1", len(cv.WatchList))
	}
	watcher := cv.WatchList[0].User
	if watcher == nil {
		t.Fatal("watchList[0].user is nil, want a reference with a null id")
	}
	if watcher.ID != nil {
		t.Errorf("watchList[0].user.id = %q, want null", *watcher.ID)
	}
	if watcher.Email != "jane.doe@example.com" || watcher.Name != "Jane Doe" {
		t.Errorf("watchList[0].user = %+v, want the watcher's email and name", watcher)
	}
	// The incumbent flat watcher fields are untouched.
	if cv.WatchList[0].ID != sysidToUUID(watcherSysid) || cv.WatchList[0].UserName != "jdoe" {
		t.Errorf("existing watcher fields changed: %+v", cv.WatchList[0])
	}

	// The wire format is the contract: a null id must be an explicit null, not
	// an absent key -- an absent key is indistinguishable from a producer that
	// does not populate the field at all.
	encoded, err := json.Marshal(cv)
	if err != nil {
		t.Fatalf("marshal case view: %v", err)
	}
	wire := string(encoded)
	if !strings.Contains(wire, `"createdBy":{"id":null,"email":"jane.doe@example.com","name":"Jane Doe"}`) {
		t.Errorf("createdBy did not serialize with an explicit null id; got:\n%s", wire)
	}
	if !strings.Contains(wire, `"assignedEngineer":{"id":"`+assigneeUUID+`"`) {
		t.Errorf("assignedEngineer did not serialize with the converted id; got:\n%s", wire)
	}
	if !strings.Contains(wire, `"user":{"id":null,"email":"jane.doe@example.com","name":"Jane Doe"}`) {
		t.Errorf("watcher user reference did not serialize with an explicit null id; got:\n%s", wire)
	}
}

// TestCaseActivityUserReference covers the activity feed: no upstream user id
// there either, so every entry carries a null id.
func TestCaseActivityUserReference(t *testing.T) {
	t.Parallel()

	const caseSysid = "cccccccccccccccccccccccccccccccc"

	body := `{"activity":[{
		"id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"type": "comment",
		"content": "a comment",
		"createdOn": "2026-01-01 10:00:00",
		"createdBy": "jane.doe@example.com",
		"createdByFirstName": "Jane",
		"createdByLastName": "Doe",
		"createdByFullName": "Jane Doe",
		"commentType": "comments"
	}],"totalRecords":1}`

	client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	resp, err := svc.SearchCaseActivities(contextWithUserIDToken("token"), domain.SearchCaseActivitiesRequest{
		CaseID:     sysidToUUID(caseSysid),
		Pagination: domain.Pagination{Limit: 10},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Activity) != 1 {
		t.Fatalf("got %d activity entries, want 1", len(resp.Activity))
	}
	got := resp.Activity[0].CreatedBy
	if got == nil {
		t.Fatal("createdBy is nil, want a reference with a null id")
	}
	if got.ID != nil {
		t.Errorf("createdBy.id = %q, want null: the activity feed carries no user id", *got.ID)
	}
	if got.Email != "jane.doe@example.com" || got.Name != "Jane Doe" {
		t.Errorf("createdBy = %+v, want the actor's email and name", got)
	}
	// The first/last name components stay as their own fields: a full name
	// cannot be split back into them reliably.
	if resp.Activity[0].CreatedByFirstName != "Jane" ||
		resp.Activity[0].CreatedByLastName != "Doe" {
		t.Errorf("activity actor name components changed: %+v", resp.Activity[0])
	}
}

// TestNewUserReferenceNoActor pins that a site with nothing to say about the
// actor emits no reference at all rather than an empty object.
func TestNewUserReferenceNoActor(t *testing.T) {
	t.Parallel()

	if ref := domain.NewUserReference("", "", ""); ref != nil {
		t.Errorf("got %+v, want nil for an absent actor", ref)
	}
}

// TestSNUserReferenceNilUpstream pins the helper's behaviour directly for the
// nil-upstream case, independent of any endpoint.
func TestSNUserReferenceNilUpstream(t *testing.T) {
	t.Parallel()

	ref := snUserReference(nil, "jane.doe@example.com", "Jane Doe")
	if ref == nil {
		t.Fatal("got nil, want a reference built from the email and name")
	}
	if ref.ID != nil {
		t.Errorf("id = %q, want null", *ref.ID)
	}
	encoded, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if want := `{"id":null,"email":"jane.doe@example.com","name":"Jane Doe"}`; string(encoded) != want {
		t.Errorf("got %s, want %s", encoded, want)
	}
}
