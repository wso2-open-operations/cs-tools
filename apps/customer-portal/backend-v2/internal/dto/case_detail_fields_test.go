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
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// TestMapCaseDetails_ExposesFieldsTheFrontendDeclares covers the fields added
// because the frontend's CaseDetails type declares them
// (features/support/types/cases.ts) while this backend never sent them —
// entity-service was discarding them from the upstream case response.
// CloseNotes is deliberately excluded from what's asserted present here (see
// the closeNotes assertion below): it's an internal CS-agent field that must
// never reach the customer-facing response, even though it's still decoded
// off the upstream CaseView.
func TestMapCaseDetails_ExposesFieldsTheFrontendDeclares(t *testing.T) {
	sla := "4 hours"
	start, end := "2026-01-01", "2026-06-30"
	auto := true
	notes := "Resolved successfully"

	raw, err := json.Marshal(MapCaseDetails(entity.CaseView{
		SLAResponseTime:     &sla,
		ClosedBy:            &entity.EntityRef{ID: "user-1", Name: "Closer"},
		CloseNotes:          &notes,
		HasAutoClosed:       &auto,
		EngagementStartDate: &start,
		EngagementEndDate:   &end,
	}))
	if err != nil {
		t.Fatalf("marshal returned error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}

	if got["slaResponseTime"] != sla {
		t.Errorf("slaResponseTime = %v, want %q", got["slaResponseTime"], sla)
	}
	if got["engagementStartDate"] != start || got["engagementEndDate"] != end {
		t.Errorf("engagement dates = %v/%v, want %q/%q",
			got["engagementStartDate"], got["engagementEndDate"], start, end)
	}
	if got["hasAutoClosed"] != true {
		t.Errorf("hasAutoClosed = %v, want true", got["hasAutoClosed"])
	}
	cb, ok := got["closedBy"].(map[string]any)
	if !ok {
		t.Fatalf("closedBy = %v, want an object", got["closedBy"])
	}
	if cb["id"] != "user-1" {
		t.Errorf("closedBy.id = %v, want user-1", cb["id"])
	}
	// CloseNotes is read off the upstream CaseView (it's still decoded above)
	// but must never reach the customer-facing response: it's an internal
	// CS-agent close note, not something a customer should see.
	if _, present := got["closeNotes"]; present {
		t.Errorf("closeNotes = %v, want omitted from the customer-facing response", got["closeNotes"])
	}
}

// TestMapCaseDetails_TrimsFieldsWithNoConsumer pins the deliberate boundary:
// acknowledgedBy and engagementPaymentType are decoded upstream for parity with
// the Ballerina entity-service, but must NOT reach the customer-facing response
// while no frontend consumer exists — per CLAUDE.md's "restrict, don't mirror"
// rule. The fix-ETA quartet is trimmed for the stronger reason that it is
// CSM-internal.
func TestMapCaseDetails_TrimsFieldsWithNoConsumer(t *testing.T) {
	now := time.Now()
	raw, err := json.Marshal(MapCaseDetails(entity.CaseView{
		AcknowledgedBy:        &entity.EntityRef{ID: "user-2", Name: "Acker"},
		EngagementPaymentType: strPtr("Prepaid"),
		WorkState:             strPtr("In Progress"),
		ResolutionCode:        strPtr("Solved"),
		Cause:                 strPtr("Bug"),
		FixEta:                &now,
		ResolutionNotes:       strPtr("Some internal resolution notes"),
		LinkedServiceRequests: []entity.LinkedServiceRequestRef{{ID: "lsr-1", Number: "SR1001", Name: "Service Req"}},
		AssignedEngineer:      &entity.AssignedEngineerRef{ID: "eng-1", Name: "Engineer", Email: strPtr("engineer@example.com")},
	}))
	if err != nil {
		t.Fatalf("marshal returned error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}

	// tags used to be on this denylist too, purely because nothing mapped it
	// yet, not because it's internal — a tag (e.g. "Security Announcement",
	// attached to every case a security announcement creates) is written
	// specifically to be customer-visible. See TestMapCaseDetails_ExposesTags.
	for _, k := range []string{
		"acknowledgedBy", "engagementPaymentType", "bestCaseFixEta", "mostLikelyFixEta", "worstCaseFixEta",
		"workState", "resolutionCode", "cause", "fixEta", "linkedServiceRequests", "resolutionNotes", "engineerEmail",
	} {
		if _, present := got[k]; present {
			t.Errorf("%q leaked into the customer-facing case response", k)
		}
	}
}

// TestMapCaseDetails_ExposesTags confirms a case's tags (e.g. the mandatory
// "Security Announcement" label a security announcement's cases carry) reach
// the customer-facing response — see this file's own TrimsFieldsWithNoConsumer
// test, which used to (accidentally) exclude "tags" for lack of a mapping,
// not because it's internal.
func TestMapCaseDetails_ExposesTags(t *testing.T) {
	got := MapCaseDetails(entity.CaseView{
		Tags: []entity.Tag{{ID: "tag-1", Label: "Security Announcement", Color: strPtr("#ff0000")}},
	})
	if len(got.Tags) != 1 || got.Tags[0].Label != "Security Announcement" {
		t.Fatalf("expected the case's own tags to be exposed, got %+v", got.Tags)
	}
}

// TestMapCaseDetails_ExposesWatchListLocked is the regression test for the
// "4 mandatory account stakeholders can never be removed" feature: the
// frontend needs entity-service's own WatchListUser.Locked flag to disable
// the remove control for those specific watchers, and it must not be lost
// in the entity -> dto mapping.
func TestMapCaseDetails_ExposesWatchListLocked(t *testing.T) {
	got := MapCaseDetails(entity.CaseView{
		WatchList: []entity.WatchListUser{
			{ID: "stakeholder-1", Email: "csm@example.com", Locked: true},
			{ID: "customer-pick-1", Email: "jane.doe@example.com", Locked: false},
		},
	})
	if len(got.WatchList) != 2 {
		t.Fatalf("expected 2 watchers, got %d", len(got.WatchList))
	}
	if !got.WatchList[0].Locked {
		t.Errorf("expected the account stakeholder watcher to be Locked, got %+v", got.WatchList[0])
	}
	if got.WatchList[1].Locked {
		t.Errorf("expected the customer-picked watcher to NOT be Locked, got %+v", got.WatchList[1])
	}
}

// TestMapCaseDetails_OmitsAbsentFields checks a case response carrying none of
// these values omits the keys rather than emitting nulls or zero values.
func TestMapCaseDetails_OmitsAbsentFields(t *testing.T) {
	raw, err := json.Marshal(MapCaseDetails(entity.CaseView{}))
	if err != nil {
		t.Fatalf("marshal returned error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	for _, k := range []string{"slaResponseTime", "closedBy", "closeNotes", "hasAutoClosed", "engagementStartDate", "engagementEndDate"} {
		if _, present := got[k]; present {
			t.Errorf("%q present with no upstream value; want omitted", k)
		}
	}
}

func strPtr(s string) *string { return &s }

func TestMapCaseDetails_MapsLinkedChangeRequestsWithNumber(t *testing.T) {
	subject := "Test Change Request Subject"
	cv := entity.CaseView{
		LinkedChangeRequests: []entity.LinkedChangeRequestRef{
			{
				ID:     "cr-uuid-1",
				Number: "CHG0039181",
				Name:   &subject,
			},
			{
				ID:     "cr-uuid-2",
				Number: "CHG0039182",
				Name:   nil,
			},
		},
	}
	details := MapCaseDetails(cv)
	if len(details.ChangeRequests) != 2 {
		t.Fatalf("expected 2 change requests, got %d", len(details.ChangeRequests))
	}

	cr1 := details.ChangeRequests[0]
	if cr1.ID != "cr-uuid-1" {
		t.Errorf("expected ID 'cr-uuid-1', got %q", cr1.ID)
	}
	if cr1.Label != subject {
		t.Errorf("expected Label %q, got %q", subject, cr1.Label)
	}
	if cr1.Number != "CHG0039181" {
		t.Errorf("expected Number 'CHG0039181', got %q", cr1.Number)
	}

	cr2 := details.ChangeRequests[1]
	if cr2.ID != "cr-uuid-2" {
		t.Errorf("expected ID 'cr-uuid-2', got %q", cr2.ID)
	}
	if cr2.Label != "CHG0039182" {
		t.Errorf("expected Label to fall back to number 'CHG0039182', got %q", cr2.Label)
	}
	if cr2.Number != "CHG0039182" {
		t.Errorf("expected Number 'CHG0039182', got %q", cr2.Number)
	}
}
