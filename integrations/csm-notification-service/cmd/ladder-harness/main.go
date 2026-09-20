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

// Command ladder-harness walks one incident call-escalation ladder against a
// single phone number, on a compressed clock, placing the calls itself.
//
// It predates the engine and is no longer the tool to reach for. It builds a
// real plan with internal/escalation and then dials it directly, so it
// exercises the timing table and the spoken message but none of the engine:
// no durable state, no idempotency under redelivery, no resumption after a
// restart, and no cancellation driven by a real event — its --ack-after is a
// flag, not an incoming signal. cmd/escalation-local drives the real engine
// against a real Redis and real event envelopes, and is what a change to the
// ladder should be tested with.
//
// What this is still good for: seeing what a ladder looks like with nothing
// running but Go. No Redis, no event bus, no engine — a plan and a clock.
//
// Safety: it never dials until --live is passed, --live refuses to run without
// an explicit --to number (so it can never page whoever
// INCIDENT_DEFAULT_CALL_TO points at), and it caps how many calls it will
// place. A dry run with no --to falls back to an unassignable placeholder.
//
// Usage:
//
//	# See the whole P1 ladder without dialling anything
//	go run ./cmd/ladder-harness --priority P1
//
//	# Place the calls for real, one ladder minute per second, to your phone
//	go run ./cmd/ladder-harness --priority P1 --to +94771234567 --live
//
//	# Acknowledge 12 compressed seconds in and watch the rest cancel
//	go run ./cmd/ladder-harness --priority P1 --to +94771234567 --live --ack-after 12s
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/escalation"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

// maxHarnessCallsCeiling bounds --max-calls itself. A full ladder is 14 calls;
// anything far beyond that means a mistake in the flags, and the cost of a
// mistake here is real phone calls.
const maxHarnessCallsCeiling = 30

// dryRunPlaceholderNumber stands in for --to when nothing is being dialled, so
// the ladder can be inspected without inventing a number that might be real.
// It is inside the +1-000 range, which is not assignable to anyone.
const dryRunPlaceholderNumber = "+10000000000"

type config struct {
	priority  string
	shift     string
	notABT    bool
	kind      string
	to        string
	minute    time.Duration
	ackAfter  time.Duration
	live      bool
	maxCalls  int
	useSSML   bool
	account   string
	team      string
	caseID    string
	incNumber string
}

func main() {
	cfg := parseFlags()

	// Mirrors cmd/server's own .env handling so the harness picks up the same
	// Twilio credentials the service would use.
	loadDotEnv(".env")

	if err := run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "\nladder-harness: %v\n", err)
		os.Exit(1)
	}
}

func parseFlags() config {
	var cfg config
	flag.StringVar(&cfg.priority, "priority", "P1", "incident priority: P0-P4, or CRITICAL/HIGH/MODERATE/LOW/PLANNING")
	flag.StringVar(&cfg.shift, "shift", "LK_MORNING", "shift when reported: LK, LK_MORNING, LK_EVENING, LK_WEEKEND, USA, USA_WEEKEND (a rotation shift adds LEVEL_0)")
	flag.BoolVar(&cfg.notABT, "not-abt", false, "treat the account as NOT ABT-eligible (the IAM side), which is what gives a USA_WEEKEND incident a LEVEL_0 - see rules R10 vs R12")
	flag.StringVar(&cfg.kind, "kind", "new", "what started the ladder: new or elevated")
	flag.StringVar(&cfg.to, "to", "", "E.164 number every level calls, e.g. +94771234567 (required with --live)")
	flag.DurationVar(&cfg.minute, "minute", time.Second, "how long one real ladder minute lasts, e.g. 1s compresses a 44m P1 ladder into 44s")
	flag.DurationVar(&cfg.ackAfter, "ack-after", 0, "acknowledge this far into the compressed run and cancel the rest; 0 runs the whole ladder")
	flag.BoolVar(&cfg.live, "live", false, "actually place calls through Twilio; without this nothing is dialled")
	flag.IntVar(&cfg.maxCalls, "max-calls", 20, "refuse to place more than this many calls")
	flag.BoolVar(&cfg.useSSML, "ssml", false, "speak the SSML message (pauses, and the case reference slowed down) instead of the plain one")
	flag.StringVar(&cfg.account, "account", "Automation Test Account", "account name spoken in the call")
	flag.StringVar(&cfg.team, "team", "Americas CS Team - Integration", "CS team spoken in the call")
	flag.StringVar(&cfg.caseID, "case-id", "WSO2-1042", "internal case reference spoken in the call")
	flag.StringVar(&cfg.incNumber, "number", "INC0012345", "incident number shown in the execution summary")
	flag.Parse()
	return cfg
}

