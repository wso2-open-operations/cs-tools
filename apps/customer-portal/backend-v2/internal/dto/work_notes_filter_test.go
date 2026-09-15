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
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

func TestMapSearchCaseActivities_FiltersWorkNotes(t *testing.T) {
	now := time.Now().UTC()
	commentTypeComment := entity.CommentTypeComment
	commentTypeWorkNote := entity.CommentTypeWorkNote
	commentTypeWorkNotesCustom := entity.CommentType("work_notes")

	resp := entity.SearchCaseActivitiesResponse{
		Activity: []entity.CaseActivity{
			{
				ID:          "act-1",
				Type:        entity.ActivityTypeComment,
				Content:     "Customer-visible comment",
				CreatedOn:   now,
				CommentType: &commentTypeComment,
			},
			{
				ID:          "act-2",
				Type:        entity.ActivityTypeComment,
				Content:     "Internal engineer discussion - MUST BE FILTERED",
				CreatedOn:   now,
				CommentType: &commentTypeWorkNote,
			},
			{
				ID:          "act-3",
				Type:        entity.ActivityTypeComment,
				Content:     "Another internal note with work_notes - MUST BE FILTERED",
				CreatedOn:   now,
				CommentType: &commentTypeWorkNotesCustom,
			},
			{
				ID:        "act-4",
				Type:      entity.ActivityType("work_note"),
				Content:   "Work note with Type set to work_note - MUST BE FILTERED",
				CreatedOn: now,
			},
			{
				ID:          "act-5",
				Type:        entity.ActivityTypeAttachment,
				FileName:    "screenshot.png",
				ContentType: "image/png",
				SizeBytes:   1024,
				CreatedOn:   now,
			},
			{
				ID:        "act-6",
				Type:      entity.ActivityTypeFieldChange,
				CreatedOn: now,
				Changes: []entity.FieldChange{
					{
						Field:         "state",
						FieldLabel:    "State",
						PreviousValue: "New",
						NewValue:      "In Progress",
					},
				},
			},
		},
		Total:   6,
		Limit:   10,
		Offset:  0,
		HasMore: false,
	}

	mapped := MapSearchCaseActivities(resp)

	if len(mapped.Activities) != 3 {
		t.Fatalf("got %d activities, want 3 (work notes must be filtered out)", len(mapped.Activities))
	}

	// Verify the remaining activities are only customer-safe entries
	if mapped.Activities[0].ID != "act-1" || mapped.Activities[0].Content != "Customer-visible comment" {
		t.Errorf("expected act-1 as first activity, got %+v", mapped.Activities[0])
	}
	if mapped.Activities[1].ID != "act-5" || mapped.Activities[1].FileName != "screenshot.png" {
		t.Errorf("expected act-5 as second activity, got %+v", mapped.Activities[1])
	}
	if mapped.Activities[2].ID != "act-6" || len(mapped.Activities[2].Changes) != 1 {
		t.Errorf("expected act-6 as third activity, got %+v", mapped.Activities[2])
	}
}

func TestMapSearchComments_FiltersWorkNotes(t *testing.T) {
	now := time.Now().UTC()

	resp := entity.SearchCommentsResponse{
		Comments: []entity.CommentView{
			{
				ID:        "c-1",
				Content:   "Customer visible comment",
				Type:      entity.CommentTypeComment,
				CreatedOn: now,
			},
			{
				ID:        "c-2",
				Content:   "Internal work note - MUST BE FILTERED",
				Type:      entity.CommentTypeWorkNote,
				CreatedOn: now,
			},
			{
				ID:        "c-3",
				Content:   "Internal work notes plural - MUST BE FILTERED",
				Type:      entity.CommentType("work_notes"),
				CreatedOn: now,
			},
			{
				ID:        "c-4",
				Content:   "Another public comment",
				Type:      entity.CommentTypeComment,
				CreatedOn: now,
			},
		},
		Total:   4,
		Limit:   10,
		Offset:  0,
		HasMore: false,
	}

	mapped := MapSearchComments(resp)

	if len(mapped.Comments) != 2 {
		t.Fatalf("got %d comments, want 2 (work notes must be filtered out)", len(mapped.Comments))
	}

	if mapped.Comments[0].ID != "c-1" || mapped.Comments[0].Content != "Customer visible comment" {
		t.Errorf("expected c-1 as first comment, got %+v", mapped.Comments[0])
	}
	if mapped.Comments[1].ID != "c-4" || mapped.Comments[1].Content != "Another public comment" {
		t.Errorf("expected c-4 as second comment, got %+v", mapped.Comments[1])
	}
}
