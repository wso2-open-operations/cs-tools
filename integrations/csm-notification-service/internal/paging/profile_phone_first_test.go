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

package paging

import (
	"context"
	"strings"
	"testing"
	"time"
)

// The number to call, first match wins: TEST_CALL_TO (fixed), the CSM profile
// number, a number the person already carries (phone book, paging-alert.yaml),
// the alert-only number in CSM, then Asgardeo.
func TestProfilePhoneResolver_ProfileFirstThenFallbacks(t *testing.T) {
	ctx := context.Background()
	inner := StaticResolver{ByLevel: map[Level][]Recipient{Level0: {
		{Name: "Profile beats book", Email: "a@example.com", Phone: "+94770000101"},
		{Name: "Book beats alert-only", Email: "b@example.com", Phone: "+94770000102"},
		{Name: "Book when profile is not E.164", Email: "c@example.com", Phone: "+94770000103"},
		{Name: "Alert-only when no book", Email: "d@example.com"},
		{Name: "Test call to", Email: "e@example.com", Phone: "+94770000999", PhoneFixed: true},
		{Name: "Book, no CSM number", Email: "f@example.com", Phone: "+94770000106"},
	}}}
	contacts := &fakePagingContacts{numbers: map[string]DialNumber{
		"a@example.com": {Number: "+94770000201", Source: DialSourceProfile},
		"b@example.com": {Number: "+94770000202", Source: DialSourcePaging},
		"c@example.com": {Number: "0770000203", Source: DialSourceProfile},
		"d@example.com": {Number: "+94770000204", Source: DialSourcePaging},
		"e@example.com": {Number: "+94770000205", Source: DialSourceProfile},
	}}
	lookup := &fakeLookup{numbers: map[string]string{"f@example.com": "+94770000306", "c@example.com": "+94770000303"}}

	got, err := NewProfilePhoneResolver(inner, lookup).WithPagingContacts(contacts).Resolve(ctx, Level0, RoutingContext{})
	if err != nil {
		t.Fatal(err)
	}
	p := phones(got)
	for email, want := range map[string]string{
		"a@example.com": "+94770000201", // CSM profile number over the phone book's
		"b@example.com": "+94770000102", // phone book over the alert-only number
		"c@example.com": "+94770000103", // profile not E.164: phone book, not Asgardeo
		"d@example.com": "+94770000204", // no phone book number: the alert-only one
		"e@example.com": "+94770000999", // TEST_CALL_TO is never replaced
		"f@example.com": "+94770000106", // phone book before Asgardeo
	} {
		if p[email] != want {
			t.Errorf("%s = %q, want %q", email, p[email], want)
		}
	}
	if strings.Contains(strings.Join(contacts.asked, ","), "e@example.com") {
		t.Errorf("CSM was asked about a fixed TEST_CALL_TO recipient: %v", contacts.asked)
	}
	lookup.mu.Lock()
	defer lookup.mu.Unlock()
	if len(lookup.calls) != 0 {
		t.Errorf("Asgardeo asked about %v; everyone had a number before it", lookup.calls)
	}
}

// Only TEST_CALL_TO marks a number fixed; a phone book entry is a fallback.
func TestResolveSRE_OnlyTestCallToIsFixed(t *testing.T) {
	rc := RoutingContext{AssignedCRETeam: "apollo", Ladder: LadderSRE, At: testClock}

	test := sreResolver(morningRota()).WithPhoneBook(PhoneBook{TestCallTo: "+94770000000"})
	got, err := test.Resolve(context.Background(), Level0, rc)
	if err != nil || len(got) != 1 || !got[0].PhoneFixed {
		t.Fatalf("TEST_CALL_TO: got %+v, %v; want a fixed number", got, err)
	}

	r := sreResolver(morningRota())
	first, err := r.Resolve(context.Background(), Level0, rc)
	if err != nil || len(first) != 1 {
		t.Fatalf("rota: got %+v, %v", first, err)
	}
	book := r.WithPhoneBook(PhoneBook{ByEmail: map[string]string{first[0].Email: "+94770000001"}})
	got, err = book.Resolve(context.Background(), Level0, rc)
	if err != nil || len(got) != 1 || got[0].Phone != "+94770000001" || got[0].PhoneFixed {
		t.Fatalf("phone book: got %+v, %v; want its number, not fixed", got, err)
	}
}