func run(cfg config) error {
	if cfg.minute <= 0 {
		return fmt.Errorf("--minute must be positive, got %s", cfg.minute)
	}
	if cfg.maxCalls < 1 || cfg.maxCalls > maxHarnessCallsCeiling {
		return fmt.Errorf("--max-calls must be between 1 and %d, got %d", maxHarnessCallsCeiling, cfg.maxCalls)
	}

	// Resolve the number before building the plan. An empty --to would
	// otherwise make every level NO_NUMBER, and the run would fail with
	// "plotted no calls" — true, but it hides the actual mistake.
	if cfg.live && cfg.to == "" {
		return fmt.Errorf("--live needs an explicit --to number; the harness will not fall back to INCIDENT_DEFAULT_CALL_TO")
	}
	if cfg.to == "" {
		cfg.to = dryRunPlaceholderNumber
		fmt.Printf("no --to given; using the placeholder %s for this dry run\n\n", cfg.to)
	}

	kind := escalation.TriggerNewIncident
	switch strings.ToLower(cfg.kind) {
	case "new":
	case "elevated":
		kind = escalation.TriggerPriorityElevated
	default:
		return fmt.Errorf("--kind must be new or elevated, got %q", cfg.kind)
	}

	shift, err := parseShift(cfg.shift)
	if err != nil {
		return err
	}

	// A ladder is built from a real trigger and expanded by the real planner —
	// the point of the harness is that this half is not simulated.
	trigger := escalation.Trigger{
		IncidentID: "harness-0000-0000-0000-000000000000",
		Number:     cfg.incNumber,
		WSO2CaseID: cfg.caseID,
		Priority:   cfg.priority,
		Title:      "Harness run: gateway returning 500s",
		Account:    cfg.account,
		Team:       cfg.team,
		Kind:       kind,
		At:         time.Now(),
		Routing: escalation.RoutingContext{
			Product:         "WSO2 API Manager",
			ABTEligible:     abtFlag(cfg.notABT),
			AssignedCRETeam: "Atlas",
			Shift:           shift,
		},
	}

	if _, ok := escalation.Lookup(escalation.DefaultPolicy, trigger.Priority); !ok {
		return fmt.Errorf("no escalation policy for priority %q", trigger.Priority)
	}

	plan, err := escalation.BuildPlan(context.Background(), trigger, escalation.DefaultPolicy, harnessResolver(cfg.to))
	if err != nil {
		return err
	}
	if len(plan.Calls) == 0 {
		return fmt.Errorf("the plan plotted no calls; nothing to run")
	}
	if len(plan.Calls) > cfg.maxCalls {
		return fmt.Errorf("plan has %d calls, which exceeds --max-calls=%d; raise it deliberately or narrow the ladder",
			len(plan.Calls), cfg.maxCalls)
	}

	printPlan(cfg, trigger, plan)

	caller, err := buildCaller(cfg)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	placed, cancelledAt := runLadder(ctx, cfg, trigger, plan, caller)

	fmt.Printf("\n%s\n", strings.Repeat("-", 78))
	fmt.Printf("Execution summary (this is the work note the engine would write back)\n")
	fmt.Printf("%s\n", strings.Repeat("-", 78))
	// The harness acknowledges by the gesture matching the trigger, which is
	// how section 3.0 pairs them: a status change for a new incident, a public
	// comment for a priority elevation.
	reason := "Acknowledged"
	if trigger.Kind == escalation.TriggerPriorityElevated {
		reason = "Public comment added"
	}
	for _, line := range plan.ExecutionSummary(nil, nil, cancelledAt, reason) {
		fmt.Println(line)
	}
	fmt.Printf("\n%d call(s) %s.\n", placed, map[bool]string{true: "placed", false: "simulated"}[cfg.live])
	return nil
}

