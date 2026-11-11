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
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp/salesentity"
)

var mismatchNow = time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

const mismatchSubjectPrefix = "[ACP] End Date Mismatch"

func mismatchDay(days int) *time.Time {
	d := mismatchNow.AddDate(0, 0, days)
	return &d
}

// mismatchProject is an Open project as the details source hands it over:
// EndDate is Salesforce's, csmEndDate what CSM held before it was replaced.
func mismatchProject(csmDays, sfDays *int, warnDay bool) project {
	p := project{
		ID: "00000000-0000-0000-0000-000000000001", Name: "Example Project", ProjectKey: "EXAMPLESUB",
		Account:             &projectAccountRef{ID: "a1", Name: "Example Corp"},
		ClosureState:        strPtr("Open"),
		EndDateClosureState: strPtr("Open"),
		endDateWarnDay:      warnDay,
	}
	if csmDays != nil {
		p.csmEndDate = mismatchDay(*csmDays)
	}
	if sfDays != nil {
		p.EndDate = mismatchDay(*sfDays)
	}
	return p
}

func ip(i int) *int { return &i }

func mismatchReader(amEmail string, rmEmail ...string) *mockEntityReader {
	account := `{"hasPrimaryPartner":false`
	if amEmail != "" {
		account += `,"accountManager":{"id":"u1","name":"Jane Doe","email":"` + amEmail + `"}`
	}
	if len(rmEmail) > 0 && rmEmail[0] != "" {
		account += `,"renewalAccountManager":{"id":"u2","name":"Jane Doe","email":"` + rmEmail[0] + `"}`
	}
	if len(rmEmail) > 1 {
		account += `,"technicalOwner":{"id":"u3","name":"Jane Doe","email":"` + rmEmail[1] + `"}`
	}
	account += `}`
	return &mockEntityReader{getAccountFn: func(context.Context, string) ([]byte, error) { return []byte(account), nil }}
}

type capturedMail struct {
	to, cc  []string
	subject string
}

type captureSender struct{ mails []capturedMail }

func (c *captureSender) SendEmail(_ context.Context, to, cc []string, subject, _ string) error {
	c.mails = append(c.mails, capturedMail{to: to, cc: cc, subject: subject})
	return nil
}

func warnings(sent []notify.Notice) []notify.Notice {
	var out []notify.Notice
	for _, n := range sent {
		if strings.HasPrefix(n.Subject, mismatchSubjectPrefix) {
			out = append(out, n)
		}
	}
	return out
}

func TestProcessProject_EndDateMismatch(t *testing.T) {
	tests := []struct {
		name         string
		csm, sf      *int
		warnDay      bool
		wantHeld     bool // the end-date track did nothing
		wantWarnings int
	}{
		{"csm later, salesforce in a notice window, warning day", ip(200), ip(7), true, true, 1},
		{"csm later, salesforce in a notice window, other day", ip(200), ip(7), false, true, 0},
		{"csm later, salesforce at day 0 (would suspend), warning day", ip(200), ip(0), true, true, 1},
		{"csm later, salesforce overdue, warning day", ip(200), ip(-3), true, true, 1},
		{"csm later, salesforce far out: nothing to do, no warning", ip(400), ip(300), true, false, 0},
		{"same calendar day, different time of day", ip(7), ip(7), true, false, 0},
		{"csm earlier: salesforce's date acts as before", ip(3), ip(7), true, false, 0},
		{"csm date missing", nil, ip(7), true, false, 0},
		{"salesforce date missing", ip(200), nil, true, false, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			updater, ntf := &mockProjectUpdater{}, delivering()
			proj := mismatchProject(tc.csm, tc.sf, tc.warnDay)
			acted, err := processProject(context.Background(), mismatchReader("am@example.com"), updater, ntf, mismatchNow, proj)
			if err != nil {
				t.Fatalf("processProject() error = %v", err)
			}
			if got := len(warnings(ntf.sent)); got != tc.wantWarnings {
				t.Errorf("warnings = %d, want %d", got, tc.wantWarnings)
			}
			others := len(ntf.sent) - len(warnings(ntf.sent))
			if tc.wantHeld {
				if others != 0 || len(updater.calls) != 0 || len(updater.checks) != 0 || len(updater.sfSuspendCalls) != 0 || acted {
					t.Errorf("held: notices=%d writes=%d probes=%d sfSuspends=%d acted=%v, want nothing", others, len(updater.calls), len(updater.checks), len(updater.sfSuspendCalls), acted)
				}
				return
			}
			// Not held: whatever the track did before still happens.
			wantAct := tc.sf != nil && *tc.sf <= 90 && tc.csm != nil
			if wantAct && !acted {
				t.Errorf("acted = false, want the end-date track to act on its date as before")
			}
		})
	}
}

