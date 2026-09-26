package service

import (
	"context"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/domain"
)

// shapeRepo answers RunTaskShape with one fixed shape and records the request
// that reached PatchRunTask, so a test can assert on what the service let
// through as well as on what it refused.
type shapeRepo struct {
	fakePairingRepo
	shape   domain.RunTaskShape
	patched *domain.PatchRunTaskRequest
}

func (r *shapeRepo) RunTaskShape(context.Context, string) (domain.RunTaskShape, error) {
	return r.shape, nil
}

func (r *shapeRepo) PatchRunTask(_ context.Context, req domain.PatchRunTaskRequest, _ string) (string, string, error) {
	r.patched = &req
	return "org-1", "IAM", nil
}

// A real UUID, per this repo's testing convention — the service validates the
// id's shape before it looks at the value, so a slug never reaches the switch.
const taskID = "11111111-1111-1111-1111-111111111111"

func singleSelectService() (*shapeRepo, OrgPlatformService) {
	repo := &shapeRepo{shape: domain.RunTaskShape{
		ValueType:   domain.ValueSingleSelect,
		OptionCodes: []string{"PRICE", "TECH", "QUIET"},
	}}
	return repo, NewOrgPlatformService(repo)
}

// A SINGLE_SELECT task must refuse an answer it does not offer.
//
// This is the case the value-type switch omitted entirely: with no
// SINGLE_SELECT branch, nothing validated the submitted code, and the database
// only requires the text be non-blank — so any string at all was stored as the
// chosen answer.
func TestSingleSelectRefusesAnAnswerItDoesNotOffer(t *testing.T) {
	repo, svc := singleSelectService()

	_, err := svc.PatchRunTask(context.Background(), domain.PatchRunTaskRequest{
		ID:        taskID,
		TextValue: ptr("NOT_AN_OPTION"),
	}, "actor-1")

	if err == nil {
		t.Fatal("want a validation error for an answer the task does not offer")
	}
	if !strings.Contains(err.Error(), "NOT_AN_OPTION") {
		t.Errorf("the error should name the rejected answer, got: %v", err)
	}
	if repo.patched != nil {
		t.Error("the write must not reach the repository")
	}
}

// An offered answer is accepted, and normalised on the way through.
func TestSingleSelectAcceptsAnOfferedAnswerAndNormalisesIt(t *testing.T) {
	repo, svc := singleSelectService()

	if _, err := svc.PatchRunTask(context.Background(), domain.PatchRunTaskRequest{
		ID:        taskID,
		TextValue: ptr("  price  "), // lower case, padded
	}, "actor-1"); err != nil {
		t.Fatalf("an offered answer should be accepted: %v", err)
	}
	if repo.patched == nil {
		t.Fatal("the write should have reached the repository")
	}
	if got := *repo.patched.TextValue; got != "PRICE" {
		t.Errorf("answer = %q, want it trimmed and upper-cased to PRICE", got)
	}
}

// A SINGLE_SELECT task takes textValue. Sending another type's field is a
// caller error and must be named as one, rather than reaching a CHECK
// constraint and surfacing as a database message.
func TestSingleSelectRefusesAnotherTypesField(t *testing.T) {
	for name, req := range map[string]domain.PatchRunTaskRequest{
		"a tick box":  {ID: taskID, BoolValue: ptr(true)},
		"a checklist": {ID: taskID, CheckedCodes: &[]string{"PRICE"}},
	} {
		t.Run(name, func(t *testing.T) {
			repo, svc := singleSelectService()
			if _, err := svc.PatchRunTask(context.Background(), req, "actor-1"); err == nil {
				t.Error("want a validation error")
			}
			if repo.patched != nil {
				t.Error("the write must not reach the repository")
			}
		})
	}
}

// The invalid-valueType message lists every accepted type. It listed four of
// five for as long as SINGLE_SELECT existed, so it is built from the enum now.
func TestValueTypeListCoversEveryType(t *testing.T) {
	got := valueTypeList()
	for _, want := range domain.TaskValueTypeOrder {
		if !strings.Contains(got, string(want)) {
			t.Errorf("%q is missing from the accepted-types message: %s", want, got)
		}
	}
}
