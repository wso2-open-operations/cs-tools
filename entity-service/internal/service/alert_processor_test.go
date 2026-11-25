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
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	testAlertFingerprint    = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	testAlertDefaultService = "00000000-0000-4000-8000-000000000001"
	testAlertCaller         = "00000000-0000-4000-8000-000000000002"
	testOpenIncident        = "00000000-0000-4000-8000-0000000000aa"
)

var testAlertReceivedOn = time.Date(2026, 11, 25, 8, 0, 0, 123_000_000, time.UTC)

// fakeProcessorRepo records what the processor asks of the database.
type fakeProcessorRepo struct {
	mu sync.Mutex
	// openByPrefix is the open incident each tag prefix finds.
	openByPrefix map[string]string
	// byCorrelation is the incident each exact correlation id finds.
	byCorrelation map[string]string
	services      map[string]string
	serviceReads  int
	lockedFP      []string
	lastPrefixAge time.Duration
	outcomes      map[string]domain.AlertProcessorOutcome
	failures      []recordedFailure
	claimLost     bool
}

type recordedFailure struct {
	alertID, lastError string
	next               time.Time
	failed             bool
}

func newFakeProcessorRepo() *fakeProcessorRepo {
	return &fakeProcessorRepo{
		openByPrefix:  map[string]string{},
		byCorrelation: map[string]string{},
		services:      map[string]string{},
		outcomes:      map[string]domain.AlertProcessorOutcome{},
	}
}

func (r *fakeProcessorRepo) ClaimAlerts(context.Context, string, time.Duration, int) ([]domain.ClaimedAlert, error) {
	return nil, nil
}

func (r *fakeProcessorRepo) WithFingerprintLock(_ context.Context, fingerprint string, fn func(repository.AlertFingerprintTx) error) error {
	r.mu.Lock()
	r.lockedFP = append(r.lockedFP, fingerprint)
	r.mu.Unlock()
	return fn(fakeFingerprintTx{r})
}

func (r *fakeProcessorRepo) RecordFailure(_ context.Context, alertID, _, lastError string, next time.Time, failed bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failures = append(r.failures, recordedFailure{alertID, lastError, next, failed})
	return nil
}

func (r *fakeProcessorRepo) ServiceIDByName(_ context.Context, name string) (string, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.serviceReads++
	id, ok := r.services[strings.ToLower(name)]
	return id, ok, nil
}

type fakeFingerprintTx struct{ r *fakeProcessorRepo }

func (t fakeFingerprintTx) OpenIncidentByTagPrefix(_ context.Context, prefix string, maxAge time.Duration) (string, bool, error) {
	t.r.lastPrefixAge = maxAge
	id, ok := t.r.openByPrefix[prefix]
	return id, ok, nil
}

func (t fakeFingerprintTx) IncidentByCorrelationID(_ context.Context, correlationID string) (string, bool, error) {
	id, ok := t.r.byCorrelation[correlationID]
	return id, ok, nil
}

func (t fakeFingerprintTx) RecordOutcome(_ context.Context, alertID, _ string, o domain.AlertProcessorOutcome) error {
	if t.r.claimLost {
		return repository.ErrAlertClaimLost
	}
	t.r.outcomes[alertID] = o
	return nil
}

// fakeIncidentCreator records every create and answers with a fixed id, or err.
type fakeIncidentCreator struct {
	requests []domain.CreateIncidentRequest
	err      error
}

func (f *fakeIncidentCreator) CreateIncident(_ context.Context, req domain.CreateIncidentRequest) (domain.CreateIncidentResponse, error) {
	f.requests = append(f.requests, req)
	if f.err != nil {
		return domain.CreateIncidentResponse{}, f.err
	}
	var resp domain.CreateIncidentResponse
	resp.Incident.ID = "00000000-0000-4000-8000-0000000000bb"
	resp.Incident.Number = "INC0000001"
	return resp, nil
}

type fakeMuteRules struct {
	rules []domain.AlertMuteRule
	err   error
	reads int
}

