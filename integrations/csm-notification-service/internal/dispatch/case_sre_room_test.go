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
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/chataudience"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

func caseCreatedWith(priority, sreGroup string) eventbus.Record {
	return caseCreatedOfType("CASE", priority, sreGroup)
}

func caseCreatedOfType(caseType, priority, sreGroup string) eventbus.Record {
	return eventbus.Record{Value: []byte(fmt.Sprintf(`{"type":"case.created","entityId":"CASE-1","payload":{"reporterName":"Reporter","projectName":"Proj","projectId":"PROJ-1","caseId":"CASE-1","caseNumber":"CS0001001","wso2CaseId":"WSO2-1001","caseTitle":"Something broke","caseType":%q,"priority":%q,"product":"api-manager","team":"Phoenix","sreTeamName":%q,"createdAt":"2026-01-01","description":"desc","recipients":["test-recipient@example.com"]}}`, caseType, priority, sreGroup))}
}

// caseCardAudiences is where the Incident Monitor "Case Reported" card went.
func caseCardAudiences(chat *mockGoogleChatSender) []string {
	chat.mu.Lock()
	defer chat.mu.Unlock()
	var out []string
	for _, c := range chat.caseCreatedCalls {
		out = append(out, c.audience)
	}
	return out
}

// roomCards is the SRE room cards: the incident card in its case form.
func roomCards(chat *mockGoogleChatSender) []sentIncidentCreatedAlert {
	chat.mu.Lock()
	defer chat.mu.Unlock()
	return slices.Clone(chat.incidentCreatedCalls)
}

// roomAudiences is the rooms the card went to, sorted.
func roomAudiences(chat *mockGoogleChatSender) []string {
	var out []string
	for _, c := range roomCards(chat) {
		out = append(out, c.audience)
	}
	slices.Sort(out)
	return out
}

func sreRoomDispatcher(chat googleChatSender) *Dispatcher {
	return sreRoomDispatcherWith(&mockEmailSender{}, chat)
}

func sreRoomDispatcherWith(email emailSender, chat googleChatSender) *Dispatcher {
	routes := &notifications.IncidentRoutes{}
	routes.Set(map[string][]string{"Default SRE Group": {"apollo", "artemis"}})
	routes.SetTeamless([]string{"apollo", "artemis"})
	// WSO2 IaaS Team is the one known SRE team; U2 SRE Group is not one.
	routes.SetKnownSRETeam(func(group string) bool { return strings.EqualFold(group, "WSO2 IaaS Team") })
	return NewDispatcher(email, chat, &mockCallSender{}, &mockLinkResolver{}, true, false, nil, true, "", nil).
		WithIncidentRoutes(routes)
}

func TestDispatcher_S0CaseCardGoesToItsSRERoom(t *testing.T) {
	cases := []struct {
		name     string
		priority string
		sreGroup string
		rooms    []string
	}{
		{"an S0 on a mapped SRE group goes to its team's room", "CATASTROPHIC", "WSO2 IaaS Team",
			[]string{notifications.IncidentAudience("WSO2 IaaS Team")}},
		{"an S0 on an SRE group the ladder does not know goes to the teamless rooms, where it is paged", "CATASTROPHIC", "U2 SRE Group",
			[]string{notifications.IncidentAudience("apollo"), notifications.IncidentAudience("artemis")}},
		{"an S0 on an account with no SRE team goes to the teamless rooms", "CATASTROPHIC", "",
			[]string{notifications.IncidentAudience("apollo"), notifications.IncidentAudience("artemis")}},
		{"an S1 gets no room card", "CRITICAL", "WSO2 IaaS Team", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chat := &mockGoogleChatSender{hasAudienceSpace: mappedSpaces}
			if err := sreRoomDispatcher(chat).Handle(context.Background(), caseCreatedWith(tc.priority, tc.sreGroup)); err != nil {
				t.Fatalf("Handle error = %v", err)
			}
			// The Incident Monitor card is untouched whatever the case.
			if got := caseCardAudiences(chat); !slices.Equal(got, []string{chataudience.IncidentMonitor}) {
				t.Errorf("Case Reported card went to %v; want only Incident Monitor", got)
			}
			if got := roomAudiences(chat); !slices.Equal(got, tc.rooms) {
				t.Errorf("room card went to %v; want %v", got, tc.rooms)
			}
		})
	}
}

