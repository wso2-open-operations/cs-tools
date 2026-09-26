package service

import (
	"context"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The slice validates the same rules the BFF does, and that duplication is
// deliberate: the slice is reachable by anything with a network route to it, so
// a rule enforced only in the BFF is a rule enforced only for well-behaved
// callers. These tests exist to keep the second copy honest — if the BFF's
// tests pass and these fail, the slice has become the weaker of the two.

type fakeOrgPlatformRepo struct {
	patched []domain.PatchOrgPlatformRequest
}

func (f *fakeOrgPlatformRepo) Patch(_ context.Context, req domain.PatchOrgPlatformRequest, _ string) (domain.PatchPairingResult, error) {
	f.patched = append(f.patched, req)
	return domain.PatchPairingResult{RowsAffected: 1}, nil
}
func (f *fakeOrgPlatformRepo) Get(context.Context, string, string) (*domain.ProductDetail, error) {
	panic("not reached")
}
func (f *fakeOrgPlatformRepo) Acknowledge(context.Context, domain.AcknowledgeRequest, string) (domain.AcknowledgeResult, error) {
	panic("not reached")
}
func (f *fakeOrgPlatformRepo) AttachPlaybook(context.Context, domain.AttachPlaybookRequest, string) error {
	panic("not reached")
}
func (f *fakeOrgPlatformRepo) DetachRun(context.Context, string) (domain.WriteResult, string, string, error) {
	panic("not reached")
}
func (f *fakeOrgPlatformRepo) PatchRunTask(context.Context, domain.PatchRunTaskRequest, string) (string, string, error) {
	panic("not reached")
}
func (f *fakeOrgPlatformRepo) RunTaskShape(context.Context, string) (domain.RunTaskShape, error) {
	panic("not reached")
}
func (f *fakeOrgPlatformRepo) CreateNote(context.Context, domain.CreateNoteRequest, string) error {
	panic("not reached")
}
func (f *fakeOrgPlatformRepo) UpdateNote(context.Context, domain.UpdateNoteRequest, string) (domain.WriteResult, string, string, error) {
	panic("not reached")
}
func (f *fakeOrgPlatformRepo) SearchRegistrations(context.Context, domain.SearchRegistrationsRequest) ([]domain.RegistrationItem, int, error) {
	panic("not reached")
}
func (f *fakeOrgPlatformRepo) LocatePairing(context.Context, string) (string, string, error) {
	panic("not reached")
}

const (
	testPairingOrg = "11111111-1111-1111-1111-111111111111"
	testActor      = "33333333-3333-3333-3333-333333333333"
)

func slicePatch(mut func(*domain.PatchOrgPlatformRequest)) domain.PatchOrgPlatformRequest {
	req := domain.PatchOrgPlatformRequest{OrganizationID: testPairingOrg, ProductCode: "IAM"}
	mut(&req)
	return req
}

func TestSliceRequiresAReasonForEveryTrackedAxis(t *testing.T) {
	axes := map[string]func(*domain.PatchOrgPlatformRequest){
		"stage":  func(r *domain.PatchOrgPlatformRequest) { r.LifecycleStage = sptr(domain.StagePlgCsEligible) },
		"health": func(r *domain.PatchOrgPlatformRequest) { r.HealthState = sptr(domain.HealthAtRisk) },
		"tier":   func(r *domain.PatchOrgPlatformRequest) { r.SubscriptionTier = sptr(domain.TierPayg) },
	}
	for name, mutate := range axes {
		t.Run(name+" without a reason", func(t *testing.T) {
			repo := &fakeOrgPlatformRepo{}
			svc := NewPairingService(repo)
			_, err := svc.PatchPairing(context.Background(), slicePatch(mutate), testActor)
			if err == nil {
				t.Fatal("accepted a change with no reason")
			}
			if !strings.Contains(err.Error(), "reason") {
				t.Errorf("error does not mention the reason: %v", err)
			}
			if len(repo.patched) != 0 {
				t.Error("the write happened anyway")
			}
		})
		t.Run(name+" with a reason", func(t *testing.T) {
			repo := &fakeOrgPlatformRepo{}
			svc := NewPairingService(repo)
			_, err := svc.PatchPairing(context.Background(), slicePatch(func(r *domain.PatchOrgPlatformRequest) {
				mutate(r)
				r.Reason = sptr("a sentence someone will read later")
			}), testActor)
			if err != nil {
				t.Fatalf("rejected a well-formed change: %v", err)
			}
			if len(repo.patched) != 1 {
				t.Fatalf("expected one write, got %d", len(repo.patched))
			}
		})
	}
}

// The forward-only rule, checked here so the caller gets a sentence rather than
// a trigger's message. The trigger is still the authority — see
// plg_check_stage_move() and TestStageTriggerRefusesBackwardsMoves — and this
// only fires when the caller supplied the precondition.
func TestSliceRefusesBackwardsStageMovesWhenToldTheCurrentStage(t *testing.T) {
	tests := []struct {
		name     string
		from, to domain.LifecycleStage
		wantErr  bool
		wantIn   string
	}{
		{"backwards", domain.StageCommercial, domain.StageRegistration, true, "forward only"},
		{"leaving abandoned", domain.StageAbandoned, domain.StageCommercial, true, "cannot leave ABANDONED"},
		{"forward", domain.StageRegistration, domain.StagePlgCsEligible, false, ""},
		{"out to abandoned", domain.StageActivated, domain.StageAbandoned, false, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeOrgPlatformRepo{}
			svc := NewPairingService(repo)
			_, err := svc.PatchPairing(context.Background(), slicePatch(func(r *domain.PatchOrgPlatformRequest) {
				r.ExpectedStage = sptr(tc.from)
				r.LifecycleStage = sptr(tc.to)
				r.Reason = sptr("because")
			}), testActor)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("accepted a move from %s to %s", tc.from, tc.to)
				}
				if !strings.Contains(err.Error(), tc.wantIn) {
					t.Errorf("error %q does not mention %q", err.Error(), tc.wantIn)
				}
				if len(repo.patched) != 0 {
					t.Error("the write happened anyway")
				}
				return
			}
			if err != nil {
				t.Fatalf("refused a legal move from %s to %s: %v", tc.from, tc.to, err)
			}
		})
	}
}

