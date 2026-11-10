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

package sweep

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp/salesentity"
)

type fakePoster struct {
	bodies []string
	err    error
}

func (f *fakePoster) PostSalesforceEvent(_ context.Context, body []byte) error {
	f.bodies = append(f.bodies, string(body))
	return f.err
}

// events returns the posted events as "entity:referenceId", sorted.
func (f *fakePoster) events(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, b := range f.bodies {
		var e struct{ EventType, Entity, ReferenceID string }
		if err := json.Unmarshal([]byte(b), &e); err != nil || e.EventType != "UPDATED" {
			t.Fatalf("event %s: %v", b, err)
		}
		out = append(out, e.Entity+":"+e.ReferenceID)
	}
	sort.Strings(out)
	return out
}

// repairWorld is a Salesforce and a CSM copy that agree, to be drifted one
// field at a time. The CSM copy of the project is listed as csm-1.
type repairWorld struct {
	sf          *fakeSalesforce
	csm         project
	invoiceRows string // the CSM copy's invoice listing rows
}

func newRepairWorld() *repairWorld {
	f := accountFilterFixture(sp("Customer"), sp("Enterprise"), nil)
	f.projects["a0dP"] = salesentity.Project{ID: "a0dP", StartDate: sp("2025-07-01"), EndDate: sp("2027-03-31"), CustomerID: sp("001P")}
	f.links = map[string][]salesentity.LinkedOpportunity{"a0dP": {{ID: "l1", OpportunityID: sp("006A")}}}
	proj := openProject("csm-1", "a0dP")
	proj.StartDate = timePtr(time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC))
	proj.EndDate = timePtr(time.Date(2027, 3, 31, 0, 0, 0, 0, time.UTC))
	proj.Account = &projectAccountRef{ID: "acc-csm", Name: "Example Corp", IsPartner: boolPtr(false)}
	proj.SuspensionProcessState = nil
	return &repairWorld{sf: f, csm: proj, invoiceRows: `{"id":"inv-csm","sfId":"a0IA","invoiceDate":"2026-09-01","invoicedDueDate":"2026-09-28","opportunity":{"id":"opp-csm"},"classification":"SUB","name":"US1"}`}
}

func (w *repairWorld) run(t *testing.T, rep *Repair, projects ...project) (Result, *mockProjectUpdater) {
	t.Helper()
	if len(projects) == 0 {
		projects = []project{w.csm}
	}
	reader := csmListing(t, projects...)
	reader.searchInvoicesFn = func(context.Context, []byte) ([]byte, error) {
		return []byte(`{"invoices":[` + w.invoiceRows + `],"total":1,"hasMore":false}`), nil
	}
	updater := &mockProjectUpdater{}
	now := time.Date(2026, 10, 14, 9, 30, 0, 0, time.UTC)
	result, err := RunWith(context.Background(), reader, NewSalesforceDetails(w.sf), updater, delivering(), now, "", nil, Options{Repair: rep})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return result, updater
}

