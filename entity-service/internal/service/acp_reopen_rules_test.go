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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"encoding/json"
	"testing"
	"time"
)

func acpDate(s string) *time.Time {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return &t
}

func acpStr(s string) *string { return &s }

func TestACPDaysBetween_MatchesTheSweep(t *testing.T) {
	now := time.Date(2026, 10, 29, 9, 30, 0, 0, time.UTC)
	cases := []struct {
		end  string
		want int
	}{
		{"2026-10-29", 0},  // today, already started: suspended
		{"2026-10-30", 1},  // tomorrow: still a day left
		{"2026-10-28", -1}, // yesterday
		{"2027-01-10", 73},
	}
	for _, c := range cases {
		if got := acpDaysBetween(now, *acpDate(c.end)); got != c.want {
			t.Errorf("daysBetween(%s) = %d, want %d", c.end, got, c.want)
		}
	}
}

func TestACPSectionEventType(t *testing.T) {
	blob := json.RawMessage(`{"based_on_subscription_end_date":{"event_type":"suspend","actionSendEmailNotification":"SUCCESSFUL"},"based_on_due_invoices":{"event_type":"30_days_notice"}}`)
	if got, _ := acpSectionEventType(blob, acpEndDateSection); got != "suspend" {
		t.Errorf("end date = %q", got)
	}
	if got, _ := acpSectionEventType(blob, acpInvoiceSection); got != "30_days_notice" {
		t.Errorf("invoice = %q", got)
	}
	for _, empty := range []string{"", "null", "{}"} {
		if got, err := acpSectionEventType(json.RawMessage(empty), acpEndDateSection); err != nil || got != "" {
			t.Errorf("%q: got %q err %v", empty, got, err)
		}
	}
	if _, err := acpSectionEventType(json.RawMessage(`{broken`), acpEndDateSection); err == nil {
		t.Error("a corrupt blob was read without an error")
	}
}