func (f *fakeMuteRules) Active(context.Context, time.Time) ([]domain.AlertMuteRule, error) {
	f.reads++
	return f.rules, f.err
}

func newTestProcessor(live bool, repo *fakeProcessorRepo, creator *fakeIncidentCreator, rules *fakeMuteRules) *AlertProcessor {
	p := NewAlertProcessor(repo, creator, rules, AlertProcessorConfig{
		Live: live, CallerID: testAlertCaller, DefaultServiceID: testAlertDefaultService, Concurrency: 1, MaxIncidentAge: 24 * time.Hour,
	})
	p.now = func() time.Time { return testAlertReceivedOn.Add(time.Minute) }
	return p
}

func claimedAlert(t *testing.T, id string, a model.Alert) domain.ClaimedAlert {
	t.Helper()
	body, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	return domain.ClaimedAlert{ID: id, Source: "azure", Alert: body, Fingerprint: testAlertFingerprint, Tier: domain.AlertTierHigh, ReceivedOn: testAlertReceivedOn}
}

var firingAlert = model.Alert{Service: "", MetricName: "High CPU", Severity: "major", Category: "", Environment: "prod", Source: "Azure", UniqueIdentifier: "vm-1", Description: "cpu <b>95%</b>"}

func TestAlertProcessor_RaisesAnIncidentAsAlertCoreDoes(t *testing.T) {
	repo, creator := newFakeProcessorRepo(), &fakeIncidentCreator{}
	p := newTestProcessor(true, repo, creator, &fakeMuteRules{})

	p.handle(context.Background(), claimedAlert(t, "ALT000000001", firingAlert))

	if len(creator.requests) != 1 {
		t.Fatalf("creates = %d, want 1", len(creator.requests))
	}
	req := creator.requests[0]
	wantTag := "[fp:0123456789ab:" + "1795593600123" + "]"
	if req.CallerID != testAlertCaller || req.ServiceID != testAlertDefaultService || req.Category != "SERVICE_INTERRUPTION" ||
		req.Impact != "MEDIUM" || req.Urgency != "HIGH" || req.Subject != "High CPU" || req.AssignmentGroupID != nil {
		t.Fatalf("request = %+v, want alert-core's mapping with the default service and no assignment group", req)
	}
	if req.CorrelationID == nil || *req.CorrelationID != wantTag {
		t.Fatalf("correlation id = %v, want %s", req.CorrelationID, wantTag)
	}
	if req.ContactType == nil || *req.ContactType != "AZURE" {
		t.Fatalf("contact type = %v, want AZURE", req.ContactType)
	}
	if req.WorkNotes == nil || !strings.Contains(*req.WorkNotes, "Incident auto-created from Alert: ALT000000001") || !strings.Contains(*req.WorkNotes, "cpu &lt;b&gt;95%&lt;/b&gt;") {
		t.Fatalf("work note = %v, want the creation note with the alert escaped", req.WorkNotes)
	}
	if o := repo.outcomes["ALT000000001"]; o.Status != domain.AlertStatusProcessed || o.IncidentID != "00000000-0000-4000-8000-0000000000bb" {
		t.Fatalf("outcome = %+v, want PROCESSED linked to the new incident", o)
	}
	if len(repo.lockedFP) != 1 || repo.lockedFP[0] != testAlertFingerprint {
		t.Fatalf("locked = %v, want the alert's fingerprint", repo.lockedFP)
	}
	if repo.lastPrefixAge != 24*time.Hour {
		t.Fatalf("max incident age = %s, want 24h passed through", repo.lastPrefixAge)
	}
}