func TestDispatcher_S0CaseRoomCardIsTheIncidentCardInCaseForm(t *testing.T) {
	chat := &mockGoogleChatSender{hasAudienceSpace: mappedSpaces}
	if err := sreRoomDispatcher(chat).Handle(context.Background(), caseCreatedWith("CATASTROPHIC", "WSO2 IaaS Team")); err != nil {
		t.Fatalf("Handle error = %v", err)
	}
	cards := roomCards(chat)
	if len(cards) != 1 {
		t.Fatalf("room cards = %d; want 1", len(cards))
	}
	a := cards[0].alert
	// IncidentID is the case id, so the SRE ladder's rung cards (thread incident-<caseId>) reply under it.
	// Number is the WSO2 case ID, as the SRE ladder's replies name the case (Trigger.caseRef).
	if !a.Case || a.IncidentID != "CASE-1" || a.Number != "WSO2-1001" || a.Team != "WSO2 IaaS Team" ||
		a.Priority != "CATASTROPHIC" || a.ShortDescription != "Something broke" || a.Service != "api-manager" || a.Project != "Proj" {
		t.Errorf("room card = %+v; want the case form of the incident card for CASE-1", a)
	}
}

func TestDispatcher_SeverityRaisedToS0GoesToItsSRERoom(t *testing.T) {
	for _, tc := range []struct {
		newSeverity string
		rooms       []string
	}{
		{"CATASTROPHIC", []string{notifications.IncidentAudience("WSO2 IaaS Team")}},
		{"CRITICAL", nil},
	} {
		t.Run(tc.newSeverity, func(t *testing.T) {
			chat := &mockGoogleChatSender{hasAudienceSpace: mappedSpaces}
			record := eventbus.Record{Value: []byte(fmt.Sprintf(`{"type":"case.severity_changed","entityId":"CASE-1","payload":{"projectId":"PROJ-1","caseId":"CASE-1","caseNumber":"CS0001001","caseTitle":"Something broke","oldSeverity":"HIGH","newSeverity":%q,"product":"api-manager","team":"Phoenix","sreTeamName":"WSO2 IaaS Team","recipients":["test-recipient@example.com"]}}`, tc.newSeverity))}
			if err := sreRoomDispatcher(chat).Handle(context.Background(), record); err != nil {
				t.Fatalf("Handle error = %v", err)
			}
			chat.mu.Lock()
			var monitor []string
			for _, c := range chat.severityChangedCalls {
				monitor = append(monitor, c.audience)
			}
			chat.mu.Unlock()
			if !slices.Equal(monitor, []string{chataudience.IncidentMonitor}) {
				t.Errorf("Severity changed card went to %v; want only Incident Monitor", monitor)
			}
			if got := roomAudiences(chat); !slices.Equal(got, tc.rooms) {
				t.Errorf("room card went to %v; want %v", got, tc.rooms)
			}
		})
	}
}

// failingSRERoomChat fails every room card.
type failingSRERoomChat struct{ *mockGoogleChatSender }

func (f failingSRERoomChat) SendIncidentCreatedAlert(context.Context, string, notifications.IncidentCreatedAlert) error {
	return errors.New("webhook gone")
}

// A failing SRE room must not fail the record: a retry, and in the end the
// dead-letter tier, would resend the Incident Monitor card and the emails.
func TestDispatcher_S0CaseSRERoomFailureDoesNotFailTheRecord(t *testing.T) {
	chat := &mockGoogleChatSender{hasAudienceSpace: mappedSpaces}
	if err := sreRoomDispatcher(failingSRERoomChat{chat}).Handle(context.Background(), caseCreatedWith("CATASTROPHIC", "WSO2 IaaS Team")); err != nil {
		t.Fatalf("Handle error = %v; want nil when only the SRE room card fails", err)
	}
	if got := caseCardAudiences(chat); !slices.Equal(got, []string{chataudience.IncidentMonitor}) {
		t.Errorf("Case Reported card went to %v; want Incident Monitor", got)
	}
}

