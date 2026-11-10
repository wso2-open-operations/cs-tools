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
	"strings"
	"testing"
	"time"
)

const (
	// The shape the legacy process saved, and ACP's own (no action results).
	savedLegacyOK     = `{"event_type":"suspend","actionSendEmailNotification":"SUCCESSFUL","actionServicePortalAnnouncement":"SUCCESSFUL","actionSuspendProject":"SUCCESSFUL"}`
	savedLegacyFailed = `{"event_type":"suspend","actionSendEmailNotification":"SUCCESSFUL","actionSuspendProject":"FAILED"}`
	savedACP          = `{"event_type":"suspend","suspended_on":"2026-05-01"}`
)

func savedState(endDateSection, invoiceSection string) json.RawMessage {
	parts := []string{}
	if endDateSection != "" {
		parts = append(parts, `"based_on_subscription_end_date":`+endDateSection)
	}
	if invoiceSection != "" {
		parts = append(parts, `"based_on_due_invoices":`+invoiceSection)
	}
	return json.RawMessage("{" + strings.Join(parts, ",") + "}")
}

// A track whose saved event is a completed suspension does nothing when the
// decision is to suspend, as the legacy process does not repeat an event it
// completed: a project a person set back to Open is not suspended again.
func TestProcessProject_SavedSuspendIsNotRepeated(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	for _, section := range []string{savedLegacyOK, savedACP} {
		t.Run("end date "+section, func(t *testing.T) {
			end := now // day 0
			proj := project{
				ID: "p1", Name: "Example Project", Account: &projectAccountRef{ID: "a1"},
				EndDate: &end, ClosureState: strPtr("Open"),
				SuspensionProcessState: savedState(section, ""),
			}
			updater, ntf := &mockProjectUpdater{}, delivering()
			if _, err := processProject(context.Background(), &mockEntityReader{}, updater, ntf, now, proj); err != nil {
				t.Fatalf("processProject() error = %v", err)
			}
			if len(ntf.sent) != 0 || len(updater.calls) != 0 || len(updater.checks) != 0 || len(updater.sfSuspendCalls) != 0 {
				t.Errorf("sent=%d writes=%d probes=%d sfSuspends=%d, want nothing", len(ntf.sent), len(updater.calls), len(updater.checks), len(updater.sfSuspendCalls))
			}
		})
		t.Run("invoice "+section, func(t *testing.T) {
			proj := project{
				ID: "p1", Name: "Example Project", Account: &projectAccountRef{ID: "a1"},
				ClosureState:           strPtr("Open"),
				SuspensionProcessState: savedState("", section),
			}
			updater, ntf := &mockProjectUpdater{}, delivering()
			if _, err := processProject(context.Background(), invoiceLinkedReader(t, now), updater, ntf, now, proj); err != nil {
				t.Fatalf("processProject() error = %v", err)
			}
			if len(ntf.sent) != 0 || len(updater.calls) != 0 || len(updater.checks) != 0 || len(updater.sfSuspendCalls) != 0 {
				t.Errorf("sent=%d writes=%d probes=%d sfSuspends=%d, want nothing", len(ntf.sent), len(updater.calls), len(updater.checks), len(updater.sfSuspendCalls))
			}
		})
	}
}

// A suspension the legacy process saved as failed was not completed: ACP acts.
func TestProcessProject_SavedSuspendFailedActsAsBefore(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := now
	proj := project{
		ID: "p1", Name: "Example Project", Account: &projectAccountRef{ID: "a1"},
		EndDate: &end, ClosureState: strPtr("Open"),
		SuspensionProcessState: savedState(savedLegacyFailed, ""),
	}
	updater, ntf := &mockProjectUpdater{}, delivering()
	if _, err := processProject(context.Background(), &mockEntityReader{}, updater, ntf, now, proj); err != nil {
		t.Fatalf("processProject() error = %v", err)
	}
	if !wroteField(updater.calls, "endDateClosureState") {
		t.Errorf("writes = %+v, want the end-date track suspended", updater.calls)
	}
}

// No saved record: unchanged (notice and suspension).
func TestProcessProject_NoSavedRecordUnchanged(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := now
	proj := project{
		ID: "p1", Name: "Example Project", Account: &projectAccountRef{ID: "a1"},
		EndDate: &end, ClosureState: strPtr("Open"),
	}
	updater, ntf := &mockProjectUpdater{}, delivering()
	if _, err := processProject(context.Background(), &mockEntityReader{}, updater, ntf, now, proj); err != nil {
		t.Fatalf("processProject() error = %v", err)
	}
	if len(ntf.sent) == 0 || !wroteField(updater.calls, "endDateClosureState") {
		t.Errorf("sent=%d writes=%+v, want notice and suspension", len(ntf.sent), updater.calls)
	}
}

// A saved suspension on the end-date track must not stop the other Open,
// due track from notifying and suspending: the skipped track closed nothing.
func TestProcessProject_SavedSuspendDoesNotSilenceOtherTrack(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := now.AddDate(0, 0, -5) // more overdue, so it would sort first
	proj := project{
		ID: "p1", Name: "Example Project", Account: &projectAccountRef{ID: "a1"},
		EndDate: &end, ClosureState: strPtr("Open"),
		SuspensionProcessState: savedState(savedLegacyOK, ""),
	}
	updater, ntf := &mockProjectUpdater{}, delivering()
	if _, err := processProject(context.Background(), invoiceLinkedReader(t, now), updater, ntf, now, proj); err != nil {
		t.Fatalf("processProject() error = %v", err)
	}
	if len(ntf.sent) == 0 {
		t.Error("no invoice notice sent, want the other track to notify before it suspends")
	}
	if !wroteField(updater.calls, "invoiceDueDateClosureState") || wroteField(updater.calls, "endDateClosureState") {
		t.Errorf("writes = %+v, want the invoice track suspended and the end-date track untouched", updater.calls)
	}
}

// A saved suspension whose date moved more than 90 days away still resets to
// open, which clears the guard.
func TestProcessProject_SavedSuspendResetsOnRenewal(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := now.AddDate(0, 0, 120)
	proj := project{
		ID: "p1", Name: "Example Project", Account: &projectAccountRef{ID: "a1"},
		EndDate: &end, ClosureState: strPtr("Open"),
		SuspensionProcessState: savedState(savedLegacyOK, ""),
	}
	updater, ntf := &mockProjectUpdater{}, delivering()
	if _, err := processProject(context.Background(), &mockEntityReader{}, updater, ntf, now, proj); err != nil {
		t.Fatalf("processProject() error = %v", err)
	}
	if len(ntf.sent) != 0 || len(updater.calls) != 1 || !strings.Contains(string(updater.calls[0].body), `"event_type":"open"`) {
		t.Errorf("sent=%d writes=%+v, want one reset to open", len(ntf.sent), updater.calls)
	}
}
