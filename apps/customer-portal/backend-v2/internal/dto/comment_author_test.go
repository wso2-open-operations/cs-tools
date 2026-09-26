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
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// entityCommentsPayload is shaped exactly as entity-service now sends it, with
// createdBy as the UserReference object that replaced
// CommentUserRef{firstName,lastName,fullName}.
const entityCommentsPayload = `{
  "comments": [
    {
      "id": "9c5a8d1c3b4fcf103e1e088aa4e45a22",
      "content": "Hello! Do you have a WSO2 question I can help with?",
      "type": "comment",
      "createdOn": "2026-08-27T16:23:50Z",
      "createdBy": {"id": null, "email": "", "name": "Novera"}
    },
    {
      "id": "185a8d1c3b4fcf103e1e088aa4e45a18",
      "content": "hi",
      "type": "comment",
      "createdOn": "2026-08-27T16:23:49Z",
      "createdBy": {"id": "u-1", "email": "jane.doe@example.com", "name": "Jane Doe"}
    }
  ],
  "total": 2, "limit": 10, "offset": 0
}`

// TestMapSearchComments_KeepsTheAssistantIdentifiable is the regression guard for
// the chat history rendering every message as "You" in the wrong order.
//
// ConversationDetailsPage decides a message came from the assistant with
// createdBy.toLowerCase() === "novera", and uses that same signal to break ties
// between messages sharing a timestamp. When createdBy arrived empty, every
// message was classified as the user's and the ordering tie-break disappeared.
func TestMapSearchComments_KeepsTheAssistantIdentifiable(t *testing.T) {
	var resp entity.SearchCommentsResponse
	if err := json.Unmarshal([]byte(entityCommentsPayload), &resp); err != nil {
		t.Fatalf("decoding entity-service's payload failed: %v", err)
	}
	if len(resp.Comments) != 2 {
		t.Fatalf("decoded %d comments, want 2", len(resp.Comments))
	}

	out := MapSearchComments(resp)
	if len(out.Comments) != 2 {
		t.Fatalf("mapped %d comments, want 2", len(out.Comments))
	}

	assistant, human := out.Comments[0], out.Comments[1]

	if assistant.CreatedBy == "" {
		t.Fatal("assistant createdBy is empty — the chat renders every message as the user's and loses its ordering tie-break")
	}
	if strings.ToLower(assistant.CreatedBy) != "novera" {
		t.Errorf("assistant createdBy = %q; the frontend matches createdBy.toLowerCase() == \"novera\"", assistant.CreatedBy)
	}
	if human.CreatedBy != "Jane Doe" {
		t.Errorf("human createdBy = %q, want the display name", human.CreatedBy)
	}
	// The display label falls back to createdBy, so a name belongs there — not an
	// email, and not an empty string.
	if strings.Contains(human.CreatedBy, "@") {
		t.Errorf("human createdBy = %q; an email renders where a name belongs", human.CreatedBy)
	}
}

// TestMapSearchComments_HandlesAnUnresolvedAuthor keeps a nil reference from
// panicking and leaves the label empty rather than inventing one.
func TestMapSearchComments_HandlesAnUnresolvedAuthor(t *testing.T) {
	out := MapSearchComments(entity.SearchCommentsResponse{
		Comments: []entity.CommentView{
			{ID: "c1", Content: "orphan", CreatedBy: nil},
			{ID: "c2", Content: "email only", CreatedBy: &entity.UserReference{Email: "a@example.com"}},
		},
		Total: 2,
	})
	if len(out.Comments) != 2 {
		t.Fatalf("mapped %d comments, want 2", len(out.Comments))
	}
	if out.Comments[0].CreatedBy != "" {
		t.Errorf("nil author: createdBy = %q, want empty", out.Comments[0].CreatedBy)
	}
	if out.Comments[1].CreatedBy != "" {
		t.Errorf("email-only author: createdBy = %q, want empty rather than an email in a name field", out.Comments[1].CreatedBy)
	}
}

