package service

import (
	"context"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/domain"
)

// These tests exercise the BFF's validation, which is the layer that turns a
// bad request into a sentence. They use a fake repository rather than a
// database because none of the rules here need one: every case below is decided
// before the repository is reached, and the fake records whether it was.
//
// That last part is the interesting assertion in most of these. "Returns 400"
// and "returns 400 AND wrote nothing" are different claims, and only the second
// one is worth having.

type fakePairingRepo struct {
	patched  []domain.PatchOrgPlatformRequest
	detail   domain.ProductDetail
	getCalls int
}

func (f *fakePairingRepo) Patch(_ context.Context, req domain.PatchOrgPlatformRequest, _ string) error {
	f.patched = append(f.patched, req)
	return nil
}

func (f *fakePairingRepo) Get(_ context.Context, _, _ string) (*domain.ProductDetail, error) {
	f.getCalls++
	d := f.detail
	return &d, nil
}

// The rest of the interface. Nothing below is reached by these tests; a panic
// is a louder failure than a zero value if that ever stops being true.
func (f *fakePairingRepo) Acknowledge(context.Context, domain.AcknowledgeRequest, string) error {
	panic("not reached")
}
func (f *fakePairingRepo) AttachPlaybook(context.Context, domain.AttachPlaybookRequest, string) error {
	panic("not reached")
}
func (f *fakePairingRepo) DetachRun(context.Context, string) (string, string, error) {
	panic("not reached")
}
func (f *fakePairingRepo) PatchRunTask(context.Context, domain.PatchRunTaskRequest, string) (string, string, error) {
	panic("not reached")
}
func (f *fakePairingRepo) RunTaskShape(context.Context, string) (domain.RunTaskShape, error) {
	panic("not reached")
}
func (f *fakePairingRepo) CreateNote(context.Context, domain.CreateNoteRequest, string) error {
	panic("not reached")
}
func (f *fakePairingRepo) UpdateNote(context.Context, domain.UpdateNoteRequest, string) (string, string, error) {
	panic("not reached")
}
func (f *fakePairingRepo) SearchRegistrations(context.Context, domain.SearchRegistrationsRequest) ([]domain.RegistrationItem, int, error) {
	panic("not reached")
}
func (f *fakePairingRepo) LocatePairing(context.Context, string) (string, string, error) {
	panic("not reached")
}

func patchReq(mut func(*domain.PatchOrgPlatformRequest)) domain.PatchOrgPlatformRequest {
	req := domain.PatchOrgPlatformRequest{
		OrganizationID: "11111111-1111-1111-1111-111111111111",
		ProductCode:    "IAM",
	}
	mut(&req)
	return req
}

