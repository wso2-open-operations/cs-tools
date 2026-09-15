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

func TestCollapseWorkflowState(t *testing.T) {
	tests := []struct {
		raw     string
		want    string
		wantErr bool
	}{
		{"draft", stateDraft, false},
		{"review", statePendingReview, false},
		{"scheduled_publish", statePublished, false},
		{"published", statePublished, false},
		{"pending_retirement", stateRetired, false},
		{"retired", stateRetired, false},
		{"outdated", stateRetired, false},
		{"u_state_rejected_lookalike", "", true}, // not a real workflow_state value
		{"", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got, err := collapseWorkflowState(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("collapseWorkflowState(%q) error = %v, wantErr %v", tt.raw, err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Errorf("collapseWorkflowState(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestComputeArticleDefaults(t *testing.T) {
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	updated := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	t.Run("draft: no timestamps, rejection_comment allowed", func(t *testing.T) {
		d := computeArticleDefaults(stateDraft, created, updated, "not good enough")
		if d.SubmittedAt != nil {
			t.Errorf("SubmittedAt = %v, want nil", d.SubmittedAt)
		}
		if d.PublishedAt != nil {
			t.Errorf("PublishedAt = %v, want nil", d.PublishedAt)
		}
		if d.RetiredAt != nil {
			t.Errorf("RetiredAt = %v, want nil", d.RetiredAt)
		}
		if d.RejectionComment == nil || *d.RejectionComment != "not good enough" {
			t.Errorf("RejectionComment = %v, want %q", d.RejectionComment, "not good enough")
		}
	})

	t.Run("draft with empty reject reason: rejection_comment stays nil", func(t *testing.T) {
		d := computeArticleDefaults(stateDraft, created, updated, "")
		if d.RejectionComment != nil {
			t.Errorf("RejectionComment = %v, want nil", d.RejectionComment)
		}
	})

	t.Run("pending_review: submitted_at set, no published/retired, no rejection_comment", func(t *testing.T) {
		d := computeArticleDefaults(statePendingReview, created, updated, "stale reason")
		if d.SubmittedAt == nil || !d.SubmittedAt.Equal(created) {
			t.Errorf("SubmittedAt = %v, want %v", d.SubmittedAt, created)
		}
		if d.PublishedAt != nil {
			t.Errorf("PublishedAt = %v, want nil", d.PublishedAt)
		}
		if d.RetiredAt != nil {
			t.Errorf("RetiredAt = %v, want nil", d.RetiredAt)
		}
		if d.RejectionComment != nil {
			t.Errorf("RejectionComment = %v, want nil (stale SN value must be dropped outside draft)", d.RejectionComment)
		}
	})

	t.Run("published: submitted_at and published_at set", func(t *testing.T) {
		d := computeArticleDefaults(statePublished, created, updated, "")
		if d.SubmittedAt == nil || !d.SubmittedAt.Equal(created) {
			t.Errorf("SubmittedAt = %v, want %v", d.SubmittedAt, created)
		}
		if d.PublishedAt == nil || !d.PublishedAt.Equal(updated) {
			t.Errorf("PublishedAt = %v, want %v", d.PublishedAt, updated)
		}
		if d.RetiredAt != nil {
			t.Errorf("RetiredAt = %v, want nil", d.RetiredAt)
		}
	})

	t.Run("retired: submitted_at and retired_at set, not published_at", func(t *testing.T) {
		d := computeArticleDefaults(stateRetired, created, updated, "")
		if d.SubmittedAt == nil || !d.SubmittedAt.Equal(created) {
			t.Errorf("SubmittedAt = %v, want %v", d.SubmittedAt, created)
		}
		if d.PublishedAt != nil {
			t.Errorf("PublishedAt = %v, want nil", d.PublishedAt)
		}
		if d.RetiredAt == nil || !d.RetiredAt.Equal(updated) {
			t.Errorf("RetiredAt = %v, want %v", d.RetiredAt, updated)
		}
	})
}