// A Chat error's response excerpt can echo case content, so only its status
// (or its type) is logged.
func TestChatErrSummary_NeverCarriesTheResponseBody(t *testing.T) {
	err := errors.Join(&apierror.Error{StatusCode: 400, Body: `{"text":"Something broke for Customer 3"}`}, errors.New("dial tcp: timeout"))
	got := chatErrSummary(err)
	if got != "status 400; *errors.errorString" {
		t.Errorf("chatErrSummary = %q; want only the status and the type", got)
	}
	if strings.Contains(got, "Customer 3") {
		t.Errorf("chatErrSummary leaked the response body: %q", got)
	}
}

// The SRE room card has its own gate, not the Incident Monitor branch's: only
// a customer case is paged, so only it is carded in an SRE room.
func TestDispatcher_S0RoomCardOnlyForCustomerCases(t *testing.T) {
	for _, tc := range []struct {
		caseType string
		rooms    int
	}{
		{"CASE", 1},
		{"case", 1},
		{"service_request", 0},
		{"security_report_analysis", 0},
	} {
		t.Run(tc.caseType, func(t *testing.T) {
			chat := &mockGoogleChatSender{hasAudienceSpace: mappedSpaces}
			if err := sreRoomDispatcher(chat).Handle(context.Background(), caseCreatedOfType(tc.caseType, "CATASTROPHIC", "WSO2 IaaS Team")); err != nil {
				t.Fatalf("Handle error = %v", err)
			}
			if got := len(roomCards(chat)); got != tc.rooms {
				t.Errorf("room cards = %d; want %d", got, tc.rooms)
			}
		})
	}
}

// blockingRoomChat holds every SRE room card until the Incident Monitor card
// has gone out, so the record completes only if the CRE-side work does not
// wait for the room card.
type blockingRoomChat struct {
	*mockGoogleChatSender
	monitorSent chan struct{}
	// timedOut is set when a room card gave up waiting for the Incident
	// Monitor card: the CRE-side work was waiting for the room card.
	timedOut *atomic.Bool
}

func (b blockingRoomChat) SendCaseCreatedAlert(ctx context.Context, audience, severityLabel, severityColor, caseNumber, wso2CaseID, productName, title, team, caseLink string) error {
	err := b.mockGoogleChatSender.SendCaseCreatedAlert(ctx, audience, severityLabel, severityColor, caseNumber, wso2CaseID, productName, title, team, caseLink)
	close(b.monitorSent)
	return err
}

func (b blockingRoomChat) SendIncidentCreatedAlert(ctx context.Context, audience string, a notifications.IncidentCreatedAlert) error {
	select {
	case <-b.monitorSent:
	case <-time.After(5 * time.Second):
		b.timedOut.Store(true)
		return errors.New("timed out: the Incident Monitor card was never sent while the SRE room card waited for it")
	}
	return b.mockGoogleChatSender.SendIncidentCreatedAlert(ctx, audience, a)
}

// The CRE-side work does not wait for the SRE room card: the Incident Monitor
// card (and the emails before it) go out while the room card is held, and
// Handle still returns only once the room card is done.
//
// Guards against putting the room card first in the handler's own sequence
// (the reorder considered in #3052): the Incident Monitor card would then wait
// for a room card that is waiting for it, and this test fails. Together with
// TestDispatcher_S0RoomCardStartsBeforeTheCRESideWork it pins the parallel path.
func TestDispatcher_S0RoomCardDoesNotDelayTheCRESideWork(t *testing.T) {
	chat := &mockGoogleChatSender{hasAudienceSpace: mappedSpaces}
	timedOut := &atomic.Bool{}
	d := sreRoomDispatcher(blockingRoomChat{mockGoogleChatSender: chat, monitorSent: make(chan struct{}), timedOut: timedOut})
	err := d.Handle(context.Background(), caseCreatedWith("CATASTROPHIC", "WSO2 IaaS Team"))
	// The room card is best effort, so a CRE side that waits for it does not
	// fail Handle: it shows as the room card timing out.
	if timedOut.Load() {
		t.Fatalf("The CRE-side work (Incident Monitor card, emails) now waits for the SRE room card. " +
			"A slow or hanging SRE webhook would then delay the Incident Monitor card and the case emails. See #3052.")
	}
	if err != nil {
		t.Fatalf("Handle error = %v", err)
	}
	if got := caseCardAudiences(chat); !slices.Equal(got, []string{chataudience.IncidentMonitor}) {
		t.Errorf("Case Reported card went to %v; want Incident Monitor", got)
	}
	if got := roomAudiences(chat); !slices.Equal(got, []string{notifications.IncidentAudience("WSO2 IaaS Team")}) {
		t.Errorf("room card went to %v by the time Handle returned; want the team's room", got)
	}
}