// TestMapSearchCaseActivities_AutomationAuthorRendersAsSystem is the regression
// guard for an automation/integration-authored case comment (e.g. an unattended
// workflow posting on a case) rendering as "" instead of a usable author. An
// empty createdBy is indistinguishable from a genuinely unknown author, so
// downstream consumers showed "Unknown" for what should read as "System" — the
// literal string the previous entity-service contract sent for the same case.
func TestMapSearchCaseActivities_AutomationAuthorRendersAsSystem(t *testing.T) {
	out := MapSearchCaseActivities(entity.SearchCaseActivitiesResponse{
		Activity: []entity.CaseActivity{
			// No resolvable identity at all: the shape an automation-authored
			// comment arrives in.
			{ID: "a1", Type: "comment", Content: "auto-update", CreatedBy: nil},
			// Present but empty on every field — same "nobody real" case as nil.
			{ID: "a2", Type: "comment", Content: "auto-update 2", CreatedBy: &entity.UserReference{}},
			// A real person still resolves normally and is unaffected.
			{ID: "a3", Type: "comment", Content: "hi", CreatedBy: &entity.UserReference{Email: "jane.doe@example.com", Name: "Jane Doe"}},
			// Resolved to a real account (email present) but with no name to
			// display: a person, not an automation actor, even though we
			// cannot label them — must stay empty, not "system".
			{ID: "a4", Type: "comment", Content: "email only", CreatedBy: &entity.UserReference{Email: "a@example.com"}},
			// The automation account: the upstream data source has no real user
			// record to resolve it against, so it reuses the raw "system"
			// literal in the email slot, with whatever display name it also
			// carries for that account.
			{ID: "a5", Type: "comment", Content: "auto-update 3", CreatedBy: &entity.UserReference{Email: "system", Name: "System"}},
			// Same automation account, but with the name slot blank — must still
			// render as "system", not empty.
			{ID: "a6", Type: "comment", Content: "auto-update 4", CreatedBy: &entity.UserReference{Email: "system", Name: ""}},
		},
		Total: 6,
	})
	if len(out.Activities) != 6 {
		t.Fatalf("mapped %d activities, want 6", len(out.Activities))
	}
	for _, a := range []CaseActivity{out.Activities[0], out.Activities[1], out.Activities[4], out.Activities[5]} {
		if a.CreatedBy != systemAuthorLabel {
			t.Errorf("%s: createdBy = %q, want %q", a.ID, a.CreatedBy, systemAuthorLabel)
		}
		if a.CreatedByFullName != systemAuthorLabel {
			t.Errorf("%s: createdByFullName = %q, want %q", a.ID, a.CreatedByFullName, systemAuthorLabel)
		}
	}
	if out.Activities[2].CreatedBy != "Jane Doe" {
		t.Errorf("human author: createdBy = %q, want the display name", out.Activities[2].CreatedBy)
	}
	if out.Activities[3].CreatedBy != "" {
		t.Errorf("email-only author: createdBy = %q, want empty rather than \"system\"", out.Activities[3].CreatedBy)
	}
	if out.Activities[3].CreatedByFullName != "" {
		t.Errorf("email-only author: createdByFullName = %q, want empty rather than \"system\"", out.Activities[3].CreatedByFullName)
	}
}

// TestMapCommentCreate_UsesTheAuthorName covers the create path, which mapped the
// same removed FullName field.
func TestMapCommentCreate_UsesTheAuthorName(t *testing.T) {
	out := MapCommentCreate(entity.CreateCommentResponse{
		Comment: entity.CommentCreated{
			ID:        "c1",
			CreatedBy: &entity.UserReference{Email: "a@example.com", Name: "Jane Doe"},
		},
	})
	if out.CreatedBy != "Jane Doe" {
		t.Errorf("createdBy = %q, want the author name", out.CreatedBy)
	}
}