// Without a precondition the service cannot know the current stage, so it lets
// the write through and the database trigger decides. This test pins that
// division of labour: it is why the trigger must exist, and why its message is
// surfaced as a 400 rather than swallowed into a 500.
func TestSliceDefersToTheTriggerWhenNoExpectedStageIsGiven(t *testing.T) {
	repo := &fakeOrgPlatformRepo{}
	svc := NewPairingService(repo)
	_, err := svc.PatchPairing(context.Background(), slicePatch(func(r *domain.PatchOrgPlatformRequest) {
		r.LifecycleStage = sptr(domain.StageRegistration) // backwards from almost anywhere
		r.Reason = sptr("because")
	}), testActor)
	if err != nil {
		t.Fatalf("the service should not have judged this without a precondition: %v", err)
	}
	if len(repo.patched) != 1 {
		t.Fatal("the request should have reached the repository, where the trigger sees it")
	}
}

func TestSliceRejectsAnUnknownActor(t *testing.T) {
	repo := &fakeOrgPlatformRepo{}
	svc := NewPairingService(repo)
	_, err := svc.PatchPairing(context.Background(), slicePatch(func(r *domain.PatchOrgPlatformRequest) {
		r.HealthState = sptr(domain.HealthAtRisk)
		r.Reason = sptr("because")
	}), "")
	if err == nil {
		t.Fatal("a write with no actor was accepted — every change records who made it")
	}
	if len(repo.patched) != 0 {
		t.Error("the write happened anyway")
	}
}

func sptr[T any](v T) *T { return &v }
