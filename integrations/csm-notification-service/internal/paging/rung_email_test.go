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
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

// --- fakes -------------------------------------------------------------------

type sentEmail struct {
	to, cc        []string
	subject, body string
}

type fakeEmail struct {
	sent []sentEmail
	// fail is returned instead of sending, while set.
	fail error
	// onSend runs inside the next send, once, as if it landed mid-flight.
	onSend func()
}

func (f *fakeEmail) SendEmail(_ context.Context, to, cc, _, _ []string, subject, body string, _ []notifications.EmailAttachment) error {
	if f.fail != nil {
		return f.fail
	}
	if hook := f.onSend; hook != nil {
		f.onSend = nil
		hook()
	}
	f.sent = append(f.sent, sentEmail{to: to, cc: cc, subject: subject, body: body})
	return nil
}

// emailResolver is a StaticResolver that also names a team's lead and the
// whole ABT lead pool, as the Team Schedule does.
type emailResolver struct {
	StaticResolver
	leads map[string][]Recipient
	pool  []Recipient
}

func (r emailResolver) TeamLead(_ context.Context, team string) ([]Recipient, error) {
	return r.leads[team], nil
}

func (r emailResolver) LeadPool(context.Context) ([]Recipient, error) { return r.pool, nil }

func person(name, email, phone string) Recipient {
	return Recipient{Name: name, Email: email, Phone: phone}
}

var (
	atlasLead    = person("Atlas Lead", "atlas.lead@example.com", "+94770000101")
	castorLead   = person("Castor Lead", "castor.lead@example.com", "+94770000102")
	creHead      = person("Cre Head", "cre.head@example.com", "+94770000103")
	csHead       = person("Cs Head", "cs.head@example.com", "+94770000104")
	amLeadA      = person("Am Lead A", "am.a@example.com", "+14150000001")
	amLeadB      = person("Am Lead B", "am.b@example.com", "+14150000002")
	americasLead = person("America Lead", "am.top@example.com", "+14150000003")
)

// dayResolver is the day ladder: L1 the case team's lead, L2 one lead from the
// pool (the call reaches a few; the email reaches all), then the heads.
func dayResolver() emailResolver {
	return emailResolver{
		StaticResolver: StaticResolver{ByLevel: map[Level][]Recipient{
			Level1: {atlasLead},
			Level2: {castorLead},
			Level3: {creHead},
			Level4: {csHead},
		}},
		leads: map[string][]Recipient{"Atlas": {atlasLead}},
		pool:  []Recipient{atlasLead, castorLead},
	}
}

// nightResolver is the night ladder: L1 the Americas team leads, L2 the
// America Team lead.
func nightResolver() emailResolver {
	r := dayResolver()
	r.ByLevel = map[Level][]Recipient{
		Level1: {amLeadA, amLeadB},
		Level2: {americasLead},
		Level3: {creHead},
		Level4: {csHead},
	}
	return r
}

type emailRig struct {
	e     *Engine
	store *memStore
	mail  *fakeEmail
	calls *fakeCaller
	now   time.Time
}

func newEmailRig(res Resolver, start time.Time) *emailRig {
	rig := &emailRig{store: newMemStore(), mail: &fakeEmail{}, calls: &fakeCaller{}, now: start}
	cfg := EngineConfig{CallSendingEnabled: true, Channel: ChannelCall, Kind: LadderCRE}
	cfg.Ladder.Teams = TeamKeys{ABTs: []string{"atlas", "castor"}, Americas: "americas"}
	cfg.Ladder.Email = RungEmail{Enabled: true, TeamLists: map[string]string{
		"atlas":    "atlas-team@example.com",
		"castor":   "castor-team@example.com",
		"americas": "americas-team@example.com",
	}, CRETeamList: "cre-team@example.com", CSTeamList: "cs-team@example.com"}
	rig.e = &Engine{
		policies:     DefaultPolicy,
		resolver:     res,
		notifiers:    []notifier{voiceNotifier{calls: rig.calls}},
		store:        rig.store,
		notes:        &fakeNotes{},
		cfg:          cfg,
		links:        PortalLinks{CSMBaseURL: "https://csm.example"},
		emails:       rig.mail,
		emailSending: RungEmailSending{Enabled: true},
		clock:        func() time.Time { return rig.now },
	}
	return rig
}

