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

package service

import (
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func commAt(s string) *time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return &t
}

// comm builds an opted-in, started, type=outage outage — the only shape that
// earns a declaration. Each test varies one thing from here.
func comm() domain.OutageForCommunication {
	return domain.OutageForCommunication{
		OutageID:         "o1",
		Number:           "OUT0001234",
		Type:             "OUTAGE",
		ShortDescription: "API gateway degraded",
		Impact:           "High",
		State:            "In Progress",
		StartOn:          commAt("2026-09-30T08:00:00Z"),
		OptedIn:          true,
	}
}

// *** THE DECLARATION ARM IS type=outage ONLY. ***
//
// ServiceNow's step 4 requires `type=outage`, and its panel label ("Declare
// Outage") gives no hint of it. A DEGRADATION or PLANNED outage clears
// step 3's wait and then fails the branch, so the parked instance sends
// nothing — forever.
//
// This is the finding most likely to be "fixed" by a later reader who sees
// an outage going unannounced and widens the condition. These cases exist to
// make that a deliberate change rather than an accidental one.
func TestDecideOutageCommunication_DeclarationIsOutageTypeOnly(t *testing.T) {
	tests := []struct {
		name     string
		typ      string
		wantKind domain.OutageCommunicationKind
	}{
		{"an outage is declared", "OUTAGE", domain.OutageCommunicationDeclared},
		{"lower case still matches", "outage", domain.OutageCommunicationDeclared},
		{"a degradation is NOT declared", "DEGRADATION", domain.OutageCommunicationNone},
		{"a planned outage is NOT declared", "PLANNED", domain.OutageCommunicationNone},
		{"an empty type is NOT declared", "", domain.OutageCommunicationNone},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			o := comm()
			o.Type = tc.typ
			got := decideOutageCommunication(o)
			if got.Kind != tc.wantKind {
				t.Fatalf("Kind = %q, want %q (reason %q)", got.Kind, tc.wantKind, got.Reason)
			}
		})
	}
}

// The two waits, and the guard that replaces run-once.
func TestDecideOutageCommunication_TheTwoArms(t *testing.T) {
	ended := commAt("2026-09-30T10:00:00Z")

	tests := []struct {
		name     string
		mutate   func(*domain.OutageForCommunication)
		wantKind domain.OutageCommunicationKind
	}{
		{
			name:     "opted in and started, never declared",
			mutate:   func(o *domain.OutageForCommunication) {},
			wantKind: domain.OutageCommunicationDeclared,
		},
		{
			// Step 3's wait, second clause. The panel renders only the first;
			// the stored query has both.
			name:     "not opted in sends nothing",
			mutate:   func(o *domain.OutageForCommunication) { o.OptedIn = false },
			wantKind: domain.OutageCommunicationNone,
		},
		{
			name:     "not started sends nothing",
			mutate:   func(o *domain.OutageForCommunication) { o.StartOn = nil },
			wantKind: domain.OutageCommunicationNone,
		},
		{
			// The guard. ServiceNow gets this from run-once; the port gets it
			// from its own log.
			name: "already declared and still running sends nothing",
			mutate: func(o *domain.OutageForCommunication) {
				o.AlreadyDeclared = true
			},
			wantKind: domain.OutageCommunicationNone,
		},
		{
			name: "declared and now ended earns the resolution",
			mutate: func(o *domain.OutageForCommunication) {
				o.AlreadyDeclared = true
				o.EndOn = ended
			},
			wantKind: domain.OutageCommunicationResolved,
		},
		{
			name: "already resolved sends nothing",
			mutate: func(o *domain.OutageForCommunication) {
				o.AlreadyDeclared, o.AlreadyResolved = true, true
				o.EndOn = ended
			},
			wantKind: domain.OutageCommunicationNone,
		},
		{
			// ServiceNow's step 4 requires `end is empty`, so an outage that
			// arrives already finished is never declared — and, never having
			// been declared, never resolved either. It falls out entirely.
			name: "already ended before declaration falls out",
			mutate: func(o *domain.OutageForCommunication) {
				o.EndOn = ended
			},
			wantKind: domain.OutageCommunicationNone,
		},
		{
			// The resolution arm carries no type clause of its own (step 11
			// is `End is not empty` and nothing else). It is narrow only
			// because it is reachable solely through step 4 — so an outage
			// whose type changed after declaration still resolves.
			name: "type changed after declaration still resolves",
			mutate: func(o *domain.OutageForCommunication) {
				o.AlreadyDeclared = true
				o.EndOn = ended
				o.Type = "DEGRADATION"
			},
			wantKind: domain.OutageCommunicationResolved,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			o := comm()
			tc.mutate(&o)
			got := decideOutageCommunication(o)
			if got.Kind != tc.wantKind {
				t.Fatalf("Kind = %q, want %q (reason %q)", got.Kind, tc.wantKind, got.Reason)
			}
		})
	}
}

