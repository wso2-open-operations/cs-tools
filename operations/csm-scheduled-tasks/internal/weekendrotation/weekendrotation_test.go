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

package weekendrotation

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/entityrotations"
)

type stubRotations struct {
	byDate map[string][]entityrotations.Member
	err    error
	calls  []string
}

func (s *stubRotations) SearchRotations(_ context.Context, _ []string, date string) ([]entityrotations.Member, error) {
	s.calls = append(s.calls, date)
	if s.err != nil {
		return nil, s.err
	}
	return s.byDate[date], nil
}

type stubEmail struct {
	sent    int
	to, cc  []string
	subject string
	body    string
	err     error
}

func (s *stubEmail) SendEmail(_ context.Context, to, cc []string, subject, htmlBody string) error {
	s.sent++
	s.to, s.cc, s.subject, s.body = to, cc, subject, htmlBody
	return s.err
}

func at(y int, m time.Month, d int) func() time.Time {
	return func() time.Time { return time.Date(y, m, d, 9, 0, 0, 0, time.UTC) }
}

// TestNextSaturday pins the date arithmetic. It agrees with the ServiceNow
// action's `6 - dayOfWeek` (Monday=1 … Sunday=7) on the two days that action
// actually ran -- Monday gives +5 and Thursday +2, both landing on the coming
// Saturday -- and additionally stays correct on the other five days, where
// that expression goes negative and would look backwards.
func TestNextSaturday(t *testing.T) {
	cases := []struct {
		name string
		from time.Time
		want string
	}{
		// 2026-09-14 is a Monday; that week's Saturday is the 19th.
		{"monday", time.Date(2026, 9, 14, 9, 0, 0, 0, time.UTC), "2026-09-19"},
		{"thursday", time.Date(2026, 9, 17, 9, 0, 0, 0, time.UTC), "2026-09-19"},
		{"friday", time.Date(2026, 9, 18, 23, 59, 0, 0, time.UTC), "2026-09-19"},
		// A Saturday maps to itself, not to the following week.
		{"saturday maps to itself", time.Date(2026, 9, 19, 9, 0, 0, 0, time.UTC), "2026-09-19"},
		// The ServiceNow bug: on a Sunday it looked up the previous day.
		{"sunday moves forward, never back", time.Date(2026, 9, 20, 9, 0, 0, 0, time.UTC), "2026-09-26"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NextSaturday(tc.from).Format(dateLayout); got != tc.want {
				t.Errorf("NextSaturday(%s) = %s, want %s", tc.from.Format(dateLayout), got, tc.want)
			}
		})
	}
}

func TestSendNotice_MailsTheRosteredTeam(t *testing.T) {
	rot := &stubRotations{byDate: map[string][]entityrotations.Member{
		"2026-09-19": {
			{Type: "ops_weekend", Date: "2026-09-19", Name: "A Person", Email: "A@wso2.com"},
			{Type: "ops_weekend_night", Date: "2026-09-19", Name: "B Person", Email: "b@wso2.com", Notes: "primary"},
		},
		"2026-09-20": {
			// Same person as Saturday: must be mailed once, not twice.
			{Type: "ops_weekend", Date: "2026-09-20", Name: "A Person", Email: "a@wso2.com"},
		},
	}}
	mail := &stubEmail{}

	err := SendNotice(rot, mail, at(2026, 9, 17), []string{"lead@wso2.com"}, []string{"cc@wso2.com"}, true)(context.Background())
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if len(rot.calls) != 2 || rot.calls[0] != "2026-09-19" || rot.calls[1] != "2026-09-20" {
		t.Fatalf("expected Saturday then Sunday lookups, got %v", rot.calls)
	}
	if mail.sent != 1 {
		t.Fatalf("expected exactly one email, got %d", mail.sent)
	}
	want := []string{"a@wso2.com", "b@wso2.com", "lead@wso2.com"}
	if len(mail.to) != len(want) {
		t.Fatalf("recipients = %v, want %v", mail.to, want)
	}
	for i, w := range want {
		if mail.to[i] != w {
			t.Errorf("recipients[%d] = %q, want %q (full: %v)", i, mail.to[i], w, mail.to)
		}
	}
	if !strings.Contains(mail.subject, "2026-09-19") || !strings.Contains(mail.subject, "2026-09-20") {
		t.Errorf("subject should name both weekend dates, got %q", mail.subject)
	}
	// Both weekend dates must render into the body, as the ServiceNow action's
	// own '( '+ saturday +' and '+ sunday +')' did.
	for _, want := range []string{"2026-09-19", "2026-09-20"} {
		if !strings.Contains(mail.body, want) {
			t.Errorf("body should name %s, got:\n%s", want, mail.body)
		}
	}
	// Each member is listed with a readable shift name, and the roster note is
	// carried through -- the action appended it as "name (notes)".
	if !strings.Contains(mail.body, "A Person") || !strings.Contains(mail.body, "Weekend Night") {
		t.Errorf("body should list the roster with a readable shift name, got:\n%s", mail.body)
	}
	if !strings.Contains(mail.body, "primary") {
		t.Errorf("body should carry each member's roster note, got:\n%s", mail.body)
	}
}