func TestACPWithSectionOpen_KeepsEveryOtherSectionAsItWas(t *testing.T) {
	blob := json.RawMessage(`{"based_on_compliance":{"event_type":"x","note":"a <b> & c"},"based_on_subscription_end_date":{"event_type":"suspend"}}`)
	got, err := acpWithSectionOpen(blob, acpEndDateSection)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"based_on_compliance":{"event_type":"x","note":"a <b> & c"},"based_on_subscription_end_date":{"event_type":"open"}}`
	if string(got) != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
	for _, empty := range []string{"", "null"} {
		got, err := acpWithSectionOpen(json.RawMessage(empty), acpInvoiceSection)
		if err != nil || string(got) != `{"based_on_due_invoices":{"event_type":"open"}}` {
			t.Errorf("%q: got %s err %v", empty, got, err)
		}
	}
}

func TestACPOpportunityCounts(t *testing.T) {
	cases := []struct {
		stage, eula *string
		want        bool
	}{
		{acpStr("50 - Closed Won"), acpStr("EULA 3.3"), true},
		{acpStr("40 - Negotiation"), acpStr("EULA 3.3"), false},
		{nil, acpStr("EULA 3.3"), false},
		{acpStr("50 - Closed Won"), nil, false},
		{acpStr("50 - Closed Won"), acpStr("  "), true}, // present, as in ACP
		{acpStr("50 - Closed Won"), acpStr("Customer contract"), false},
	}
	for _, c := range cases {
		if got := acpOpportunityCounts(c.stage, c.eula); got != c.want {
			t.Errorf("stage=%v eula=%v: got %v", c.stage, c.eula, got)
		}
	}
}

func TestACPEULAVersionDecimal(t *testing.T) {
	cases := map[string]float64{"": 3.4, "WSO2 EULA 3.2": 3.2, "3.4": 3.4, "no version": 0}
	for in, want := range cases {
		if got := acpEULAVersionDecimal(acpStr(in)); got != want {
			t.Errorf("%q = %v, want %v", in, got, want)
		}
	}
	if got := acpEULAVersionDecimal(nil); got != 3.4 {
		t.Errorf("nil = %v", got)
	}
}

func TestACPInvoiceSuspendDate(t *testing.T) {
	inv, due := *acpDate("2026-09-01"), *acpDate("2026-10-01")
	if got := acpInvoiceSuspendDate(inv, due, 3.4, false); !got.Equal(due) {
		t.Errorf("no grace: %v", got)
	}
	if got := acpInvoiceSuspendDate(inv, due, 3.2, false); !got.Equal(*acpDate("2026-10-31")) {
		t.Errorf("EULA 3.2: %v, want invoice date + 60", got)
	}
	if got := acpInvoiceSuspendDate(inv, due, 3.4, true); !got.Equal(*acpDate("2026-10-31")) {
		t.Errorf("primary partner: %v, want invoice date + 60", got)
	}
	longDue := *acpDate("2026-12-01")
	if got := acpInvoiceSuspendDate(inv, longDue, 3.2, false); !got.Equal(longDue) {
		t.Errorf("credit period over 60 days: %v, want the due date", got)
	}
}

func TestACPInvoiceCounts(t *testing.T) {
	start := acpDate("2026-01-01")
	base := acpInvoice{DueDate: acpDate("2026-10-01")}
	if !acpInvoiceCounts(base, start) {
		t.Error("an unpaid invoice due after the start does not count")
	}
	paid := base
	paid.PaidDate = acpDate("2026-10-05")
	noDue := acpInvoice{}
	tam := base
	tam.Classification = acpStr("TAM")
	ps := base
	ps.Name = acpStr("Auto Created PS")
	early := acpInvoice{DueDate: acpDate("2025-12-01")}
	for name, inv := range map[string]acpInvoice{"paid": paid, "no due date": noDue, "TAM": tam, "Auto Created PS": ps, "before start": early} {
		if acpInvoiceCounts(inv, start) {
			t.Errorf("%s counts", name)
		}
	}
	if !acpInvoiceCounts(early, nil) {
		t.Error("with no project start an early invoice should count")
	}
}

func TestACPInvoiceTrackClear(t *testing.T) {
	now := time.Date(2026, 10, 29, 9, 30, 0, 0, time.UTC)
	overdue := acpInvoice{InvoiceDate: acpDate("2026-08-01"), DueDate: acpDate("2026-09-01"), EULAVersion: 3.4}
	notYetDue := acpInvoice{InvoiceDate: acpDate("2026-10-01"), DueDate: acpDate("2026-11-30"), EULAVersion: 3.4}
	paid := overdue
	paid.PaidDate = acpDate("2026-10-28")

	if !acpInvoiceTrackClear(now, nil, nil, false, false) {
		t.Error("no invoices: not clear")
	}
	if !acpInvoiceTrackClear(now, []acpInvoice{paid}, nil, false, false) {
		t.Error("the overdue invoice was paid: not clear")
	}
	if acpInvoiceTrackClear(now, []acpInvoice{overdue}, nil, false, false) {
		t.Error("an overdue invoice: clear")
	}
	if !acpInvoiceTrackClear(now, []acpInvoice{notYetDue}, nil, false, false) {
		t.Error("only a future invoice: not clear")
	}
	// The earliest due invoice decides, as in the sweep.
	if acpInvoiceTrackClear(now, []acpInvoice{notYetDue, overdue}, nil, false, false) {
		t.Error("an overdue invoice among others: clear")
	}
	// A Partner without a primary partner has no invoice track.
	if !acpInvoiceTrackClear(now, []acpInvoice{overdue}, nil, true, false) {
		t.Error("Partner without a primary partner: not clear")
	}
	// A primary partner gives the grace period: due 2026-10-01, invoiced
	// 2026-09-15, suspends on 2026-11-14, so not overdue yet on 29 October.
	grace := acpInvoice{InvoiceDate: acpDate("2026-09-15"), DueDate: acpDate("2026-10-01"), EULAVersion: 3.4}
	if acpInvoiceTrackClear(now, []acpInvoice{grace}, nil, false, false) {
		t.Error("no grace period: the invoice is overdue and must not be clear")
	}
	if !acpInvoiceTrackClear(now, []acpInvoice{grace}, nil, true, true) {
		t.Error("primary partner grace period: still within it, should be clear")
	}
}
