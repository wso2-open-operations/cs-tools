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
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp/closure"
)

// A track whose own closure state is not Open must do nothing: suspend would
// skip it, so a day-0 notice would tell the customer the project is
// suspended when nothing is. The project is Open overall (a person reopened
// it, say), each track keeps whatever state it was left in.
func TestProcessProject_NonOpenTrackDoesNothing(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	for _, state := range []string{"Pending Suspended", "Pending Closed", "Notified", "Closed", "Suspended"} {
		t.Run("end date "+state, func(t *testing.T) {
			end := now // day 0: notify and suspend
			proj := project{
				ID: "p1", Name: "Example Project", Account: &projectAccountRef{ID: "a1"},
				EndDate: &end, ClosureState: strPtr("Open"),
				EndDateClosureState: strPtr(state),
			}
			updater, ntf := &mockProjectUpdater{}, &mockNotifier{}
			_, err := processProject(context.Background(), &mockEntityReader{}, updater, ntf, now, proj)
			if err != nil {
				t.Fatalf("processProject() error = %v", err)
			}
			if len(ntf.sent) != 0 || len(updater.calls) != 0 || len(updater.checks) != 0 || len(updater.sfSuspendCalls) != 0 {
				t.Errorf("sent=%d writes=%d probes=%d sfSuspends=%d, want nothing", len(ntf.sent), len(updater.calls), len(updater.checks), len(updater.sfSuspendCalls))
			}
		})
		t.Run("invoice "+state, func(t *testing.T) {
			reader := invoiceLinkedReader(t, now)
			proj := project{
				ID: "p1", Name: "Example Project", Account: &projectAccountRef{ID: "a1"},
				ClosureState: strPtr("Open"), InvoiceDueDateClosureState: strPtr(state),
			}
			updater, ntf := &mockProjectUpdater{}, &mockNotifier{}
			_, err := processProject(context.Background(), reader, updater, ntf, now, proj)
			if err != nil {
				t.Fatalf("processProject() error = %v", err)
			}
			if len(ntf.sent) != 0 || len(updater.calls) != 0 || len(updater.checks) != 0 || len(updater.sfSuspendCalls) != 0 {
				t.Errorf("sent=%d writes=%d probes=%d sfSuspends=%d, want nothing", len(ntf.sent), len(updater.calls), len(updater.checks), len(updater.sfSuspendCalls))
			}
		})
	}
}

// The act functions are guarded too, for a caller that reaches them without
// the cascade builders.
func TestActFunctions_NonOpenTrackDoNothing(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	day0 := closure.Decision{Fires: true, ShouldNotify: true, ShouldSuspend: true}
	proj := project{
		ID: "p1", Account: &projectAccountRef{ID: "a1"},
		EndDateClosureState: strPtr("Pending Suspended"), InvoiceDueDateClosureState: strPtr("Notified"),
	}
	updater, ntf := &mockProjectUpdater{}, &mockNotifier{}
	history := proj.SuspensionProcessState
	if err := actSubscription(context.Background(), &mockEntityReader{}, updater, ntf, proj, day0, false, &history, now); err != nil {
		t.Fatalf("actSubscription: %v", err)
	}
	if err := actInvoice(context.Background(), &mockEntityReader{}, updater, ntf, proj, day0, dueInvoice{}, false, &history, now); err != nil {
		t.Fatalf("actInvoice: %v", err)
	}
	if len(ntf.sent) != 0 || len(updater.calls) != 0 || len(updater.checks) != 0 {
		t.Errorf("sent=%d writes=%d probes=%d, want nothing", len(ntf.sent), len(updater.calls), len(updater.checks))
	}
}

// A non-Open end-date track must not stop the Open invoice track from
// notifying before it suspends: the skipped cascade closed nothing, so it
// must not mark the project closed for the other one.
func TestProcessProject_NonOpenEndDateTrackDoesNotSilenceInvoiceNotice(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := now.AddDate(0, 0, -5) // more overdue than the invoice, so it would sort first
	reader := invoiceLinkedReader(t, now)
	proj := project{
		ID: "p1", Name: "Example Project", Account: &projectAccountRef{ID: "a1"},
		EndDate: &end, ClosureState: strPtr("Open"),
		EndDateClosureState: strPtr("Pending Suspended"), InvoiceDueDateClosureState: strPtr("Open"),
	}
	updater, ntf := &mockProjectUpdater{}, delivering()
	if _, err := processProject(context.Background(), reader, updater, ntf, now, proj); err != nil {
		t.Fatalf("processProject() error = %v", err)
	}
	if len(ntf.sent) == 0 {
		t.Error("no invoice notice sent, want the Open invoice track to notify before it suspends")
	}
	if !wroteField(updater.calls, "invoiceDueDateClosureState") || wroteField(updater.calls, "endDateClosureState") {
		t.Errorf("writes = %+v, want the invoice track suspended and the end-date track untouched", updater.calls)
	}
	for _, c := range updater.calls {
		if strings.Contains(string(c.body), "based_on_subscription_end_date") {
			t.Errorf("a write recorded the end-date track, which is not Open: %s", c.body)
		}
	}
}

// An Open track behaves as before: notice, record, suspend.
func TestProcessProject_OpenTrackUnchanged(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := now
	for _, state := range []*string{strPtr("Open"), nil} {
		proj := project{
			ID: "p1", Name: "Example Project", Account: &projectAccountRef{ID: "a1"},
			EndDate: &end, ClosureState: strPtr("Open"), EndDateClosureState: state,
		}
		updater, ntf := &mockProjectUpdater{}, delivering()
		_, err := processProject(context.Background(), &mockEntityReader{}, updater, ntf, now, proj)
		if err != nil {
			t.Fatalf("processProject() error = %v", err)
		}
		if len(ntf.sent) == 0 || !wroteField(updater.calls, "endDateClosureState") {
			t.Errorf("state %v: sent=%d writes=%+v, want notice and suspension", state, len(ntf.sent), updater.calls)
		}
	}
}