func TestAlertProcessor_JoinsTheOpenIncidentOfItsFingerprint(t *testing.T) {
	repo, creator := newFakeProcessorRepo(), &fakeIncidentCreator{}
	repo.openByPrefix["[fp:0123456789ab:"] = testOpenIncident
	p := newTestProcessor(true, repo, creator, &fakeMuteRules{})

	p.handle(context.Background(), claimedAlert(t, "ALT000000002", firingAlert))

	if len(creator.requests) != 0 {
		t.Fatal("an alert with an open incident raised another one")
	}
	if o := repo.outcomes["ALT000000002"]; o.Status != domain.AlertStatusProcessed || o.IncidentID != testOpenIncident {
		t.Fatalf("outcome = %+v, want PROCESSED linked to the open incident", o)
	}
}

func TestAlertProcessor_FindsAnIncidentAnEarlierAttemptRaised(t *testing.T) {
	repo, creator := newFakeProcessorRepo(), &fakeIncidentCreator{}
	repo.byCorrelation["[fp:0123456789ab:1795593600123]"] = testOpenIncident
	p := newTestProcessor(true, repo, creator, &fakeMuteRules{})

	p.handle(context.Background(), claimedAlert(t, "ALT000000003", firingAlert))

	if len(creator.requests) != 0 {
		t.Fatal("a retried alert raised its incident a second time")
	}
	if o := repo.outcomes["ALT000000003"]; o.IncidentID != testOpenIncident {
		t.Fatalf("outcome = %+v, want the incident the earlier attempt raised", o)
	}
}

func TestAlertProcessor_ResolvingAlertWithNoOpenIncidentRaisesNothing(t *testing.T) {
	for _, severity := range []string{"ok", "Clear"} {
		repo, creator := newFakeProcessorRepo(), &fakeIncidentCreator{}
		p := newTestProcessor(true, repo, creator, &fakeMuteRules{})
		a := firingAlert
		a.Severity = severity

		p.handle(context.Background(), claimedAlert(t, "ALT000000004", a))

		if len(creator.requests) != 0 {
			t.Fatalf("severity %s raised an incident", severity)
		}
		if o := repo.outcomes["ALT000000004"]; o.Status != domain.AlertStatusProcessed || o.IncidentID != "" {
			t.Fatalf("severity %s: outcome = %+v, want PROCESSED with no incident", severity, o)
		}
	}
}

func TestAlertProcessor_ResolvingAlertJoinsTheOpenIncident(t *testing.T) {
	repo, creator := newFakeProcessorRepo(), &fakeIncidentCreator{}
	repo.openByPrefix["[fp:0123456789ab:"] = testOpenIncident
	p := newTestProcessor(true, repo, creator, &fakeMuteRules{})
	a := firingAlert
	a.Severity = "ok"

	p.handle(context.Background(), claimedAlert(t, "ALT000000005", a))

	if o := repo.outcomes["ALT000000005"]; o.IncidentID != testOpenIncident {
		t.Fatalf("outcome = %+v, want the resolving alert linked to its open incident", o)
	}
}

func activeRule(id string, conditions ...domain.AlertMuteRuleCondition) domain.AlertMuteRule {
	return domain.AlertMuteRule{ID: id, StartOn: testAlertReceivedOn.Add(-time.Hour), EndOn: testAlertReceivedOn.Add(time.Hour), Conditions: conditions}
}

func TestAlertProcessor_MuteRuleHoldsTheAlertBack(t *testing.T) {
	repo, creator := newFakeProcessorRepo(), &fakeIncidentCreator{}
	rules := &fakeMuteRules{rules: []domain.AlertMuteRule{
		activeRule("00000000-0000-4000-8000-0000000000c1", domain.AlertMuteRuleCondition{Field: "service", Value: "other"}),
		activeRule("00000000-0000-4000-8000-0000000000c2",
			domain.AlertMuteRuleCondition{Field: "environment", Value: "Production"},
			domain.AlertMuteRuleCondition{Field: "metric_name", Value: " high cpu "}),
	}}
	p := newTestProcessor(true, repo, creator, rules)

	p.handle(context.Background(), claimedAlert(t, "ALT000000006", firingAlert))

	if len(creator.requests) != 0 {
		t.Fatal("a muted alert raised an incident")
	}
	if o := repo.outcomes["ALT000000006"]; o.Status != domain.AlertStatusMuted || o.MuteRuleID != "00000000-0000-4000-8000-0000000000c2" {
		t.Fatalf("outcome = %+v, want MUTED by the matching rule", o)
	}
}

