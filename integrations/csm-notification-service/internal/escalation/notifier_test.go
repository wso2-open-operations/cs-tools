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

package escalation

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

// fakeChat records the cards a run would have posted.
type fakeChat struct {
	posted []notifications.EscalationAlert
	err    error
}

func (f *fakeChat) SendEscalationAlert(_ context.Context, a notifications.EscalationAlert) error {
	if f.err != nil {
		return f.err
	}
	f.posted = append(f.posted, a)
	return nil
}

type fakeLinks struct{}

func (fakeLinks) IncidentLink(id string) string {
	return "https://csm.example/operations/incidents/" + id
}

func chatEngine(t *testing.T, chat chatSender, store ladderStore, notes incidentNotes) *Engine {
	t.Helper()
	return &Engine{
		policies:  DefaultPolicy,
		resolver:  fullResolver(),
		notifiers: []notifier{chatNotifier{chat: chat, links: fakeLinks{}, defaultProduct: "WSO2 API Manager"}},
		store:     store,
		notes:     notes,
		cfg:       EngineConfig{CallSendingEnabled: true, Channel: ChannelChat},
		clock:     func() time.Time { return testClock },
	}
}

func TestParseChannel(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want Channel
		ok   bool
	}{
		{"", ChannelCall, true}, // unset must not silently become chat
		{"call", ChannelCall, true},
		{"chat", ChannelChat, true},
		{"both", ChannelBoth, true},
		{" BOTH ", ChannelBoth, true},
		{"sms", "", false},
	} {
		got, err := ParseChannel(tc.raw)
		if tc.ok && (err != nil || got != tc.want) {
			t.Errorf("ParseChannel(%q) = (%q, %v), want (%q, nil)", tc.raw, got, err, tc.want)
		}
		if !tc.ok && err == nil {
			t.Errorf("ParseChannel(%q) accepted an unknown channel", tc.raw)
		}
	}
}

func TestChannel_Uses(t *testing.T) {
	if !ChannelBoth.Uses(ChannelCall) || !ChannelBoth.Uses(ChannelChat) {
		t.Error("both must include each channel")
	}
	if ChannelCall.Uses(ChannelChat) || ChannelChat.Uses(ChannelCall) {
		t.Error("a single channel must not include the other")
	}
}

// One card per rung, not one per attempt. A rung's repeats exist because a
// phone was not answered, a question a posted card cannot ask, and a P1
// ladder's fourteen attempts would bury the room in the thing meant to alert
// it.
func TestChatNotifier_PostsOncePerRung(t *testing.T) {
	chat, store := &fakeChat{}, newMemStore()
	e := chatEngine(t, chat, store, &fakeNotes{})

	at := ist(2026, 9, 9, 10, 0)
	if err := e.Handle(context.Background(), createdEvent(t, "CRITICAL", at)); err != nil {
		t.Fatal(err)
	}
	if err := e.Tick(context.Background(), at.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}

	// A business-hours P1 climbs LEVEL_1 to LEVEL_4: four rungs, twelve
	// attempts.
	if len(chat.posted) != 4 {
		t.Fatalf("posted %d cards, want 4 (one per rung, not one per attempt)", len(chat.posted))
	}
	wantRungs := []string{"LEVEL_1", "LEVEL_2", "LEVEL_3", "LEVEL_4"}
	for i, want := range wantRungs {
		if chat.posted[i].Rung != want {
			t.Errorf("card %d is %s, want %s (the rungs must climb in order)", i+1, chat.posted[i].Rung, want)
		}
	}
	// Every attempt still counts as delivered: the rung was notified and the
	// card is still in the room.
	st, _, _ := store.Get(context.Background(), testIncidentID)
	if !st.AllSettled() {
		t.Error("attempts after the first were left unsettled; the ladder would never finish")
	}
}

