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

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// AlertIncidentCreator creates incidents; IncidentService satisfies it.
type AlertIncidentCreator interface {
	CreateIncident(ctx context.Context, req domain.CreateIncidentRequest) (domain.CreateIncidentResponse, error)
}

// AlertMuteRuleSource lists the alert mute rules active at now; repository.AlertMuteRuleRepository satisfies it.
type AlertMuteRuleSource interface {
	Active(ctx context.Context, now time.Time) ([]domain.AlertMuteRule, error)
}

// AlertProcessorConfig tunes the alert processor.
type AlertProcessorConfig struct {
	// Live raises incidents (ALERTING_VERSION v2). Off (hybrid), every alert is SHADOWED with what would have been done.
	Live bool
	// CallerID is the user incidents are raised by; required when Live.
	CallerID string
	// DefaultServiceID is the service an incident gets when its alert names no service, or one CSM does not have.
	DefaultServiceID string
	// Concurrency is how many alerts are worked on at once.
	Concurrency int
	// PollInterval is the backstop: how long an idle worker waits before looking again when no wake-up came (a missed
	// notification, a retry falling due, another replica's expired lease).
	PollInterval time.Duration
	// MaxIncidentAge is how old an open incident may be and still take new alerts of its fingerprint; 0 is no limit.
	MaxIncidentAge time.Duration
	// Owner names this replica on the claims it takes.
	Owner string
}

// alertProcessorActor is the created_by of the incidents the processor raises.
const alertProcessorActor = "alert-processor"

// alertClaimLease is how long a claimed alert is this replica's; an alert whose replica died is claimed again after it.
const alertClaimLease = 2 * time.Minute

// alertProcessTimeout bounds one alert's decision and incident create, well inside the lease.
const alertProcessTimeout = 60 * time.Second

// alertClaimBatch is how many alerts one worker claims at a time. One, so every claim takes the most urgent waiting
// alert: a priority alert that arrives while workers are busy is the next one any of them takes.
const alertClaimBatch = 1

// alertMaxAttempts is how many failed attempts make an alert FAILED.
const alertMaxAttempts = 8

// Failed attempts are retried after alertRetryBase, doubling up to alertRetryMax.
const (
	alertRetryBase = 5 * time.Second
	alertRetryMax  = 5 * time.Minute
)

// alertMuteRulesMaxAge is how long a read of the active mute rules is reused.
const alertMuteRulesMaxAge = 15 * time.Second

// alertMuteRulesMaxStale is how long the last good set of mute rules is still used while reads fail.
const alertMuteRulesMaxStale = 5 * time.Minute

// alertServiceCacheTTL is how long a service name's id is reused.
const alertServiceCacheTTL = 10 * time.Minute

// errAlertUnreadable marks a stored alert the processor can never decide on, so it fails at once instead of being retried.
var errAlertUnreadable = errors.New("the stored alert cannot be read")

// AlertProcessor turns the alerts the queue consumer stored into incidents: it joins an alert to the open incident of
// its fingerprint, or raises one, and records the decision on the alert row.
type AlertProcessor struct {
	repo      repository.AlertProcessorRepository
	incidents AlertIncidentCreator
	muteRules AlertMuteRuleSource
	cfg       AlertProcessorConfig
	now       func() time.Time
	sleep     func(ctx context.Context, d time.Duration) bool
	wake      alertWaker

	rulesMu     sync.Mutex
	rules       []domain.AlertMuteRule
	rulesReadOn time.Time

	servicesMu sync.Mutex
	services   map[string]cachedServiceID
}

type cachedServiceID struct {
	id      string
	readOn  time.Time
	missing bool
}

// NewAlertProcessor returns an AlertProcessor; Run starts it.
func NewAlertProcessor(repo repository.AlertProcessorRepository, incidents AlertIncidentCreator, muteRules AlertMuteRuleSource, cfg AlertProcessorConfig) *AlertProcessor {
	cfg.Concurrency = max(cfg.Concurrency, 1)
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 5 * time.Second
	}
	if cfg.Owner == "" {
		cfg.Owner = alertProcessorActor
	}
	return &AlertProcessor{
		repo:      repo,
		incidents: incidents,
		muteRules: muteRules,
		cfg:       cfg,
		now:       time.Now,
		sleep:     sleepContext,
		wake:      alertWaker{ch: make(chan struct{})},
		services:  map[string]cachedServiceID{},
	}
}

