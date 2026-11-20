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
package plgsignups

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/moesif"
)

var now = time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)

type fakeSearcher struct {
	hits []moesif.Hit
	more bool
	err  error
	q    moesif.Query
}

func (f *fakeSearcher) SearchEvents(_ context.Context, q moesif.Query) ([]moesif.Hit, bool, error) {
	f.q = q
	return f.hits, f.more, f.err
}

type fakeEntity struct {
	batches  [][]Registration
	status   map[string]string // company id -> status; default created
	err      error
	failures []Failure
	refuse   int // RecordFailure calls refused, from the first
	attempts int
}

func (f *fakeEntity) Ingest(_ context.Context, regs []Registration) ([]Result, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.batches = append(f.batches, regs)
	var out []Result
	for _, r := range regs {
		status := "created"
		if s, ok := f.status[r.CompanyID]; ok {
			status = s
		}
		out = append(out, Result{Status: status, Message: "m"})
	}
	return out, nil
}

func (f *fakeEntity) RecordFailure(_ context.Context, fl Failure) error {
	f.attempts++
	if f.attempts <= f.refuse {
		return errors.New("status 400")
	}
	f.failures = append(f.failures, fl)
	return nil
}

func registration(id, company, product string) moesif.Hit {
	return moesif.Hit{ID: id, Source: map[string]any{
		"action_name": "cloud-core.org-account-management.register-organization",
		"user_id":     "u-" + id, "company_id": company,
		"request":  map[string]any{"time": "2026-11-09T11:00:00.000"},
		"metadata": map[string]any{"initiating_product": product},
	}}
}

func subscription(id, company, product string) moesif.Hit {
	return moesif.Hit{ID: id, Source: map[string]any{
		"action_name": "cloud-core.billing.select-subscription-plan",
		"user_id":     "u-" + id, "company_id": company,
		"request":  map[string]any{"time": "2026-11-09T11:30:00.000Z"},
		"metadata": map[string]any{"product": product},
	}}
}

func opts() Options {
	return Options{Events: DefaultEvents, ProductFields: DefaultProductFields, Window: 2 * time.Hour, BatchSize: 2, MaxPages: 5}
}

func run(src *fakeSearcher, ent *fakeEntity) error {
	return Sync(src, ent, opts(), func() time.Time { return now })(context.Background())
}

func TestEachEventLandsAsARegistrationByIdOnly(t *testing.T) {
	src := &fakeSearcher{hits: []moesif.Hit{
		registration("1", "c-1", "WSO2Cloud"), subscription("2", "c-1", "api-platform"), subscription("3", "c-2", "identity-platform"),
	}}
	ent := &fakeEntity{}
	if err := run(src, ent); err != nil {
		t.Fatal(err)
	}
	if !src.q.From.Equal(now.Add(-2*time.Hour)) || !src.q.To.Equal(now) || src.q.MaxPages != 5 {
		t.Errorf("query window = %v..%v, pages %d", src.q.From, src.q.To, src.q.MaxPages)
	}
	if len(ent.batches) != 2 {
		t.Fatalf("batches = %d, want 2 of size 2", len(ent.batches))
	}
	raw, _ := json.Marshal(ent.batches[0])
	want := `[{"createdOn":"2026-11-09T11:00:00.000Z","companyId":"c-1","userId":"u-1","initiatingProduct":"wso2cloud"},` +
		`{"createdOn":"2026-11-09T11:30:00.000Z","companyId":"c-1","userId":"u-2","initiatingProduct":"api-platform"}]`
	if string(raw) != want {
		t.Errorf("sent %s\nwant %s", raw, want)
	}
	if len(ent.failures) != 0 {
		t.Errorf("failures = %+v", ent.failures)
	}
}