func TestRepair_WhatDriftedPostsOneEventForThatRecord(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(w *repairWorld)
		want   []string
	}{
		{"no drift", func(w *repairWorld) {}, nil},
		{"project start date", func(w *repairWorld) { w.csm.StartDate = timePtr(time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)) }, []string{"Project__c:a0dP"}},
		{"project end date", func(w *repairWorld) { w.csm.EndDate = timePtr(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)) }, []string{"Project__c:a0dP"}},
		{"project end date missing in the copy", func(w *repairWorld) { w.csm.EndDate = nil }, []string{"Project__c:a0dP"}},
		{"account partner flag", func(w *repairWorld) { w.csm.Account.IsPartner = boolPtr(true) }, []string{"Account:001P"}},
		{"account partner flag unknown in the copy", func(w *repairWorld) { w.csm.Account.IsPartner = nil }, nil},
		{"invoice due date", func(w *repairWorld) {
			w.invoiceRows = `{"id":"inv-csm","sfId":"a0IA","invoiceDate":"2026-09-01","invoicedDueDate":"2026-12-31","opportunity":{"id":"opp-csm"}}`
		}, []string{"Invoice__c:a0IA"}},
		{"invoice paid date", func(w *repairWorld) {
			w.invoiceRows = `{"id":"inv-csm","sfId":"a0IA","invoiceDate":"2026-09-01","invoicedDueDate":"2026-09-28","invoicedPaidDate":"2026-10-01","opportunity":{"id":"opp-csm"}}`
		}, []string{"Invoice__c:a0IA"}},
		{"invoice date", func(w *repairWorld) {
			w.invoiceRows = `{"id":"inv-csm","sfId":"a0IA","invoiceDate":"2026-08-01","invoicedDueDate":"2026-09-28","opportunity":{"id":"opp-csm"}}`
		}, []string{"Invoice__c:a0IA"}},
		{"invoice absent from the copy", func(w *repairWorld) {
			w.invoiceRows = `{"id":"other","sfId":"a0IZ","invoiceDate":"2026-09-01","invoicedDueDate":"2026-09-28"}`
		}, []string{"Invoice__c:a0IA"}},
		{"unrelated invoice fields (name, classification, amount)", func(w *repairWorld) {
			w.invoiceRows = `{"id":"inv-csm","sfId":"a0IA","invoiceDate":"2026-09-01","invoicedDueDate":"2026-09-28","classification":"XX","name":"Other","invoicedAmount":"9"}`
		}, nil},
		{"unrelated project fields (name, key)", func(w *repairWorld) { w.csm.Name, w.csm.ProjectKey = "Renamed", "NEWKEY" }, nil},
		{"several records drifted", func(w *repairWorld) {
			w.csm.EndDate = nil
			w.csm.Account.IsPartner = boolPtr(true)
			w.invoiceRows = `{"id":"x","sfId":"a0IA","invoiceDate":"2026-09-01","invoicedDueDate":"2030-01-01"}`
		}, []string{"Account:001P", "Invoice__c:a0IA", "Project__c:a0dP"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newRepairWorld()
			tc.mutate(w)
			poster := &fakePoster{}
			result, _ := w.run(t, &Repair{Poster: poster, MaxEvents: 50})
			got := poster.events(t)
			if len(got) != len(tc.want) {
				t.Fatalf("events = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("events = %v, want %v", got, tc.want)
				}
			}
			if result.ProjectsActed != 1 {
				t.Errorf("acted = %d, want 1: the repair must not change the decision", result.ProjectsActed)
			}
		})
	}
}

func TestRepair_DecisionIsTheSameWithAndWithoutIt(t *testing.T) {
	w := newRepairWorld()
	w.csm.EndDate = nil
	off, offUp := w.run(t, nil)
	on, onUp := w.run(t, &Repair{Poster: &fakePoster{}, MaxEvents: 10})
	if off.ProjectsActed != on.ProjectsActed || len(offUp.calls) != len(onUp.calls) {
		t.Errorf("off acted=%d writes=%d, on acted=%d writes=%d", off.ProjectsActed, len(offUp.calls), on.ProjectsActed, len(onUp.calls))
	}
}

func TestRepair_OneEventPerRecordPerRun(t *testing.T) {
	w := newRepairWorld()
	w.csm.EndDate = nil
	w.csm.Account.IsPartner = boolPtr(true)
	second := w.csm
	second.ID = "csm-2" // another CSM project onto the same Salesforce project and account
	poster := &fakePoster{}
	w.run(t, &Repair{Poster: poster, MaxEvents: 50}, w.csm, second)
	if got := poster.events(t); len(got) != 2 {
		t.Errorf("events = %v, want one for the project and one for the account", got)
	}
}