// Every room is posted; one failing room does not stop the others.
func TestDispatcher_S0RoomCardPostsEveryRoom(t *testing.T) {
	chat := &mockGoogleChatSender{hasAudienceSpace: mappedSpaces}
	if err := sreRoomDispatcher(chat).Handle(context.Background(), caseCreatedWith("CATASTROPHIC", "")); err != nil {
		t.Fatalf("Handle error = %v", err)
	}
	got := roomAudiences(chat)
	if want := []string{notifications.IncidentAudience("apollo"), notifications.IncidentAudience("artemis")}; !slices.Equal(got, want) {
		t.Errorf("room cards went to %v; want %v", got, want)
	}
}

// roomSignalChat closes roomSent the moment the SRE room card is sent.
type roomSignalChat struct {
	*mockGoogleChatSender
	roomSent chan struct{}
	once     sync.Once
}

func (r *roomSignalChat) SendIncidentCreatedAlert(ctx context.Context, audience string, a notifications.IncidentCreatedAlert) error {
	err := r.mockGoogleChatSender.SendIncidentCreatedAlert(ctx, audience, a)
	r.once.Do(func() { close(r.roomSent) })
	return err
}

// emailWaitsForRoomCard sends no email until the SRE room card has been sent,
// and fails after 5 s.
type emailWaitsForRoomCard struct {
	*mockEmailSender
	roomSent <-chan struct{}
}

func (e emailWaitsForRoomCard) SendEmail(ctx context.Context, to, cc, bcc, replyTo []string, subject, htmlBody string, attachments []notifications.EmailAttachment) error {
	return e.SendEmailFrom(ctx, "", to, cc, bcc, replyTo, subject, htmlBody, attachments)
}

func (e emailWaitsForRoomCard) SendEmailFrom(ctx context.Context, from string, to, cc, bcc, replyTo []string, subject, htmlBody string, attachments []notifications.EmailAttachment) error {
	select {
	case <-e.roomSent:
	case <-time.After(5 * time.Second):
		return errors.New("timed out: the case emails started before any SRE room card was sent")
	}
	return e.mockEmailSender.SendEmailFrom(ctx, from, to, cc, bcc, replyTo, subject, htmlBody, attachments)
}

// The SRE room card starts before the CRE-side work: the case emails, and so
// the Incident Monitor card after them, only go out once it has been sent.
//
// Guards against the room card going back to the end of the handler (#3038's
// order): the emails would then wait for a card that only comes after them,
// time out, and this test fails. Together with
// TestDispatcher_S0RoomCardDoesNotDelayTheCRESideWork it pins the parallel path.
func TestDispatcher_S0RoomCardStartsBeforeTheCRESideWork(t *testing.T) {
	roomSent := make(chan struct{})
	chat := &roomSignalChat{mockGoogleChatSender: &mockGoogleChatSender{hasAudienceSpace: mappedSpaces}, roomSent: roomSent}
	email := emailWaitsForRoomCard{mockEmailSender: &mockEmailSender{}, roomSent: roomSent}
	if err := sreRoomDispatcherWith(email, chat).Handle(context.Background(), caseCreatedWith("CATASTROPHIC", "WSO2 IaaS Team")); err != nil {
		if len(roomCards(chat.mockGoogleChatSender)) == 0 {
			t.Fatalf("No SRE room card was sent for this S0 case. Check the room selection (CaseSREAudiences) "+
				"and the S0 gate; this is not an ordering problem. (%v)", err)
		}
		t.Fatalf("The SRE room card no longer starts before the CRE-side work (emails, Incident Monitor card). "+
			"Notifications still work, but the SRE engine can post its first rung (\"Notifying L1\") before the case card, "+
			"so the rung opens the thread in the SRE room and the \"Case Reported\" card lands under it as a reply. See #3052. (%v)", err)
	}
	if len(email.mockEmailSender.calls) == 0 {
		t.Error("no case email was sent")
	}
	if got := caseCardAudiences(chat.mockGoogleChatSender); !slices.Equal(got, []string{chataudience.IncidentMonitor}) {
		t.Errorf("Case Reported card went to %v; want Incident Monitor", got)
	}
}

