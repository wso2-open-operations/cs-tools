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

package main

import (
	"testing"
	"time"
)

func TestBuildArticlePlan_Published(t *testing.T) {
	created := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	updated := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	row := snKnowledge{
		SysID:                  "aaaa1111aaaa1111aaaa1111aaaa1111",
		ShortDescription:       "How to X",
		Text:                   "<p>body</p>",
		WorkflowState:          "published",
		RejectReason:           "stale reason should be dropped",
		GeneratedWithNowAssist: true,
		GeneratedBy:            "now_assist",
		HelpfulCount:           3,
		UseCount:               10,
		ViewCount:              50,
		CreatedOn:              created,
		UpdatedOn:              updated,
	}
	updatedBy := "resolved-reviser"
	caseID := "resolved-case-uuid"

	plan, err := buildArticlePlan(row, "kb-uuid", "resolved-author", &updatedBy, &caseID)
	if err != nil {
		t.Fatalf("buildArticlePlan: %v", err)
	}
	if plan.State != statePublished {
		t.Errorf("State = %q, want %q", plan.State, statePublished)
	}
	if plan.RejectionComment != nil {
		t.Errorf("RejectionComment = %v, want nil for a published article", plan.RejectionComment)
	}
	if plan.SubmittedAt == nil || !plan.SubmittedAt.Equal(created) {
		t.Errorf("SubmittedAt = %v, want %v", plan.SubmittedAt, created)
	}
	if plan.PublishedAt == nil || !plan.PublishedAt.Equal(updated) {
		t.Errorf("PublishedAt = %v, want %v", plan.PublishedAt, updated)
	}
	if plan.RetiredAt != nil {
		t.Errorf("RetiredAt = %v, want nil", plan.RetiredAt)
	}
	if plan.AuthorID != "resolved-author" || plan.UpdatedBy == nil || *plan.UpdatedBy != "resolved-reviser" {
		t.Errorf("author/updated_by not carried through: %+v", plan)
	}
	if plan.SourceCaseID == nil || *plan.SourceCaseID != "resolved-case-uuid" {
		t.Errorf("SourceCaseID not carried through: %v", plan.SourceCaseID)
	}
	if !plan.GeneratedWithAI || plan.AIGeneratedBy == nil || *plan.AIGeneratedBy != "now_assist" {
		t.Errorf("AI fields not carried through: %+v", plan)
	}
}

func TestBuildArticlePlan_Draft_NoOptionalRefs(t *testing.T) {
	row := snKnowledge{
		SysID:            "bbbb2222bbbb2222bbbb2222bbbb2222",
		ShortDescription: "Draft article",
		Text:             "<p>wip</p>",
		WorkflowState:    "draft",
		RejectReason:     "needs more detail",
		CreatedOn:        time.Now(),
		UpdatedOn:        time.Now(),
	}
	plan, err := buildArticlePlan(row, "kb-uuid", "resolved-author", nil, nil)
	if err != nil {
		t.Fatalf("buildArticlePlan: %v", err)
	}
	if plan.State != stateDraft {
		t.Errorf("State = %q, want draft", plan.State)
	}
	if plan.SubmittedAt != nil || plan.PublishedAt != nil || plan.RetiredAt != nil {
		t.Errorf("expected no lifecycle timestamps on a draft, got %+v", plan)
	}
	if plan.RejectionComment == nil || *plan.RejectionComment != "needs more detail" {
		t.Errorf("RejectionComment = %v, want set on a draft", plan.RejectionComment)
	}
	if plan.UpdatedBy != nil {
		t.Errorf("UpdatedBy = %v, want nil", plan.UpdatedBy)
	}
	if plan.SourceCaseID != nil {
		t.Errorf("SourceCaseID = %v, want nil", plan.SourceCaseID)
	}
	if plan.AIGeneratedBy != nil {
		t.Errorf("AIGeneratedBy = %v, want nil for an empty u_generated_by", plan.AIGeneratedBy)
	}
}

func TestBuildArticlePlan_UnrecognisedWorkflowState(t *testing.T) {
	row := snKnowledge{SysID: "cccc3333cccc3333cccc3333cccc3333", WorkflowState: "not_a_real_state"}
	if _, err := buildArticlePlan(row, "kb-uuid", "author", nil, nil); err == nil {
		t.Fatal("expected an error for an unrecognised workflow_state")
	}
}

func TestHistoryChangedBySource(t *testing.T) {
	t.Run("prefers revised_by when present", func(t *testing.T) {
		row := snKnowledge{Author: "author-sysid", RevisedBy: "revisor-sysid"}
		if got := historyChangedBySource(row); got != "revisor-sysid" {
			t.Errorf("got %q, want %q", got, "revisor-sysid")
		}
	})
	t.Run("falls back to author when revised_by empty", func(t *testing.T) {
		row := snKnowledge{Author: "author-sysid", RevisedBy: ""}
		if got := historyChangedBySource(row); got != "author-sysid" {
			t.Errorf("got %q, want %q", got, "author-sysid")
		}
	})
}

func TestBuildHistoryPlan(t *testing.T) {
	row := snKnowledge{
		SysID:                  "dddd4444dddd4444dddd4444dddd4444",
		ShortDescription:       "Old title",
		Text:                   "<p>old body</p>",
		WorkflowState:          "retired",
		GeneratedWithNowAssist: false,
		GeneratedBy:            "",
	}
	plan, err := buildHistoryPlan(row, "resolved-changed-by")
	if err != nil {
		t.Fatalf("buildHistoryPlan: %v", err)
	}
	if plan.State != stateRetired {
		t.Errorf("State = %q, want retired", plan.State)
	}
	if plan.ChangedBy != "resolved-changed-by" {
		t.Errorf("ChangedBy = %q", plan.ChangedBy)
	}
	if plan.AIGeneratedBy != nil {
		t.Errorf("AIGeneratedBy = %v, want nil", plan.AIGeneratedBy)
	}
	if plan.SourceSysID != row.SysID {
		t.Errorf("SourceSysID = %q, want %q (each history row keeps its OWN sys_id)", plan.SourceSysID, row.SysID)
	}
}

func TestBuildHistoryPlan_UnrecognisedWorkflowState(t *testing.T) {
	row := snKnowledge{SysID: "eeee5555eeee5555eeee5555eeee5555", WorkflowState: "??"}
	if _, err := buildHistoryPlan(row, "someone"); err == nil {
		t.Fatal("expected an error for an unrecognised workflow_state")
	}
}