func TestAlertProcessor_MuteRuleReadFailureFailsOpenAfterTheStaleLimit(t *testing.T) {
	repo, creator := newFakeProcessorRepo(), &fakeIncidentCreator{}
	rules := &fakeMuteRules{rules: []domain.AlertMuteRule{activeRule("00000000-0000-4000-8000-0000000000c3", domain.AlertMuteRuleCondition{Field: "source", Value: "azure"})}}
	p := newTestProcessor(true, repo, creator, rules)
	now := testAlertReceivedOn
	p.now = func() time.Time { return now }

	p.handle(context.Background(), claimedAlert(t, "ALT000000007", firingAlert))
	if o := repo.outcomes["ALT000000007"]; o.Status != domain.AlertStatusMuted {
		t.Fatalf("outcome = %+v, want MUTED while the rules read", o)
	}

	rules.err = errors.New("db down")
	now = now.Add(time.Minute)
	p.handle(context.Background(), claimedAlert(t, "ALT000000008", firingAlert))
	if o := repo.outcomes["ALT000000008"]; o.Status != domain.AlertStatusMuted {
		t.Fatalf("outcome = %+v, want the last good rules used while they are recent", o)
	}

	now = now.Add(alertMuteRulesMaxStale)
	p.handle(context.Background(), claimedAlert(t, "ALT000000009", firingAlert))
	if o := repo.outcomes["ALT000000009"]; o.Status != domain.AlertStatusProcessed || len(creator.requests) != 1 {
		t.Fatalf("outcome = %+v with %d creates, want muting off and the incident raised", o, len(creator.requests))
	}
}

func TestAlertProcessor_HybridRecordsTheDecisionAndRaisesNothing(t *testing.T) {
	cases := map[string]struct {
		open     bool
		severity string
		rule     bool
		want     string
		incident string
	}{
		"would create":  {want: domain.AlertShadowWouldCreate},
		"would attach":  {open: true, want: domain.AlertShadowWouldAttach, incident: testOpenIncident},
		"would mute":    {rule: true, want: domain.AlertShadowWouldMute},
		"would ignore":  {severity: "ok", want: domain.AlertShadowWouldIgnore},
		"attach wins":   {open: true, rule: true, want: domain.AlertShadowWouldAttach, incident: testOpenIncident},
		"ok and attach": {open: true, severity: "ok", want: domain.AlertShadowWouldAttach, incident: testOpenIncident},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			repo, creator := newFakeProcessorRepo(), &fakeIncidentCreator{}
			if tc.open {
				repo.openByPrefix["[fp:0123456789ab:"] = testOpenIncident
			}
			rules := &fakeMuteRules{}
			if tc.rule {
				rules.rules = []domain.AlertMuteRule{activeRule("00000000-0000-4000-8000-0000000000c4", domain.AlertMuteRuleCondition{Field: "source", Value: "azure"})}
			}
			p := newTestProcessor(false, repo, creator, rules)
			a := firingAlert
			if tc.severity != "" {
				a.Severity = tc.severity
			}

			p.handle(context.Background(), claimedAlert(t, "ALT000000010", a))

			if len(creator.requests) != 0 {
				t.Fatal("hybrid raised an incident")
			}
			o := repo.outcomes["ALT000000010"]
			if o.Status != domain.AlertStatusShadowed || o.ShadowDecision != tc.want || o.ShadowIncidentID != tc.incident || o.IncidentID != "" {
				t.Fatalf("outcome = %+v, want SHADOWED %s (incident %q) and no link", o, tc.want, tc.incident)
			}
		})
	}
}