func TestAnEventOutsideTheWindowIsSkipped(t *testing.T) {
	old := registration("old", "c-9", "core")
	old.Source["request"] = map[string]any{"time": "2026-11-09T01:00:00.000"}
	ent := &fakeEntity{}
	if err := run(&fakeSearcher{hits: []moesif.Hit{old}}, ent); err != nil {
		t.Fatal(err)
	}
	if len(ent.batches) != 0 || len(ent.failures) != 0 {
		t.Errorf("an event 11 hours old was used: %+v %+v", ent.batches, ent.failures)
	}
}

func TestOnlyTheFieldsUsedAreRequested(t *testing.T) {
	src := &fakeSearcher{}
	if err := run(src, &fakeEntity{}); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(src.q.Fields, ",")
	if got != "request.time,action_name,user_id,company_id,metadata.initiating_product,metadata.product" {
		t.Errorf("fields = %s", got)
	}
}

func TestAnInvalidOrRejectedEventIsRecordedByIdsOnly(t *testing.T) {
	src := &fakeSearcher{hits: []moesif.Hit{
		registration("1", "", "core"), subscription("2", "c-2", ""), subscription("3", "c-3", "choreo"),
	}}
	ent := &fakeEntity{status: map[string]string{"c-3": "FAILED"}}
	if err := run(src, ent); err != nil {
		t.Fatalf("a rejected event must not fail the run: %v", err)
	}
	if len(ent.failures) != 3 {
		t.Fatalf("failures = %d, want 3", len(ent.failures))
	}
	if f := ent.failures[0]; f.EventID != "1" || !strings.Contains(f.Failure, "no company id") {
		t.Errorf("first failure = %+v", f)
	}
	if f := ent.failures[2]; f.EventType != "cloud-core.billing.select-subscription-plan" || !strings.Contains(f.Failure, "FAILED") {
		t.Errorf("rejected failure = %+v", f)
	}
	for _, f := range ent.failures {
		var p map[string]string
		_ = json.Unmarshal(f.Payload, &p)
		for k := range p {
			if k != "companyId" && k != "userId" && k != "actionName" && k != "product" && k != "requestTime" {
				t.Errorf("payload carries %q", k)
			}
		}
	}
}

func TestAnUnreachableServiceFailsTheRun(t *testing.T) {
	if err := run(&fakeSearcher{err: moesif.ErrUnavailable}, &fakeEntity{}); !errors.Is(err, moesif.ErrUnavailable) {
		t.Errorf("Moesif down: err = %v", err)
	}
	down := errors.New("down")
	if err := run(&fakeSearcher{hits: []moesif.Hit{registration("1", "c-1", "core")}}, &fakeEntity{err: down}); !errors.Is(err, down) {
		t.Errorf("entity-service down: err = %v", err)
	}
}

func TestAWindowLargerThanOneRunFailsAfterLandingWhatItRead(t *testing.T) {
	ent := &fakeEntity{}
	err := run(&fakeSearcher{hits: []moesif.Hit{registration("1", "c-1", "core")}, more: true}, ent)
	if err == nil || !strings.Contains(err.Error(), "PLG_MOESIF_SYNC_MAX_PAGES") {
		t.Errorf("err = %v", err)
	}
	if len(ent.batches) != 1 {
		t.Errorf("what was read should still land")
	}
}

func TestARefusedFailureRecordDoesNotHideTheRest(t *testing.T) {
	src := &fakeSearcher{hits: []moesif.Hit{
		registration("1", "c-1", "WSO2Cloud"), registration("2", "c-2", "WSO2Cloud"), registration("3", "c-3", "WSO2Cloud"),
	}}
	ent := &fakeEntity{status: map[string]string{"c-1": "FAILED", "c-2": "FAILED", "c-3": "FAILED"}, refuse: 1}
	err := run(src, ent)
	if err == nil || !strings.Contains(err.Error(), "record 1 of 3 failures") {
		t.Fatalf("err = %v, want the one refused record named", err)
	}
	if ent.attempts != 3 || len(ent.failures) != 2 {
		t.Errorf("attempts = %d, recorded = %d; want all 3 attempted, 2 recorded", ent.attempts, len(ent.failures))
	}
}
