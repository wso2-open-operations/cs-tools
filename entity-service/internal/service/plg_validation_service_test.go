package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

type fakeValidationRepo struct {
	queueLimit int
	queueCode  string
	applied    bool
}

func (f *fakeValidationRepo) Queue(_ context.Context, code string, _ time.Time, limit int) ([]domain.ValidationQueueItem, error) {
	f.queueCode, f.queueLimit = code, limit
	return []domain.ValidationQueueItem{}, nil
}

func (f *fakeValidationRepo) Apply(context.Context, domain.ApplyValidationRequest) (domain.ApplyValidationResult, error) {
	f.applied = true
	return domain.ApplyValidationResult{}, nil
}

func TestValidationQueueNormalisesItsRequest(t *testing.T) {
	repo := &fakeValidationRepo{}
	svc := NewValidationService(repo)
	after := time.Now().Add(-96 * time.Hour)

	if _, err := svc.Queue(context.Background(), domain.ValidationQueueRequest{ProductCode: " agent_platform ", RegisteredAfter: after}); err != nil {
		t.Fatalf("queue: %v", err)
	}
	if repo.queueCode != "AGENT_PLATFORM" || repo.queueLimit != defaultValidationQueueLimit {
		t.Errorf("repo got code=%q limit=%d", repo.queueCode, repo.queueLimit)
	}
	if _, _ = svc.Queue(context.Background(), domain.ValidationQueueRequest{ProductCode: "IAM", RegisteredAfter: after, Limit: 5000}); repo.queueLimit != maxValidationQueueLimit {
		t.Errorf("limit not capped: %d", repo.queueLimit)
	}
	for name, req := range map[string]domain.ValidationQueueRequest{
		"no product": {RegisteredAfter: after},
		"no date":    {ProductCode: "IAM"},
	} {
		var ve *apierror.ValidationError
		if _, err := svc.Queue(context.Background(), req); !errors.As(err, &ve) {
			t.Errorf("%s: err = %v, want a validation error", name, err)
		}
	}
}

func TestValidationApplyRefusesWhatTheJobMustNotSend(t *testing.T) {
	now := time.Now()
	text := "COMPLETED"
	valid := domain.ApplyValidationRequest{
		OrgPlatformID: "019a2b3c-4d5e-7f60-8a9b-0c1d2e3f4a5b",
		Summary:       domain.ActivitySummary{WindowStart: now, WindowEnd: now.Add(time.Hour)},
		Values:        []domain.ValidationValue{{Code: "ONBOARDING_STATUS", TextValue: &text}},
	}
	repo := &fakeValidationRepo{}
	if _, err := NewValidationService(repo).Apply(context.Background(), valid); err != nil || !repo.applied {
		t.Fatalf("valid request: err = %v, applied = %v", err, repo.applied)
	}

	bad := map[string]func(r *domain.ApplyValidationRequest){
		"pairing id":      func(r *domain.ApplyValidationRequest) { r.OrgPlatformID = "nope" },
		"no window":       func(r *domain.ApplyValidationRequest) { r.Summary.WindowStart = time.Time{} },
		"window reversed": func(r *domain.ApplyValidationRequest) { r.Summary.WindowEnd = now.Add(-time.Hour) },
		"negative count":  func(r *domain.ApplyValidationRequest) { r.Summary.EventCount = -1 },
		"bad action":      func(r *domain.ApplyValidationRequest) { r.Summary.ActionCounts = map[string]int{" ": 1} },
		"closes the run": func(r *domain.ApplyValidationRequest) {
			yes := true
			r.Values = []domain.ValidationValue{{Code: domain.TaskCodeClose, BoolValue: &yes}}
		},
		"un-initiates": func(r *domain.ApplyValidationRequest) {
			no := false
			r.Values = []domain.ValidationValue{{Code: domain.TaskCodeInitiate, BoolValue: &no}}
		},
		"abandons without EXCLUDED": func(r *domain.ApplyValidationRequest) { r.Abandon = true },
		"repeated code":             func(r *domain.ApplyValidationRequest) { r.Values = append(r.Values, r.Values[0]) },
		"empty code":                func(r *domain.ApplyValidationRequest) { r.Values = []domain.ValidationValue{{}} },
	}
	for name, mutate := range bad {
		req := valid
		req.Values = append([]domain.ValidationValue(nil), valid.Values...)
		mutate(&req)
		repo := &fakeValidationRepo{}
		var ve *apierror.ValidationError
		if _, err := NewValidationService(repo).Apply(context.Background(), req); !errors.As(err, &ve) || repo.applied {
			t.Errorf("%s: err = %v, applied = %v; want refused before the repository", name, err, repo.applied)
		}
	}
}
