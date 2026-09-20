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
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/acp-closure-service/internal/recipients"
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

// TestLoggingNotifier_Send_LogsRecipientsIncludingCustomerWhenPresent covers
// the structured Recipients attribute: Account Owner/Renewal Manager/
// Technical Owner's names AND emails are always logged (names matter here —
// Renewal Manager and Technical Owner never appear by name anywhere else in
// the log, unlike Account Owner which also shows up in Body), and Customer
// is logged too when present (a resolved 15/7/0-window customer contact).
func TestLoggingNotifier_Send_LogsRecipientsIncludingCustomerWhenPresent(t *testing.T) {
	h := &capturingHandler{}
	n := &LoggingNotifier{Logger: slog.New(h)}

	_, err := n.Send(context.Background(), Notice{
		ProjectID: "p1",
		Window:    7,
		Recipients: Recipients{
			AccountOwner:   recipients.Contact{Name: "Jordan Perera", Email: "jordan.perera@wso2.example"},
			RenewalManager: recipients.Contact{Name: "Sam Jayasuriya", Email: "sam.jayasuriya@wso2.example"},
			TechnicalOwner: recipients.Contact{Name: "Alex Fernando", Email: "alex.fernando@wso2.example"},
			Customer:       &recipients.Contact{Name: "Bob", Email: "bob@customer.example"},
		},
		ResolvedVia: recipients.ResolvedViaBusinessContact,
	})
	if err != nil {
		t.Fatalf("Send() error = %v, want nil", err)
	}
	if len(h.records) != 1 {
		t.Fatalf("records = %d, want 1", len(h.records))
	}

	wantAttrs := map[string]string{
		"accountOwner":       "jordan.perera@wso2.example",
		"accountOwnerName":   "Jordan Perera",
		"renewalManager":     "sam.jayasuriya@wso2.example",
		"renewalManagerName": "Sam Jayasuriya",
		"technicalOwner":     "alex.fernando@wso2.example",
		"technicalOwnerName": "Alex Fernando",
		"customer":           "bob@customer.example",
		"customerName":       "Bob",
		"resolvedVia":        string(recipients.ResolvedViaBusinessContact),
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

// TestLoggingNotifier_Send_OmitsCustomerAttributeWhenNil covers the
// internal-only (90/60/30) case: Recipients.Customer is nil, and the log
// must not carry a misleading empty "customer" attribute implying a
// customer was in scope for this notice at all.
func TestLoggingNotifier_Send_OmitsCustomerAttributeWhenNil(t *testing.T) {
	h := &capturingHandler{}
	n := &LoggingNotifier{Logger: slog.New(h)}

	_, err := n.Send(context.Background(), Notice{
		ProjectID: "p1",
		Window:    90,
		Recipients: Recipients{
			AccountOwner: recipients.Contact{Name: "Jordan Perera", Email: "jordan.perera@wso2.example"},
		},
	})
	if err != nil {
		t.Fatalf("Send() error = %v, want nil", err)
	}
	if len(h.records) != 1 {
		t.Fatalf("records = %d, want 1", len(h.records))
	}

	if _, found := attrValue(t, h.records[0], "customer"); found {
		t.Error("customer attribute present in log record, want absent when Recipients.Customer is nil")
	}
	if _, found := attrValue(t, h.records[0], "customerName"); found {
		t.Error("customerName attribute present in log record, want absent when Recipients.Customer is nil")
	}
}

// TestLoggingNotifier_Send_LogsBodyWhenPresent covers the no-business-contact
// notice's Body field, the one notice type that carries one today.
func TestLoggingNotifier_Send_LogsBodyWhenPresent(t *testing.T) {
	h := &capturingHandler{}
	n := &LoggingNotifier{Logger: slog.New(h)}

	const body = "Internal - Customer Project without Business Contacts\n\nUrgent reminder..."

	_, err := n.Send(context.Background(), Notice{
		ProjectID: "p1",
		Subject:   "[Urgent] [ACP] No Business Contacts Specified for Project HFC Subscription - Subscription",
		Body:      body,
	})
	if err != nil {
		t.Fatalf("Send() error = %v, want nil", err)
	}
	if len(h.records) != 1 {
		t.Fatalf("records = %d, want 1", len(h.records))
	}

	got, found := attrValue(t, h.records[0], "body")
	if !found {
		t.Fatal("body attribute not present in log record")
	}
	if got != body {
		t.Errorf("body = %q, want %q", got, body)
	}
}
