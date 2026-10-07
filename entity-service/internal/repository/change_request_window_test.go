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
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func strp(s string) *string { return &s }

func TestNormalizePlannedTimestamp_AcceptsExactlyTwoLayoutsAsUTC(t *testing.T) {
	for in, want := range map[string]string{
		"2030-03-01T09:00:00Z":           "2030-03-01T09:00:00Z",
		"2030-03-01T14:30:00+05:30":      "2030-03-01T09:00:00Z",
		"2030-03-01T04:00:00-05:00":      "2030-03-01T09:00:00Z",
		"2030-03-01 09:00:00":            "2030-03-01T09:00:00Z", // no zone: UTC, whatever the database's session says
		"2030-03-01T09:00:00.123456789Z": "2030-03-01T09:00:00.123456Z",
		"2000-01-01 00:00:00":            "2000-01-01T00:00:00Z",
		"2100-12-31T23:59:59Z":           "2100-12-31T23:59:59Z",
		"2030-03-01T00:30:00+05:30":      "2030-02-28T19:00:00Z",
	} {
		got, err := normalizePlannedTimestamp("plannedStartOn", strp(in))
		if err != nil || got == nil || *got != want {
			t.Errorf("normalize(%q) = %v, %v; want %q", in, got, err, want)
		}
	}
	if got, err := normalizePlannedTimestamp("plannedStartOn", nil); got != nil || err != nil {
		t.Errorf("normalize(nil) = %v, %v; want nil, nil", got, err)
	}
}

func TestNormalizePlannedTimestamp_RefusesWhatPostgresWouldHaveAccepted(t *testing.T) {
	for _, in := range []string{
		"infinity", "-infinity", "+infinity", "Infinity", "now", "today", "tomorrow", "yesterday", "epoch", "allballs",
		"2030-03-01", "2030-03-01T09:00:00", "2030-03-01 09:00:00 America/New_York", "2030-03-01 09:00:00 PST",
		"2030-03-01 09:00:00+05:30", "2030-3-1 9:00:00", "March 1 2030", "next tuesday",
		"10000-01-01 00:00:00", "294276-12-31 23:59:59", "1999-12-31T23:59:59Z", "2101-01-01T00:00:00Z", "0001-01-01T00:00:00Z",
		"2000-01-01T00:00:00+14:00", // 1999-12-31T10:00Z: its year, in UTC, is out of range
		" 2030-03-01 09:00:00", "2030-03-01 09:00:00 ", "2030-03-01 09:00:00\x00", "", "   ", "1e9", "1900000000",
		"2030-03-01 09:00:00'; DROP TABLE change_request; --", "2030-02-30 09:00:00", "2030-03-01 24:00:00", "2030-03-01 09:00:60",
	} {
		got, err := normalizePlannedTimestamp("plannedEndOn", strp(in))
		var ve *apierror.ValidationError
		if err == nil || !errors.As(err, &ve) || got != nil {
			t.Errorf("normalize(%q) = %v, %v; want a ValidationError", in, got, err)
			continue
		}
		if !strings.HasPrefix(ve.Msg, "plannedEndOn must be a valid date-time") {
			t.Errorf("normalize(%q): message %q does not name the field and the formats", in, ve.Msg)
		}
	}
}

func TestNormalizePatchAndCreateWindows(t *testing.T) {
	req, err := normalizePatchPlannedWindow(domain.PatchChangeRequestRequest{PlannedStartOn: strp("2030-03-01 09:00:00"), PlannedEndOn: strp("2030-03-01T16:30:00+05:30")})
	if err != nil || *req.PlannedStartOn != "2030-03-01T09:00:00Z" || *req.PlannedEndOn != "2030-03-01T11:00:00Z" {
		t.Fatalf("patch window = %v..%v (%v)", req.PlannedStartOn, req.PlannedEndOn, err)
	}
	if _, err := normalizePatchPlannedWindow(domain.PatchChangeRequestRequest{PlannedStartOn: strp("2030-03-01 09:00:00"), PlannedEndOn: strp("infinity")}); err == nil || !strings.Contains(err.Error(), "plannedEndOn") {
		t.Fatalf("a bad end was not refused by name: %v", err)
	}
	if _, err := normalizePatchPlannedWindow(domain.PatchChangeRequestRequest{}); err != nil {
		t.Fatalf("a request with no window: %v", err)
	}
	create, err := normalizeCreatePlannedWindow(domain.CreateChangeRequestRequest{PlannedStartDate: strp("2030-03-01 09:00:00")})
	if err != nil || *create.PlannedStartDate != "2030-03-01T09:00:00Z" || create.PlannedEndDate != nil {
		t.Fatalf("create window = %v..%v (%v)", create.PlannedStartDate, create.PlannedEndDate, err)
	}
	if _, err := normalizeCreatePlannedWindow(domain.CreateChangeRequestRequest{PlannedEndDate: strp("tomorrow")}); err == nil || !strings.Contains(err.Error(), "plannedEndDate") {
		t.Fatalf("a bad create end was not refused by name: %v", err)
	}
}