func TestAlertProcessor_ResolvesTheServiceByName(t *testing.T) {
	repo, creator := newFakeProcessorRepo(), &fakeIncidentCreator{}
	repo.services["checkout"] = "00000000-0000-4000-8000-0000000000d1"
	p := newTestProcessor(true, repo, creator, &fakeMuteRules{})

	for i, service := range []string{"Checkout", "checkout", "unknown-svc"} {
		a := firingAlert
		a.Service = service
		a.UniqueIdentifier = service
		p.handle(context.Background(), claimedAlert(t, "ALT00000002"+string(rune('0'+i)), a))
	}

	if got := []string{creator.requests[0].ServiceID, creator.requests[1].ServiceID, creator.requests[2].ServiceID}; got[0] != "00000000-0000-4000-8000-0000000000d1" ||
		got[1] != got[0] || got[2] != testAlertDefaultService {
		t.Fatalf("services = %v, want checkout twice (any case) and the default service for an unknown name", got)
	}
	if repo.serviceReads != 2 {
		t.Fatalf("service reads = %d, want 2: the second checkout comes from the cache", repo.serviceReads)
	}
}

func TestAlertProcessor_RetriesAFailedCreateThenGivesUp(t *testing.T) {
	repo := newFakeProcessorRepo()
	creator := &fakeIncidentCreator{err: errors.New("connection reset")}
	p := newTestProcessor(true, repo, creator, &fakeMuteRules{})

	a := claimedAlert(t, "ALT000000030", firingAlert)
	p.handle(context.Background(), a)
	a.Attempts = alertMaxAttempts - 1
	p.handle(context.Background(), a)

	if len(repo.failures) != 2 {
		t.Fatalf("failures = %d, want 2", len(repo.failures))
	}
	if f := repo.failures[0]; f.failed || !f.next.Equal(p.now().Add(alertRetryBase)) || !strings.Contains(f.lastError, "connection reset") {
		t.Fatalf("first failure = %+v, want a retry after %s", f, alertRetryBase)
	}
	if f := repo.failures[1]; !f.failed {
		t.Fatalf("failure on attempt %d = %+v, want FAILED", alertMaxAttempts, f)
	}
	if _, ok := repo.outcomes["ALT000000030"]; ok {
		t.Fatal("a failed create recorded an outcome")
	}
}

func TestAlertProcessor_RefusedCreateFailsAtOnce(t *testing.T) {
	repo := newFakeProcessorRepo()
	creator := &fakeIncidentCreator{err: &apierror.ValidationError{Msg: "invalid serviceId"}}
	p := newTestProcessor(true, repo, creator, &fakeMuteRules{})

	p.handle(context.Background(), claimedAlert(t, "ALT000000031", firingAlert))

	if len(repo.failures) != 1 || !repo.failures[0].failed {
		t.Fatalf("failures = %+v, want one final failure: a refused create never succeeds on retry", repo.failures)
	}
}

func TestAlertProcessor_UnreadableAlertFailsAtOnce(t *testing.T) {
	repo := newFakeProcessorRepo()
	p := newTestProcessor(true, repo, &fakeIncidentCreator{}, &fakeMuteRules{})

	p.handle(context.Background(), domain.ClaimedAlert{ID: "ALT000000032", Alert: json.RawMessage(`not json`), Fingerprint: testAlertFingerprint})

	if len(repo.failures) != 1 || !repo.failures[0].failed {
		t.Fatalf("failures = %+v, want one final failure", repo.failures)
	}
	if len(repo.lockedFP) != 0 {
		t.Fatal("an unreadable alert took the fingerprint lock")
	}
}

func TestAlertProcessor_LostClaimRecordsNothing(t *testing.T) {
	repo := newFakeProcessorRepo()
	repo.claimLost = true
	p := newTestProcessor(true, repo, &fakeIncidentCreator{}, &fakeMuteRules{})

	p.handle(context.Background(), claimedAlert(t, "ALT000000033", firingAlert))

	if len(repo.failures) != 0 {
		t.Fatalf("failures = %+v, want none: the worker that holds the claim now decides", repo.failures)
	}
}

