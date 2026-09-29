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
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

// The whole chain, minus Twilio itself.
//
// Every other engine test substitutes a fake for the call client, which is
// right for asserting on the ladder's behaviour but leaves a real gap: nothing
// proved that the engine, driving the PRODUCTION Twilio client, puts a correct
// request on the wire. The document, the destination, the caller ID, the ring
// timeout and the auth header are all assembled below the engine, by code no
// engine test ever reaches.
//
// So these wire the real notifications.TwilioClient — pointed at a local
// server through its own APIBaseURL override — into a real Engine, and assert
// on what Twilio would have received. What they cannot prove is the only thing
// left: that Twilio accepts it and a handset rings.

// twilioSpy is a stand-in for Twilio's REST API that records what it was sent
// and replies however a test needs.
type twilioSpy struct {
	mu       sync.Mutex
	requests []url.Values
	auth     []string
	paths    []string
	// status and body drive the reply; zero status means 201 Created.
	status int
	body   string
}

func (s *twilioSpy) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		s.mu.Lock()
		s.requests = append(s.requests, r.PostForm)
		s.auth = append(s.auth, r.Header.Get("Authorization"))
		s.paths = append(s.paths, r.URL.Path)
		status, body := s.status, s.body
		s.mu.Unlock()

		if status == 0 {
			status, body = http.StatusCreated, `{"sid":"CA0000000000000000000000000000001","status":"queued"}`
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func (s *twilioSpy) snapshot() ([]url.Values, []string, []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]url.Values(nil), s.requests...),
		append([]string(nil), s.auth...),
		append([]string(nil), s.paths...)
}

// wiredEngine builds a real Engine over the real Twilio client, pointed at the
// spy. The store stays in-memory: this is about the call path, and a Redis
// dependency would make it skip on a laptop without one.
func wiredEngine(t *testing.T, spy *twilioSpy, cfg EngineConfig, ringSeconds int) (*Engine, *memStore, func()) {
	t.Helper()
	srv := httptest.NewServer(spy.handler())
	client := notifications.NewTwilioClient(notifications.TwilioConfig{
		AccountSID:         "ACtest",
		AuthToken:          "sekret",
		FromNumber:         "+15550000001",
		Voice:              "Polly.Aditi",
		Language:           "en-IN",
		APIBaseURL:         srv.URL,
		RingTimeoutSeconds: ringSeconds,
	})
	store := newMemStore()
	if cfg.Channel == "" {
		cfg.Channel = ChannelCall
	}
	e := &Engine{
		policies:  DefaultPolicy,
		resolver:  perRungResolver(),
		notifiers: []notifier{voiceNotifier{calls: client, useSSML: cfg.UseSSML}},
		store:     store,
		cfg:       cfg,
		clock:     func() time.Time { return testClock },
	}
	return e, store, srv.Close
}

// perRungResolver gives every rung its own destination, so the sequence of
// numbers the spy receives is itself the proof the ladder climbed in order.
func perRungResolver() StaticResolver {
	return StaticResolver{ByLevel: map[Level][]Recipient{
		Level0: {rec("rotation.lead@wso2.com", "+94770000000")},
		Level1: {rec("abt.lead@wso2.com", "+94770000001")},
		Level2: {rec("abt.team.lead@wso2.com", "+94770000002")},
		Level3: {rec("bu.head@wso2.com", "+94770000003")},
		Level4: {rec("cre.head@wso2.com", "+94770000004")},
	}}
}

// A P1 ladder, dialled for real through the production client, climbs the
// rungs in order — LEVEL_0 once, then three attempts at each of LEVEL_1
// through LEVEL_4 — and every request carries the right caller ID.
func TestWire_LadderClimbsInOrderOnTheWire(t *testing.T) {
	spy := &twilioSpy{}
	e, _, closeSrv := wiredEngine(t, spy, enabled(), 0)
	defer closeSrv()

	// A morning-rotation incident, so LEVEL_0 exists. The engine's clock has
	// to sit at the report time: a ladder whose calls are all in the past is
	// deliberately refused (see the stale-trigger guard), and 07:00 is two
	// hours before the shared test clock.
	at := ist(2026, 9, 9, 7, 0)
	e.clock = func() time.Time { return at }
	if err := e.Handle(context.Background(), createdEvent(t, "CRITICAL", at)); err != nil {
		t.Fatal(err)
	}
	if err := e.Tick(context.Background(), at.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}

	reqs, auth, paths := spy.snapshot()
	want := []string{
		"+94770000000", "+94770000000", // LEVEL_0, two attempts
		"+94770000001", "+94770000001", "+94770000001", // LEVEL_1
		"+94770000002", "+94770000002", "+94770000002", // LEVEL_2
		"+94770000003", "+94770000003", "+94770000003", // LEVEL_3
		"+94770000004", "+94770000004", "+94770000004", // LEVEL_4
	}
	if len(reqs) != len(want) {
		t.Fatalf("Twilio received %d calls, want %d", len(reqs), len(want))
	}
	for i, wantTo := range want {
		if got := reqs[i].Get("To"); got != wantTo {
			t.Errorf("call %d went to %s, want %s (the ladder climbed out of order)", i+1, got, wantTo)
		}
		if got := reqs[i].Get("From"); got != "+15550000001" {
			t.Errorf("call %d came from %q, want the configured caller ID", i+1, got)
		}
		if !strings.HasSuffix(paths[i], "/Accounts/ACtest/Calls.json") {
			t.Errorf("call %d posted to %s, want the Calls resource", i+1, paths[i])
		}
	}

	// Basic auth, since a wrong header is a 401 nobody sees until production.
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("ACtest:sekret"))
	if auth[0] != wantAuth {
		t.Errorf("Authorization header = %q, want basic auth for the configured account", auth[0])
	}
}