// Wake starts every idle worker at once; the alert_stored listener calls it when the consumer has committed a NEW
// alert. Safe to call from any goroutine, any number of times.
func (p *AlertProcessor) Wake() {
	p.wake.broadcast()
}

// alertWaker wakes every waiting worker together: broadcast closes the channel the workers wait on and starts a new one.
type alertWaker struct {
	mu sync.Mutex
	ch chan struct{}
}

func (w *alertWaker) wait() <-chan struct{} {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.ch
}

func (w *alertWaker) broadcast() {
	w.mu.Lock()
	defer w.mu.Unlock()
	close(w.ch)
	w.ch = make(chan struct{})
}

// idle waits for a wake-up or PollInterval, whichever comes first; false when ctx is done.
func (p *AlertProcessor) idle(ctx context.Context, woken <-chan struct{}) bool {
	select {
	case <-woken:
		return ctx.Err() == nil
	default:
	}
	waitCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-woken:
			cancel()
		case <-waitCtx.Done():
		}
	}()
	p.sleep(waitCtx, p.cfg.PollInterval)
	return ctx.Err() == nil
}

// Run works on waiting alerts until ctx is done; an alert in flight is finished first.
func (p *AlertProcessor) Run(ctx context.Context) {
	var workers sync.WaitGroup
	for range p.cfg.Concurrency {
		workers.Go(func() { p.work(ctx) })
	}
	workers.Wait()
}

// work claims the most urgent waiting alert and decides it, again and again. With nothing waiting it idles until a
// wake-up (a NEW alert committed) or PollInterval. The wake-up channel is taken before the claim, so an alert committed
// between an empty claim and the wait still wakes it.
func (p *AlertProcessor) work(ctx context.Context) {
	for ctx.Err() == nil {
		woken := p.wake.wait()
		claimed, err := p.repo.ClaimAlerts(ctx, p.cfg.Owner, alertClaimLease, alertClaimBatch)
		if err != nil {
			if ctx.Err() == nil {
				slog.ErrorContext(ctx, "alert processor: claiming alerts failed, retrying", "err", err)
			}
			// Not idle: a wake-up must not turn a failing database into a busy loop.
			if !p.sleep(ctx, p.cfg.PollInterval) {
				return
			}
			continue
		}
		if len(claimed) == 0 {
			if !p.idle(ctx, woken) {
				return
			}
			continue
		}
		for _, alert := range claimed {
			// A claimed alert is finished even during shutdown; the rest of the batch goes back when its lease ends.
			if ctx.Err() != nil {
				return
			}
			p.handle(ctx, alert)
		}
	}
}

// handle decides one alert and records the outcome, or counts a failed attempt.
func (p *AlertProcessor) handle(ctx context.Context, claimed domain.ClaimedAlert) {
	processCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), alertProcessTimeout)
	defer cancel()
	outcome, err := p.process(processCtx, claimed)
	if err == nil {
		slog.InfoContext(ctx, "alert processor: alert decided", "alertId", claimed.ID, "tier", claimed.Tier, "status", outcome.Status,
			"incidentId", outcome.IncidentID, "shadowDecision", outcome.ShadowDecision, "muteRuleId", outcome.MuteRuleID)
		return
	}
	if errors.Is(err, repository.ErrAlertClaimLost) {
		slog.WarnContext(ctx, "alert processor: claim expired before the decision was recorded, another worker has the alert", "alertId", claimed.ID)
		return
	}
	attempts := claimed.Attempts + 1
	failed := attempts >= alertMaxAttempts || isPermanentAlertError(err)
	next := p.now().Add(alertRetryDelay(attempts))
	recordCtx, cancelRecord := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancelRecord()
	if recordErr := p.repo.RecordFailure(recordCtx, claimed.ID, p.cfg.Owner, err.Error(), next, failed); recordErr != nil {
		slog.ErrorContext(ctx, "alert processor: could not record a failed attempt; the alert is retried when its claim expires",
			"alertId", claimed.ID, "err", err, "recordErr", recordErr)
		return
	}
	if failed {
		// FAILED means no incident was raised for this alert; log at ERROR so log-based alerting pages someone.
		slog.ErrorContext(ctx, "alert processor: alert FAILED, no incident was raised for it", "alertId", claimed.ID, "attempts", attempts, "err", err)
		return
	}
	slog.WarnContext(ctx, "alert processor: attempt failed, will retry", "alertId", claimed.ID, "attempts", attempts, "retryAt", next, "err", err)
}