// harnessResolver points every level at the one number under test, with a
// per-level identity so the execution summary stays readable. The names are
// the specification's own roles; the addresses use .invalid, a reserved TLD,
// so a stray email can never reach anyone.
func harnessResolver(to string) escalation.Resolver {
	name := func(role string) escalation.Recipient {
		return escalation.Recipient{
			Email: fmt.Sprintf("%s@harness.invalid", role),
			Name:  role,
			Phone: to,
		}
	}
	return escalation.StaticResolver{ByLevel: map[escalation.Level][]escalation.Recipient{
		escalation.Level0: {name("rotation-engineer")},
		escalation.Level1: {name("sub-lead")},
		escalation.Level2: {name("team-lead")},
		escalation.Level3: {name("bu-head")},
		escalation.Level4: {name("head-of-cre")},
	}}
}

func parseShift(s string) (escalation.Shift, error) {
	switch escalation.Shift(strings.ToUpper(s)) {
	case escalation.ShiftLK:
		return escalation.ShiftLK, nil
	case escalation.ShiftLKMorning:
		return escalation.ShiftLKMorning, nil
	case escalation.ShiftLKEvening:
		return escalation.ShiftLKEvening, nil
	case escalation.ShiftLKWeekend:
		return escalation.ShiftLKWeekend, nil
	case escalation.ShiftUSA:
		return escalation.ShiftUSA, nil
	case escalation.ShiftUSAWeekend:
		return escalation.ShiftUSAWeekend, nil
	default:
		return "", fmt.Errorf("unknown --shift %q", s)
	}
}

// caller places one call for a trigger, or reports what it would have placed.
// Taking the trigger rather than a rendered string is what lets --ssml pick the
// structured document over the flat one at the point of dialling.
type caller func(ctx context.Context, to string, t escalation.Trigger) error

func buildCaller(cfg config) (caller, error) {
	if !cfg.live {
		return func(_ context.Context, _ string, _ escalation.Trigger) error { return nil }, nil
	}
	missing := []string{}
	for _, k := range []string{"TWILIO_ACCOUNT_SID", "TWILIO_AUTH_TOKEN", "TWILIO_FROM_NUMBER"} {
		if os.Getenv(k) == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("--live needs %s set in .env or the environment", strings.Join(missing, ", "))
	}
	client := notifications.NewTwilioClient(notifications.TwilioConfig{
		AccountSID: os.Getenv("TWILIO_ACCOUNT_SID"),
		AuthToken:  os.Getenv("TWILIO_AUTH_TOKEN"),
		FromNumber: os.Getenv("TWILIO_FROM_NUMBER"),
		Voice:      os.Getenv("TWILIO_VOICE"),
		Language:   os.Getenv("TWILIO_LANGUAGE"),
		APIBaseURL: os.Getenv("TWILIO_API_BASE_URL"),
	})
	if cfg.useSSML {
		return func(ctx context.Context, to string, t escalation.Trigger) error {
			_, err := client.MakeSSMLCall(ctx, to, t.VoiceSpeech())
			return err
		}, nil
	}
	return func(ctx context.Context, to string, t escalation.Trigger) error {
		_, err := client.MakeCall(ctx, to, t.VoiceMessagePlain())
		return err
	}, nil
}