// The plain document reaches Twilio as a spoken <Say>, with the configured
// voice and language, and the case reference spelled out.
func TestWire_PlainDocumentOnTheWire(t *testing.T) {
	spy := &twilioSpy{}
	e, _, closeSrv := wiredEngine(t, spy, enabled(), 0)
	defer closeSrv()

	at := ist(2026, 9, 9, 10, 0)
	if err := e.Handle(context.Background(), createdEvent(t, "CRITICAL", at)); err != nil {
		t.Fatal(err)
	}
	if err := e.Tick(context.Background(), at.Add(7*time.Minute)); err != nil {
		t.Fatal(err)
	}

	reqs, _, _ := spy.snapshot()
	if len(reqs) == 0 {
		t.Fatal("no call reached Twilio")
	}
	twiml := reqs[0].Get("Twiml")
	for _, want := range []string{
		`<Say voice="Polly.Aditi" language="en-IN">`,
		"WSO2 Support Alert.",
		"Priority, CRITICAL.",
		"I N C 0 0 1 2 3 4 5", // the reference, spelled for the listener
		"Update the ticket status to Work In Progress",
	} {
		if !strings.Contains(twiml, want) {
			t.Errorf("the document Twilio received is missing %q:\n%s", want, twiml)
		}
	}
}

// With SSML enabled the same path carries real nested markup rather than
// escaped tags — the bug that made the SSML message undeliverable.
func TestWire_SSMLDocumentOnTheWire(t *testing.T) {
	spy := &twilioSpy{}
	e, _, closeSrv := wiredEngine(t, spy, EngineConfig{CallSendingEnabled: true, UseSSML: true}, 0)
	defer closeSrv()

	at := ist(2026, 9, 9, 10, 0)
	if err := e.Handle(context.Background(), createdEvent(t, "CRITICAL", at)); err != nil {
		t.Fatal(err)
	}
	if err := e.Tick(context.Background(), at.Add(7*time.Minute)); err != nil {
		t.Fatal(err)
	}

	reqs, _, _ := spy.snapshot()
	twiml := reqs[0].Get("Twiml")
	for _, want := range []string{
		`<break time="500ms">`,
		`<prosody rate="90%">INC0012345</prosody>`,
		`<emphasis level="moderate">`,
		"<s>WSO2 Support Alert.</s>",
	} {
		if !strings.Contains(twiml, want) {
			t.Errorf("the SSML Twilio received is missing %s:\n%s", want, twiml)
		}
	}
	if strings.Contains(twiml, "&lt;s&gt;") {
		t.Errorf("the markup arrived escaped, which is what makes Twilio read it aloud:\n%s", twiml)
	}
	if strings.Contains(twiml, "<speak>") {
		t.Errorf("TwiML must not carry a <speak> root:\n%s", twiml)
	}
}

// The ring timeout reaches Twilio, so a call nobody answers stops ringing
// when the operator said it should.
func TestWire_RingTimeoutReachesTwilio(t *testing.T) {
	spy := &twilioSpy{}
	e, _, closeSrv := wiredEngine(t, spy, enabled(), 5)
	defer closeSrv()

	at := ist(2026, 9, 9, 10, 0)
	if err := e.Handle(context.Background(), createdEvent(t, "CRITICAL", at)); err != nil {
		t.Fatal(err)
	}
	if err := e.Tick(context.Background(), at.Add(7*time.Minute)); err != nil {
		t.Fatal(err)
	}

	reqs, _, _ := spy.snapshot()
	if got := reqs[0].Get("Timeout"); got != "5" {
		t.Errorf("Timeout = %q, want 5 seconds", got)
	}
}

// Without one configured, nothing is sent and Twilio applies its own default
// rather than receiving a zero that would ring for no time at all.
func TestWire_NoRingTimeoutSendsNoTimeout(t *testing.T) {
	spy := &twilioSpy{}
	e, _, closeSrv := wiredEngine(t, spy, enabled(), 0)
	defer closeSrv()

	at := ist(2026, 9, 9, 10, 0)
	if err := e.Handle(context.Background(), createdEvent(t, "CRITICAL", at)); err != nil {
		t.Fatal(err)
	}
	if err := e.Tick(context.Background(), at.Add(7*time.Minute)); err != nil {
		t.Fatal(err)
	}

	reqs, _, _ := spy.snapshot()
	if _, present := reqs[0]["Timeout"]; present {
		t.Errorf("Timeout was sent as %q; it should be absent so Twilio uses its own default", reqs[0].Get("Timeout"))
	}
}

