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
package dispatch

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
)

func TestCustomerStatus(t *testing.T) {
	for in, want := range map[string]string{
		"Work In Progress":  "In Progress",
		"work_in_progress":  "In Progress",
		"Waiting on WSO2":   "In Progress",
		"Waiting On WSO2":   "In Progress",
		"WAITING_ON_WSO2":   "In Progress",
		"Awaiting Info":     "Awaiting Info",
		"Closed":            "Closed",
		"Solution Proposed": "Solution Proposed",
		"":                  "",
	} {
		if got := customerStatus(in); got != want {
			t.Errorf("customerStatus(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCustomerSeesStatusChange(t *testing.T) {
	cases := []struct {
		previous, next string
		want           bool
	}{
		{"Work In Progress", "Waiting on WSO2", false},
		{"Waiting on WSO2", "Work In Progress", false},
		{"Awaiting Info", "Waiting on WSO2", true},
		{"Solution Proposed", "Waiting on WSO2", true},
		{"Waiting on WSO2", "Closed", true},
		{"Open", "Work In Progress", true},
		{"", "Waiting on WSO2", true},
	}
	for _, c := range cases {
		if got := customerSeesStatusChange(c.previous, c.next); got != c.want {
			t.Errorf("customerSeesStatusChange(%q, %q) = %v, want %v", c.previous, c.next, got, c.want)
		}
	}
}

const (
	customerEmail = "customer@example.com"
	staffEmail    = "staff@example.com"
)

// newSplitAudienceDispatcher resolves customerEmail to a customer-portal link
// and everyone else to the CSM link, so one event produces two emails.
func newSplitAudienceDispatcher(mock *mockEmailSender) *Dispatcher {
	links := &mockLinkResolver{linkFor: func(email string) string {
		if email == customerEmail {
			return "https://customer.example/cases/CASE-1"
		}
		return "https://csm.example/cases/CASE-1"
	}}
	return NewDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{}, links, true, false, nil, true, "", nil)
}

func statusChangedRecord(previous, next string) eventbus.Record {
	prev := ""
	if previous != "" {
		prev = fmt.Sprintf(`"previousStatus":%q,`, previous)
	}
	return eventbus.Record{Value: []byte(fmt.Sprintf(`{"type":"case.status_changed","entityId":"CASE-1","payload":{"projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"Login error",`+prev+`"newStatus":%q,"recipients":[%q,%q]}}`, next, customerEmail, staffEmail))}
}

func emailsByRecipient(mock *mockEmailSender) map[string]string {
	out := map[string]string{}
	for _, c := range mock.calls {
		for _, to := range c.to {
			out[to] = c.htmlBody
		}
	}
	return out
}

// A customer who is told the case went from Awaiting Info to Waiting On WSO2
// sees it become In Progress; staff keep the real status.
func TestDispatcher_Handle_StatusChanged_CustomerSeesInProgress(t *testing.T) {
	mock := &mockEmailSender{}
	d := newSplitAudienceDispatcher(mock)

	if err := d.Handle(context.Background(), statusChangedRecord("Awaiting Info", "Waiting on WSO2")); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	got := emailsByRecipient(mock)
	if !strings.Contains(got[customerEmail], "In Progress") || strings.Contains(got[customerEmail], "Waiting on WSO2") {
		t.Errorf("customer email should say In Progress and not Waiting on WSO2")
	}
	if !strings.Contains(got[staffEmail], "Waiting on WSO2") {
		t.Errorf("staff email should keep the real status")
	}
}

// The reported problem: an engineer picks the case up after the customer's
// reply (Waiting On WSO2 to Work In Progress). The customer already sees In
// Progress, so they must not be emailed again; staff still are.
func TestDispatcher_Handle_StatusChanged_NoCustomerEmailWhenNothingChangedForThem(t *testing.T) {
	for _, move := range [][2]string{{"Waiting on WSO2", "Work In Progress"}, {"Work In Progress", "Waiting on WSO2"}} {
		mock := &mockEmailSender{}
		d := newSplitAudienceDispatcher(mock)

		if err := d.Handle(context.Background(), statusChangedRecord(move[0], move[1])); err != nil {
			t.Fatalf("Handle() error = %v", err)
		}
		got := emailsByRecipient(mock)
		if _, sent := got[customerEmail]; sent {
			t.Errorf("%s -> %s: customer was emailed a change they cannot see", move[0], move[1])
		}
		if _, sent := got[staffEmail]; !sent {
			t.Errorf("%s -> %s: staff must still be emailed", move[0], move[1])
		}
	}
}

// Without a previous status there is nothing to compare, so the customer is
// still emailed, with the merged label.
func TestDispatcher_Handle_StatusChanged_NoPreviousStatusStillEmailsCustomer(t *testing.T) {
	mock := &mockEmailSender{}
	d := newSplitAudienceDispatcher(mock)

	if err := d.Handle(context.Background(), statusChangedRecord("", "Waiting on WSO2")); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	got := emailsByRecipient(mock)
	if !strings.Contains(got[customerEmail], "In Progress") {
		t.Errorf("customer should still be emailed, with In Progress")
	}
}

// One email per audience, never two to the same customer.
func TestDispatcher_Handle_StatusChanged_OneEmailPerAudience(t *testing.T) {
	mock := &mockEmailSender{}
	d := newSplitAudienceDispatcher(mock)

	if err := d.Handle(context.Background(), statusChangedRecord("Awaiting Info", "Waiting on WSO2")); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(mock.calls) != 2 {
		t.Fatalf("got %d emails, want 2 (one customer, one staff)", len(mock.calls))
	}
}