// Every tracked axis needs a reason, and the trial dates do not. Health is the
// easiest of the three to argue an exemption for, so it is the one most worth
// pinning.
func TestPatchRequiresAReasonForEveryTrackedAxis(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*domain.PatchOrgPlatformRequest)
		wantError bool
	}{
		{"stage without a reason", func(r *domain.PatchOrgPlatformRequest) {
			r.LifecycleStage = ptr(domain.StagePlgCsEligible)
		}, true},
		{"health without a reason", func(r *domain.PatchOrgPlatformRequest) {
			r.HealthState = ptr(domain.HealthAtRisk)
		}, true},
		{"tier without a reason", func(r *domain.PatchOrgPlatformRequest) {
			r.SubscriptionTier = ptr(domain.TierPayg)
		}, true},
		{"a reason of only spaces", func(r *domain.PatchOrgPlatformRequest) {
			r.HealthState = ptr(domain.HealthAtRisk)
			r.Reason = ptr("   ")
		}, true},
		{"an empty reason", func(r *domain.PatchOrgPlatformRequest) {
			r.HealthState = ptr(domain.HealthAtRisk)
			r.Reason = ptr("")
		}, true},
		{"stage with a reason", func(r *domain.PatchOrgPlatformRequest) {
			r.LifecycleStage = ptr(domain.StagePlgCsEligible)
			r.Reason = ptr("Real org, clear use case")
		}, false},
		{"health with a reason", func(r *domain.PatchOrgPlatformRequest) {
			r.HealthState = ptr(domain.HealthAtRisk)
			r.Reason = ptr("No logins for eleven days")
		}, false},
		{"tier with a reason", func(r *domain.PatchOrgPlatformRequest) {
			r.SubscriptionTier = ptr(domain.TierPayg)
			r.Reason = ptr("Converted after the pilot")
		}, false},
		// The trial end date is a date copied off a contract, not a judgement
		// anyone has to defend, so it is deliberately not a tracked axis.
		{"trial date alone needs no reason", func(r *domain.PatchOrgPlatformRequest) {
			r.TrialEndDate = ptr("2026-12-31")
		}, false},
		{"clearing the trial date needs no reason", func(r *domain.PatchOrgPlatformRequest) {
			r.ClearTrialEndDate = true
		}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakePairingRepo{}
			svc := NewOrgPlatformService(repo)

			_, err := svc.Patch(context.Background(), patchReq(tc.mutate), "actor")

			if tc.wantError {
				if err == nil {
					t.Fatal("expected a validation error, got none")
				}
				// The claim that matters: refused BEFORE anything was written.
				if len(repo.patched) != 0 {
					t.Errorf("request was rejected but the repository was still called %d time(s)", len(repo.patched))
				}
				if !strings.Contains(err.Error(), "reason") {
					t.Errorf("error does not mention the reason: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected the request to be accepted, got %v", err)
			}
			if len(repo.patched) != 1 {
				t.Fatalf("expected one write, got %d", len(repo.patched))
			}
		})
	}
}

// A reason is trimmed before it reaches the repository, so the history does not
// record leading whitespace someone pasted in.
func TestPatchTrimsTheReason(t *testing.T) {
	repo := &fakePairingRepo{}
	svc := NewOrgPlatformService(repo)

	_, err := svc.Patch(context.Background(), patchReq(func(r *domain.PatchOrgPlatformRequest) {
		r.HealthState = ptr(domain.HealthAtRisk)
		r.Reason = ptr("  sponsor left  ")
	}), "actor")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := repo.patched[0].Reason
	if got == nil || *got != "sponsor left" {
		t.Errorf("reason reached the repository as %q, want %q", derefOr(got, "<nil>"), "sponsor left")
	}
}

func TestPatchRejectsAnEmptyRequest(t *testing.T) {
	repo := &fakePairingRepo{}
	svc := NewOrgPlatformService(repo)

	if _, err := svc.Patch(context.Background(), patchReq(func(*domain.PatchOrgPlatformRequest) {}), "actor"); err == nil {
		t.Fatal("a request that changed nothing was accepted")
	}
	if len(repo.patched) != 0 {
		t.Error("an empty request reached the repository")
	}
}

func TestPatchRejectsUnknownEnumValues(t *testing.T) {
	cases := map[string]func(*domain.PatchOrgPlatformRequest){
		"stage": func(r *domain.PatchOrgPlatformRequest) {
			r.LifecycleStage = ptr(domain.LifecycleStage("SOMEWHERE_ELSE"))
			r.Reason = ptr("because")
		},
		"health": func(r *domain.PatchOrgPlatformRequest) {
			r.HealthState = ptr(domain.HealthState("UNKNOWN"))
			r.Reason = ptr("because")
		},
		"tier": func(r *domain.PatchOrgPlatformRequest) {
			r.SubscriptionTier = ptr(domain.SubscriptionTier("PLATINUM"))
			r.Reason = ptr("because")
		},
		// A value that looks like a health state but is not one.
		"a plausible non-value": func(r *domain.PatchOrgPlatformRequest) {
			r.HealthState = ptr(domain.HealthState("RISK_INTERVENTION"))
			r.Reason = ptr("because")
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &fakePairingRepo{}
			svc := NewOrgPlatformService(repo)
			if _, err := svc.Patch(context.Background(), patchReq(mutate), "actor"); err == nil {
				t.Fatal("an unknown enum value was accepted")
			}
			if len(repo.patched) != 0 {
				t.Error("the write happened anyway")
			}
		})
	}
}

func derefOr(s *string, fallback string) string {
	if s == nil {
		return fallback
	}
	return *s
}