// alertRetryDelay grows alertRetryBase by doubling per attempt, up to alertRetryMax.
func alertRetryDelay(attempts int) time.Duration {
	d := alertRetryBase
	for i := 1; i < attempts && d < alertRetryMax; i++ {
		d *= 2
	}
	return min(d, alertRetryMax)
}

// isPermanentAlertError reports an error no retry can fix: an unreadable alert, or a create CSM refuses as invalid.
func isPermanentAlertError(err error) bool {
	var validation *apierror.ValidationError
	return errors.Is(err, errAlertUnreadable) || errors.As(err, &validation)
}

// process decides one alert under its fingerprint's lock. The decision, in order:
//  1. an open incident of the fingerprint (by correlation tag, so alert-core's open incidents count too) takes the alert;
//  2. a resolving alert (OK, Clear) with no open incident raises nothing;
//  3. an active alert mute rule that matches holds the alert back (MUTED);
//  4. otherwise an incident is raised; a create that already reached CSM on an earlier attempt is found by its tag.
//
// In hybrid (not Live) the same decision is recorded as SHADOWED with shadow_decision, and nothing is raised.
func (p *AlertProcessor) process(ctx context.Context, claimed domain.ClaimedAlert) (domain.AlertProcessorOutcome, error) {
	var alert model.Alert
	if len(claimed.Alert) == 0 || claimed.Fingerprint == "" {
		return domain.AlertProcessorOutcome{}, fmt.Errorf("%w: no normalised alert or fingerprint", errAlertUnreadable)
	}
	if err := json.Unmarshal(claimed.Alert, &alert); err != nil {
		return domain.AlertProcessorOutcome{}, fmt.Errorf("%w: %v", errAlertUnreadable, err)
	}
	level, recognized := alertSeverity(alert.Severity)
	if !recognized {
		slog.WarnContext(ctx, "alert processor: unrecognised severity label, treating it as critical", "alertId", claimed.ID, "severity", alert.Severity)
	}

	var outcome domain.AlertProcessorOutcome
	err := p.repo.WithFingerprintLock(ctx, claimed.Fingerprint, func(tx repository.AlertFingerprintTx) error {
		openID, open, err := tx.OpenIncidentByTagPrefix(ctx, alertTagPrefix(claimed.Fingerprint), p.cfg.MaxIncidentAge)
		if err != nil {
			return err
		}
		outcome, err = p.decide(ctx, tx, claimed, alert, level, openID, open)
		if err != nil {
			return err
		}
		return tx.RecordOutcome(ctx, claimed.ID, p.cfg.Owner, outcome)
	})
	return outcome, err
}

// decide picks the outcome for one alert, raising the incident when Live calls for one.
func (p *AlertProcessor) decide(ctx context.Context, tx repository.AlertFingerprintTx, claimed domain.ClaimedAlert, alert model.Alert, level int, openID string, open bool) (domain.AlertProcessorOutcome, error) {
	switch {
	case open:
		if !p.cfg.Live {
			return shadowOutcome(domain.AlertShadowWouldAttach, openID), nil
		}
		return domain.AlertProcessorOutcome{Status: domain.AlertStatusProcessed, IncidentID: openID}, nil
	case alertIsResolving(level):
		if !p.cfg.Live {
			return shadowOutcome(domain.AlertShadowWouldIgnore, ""), nil
		}
		return domain.AlertProcessorOutcome{Status: domain.AlertStatusProcessed}, nil
	}

	if rule, muted := matchAlertMuteRule(p.activeMuteRules(ctx), alert, p.now()); muted {
		if !p.cfg.Live {
			o := shadowOutcome(domain.AlertShadowWouldMute, "")
			o.MuteRuleID = rule.ID
			return o, nil
		}
		return domain.AlertProcessorOutcome{Status: domain.AlertStatusMuted, MuteRuleID: rule.ID}, nil
	}
	if !p.cfg.Live {
		return shadowOutcome(domain.AlertShadowWouldCreate, ""), nil
	}

	tag := alertCorrelationTag(claimed.Fingerprint, claimed.ReceivedOn)
	// A create whose result was lost (a crash, a timeout after CSM committed) is found here instead of raised twice.
	if id, found, err := tx.IncidentByCorrelationID(ctx, tag); err != nil {
		return domain.AlertProcessorOutcome{}, err
	} else if found {
		return domain.AlertProcessorOutcome{Status: domain.AlertStatusProcessed, IncidentID: id}, nil
	}
	id, err := p.createIncident(ctx, claimed, alert, level, tag)
	if err != nil {
		return domain.AlertProcessorOutcome{}, err
	}
	return domain.AlertProcessorOutcome{Status: domain.AlertStatusProcessed, IncidentID: id}, nil
}