// caseCreatedAt is a support case created at the given instant.
func caseCreatedAt(t *testing.T, severity string, at time.Time) eventbus.Record {
	return record(t, events.TypeCaseCreated, events.CaseCreatedPayload{
		ReporterName: "Jane Customer", ProjectName: "Acme", ProjectID: "p-1", CaseID: testIncidentID,
		CaseNumber: "CS0099001", CaseTitle: "Gateway down", CaseType: "CASE", Priority: severity,
		Team: "Atlas", CreatedAt: at.Format(time.RFC3339), Description: "everything fails",
		Recipients: []string{"watcher@example.com"},
	})
}

func (r *emailRig) handle(t *testing.T, rec eventbus.Record) {
	t.Helper()
	if err := r.e.Handle(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
}

// runUntil ticks once a minute up to end, as the ticker would.
func (r *emailRig) runUntil(t *testing.T, end time.Time) {
	t.Helper()
	for r.now.Before(end) {
		r.now = r.now.Add(time.Minute)
		_ = r.e.Tick(context.Background(), r.now)
	}
}

// --- tests -------------------------------------------------------------------

// The agreed ladder by day (2026-10-13): four emails, one per rung, each To
// its rung and Cc its team list, all on one subject.
func TestRungEmail_DayRecipientsPerRung(t *testing.T) {
	rig := newEmailRig(dayResolver(), testClock)
	rig.handle(t, caseCreatedAt(t, "CRITICAL", testClock))
	rig.runUntil(t, testClock.Add(4*time.Hour))

	sent := rig.mail.sent
	if len(sent) != 4 {
		t.Fatalf("sent %d emails, want 4 (L1..L4)", len(sent))
	}
	want := []struct {
		to, cc   []string
		greeting string
	}{
		{[]string{"atlas.lead@example.com"}, []string{"atlas-team@example.com"}, "Hi Atlas Lead,"},
		{[]string{"atlas.lead@example.com", "castor.lead@example.com"}, []string{"atlas-team@example.com", "castor-team@example.com"}, "Hi ABT Team Leads,"},
		{[]string{"cre.head@example.com"}, []string{"cre-team@example.com"}, "Hi Cre Head,"},
		{[]string{"cs.head@example.com"}, []string{"cs-team@example.com"}, "Hi Cs Head,"},
	}
	for i, w := range want {
		if !reflect.DeepEqual(sent[i].to, w.to) {
			t.Errorf("L%d To = %v, want %v", i+1, sent[i].to, w.to)
		}
		if !reflect.DeepEqual(sent[i].cc, w.cc) {
			t.Errorf("L%d Cc = %v, want %v", i+1, sent[i].cc, w.cc)
		}
		if !strings.Contains(sent[i].body, w.greeting) {
			t.Errorf("L%d body has no %q", i+1, w.greeting)
		}
	}
	for i, line := range []string{
		"S1 alert at L1 for Atlas ABT Team Lead",
		"S1 alert at L2 for all ABT Team Leads",
		"S1 alert at L3 for CRE Head",
		"S1 alert at L4 for CS Head",
	} {
		// One subject for the whole chain, so a case's emails group into one
		// conversation; the rung is the body's first line.
		if want := "[Case Alerts] S1 CS0099001 not acknowledged"; sent[i].subject != want {
			t.Errorf("L%d subject = %q, want %q", i+1, sent[i].subject, want)
		}
		if !strings.Contains(sent[i].body, `">`+line+"</p>") {
			t.Errorf("L%d body does not open with %q", i+1, line)
		}
	}
	if !strings.Contains(sent[3].body, "Atlas Lead (Atlas ABT Team Lead)") || !strings.Contains(sent[3].body, "All ABT Team Leads, ") ||
		!strings.Contains(sent[3].body, "Cre Head (CRE Head)") || !strings.Contains(sent[3].body, "CSM Case Alerts") {
		t.Error("the L4 email does not list who was already notified")
	}
	if _, found, _ := rig.store.Get(context.Background(), testIncidentID); found {
		t.Error("the ladder is still tracked after every call and email settled")
	}
}

func containsAddr(list []string, addr string) bool {
	for _, a := range list {
		if a == addr {
			return true
		}
	}
	return false
}

// Each email goes when its rung opens -- the first call of that rung -- not
// before.
func TestRungEmail_SentWhenTheRungOpens(t *testing.T) {
	rig := newEmailRig(dayResolver(), testClock)
	rig.handle(t, caseCreatedAt(t, "CRITICAL", testClock))
	st, _, _ := rig.store.Get(context.Background(), testIncidentID)
	opens := rungOpenTimes(st.Plan)
	for _, m := range st.Plan.Emails {
		if !m.At.Equal(opens[m.Level]) {
			t.Errorf("%s email at %s; the rung opens at %s", m.Level, m.At, opens[m.Level])
		}
	}
	rig.runUntil(t, st.Plan.Emails[1].At.Add(-time.Minute))
	if len(rig.mail.sent) != 1 {
		t.Fatalf("a minute before L2 opens, %d emails sent; want only L1's", len(rig.mail.sent))
	}
}

// Acknowledged (assigned + the assignee's public comment): no further email.
func TestRungEmail_AcknowledgementStopsTheEmails(t *testing.T) {
	rig := newEmailRig(dayResolver(), testClock)
	rig.handle(t, caseCreatedAt(t, "CRITICAL", testClock))
	st, _, _ := rig.store.Get(context.Background(), testIncidentID)
	rig.runUntil(t, st.Plan.Emails[0].At)
	if len(rig.mail.sent) != 1 {
		t.Fatalf("sent %d before the acknowledgement, want 1", len(rig.mail.sent))
	}
	rig.handle(t, caseAssigned(t, "eng@example.com"))
	rig.handle(t, caseComment(t, "eng@example.com", false, true))
	// The stop drops the pending email wakes with the calls', rather than
	// leaving them for the ticker to find and discard.
	if n := len(rig.store.wakes); n != 0 {
		t.Fatalf("%d wake entries left after the acknowledgement", n)
	}
	rig.runUntil(t, testClock.Add(4*time.Hour))
	if len(rig.mail.sent) != 1 {
		t.Fatalf("sent %d emails in all; the acknowledgement should have stopped them at 1", len(rig.mail.sent))
	}
}

// A closed case stops them too.
func TestRungEmail_ClosedCaseStopsTheEmails(t *testing.T) {
	rig := newEmailRig(dayResolver(), testClock)
	rig.handle(t, caseCreatedAt(t, "CRITICAL", testClock))
	rig.handle(t, caseStatus(t, "Closed"))
	rig.runUntil(t, testClock.Add(4*time.Hour))
	if len(rig.mail.sent) != 0 {
		t.Fatalf("sent %d emails for a closed case", len(rig.mail.sent))
	}
}

// A redelivered case.created, and two replicas ticking the same instant,
// still send each rung's email once.
func TestRungEmail_RedeliveryAndDoubleTicksSendOnce(t *testing.T) {
	rig := newEmailRig(dayResolver(), testClock)
	rig.handle(t, caseCreatedAt(t, "CRITICAL", testClock))
	rig.handle(t, caseCreatedAt(t, "CRITICAL", testClock))
	for rig.now.Before(testClock.Add(4 * time.Hour)) {
		rig.now = rig.now.Add(time.Minute)
		_ = rig.e.Tick(context.Background(), rig.now)
		_ = rig.e.Tick(context.Background(), rig.now)
	}
	if len(rig.mail.sent) != 4 {
		t.Fatalf("sent %d emails, want exactly 4", len(rig.mail.sent))
	}
}

// The calls are held until every ABT lead has a number; the emails need no
// number and are not held.
func TestRungEmail_SentWhileTheCallsAreHeld(t *testing.T) {
	res := dayResolver()
	res.pool = []Recipient{atlasLead, person("No Phone", "nophone@example.com", "")}
	rig := newEmailRig(res, testClock)
	rig.handle(t, caseCreatedAt(t, "CRITICAL", testClock))
	rig.runUntil(t, testClock.Add(4*time.Hour))
	if len(rig.calls.placed) != 0 {
		t.Fatalf("placed %d calls with the lead pool unverified", len(rig.calls.placed))
	}
	if len(rig.mail.sent) != 4 {
		t.Fatalf("sent %d emails while the calls were held, want 4", len(rig.mail.sent))
	}
}

// Somebody the call plan drops for want of a number is still emailed.
func TestRungEmail_ReachesARecipientWithNoNumber(t *testing.T) {
	res := dayResolver()
	res.ByLevel[Level3] = []Recipient{person("Cre Head", "cre.head@example.com", "")}
	rig := newEmailRig(res, testClock)
	rig.handle(t, caseCreatedAt(t, "CRITICAL", testClock))
	rig.runUntil(t, testClock.Add(4*time.Hour))
	if len(rig.mail.sent) != 4 || rig.mail.sent[2].to[0] != "cre.head@example.com" {
		t.Fatalf("the CRE head with no number was not emailed at L3: %+v", rig.mail.sent)
	}
}

// At night L1 and L2 ADD the Americas to the day recipients: the Americas
// team leads at L1, the America Team lead at L2, and the Americas team list in
// both Cc's. L3 and L4 are as by day.
func TestRungEmail_NightAddsTheAmericas(t *testing.T) {
	night := ist(2026, 9, 9, 22, 30) // a Wednesday, USA shift
	rig := newEmailRig(nightResolver(), night)
	rig.handle(t, caseCreatedAt(t, "CRITICAL", night))
	rig.runUntil(t, night.Add(4*time.Hour))
	sent := rig.mail.sent
	if len(sent) != 4 {
		t.Fatalf("sent %d emails, want 4", len(sent))
	}
	want := []struct {
		to, cc   []string
		greeting string
	}{
		{[]string{"atlas.lead@example.com", "am.a@example.com", "am.b@example.com"},
			[]string{"atlas-team@example.com", "americas-team@example.com"}, "Hi Atlas Lead and Americas Team Leads,"},
		{[]string{"atlas.lead@example.com", "castor.lead@example.com", "am.top@example.com"},
			[]string{"atlas-team@example.com", "castor-team@example.com", "americas-team@example.com"}, "Hi ABT Team Leads and America Lead,"},
		{[]string{"cre.head@example.com"}, []string{"cre-team@example.com"}, "Hi Cre Head,"},
		{[]string{"cs.head@example.com"}, []string{"cs-team@example.com"}, "Hi Cs Head,"},
	}
	for i, w := range want {
		if !reflect.DeepEqual(sent[i].to, w.to) || !reflect.DeepEqual(sent[i].cc, w.cc) {
			t.Errorf("night L%d To %v Cc %v; want To %v Cc %v", i+1, sent[i].to, sent[i].cc, w.to, w.cc)
		}
		if !strings.Contains(sent[i].body, w.greeting) {
			t.Errorf("night L%d body has no %q", i+1, w.greeting)
		}
	}
	for i, line := range []string{
		"S1 alert at L1 for Atlas ABT Team Lead and Americas Team Leads",
		"S1 alert at L2 for all ABT Team Leads and America Team Lead",
		"S1 alert at L3 for CRE Head",
		"S1 alert at L4 for CS Head",
	} {
		if want := "[Case Alerts] S1 CS0099001 not acknowledged"; sent[i].subject != want {
			t.Errorf("night L%d subject = %q, want %q", i+1, sent[i].subject, want)
		}
		if !strings.Contains(sent[i].body, `">`+line+"</p>") {
			t.Errorf("night L%d body does not open with %q", i+1, line)
		}
	}
	if !strings.Contains(sent[2].body, "Atlas ABT Team Lead and Americas Team Leads, ") ||
		!strings.Contains(sent[2].body, "All ABT Team Leads and America Team Lead, ") {
		t.Error("the night L3 email does not name the night rungs already notified")
	}
}

// By day the Americas are never on an email, whatever the lists hold.
func TestRungEmail_DayHasNoAmericas(t *testing.T) {
	rig := newEmailRig(dayResolver(), testClock)
	rig.handle(t, caseCreatedAt(t, "CRITICAL", testClock))
	rig.runUntil(t, testClock.Add(4*time.Hour))
	for i, m := range rig.mail.sent {
		if containsAddr(m.cc, "americas-team@example.com") || strings.Contains(m.body, "America") {
			t.Errorf("day L%d mentions the Americas: %q Cc %v", i+1, m.subject, m.cc)
		}
	}
}

// A rung whose list is not configured sends with no Cc rather than not at all.
func TestRungEmail_MissingListSendsWithoutCc(t *testing.T) {
	rig := newEmailRig(dayResolver(), testClock)
	rig.e.cfg.Ladder.Email.CRETeamList = ""
	rig.handle(t, caseCreatedAt(t, "CRITICAL", testClock))
	rig.runUntil(t, testClock.Add(4*time.Hour))
	if len(rig.mail.sent) != 4 || rig.mail.sent[2].cc != nil {
		t.Fatalf("L3 with no creTeamList: %d sent, Cc %v", len(rig.mail.sent), rig.mail.sent[2].cc)
	}
}

// The case team's list is found through teams.aliases, as the team is.
func TestRungEmail_CaseTeamListThroughAliases(t *testing.T) {
	rig := newEmailRig(dayResolver(), testClock)
	rig.e.cfg.Ladder.Teams.Aliases = map[string]string{"Atlas": "castor"}
	rig.handle(t, caseCreatedAt(t, "CRITICAL", testClock))
	st, _, _ := rig.store.Get(context.Background(), testIncidentID)
	if got := st.Plan.Emails[0].Cc; !reflect.DeepEqual(got, []string{"castor-team@example.com"}) {
		t.Fatalf("L1 Cc = %v; want the aliased team's list", got)
	}
}

// A severity change starts a new chain, and with it a new series from L1.
func TestRungEmail_SeverityChangeStartsANewSeries(t *testing.T) {
	rig := newEmailRig(dayResolver(), testClock)
	rig.handle(t, caseCreatedAt(t, "HIGH", testClock))
	st, _, _ := rig.store.Get(context.Background(), testIncidentID)
	rig.runUntil(t, st.Plan.Emails[0].At)
	rig.handle(t, severityChanged(t, "HIGH", "CRITICAL"))
	st, _, _ = rig.store.Get(context.Background(), testIncidentID)
	rig.runUntil(t, st.Plan.Emails[0].At)
	if len(rig.mail.sent) != 2 {
		t.Fatalf("sent %d emails, want L1 of each chain", len(rig.mail.sent))
	}
	if !strings.HasPrefix(rig.mail.sent[1].subject, "[Case Alerts] S1 ") || rig.mail.sent[1].to[0] != "atlas.lead@example.com" {
		t.Errorf("the new chain's first email = %q to %v; want an S1 L1 email", rig.mail.sent[1].subject, rig.mail.sent[1].to)
	}
}

// A severity change landing while an email is in flight replaces the chain;
// the old send must not mark the new chain's email at the same index as sent,
// nor drop its wake entry (the member is the same).
func TestRungEmail_SeverityChangeDuringSendLeavesTheNewChainAlone(t *testing.T) {
	rig := newEmailRig(dayResolver(), testClock)
	rig.handle(t, caseCreatedAt(t, "HIGH", testClock))
	st, _, _ := rig.store.Get(context.Background(), testIncidentID)
	rig.mail.onSend = func() { rig.handle(t, severityChanged(t, "HIGH", "CRITICAL")) }
	rig.runUntil(t, st.Plan.Emails[0].At)

	fresh, found, _ := rig.store.Get(context.Background(), testIncidentID)
	if !found || NormalisePriority(fresh.Plan.Trigger.Priority) != "P1" {
		t.Fatalf("the new S1 chain is not running")
	}
	if fresh.emailDone(0) {
		t.Fatal("the old chain's send marked the new chain's L1 email as sent")
	}
	if _, ok := rig.store.wakes[emailWakeMember(testIncidentID, 0)]; !ok {
		t.Fatal("the old chain's send dropped the new chain's L1 wake entry")
	}
	rig.runUntil(t, fresh.Plan.Emails[0].At)
	if len(rig.mail.sent) != 2 || !strings.HasPrefix(rig.mail.sent[1].subject, "[Case Alerts] S1 ") {
		t.Fatalf("sent %d emails; want the old chain's L1 and then the new chain's", len(rig.mail.sent))
	}
}

// Off unless cre.email.enabled: the plan carries no email and nothing is sent.
func TestRungEmail_OffByDefault(t *testing.T) {
	rig := newEmailRig(dayResolver(), testClock)
	rig.e.cfg.Ladder.Email.Enabled = false
	rig.handle(t, caseCreatedAt(t, "CRITICAL", testClock))
	st, _, _ := rig.store.Get(context.Background(), testIncidentID)
	if len(st.Plan.Emails) != 0 || st.EmailSent != nil {
		t.Fatalf("plan has %d emails with cre.email off", len(st.Plan.Emails))
	}
	rig.runUntil(t, testClock.Add(4*time.Hour))
	if len(rig.mail.sent) != 0 {
		t.Fatalf("sent %d emails with cre.email off", len(rig.mail.sent))
	}
}

// The SRE ladder never sends a rung email, whatever its file says.
func TestRungEmail_NeverOnTheSRELadder(t *testing.T) {
	rig := newEmailRig(dayResolver(), testClock)
	rig.e.cfg.Kind = LadderSRE
	if rig.e.rungEmailsOn(Trigger{Record: RecordCase}) {
		t.Fatal("the SRE engine would email the rungs")
	}
}

// EMAIL_DEBUG_MODE: To the debug list, no Cc, and the body says who it was for.
func TestRungEmail_DebugModeRedirects(t *testing.T) {
	rig := newEmailRig(dayResolver(), testClock)
	rig.e.emailSending = RungEmailSending{Enabled: true, DebugMode: true, DebugRecipients: []string{"qa@example.com"}}
	rig.handle(t, caseCreatedAt(t, "CRITICAL", testClock))
	rig.runUntil(t, testClock.Add(4*time.Hour))
	for i, m := range rig.mail.sent {
		if !reflect.DeepEqual(m.to, []string{"qa@example.com"}) || m.cc != nil {
			t.Fatalf("email %d went to %v cc %v in debug mode", i, m.to, m.cc)
		}
	}
	if len(rig.mail.sent) != 4 || !strings.Contains(rig.mail.sent[1].body, "Expected sent to: atlas.lead@example.com, castor.lead@example.com, atlas-team@example.com, castor-team@example.com") {
		t.Fatal("the debug copy does not say who it was meant for")
	}
}

// EMAIL_SENDING_ENABLED=false sends nothing, and the ladder still finishes.
func TestRungEmail_EmailSendingDisabled(t *testing.T) {
	rig := newEmailRig(dayResolver(), testClock)
	rig.e.emailSending.Enabled = false
	rig.handle(t, caseCreatedAt(t, "CRITICAL", testClock))
	rig.runUntil(t, testClock.Add(4*time.Hour))
	if len(rig.mail.sent) != 0 {
		t.Fatalf("sent %d emails with email sending off", len(rig.mail.sent))
	}
	if _, found, _ := rig.store.Get(context.Background(), testIncidentID); found {
		t.Error("the ladder never settled with email sending off")
	}
}

// CALL_SENDING_ENABLED=false silences the phones, not the email.
func TestRungEmail_CallSendingDisabledStillEmails(t *testing.T) {
	rig := newEmailRig(dayResolver(), testClock)
	rig.e.cfg.CallSendingEnabled = false
	rig.handle(t, caseCreatedAt(t, "CRITICAL", testClock))
	rig.runUntil(t, testClock.Add(4*time.Hour))
	if len(rig.calls.placed) != 0 || len(rig.mail.sent) != 4 {
		t.Fatalf("calls %d emails %d; want 0 calls and 4 emails", len(rig.calls.placed), len(rig.mail.sent))
	}
}

// safety.maxLevel keeps the emails away from the heads as it does the calls.
func TestRungEmail_MaxLevelCapsTheEmails(t *testing.T) {
	rig := newEmailRig(dayResolver(), testClock)
	two := int(Level2)
	rig.e.cfg.Ladder.Safety.MaxLevel = &two
	rig.handle(t, caseCreatedAt(t, "CRITICAL", testClock))
	rig.runUntil(t, testClock.Add(4*time.Hour))
	if len(rig.mail.sent) != 2 {
		t.Fatalf("sent %d emails with maxLevel 2, want L1 and L2", len(rig.mail.sent))
	}
}

// A transient failure is retried next tick; one that persists is given up on
// after emailGiveUpAfter, so the ladder can still finish.
func TestRungEmail_TransientFailureRetriesThenGivesUp(t *testing.T) {
	rig := newEmailRig(dayResolver(), testClock)
	rig.handle(t, caseCreatedAt(t, "CRITICAL", testClock))
	st, _, _ := rig.store.Get(context.Background(), testIncidentID)
	l1 := st.Plan.Emails[0].At

	rig.mail.fail = errors.New("connection reset")
	rig.runUntil(t, l1.Add(2*time.Minute))
	rig.mail.fail = nil
	rig.runUntil(t, l1.Add(3*time.Minute))
	if len(rig.mail.sent) != 1 {
		t.Fatalf("after recovering, sent %d L1 emails, want 1", len(rig.mail.sent))
	}

	rig.mail.fail = errors.New("connection reset")
	rig.runUntil(t, testClock.Add(4*time.Hour))
	if _, found, _ := rig.store.Get(context.Background(), testIncidentID); found {
		t.Error("a persistently failing email kept the ladder alive")
	}
}

// A rejection (4xx) is not retried.
func TestRungEmail_RejectionIsNotRetried(t *testing.T) {
	rig := newEmailRig(dayResolver(), testClock)
	rig.handle(t, caseCreatedAt(t, "CRITICAL", testClock))
	st, _, _ := rig.store.Get(context.Background(), testIncidentID)
	rig.mail.fail = &apierror.Error{StatusCode: 400}
	rig.runUntil(t, st.Plan.Emails[0].At)
	st, _, _ = rig.store.Get(context.Background(), testIncidentID)
	if !st.emailDone(0) {
		t.Fatal("a rejected email is still pending a retry")
	}
}

// A ladder stored before rung emails existed has none, and still settles.
func TestRungEmail_LadderWithoutEmailsSettlesAsBefore(t *testing.T) {
	st := LadderState{Plan: Plan{Calls: []PlannedCall{{}}}, Placed: []bool{true}}
	if !st.AllSettled() {
		t.Fatal("a ladder with no emails is not settled once its calls are")
	}
	if got := pendingMembers("x", st); len(got) != 0 {
		t.Fatalf("pending = %v", got)
	}
}

func TestEmailSeverity(t *testing.T) {
	for in, want := range map[string]string{"CATASTROPHIC": "S0 – Catastrophic", "CRITICAL": "S1 – Critical", "S2": "S2 – High", "MEDIUM": "S3 – Medium", "LOW": "S4 – Low"} {
		if _, got := emailSeverity(in); got != want {
			t.Errorf("emailSeverity(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestJoinNames(t *testing.T) {
	if got := joinNames([]Recipient{atlasLead, amLeadA, amLeadB}); got != "Atlas Lead, Am Lead A and Am Lead B" {
		t.Errorf("joinNames = %q", got)
	}
}

// --- configuration ----------------------------------------------------------

func TestConfig_RungEmail(t *testing.T) {
	cfg, err := loadYAML(t, `
enabled: true
cre:
  enabled: true
  teams:
    abts: [atlas, castor]
    americas: americas
  email:
    enabled: true
    teamLists:
      atlas: atlas-team@example.com
      americas: americas-team@example.com
    creTeamList: cre-team@example.com
    csTeamList: cs-team@example.com
`)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.CRE.Email.Enabled || cfg.CRE.Email.TeamLists["atlas"] != "atlas-team@example.com" {
		t.Fatalf("email = %+v", cfg.CRE.Email)
	}

	for name, body := range map[string]string{
		"on the SRE ladder": `
sre:
  email:
    enabled: true
`,
		"not an address": `
cre:
  teams: {abts: [atlas]}
  email:
    teamLists: {atlas: "atlas-team"}
`,
		"two addresses": `
cre:
  teams: {abts: [atlas]}
  email:
    teamLists: {atlas: "a@example.com, b@example.com"}
`,
		"an unknown team": `
cre:
  teams: {abts: [atlas]}
  email:
    teamLists: {vega: vega-team@example.com}
`,
		"a bad CRE team list": `
cre:
  email:
    creTeamList: "cre team@example.com"
`,
		"a CS team list on the SRE ladder": `
sre:
  email:
    csTeamList: cs-team@example.com
`,
		"a misspelled key": `
cre:
  email:
    enable: true
`,
	} {
		if _, err := loadYAML(t, body); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// The email list stays off the address-free logs and off the SRE file: the
// default configuration has no rung email.
func TestConfig_RungEmailOffByDefault(t *testing.T) {
	if DefaultConfig().CRE.Email.Enabled {
		t.Fatal("rung email on by default")
	}
}

// Every email wake member parses back to its ladder and index, and is told
// apart from a call's.
func TestEmailWakeMember_RoundTrips(t *testing.T) {
	id, i, ok := parseWakeMember(emailWakeMember(testIncidentID, 3))
	key, isEmail := strings.CutSuffix(id, emailWakeTag)
	if !ok || !isEmail || key != testIncidentID || i != 3 {
		t.Fatalf("parsed (%q, %d, %v)", id, i, ok)
	}
	if _, isEmail := strings.CutSuffix(testIncidentID, emailWakeTag); isEmail {
		t.Fatal("a call's ladder key reads as an email's")
	}
}

// cre.email lives in the file, not on the Alert Settings tab: merging the
// saved settings over the small file keeps it exactly as the file has it.
func TestRungEmail_ConfigSurvivesSavedSettings(t *testing.T) {
	full, errFull := LoadConfig("../../../../scripts/csm-compose/paging-alert.yaml")
	small, errSmall := LoadConfig("../../../../scripts/csm-compose/paging-alert.minimal.yaml")
	if errFull != nil || errSmall != nil {
		t.Fatalf("shipped examples: %v / %v", errFull, errSmall)
	}
	small.CRE.Email = RungEmail{Enabled: true, TeamLists: map[string]string{"atlas": "atlas-team@example.com"},
		CRETeamList: "cre-team@example.com", CSTeamList: "cs-team@example.com"}
	saved := savedFrom(full)
	saved.Values = valuesFrom(t, full)
	merged, _, _, err := MergeCaseAlertSettings(small, saved)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if !reflect.DeepEqual(merged.CRE.Email, small.CRE.Email) {
		t.Fatalf("cre.email after the merge = %+v, want the file's %+v", merged.CRE.Email, small.CRE.Email)
	}
}

// The opening says the case is still open and how long is left on its
// response SLA, from the fetched policy when there is one, else the published
// default; past the deadline it says by how much it was breached.
func TestRungEmail_OpeningCountsDownTheResponseSLA(t *testing.T) {
	e := &Engine{}
	at := testClock
	s0 := Trigger{Number: "CS1", Priority: "CATASTROPHIC", At: at}
	for _, tc := range []struct {
		name string
		t    Trigger
		now  time.Time
		sla  map[string]map[string]time.Duration
		want string
	}{
		{"S0 default, 14m left", s0, at.Add(time.Minute), nil,
			"Case CS1 (S0) is still in open state and response SLA remaining time is 14m."},
		{"S0 default, breached", s0, at.Add(18 * time.Minute), nil,
			"Case CS1 (S0) is still in open state and the response SLA was breached 3m ago."},
		{"S1 fetched policy wins", Trigger{Number: "CS2", Priority: "CRITICAL", At: at}, at.Add(18 * time.Minute),
			map[string]map[string]time.Duration{"CRITICAL": {"response": 2 * time.Hour}},
			"Case CS2 (S1) is still in open state and response SLA remaining time is 1h42m."},
		{"S3 default, hours", Trigger{Number: "CS3", Priority: "MEDIUM", At: at}, at.Add(90 * time.Minute), nil,
			"Case CS3 (S3) is still in open state and response SLA remaining time is 4h30m."},
		{"unknown severity", Trigger{Number: "CS4", Priority: "WHATEVER", At: at}, at, nil,
			"Case CS4 (WHATEVER) is still in open state."},
	} {
		e.emailSending.SLADurations = tc.sla
		code, _ := emailSeverity(tc.t.Priority)
		if got := e.rungEmailOpening(tc.t, code, tc.now); got != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}
}

// Every rung's email opens with the SLA countdown, L1 included.
func TestRungEmail_EveryRungOpensWithTheSLA(t *testing.T) {
	rig := newEmailRig(dayResolver(), testClock)
	rig.handle(t, caseCreatedAt(t, "CRITICAL", testClock))
	rig.runUntil(t, testClock.Add(4*time.Hour))
	if len(rig.mail.sent) != 4 {
		t.Fatalf("sent %d emails, want 4", len(rig.mail.sent))
	}
	for i, m := range rig.mail.sent {
		if !strings.Contains(m.body, "Case CS0099001 (S1) is still in open state and ") {
			t.Errorf("L%d does not open with the SLA sentence", i+1)
		}
	}
}
