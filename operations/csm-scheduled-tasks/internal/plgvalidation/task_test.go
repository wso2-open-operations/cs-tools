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

package plgvalidation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/moesif"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/plgactivity"
)

var now = time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)

type fakeMoesif struct {
	hits    []moesif.Hit
	queries []moesif.Query
	err     error
}

func (f *fakeMoesif) SearchEvents(_ context.Context, q moesif.Query) ([]moesif.Hit, bool, error) {
	f.queries = append(f.queries, q)
	return f.hits, false, f.err
}

type fakeEntity struct {
	items   []QueueItem
	applied map[string]ApplyRequest
	fail    map[string]error
}

func (f *fakeEntity) Queue(context.Context, string, time.Time, int) ([]QueueItem, error) {
	return f.items, nil
}

func (f *fakeEntity) Apply(_ context.Context, id string, req ApplyRequest) (ApplyResult, error) {
	if err := f.fail[id]; err != nil {
		return ApplyResult{}, err
	}
	f.applied[id] = req
	return ApplyResult{Written: []string{codeInitiate}, Abandoned: req.Abandon}, nil
}

func hit(company string, at time.Time, action string) moesif.Hit {
	return moesif.Hit{Source: map[string]any{
		"company_id": company, "request.time": at.Format("2006-01-02T15:04:05.000"),
		"metadata": map[string]any{"action": action},
	}}
}

func TestRunWritesEachPairing(t *testing.T) {
	reg := now.Add(-24 * time.Hour)
	disposable := "DISPOSABLE"
	ent := &fakeEntity{
		items: []QueueItem{
			{OrgPlatformID: "p-active", CompanyID: "c1", RegisteredOn: reg},
			{OrgPlatformID: "p-quiet", CompanyID: "c2", RegisteredOn: reg},
			{OrgPlatformID: "p-disposable", CompanyID: "c3", RegisteredOn: reg, EmailClassification: &disposable},
		},
		applied: map[string]ApplyRequest{},
	}
	var hits []moesif.Hit
	for i := range 20 {
		hits = append(hits, hit("c1", reg.Add(time.Duration(i)*time.Minute), "created-agent"))
	}
	hits = append(hits,
		hit("c1", reg.Add(-time.Hour), "created-agent"),       // before signup
		hit("c1", reg.Add(13*time.Hour), "created-agent"),     // after the window
		hit("c2", reg.Add(time.Minute), "registered-gateway"), // platform setup
	)
	src := &fakeMoesif{hits: hits}
	opts := Options{Products: []string{"AGENT_PLATFORM"}, Window: 12 * time.Hour, Grace: 24 * time.Hour}
	if err := Run(src, ent, opts, func() time.Time { return now })(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}

	if len(src.queries) != 1 || !src.queries[0].To.Equal(reg.Add(12*time.Hour)) {
		t.Fatalf("queries = %+v, want one ending with the window", src.queries)
	}
	active := ent.applied["p-active"]
	if got := value(active, codeOutcome); got != OutcomeEligible || active.Abandon {
		t.Errorf("active: outcome %q abandon %v", got, active.Abandon)
	}
	if got := value(active, codeLevel); got != LevelHigh || active.Summary.EventCount != 20 {
		t.Errorf("active: level %q events %d", got, active.Summary.EventCount)
	}
	quiet := ent.applied["p-quiet"]
	if got := value(quiet, codeOutcome); got != OutcomeMonitor || value(quiet, codeLevel) != LevelNone || quiet.Summary.EventCount != 1 {
		t.Errorf("quiet: outcome %q level %q events %d", got, value(quiet, codeLevel), quiet.Summary.EventCount)
	}
	if v := find(quiet, codeSession); v.NumberValue != nil {
		t.Errorf("quiet: session %v, want cleared", *v.NumberValue)
	}
	if d := ent.applied["p-disposable"]; value(d, codeOutcome) != OutcomeExcluded || !d.Abandon {
		t.Errorf("disposable: outcome %q abandon %v", value(d, codeOutcome), d.Abandon)
	}
}

func TestRunDryRunWritesNothing(t *testing.T) {
	ent := &fakeEntity{items: []QueueItem{{OrgPlatformID: "p", CompanyID: "c", RegisteredOn: now}}, applied: map[string]ApplyRequest{}}
	opts := Options{Products: []string{"AGENT_PLATFORM"}, Window: 72 * time.Hour, DryRun: true}
	if err := Run(&fakeMoesif{}, ent, opts, func() time.Time { return now })(context.Background()); err != nil || len(ent.applied) != 0 {
		t.Fatalf("err = %v, applied = %d", err, len(ent.applied))
	}
}

func TestRunFailures(t *testing.T) {
	items := []QueueItem{{OrgPlatformID: "p1", CompanyID: "c", RegisteredOn: now}, {OrgPlatformID: "p2", CompanyID: "c", RegisteredOn: now}}
	opts := Options{Products: []string{"AGENT_PLATFORM"}, Window: 72 * time.Hour}
	clock := func() time.Time { return now }

	ent := &fakeEntity{items: items, applied: map[string]ApplyRequest{}, fail: map[string]error{"p1": &apierror.Error{StatusCode: 500}}}
	if err := Run(&fakeMoesif{}, ent, opts, clock)(context.Background()); err == nil || len(ent.applied) != 1 {
		t.Errorf("one failed write: err = %v, applied = %d; want an error and the other written", err, len(ent.applied))
	}

	noPlaybook := &apierror.Error{StatusCode: 404, Body: `{"message":"no active registration playbook for this product"}`}
	ent = &fakeEntity{items: items, applied: map[string]ApplyRequest{}, fail: map[string]error{"p1": noPlaybook}}
	if err := Run(&fakeMoesif{}, ent, opts, clock)(context.Background()); err != nil || len(ent.applied) != 0 {
		t.Errorf("no playbook: err = %v, applied = %d; want the product skipped quietly", err, len(ent.applied))
	}

	ent = &fakeEntity{items: items, applied: map[string]ApplyRequest{}}
	if err := Run(&fakeMoesif{err: errors.New("down")}, ent, opts, clock)(context.Background()); err == nil || len(ent.applied) != 0 {
		t.Errorf("Moesif down: err = %v, applied = %d", err, len(ent.applied))
	}
}

func TestDecideLevels(t *testing.T) {
	th := Thresholds{Eligible: 10, Moderate: 5}
	corporate := "CORPORATE"
	for _, c := range []struct {
		actions        int
		class          *string
		level, outcome string
	}{
		{0, nil, LevelNone, OutcomeMonitor},
		{4, nil, LevelLow, OutcomeMonitor},
		{5, nil, LevelModerate, OutcomeMonitor},
		{9, nil, LevelModerate, OutcomeMonitor},
		{10, nil, LevelHigh, OutcomeEligible},
		{0, &corporate, LevelNone, OutcomeEligible},
	} {
		values, outcome, _ := Decide(plgactivity.Activity{Actions: c.actions}, c.class, th)
		req := ApplyRequest{Values: values}
		if value(req, codeLevel) != c.level || outcome != c.outcome {
			t.Errorf("%d actions: level %q outcome %q, want %q %q", c.actions, value(req, codeLevel), outcome, c.level, c.outcome)
		}
	}
}

func find(req ApplyRequest, code string) Value {
	for _, v := range req.Values {
		if v.Code == code {
			return v
		}
	}
	return Value{}
}

func value(req ApplyRequest, code string) string {
	if v := find(req, code); v.TextValue != nil {
		return *v.TextValue
	}
	return ""
}