func TestRepair_MaxEventsBoundsTheRun(t *testing.T) {
	w := newRepairWorld()
	w.csm.EndDate = nil
	w.csm.Account.IsPartner = boolPtr(true)
	w.invoiceRows = `{"id":"x","sfId":"a0IA","invoiceDate":"2026-09-01","invoicedDueDate":"2030-01-01"}`
	poster := &fakePoster{}
	w.run(t, &Repair{Poster: poster, MaxEvents: 2})
	if len(poster.bodies) != 2 {
		t.Errorf("posted %d events, want the bound of 2", len(poster.bodies))
	}
	off := &fakePoster{}
	w.run(t, &Repair{Poster: off, MaxEvents: 0})
	if len(off.bodies) != 0 {
		t.Errorf("MaxEvents 0 posted %d events, want repair off", len(off.bodies))
	}
}

func TestRepair_AFailedPostIsIgnored(t *testing.T) {
	w := newRepairWorld()
	w.csm.EndDate = nil
	poster := &fakePoster{err: errors.New("entity-service unavailable")}
	result, updater := w.run(t, &Repair{Poster: poster, MaxEvents: 5})
	if len(result.Failures) != 0 || len(result.Skipped) != 0 || result.ProjectsActed != 1 || len(updater.calls) == 0 {
		t.Errorf("result = %+v, writes = %d: a failed event must not affect the run", result, len(updater.calls))
	}
	if len(poster.bodies) != 1 {
		t.Errorf("attempts = %d, want 1", len(poster.bodies))
	}
}

func TestRepair_DryRunPostsNothing(t *testing.T) {
	w := newRepairWorld()
	w.csm.EndDate = nil
	poster := &fakePoster{}
	w.run(t, &Repair{Poster: poster, MaxEvents: 5, DryRun: true})
	if len(poster.bodies) != 0 {
		t.Errorf("a dry run posted %v", poster.bodies)
	}
}

// A project the filters skip, or screening leaves out, is never compared.
func TestRepair_FilteredAndScreenedOutProjectsAreNotCompared(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(w *repairWorld, proj *project)
	}{
		{"partner account", func(w *repairWorld, _ *project) {
			cu := w.sf.customers["001P"]
			cu.AccountClassification = sp("Partner")
			w.sf.customers["001P"] = cu
		}},
		{"not a customer", func(w *repairWorld, _ *project) {
			cu := w.sf.customers["001P"]
			cu.Status = sp("Prospect")
			w.sf.customers["001P"] = cu
		}},
		{"overridden", func(_ *repairWorld, p *project) { p.SuspensionProcessOverridden = boolPtr(true) }},
		{"ineligible type", func(_ *repairWorld, p *project) { p.SubscriptionType = sp("internal") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newRepairWorld()
			w.csm.EndDate = nil
			tc.mutate(w, &w.csm)
			poster := &fakePoster{}
			w.run(t, &Repair{Poster: poster, MaxEvents: 5})
			if len(poster.bodies) != 0 {
				t.Errorf("events = %v, want none for a skipped project", poster.bodies)
			}
		})
	}
	t.Run("screened out", func(t *testing.T) {
		w := newRepairWorld()
		far := w.csm
		far.EndDate = timePtr(time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)) // far, nothing due
		w.invoiceRows = ``
		poster := &fakePoster{}
		reader := csmListing(t, far)
		reader.searchInvoicesFn = func(context.Context, []byte) ([]byte, error) {
			return []byte(`{"invoices":[],"total":0,"hasMore":false}`), nil
		}
		now := time.Date(2026, 10, 14, 9, 30, 0, 0, time.UTC)
		result, err := RunWith(context.Background(), reader, NewSalesforceDetails(w.sf), &mockProjectUpdater{}, delivering(), now, "", nil,
			Options{Screening: true, Repair: &Repair{Poster: poster, MaxEvents: 5}})
		if err != nil || result.ProjectsScreenedOut != 1 || len(poster.bodies) != 0 {
			t.Errorf("err=%v screenedOut=%d events=%v, want a screened-out project left alone", err, result.ProjectsScreenedOut, poster.bodies)
		}
	})
}
