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

package mailpaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

const (
	tickBatch = 20
	// maxSpokenSubject keeps the spoken message short; the subject is
	// email-supplied text.
	maxSpokenSubject = 100
)

// Caller places one voice call; *notifications.TwilioClient implements it.
type Caller interface {
	MakeCall(ctx context.Context, to, message string) (notifications.Call, error)
}

// step is one pending level of one alert. It is the sorted-set member, so
// everything needed to place the calls travels with it and no second record
// has to be kept in step with it.
type step struct {
	AlertID string `json:"a"`
	Rule    string `json:"r"`
	Subject string `json:"s"`
	Level   int    `json:"l"`
}

// Engine runs the ladders.
type Engine struct {
	cfg     Config
	store   Store
	caller  Caller
	sending bool
	now     func() time.Time
}

// NewEngine builds an Engine. sending=false (CALL_SENDING_ENABLED) logs each
// call instead of placing it.
func NewEngine(cfg Config, store Store, caller Caller, sending bool) *Engine {
	return &Engine{cfg: cfg, store: store, caller: caller, sending: sending, now: time.Now}
}

// HandleAlert starts the ladder for one mail.alert.received. A rule with no
// ladder is not an error: the mail service may publish rules this service has
// no ladder for.
func (e *Engine) HandleAlert(ctx context.Context, p events.MailAlertReceivedPayload) error {
	if _, ok := e.cfg.Rules[p.RuleName]; !ok {
		slog.InfoContext(ctx, "mailpaging: no ladder configured for this rule; ignoring", "rule", p.RuleName)
		return nil
	}
	started, err := e.store.Started(ctx, p.DedupeKey)
	if err != nil {
		return fmt.Errorf("mailpaging: check started: %w", err)
	}
	if started {
		slog.InfoContext(ctx, "mailpaging: ladder already started for this alert", "rule", p.RuleName)
		return nil
	}
	// Scheduled before it is marked: a failure in between makes a retry
	// schedule the same step again (a no-op while it is pending) rather than
	// leaving an alert marked started with nothing scheduled.
	if err := e.schedule(ctx, step{AlertID: p.DedupeKey, Rule: p.RuleName, Subject: p.Subject}, e.now()); err != nil {
		return err
	}
	if err := e.store.MarkStarted(ctx, p.DedupeKey); err != nil {
		return fmt.Errorf("mailpaging: mark started: %w", err)
	}
	slog.InfoContext(ctx, "mailpaging: ladder started", "rule", p.RuleName)
	return nil
}

func (e *Engine) schedule(ctx context.Context, s step, at time.Time) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if err := e.store.Schedule(ctx, string(b), at); err != nil {
		return fmt.Errorf("mailpaging: schedule level %d: %w", s.Level, err)
	}
	return nil
}

// RunTicker places due levels every interval until ctx ends.
func (e *Engine) RunTicker(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := e.Tick(ctx); err != nil {
				slog.ErrorContext(ctx, "mailpaging: tick failed", "err", err)
			}
		}
	}
}

// Tick places every level that is due.
func (e *Engine) Tick(ctx context.Context) error {
	due, err := e.store.Due(ctx, e.now(), tickBatch)
	if err != nil {
		return fmt.Errorf("mailpaging: list due: %w", err)
	}
	for _, member := range due {
		won, err := e.store.Claim(ctx, member)
		if err != nil {
			slog.ErrorContext(ctx, "mailpaging: claim failed", "err", err)
			continue
		}
		if !won {
			continue
		}
		e.run(ctx, member)
	}
	return nil
}

// run places one level's calls. The next level is scheduled before any call
// is placed, so a crash part-way through this level still lets the ladder go
// on; the cost is that a crash can leave part of this level uncalled, which
// the next level then covers.
func (e *Engine) run(ctx context.Context, member string) {
	var s step
	if err := json.Unmarshal([]byte(member), &s); err != nil {
		slog.ErrorContext(ctx, "mailpaging: dropping an unreadable step", "err", err)
		return
	}
	rule, ok := e.cfg.Rules[s.Rule]
	if !ok || s.Level < 0 || s.Level >= len(rule.Levels) {
		slog.WarnContext(ctx, "mailpaging: step no longer matches the config; dropping it", "rule", s.Rule, "level", s.Level)
		return
	}
	if s.Level+1 < len(rule.Levels) {
		next := s
		next.Level++
		if err := e.schedule(ctx, next, e.now().Add(rule.Wait())); err != nil {
			slog.ErrorContext(ctx, "mailpaging: could not schedule the next level", "rule", s.Rule, "level", rule.Levels[s.Level].Name, "err", err)
		}
	}
	level := rule.Levels[s.Level]
	msg := spokenMessage(s.Rule, level.Name, s.Subject)
	placed, failed := 0, 0
	for _, phone := range level.Phones {
		if !e.sending {
			slog.InfoContext(ctx, "mailpaging: CALL_SENDING_ENABLED=false; call logged, not placed", "rule", s.Rule, "level", level.Name)
			continue
		}
		if _, err := e.caller.MakeCall(ctx, phone, msg); err != nil {
			failed++
			slog.ErrorContext(ctx, "mailpaging: call failed", "rule", s.Rule, "level", level.Name, "err", err)
			continue
		}
		placed++
	}
	slog.InfoContext(ctx, "mailpaging: level called", "rule", s.Rule, "level", level.Name,
		"people", len(level.Phones), "placed", placed, "failed", failed)
}

// spokenMessage is what the call reads aloud. The subject is email-supplied,
// so control characters are dropped and it is cut short; the Twilio client
// builds the TwiML with encoding/xml, so it cannot inject markup.
func spokenMessage(rule, level, subject string) string {
	clean := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, subject)
	clean = strings.Join(strings.Fields(clean), " ")
	if r := []rune(clean); len(r) > maxSpokenSubject {
		clean = string(r[:maxSpokenSubject])
	}
	if clean == "" {
		return fmt.Sprintf("Alert email received for %s. Escalation level %s.", rule, level)
	}
	return fmt.Sprintf("Alert email received for %s. Escalation level %s. Subject: %s.", rule, level, clean)
}