// A real 4xx from the API — the shape a live run actually hit — is recorded
// as permanent with the provider's own code, and the ladder still completes.
func TestWire_RealRejectionIsPermanentAndCarriesTheCode(t *testing.T) {
	spy := &twilioSpy{
		status: http.StatusBadRequest,
		// The real body, long enough that the client's error budget truncates
		// it — which is exactly what once lost the code.
		body: `{"code": 21219, "message": "The number +94716531267 is unverified. Trial accounts may only make calls to verified numbers.", "more_info": "https://www.twilio.com/docs/errors/21219", "status": 400}`,
	}
	e, store, closeSrv := wiredEngine(t, spy, enabled(), 0)
	defer closeSrv()

	at := ist(2026, 9, 9, 10, 0)
	if err := e.Handle(context.Background(), createdEvent(t, "CRITICAL", at)); err != nil {
		t.Fatal(err)
	}
	if err := e.Tick(context.Background(), at.Add(7*time.Minute)); err != nil {
		t.Fatalf("a permanent rejection must not surface as a tick error: %v", err)
	}

	st, _, _ := store.Get(context.Background(), testIncidentID)
	if got := st.failure(0); got != "REJECTED_400_21219" {
		t.Errorf("failure = %q, want REJECTED_400_21219 — the provider's code must survive truncation", got)
	}
	if strings.Contains(strings.Join(st.Failed, " "), "+94716531267") {
		t.Error("the recorded failure quotes the phone number back")
	}
}

// A 5xx is the other half: transient, left scheduled, retried.
func TestWire_RealServerErrorIsRetried(t *testing.T) {
	spy := &twilioSpy{status: http.StatusServiceUnavailable, body: `{"message":"unavailable"}`}
	e, store, closeSrv := wiredEngine(t, spy, enabled(), 0)
	defer closeSrv()

	at := ist(2026, 9, 9, 10, 0)
	if err := e.Handle(context.Background(), createdEvent(t, "CRITICAL", at)); err != nil {
		t.Fatal(err)
	}
	if err := e.Tick(context.Background(), at.Add(7*time.Minute)); err == nil {
		t.Fatal("a 5xx must surface so the tick is retried")
	}
	st, _, _ := store.Get(context.Background(), testIncidentID)
	if st.failure(0) != "" {
		t.Error("a 5xx must not be recorded as a permanent failure")
	}
	if _, scheduled := store.wakes[wakeMember(testIncidentID, 0)]; !scheduled {
		t.Error("a transiently failed call must stay scheduled")
	}
}

// The killswitch has to stop the request reaching the wire at all, not merely
// stop the engine recording it.
func TestWire_KillswitchSendsNothing(t *testing.T) {
	spy := &twilioSpy{}
	e, _, closeSrv := wiredEngine(t, spy, EngineConfig{CallSendingEnabled: false}, 0)
	defer closeSrv()

	at := ist(2026, 9, 9, 10, 0)
	if err := e.Handle(context.Background(), createdEvent(t, "CRITICAL", at)); err != nil {
		t.Fatal(err)
	}
	if err := e.Tick(context.Background(), at.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if reqs, _, _ := spy.snapshot(); len(reqs) != 0 {
		t.Errorf("%d calls reached Twilio with sending disabled", len(reqs))
	}
}

// An acknowledgement partway must stop the wire traffic, not just the
// bookkeeping.
func TestWire_AcknowledgementStopsTheCalls(t *testing.T) {
	spy := &twilioSpy{}
	e, _, closeSrv := wiredEngine(t, spy, enabled(), 0)
	defer closeSrv()

	at := ist(2026, 9, 9, 10, 0)
	if err := e.Handle(context.Background(), createdEvent(t, "CRITICAL", at)); err != nil {
		t.Fatal(err)
	}
	if err := e.Tick(context.Background(), at.Add(9*time.Minute)); err != nil {
		t.Fatal(err)
	}
	before, _, _ := spy.snapshot()
	if len(before) == 0 {
		t.Fatal("expected the ladder to have started calling")
	}

	ack := record(t, events.TypeIncidentAcknowledged, events.IncidentAcknowledgedPayload{
		PreviousState: "NEW", NewState: "IN_PROGRESS",
	})
	if err := e.Handle(context.Background(), ack); err != nil {
		t.Fatal(err)
	}
	if err := e.Tick(context.Background(), at.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	after, _, _ := spy.snapshot()
	if len(after) != len(before) {
		t.Errorf("%d more calls reached Twilio after the acknowledgement", len(after)-len(before))
	}
}