func TestAlertRetryDelay(t *testing.T) {
	for attempts, want := range map[int]time.Duration{1: 5 * time.Second, 2: 10 * time.Second, 4: 40 * time.Second, 7: 5 * time.Minute, 20: 5 * time.Minute} {
		if got := alertRetryDelay(attempts); got != want {
			t.Errorf("alertRetryDelay(%d) = %s, want %s", attempts, got, want)
		}
	}
}

func TestAlertIncidentRules(t *testing.T) {
	if got := alertCorrelationTag(testAlertFingerprint, time.UnixMilli(1700000000000)); got != "[fp:0123456789ab:1700000000000]" {
		t.Errorf("tag = %s, want alert-core's [fp:<12>:<ms>] format", got)
	}
	for label, want := range map[string]int{"Critical": 1, " major ": 2, "minor": 3, "warning": 4, "OK": 5, "clear": 0, "1": 1, "": 1} {
		if got, _ := alertSeverity(label); got != want {
			t.Errorf("alertSeverity(%q) = %d, want %d", label, got, want)
		}
	}
	for level, want := range map[int][2]string{1: {"HIGH", "HIGH"}, 2: {"MEDIUM", "HIGH"}, 3: {"MEDIUM", "MEDIUM"}, 4: {"MEDIUM", "LOW"}, 5: {"LOW", "LOW"}} {
		if i, u := alertImpactUrgency(level); string(i) != want[0] || string(u) != want[1] {
			t.Errorf("alertImpactUrgency(%d) = %s/%s, want %s/%s", level, i, u, want[0], want[1])
		}
	}
	for category, want := range map[string]string{"Security": "SECURITY", "inquiry": "INQUIRY", "availability": "SERVICE_INTERRUPTION", "": "SERVICE_INTERRUPTION"} {
		if got := alertIncidentCategory(category); string(got) != want {
			t.Errorf("alertIncidentCategory(%q) = %s, want %s", category, got, want)
		}
	}
	for source, want := range map[string]string{"Azure Monitor": "AZURE", "Site 24x7": "SITE_247", "Microsoft Sentinel": "SENTINEL", "AWS": ""} {
		got := alertContactType(source)
		if (got == nil && want != "") || (got != nil && string(*got) != want) {
			t.Errorf("alertContactType(%q) = %v, want %q", source, got, want)
		}
	}
	if got := alertIncidentSubject(model.Alert{Service: "checkout"}); got != "checkout" {
		t.Errorf("subject with no metric name = %q, want the service", got)
	}
}

func TestAlertMuteRuleMatches(t *testing.T) {
	a := model.Alert{Service: "Checkout", Environment: "prd", Source: "Azure", MetricName: "cpu"}
	cases := map[string]struct {
		conditions []domain.AlertMuteRuleCondition
		want       bool
	}{
		"every condition matches":     {[]domain.AlertMuteRuleCondition{{Field: "service", Value: " checkout"}, {Field: "environment", Value: "live"}}, true},
		"one condition differs":       {[]domain.AlertMuteRuleCondition{{Field: "service", Value: "checkout"}, {Field: "environment", Value: "staging"}}, false},
		"no conditions":               {nil, false},
		"unknown field":               {[]domain.AlertMuteRuleCondition{{Field: "region", Value: "eu"}}, false},
		"unknown environment matches": {[]domain.AlertMuteRuleCondition{{Field: "environment", Value: "PRD"}}, true},
	}
	for name, tc := range cases {
		if got := alertMuteRuleMatches(domain.AlertMuteRule{Conditions: tc.conditions}, a); got != tc.want {
			t.Errorf("%s: matches = %v, want %v", name, got, tc.want)
		}
	}
	ended := activeRule("r", domain.AlertMuteRuleCondition{Field: "service", Value: "checkout"})
	endedOn := testAlertReceivedOn
	ended.EndedOn = &endedOn
	if _, ok := matchAlertMuteRule([]domain.AlertMuteRule{ended}, a, testAlertReceivedOn); ok {
		t.Error("an ended rule matched")
	}
}