// The card has to carry enough for a reader to act without opening anything.
func TestChatNotifier_CardCarriesTheContext(t *testing.T) {
	chat := &fakeChat{}
	e := chatEngine(t, chat, newMemStore(), &fakeNotes{})

	at := ist(2026, 9, 9, 10, 0)
	if err := e.Handle(context.Background(), createdEvent(t, "CRITICAL", at)); err != nil {
		t.Fatal(err)
	}
	if err := e.Tick(context.Background(), at.Add(7*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(chat.posted) != 1 {
		t.Fatalf("posted %d cards, want 1", len(chat.posted))
	}

	card := chat.posted[0]
	if card.Rung != "LEVEL_1" || card.RungRole == "" {
		t.Errorf("rung = %q / %q; a reader needs both the level and who it is", card.Rung, card.RungRole)
	}
	if card.IncidentRef != "INC0012345" {
		t.Errorf("incidentRef = %q", card.IncidentRef)
	}
	if card.Priority != "CRITICAL" {
		t.Errorf("priority = %q", card.Priority)
	}
	if card.RecipientName == "" {
		t.Error("the card does not say who is being called")
	}
	if !strings.Contains(card.Instruction, "Work In Progress") {
		t.Errorf("instruction = %q; the card must say what stops the ladder", card.Instruction)
	}
	if card.Rule == "" {
		t.Error("the card does not name the routing path")
	}
	if !strings.Contains(card.PortalURL, testIncidentID) {
		t.Errorf("portalURL = %q; it should open this incident", card.PortalURL)
	}
	if card.Product != "WSO2 API Manager" {
		t.Errorf("product = %q; the card must route to a space", card.Product)
	}
}

// A phone number must never reach a room. The card names a person.
func TestChatNotifier_CardCarriesNoPhoneNumber(t *testing.T) {
	chat := &fakeChat{}
	e := chatEngine(t, chat, newMemStore(), &fakeNotes{})

	at := ist(2026, 9, 9, 10, 0)
	if err := e.Handle(context.Background(), createdEvent(t, "CRITICAL", at)); err != nil {
		t.Fatal(err)
	}
	if err := e.Tick(context.Background(), at.Add(7*time.Minute)); err != nil {
		t.Fatal(err)
	}
	for _, c := range chat.posted {
		for _, field := range []string{c.RecipientName, c.Title, c.Instruction, c.IncidentRef} {
			if strings.Contains(field, "+9477") {
				t.Errorf("a phone number reached the card: %q", field)
			}
		}
	}
}

// A webhook failure is an attempt failure, so the tick surfaces it and retries
// rather than marking the rung notified.
func TestChatNotifier_FailureIsRetried(t *testing.T) {
	chat := &fakeChat{err: errors.New("webhook 503")}
	store := newMemStore()
	e := chatEngine(t, chat, store, &fakeNotes{})

	at := ist(2026, 9, 9, 10, 0)
	if err := e.Handle(context.Background(), createdEvent(t, "CRITICAL", at)); err != nil {
		t.Fatal(err)
	}
	if err := e.Tick(context.Background(), at.Add(7*time.Minute)); err == nil {
		t.Fatal("a webhook failure must surface so the tick retries")
	}
	st, _, _ := store.Get(context.Background(), testIncidentID)
	if st.Placed[0] {
		t.Error("a failed post must not be recorded as delivered")
	}
}

// With both channels, each is attempted even when the other fails: a webhook
// being down must not stop the phone ringing, and a phone failing must not
// cost the room its sight of the escalation.
func TestBothChannels_EachIsAttemptedIndependently(t *testing.T) {
	chat := &fakeChat{err: errors.New("webhook down")}
	caller := &fakeCaller{}
	store := newMemStore()
	e := &Engine{
		policies: DefaultPolicy,
		resolver: fullResolver(),
		notifiers: []notifier{
			voiceNotifier{calls: caller},
			chatNotifier{chat: chat, links: fakeLinks{}, defaultProduct: "WSO2 API Manager"},
		},
		store: store,
		notes: &fakeNotes{},
		cfg:   EngineConfig{CallSendingEnabled: true, Channel: ChannelBoth},
		clock: func() time.Time { return testClock },
	}

	at := ist(2026, 9, 9, 10, 0)
	if err := e.Handle(context.Background(), createdEvent(t, "CRITICAL", at)); err != nil {
		t.Fatal(err)
	}
	if err := e.Tick(context.Background(), at.Add(7*time.Minute)); err == nil {
		t.Fatal("the chat failure should surface")
	}
	if len(caller.placed) != 1 {
		t.Errorf("the call was not placed despite the chat failure: %d calls", len(caller.placed))
	}
}

// The killswitch has to stop every channel, not only the voice one.
func TestChatNotifier_KillswitchPostsNothing(t *testing.T) {
	chat := &fakeChat{}
	e := chatEngine(t, chat, newMemStore(), &fakeNotes{})
	e.cfg.CallSendingEnabled = false

	at := ist(2026, 9, 9, 10, 0)
	if err := e.Handle(context.Background(), createdEvent(t, "CRITICAL", at)); err != nil {
		t.Fatal(err)
	}
	if err := e.Tick(context.Background(), at.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if len(chat.posted) != 0 {
		t.Errorf("%d cards posted with sending disabled", len(chat.posted))
	}
}

// Configured for a channel whose client was never built: nobody is contacted,
// and that must be loud rather than a silent no-op that looks like coverage.
func TestEngine_NoNotifierConfigured(t *testing.T) {
	store := newMemStore()
	e := &Engine{
		policies: DefaultPolicy,
		resolver: fullResolver(),
		store:    store,
		notes:    &fakeNotes{},
		cfg:      EngineConfig{CallSendingEnabled: true, Channel: ChannelChat},
		clock:    func() time.Time { return testClock },
	}
	at := ist(2026, 9, 9, 10, 0)
	if err := e.Handle(context.Background(), createdEvent(t, "CRITICAL", at)); err != nil {
		t.Fatal(err)
	}
	// It must not error the tick forever, since no retry can fix a missing
	// client, and the ladder must still complete and write its summary.
	if err := e.Tick(context.Background(), at.Add(2*time.Hour)); err != nil {
		t.Fatalf("a missing client must not be retried: %v", err)
	}
	if _, found, _ := store.Get(context.Background(), testIncidentID); found {
		t.Error("the ladder never finished")
	}
}

func TestElapsedSince(t *testing.T) {
	base := ist(2026, 9, 9, 10, 0)
	for _, tc := range []struct {
		at   time.Time
		want string
	}{
		{base, ""},
		{base.Add(18 * time.Minute), "18m"},
		{base.Add(90 * time.Minute), "1h30m"},
		{base.Add(2 * time.Hour), "2h00m"},
	} {
		if got := elapsedSince(base, tc.at); got != tc.want {
			t.Errorf("elapsedSince(+%v) = %q, want %q", tc.at.Sub(base), got, tc.want)
		}
	}
}

// A nil link resolver must leave the card without a portal URL, not panic
// mid-page. Guards the typed-nil-in-interface trap: assigning a nil
// *recipientlinks.Resolver straight into the interface field stores a non-nil
// interface holding a nil pointer, so Deliver's own nil check passes and the
// method is called on a nil receiver. A real run found this.
func TestChatNotifier_NilLinkResolverDoesNotPanic(t *testing.T) {
	chat := &fakeChat{}
	e := NewEngine(DefaultPolicy, fullResolver(), nil, &notifications.GoogleChatClient{}, nil,
		nil, nil, "WSO2 API Manager",
		EngineConfig{CallSendingEnabled: true, Channel: ChannelChat})

	// The constructor is what has to be safe; swap in the fake to deliver.
	e.notifiers = []notifier{chatNotifier{chat: chat, defaultProduct: "WSO2 API Manager"}}
	e.store = newMemStore()
	e.notes = &fakeNotes{}
	e.clock = func() time.Time { return testClock }

	at := ist(2026, 9, 9, 10, 0)
	if err := e.Handle(context.Background(), createdEvent(t, "CRITICAL", at)); err != nil {
		t.Fatal(err)
	}
	if err := e.Tick(context.Background(), at.Add(7*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(chat.posted) != 1 {
		t.Fatalf("posted %d cards, want 1", len(chat.posted))
	}
	if chat.posted[0].PortalURL != "" {
		t.Errorf("portalURL = %q, want empty with no resolver", chat.posted[0].PortalURL)
	}
}

// And the constructor itself must not store a nil pointer behind the
// interface, which is what made the panic reachable.
func TestNewEngine_NilLinksLeavesTheFieldNil(t *testing.T) {
	e := NewEngine(DefaultPolicy, fullResolver(), nil, &notifications.GoogleChatClient{}, nil,
		nil, nil, "", EngineConfig{CallSendingEnabled: true, Channel: ChannelChat})
	if len(e.notifiers) != 1 {
		t.Fatalf("built %d notifiers, want 1", len(e.notifiers))
	}
	n, ok := e.notifiers[0].(chatNotifier)
	if !ok {
		t.Fatalf("notifier is %T, want chatNotifier", e.notifiers[0])
	}
	if n.links != nil {
		t.Error("a nil resolver was stored as a non-nil interface; Deliver would call a nil receiver")
	}
}

// The ladder climbs over time and stops the moment somebody picks the
// incident up, on chat exactly as on calls. The pacing and the cancellation
// both live in the engine, above the channel, so neither changes with it -
// this pins that, because a channel that posted its whole ladder at once
// would be a notification dump rather than an escalation.
func TestChatChannel_ClimbsOverTimeAndStopsOnAcknowledgement(t *testing.T) {
	chat, store := &fakeChat{}, newMemStore()
	e := chatEngine(t, chat, store, &fakeNotes{})

	at := ist(2026, 9, 9, 10, 0)
	if err := e.Handle(context.Background(), createdEvent(t, "CRITICAL", at)); err != nil {
		t.Fatal(err)
	}

	// Nothing is due before the priority's own initial wait, however often
	// the engine ticks.
	for _, early := range []time.Duration{0, time.Minute, 5 * time.Minute} {
		if err := e.Tick(context.Background(), at.Add(early)); err != nil {
			t.Fatal(err)
		}
	}
	if len(chat.posted) != 0 {
		t.Fatalf("%d cards posted inside P1's six-minute initial wait", len(chat.posted))
	}

	// LEVEL_1 opens at six minutes, and only LEVEL_1.
	if err := e.Tick(context.Background(), at.Add(6*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(chat.posted) != 1 || chat.posted[0].Rung != "LEVEL_1" {
		t.Fatalf("after six minutes: %d cards, first %v; want one LEVEL_1 card", len(chat.posted), rungsOf(chat.posted))
	}

	// LEVEL_2 does not open until fifteen: the initial wait plus LEVEL_1's
	// own duration, which is (3 calls x 2m) + 3m. This incident was reported
	// during business hours, so it has no LEVEL_0 and nothing else is in
	// front of it.
	if err := e.Tick(context.Background(), at.Add(14*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(chat.posted) != 1 {
		t.Errorf("a later rung posted early: %v", rungsOf(chat.posted))
	}
	if err := e.Tick(context.Background(), at.Add(15*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(chat.posted) != 2 || chat.posted[1].Rung != "LEVEL_2" {
		t.Fatalf("after fifteen minutes: %v; want LEVEL_1 then LEVEL_2", rungsOf(chat.posted))
	}

	// Somebody picks it up. Every remaining rung is cancelled.
	ack := record(t, events.TypeIncidentCommentAdded, events.IncidentCommentAddedPayload{
		CommentID: "c-1", IsPublic: true,
	})
	if err := e.Handle(context.Background(), ack); err != nil {
		t.Fatal(err)
	}
	if err := e.Tick(context.Background(), at.Add(3*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if len(chat.posted) != 2 {
		t.Errorf("cards kept arriving after the acknowledgement: %v", rungsOf(chat.posted))
	}
}

func rungsOf(alerts []notifications.EscalationAlert) []string {
	out := make([]string, 0, len(alerts))
	for _, a := range alerts {
		out = append(out, a.Rung)
	}
	return out
}

// A card has to carry the clock, because a space does not show one. These are
// the two fields that turn a list of notices into a visible escalation: how
// long this has gone unattended, and how long until it climbs again.
func TestChatNotifier_CardCarriesTheClock(t *testing.T) {
	chat := &fakeChat{}
	e := chatEngine(t, chat, newMemStore(), &fakeNotes{})

	at := ist(2026, 9, 9, 10, 0)
	if err := e.Handle(context.Background(), createdEvent(t, "CRITICAL", at)); err != nil {
		t.Fatal(err)
	}
	if err := e.Tick(context.Background(), at.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if len(chat.posted) != 4 {
		t.Fatalf("posted %d cards, want 4", len(chat.posted))
	}

	// LEVEL_1 opens six minutes in and LEVEL_2 nine minutes after that.
	first := chat.posted[0]
	if first.Elapsed != "6m" {
		t.Errorf("first card says unattended %q, want 6m", first.Elapsed)
	}
	if first.NextRung != "LEVEL_2" || first.NextIn != "9m" {
		t.Errorf("first card points at %q in %q, want LEVEL_2 in 9m", first.NextRung, first.NextIn)
	}

	// Every rung of one incident belongs to one conversation.
	for _, c := range chat.posted {
		if c.ThreadKey != "incident-escalation-"+testIncidentID {
			t.Errorf("card %s threaded under %q", c.Rung, c.ThreadKey)
		}
	}

	// The last rung has nowhere to climb and must not claim otherwise.
	last := chat.posted[len(chat.posted)-1]
	if last.Rung != "LEVEL_4" {
		t.Fatalf("last card is %s, want LEVEL_4", last.Rung)
	}
	if last.NextRung != "" || last.NextIn != "" {
		t.Errorf("the final rung claims it escalates to %q in %q", last.NextRung, last.NextIn)
	}
}
