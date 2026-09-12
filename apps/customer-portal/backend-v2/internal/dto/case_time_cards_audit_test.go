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

func sp(v string) *string { return &v }

// TestMapCaseTimeCardSearchResponse_EmitsCaseAuditFields is the regression guard
// for the time-tracking card rendering without its author.
//
// TimeTrackingCard.tsx renders "Created by <email>" from case.createdBy. The
// field was declared on no Go struct — not in entity-service's decode, not in
// its domain view, not in backend-v2's mirror — so encoding/json discarded it
// silently at the first hop and the card showed only a number and a title.
func TestMapCaseTimeCardSearchResponse_EmitsCaseAuditFields(t *testing.T) {
	out := MapCaseTimeCardSearchResponse(entity.SearchCaseTimeCardsResponse{
		Cases: []entity.CaseTimeCardSummary{{
			Case: entity.CaseTimeCardCaseRef{
				ID:        "70d63bca-3bd2-0310-9140-4c6aa5e45a37",
				Number:    "CS0440883",
				Name:      "Combined-field PATCH test case title",
				UpdatedOn: "2026-08-06 17:26:23",
				CreatedOn: sp("2026-07-27 08:39:48"),
				CreatedBy: sp("jane.doe@example.com"),
				UpdatedBy: sp("john.smith@example.com"),
			},
			TotalTime:  450,
			TotalCount: 1,
			Billable:   entity.CaseTimeCardBillingInfo{TotalTime: 450, Count: 1},
		}},
		Total: 1,
	})

	b, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var probe struct {
		CaseTimeCards []struct {
			Case struct {
				CreatedBy *string `json:"createdBy"`
				CreatedOn *string `json:"createdOn"`
				UpdatedBy *string `json:"updatedBy"`
			} `json:"case"`
			Billable struct {
				Count int `json:"count"`
			} `json:"billable"`
		} `json:"caseTimeCards"`
	}
	if err := json.Unmarshal(b, &probe); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(probe.CaseTimeCards) != 1 {
		t.Fatalf("got %d cards, want 1", len(probe.CaseTimeCards))
	}
	c := probe.CaseTimeCards[0]

	if c.Case.CreatedBy == nil || *c.Case.CreatedBy != "jane.doe@example.com" {
		t.Errorf("case.createdBy = %v, want jane.doe@example.com — the card renders \"Created by\" from this", c.Case.CreatedBy)
	}
	if c.Case.CreatedOn == nil || *c.Case.CreatedOn != "2026-07-27 08:39:48" {
		t.Errorf("case.createdOn = %v", c.Case.CreatedOn)
	}
	if c.Case.UpdatedBy == nil || *c.Case.UpdatedBy != "john.smith@example.com" {
		t.Errorf("case.updatedBy = %v", c.Case.UpdatedBy)
	}
	// The Billable chip is gated on (billable?.count ?? 0) > 0, so the count has
	// to survive the mapping alongside totalTime.
	if c.Billable.Count != 1 {
		t.Errorf("billable.count = %d, want 1 — the Billable chip depends on it", c.Billable.Count)
	}
}

// TestMapCaseTimeCardSearchResponse_OmitsAbsentAuditFields keeps an absent value
// out of the payload rather than sending an empty string, so the card renders no
// author line instead of "Created by ".
func TestMapCaseTimeCardSearchResponse_OmitsAbsentAuditFields(t *testing.T) {
	out := MapCaseTimeCardSearchResponse(entity.SearchCaseTimeCardsResponse{
		Cases: []entity.CaseTimeCardSummary{{
			Case: entity.CaseTimeCardCaseRef{ID: "x", Number: "CS1", Name: "n"},
		}},
	})
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var probe struct {
		CaseTimeCards []struct {
			Case map[string]any `json:"case"`
		} `json:"caseTimeCards"`
	}
	if err := json.Unmarshal(b, &probe); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"createdBy", "createdOn", "updatedBy"} {
		if _, present := probe.CaseTimeCards[0].Case[key]; present {
			t.Errorf("%s present when absent upstream; want it omitted", key)
		}
	}
}
