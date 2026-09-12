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

package dto

import (
	"encoding/json"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// entitySearchCasesPayload is a verbatim-shaped entity-service response, with
// createdBy as the object it became in the "one canonical person reference per
// response field" change. Decoding this against a string field is what produced
// "Failed to search cases." on every case search.
const entitySearchCasesPayload = `{
  "cases": [
    {
      "id": "9fe85754-3ba2-cb10-3e1e-088aa4e45aae",
      "internalId": "CUPRSUB-1015",
      "number": "CS0441080",
      "createdOn": "2026-08-03 06:15:31",
      "updatedOn": "2026-08-03 07:02:30",
      "createdBy": {"id": "u-1", "email": "jane.doe@example.com", "name": "Jane Doe"},
      "subject": "test",
      "state": "Open",
      "type": "announcement",
      "project": {"id": "6fa0b42d-1bfa-a694-a002-c9d3604bcb77", "name": "Customer 3 Project"}
    }
  ],
  "total": 1, "limit": 10, "offset": 0
}`

// TestSearchCases_DecodesObjectCreatedBy is the regression guard for
// "Failed to search cases." — a 500 produced entirely inside backend-v2 while
// entity-service returned a valid 200.
func TestSearchCases_DecodesObjectCreatedBy(t *testing.T) {
	var resp entity.SearchCasesResponse
	if err := json.Unmarshal([]byte(entitySearchCasesPayload), &resp); err != nil {
		t.Fatalf("decoding entity-service's real payload failed: %v", err)
	}
	if len(resp.Cases) != 1 {
		t.Fatalf("decoded %d cases, want 1", len(resp.Cases))
	}
	if resp.Cases[0].CreatedBy == nil || resp.Cases[0].CreatedBy.Email != "jane.doe@example.com" {
		t.Fatalf("createdBy = %+v, want the email decoded from the object", resp.Cases[0].CreatedBy)
	}

	// The portal contract stays a plain identity string.
	b, err := json.Marshal(MapSearchCases(resp))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out struct {
		Cases []map[string]json.RawMessage `json:"cases"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	var identity string
	if err := json.Unmarshal(out.Cases[0]["createdBy"], &identity); err != nil {
		t.Fatalf("createdBy is not a JSON string (%s); the frontend reads it as one", out.Cases[0]["createdBy"])
	}
	if identity != "jane.doe@example.com" {
		t.Errorf("createdBy = %q, want the email — isNoveraOrBotSender treats this as an identity", identity)
	}
}

// TestUserRefIdentity_PrefersEmail pins the identity choice. The frontend
// compares createdBy against "novera" for bot attribution and the Ballerina
// backend sent the email here, so the display name must not win.
func TestUserRefIdentity_PrefersEmail(t *testing.T) {
	both := &entity.UserReference{Email: "a@example.com", Name: "Jane Doe"}
	if got := userRefIdentity(both); got != "a@example.com" {
		t.Errorf("userRefIdentity = %q, want the email", got)
	}
	nameOnly := &entity.UserReference{Name: "Jane Doe"}
	if got := userRefIdentity(nameOnly); got != "Jane Doe" {
		t.Errorf("no email: got %q, want the name as a fallback rather than empty", got)
	}
	if got := userRefIdentity(nil); got != "" {
		t.Errorf("nil: got %q, want empty", got)
	}
}

// TestUserRefDisplayName_IsNameOnly keeps an email out of a field that renders as
// a person's name — showing "a@example.com" where a name belongs is worse than
// showing nothing, because CommentBubble falls back to createdBy when this is
// empty.
func TestUserRefDisplayName_IsNameOnly(t *testing.T) {
	if got := userRefDisplayName(&entity.UserReference{Email: "a@example.com"}); got != "" {
		t.Errorf("email-only reference: got %q, want empty", got)
	}
	if got := userRefDisplayName(&entity.UserReference{Email: "a@example.com", Name: "Jane Doe"}); got != "Jane Doe" {
		t.Errorf("got %q, want the name", got)
	}
	if got := userRefDisplayName(nil); got != "" {
		t.Errorf("nil: got %q, want empty", got)
	}
}

// TestCaseActivities_DecodeObjectCreatedBy covers the second broken endpoint,
// POST /cases/{id}/activities/search, and the field entity-service removed:
// createdByFullName no longer arrives, so the display name has to come from the
// reference or every comment renders as "Unknown".
func TestCaseActivities_DecodeObjectCreatedBy(t *testing.T) {
	payload := `{"activity":[{
	  "id":"a1","type":"comment","content":"hello","createdOn":"2026-08-03T06:15:31Z",
	  "createdBy":{"id":"u-1","email":"jane.doe@example.com","name":"Jane Doe"},
	  "createdByFirstName":"Anuradha","createdByLastName":"B"
	}],"total":1,"limit":10,"offset":0}`

	var resp entity.SearchCaseActivitiesResponse
	if err := json.Unmarshal([]byte(payload), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	mapped := MapSearchCaseActivities(resp)
	if len(mapped.Activities) != 1 {
		t.Fatalf("got %d activities, want 1", len(mapped.Activities))
	}
	a := mapped.Activities[0]
	if a.CreatedByFullName != "Jane Doe" {
		t.Errorf("createdByFullName = %q, want the name from the reference", a.CreatedByFullName)
	}
	if a.CreatedBy != "Jane Doe" {
		t.Errorf("createdBy = %q, want the same display name this mapper produced before the upstream change", a.CreatedBy)
	}
	if a.CreatedByFirstName != "Anuradha" || a.CreatedByLastName != "B" {
		t.Errorf("first/last name lost: %q %q", a.CreatedByFirstName, a.CreatedByLastName)
	}
}