func TestRequireFutureWindow(t *testing.T) {
	now := time.Date(2030, 3, 1, 12, 0, 0, 0, time.UTC)
	past, future := "2030-03-01T11:59:59Z", "2030-03-01T12:00:01Z"
	if err := requireFutureWindow(now, &future, &future); err != nil {
		t.Errorf("a future window: %v", err)
	}
	if err := requireFutureWindow(now, nil, nil); err != nil {
		t.Errorf("no window: %v", err)
	}
	for name, w := range map[string][2]*string{"start": {&past, nil}, "end": {nil, &past}, "now itself": {strp("2030-03-01T12:00:00Z"), nil}} {
		err := requireFutureWindow(now, w[0], w[1])
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) || !strings.Contains(ve.Msg, "is in the past") {
			t.Errorf("%s in the past: %v, want a ValidationError saying so", name, err)
		}
	}
}

func TestParseExpectedTimestamp(t *testing.T) {
	// What the API printed round-trips, and a stored value outside the range a
	// window may be proposed in still can be named (it is only compared).
	for _, in := range []string{"2030-03-01T09:00:00Z", "2030-03-01 09:00:00", "1998-01-01T00:00:00Z", "2030-03-01T09:00:00.123456Z"} {
		if got, err := parseExpectedTimestamp("expectedPlannedStartOn", strp(in)); err != nil || got == nil {
			t.Errorf("parse(%q) = %v, %v", in, got, err)
		}
	}
	if got, err := parseExpectedTimestamp("expectedPlannedStartOn", nil); got != nil || err != nil {
		t.Errorf("parse(nil) = %v, %v", got, err)
	}
	for _, in := range []string{"infinity", "now", "", "2030-03-01", "next week"} {
		if _, err := parseExpectedTimestamp("expectedPlannedStartOn", strp(in)); err == nil {
			t.Errorf("parse(%q) was accepted", in)
		}
	}
}

func TestRedactInternalApprovalStages(t *testing.T) {
	label := func(s string) *string { return &s }
	stages := []changeRequestApprovalStageRow{
		{checkpointLabel: label(approvalStageLabelPeer)},
		{checkpointLabel: label(approvalStageLabelCAB)},
		{checkpointLabel: label(approvalStageLabelECAB)},
		{checkpointLabel: label(approvalStageLabelReview)},
		{checkpointLabel: label(approvalStageLabelCustomerApproval)},
		{checkpointLabel: label(approvalStageLabelCustomerReview)},
		{checkpointLabel: nil}, // a stage with no label: classified by position (6th: other), so internal
	}
	build := func() domain.ChangeRequestApprovals {
		var out domain.ChangeRequestApprovals
		for _, st := range stages {
			name := "A Person"
			out.Approvals = append(out.Approvals, domain.ChangeRequestApproval{
				Stage: "stage", ApproverName: "Some Group", Status: domain.ChangeRequestApprovalStatusPending,
				AssignmentGroup: &domain.ChangeRequestApprovalGroup{ID: "g", Name: "Some Group"},
				Approvers:       []domain.ChangeRequestApprover{{ID: "11111111-1111-1111-1111-111111111111", Name: name, Status: "REQUESTED", CanDecide: st.checkpointLabel != nil}},
			})
		}
		return out
	}
	got := build()
	redactInternalApprovalStages(stages, &got)
	for i, a := range got.Approvals {
		internal := i < 4 || i == 6
		if internal {
			if a.ApproverName != "" || a.AssignmentGroup != nil || a.Approvers == nil || len(a.Approvers) != 0 {
				t.Errorf("stage %d (internal) still carries %+v", i, a)
			}
			if a.Stage != "stage" || a.Status != domain.ChangeRequestApprovalStatusPending {
				t.Errorf("stage %d lost its label or status: %+v", i, a)
			}
			continue
		}
		if a.ApproverName == "" || a.AssignmentGroup == nil || len(a.Approvers) != 1 || a.Approvers[0].Name != "A Person" || !a.Approvers[0].CanDecide {
			t.Errorf("stage %d (the customer's) was cut down: %+v", i, a)
		}
	}
}