// failFirstRoomChat fails the first room card and accepts the rest.
type failFirstRoomChat struct {
	*mockGoogleChatSender
	mu        sync.Mutex
	attempted []string
}

func (f *failFirstRoomChat) SendIncidentCreatedAlert(ctx context.Context, audience string, a notifications.IncidentCreatedAlert) error {
	f.mu.Lock()
	f.attempted = append(f.attempted, audience)
	first := len(f.attempted) == 1
	f.mu.Unlock()
	if first {
		return errors.New("room unavailable")
	}
	return f.mockGoogleChatSender.SendIncidentCreatedAlert(ctx, audience, a)
}

// One failing SRE room does not stop the others, and does not fail the record.
func TestDispatcher_S0RoomCardContinuesAfterRoomFailure(t *testing.T) {
	chat := &mockGoogleChatSender{hasAudienceSpace: mappedSpaces}
	sender := &failFirstRoomChat{mockGoogleChatSender: chat}
	if err := sreRoomDispatcher(sender).Handle(context.Background(), caseCreatedWith("CATASTROPHIC", "")); err != nil {
		t.Fatalf("Handle error = %v; a failing room must not fail the record", err)
	}
	sender.mu.Lock()
	attempted := slices.Clone(sender.attempted)
	sender.mu.Unlock()
	slices.Sort(attempted)
	want := []string{notifications.IncidentAudience("apollo"), notifications.IncidentAudience("artemis")}
	if !slices.Equal(attempted, want) {
		t.Errorf("rooms attempted = %v; want %v", attempted, want)
	}
	if got := roomAudiences(chat); len(got) != 1 || !slices.Contains(want, got[0]) {
		t.Errorf("room cards delivered to %v; want one of the teamless rooms", got)
	}
}

// hangingRoomChat never answers a room card until its context ends.
type hangingRoomChat struct{ *mockGoogleChatSender }

func (hangingRoomChat) SendIncidentCreatedAlert(ctx context.Context, _ string, _ notifications.IncidentCreatedAlert) error {
	<-ctx.Done()
	return ctx.Err()
}

// A hanging SRE webhook holds the record no longer than the budget, however
// many rooms there are, and does not fail it: the next record on the
// partition, possibly another case's CRE event, is not held for long.
func TestDispatcher_S0RoomCardHangingWebhookIsBounded(t *testing.T) {
	was := caseSRERoomCardBudget
	caseSRERoomCardBudget = 200 * time.Millisecond
	t.Cleanup(func() { caseSRERoomCardBudget = was })

	chat := &mockGoogleChatSender{hasAudienceSpace: mappedSpaces}
	start := time.Now()
	if err := sreRoomDispatcher(hangingRoomChat{chat}).Handle(context.Background(), caseCreatedWith("CATASTROPHIC", "")); err != nil {
		t.Fatalf("Handle error = %v; a hanging room must not fail the record", err)
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Errorf("Handle took %v with a hanging SRE webhook; want about the %v budget", took, caseSRERoomCardBudget)
	}
	if got := caseCardAudiences(chat); !slices.Equal(got, []string{chataudience.IncidentMonitor}) {
		t.Errorf("Case Reported card went to %v; want Incident Monitor", got)
	}
}

// holdingSLA holds the first RegisterClocks call until released: on the code
// before this fix that call sat between the SRE room card claiming its key and
// the record being registered (beginRecord).
type holdingSLA struct {
	entered, release chan struct{}
	once             sync.Once
}