func TestProcessProject_EndDateMismatch_WarningContent(t *testing.T) {
	ntf := delivering()
	proj := mismatchProject(ip(200), ip(7), true)
	if _, err := processProject(context.Background(), mismatchReader("am@example.com"), &mockProjectUpdater{}, ntf, mismatchNow, proj); err != nil {
		t.Fatal(err)
	}
	w := warnings(ntf.sent)
	if len(w) != 1 {
		t.Fatalf("warnings = %d, want 1", len(w))
	}
	n := w[0]
	if n.Subject != "[ACP] End Date Mismatch for Project Example Project of Example Corp" {
		t.Errorf("subject = %q", n.Subject)
	}
	for _, want := range []string{"Example Project", "EXAMPLESUB", "2026-12-18", "2026-06-08", "No notice or suspension was sent", "Dear Account Manager and Renewal Manager"} {
		if !strings.Contains(n.Body, want) {
			t.Errorf("body lacks %q:\n%s", want, n.Body)
		}
	}
	for _, banned := range []string{"legacy"} {
		if strings.Contains(n.Body, banned) {
			t.Errorf("body mentions %q", banned)
		}
	}
	if !n.StandingAsCC || n.Recipients.AccountOwner.Email != "am@example.com" || len(n.Recipients.Customers) != 0 {
		t.Errorf("recipients = %+v standingAsCC=%v, want the Account Manager only, standing lists as cc", n.Recipients, n.StandingAsCC)
	}
}

// The recipients through the real notifier: To = Account Manager, Cc = the
// standing lists; no Account Manager: the standing lists are To; neither:
// nothing is sent.
func TestProcessProject_EndDateMismatch_Recipients(t *testing.T) {
	lists := []string{"lists@example.com"}
	tests := []struct {
		name       string
		am, rm, to string // to: the technical owner's address, never a recipient
		standing   []string
		wantTo     []string
		wantCc     []string
	}{
		{"both people and standing lists", "am@example.com", "rm@example.com", "", lists, []string{"am@example.com", "rm@example.com"}, lists},
		{"shared address once, any case", "am@example.com", "AM@Example.com", "", lists, []string{"am@example.com"}, lists},
		{"only the renewal manager", "", "rm@example.com", "", lists, []string{"rm@example.com"}, lists},
		{"only the account manager", "am@example.com", "", "", lists, []string{"am@example.com"}, lists},
		{"both people, no standing lists", "am@example.com", "rm@example.com", "", nil, []string{"am@example.com", "rm@example.com"}, nil},
		{"technical owner is not added", "am@example.com", "rm@example.com", "to@example.com", lists, []string{"am@example.com", "rm@example.com"}, lists},
		{"a person also in the standing lists is not repeated in cc", "am@example.com", "", "", []string{"AM@example.com", "lists@example.com"}, []string{"am@example.com"}, lists},
		{"neither person: standing lists are to", "", "", "", []string{"lists@example.com", "more@example.com"}, []string{"lists@example.com", "more@example.com"}, nil},
		{"technical owner alone is not a recipient", "", "", "to@example.com", lists, lists, nil},
		{"neither person and no standing lists: nothing", "", "", "", nil, nil, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sender := &captureSender{}
			ntf := &notify.EmailNotifier{Sender: sender, Logger: discardSlog(), AllowNonWSO2Recipients: true, StandingRecipients: tc.standing}
			proj := mismatchProject(ip(200), ip(7), true)
			if _, err := processProject(context.Background(), mismatchReader(tc.am, tc.rm, tc.to), &mockProjectUpdater{}, ntf, mismatchNow, proj); err != nil {
				t.Fatal(err)
			}
			if tc.wantTo == nil {
				if len(sender.mails) != 0 {
					t.Fatalf("mails = %+v, want none", sender.mails)
				}
				return
			}
			if len(sender.mails) != 1 {
				t.Fatalf("mails = %d, want 1", len(sender.mails))
			}
			m := sender.mails[0]
			if strings.Join(m.to, ",") != strings.Join(tc.wantTo, ",") || strings.Join(m.cc, ",") != strings.Join(tc.wantCc, ",") {
				t.Errorf("to=%v cc=%v, want to=%v cc=%v", m.to, m.cc, tc.wantTo, tc.wantCc)
			}
		})
	}
}

// Emails off: the logging notifier is the one in use, which sends nothing.
func TestProcessProject_EndDateMismatch_EmailsOffSendsNothing(t *testing.T) {
	ntf := &notify.LoggingNotifier{Logger: discardSlog()}
	updater := &mockProjectUpdater{}
	proj := mismatchProject(ip(200), ip(7), true)
	if _, err := processProject(context.Background(), mismatchReader("am@example.com"), updater, ntf, mismatchNow, proj); err != nil {
		t.Fatal(err)
	}
	if len(updater.calls) != 0 {
		t.Errorf("writes = %d, want 0", len(updater.calls))
	}
}

// A dry run records nothing: the dry-run updater is a no-op and the held
// track makes no write attempt at all.
func TestProcessProject_EndDateMismatch_DryRunWritesNothing(t *testing.T) {
	ntf := delivering()
	proj := mismatchProject(ip(200), ip(7), true)
	if _, err := processProject(context.Background(), mismatchReader("am@example.com"), &DryRunProjectUpdater{}, ntf, mismatchNow, proj); err != nil {
		t.Fatal(err)
	}
	if len(warnings(ntf.sent)) != 1 {
		t.Errorf("warnings = %d, want 1 (sent like a notice)", len(warnings(ntf.sent)))
	}
}

