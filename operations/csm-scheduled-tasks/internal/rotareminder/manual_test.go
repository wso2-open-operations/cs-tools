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

package rotareminder

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/teamschedule"
)

type fakeQueue struct {
	waiting  []teamschedule.ReminderRequest
	reports  []string
	claimErr error
}

func (f *fakeQueue) ClaimReminderRequest(context.Context, string) (*teamschedule.ReminderRequest, error) {
	if f.claimErr != nil {
		return nil, f.claimErr
	}
	if len(f.waiting) == 0 {
		return nil, nil
	}
	r := f.waiting[0]
	f.waiting = f.waiting[1:]
	return &r, nil
}

func (f *fakeQueue) ReportReminderRequest(_ context.Context, _, id, status, result string) error {
	f.reports = append(f.reports, id+" "+status+" "+result)
	return nil
}

func manualConfigs(now string) (Config, Config) {
	weekday := config(now, weekdayRotations)
	weekday.EmailsEnabled = true
	weekend := weekendConfig(now, "rota_manual_reminder")
	weekend.EmailsEnabled = true
	return weekday, weekend
}

// A weekend asked for on a Monday is the coming one, sent even though the
// automatic weekend reminders are switched off, and reported with its count.
func TestManual_WeekendIsSentWhateverTheSwitch(t *testing.T) {
	weekday, weekend := manualConfigs("2026-10-19 10:00")
	off := savedSettings()
	off.WeekdayEnabled, off.WeekendEnabled = false, false
	weekend.Settings = fakeSettings{s: off}
	queue := &fakeQueue{waiting: []teamschedule.ReminderRequest{{ID: "r1", Kind: "weekend", RequestedBy: "ann@example.com"}}}
	mail := &fakeEmail{}
	rota := &fakeRota{assignments: fullWeekend()}
	if err := ManualReminder(rota, &fakeStore{}, queue, mail, weekday, weekend)(context.Background()); err != nil {
		t.Fatal(err)
	}
	if rota.from != "2026-10-24" || len(mail.sent) != 5 || len(queue.reports) != 1 || queue.reports[0] != "r1 SENT Sent 5 emails." {
		t.Fatalf("read %s, sent %d, reports %v", rota.from, len(mail.sent), queue.reports)
	}
}

// From a Saturday it is this weekend, not the next.
func TestManual_FromASaturdayItIsThisWeekend(t *testing.T) {
	weekday, weekend := manualConfigs("2026-10-24 05:00")
	queue := &fakeQueue{waiting: []teamschedule.ReminderRequest{{ID: "r1", Kind: "weekend"}}}
	rota := &fakeRota{assignments: fullWeekend()}
	if err := ManualReminder(rota, &fakeStore{}, queue, &fakeEmail{}, weekday, weekend)(context.Background()); err != nil {
		t.Fatal(err)
	}
	if rota.from != "2026-10-24" || !strings.HasPrefix(queue.reports[0], "r1 SENT") {
		t.Fatalf("read %s, reports %v", rota.from, queue.reports)
	}
}

