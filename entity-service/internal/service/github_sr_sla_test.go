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
	"errors"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// dedupingSLARepo is fakeFullSLAEngineRepo with RegisterClock's real
// contract: a (work item, policy) pair that already has a clock registers
// nothing and reports false.
type dedupingSLARepo struct {
	*fakeFullSLAEngineRepo
	clocks map[string]bool
}

func (r *dedupingSLARepo) RegisterClock(ctx context.Context, workItemID string, policy repository.SLAPolicyRef, startOn time.Time) (bool, error) {
	key := workItemID + "|" + policy.ID
	if r.clocks[key] {
		return false, nil
	}
	if _, err := r.fakeFullSLAEngineRepo.RegisterClock(ctx, workItemID, policy, startOn); err != nil {
		return false, err
	}
	r.clocks[key] = true
	return true, nil
}

// serviceRequestSLARepo seeds the two policies a new service request matches,
// with their real start conditions, and the facts of a just-created request.
func serviceRequestSLARepo() *dedupingSLARepo {
	base := newFakeFullSLAEngineRepo()
	base.candidatesByTarget = map[string][]repository.SLAPolicyRef{
		"RESPONSE": {
			{ID: "sr-resp", Name: "ServiceRequest SLA", Target: "RESPONSE", StartCondition: "case_type=service_request^state=OPEN"},
			{ID: "case-resp", Name: "S4 Response", Target: "RESPONSE", StartCondition: "state=OPEN^case_type=case^priority=S4"},
		},
		"RESOLUTION": {
			{ID: "sr-res", Name: "ServiceRequest resolution(2 B-weeks)", Target: "RESOLUTION", StartCondition: "case_type=service_request"},
		},
	}
	base.factsByCase["sr-new"] = repository.CaseFacts{State: "OPEN", CaseType: "service_request", CreatedOn: time.Now()}
	return &dedupingSLARepo{fakeFullSLAEngineRepo: base, clocks: map[string]bool{}}
}

func ghSRDelivery() Delivery {
	return ghDelivery("issues", "labeled", "[CR]: rotate gateway certificates", []string{"CR/NormalChange"})
}

func slaSvc(repo repository.SLAEngineRepository) GithubSyncService {
	svc := writingSvc(&fakeGhRepo{mapping: mapped()}, &fakeGhMutations{}, &fakeGhClient{})
	return WithGithubSLAEngine(svc, NewSLAEngineService(repo))
}

// An SR created from a GitHub issue registers the clocks of exactly the
// policies that match a service request.
func TestGithubSR_NewRecordRegistersMatchingSLAClocks(t *testing.T) {
	repo := serviceRequestSLARepo()
	if _, err := slaSvc(repo).HandleWebhook(context.Background(), ghSRDelivery()); err != nil {
		t.Fatalf("HandleWebhook: %v", err)
	}
	want := []string{"sr-new|sr-resp", "sr-new|sr-res"}
	if len(repo.registered) != len(want) {
		t.Fatalf("registered = %v, want %v", repo.registered, want)
	}
	for i, w := range want {
		if repo.registered[i] != w {
			t.Errorf("registered[%d] = %q, want %q", i, repo.registered[i], w)
		}
	}
}

// The engine is called once, for the new record.
func TestGithubSR_WebhookCallsEngineOnceForTheNewRecord(t *testing.T) {
	e := &fakeSLAEngineService{}
	svc := WithGithubSLAEngine(writingSvc(&fakeGhRepo{mapping: mapped()}, &fakeGhMutations{}, &fakeGhClient{}), e)
	if _, err := svc.HandleWebhook(context.Background(), ghSRDelivery()); err != nil {
		t.Fatalf("HandleWebhook: %v", err)
	}
	if len(e.registerCalls) != 1 || e.registerCalls[0].caseID != "sr-new" {
		t.Errorf("registerCalls = %v, want one for sr-new", e.registerCalls)
	}
}