// TEST_CALL_TO replaces even a number written in paging-alert.yaml, and the
// profile lookup leaves it alone: a test environment never rings a head's
// real phone.
func TestTestCallTo_OverridesConfiguredHeadNumber(t *testing.T) {
	r := testResolver(&stubScheduleReader{}).WithHeads(Heads{
		CRE: Person{Name: "CRE Head", Email: "cre.head@example.com", Phone: "+94770000001"},
	}).WithPhoneBook(PhoneBook{TestCallTo: "+94770000000"})
	contacts := &fakePagingContacts{numbers: map[string]DialNumber{
		"cre.head@example.com": {Number: "+94770000002", Source: DialSourceProfile},
	}}
	rc := RoutingContext{Shift: ShiftLK, AssignedCRETeam: "vega", At: time.Now()}

	got, err := NewProfilePhoneResolver(r, nil).WithPagingContacts(contacts).Resolve(context.Background(), Level3, rc)
	if err != nil || len(got) != 1 {
		t.Fatalf("got %+v, %v", got, err)
	}
	if got[0].Phone != "+94770000000" || !got[0].PhoneFixed {
		t.Errorf("head = %q fixed=%v; want TEST_CALL_TO, fixed", got[0].Phone, got[0].PhoneFixed)
	}
	if len(contacts.asked) != 0 {
		t.Errorf("CSM asked about %v under TEST_CALL_TO", contacts.asked)
	}
}

// Roster mode too: TEST_CALL_TO rings every roster recipient on one number,
// fixed, and leaves the roster itself untouched. Without it, a valid CSM
// profile number replaces the roster's.
func TestRosterResolver_TestCallToIsFixed(t *testing.T) {
	roster := Roster{Default: LevelRoster{Level0.String(): {{Name: "Ana", Email: "ana@example.com", Phone: "+94770000001"}}}}
	contacts := &fakePagingContacts{numbers: map[string]DialNumber{
		"ana@example.com": {Number: "+94770000002", Source: DialSourceProfile},
	}}
	resolve := func(r Resolver) Recipient {
		t.Helper()
		got, err := NewProfilePhoneResolver(r, nil).WithPagingContacts(contacts).Resolve(context.Background(), Level0, RoutingContext{})
		if err != nil || len(got) != 1 {
			t.Fatalf("got %+v, %v", got, err)
		}
		return got[0]
	}

	if got := resolve(NewRosterResolver(roster).WithTestCallTo(" +94770000000 ")); got.Phone != "+94770000000" || !got.PhoneFixed {
		t.Errorf("TEST_CALL_TO: %q fixed=%v; want the test number, fixed", got.Phone, got.PhoneFixed)
	}
	if roster.Default[Level0.String()][0].Phone != "+94770000001" {
		t.Errorf("the roster itself was changed: %+v", roster.Default)
	}
	if got := resolve(NewRosterResolver(roster)); got.Phone != "+94770000002" || got.PhoneFixed {
		t.Errorf("no TEST_CALL_TO: %q fixed=%v; want the CSM profile number", got.Phone, got.PhoneFixed)
	}
}

// An invalid number the recipient already carries must not hide a valid one
// from a later source, nor the rejection from the note.
func TestProfilePhoneResolver_InvalidCarriedNumber(t *testing.T) {
	ctx := context.Background()
	inner := StaticResolver{ByLevel: map[Level][]Recipient{Level0: {
		{Name: "Book bad, directory good", Email: "a@example.com", Phone: "0770000101"},
		{Name: "Book bad, nothing else", Email: "b@example.com", Phone: "0770000102"},
		{Name: "Fixed", Email: "c@example.com", Phone: "not-a-number", PhoneFixed: true},
		{Name: "Book good", Email: "d@example.com", Phone: "+15550100000"},
	}}}
	lookup := &fakeLookup{numbers: map[string]string{"a@example.com": "+15550100001"}}
	got, err := NewProfilePhoneResolver(inner, lookup).Resolve(ctx, Level0, RoutingContext{})
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]Recipient{}
	for _, r := range got {
		by[r.Email] = r
	}
	if by["a@example.com"].Phone != "+15550100001" || by["a@example.com"].PhoneRejected != "" {
		t.Errorf("valid directory number not used: %+v", by["a@example.com"])
	}
	if b := by["b@example.com"]; b.Phone != "" || b.PhoneRejected != PhoneInvalidFormat {
		t.Errorf("invalid carried number must be cleared and marked rejected: %+v", b)
	}
	if c := by["c@example.com"]; c.Phone != "not-a-number" || c.PhoneRejected != "" {
		t.Errorf("PhoneFixed recipient was altered: %+v", c)
	}
	if d := by["d@example.com"]; d.Phone != "+15550100000" {
		t.Errorf("valid carried number changed: %+v", d)
	}
	plan := Plan{Trigger: testTrigger("P1", ShiftLKMorning), Channel: ChannelCall,
		Calls: []PlannedCall{{Level: Level1, Ordinal: 1, At: triggerAt, Recipient: by["b@example.com"]}}}
	note := plan.WorkNote([]bool{true}, nil, nil, "")
	if !strings.Contains(note, "[ERROR][INVALID_PHONE_FORMAT][b@example.com]") {
		t.Errorf("note should say INVALID_PHONE_FORMAT:\n%s", note)
	}
}