func (h *holdingSLA) RegisterClocks(context.Context, string, string, time.Time, string, string, string, string, string, string) {
	first := false
	h.once.Do(func() { first = true })
	if first {
		close(h.entered)
		<-h.release
	}
}
func (h *holdingSLA) ApplyStateEffects(context.Context, string, string)     {}
func (h *holdingSLA) CompleteResponseClock(context.Context, string)         {}
func (h *holdingSLA) CompleteWorkaroundClock(context.Context, string) error { return nil }

// A redelivery of the same record while the first call is still in flight
// (a rebalance) must not release the SRE room card's key: the record is
// registered before the card claims it, so the concurrent call does not see
// itself as the last one. Before, a third delivery could then claim the key
// again and post the card twice.
func TestDispatcher_S0RoomCardKeyHeldWhileTheFirstCallIsInFlight(t *testing.T) {
	rec := eventbus.Record{Value: []byte(`{"type":"case.created","entityId":"CASE-1","payload":{"reporterName":"Reporter","projectName":"Proj","projectId":"PROJ-1","caseId":"CASE-1","caseNumber":"CS0001001","wso2CaseId":"WSO2-1001","caseTitle":"Something broke","caseType":"CASE","priority":"CATASTROPHIC","product":"api-manager","team":"Phoenix","sreTeamName":"WSO2 IaaS Team","createdAt":"2026-01-01T00:00:00Z","description":"desc","recipients":["test-recipient@example.com"]}}`)}
	chat := &mockGoogleChatSender{hasAudienceSpace: mappedSpaces}
	sla := &holdingSLA{entered: make(chan struct{}), release: make(chan struct{})}
	d := sreRoomDispatcher(chat).WithSLAEngine(sla)

	first := make(chan error, 1)
	go func() { first <- d.Handle(context.Background(), rec) }()
	select {
	case <-sla.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the first call never reached the SLA clocks")
	}
	// The redelivery runs to the end while the first call is held.
	if err := d.Handle(context.Background(), rec); err != nil {
		t.Fatalf("redelivery: %v", err)
	}
	// A third delivery: the key must still be held by the first call.
	if err := d.Handle(context.Background(), rec); err != nil {
		t.Fatalf("third delivery: %v", err)
	}
	close(sla.release)
	if err := <-first; err != nil {
		t.Fatalf("first call: %v", err)
	}
	if got := roomAudiences(chat); len(got) != 1 {
		t.Errorf("SRE room cards posted = %d (%v); want 1: a concurrent call released the room card's key while the first was still sending", len(got), got)
	}
}

// Each time a case becomes S0 its SRE room card starts a thread of its own,
// keyed by the record that made it S0, so a case raised to S0 again does not
// post a second card into the first episode's thread.
func TestDispatcher_S0RoomCardThreadIsPerS0Episode(t *testing.T) {
	chat := &mockGoogleChatSender{hasAudienceSpace: mappedSpaces}
	d := sreRoomDispatcher(chat)
	created := caseCreatedWith("CATASTROPHIC", "WSO2 IaaS Team")
	raised := eventbus.Record{Value: []byte(`{"type":"case.severity_changed","entityId":"CASE-1","payload":{"projectId":"PROJ-1","caseId":"CASE-1","caseNumber":"CS0001001","caseTitle":"Something broke","oldSeverity":"CRITICAL","newSeverity":"CATASTROPHIC","product":"api-manager","team":"Phoenix","sreTeamName":"WSO2 IaaS Team","recipients":["test-recipient@example.com"]}}`)}
	for _, rec := range []eventbus.Record{created, raised} {
		if err := d.Handle(context.Background(), rec); err != nil {
			t.Fatalf("Handle: %v", err)
		}
	}
	cards := roomCards(chat)
	if len(cards) != 2 {
		t.Fatalf("room cards = %d; want one per S0 episode", len(cards))
	}
	first, second := cards[0].alert.ThreadKey, cards[1].alert.ThreadKey
	if first != notifications.CaseS0ThreadKey("CASE-1", created.Value) || second != notifications.CaseS0ThreadKey("CASE-1", raised.Value) {
		t.Errorf("threads = %q, %q; want each episode's CaseS0ThreadKey", first, second)
	}
	if first == second {
		t.Errorf("both S0 episodes posted into thread %q; want a new thread for the raise", first)
	}
}