// No engine (CSM_SLA_ENGINE_ENABLED off): the SR is still created, nothing is
// registered, and wiring a nil engine leaves the sync without one.
func TestGithubSR_NoSLAEngineRegistersNothing(t *testing.T) {
	mut := &fakeGhMutations{}
	svc := WithGithubSLAEngine(writingSvc(&fakeGhRepo{mapping: mapped()}, mut, &fakeGhClient{}), nil)
	if svc.(*githubSyncService).slaEngine != nil {
		t.Fatal("a nil engine must leave the sync without one")
	}
	out, err := svc.HandleWebhook(context.Background(), ghSRDelivery())
	if err != nil || out.Action != "created" || mut.created != 1 {
		t.Fatalf("HandleWebhook = %+v, %v (created %d), want a created SR", out, err, mut.created)
	}
}

// A redelivery registers nothing new: the engine's per-(record, policy)
// guard holds. When the first delivery's registration failed, the redelivery
// (which finds the record already inserted) repairs it.
func TestGithubSR_RedeliveryIsIdempotentAndRepairs(t *testing.T) {
	repo := serviceRequestSLARepo()
	if _, err := slaSvc(repo).HandleWebhook(context.Background(), ghSRDelivery()); err != nil {
		t.Fatalf("first delivery: %v", err)
	}
	first := len(repo.registered)

	// The record now exists: each further delivery (a fresh claim, as GitHub
	// redelivers under a new id) loses the insert and reports the winner.
	repo.factsByCase["sr-winner"] = repo.factsByCase["sr-new"]
	redeliver := func() {
		t.Helper()
		svc := WithGithubSLAEngine(writingSvc(&fakeGhRepo{mapping: mapped()}, &fakeGhMutations{srAlreadyExists: true}, &fakeGhClient{}), NewSLAEngineService(repo))
		if _, err := svc.HandleWebhook(context.Background(), ghSRDelivery()); err != nil {
			t.Fatalf("redelivery: %v", err)
		}
	}
	// The first redelivery registers the winner's two clocks (the repair); the
	// next adds none.
	redeliver()
	afterRepair := len(repo.registered)
	if afterRepair != first+2 {
		t.Fatalf("registered %d after a repair delivery, want %d", afterRepair, first+2)
	}
	redeliver()
	if len(repo.registered) != afterRepair {
		t.Errorf("a repeated redelivery registered %d new clocks, want 0", len(repo.registered)-afterRepair)
	}
}

// Registration is best-effort: an engine failure must not fail the delivery.
func TestGithubSR_RegistrationFailureDoesNotFailCreation(t *testing.T) {
	repo := serviceRequestSLARepo()
	repo.registerErr = errors.New("db unavailable")
	out, err := slaSvc(repo).HandleWebhook(context.Background(), ghSRDelivery())
	if err != nil || out.Action != "created" {
		t.Fatalf("HandleWebhook = %+v, %v, want a created SR", out, err)
	}
}

// orderRecorder logs the engine and automation calls in one sequence.
type orderRecorder struct {
	*fakeSLAEngineService
	log *[]string
}

func (o orderRecorder) RegisterCaseClocks(ctx context.Context, id string, s *domain.CaseSeverity, p string) {
	*o.log = append(*o.log, "clocks")
	o.fakeSLAEngineService.RegisterCaseClocks(ctx, id, s, p)
}

type orderNotifier struct {
	*recordingSRNotifier
	log *[]string
}

func (o orderNotifier) OnCreated(ctx context.Context, id string) {
	*o.log = append(*o.log, "automation")
	o.recordingSRNotifier.OnCreated(ctx, id)
}

// Acknowledge only writes state OPEN when the SR is not already OPEN, and a
// GitHub-created SR is inserted OPEN, so the automation never changes a field
// the clocks read; it needs no engine call of its own. What matters is that
// clocks exist before the automation (and the sr.created event) runs.
func TestGithubSR_ClocksAreRegisteredBeforeTheAutomation(t *testing.T) {
	var log []string
	svc := WithGithubSLAEngine(writingSvc(&fakeGhRepo{mapping: mapped()}, &fakeGhMutations{}, &fakeGhClient{}),
		orderRecorder{&fakeSLAEngineService{}, &log})
	svc.(*githubSyncService).srNotices = orderNotifier{&recordingSRNotifier{}, &log}
	if _, err := svc.HandleWebhook(context.Background(), ghSRDelivery()); err != nil {
		t.Fatalf("HandleWebhook: %v", err)
	}
	if len(log) != 2 || log[0] != "clocks" || log[1] != "automation" {
		t.Errorf("order = %v, want [clocks automation]", log)
	}
}
