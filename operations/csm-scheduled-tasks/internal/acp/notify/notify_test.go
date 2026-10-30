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

package notify

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp/recipients"
)

// capturingHandler records every log record passed to it, so tests can
// assert on structured attributes directly rather than parsing formatted
// text output.
type capturingHandler struct {
	records []slog.Record
}

func (h *capturingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *capturingHandler) Handle(_ context.Context, r slog.Record) error {
	h.records = append(h.records, r)
	return nil
}

func (h *capturingHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *capturingHandler) WithGroup(_ string) slog.Handler      { return h }

func attrValue(t *testing.T, r slog.Record, key string) (string, bool) {
	t.Helper()
	var val string
	var found bool
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == key {
			val = a.Value.String()
			found = true
			return false
		}
		return true
	})
	return val, found
}

// TestLoggingNotifier_Send_LogsProjectAndSubjectFields verifies the core
// project-identity and subject-line attributes land in the log record — the
// fields Chamara asked to have visible directly in the logs (project id,
// project name, start date, end date), plus the new Subject that replaces
// the old internal/customer/am_nudge Kind label entirely.
func TestLoggingNotifier_Send_LogsProjectAndSubjectFields(t *testing.T) {
	h := &capturingHandler{}
	n := &LoggingNotifier{Logger: slog.New(h)}

	startDate := time.Date(2025, 7, 29, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)

	_, err := n.Send(context.Background(), Notice{
		ProjectID:   "p1",
		ProjectName: "TICKETNETWORK - Subscription",
		ProjectKey:  "TICKETNET",
		StartDate:   startDate,
		EndDate:     endDate,
		Window:      90,
		Subject:     "[ACP] 90 Days Reminder of Project for TICKETNETWORK - Subscription of TicketNetwork",
	})
	if err != nil {
		t.Fatalf("Send() error = %v, want nil", err)
	}
	if len(h.records) != 1 {
		t.Fatalf("records = %d, want 1", len(h.records))
	}

	wantAttrs := map[string]string{
		"projectID":   "p1",
		"projectName": "TICKETNETWORK - Subscription",
		"projectKey":  "TICKETNET",
		"subject":     "[ACP] 90 Days Reminder of Project for TICKETNETWORK - Subscription of TicketNetwork",
	}
	for key, want := range wantAttrs {
		got, found := attrValue(t, h.records[0], key)
		if !found {
			t.Errorf("attribute %q not present in log record", key)
			continue
		}
		if got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

// TestLoggingNotifier_Send_LogsNoPersonalData: the log line must carry no
// email address, no person's name (staff or customer) and no email body,
// masked or not — logs must hold no personal data in any mode (Rashmika's
// review of PR #2134). Every attribute value is checked, not just the ones
// that used to hold recipients, so a new attribute can't reintroduce it.
func TestLoggingNotifier_Send_LogsNoPersonalData(t *testing.T) {
	h := &capturingHandler{}
	n := &LoggingNotifier{Logger: slog.New(h)}

	_, err := n.Send(context.Background(), Notice{
		ProjectID:   "p1",
		ProjectName: "Acme - Subscription",
		Window:      7,
		Subject:     "Upcoming Project Suspension Notice - Acme - Subscription",
		Body:        "Dear Jordan Perera, the project needs renewal.",
		Recipients: Recipients{
			AccountOwner:   recipients.Contact{Name: "Jordan Perera", Email: "jordan.perera@wso2.example"},
			RenewalManager: recipients.Contact{Name: "Sam Jayasuriya", Email: "sam.jayasuriya@wso2.example"},
			TechnicalOwner: recipients.Contact{Name: "Alex Fernando", Email: "alex.fernando@wso2.example"},
			Customers:      []recipients.Contact{{Name: "Bob Silva", Email: "bob@customer.example"}},
		},
		ResolvedVia: recipients.ResolvedViaBusinessContact,
	})
	if err != nil {
		t.Fatalf("Send() error = %v, want nil", err)
	}
	if len(h.records) != 1 {
		t.Fatalf("records = %d, want 1", len(h.records))
	}

	forbidden := []string{"@", "Jordan", "Perera", "Sam", "Jayasuriya", "Alex", "Fernando", "Bob", "Silva", "Dear", "renewal"}
	h.records[0].Attrs(func(a slog.Attr) bool {
		v := a.Value.String()
		for _, f := range forbidden {
			if strings.Contains(v, f) {
				t.Errorf("attribute %s = %q contains %q, want no personal data in the log", a.Key, v, f)
			}
		}
		return true
	})
}

// TestLoggingNotifier_Send_LogsRecipientCounts: instead of who a notice is
// for, the log says how many — the same to/cc split EmailNotifier uses
// (before its standing list and staging filter), plus how many of them are
// customers.
func TestLoggingNotifier_Send_LogsRecipientCounts(t *testing.T) {
	internal := Recipients{
		AccountOwner:   recipients.Contact{Email: "am@wso2.example"},
		TechnicalOwner: recipients.Contact{Email: "to@wso2.example"},
	}
	customer := internal
	customer.Customers = []recipients.Contact{{Email: "a@customer.example"}, {Email: "b@customer.example"}}

	tests := []struct {
		name       string
		recipients Recipients
		want       map[string]string
	}{
		{"internal notice", internal, map[string]string{"toCount": "2", "ccCount": "0", "customerCount": "0"}},
		{"customer notice", customer, map[string]string{"toCount": "2", "ccCount": "2", "customerCount": "2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &capturingHandler{}
			n := &LoggingNotifier{Logger: slog.New(h)}
			if _, err := n.Send(context.Background(), Notice{ProjectID: "p1", Recipients: tt.recipients}); err != nil {
				t.Fatalf("Send() error = %v, want nil", err)
			}
			for key, want := range tt.want {
				got, found := attrValue(t, h.records[0], key)
				if !found || got != want {
					t.Errorf("%s = %q (found %v), want %q", key, got, found, want)
				}
			}
		})
	}
}