func shadowOutcome(decision, incidentID string) domain.AlertProcessorOutcome {
	return domain.AlertProcessorOutcome{Status: domain.AlertStatusShadowed, ShadowDecision: decision, ShadowIncidentID: incidentID}
}

// createIncident raises the incident for an alert as alert-core does: no assignment group (the service's support group
// is used), the creation note as its first work note, and the correlation tag.
func (p *AlertProcessor) createIncident(ctx context.Context, claimed domain.ClaimedAlert, alert model.Alert, level int, tag string) (string, error) {
	serviceID, err := p.serviceID(ctx, alert.Service)
	if err != nil {
		return "", err
	}
	impact, urgency := alertImpactUrgency(level)
	note := alertCreationNote(claimed.ID, alert)
	req := domain.CreateIncidentRequest{
		CallerID:      p.cfg.CallerID,
		Category:      alertIncidentCategory(alert.Category),
		ServiceID:     serviceID,
		Impact:        impact,
		Urgency:       urgency,
		Subject:       alertIncidentSubject(alert),
		WorkNotes:     &note,
		ContactType:   alertContactType(alert.Source),
		CorrelationID: &tag,
	}
	createCtx := auth.WithIdentity(ctx, auth.Identity{ClientID: alertProcessorActor})
	resp, err := p.incidents.CreateIncident(createCtx, req)
	if err != nil {
		return "", fmt.Errorf("create incident: %w", err)
	}
	if resp.Incident.ID == "" {
		return "", errors.New("create incident: no incident id in the response")
	}
	slog.InfoContext(ctx, "alert processor: incident raised", "alertId", claimed.ID, "incidentId", resp.Incident.ID,
		"incidentNumber", resp.Incident.Number, "tier", claimed.Tier, "correlationId", tag)
	return resp.Incident.ID, nil
}

// serviceID resolves the alert's service name to a CSM service, falling back to the default service when the alert
// names none or CSM has no service of that name. A lookup error is returned, never replaced by the default, so the
// alert is retried rather than raised against the wrong service.
func (p *AlertProcessor) serviceID(ctx context.Context, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return p.cfg.DefaultServiceID, nil
	}
	key := strings.ToLower(name)
	p.servicesMu.Lock()
	cached, ok := p.services[key]
	p.servicesMu.Unlock()
	if ok && p.now().Sub(cached.readOn) < alertServiceCacheTTL {
		if cached.missing {
			return p.cfg.DefaultServiceID, nil
		}
		return cached.id, nil
	}
	id, found, err := p.repo.ServiceIDByName(ctx, name)
	if err != nil {
		return "", err
	}
	p.servicesMu.Lock()
	p.services[key] = cachedServiceID{id: id, readOn: p.now(), missing: !found}
	p.servicesMu.Unlock()
	if !found {
		slog.WarnContext(ctx, "alert processor: no service with the alert's service name, using the default service", "service", name)
		return p.cfg.DefaultServiceID, nil
	}
	return id, nil
}

// activeMuteRules returns the active alert mute rules, read at most every alertMuteRulesMaxAge. A failed read keeps the
// last good set until it is alertMuteRulesMaxStale old; after that muting fails open (no rule matches) and the alert pages,
// as alert-core does: a missed page costs more than a muted alert that got through.
func (p *AlertProcessor) activeMuteRules(ctx context.Context) []domain.AlertMuteRule {
	if p.muteRules == nil {
		return nil
	}
	p.rulesMu.Lock()
	defer p.rulesMu.Unlock()
	now := p.now()
	if !p.rulesReadOn.IsZero() && now.Sub(p.rulesReadOn) < alertMuteRulesMaxAge {
		return p.rules
	}
	rules, err := p.muteRules.Active(ctx, now)
	if err == nil {
		p.rules, p.rulesReadOn = rules, now
		return rules
	}
	if !p.rulesReadOn.IsZero() && now.Sub(p.rulesReadOn) < alertMuteRulesMaxStale {
		slog.WarnContext(ctx, "alert processor: reading alert mute rules failed, using the last good set", "age", now.Sub(p.rulesReadOn), "err", err)
		return p.rules
	}
	slog.ErrorContext(ctx, "alert processor: reading alert mute rules failed and no recent set is held; muting is off until a read succeeds", "err", err)
	return nil
}
