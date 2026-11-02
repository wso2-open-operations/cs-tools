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
	"strings"
	"testing"
	"time"
)

// day0Project is an Open project whose end date passed three days ago, with
// the day-0 notice already recorded, so processProject goes straight to
// suspend.
func day0Project(sfID *string) project {
	endDate := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)
	open := "Open"
	return project{
		ID:                     "p1",
		SfID:                   sfID,
		Account:                &projectAccountRef{ID: "a1"},
		EndDate:                &endDate,
		ClosureState:           &open,
		SuspensionProcessState: []byte(`{"based_on_subscription_end_date":{"event_type":"suspend"}}`),
	}
}

var day0Now = time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)

func strPtr(s string) *string { return &s }

// TestSuspend_WritesSalesforceBeforeCSM: an end-date suspension sets
// Salesforce State__c too, by the project's Salesforce id, before the CSM
// suspension is written (see suspendInSalesforce for why).
func TestSuspend_WritesSalesforceBeforeCSM(t *testing.T) {
	updater := &mockProjectUpdater{}
	updater.sfSuspendFn = func(_ context.Context, _ string) error {
		if len(updater.calls) != 0 {
			t.Errorf("Salesforce written after %d CSM writes, want before the CSM suspension", len(updater.calls))
		}
		return nil
	}

	err := processProject(context.Background(), &mockEntityReader{}, updater, &mockNotifier{}, day0Now, day0Project(strPtr("a0dSF1")))
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}
	if len(updater.sfSuspendCalls) != 1 || updater.sfSuspendCalls[0] != "a0dSF1" {
		t.Errorf("SuspendInSalesforce calls = %v, want [a0dSF1]", updater.sfSuspendCalls)
	}
}

// TestSuspendInvoice_WritesSalesforce: an invoice suspension sets Salesforce
// State__c too.
func TestSuspendInvoice_WritesSalesforce(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	updater := &mockProjectUpdater{}
	proj := project{ID: "p1", SfID: strPtr("a0dSF1"), Name: "Test Project", Account: &projectAccountRef{ID: "a1"}}

	err := processProject(context.Background(), invoiceLinkedReader(t, now), updater, &mockNotifier{}, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}
	var sawInvoiceSuspend bool
	for _, c := range updater.calls {
		var body map[string]json.RawMessage
		if err := json.Unmarshal(c.body, &body); err != nil {
			t.Fatalf("parse update body: %v", err)
		}
		if _, ok := body["invoiceDueDateClosureState"]; ok {
			sawInvoiceSuspend = true
		}
	}
	if !sawInvoiceSuspend {
		t.Fatal("the invoice cascade did not suspend in CSM; the test needs a day-0 invoice")
	}
	if len(updater.sfSuspendCalls) != 1 || updater.sfSuspendCalls[0] != "a0dSF1" {
		t.Errorf("SuspendInSalesforce calls = %v, want [a0dSF1]", updater.sfSuspendCalls)
	}
}

// TestSuspend_CSMFailureFailsTheProject: the CSM failure is reported, after
// Salesforce was already written.
func TestSuspend_CSMFailureFailsTheProject(t *testing.T) {
	updater := &mockProjectUpdater{
		updateProjectFn: func(context.Context, string, []byte) ([]byte, error) { return nil, errors.New("csm down") },
	}

	err := processProject(context.Background(), &mockEntityReader{}, updater, &mockNotifier{}, day0Now, day0Project(strPtr("a0dSF1")))
	if err == nil {
		t.Fatal("processProject() error = nil, want the CSM failure")
	}
	// Salesforce was written first; the project stays Open in CSM, so the
	// next run writes both again (State__c Suspended again is harmless).
	if len(updater.sfSuspendCalls) != 1 {
		t.Errorf("SuspendInSalesforce calls = %v, want one, before the CSM write", updater.sfSuspendCalls)
	}
}