// The renewal reset must not fire for a held track: a recorded notice and
// Salesforce's date far out would reset the state, but a held track with
// Salesforce in a window has no reset to make.
func TestProcessProject_EndDateMismatch_NoReset(t *testing.T) {
	updater, ntf := &mockProjectUpdater{}, delivering()
	proj := mismatchProject(ip(200), ip(7), true)
	proj.SuspensionProcessState = savedState(`{"event_type":"30_days_notice","actionSendEmailNotification":"SUCCESSFUL"}`, "")
	if _, err := processProject(context.Background(), mismatchReader("am@example.com"), updater, ntf, mismatchNow, proj); err != nil {
		t.Fatal(err)
	}
	if len(updater.calls) != 0 {
		t.Errorf("writes = %+v, want none", updater.calls)
	}
}

// The invoice track is unaffected by a held end-date track.
func TestProcessProject_EndDateMismatch_InvoiceTrackStillActs(t *testing.T) {
	reader := invoiceLinkedReader(t, mismatchNow)
	reader.getAccountFn = func(context.Context, string) ([]byte, error) {
		return []byte(`{"hasPrimaryPartner":false,"accountManager":{"name":"Jane Doe","email":"am@example.com"}}`), nil
	}
	updater, ntf := &mockProjectUpdater{}, delivering()
	proj := mismatchProject(ip(200), ip(7), true)
	proj.InvoiceDueDateClosureState = strPtr("Open")
	acted, err := processProject(context.Background(), reader, updater, ntf, mismatchNow, proj)
	if err != nil {
		t.Fatal(err)
	}
	if !acted || len(warnings(ntf.sent)) != 1 || len(ntf.sent)-1 == 0 {
		t.Errorf("acted=%v sent=%d warnings=%d, want the invoice notices plus one warning", acted, len(ntf.sent), len(warnings(ntf.sent)))
	}
	if !wroteField(updater.calls, "invoiceDueDateClosureState") {
		t.Errorf("the invoice track did not suspend: %+v", updater.calls)
	}
	if wroteField(updater.calls, "endDateClosureState") {
		t.Error("the held end-date track wrote its closure state")
	}
}

// The warning does not count toward ACP_DAILY_LIMIT: a project that only
// warns is not "acted".
func TestProcessProject_EndDateMismatch_WarningIsNotActed(t *testing.T) {
	acted, err := processProject(context.Background(), mismatchReader("am@example.com"), &mockProjectUpdater{}, delivering(), mismatchNow, mismatchProject(ip(200), ip(7), true))
	if err != nil || acted {
		t.Errorf("acted=%v err=%v, want false, nil", acted, err)
	}
}

// End to end through the Salesforce details source and Run: CSM holds a later
// date than Salesforce, so the project is held and the warning goes only on
// the warning day.
func TestRun_EndDateMismatch_FromSalesforceDetails(t *testing.T) {
	sfEnd := mismatchNow.AddDate(0, 0, 7).Format("2006-01-02")
	for _, warnDay := range []bool{true, false} {
		sf := &fakeSalesforce{
			projects:  map[string]salesentity.Project{"a0dX": {ID: "a0dX", StartDate: sp("2025-07-01"), EndDate: sp(sfEnd), CustomerID: sp("001X")}},
			customers: map[string]salesentity.Customer{"001X": {ID: "001X", Status: sp("Customer"), Owner: &salesentity.User{Name: sp("Jane Doe"), Email: sp("am@example.com")}}},
		}
		csmEnd := mismatchNow.AddDate(0, 0, 200).Format(time.RFC3339)
		listed := `{"id":"p1","name":"Example Project","key":"EXAMPLESUB","sfId":"a0dX","account":{"id":"acc-csm","name":"Example Corp"},` +
			`"closureState":"Open","endDateClosureState":"Open","endDate":"` + csmEnd + `","suspensionProcessOverridden":false}`
		reader := &mockEntityReader{searchProjectsFn: func(context.Context, []byte) ([]byte, error) {
			return []byte(`{"projects":[` + listed + `],"total":1,"hasMore":false}`), nil
		}}
		updater, ntf := &mockProjectUpdater{}, delivering()
		res, err := RunWith(context.Background(), reader, NewSalesforceDetails(sf), updater, ntf, mismatchNow, "", nil, Options{EndDateWarningDay: warnDay})
		if err != nil || len(res.Failures) != 0 {
			t.Fatalf("RunWith() err=%v failures=%v", err, res.Failures)
		}
		want := 0
		if warnDay {
			want = 1
		}
		if len(ntf.sent) != want || len(warnings(ntf.sent)) != want || len(updater.calls) != 0 || len(updater.sfSuspendCalls) != 0 {
			t.Errorf("warnDay=%v: sent=%d warnings=%d writes=%d sfSuspends=%d, want %d warning(s) and nothing else", warnDay, len(ntf.sent), len(warnings(ntf.sent)), len(updater.calls), len(updater.sfSuspendCalls), want)
		}
	}
}

func discardSlog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }
