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

// Command escalation-local runs the REAL incident call-escalation engine on a
// laptop, against a real Redis and a real Twilio client, fed the same event
// envelopes Event Hub would deliver.
//
// It exists because the whole service is awkward to run locally: internal/
// eventbus dials its broker with TLS and SASL/PLAIN unconditionally — correct
// for Azure Event Hub, impractical for a laptop — so starting cmd/server means
// real Event Hub credentials. Everything after that hop is what this
// exercises, with the production types rather than copies of them: decoding
// the envelope, deriving the effective shift, picking the section 7.0 policy
// row, resolving recipients, persisting the ladder in Redis, waking on
// schedule, dialling, cancelling, and rendering the execution summary. It
// drives escalation.Engine.Handle and escalation.Engine.Tick directly.
//
// Dry runs are not stubbed out at the client boundary. The real
// notifications.TwilioClient is pointed at a local HTTP server via its own
// APIBaseURL override, so the TwiML — including the SSML document, which is
// the part most worth eyeballing — is built by production code and printed
// exactly as Twilio would receive it. --live points the same client at Twilio.
//
// HOW THIS DIFFERS FROM cmd/ladder-harness: the harness builds a plan and
// places the calls itself, with no engine, no durable state and no events. It
// answers "what does the ladder sound like". This answers "does the engine
// actually do it", including the parts only the engine has — idempotency under
// redelivery, resumption from Redis, and cancellation driven by a real
// incoming event.
//
// Safety: it never dials without --live, --live refuses to run without an
// explicit --to (so it cannot page whoever a real roster points at), and it
// refuses to start at all if the plan is larger than --max-calls.
//
// A ladder outlives the process that started it — that is what the durable
// state is for — so an interrupted run can leave one in Redis that the next
// run's engine resumes and keeps dialling. This retires its own ladder on the
// way out, and that works when the signal actually reaches it: `go run` does
// not forward one to the child process it spawns, so an interrupted `go run`
// CAN orphan a ladder even though the compiled binary handles the same signal
// correctly. --cleanup retires anything left behind, and is worth running
// before any --live session.
//
// Usage:
//
//	# dry run of the whole flow, one ladder minute per second
//	go run ./cmd/escalation-local -priority CRITICAL
//
//	# hear it, on one number, acknowledged part-way by a public comment
//	go run ./cmd/escalation-local -priority P0 -live -to +9477xxxxxxx -cancel-after 8s
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/escalation"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

// placeholderNumber stands in for a real destination in a dry run. Not a
// routable number.
const placeholderNumber = "+10000000000"

type config struct {
	priority    string
	team        string
	at          string
	weekend     bool
	shift       string
	kind        string
	to          string
	live        bool
	ssml        bool
	notABT      bool
	minute      time.Duration
	tick        time.Duration
	cancelAfter time.Duration
	cancelBy    string
	cancelAt    string
	maxCalls    int
	redisAddr   string
	incidentID  string
	showTwiML   bool
	ringSeconds int
	speak       bool
	channel     string
	chatProduct string
	sayVoice    string
	keep        bool
	cleanup     bool
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "\nerror: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	loadDotEnv(".env")

	cfg := parseFlags()
	if cfg.live && strings.TrimSpace(cfg.to) == "" {
		return errors.New("--live requires an explicit --to, so this cannot page anyone by accident")
	}
	if cfg.incidentID == "" {
		cfg.incidentID = fmt.Sprintf("local-%d", time.Now().Unix())
	}
	to := cfg.to
	if to == "" {
		to = placeholderNumber
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// go-redis logs its own dial retries at package level, which buries the
	// one message that actually helps on a laptop ("Redis is not running").
	// Silence it and fail fast instead of retrying a connection that is not
	// going to appear.
	redis.SetLogger(quietLogger{})
	rdb, redisWhere, err := openRedis(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = rdb.Close() }()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("cannot reach Redis at %s.\n"+
			"       The ladder keeps its state there, so this tool needs one running:\n"+
			"           docker run --rm -p 6379:6379 redis\n"+
			"       underlying error: %w", redisWhere, err)
	}

	if cfg.cleanup {
		return cleanupLocalLadders(ctx, escalation.NewStore(rdb), rdb)
	}

	speaker := newSpeaker()
	twilio, recorder, closeTwilio, err := buildTwilioClient(cfg, speaker)
	if err != nil {
		return err
	}
	defer closeTwilio()
	defer speaker.wait()

	channel, err := escalation.ParseChannel(cfg.channel)
	if err != nil {
		return err
	}
	engine := escalation.NewEngine(
		escalation.DefaultPolicy,
		localResolver(to),
		twilio,
		localChatClient(cfg),
		// Empty base: a local card carries no portal URL, which PortalLinks
		// renders as no link rather than a broken one.
		escalation.PortalLinks{},
		escalation.NewStore(rdb),
		nil, // no entity-service locally; the summary is printed here instead
		cfg.chatProduct,
		escalation.EngineConfig{CallSendingEnabled: true, UseSSML: cfg.ssml, Channel: channel},
	)

	// The trigger instant decides the effective shift (the engine derives it
	// from the event's own timestamp), so it is placed inside the shift under
	// test rather than at "now". Every call is an offset from it, and Tick is
	// handed a "now" that advances at cfg.minute per ladder minute — which is
	// what compresses a 113-minute P4 ladder into something observable. No
	// engine code is aware this is a test.
	trigger := triggerTime(escalation.Shift(cfg.shift))
	if cfg.at != "" {
		t, err := reportTime(cfg.at, cfg.weekend, time.Now().In(escalation.IST))
		if err != nil {
			return err
		}
		trigger = t
	}

	if err := engine.Handle(ctx, startRecord(cfg, trigger)); err != nil {
		return fmt.Errorf("starting the ladder: %w", err)
	}

	st, found, err := escalation.NewStore(rdb).Get(ctx, cfg.incidentID)
	if err != nil {
		return fmt.Errorf("reading the stored ladder: %w", err)
	}
	if !found {
		return fmt.Errorf("the engine scheduled no ladder for priority %q — it has no section 7.0 policy row, "+
			"or no level resolved to a reachable recipient", cfg.priority)
	}
	// A hard cap, checked before a single call goes out, rather than a running
	// total that can only notice an overshoot after the fact.
	if len(st.Plan.Calls) > cfg.maxCalls {
		// Retire it properly rather than just dropping the state key:
		// Handle has already seeded a wake entry per planned call, and those
		// live in a sorted set shared with every other ladder. Deleting the
		// state alone would strand them there, rescanned on every tick, with
		// nothing able to reclaim them afterwards.
		if err := retireLadder(ctx, escalation.NewStore(rdb), cfg.incidentID); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not retire the over-cap ladder %s: %v\n", cfg.incidentID, err)
		}
		return fmt.Errorf("this plan is %d calls, more than --max-calls=%d; raise the cap or pick a shorter priority",
			len(st.Plan.Calls), cfg.maxCalls)
	}

	// A ladder outlives the process that started it — that is the whole point
	// of the durable state — so an interrupted run leaves one in Redis that
	// the NEXT run's engine will happily resume and keep dialling. Harmless in
	// a dry run, genuinely dangerous with --live. Always retire it on the way
	// out unless resumption is what is being tested.
	if !cfg.keep {
		defer func() {
			if err := retireLadder(context.Background(), escalation.NewStore(rdb), cfg.incidentID); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not retire ladder %s: %v\n", cfg.incidentID, err)
			}
		}()
	}

	cfg.to = to
	runStart := time.Now()
	printHeader(cfg, st.Plan, trigger, to)
	return runTicks(ctx, cfg, engine, rdb, trigger, recorder, st.Plan, runStart)
}

func parseFlags() config {
	var cfg config
	flag.StringVar(&cfg.priority, "priority", "CRITICAL", "incident priority: P0-P4, or CRITICAL/HIGH/MODERATE/LOW")
	flag.StringVar(&cfg.team, "team", defaultTeam,
		"the incident's assignment group; an ABT key (vega, castor, ...) routes an ABT rule, anything else the not-an-ABT row")
	flag.StringVar(&cfg.at, "at", "",
		"when the incident was reported, in IST: HH:MM for the next weekday at that time, or YYYY-MM-DDTHH:MM; "+
			"the engine derives the shift from it exactly as it would for a real incident. Overrides -shift")
	flag.BoolVar(&cfg.weekend, "weekend", false,
		"with -at HH:MM, the next Saturday or Sunday at that time rather than the next weekday")
	flag.StringVar(&cfg.shift, "shift", "LK_MORNING", "shift when reported: LK, LK_MORNING, LK_EVENING, LK_WEEKEND, USA, USA_WEEKEND")
	flag.StringVar(&cfg.kind, "kind", "new", "what starts the ladder: new or elevated")
	flag.StringVar(&cfg.to, "to", "", "the one number every level resolves to; required with -live")
	flag.BoolVar(&cfg.live, "live", false, "actually place calls through Twilio; without this they go to a local stub")
	flag.BoolVar(&cfg.ssml, "ssml", false, "speak the SSML message (pauses, slowed case reference) instead of the plain one")
	flag.BoolVar(&cfg.notABT, "not-abt", false, "treat the account as NOT ABT-eligible, which is what gives a USA_WEEKEND incident a LEVEL_0")
	flag.DurationVar(&cfg.minute, "minute", time.Second, "how long one ladder minute lasts; 1s compresses a 44m P1 ladder into 44s")
	flag.DurationVar(&cfg.tick, "tick", 200*time.Millisecond, "how often the engine scans for due calls")
	flag.DurationVar(&cfg.cancelAfter, "cancel-after", 0, "acknowledge this far into the run; 0 runs the whole ladder")
	flag.StringVar(&cfg.cancelBy, "cancel-by", "both",
		"how to acknowledge: both (a move out of NEW AND a public comment -- the only thing that actually stops a CRE ladder), comment, or status")
	flag.StringVar(&cfg.cancelAt, "cancel-at", "",
		"acknowledge once the ladder has called this rung, e.g. LEVEL_2; easier than timing -cancel-after by hand, and the two are mutually exclusive")
	flag.IntVar(&cfg.maxCalls, "max-calls", 20, "refuse to run a plan larger than this")
	flag.StringVar(&cfg.redisAddr, "redis", envOr("REDIS_ADDR", "localhost:6379"), "Redis address holding the ladder state")
	flag.StringVar(&cfg.incidentID, "incident-id", "", "incident id to use; defaults to a fresh one per run")
	flag.StringVar(&cfg.channel, "channel", "call", "how a rung reaches people: call, chat, or both")
	flag.StringVar(&cfg.chatProduct, "chat-product", "", "which GOOGLE_CHAT_SPACES product routes the card; empty uses the default space")
	flag.BoolVar(&cfg.speak, "speak", false, "speak each call's message aloud through the local synthesiser instead of only printing it; needs no Twilio account")
	flag.StringVar(&cfg.sayVoice, "say-voice", "Aman", "which local voice to speak with (macOS: `say -v '?'` lists them)")
	flag.IntVar(&cfg.ringSeconds, "ring-seconds", 5, "how long each live call may ring before Twilio gives up; 0 uses Twilio's 60s default")
	flag.BoolVar(&cfg.showTwiML, "show-twiml", false, "print the TwiML document of each call (dry runs only)")
	flag.BoolVar(&cfg.keep, "keep", false, "leave this run's ladder in Redis on exit, so a later run resumes it (for testing resumption)")
	flag.BoolVar(&cfg.cleanup, "cleanup", false, "retire every ladder this tool has left in Redis, then exit")
	flag.Parse()
	return cfg
}

// callRecorder counts what the Twilio client actually sent, and keeps the last
// TwiML document so a dry run can show what Twilio would have received.
type callRecorder struct {
	mu        sync.Mutex
	count     int
	lastTwiML string
}

func (r *callRecorder) record(twiml string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.count++
	r.lastTwiML = twiml
}

func (r *callRecorder) snapshot() (int, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.count, r.lastTwiML
}

// buildTwilioClient returns the real client, pointed either at Twilio or at a
// local stub. The stub matters: it means a dry run still marshals the TwiML
// through production code rather than skipping the call path entirely, which
// is the only way to eyeball the SSML document before dialling anyone.
func buildTwilioClient(cfg config, speaker *speaker) (*notifications.TwilioClient, *callRecorder, func(), error) {
	rec := &callRecorder{}
	if cfg.live {
		missing := []string{}
		for _, k := range []string{"TWILIO_ACCOUNT_SID", "TWILIO_AUTH_TOKEN", "TWILIO_FROM_NUMBER"} {
			if os.Getenv(k) == "" {
				missing = append(missing, k)
			}
		}
		if len(missing) > 0 {
			return nil, nil, nil, fmt.Errorf("--live needs %s set in .env or the environment", strings.Join(missing, ", "))
		}
		client := notifications.NewTwilioClient(notifications.TwilioConfig{
			AccountSID:         os.Getenv("TWILIO_ACCOUNT_SID"),
			AuthToken:          os.Getenv("TWILIO_AUTH_TOKEN"),
			FromNumber:         os.Getenv("TWILIO_FROM_NUMBER"),
			Voice:              os.Getenv("TWILIO_VOICE"),
			Language:           os.Getenv("TWILIO_LANGUAGE"),
			APIBaseURL:         os.Getenv("TWILIO_API_BASE_URL"),
			RingTimeoutSeconds: cfg.ringSeconds,
		})
		return client, rec, func() {}, nil
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		twiml := r.PostFormValue("Twiml")
		rec.record(twiml)
		// Answer first, then speak: the engine is waiting on this response,
		// and a fifteen-second message would otherwise look like a fifteen-
		// second Twilio call.
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"sid":"CA-local-stub","status":"queued"}`))
		if cfg.speak {
			speaker.play(twiml, cfg.sayVoice)
		}
	}))
	client := notifications.NewTwilioClient(notifications.TwilioConfig{
		AccountSID: "AC-local", AuthToken: "local", FromNumber: "+15550000000",
		Voice: os.Getenv("TWILIO_VOICE"), Language: os.Getenv("TWILIO_LANGUAGE"),
		APIBaseURL: srv.URL,
	})
	return client, rec, srv.Close, nil
}

// localChatClient builds the real Google Chat client from GOOGLE_CHAT_SPACES,
// so --channel chat posts to a genuine space. Nil when nothing is configured,
// which the engine reports rather than failing silently.
func localChatClient(cfg config) *notifications.GoogleChatClient {
	raw := os.Getenv("GOOGLE_CHAT_SPACES")
	if raw == "" {
		return nil
	}
	var spaces []notifications.GoogleChatAudienceSpace
	if err := json.Unmarshal([]byte(raw), &spaces); err != nil {
		fmt.Fprintf(os.Stderr, "warning: GOOGLE_CHAT_SPACES does not parse; --channel chat will have no notifier\n")
		return nil
	}
	return notifications.NewGoogleChatClient(notifications.GoogleChatConfig{AudienceSpaces: spaces})
}

// localResolver decides where the rungs come from.
//
// By default, the one recipient given on the command line, on every rung: the
// point of a local run is usually the ladder's timing and its cards, not who
// it reaches, and nobody wants a harness that needs a database to print a
// card.
//
// Set CUSTOMER_ENTITY_BASE_URL to resolve real people from the Team Schedule
// instead, the way a deployment does. That needs entity-service reachable and
// this harness registered as an internal client there, or every rung comes
// back 403 and the ladder reaches nobody.
//
// Remember that the rung model the schedule resolver implements is still an
// assumption. A run against it exercises a complete, valid flow, which is the
// point; it does not confirm the flow is right.
func localResolver(to string) escalation.Resolver {
	base := os.Getenv("CUSTOMER_ENTITY_BASE_URL")
	if base == "" {
		return ruleAwareRoster{
			Resolver: escalation.NewRosterResolver(localRoster(to)),
			rules:    escalation.DefaultRules,
			teams:    localTeamKeys(),
		}
	}
	fmt.Printf("resolving rungs from the Team Schedule at %s\n", base)
	return escalation.NewTeamScheduleResolver(
		escalation.NewEntityClient(escalation.EntityConfig{
			BaseURL:      base,
			TokenURL:     os.Getenv("OAUTH2_TOKEN_URL"),
			ClientID:     os.Getenv("OAUTH2_CLIENT_ID"),
			ClientSecret: os.Getenv("OAUTH2_CLIENT_SECRET"),
			Scopes:       splitCommaEnv("CUSTOMER_ENTITY_SCOPES"),
		}),
		localTeamKeys(),
		nil,
	)
}

// localTeamKeys mirrors the deployed configuration's own teams block, so a dry
// run routes by the same rules a real incident would. Overridable from the
// environment for a deployment whose ABTs differ.
func localTeamKeys() escalation.TeamKeys {
	abts := splitCommaEnv("INCIDENT_ESCALATION_ABT_TEAMS")
	if len(abts) == 0 {
		// The seven cre-abt teams, as escalation.yaml's cre.teams.abts names
		// them. This fallback used to be an older list that named two SRE
		// teams (apollo, artemis) and omitted two CRE ones (rigel, sirius):
		// escalation.yaml was corrected and this was not, so every local run
		// put SRE people on a CRE ladder -- in R3's one-per-ABT Level 0 and in
		// the Level 2 lead pool -- and never reached rigel or sirius at all.
		abts = []string{"atlas", "castor", "draco", "phoenix", "rigel", "sirius", "vega"}
	}
	americas := os.Getenv("INCIDENT_ESCALATION_AMERICAS_TEAM")
	if americas == "" {
		americas = "americas"
	}
	return escalation.TeamKeys{
		ABTs:       abts,
		Americas:   americas,
		Leadership: os.Getenv("INCIDENT_ESCALATION_LEADERSHIP_TEAM"),
	}
}

// splitCommaEnv reads an optional comma-separated list.
func splitCommaEnv(name string) []string {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return nil
	}
	return strings.Split(raw, ",")
}

// localRoster points every level at the one number under test, with names that
// say which level is calling — the point is to hear the ladder climb, not to
// model a real rotation.
func localRoster(to string) escalation.Roster {
	person := func(role string) escalation.Recipient {
		return escalation.Recipient{Email: role + "@local.invalid", Name: role, Phone: to}
	}
	// Named as the updated rule table names them, so the printed ladder is
	// recognisable against the spreadsheet rather than against this file.
	// LEVEL_1 is the incident's own team lead and LEVEL_2 is every team lead
	// — the reverse of the previous model, which is exactly the thing worth
	// seeing spelled out in a dry run.
	return escalation.Roster{Default: escalation.LevelRoster{
		"LEVEL_0": {person("first-responders")},
		"LEVEL_1": {person("team-lead")},
		"LEVEL_2": {person("team-leads")},
		"LEVEL_3": {person("cre-head")},
		"LEVEL_4": {person("cs-head")},
	}}
}

// ruleAwareRoster answers rungs from the local roster but routes by the real
// rule table.
//
// Without it a dry run silently skipped LEVEL_0 on every LK shift: the plan
// asks the resolver which rule applies and falls back to the previous model's
// "rotation shifts only" answer when it cannot say. The roster resolver could
// not, so the harness disagreed with the deployed service about the shape of
// the ladder — which is the one thing a harness must never do.
type ruleAwareRoster struct {
	escalation.Resolver
	rules []escalation.Rule
	teams escalation.TeamKeys
}

// RuleFor satisfies the same interface the Team Schedule resolver does.
func (r ruleAwareRoster) RuleFor(rc escalation.RoutingContext) (escalation.Rule, bool) {
	key := strings.ToLower(strings.TrimSpace(rc.AssignedCRETeam))
	isABT := false
	for _, k := range r.teams.ABTs {
		if strings.ToLower(strings.TrimSpace(k)) == key {
			isABT = true
			break
		}
	}
	return escalation.MatchRule(r.rules, rc.Shift, isABT, key != "")
}

// triggerTime picks an instant inside the requested shift, so the engine's own
// ShiftAt derives the shift under test rather than whatever the wall clock
// happens to be.
//
// It is the NEXT such instant, never a past one. The engine drops a trigger
// whose whole ladder is already over — the guard that stops a topic replay
// from burst-dialling stale incidents — so a fixed reference date days in the
// past would be refused. A trigger a few hours ahead is fine: every call is an
// offset from it, and runTicks feeds the engine a "now" measured from it.
// reportTime turns -at into the trigger instant.
//
// HH:MM is the next WEEKDAY at that IST time (or the next weekend day with
// -weekend), because the shift depends on the day as well as the hour -- 10:00
// is LK on a Tuesday and LK_WEEKEND on a Saturday -- and "today or tomorrow"
// would quietly change it on a Friday evening. A full date is taken as given.
//
// Either way it must be in the future. The engine drops a trigger whose whole
// ladder is already behind it, so a past time would either be refused outright
// or, if recent, have its early rungs fire in one burst on the first tick --
// neither of which is the run that was asked for.
func reportTime(at string, weekend bool, now time.Time) (time.Time, error) {
	at = strings.TrimSpace(at)
	var hh, mm int
	if n, _ := fmt.Sscanf(at, "%d:%d", &hh, &mm); n == 2 && len(at) <= 5 {
		if hh < 0 || hh > 23 || mm < 0 || mm > 59 {
			return time.Time{}, fmt.Errorf("-at %q is not a time of day", at)
		}
		for day := 0; day < 8; day++ {
			d := now.AddDate(0, 0, day)
			isWeekend := d.Weekday() == time.Saturday || d.Weekday() == time.Sunday
			if isWeekend != weekend {
				continue
			}
			t := time.Date(d.Year(), d.Month(), d.Day(), hh, mm, 0, 0, escalation.IST)
			if t.After(now) {
				return t, nil
			}
		}
		return time.Time{}, fmt.Errorf("-at %q: no matching day in the next week", at)
	}
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02 15:04"} {
		if t, err := time.ParseInLocation(layout, at, escalation.IST); err == nil {
			if !t.After(now) {
				return time.Time{}, fmt.Errorf("-at %s IST is in the past; give a future time, "+
					"since the engine drops a ladder that is already over", t.Format("Mon 2006-01-02 15:04"))
			}
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("-at %q: use HH:MM or YYYY-MM-DDTHH:MM (IST)", at)
}

func triggerTime(shift escalation.Shift) time.Time {
	hour, weekend := 11, false
	switch shift {
	case escalation.ShiftLKMorning:
		hour = 7
	case escalation.ShiftLKEvening:
		hour = 19
	case escalation.ShiftUSA:
		hour = 22
	case escalation.ShiftLKWeekend:
		hour, weekend = 10, true
	case escalation.ShiftUSAWeekend:
		hour, weekend = 22, true
	}
	now := time.Now().In(escalation.IST)
	for day := 0; day < 8; day++ {
		d := now.AddDate(0, 0, day)
		isWeekend := d.Weekday() == time.Saturday || d.Weekday() == time.Sunday
		if isWeekend != weekend {
			continue
		}
		at := time.Date(d.Year(), d.Month(), d.Day(), hour, 0, 0, 0, escalation.IST)
		if at.After(now) {
			return at
		}
	}
	// Unreachable: eight days always contain both a weekday and a weekend
	// day with the hour still ahead.
	return now
}

// defaultTeam is the assignment group a run uses when -team is not given. It
// is deliberately NOT an ABT key: the not-an-ABT row is the one a local run is
// most likely to want checked, since it is the row an unmapped ServiceNow
// group falls to in production.
const defaultTeam = "Americas CS Team - Integraion"

// startRecord builds the incident.created or incident.priority_elevated
// envelope that starts the ladder — the same bytes Event Hub would carry.
func startRecord(cfg config, at time.Time) eventbus.Record {
	if cfg.kind == "elevated" {
		return envelope(cfg.incidentID, events.TypeIncidentPriorityElevated, events.IncidentPriorityElevatedPayload{
			OldPriority: "MODERATE",
			NewPriority: cfg.priority,
			Title:       "Gateway returning 500s in production",
			Number:      "INC0012345",
			Account:     "Automation Test Account",
			Team:        cfg.team,
			Product:     "WSO2 API Manager",
			ABTEligible: abtFlag(cfg.notABT),
			ElevatedAt:  at.Format(time.RFC3339),
		})
	}
	return envelope(cfg.incidentID, events.TypeIncidentCreated, events.IncidentCreatedPayload{
		Title:            "Gateway returning 500s in production",
		ShortDescription: "Every request to the gateway is failing",
		Number:           "INC0012345",
		Priority:         cfg.priority,
		Account:          "Automation Test Account",
		Team:             cfg.team,
		Product:          "WSO2 API Manager",
		ABTEligible:      abtFlag(cfg.notABT),
		ReportedAt:       at.Format(time.RFC3339),
	})
}

// cancelRecord builds whichever acknowledgement gesture was asked for: the
// public comment an elevation's voice message instructs, or the move out of
// NEW a new incident's does.
func cancelRecord(cfg config) eventbus.Record {
	if cfg.cancelBy == "status" {
		return statusRecord(cfg)
	}
	return commentRecord(cfg)
}

func statusRecord(cfg config) eventbus.Record {
	return envelope(cfg.incidentID, events.TypeIncidentAcknowledged, events.IncidentAcknowledgedPayload{
		PreviousState: "NEW", NewState: "IN_PROGRESS",
	})
}

func commentRecord(cfg config) eventbus.Record {
	return envelope(cfg.incidentID, events.TypeIncidentCommentAdded, events.IncidentCommentAddedPayload{
		CommentID: "local-comment-1", IsPublic: true,
	})
}

// cancelRecords is every gesture this run acknowledges with.
//
// A CRE ladder stops on BOTH a move out of NEW and a public comment; either
// alone is recorded and the ladder keeps climbing, which is the whole point of
// the rule. -cancel-by comment and -cancel-by status therefore demonstrate a
// ladder that does NOT stop -- useful, and what the half-ack scenario wants,
// but not an acknowledgement. "both" is the default so that asking this tool
// to acknowledge actually acknowledges.
func cancelRecords(cfg config) []eventbus.Record {
	switch cfg.cancelBy {
	case "status":
		return []eventbus.Record{statusRecord(cfg)}
	case "comment":
		return []eventbus.Record{commentRecord(cfg)}
	default:
		return []eventbus.Record{statusRecord(cfg), commentRecord(cfg)}
	}
}

// dueByTime reports whether -cancel-after has elapsed. Separate so the
// level-triggered path reads as the alternative it is.
func dueByTime(cfg config, elapsed time.Duration) bool {
	return cfg.cancelAt == "" && cfg.cancelAfter > 0 && elapsed >= cfg.cancelAfter
}

// gestureName describes what -cancel-by will send, for the run's own log.
func gestureName(by string) string {
	switch by {
	case "status":
		return "a move out of NEW (one gesture only -- the ladder keeps climbing)"
	case "comment":
		return "a public comment (one gesture only -- the ladder keeps climbing)"
	default:
		return "a move out of NEW AND a public comment"
	}
}

func envelope(entityID string, t events.Type, payload any) eventbus.Record {
	raw, err := json.Marshal(payload)
	if err != nil {
		panic(err) // only reachable from a programming error in this file
	}
	body, err := json.Marshal(events.Envelope{Type: t, EntityID: entityID, Payload: raw})
	if err != nil {
		panic(err)
	}
	return eventbus.Record{Value: body}
}

// runTicks drives the engine's own Tick on the compressed clock and fires the
// acknowledgement part-way when asked.
func runTicks(ctx context.Context, cfg config, engine *escalation.Engine, rdb *redis.Client, trigger time.Time, rec *callRecorder, plan escalation.Plan, runStart time.Time) error {
	fmt.Printf("\n  running (ctrl-c to stop)...\n\n")
	store := escalation.NewStore(rdb)
	ticker := time.NewTicker(cfg.tick)
	defer ticker.Stop()

	realStart := time.Now()
	cancelled := false
	var cancelledAtLadderTime *time.Time
	// The engine deletes a ladder's state the moment it finishes or is
	// acknowledged, so the placed flags have to be kept as they are observed —
	// otherwise the summary falls back to "everything scheduled before the
	// cancellation", which over-reports exactly the calls the cancellation
	// retired.
	var lastPlaced []bool
	var lastFailed []string
	seen := 0
	reachedCancelLevel := false

	for {
		select {
		case <-ctx.Done():
			fmt.Printf("\n  interrupted\n")
			return summarise(context.Background(), cfg, store, rec, plan, cancelledAtLadderTime, lastPlaced, lastFailed, runStart)
		case <-ticker.C:
			elapsed := time.Since(realStart)

			// Ladder time, expanded back out from the compressed real clock.
			now := trigger.Add(time.Duration(float64(elapsed) / float64(cfg.minute) * float64(time.Minute)))

			if !cancelled && (dueByTime(cfg, elapsed) || reachedCancelLevel) {
				cancelled = true
				at := now
				cancelledAtLadderTime = &at
				fmt.Printf("  [%7s] ACKNOWLEDGED by %s — the engine should stop calling\n",
					short(elapsed), gestureName(cfg.cancelBy))
				// Each gesture is a separate event, exactly as entity-service
				// publishes them, so the engine's own both-gestures rule is
				// what decides whether this stops the ladder.
				for _, r := range cancelRecords(cfg) {
					if err := engine.Handle(ctx, r); err != nil {
						return fmt.Errorf("acknowledging: %w", err)
					}
				}
			}

			if err := engine.Tick(ctx, now); err != nil {
				fmt.Printf("  [%7s] tick error: %v\n", short(elapsed), err)
			}

			// Report progress from the engine's own durable state rather than
			// by intercepting anything, so this reads exactly what a restart
			// would resume from.
			st, found, err := store.Get(ctx, cfg.incidentID)
			if err != nil {
				return fmt.Errorf("reading the stored ladder: %w", err)
			}
			if !found {
				fmt.Printf("\n  the engine has finished with this incident and cleared its state\n")
				return summarise(ctx, cfg, store, rec, plan, cancelledAtLadderTime, lastPlaced, lastFailed, runStart)
			}
			lastPlaced = append(lastPlaced[:0], st.Placed...)
			lastFailed = append(lastFailed[:0], st.Failed...)
			for i, done := range st.Placed {
				if done && cfg.cancelAt != "" && !cancelled &&
					strings.EqualFold(st.Plan.Calls[i].Level.String(), cfg.cancelAt) {
					reachedCancelLevel = true
				}
				if done && i >= seen {
					c := st.Plan.Calls[i]
					fmt.Printf("  [%7s] %-8s #%d  %-14s called %s  (ladder +%s)\n",
						short(elapsed), c.Level, c.Ordinal, c.Recipient.Name,
						maskPhone(c.Recipient.Phone), short(c.At.Sub(trigger)))
					seen = i + 1
				}
			}
		}
	}
}

// summarise prints what the engine would have written back to the incident as
// a work note, plus what the Twilio client actually sent.
func summarise(ctx context.Context, cfg config, store *escalation.Store, rec *callRecorder, plan escalation.Plan, localCancelledAt *time.Time, observedPlaced []bool, observedFailed []string, runStart time.Time) error {
	count, twiml := rec.snapshot()

	fmt.Printf("\n%s\n", strings.Repeat("-", 78))
	fmt.Printf("Execution summary (this is the work note the engine writes back)\n")
	fmt.Printf("%s\n", strings.Repeat("-", 78))

	// The plan captured at startup is what makes this printable at all: a
	// ladder that finishes or is acknowledged has had its state deleted by the
	// engine by now, so reading it back would show nothing. Cancellation
	// details still come from the stored state when it is there, since only
	// the engine knows when and why it stopped.
	// An acknowledged ladder has had its state deleted by the engine before
	// this runs, taking the cancellation with it — which is exactly the run
	// whose summary matters most. Fall back to what this tool itself knows:
	// when it sent the acknowledgement, and which gesture it used.
	placed, failed := observedPlaced, observedFailed
	cancelledAt, reason := localCancelledAt, ""
	if cancelledAt != nil {
		reason = "Public comment added"
		if cfg.cancelBy == "status" {
			reason = "Acknowledged"
		}
	}
	if st, found, err := store.Get(ctx, cfg.incidentID); err == nil && found {
		plan, placed, failed = st.Plan, st.Placed, st.Failed
		if st.Cancelled != nil {
			cancelledAt, reason = st.Cancelled, st.CancelReason
		}
	}
	for _, line := range plan.ExecutionSummary(placed, failed, cancelledAt, reason) {
		fmt.Println(line)
	}

	fmt.Printf("\n%d call(s) reached %s.\n", count,
		map[bool]string{true: "Twilio", false: "the local stub"}[cfg.live])
	if cfg.live {
		reportCallOutcomes(ctx, cfg, runStart)
	}
	if twiml != "" && (cfg.showTwiML || !cfg.live) {
		fmt.Printf("\nThe TwiML of the last call, as production code built it:\n%s\n", twiml)
	}
	return nil
}

func printHeader(cfg config, plan escalation.Plan, trigger time.Time, to string) {
	mode := "DRY RUN - calls go to a local stub, nothing is dialled"
	if cfg.live {
		mode = fmt.Sprintf("LIVE - up to %d real call(s) to %s", len(plan.Calls), maskPhone(to))
	}
	fmt.Printf("%s\n", strings.Repeat("=", 78))
	fmt.Printf("Incident call escalation - the real engine, on local Redis - %s\n", mode)
	fmt.Printf("%s\n", strings.Repeat("=", 78))
	fmt.Printf("  incident        %s\n", cfg.incidentID)
	fmt.Printf("  priority        %s\n", cfg.priority)
	fmt.Printf("  reported at     %s\n", trigger.Format("Mon 2006-01-02 15:04 MST"))
	fmt.Printf("  shift           %s (derived by the engine; ABT-eligible: %s)\n",
		plan.Trigger.Routing.Shift, plan.Trigger.Routing.ABTEligibility())
	fmt.Printf("  trigger         %s\n", plan.Trigger.Kind)
	fmt.Printf("  message         %s\n", map[bool]string{true: "SSML", false: "plain"}[cfg.ssml])
	fmt.Printf("  clock           1 ladder minute = %s\n", cfg.minute)
	if cfg.speak {
		fmt.Printf("  audio           speaking each call aloud (voice %s) — this stretches the compressed clock\n", cfg.sayVoice)
	}
	if cfg.live && cfg.ringSeconds > 0 {
		fmt.Printf("  ring            %ds, then Twilio gives up on the call\n", cfg.ringSeconds)
	}
	if cfg.cancelAt != "" {
		fmt.Printf("  acknowledge at  %s, by %s\n", strings.ToUpper(cfg.cancelAt), gestureName(cfg.cancelBy))
	} else if cfg.cancelAfter > 0 {
		fmt.Printf("  acknowledge at  %s into the run, by %s\n", cfg.cancelAfter, gestureName(cfg.cancelBy))
	}
	fmt.Printf("\n  the ladder the engine scheduled (%d calls):\n", len(plan.Calls))
	for _, c := range plan.Calls {
		fmt.Printf("    +%-8s %-8s #%d  %s\n", short(c.At.Sub(trigger)), c.Level, c.Ordinal, c.Recipient.Name)
	}
	for _, is := range plan.Issues {
		fmt.Printf("    %-9s %-8s %s %s\n", "", is.Level, is.Reason, is.Detail)
	}
}

// openRedis follows the service's own precedence: REDIS_URL (a rediss://
// connection string — the managed, TLS instance a deployment uses) ahead of
// the plain --redis / REDIS_ADDR address. Running against the managed
// instance is legitimate here: the engine's keys are its own namespace, this
// tool's incident ids are prefixed "local-", and it retires its ladder on the
// way out — but it does share that Redis with whatever else uses it, so
// --cleanup exists for the run that was interrupted before it could.
func openRedis(cfg config) (*redis.Client, string, error) {
	if url := os.Getenv("REDIS_URL"); url != "" {
		opts, err := redis.ParseURL(url)
		if err != nil {
			// Not echoing the URL: it carries the password.
			return nil, "", errors.New("REDIS_URL is set but does not parse as a redis:// or rediss:// URL")
		}
		opts.MaxRetries = -1
		return redis.NewClient(opts), opts.Addr + " (from REDIS_URL)", nil
	}
	return redis.NewClient(&redis.Options{Addr: cfg.redisAddr, MaxRetries: -1}), cfg.redisAddr, nil
}

// reportCallOutcomes asks Twilio what became of the calls this run placed.
//
// Placing a call returns "queued" — that means Twilio accepted the request,
// not that a phone rang. Only the call resource read back afterwards
// distinguishes ringing from answered from no-answer from failed, and that is
// the difference between "the alert was triggered" and "the alert was
// delivered".
//
// It queries by destination and start time rather than by the sids the engine
// placed: the engine holds the concrete Twilio client and logs its own sids,
// so intercepting them here would mean wrapping a type this tool deliberately
// does not own. Since every call in a run goes to the one --to number, "calls
// to this number since the run began" is the same set.
func reportCallOutcomes(ctx context.Context, cfg config, since time.Time) {
	acct, token := os.Getenv("TWILIO_ACCOUNT_SID"), os.Getenv("TWILIO_AUTH_TOKEN")
	if acct == "" || token == "" {
		return
	}
	base := os.Getenv("TWILIO_API_BASE_URL")
	if base == "" {
		base = "https://api.twilio.com/2010-04-01"
	}

	fmt.Printf("\n%s\n", strings.Repeat("-", 78))
	fmt.Printf("What Twilio says became of each call\n")
	fmt.Printf("%s\n", strings.Repeat("-", 78))
	fmt.Printf("  (waiting for the calls to reach a final state…)\n\n")

	client := &http.Client{Timeout: 15 * time.Second}
	calls := pollCalls(ctx, client, base, acct, token, cfg.to, since)
	if len(calls) == 0 {
		fmt.Printf("  Twilio reports no calls to this number since the run started.\n")
		return
	}
	for _, c := range calls {
		line := fmt.Sprintf("  %s  %-11s", c.SID, c.Status)
		if c.Duration != "" && c.Duration != "0" {
			line += "  " + c.Duration + "s of audio"
		}
		fmt.Println(line)
	}
	fmt.Printf("\n  completed / in-progress  the phone rang and was answered\n")
	fmt.Printf("  ringing                  still dialling as this printed\n")
	fmt.Printf("  no-answer / busy         it rang, nobody took it\n")
	fmt.Printf("  failed / canceled        it never rang — check the status above\n")
}

// twilioCall is the part of a call resource this report reads.
type twilioCall struct {
	SID      string `json:"sid"`
	Status   string `json:"status"`
	Duration string `json:"duration"`
}

// pollCalls re-reads the run's calls until each has reached a final state, so
// the reported outcome is what actually happened rather than whatever it
// looked like a moment after dialling.
func pollCalls(ctx context.Context, client *http.Client, base, acct, token, to string, since time.Time) []twilioCall {
	const attempts = 15
	var calls []twilioCall
	for i := 0; i < attempts; i++ {
		fetched, err := listCalls(ctx, client, base, acct, token, to, since)
		if err != nil {
			return calls
		}
		calls = fetched
		settled := len(calls) > 0
		for _, c := range calls {
			switch c.Status {
			case "completed", "busy", "no-answer", "failed", "canceled":
			default:
				settled = false
			}
		}
		if settled {
			return calls
		}
		select {
		case <-ctx.Done():
			return calls
		case <-time.After(2 * time.Second):
		}
	}
	return calls
}

// listCalls fetches this run's calls, newest first, reversed so the report
// reads in the order the ladder placed them.
func listCalls(ctx context.Context, client *http.Client, base, acct, token, to string, since time.Time) ([]twilioCall, error) {
	q := url.Values{}
	q.Set("To", to)
	// Twilio's StartTime filter has minute granularity and is inclusive, so a
	// minute of slack cannot miss a call placed in the same minute the run
	// began.
	q.Set("StartTime>", since.Add(-time.Minute).UTC().Format("2006-01-02T15:04:05Z"))
	q.Set("PageSize", "50")
	endpoint := fmt.Sprintf("%s/Accounts/%s/Calls.json?%s", base, url.PathEscape(acct), q.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(acct, token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var body struct {
		Calls []twilioCall `json:"calls"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	for i, j := 0, len(body.Calls)-1; i < j; i, j = i+1, j-1 {
		body.Calls[i], body.Calls[j] = body.Calls[j], body.Calls[i]
	}
	return body.Calls, nil
}

// retireLadder drops a ladder's outstanding calls and its state, so nothing
// resumes it after this process exits.
//
// The wake entries are found by scanning the index rather than by reading
// them out of the stored state, because the two can come apart: a ladder
// abandoned between seeding its wakes and writing — or after losing — its
// state would otherwise strand every one of those entries in a sorted set
// shared with every other ladder, rescanned on every tick, with nothing able
// to reclaim them. Scanning means this works from either half alone.
func retireLadder(ctx context.Context, store *escalation.Store, incidentID string) error {
	// Deleting the state before the wakes are gone strands them: the entries
	// live in an index shared with every other ladder, and nothing can
	// reclaim one whose state no longer exists. So a failed scan has to stop
	// this, not be treated as "no entries found".
	members, err := wakeMembersFor(ctx, store, incidentID)
	if err != nil {
		return err
	}
	if err := store.RemoveWakes(ctx, members...); err != nil {
		return err
	}
	return store.Delete(ctx, incidentID)
}

// wakeMembersFor lists every scheduled call belonging to one incident.
//
// "Due arbitrarily far in the future" is how the whole index is read: the
// store exposes a due-by query, and a decade ahead covers every entry it
// could hold.
func wakeMembersFor(ctx context.Context, store *escalation.Store, incidentID string) ([]string, error) {
	members, err := store.DueMembers(ctx, time.Now().AddDate(10, 0, 0))
	if err != nil {
		return nil, err
	}
	prefix := incidentID + "|"
	var mine []string
	for _, m := range members {
		if strings.HasPrefix(m, prefix) {
			mine = append(mine, m)
		}
	}
	return mine, nil
}

// cleanupLocalLadders retires every ladder this tool has ever left behind —
// the escape hatch for a developer who interrupted a run before --keep existed,
// or who used --keep and is now done with it.
//
// Scans the wake index by asking for everything due arbitrarily far in the
// future, which is every member it holds, and acts only on this tool's own
// incident ids so a real ladder sharing the Redis is never touched.
func cleanupLocalLadders(ctx context.Context, store *escalation.Store, rdb *redis.Client) error {
	members, err := store.DueMembers(ctx, time.Now().AddDate(10, 0, 0))
	if err != nil {
		return fmt.Errorf("scanning the wake index: %w", err)
	}
	ids := map[string]bool{}
	for _, m := range members {
		if i := strings.LastIndex(m, "|"); i > 0 && strings.HasPrefix(m[:i], "local-") {
			ids[m[:i]] = true
		}
	}
	// State keys can outlive their wake entries (every call placed, no
	// cancellation), so sweep those too.
	keys, err := rdb.Keys(ctx, "incident:escalation:state:local-*").Result()
	if err != nil {
		return fmt.Errorf("scanning ladder state: %w", err)
	}
	for _, k := range keys {
		ids[strings.TrimPrefix(k, "incident:escalation:state:")] = true
	}

	if len(ids) == 0 {
		fmt.Println("nothing to clean up: this tool has left no ladders in Redis")
		return nil
	}
	for id := range ids {
		if err := retireLadder(ctx, store, id); err != nil {
			return fmt.Errorf("retiring %s: %w", id, err)
		}
		fmt.Printf("retired %s\n", id)
	}
	return nil
}

// short renders a duration without the noise of sub-second precision.
func short(d time.Duration) string {
	return d.Round(time.Second).String()
}

// maskPhone keeps only the last four digits, matching the convention the rest
// of this service logs numbers with.
func maskPhone(phone string) string {
	if len(phone) <= 4 {
		return "****"
	}
	return "********" + phone[len(phone)-4:]
}

// quietLogger discards go-redis's internal chatter. Its Printf signature is
// what redis.SetLogger's own (internal) interface requires.
type quietLogger struct{}

func (quietLogger) Printf(context.Context, string, ...interface{}) {}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// loadDotEnv reads a .env file and sets any unset environment variables from
// it, so Twilio credentials can live alongside the service's own config.
func loadDotEnv(path string) {
	f, err := os.Open(path) // #nosec G304 -- path is always the hardcoded literal ".env" at the only call site
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.Trim(strings.TrimSpace(v), `"'`)
		if _, exists := os.LookupEnv(k); !exists {
			_ = os.Setenv(k, v)
		}
	}
}

// abtFlag turns the --not-abt flag into the definite answer the payload now
// carries. A harness always knows which side it is testing, so it never sends
// the "unknown" a real publisher currently does.
func abtFlag(notABT bool) *bool {
	eligible := !notABT
	return &eligible
}