// *** THE RESOLUTION REUSES THE DECLARATION'S SUBJECT. ***
//
// ServiceNow achieves this by writing the subject onto the INCIDENT at step
// 8 and reading it back at step 12, purely so the two mails thread. The port
// reads it off its own log row instead — same outcome, one fewer table.
func TestRenderOutageResolution_ReusesTheDeclarationSubject(t *testing.T) {
	o := comm()
	o.AlreadyDeclared = true
	o.EndOn = commAt("2026-09-30T10:00:00Z")
	o.DeclaredSubject = "Choreo Outage OUT0001234 on API gateway degraded"

	got := decideOutageCommunication(o)
	if got.Subject != o.DeclaredSubject {
		t.Fatalf("Subject = %q, want the declaration's %q", got.Subject, o.DeclaredSubject)
	}

	// And it must never send a blank subject if that row is missing.
	o.DeclaredSubject = ""
	got = decideOutageCommunication(o)
	if got.Subject == "" {
		t.Fatal("Subject is empty when the declaration row is missing; want a re-derived one")
	}
}

// The wording is what the SRE group reads, so it is pinned rather than left
// to drift. Includes the two placeholders awaiting a stakeholder answer, so
// that changing them is a visible test change.
func TestRenderOutageCommunication_MatchesTheOriginalWording(t *testing.T) {
	o := comm()
	decl := decideOutageCommunication(o)

	if want := "Choreo Outage OUT0001234 on API gateway degraded"; decl.Subject != want {
		t.Errorf("declaration subject = %q, want %q", decl.Subject, want)
	}
	for _, frag := range []string{
		"Hello Team,",
		"currently affecting our services",
		"Incident Number: OUT0001234", // ServiceNow's label; bound to the OUTAGE number
		"Start Time: 2026-09-30T08:00:00Z",
		"Current Status: In Progress",
		"Next Steps:",
		"Best regards,\nSRE\n", // replaces ServiceNow's unfilled "[Team Name]"
	} {
		if !strings.Contains(decl.Body, frag) {
			t.Errorf("declaration body missing %q\n---\n%s", frag, decl.Body)
		}
	}

	o.AlreadyDeclared = true
	o.EndOn = commAt("2026-09-30T10:00:00Z")
	o.DurationSeconds = 2 * 60 * 60
	res := decideOutageCommunication(o)
	for _, frag := range []string{
		"Hi Team,",
		"fully resolved",
		"Outage Duration: 2h",
		"End Time: 2026-09-30T10:00:00Z",
		"Root Cause Analysis (RCA)", // a promise no active flow keeps
		"Best regards,\nSRE\n",
	} {
		if !strings.Contains(res.Body, frag) {
			t.Errorf("resolution body missing %q\n---\n%s", frag, res.Body)
		}
	}
}

// The declaration arm must never render an empty End Time into a body.
func TestFormatOutageInstant_NilIsEmptyNotZeroTime(t *testing.T) {
	if got := formatOutageInstant(nil); got != "" {
		t.Fatalf("formatOutageInstant(nil) = %q, want empty", got)
	}
}

// *** THE MICROSECONDS BUG, PINNED. ***
//
// The first live send rendered "Outage Duration: 00:31:25.634362" — a raw
// Postgres interval cast to text. No unit test caught it because the tests
// asserted against a hand-written string and so only agreed with themselves;
// it took a real message in a real inbox.
//
// These cases assert on the formatter directly, so the next change to it
// cannot quietly reintroduce a machine-shaped value.
func TestFormatOutageDuration(t *testing.T) {
	tests := []struct {
		name    string
		seconds int64
		want    string
	}{
		{"the outage that exposed this, rounded", 31*60 + 25, "31m 25s"},
		{"seconds only", 45, "45s"},
		{"exact minute drops the seconds", 120, "2m"},
		{"exact hour", 2 * 60 * 60, "2h"},
		// Trailing seconds are noise once hours are involved, and actively
		// unhelpful once days are.
		{"hours suppress seconds", 2*60*60 + 5*60 + 12, "2h 5m"},
		{"days suppress seconds", 2*24*60*60 + 3*60*60 + 4*60 + 9, "2d 3h 4m"},
		{"a day exactly", 24 * 60 * 60, "1d"},
		// Not ended: blank, never "0s", which would assert the outage
		// ended instantly rather than that its length is unknown.
		{"zero is blank", 0, ""},
		{"negative is blank", -5, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatOutageDuration(tc.seconds); got != tc.want {
				t.Fatalf("formatOutageDuration(%d) = %q, want %q", tc.seconds, got, tc.want)
			}
		})
	}
}

// And the regression itself: no rendered duration may ever carry a decimal
// point, which is what a Postgres interval brings with it.
func TestFormatOutageDuration_NeverCarriesFractionalSeconds(t *testing.T) {
	for _, s := range []int64{1, 59, 61, 3599, 3601, 86399, 86401, 999999} {
		if got := formatOutageDuration(s); strings.Contains(got, ".") {
			t.Errorf("formatOutageDuration(%d) = %q — contains a decimal point", s, got)
		}
	}
}