func printPlan(cfg config, trigger escalation.Trigger, plan escalation.Plan) {
	mode := "DRY RUN - nothing will be dialled"
	if cfg.live {
		mode = fmt.Sprintf("LIVE - %d real call(s) to %s", len(plan.Calls), cfg.to)
	}

	fmt.Printf("%s\n", strings.Repeat("=", 78))
	fmt.Printf("Incident call escalation ladder - %s\n", mode)
	fmt.Printf("%s\n", strings.Repeat("=", 78))
	fmt.Printf("  priority        %s\n", trigger.Priority)
	fmt.Printf("  shift           %s (rotation: %t, ABT-eligible: %s, so LEVEL_0 %s)\n",
		trigger.Routing.Shift, trigger.Routing.Shift.IsRotation(), trigger.Routing.ABTEligibility(),
		map[bool]string{true: "runs", false: "is skipped"}[trigger.Routing.HasNotificationLevel()])
	fmt.Printf("  trigger         %s\n", trigger.Kind)
	fmt.Printf("  clock           1 ladder minute = %s\n", cfg.minute)
	if cfg.ackAfter > 0 {
		fmt.Printf("  acknowledge at  %s into the run (ladder minute %.0f)\n",
			cfg.ackAfter, float64(cfg.ackAfter)/float64(cfg.minute))
	}
	fmt.Printf("\n  the ladder, in real time:\n")
	for _, c := range plan.Calls {
		offset := c.At.Sub(trigger.At)
		fmt.Printf("    +%-7s %-8s #%d  %s\n", short(offset), c.Level, c.Ordinal, c.Recipient.Name)
	}
	for _, is := range plan.Issues {
		fmt.Printf("    %-8s %-8s %s %s\n", "", is.Level, is.Reason, is.Detail)
	}

	// Always the plain rendering, and the label says so rather than
	// announcing SSML and printing something else. The SSML document is a
	// tree of elements, not a string, so printing it would mean showing
	// markup — and the words are identical either way. What --ssml changes is
	// how a real call sounds (pauses, a slowed case reference), which
	// cmd/escalation-local --speak is the way to hear.
	fmt.Printf("\n  spoken message (plain rendering; --ssml changes the delivery, not the words):\n    %s\n",
		trigger.VoiceMessagePlain())
	fmt.Printf("\n  running (ctrl-c to stop)...\n\n")
}

// runLadder walks the plan on the compressed clock, placing each call as it
// comes due. It returns how many calls went out and, if the run was
// acknowledged or interrupted, the real-clock instant that happened at.
func runLadder(ctx context.Context, cfg config, trigger escalation.Trigger, plan escalation.Plan, place caller) (int, *time.Time) {
	start := time.Now()
	var placed int

	for _, c := range plan.Calls {
		realOffset := c.At.Sub(trigger.At)
		due := start.Add(compress(realOffset, cfg.minute))

		if cfg.ackAfter > 0 && !due.Before(start.Add(cfg.ackAfter)) {
			at := trigger.At.Add(expand(cfg.ackAfter, cfg.minute))
			fmt.Printf("  [%6s] ACKNOWLEDGED - cancelling every remaining call\n", short(cfg.ackAfter))
			return placed, &at
		}

		if wait := time.Until(due); wait > 0 {
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				at := trigger.At.Add(expand(time.Since(start), cfg.minute))
				fmt.Printf("\n  interrupted - treating it as an acknowledgement for the summary\n")
				return placed, &at
			}
		}

		elapsed := time.Since(start).Round(time.Millisecond)
		if err := place(ctx, c.Recipient.Phone, trigger); err != nil {
			fmt.Printf("  [%6s] %-8s #%d  %-18s FAILED: %v\n", short(elapsed), c.Level, c.Ordinal, c.Recipient.Name, err)
			continue
		}
		placed++
		verb := "would call"
		if cfg.live {
			verb = "calling"
		}
		fmt.Printf("  [%6s] %-8s #%d  %-18s %s %s  (ladder +%s)\n",
			short(elapsed), c.Level, c.Ordinal, c.Recipient.Name, verb, c.Recipient.Phone, short(realOffset))
	}
	return placed, nil
}

// compress maps a real ladder offset onto the harness clock.
func compress(realOffset, perMinute time.Duration) time.Duration {
	return time.Duration(float64(realOffset) / float64(time.Minute) * float64(perMinute))
}

// expand is compress's inverse: it turns elapsed harness time back into the
// ladder time it stands for, so the execution summary carries the timestamps
// a real run would have produced.
func expand(harnessElapsed, perMinute time.Duration) time.Duration {
	return time.Duration(float64(harnessElapsed) / float64(perMinute) * float64(time.Minute))
}

func short(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	return d.Round(time.Second).String()
}

// loadDotEnv reads a .env file and sets any unset environment variables from
// it. Kept deliberately small and separate from cmd/server's copy: this
// command is a test tool and must not drag the server's startup wiring in.
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
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" {
			continue
		}
		if _, set := os.LookupEnv(key); !set {
			_ = os.Setenv(key, value)
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