// TestSendNotice_EmptyRosterSendsNothingAndSucceeds is the behaviour that
// matters most: an unstaffed weekend must not error. The ServiceNow flow sat
// in exactly this state for over a year, failing on "Email has no recipients".
func TestSendNotice_EmptyRosterSendsNothingAndSucceeds(t *testing.T) {
	rot := &stubRotations{byDate: map[string][]entityrotations.Member{}}
	mail := &stubEmail{}

	if err := SendNotice(rot, mail, at(2026, 9, 17), []string{"lead@wso2.com"}, nil, true)(context.Background()); err != nil {
		t.Fatalf("an empty roster must succeed, got: %v", err)
	}
	if mail.sent != 0 {
		t.Errorf("expected no email for an empty roster, sent %d", mail.sent)
	}
}

func TestSendNotice_EmailsDisabledSendsNothing(t *testing.T) {
	rot := &stubRotations{byDate: map[string][]entityrotations.Member{
		"2026-09-19": {{Type: "ops_weekend", Email: "a@wso2.com"}},
	}}
	mail := &stubEmail{}

	if err := SendNotice(rot, mail, at(2026, 9, 17), nil, nil, false)(context.Background()); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if mail.sent != 0 || len(rot.calls) != 0 {
		t.Errorf("disabled emails must skip both the roster lookup and the send (calls=%v sent=%d)", rot.calls, mail.sent)
	}
}

func TestSendNotice_RosterLookupFailureSurfaces(t *testing.T) {
	sentinel := errors.New("roster unavailable")
	rot := &stubRotations{err: sentinel}

	err := SendNotice(rot, &stubEmail{}, at(2026, 9, 17), nil, nil, true)(context.Background())
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected the lookup error to surface, got %T: %v", err, err)
	}
}

// TestSendNotice_MemberWithNoEmailIsSkipped guards the other half of the
// ServiceNow failure: a roster row that resolves to nobody must not become an
// empty-string recipient.
func TestSendNotice_MemberWithNoEmailIsSkipped(t *testing.T) {
	rot := &stubRotations{byDate: map[string][]entityrotations.Member{
		"2026-09-19": {
			{Type: "ops_weekend", Name: "Ghost", Email: ""},
			{Type: "ops_weekend", Name: "Real", Email: "real@wso2.com"},
		},
	}}
	mail := &stubEmail{}

	if err := SendNotice(rot, mail, at(2026, 9, 17), nil, nil, true)(context.Background()); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if len(mail.to) != 1 || mail.to[0] != "real@wso2.com" {
		t.Fatalf("recipients = %v, want exactly [real@wso2.com]", mail.to)
	}
}