// TestSuspend_SalesforceFailureFailsTheProject: the project is reported as
// failed with nothing written in CSM, so it is still Open there and the next
// run tries again.
func TestSuspend_SalesforceFailureFailsTheProject(t *testing.T) {
	updater := &mockProjectUpdater{
		sfSuspendFn: func(context.Context, string) error { return errors.New("sales-entity down") },
	}

	err := processProject(context.Background(), &mockEntityReader{}, updater, &mockNotifier{}, day0Now, day0Project(strPtr("a0dSF1")))
	if err == nil || !strings.Contains(err.Error(), "sales-entity down") {
		t.Fatalf("processProject() error = %v, want the Salesforce failure", err)
	}
	if !strings.Contains(err.Error(), "Salesforce") {
		t.Errorf("error = %q, want it to say the Salesforce write failed", err)
	}
	if len(updater.calls) != 0 {
		t.Errorf("CSM writes = %d, want none (the project stays Open for the next run)", len(updater.calls))
	}
}

// TestSuspend_AlreadySuspendedSkipsSalesforce: the guard that skips the CSM
// write skips the Salesforce write too.
func TestSuspend_AlreadySuspendedSkipsSalesforce(t *testing.T) {
	updater := &mockProjectUpdater{}
	proj := day0Project(strPtr("a0dSF1"))
	proj.EndDateClosureState = strPtr("Closed")

	if err := processProject(context.Background(), &mockEntityReader{}, updater, &mockNotifier{}, day0Now, proj); err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}
	if len(updater.sfSuspendCalls) != 0 {
		t.Errorf("SuspendInSalesforce calls = %v, want none", updater.sfSuspendCalls)
	}
}

// TestNoticeOnly_SkipsSalesforce: a notice before day 0 writes nothing to
// Salesforce.
func TestNoticeOnly_SkipsSalesforce(t *testing.T) {
	updater := &mockProjectUpdater{}
	proj := day0Project(strPtr("a0dSF1"))
	endDate := day0Now.AddDate(0, 0, 30)
	proj.EndDate = &endDate
	proj.SuspensionProcessState = nil

	if err := processProject(context.Background(), &mockEntityReader{}, updater, &mockNotifier{}, day0Now, proj); err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}
	if len(updater.calls) == 0 {
		t.Fatal("no notice was recorded; the test needs a notice window")
	}
	if len(updater.sfSuspendCalls) != 0 {
		t.Errorf("SuspendInSalesforce calls = %v, want none", updater.sfSuspendCalls)
	}
}

type fakeCSMWriter struct{ calls []updateCall }

func (f *fakeCSMWriter) UpdateProject(_ context.Context, id string, body []byte) ([]byte, error) {
	f.calls = append(f.calls, updateCall{id: id, body: body})
	return []byte(`{}`), nil
}

type statusCall struct{ id, status string }

type fakeSalesforceWriter struct{ calls []statusCall }

func (f *fakeSalesforceWriter) UpdateProjectStatus(_ context.Context, id, status string) error {
	f.calls = append(f.calls, statusCall{id: id, status: status})
	return nil
}

// TestWriter: CSM writes go to CSM; a Salesforce suspension writes status
// "Suspended" by the Salesforce id, and a blank id is an error, not a call.
func TestWriter(t *testing.T) {
	csm, sf := &fakeCSMWriter{}, &fakeSalesforceWriter{}
	w := NewWriter(csm, sf)

	if _, err := w.UpdateProject(context.Background(), "p1", []byte(`{"a":1}`)); err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}
	if len(csm.calls) != 1 || csm.calls[0].id != "p1" || string(csm.calls[0].body) != `{"a":1}` {
		t.Errorf("CSM calls = %+v", csm.calls)
	}

	if err := w.SuspendInSalesforce(context.Background(), "a0dSF1"); err != nil {
		t.Fatalf("SuspendInSalesforce: %v", err)
	}
	if len(sf.calls) != 1 || sf.calls[0] != (statusCall{id: "a0dSF1", status: "Suspended"}) {
		t.Errorf("Salesforce calls = %+v, want [{a0dSF1 Suspended}]", sf.calls)
	}

	if err := w.SuspendInSalesforce(context.Background(), "  "); err == nil {
		t.Error("blank Salesforce id: err = nil, want an error")
	}
	if len(sf.calls) != 1 {
		t.Errorf("Salesforce calls = %d after a blank id, want still 1", len(sf.calls))
	}
}

// TestDryRun_SkipsSalesforce: a dry run writes nothing to Salesforce either.
func TestDryRun_SkipsSalesforce(t *testing.T) {
	if err := (&DryRunProjectUpdater{}).SuspendInSalesforce(context.Background(), "a0dSF1"); err != nil {
		t.Errorf("SuspendInSalesforce: %v, want nil", err)
	}
}
