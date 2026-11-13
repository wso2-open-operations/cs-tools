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

package repository

import (
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func TestIsCABStageInHistory(t *testing.T) {
	str := func(s string) *string { return &s }
	cabGroup, peerGroup := str(domain.CABApprovalGroupName), str("Peer Reviewers")
	for _, tc := range []struct {
		name         string
		label, group *string
		pos          int
		model        string
		want         bool
	}{
		{"labelled CAB stage", str("CAB Approval"), nil, 0, "NORMAL", true},
		{"legacy Authorize label", str("Authorize"), nil, 0, "NORMAL", true},
		{"historic ECAB label", str("ECAB Approval"), nil, 0, "EMERGENCY", true},
		{"labelled peer stage, even in the CAB group", str("Peer Approval"), cabGroup, 1, "NORMAL", false},
		{"labelled review stage", str("Review"), nil, 1, "NORMAL", false},
		{"customer stage", str("Customer Approval"), nil, 1, "NORMAL", false},
		{"the standard change authority's label is not a change request stage", str("Standard Change Authority"), cabGroup, 1, "STANDARD", false},
		{"unlabelled, second stage (Authorize)", nil, peerGroup, 1, "NORMAL", true},
		{"unlabelled, first stage (Assess)", nil, peerGroup, 0, "NORMAL", false},
		{"unlabelled, third stage (Customer Approval)", nil, peerGroup, 2, "NORMAL", false},
		{"unlabelled, first stage in the CAB group", nil, cabGroup, 0, "NORMAL", true},
		{"unlabelled emergency stage in the CAB group", nil, cabGroup, 0, "EMERGENCY", true},
		{"empty label reads as none", str(""), nil, 1, "NORMAL", true},
	} {
		if got := isCABStageInHistory(tc.label, tc.pos, tc.group, tc.model); got != tc.want {
			t.Errorf("%s: isCABStageInHistory = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestIsCABStageInHistory_AgreesWithTheApprovalsView pins the history reading to
// the approvals view's own: every stage the view names as the CAB's is a CAB
// stage here, and the history agrees with the decision flow at Authorize
// (runtimeApprovalStageKind) except where the flow only guesses from the
// change's type and state (an Emergency change's unlabelled stage outside the
// CAB group, which history judges by position as the view does).
func TestIsCABStageInHistory_AgreesWithTheApprovalsView(t *testing.T) {
	str := func(s string) *string { return &s }
	labels := []*string{nil, str(""), str("CAB Approval"), str("Authorize"), str("ECAB Approval"), str("Peer Approval"),
		str("Assess"), str("Review"), str("Customer Approval"), str("Customer Review"), str("Something else")}
	groups := []*string{nil, str(domain.CABApprovalGroupName), str("Peer Reviewers")}
	for _, label := range labels {
		for _, group := range groups {
			for pos := 0; pos < 4; pos++ {
				for _, model := range []string{"NORMAL", "EMERGENCY", "STANDARD", ""} {
					got := isCABStageInHistory(label, pos, group, model)
					shown, _ := changeRequestApprovalStageLabel(changeRequestApprovalStageRow{checkpointLabel: label, assignmentGroupName: group}, pos, model)
					if classifyApprovalStage(&shown, pos) == stageKindCAB && !got {
						t.Errorf("label %v group %v pos %d model %q: the view shows %q (CAB) but history says no", cabDeref(label), cabDeref(group), pos, model, shown)
					}
					runtime := runtimeApprovalStageKind(label, pos, group, model, crStateAuthorize) == stageKindCAB
					guessed := (label == nil || *label == "") && isEmergencyModel(model) && cabDeref(group) != domain.CABApprovalGroupName
					if !guessed && runtime != got {
						t.Errorf("label %v group %v pos %d model %q: the flow at Authorize says CAB=%v, history %v", cabDeref(label), cabDeref(group), pos, model, runtime, got)
					}
				}
			}
		}
	}
}

func cabDeref(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}