// Every waiting request is sent in one run; a weekday with nobody on the rota
// is reported as such; emails turned off in the environment is a failure the
// card can show, not a silent success.
func TestManual_SeveralRequestsAndWhatIsReported(t *testing.T) {
	weekday, weekend := manualConfigs("2026-10-19 10:00")
	queue := &fakeQueue{waiting: []teamschedule.ReminderRequest{{ID: "r1", Kind: "weekday"}, {ID: "r2", Kind: "weekend"}}}
	if err := ManualReminder(&fakeRota{}, &fakeStore{}, queue, &fakeEmail{}, weekday, weekend)(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(queue.reports) != 2 ||
		queue.reports[0] != "r1 SENT Nothing was sent: nobody is rostered on Tuesday, 20 October 2026." ||
		queue.reports[1] != "r2 SENT Nothing was sent: nobody is rostered for the weekend of Saturday, 24 October 2026." {
		t.Fatalf("reports %v", queue.reports)
	}
	weekday.EmailsEnabled = false
	queue = &fakeQueue{waiting: []teamschedule.ReminderRequest{{ID: "r3", Kind: "weekday"}}}
	if err := ManualReminder(&fakeRota{}, &fakeStore{}, queue, &fakeEmail{}, weekday, weekend)(context.Background()); err != nil {
		t.Fatal(err)
	}
	if queue.reports[0] != "r3 FAILED emails are turned off in this environment (ALERTS_ENABLED)" {
		t.Fatalf("reports %v", queue.reports)
	}
}

// A failed send is reported FAILED with its reason and the run succeeds; a
// failed claim fails the run.
func TestManual_FailuresAreReported(t *testing.T) {
	weekday, weekend := manualConfigs("2026-10-19 10:00")
	queue := &fakeQueue{waiting: []teamschedule.ReminderRequest{{ID: "r1", Kind: "weekend"}}}
	mail := &fakeEmail{failOn: "Weekend Rotation –"}
	if err := ManualReminder(&fakeRota{assignments: fullWeekend()}, &fakeStore{}, queue, mail, weekday, weekend)(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(queue.reports[0], "r1 FAILED") || !strings.Contains(queue.reports[0], "mail service down") {
		t.Fatalf("reports %v", queue.reports)
	}
	queue = &fakeQueue{claimErr: errors.New("entity-service down")}
	if err := ManualReminder(&fakeRota{}, &fakeStore{}, queue, &fakeEmail{}, weekday, weekend)(context.Background()); err == nil {
		t.Fatal("a failed claim passed")
	}
}

// A test goes only to the person who asked, marked in its subject, with test
// event ids, and never touches the weekend's record: the real reminders still
// announce the weekend afterwards.
func TestManual_ATestGoesOnlyToWhoAsked(t *testing.T) {
	weekday, weekend := manualConfigs("2026-10-19 10:00")
	weekend.Copy = []string{"lead@example.com"}
	store := &fakeStore{getErr: errors.New("a test must not read the record")}
	queue := &fakeQueue{waiting: []teamschedule.ReminderRequest{{ID: "t1", Kind: "weekend", Test: true, RequestedBy: "zoe@example.com"}}}
	mail := &fakeEmail{}
	if err := ManualReminder(&fakeRota{assignments: fullWeekend()}, store, queue, mail, weekday, weekend)(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(mail.sent) != 5 || queue.reports[0] != "t1 SENT Sent 5 emails to zoe@example.com (test)." || len(store.recorded) != 0 {
		t.Fatalf("sent %d, reports %v, recorded %d", len(mail.sent), queue.reports, len(store.recorded))
	}
	for _, m := range mail.sent {
		if strings.Join(m.to, ",") != "zoe@example.com" || len(m.cc) != 0 || !strings.HasPrefix(m.subject, "[Test] [CRE Operations]") ||
			!strings.Contains(m.body, "[Test copy] Not sent to the rota.") {
			t.Errorf("%q to %v cc %v", m.subject, m.to, m.cc)
		}
		for _, a := range m.attachments {
			ics := unfolded(a.Content)
			if !strings.Contains(ics, "-test@csm-team-schedule") || !strings.Contains(ics, "mailto:zoe@example.com") || strings.Contains(ics, "mailto:ann@example.com") {
				t.Errorf("invite:\n%s", ics)
			}
		}
	}

	// The real weekend notice afterwards is unaffected: it reads the record,
	// finds nothing, and announces the weekend with the real event ids.
	real := &fakeStore{}
	mail = &fakeEmail{}
	cfg := weekendConfig("2026-10-19 20:00", "rota_weekend_notice")
	if err := WeekendNotice(&fakeRota{assignments: fullWeekend()}, real, mail, cfg)(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(mail.sent) != 5 || len(real.recorded) != 1 || strings.Contains(string(mail.sent[1].attachments[0].Content), "-test@") {
		t.Fatalf("real notice: sent %d, recorded %d", len(mail.sent), len(real.recorded))
	}
}

// When some reminder emails go out and some fail, the run succeeds (a retry
// would email the others twice), but the card is told how many did not.
func TestManual_APartlyFailedSendSaysSo(t *testing.T) {
	weekday, weekend := manualConfigs("2026-10-19 10:00")
	rota := &fakeRota{assignments: []teamschedule.Assignment{
		assignment("Ann", "ann@example.com", "vega", "CRE_MORNING", "2026-10-20", "2026-10-20 06:00", "2026-10-20 09:00"),
		assignment("Ben", "ben@example.com", "vega", "CRE_EVENING", "2026-10-20", "2026-10-20 18:00", "2026-10-20 21:00"),
	}}
	queue := &fakeQueue{waiting: []teamschedule.ReminderRequest{{ID: "r1", Kind: "weekday"}}}
	mail := &fakeEmail{failOn: "Morning Rotation"}
	if err := ManualReminder(rota, &fakeStore{}, queue, mail, weekday, weekend)(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(queue.reports) != 1 || !strings.HasPrefix(queue.reports[0], "r1 SENT Sent 1 of 2 emails; 1 could not be sent") {
		t.Fatalf("reports %v", queue.reports)
	}
}

// refusesInvites is an email service that answers 400 to any email carrying
// an attachment, as one that cannot take calendar invitations would.
type refusesInvites struct{ fakeEmail }

func (r *refusesInvites) SendEmail(ctx context.Context, to, cc []string, subject, body string) error {
	return r.SendEmailWithAttachments(ctx, to, cc, subject, body, nil)
}

func (r *refusesInvites) SendEmailWithAttachments(ctx context.Context, to, cc []string, subject, body string, a []notify.Attachment) error {
	if len(a) > 0 {
		return &apierror.Error{StatusCode: 400}
	}
	return r.fakeEmail.SendEmailWithAttachments(ctx, to, cc, subject, body, nil)
}

// A weekday email whose invitation is refused goes out without it; the card
// counts it as sent, once, not as one failed and one sent.
func TestManual_AnEmailSentWithoutItsInvitationCountsOnce(t *testing.T) {
	weekday, weekend := manualConfigs("2026-10-19 10:00")
	weekday.Invites, weekday.OrganizerEmail = true, "rota@example.com"
	rota := &fakeRota{assignments: []teamschedule.Assignment{
		assignment("Ann", "ann@example.com", "vega", "CRE_MORNING", "2026-10-20", "2026-10-20 06:00", "2026-10-20 09:00"),
		assignment("Ben", "ben@example.com", "vega", "CRE_EVENING", "2026-10-20", "2026-10-20 18:00", "2026-10-20 21:00"),
	}}
	queue := &fakeQueue{waiting: []teamschedule.ReminderRequest{{ID: "r1", Kind: "weekday"}}}
	mail := &refusesInvites{}
	if err := ManualReminder(rota, &fakeStore{}, queue, mail, weekday, weekend)(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(mail.sent) != 2 || queue.reports[0] != "r1 SENT Sent 2 emails." {
		t.Fatalf("delivered %d, reports %v", len(mail.sent), queue.reports)
	}
}

// When nothing goes out the card says why, window by window: one that has
// already started, one whose people have no email address.
func TestManual_NothingSentSaysWhy(t *testing.T) {
	weekday, weekend := manualConfigs("2026-10-19 10:00")
	rota := &fakeRota{assignments: []teamschedule.Assignment{
		assignment("Ann", "ann@example.com", "vega", "CRE_MORNING", "2026-10-20", "2026-10-19 09:00", "2026-10-20 09:00"),
		assignment("Ben", "", "vega", "CRE_EVENING", "2026-10-20", "2026-10-20 18:00", "2026-10-20 21:00"),
	}}
	queue := &fakeQueue{waiting: []teamschedule.ReminderRequest{{ID: "r1", Kind: "weekday"}}}
	mail := &fakeEmail{}
	if err := ManualReminder(rota, &fakeStore{}, queue, mail, weekday, weekend)(context.Background()); err != nil {
		t.Fatal(err)
	}
	r := queue.reports[0]
	if len(mail.sent) != 0 || !strings.HasPrefix(r, "r1 SENT Nothing was sent: ") ||
		!strings.Contains(r, "on Tuesday, 20 October 2026 has already started; nobody on ") ||
		!strings.HasSuffix(r, " on Tuesday, 20 October 2026 has an email address.") {
		t.Fatalf("sent %d, reports %v", len(mail.sent), queue.reports)
	}
}

// Only the rota's reminders are counted where they are sent: the weekend
// summary and its invitations, not the email asking the rota admins to fill
// a gap.
func TestManual_TheRotaAdminsEmailIsNotCounted(t *testing.T) {
	weekday, weekend := manualConfigs("2026-10-19 10:00")
	store := &fakeStore{admins: []teamschedule.RotaAdmin{{Name: "Ola", Email: "ola@example.com"}}}
	queue := &fakeQueue{waiting: []teamschedule.ReminderRequest{{ID: "r1", Kind: "weekend"}}}
	mail := &fakeEmail{}
	if err := ManualReminder(&fakeRota{assignments: fullWeekend()[1:]}, store, queue, mail, weekday, weekend)(context.Background()); err != nil {
		t.Fatal(err)
	}
	toAdmins := 0
	for _, m := range mail.sent {
		if strings.Join(m.to, ",") == "ola@example.com" {
			toAdmins++
		}
	}
	if toAdmins != 1 || queue.reports[0] != fmt.Sprintf("r1 SENT Sent %d emails.", len(mail.sent)-1) {
		t.Fatalf("sent %d (%d to the rota admins), reports %v", len(mail.sent), toAdmins, queue.reports)
	}
}
