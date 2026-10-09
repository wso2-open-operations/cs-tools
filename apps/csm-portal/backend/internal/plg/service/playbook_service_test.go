package service

import (
	"context"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/domain"
)

// testActorID stands in for the "user" id PLG's identity middleware resolves.
const testActorID = "33333333-3333-3333-3333-333333333333"

type fakePlaybookRepo struct {
	created []domain.CreatePlaybookRequest
	// createdBy and patchedBy record the actor each call carried, so a test can
	// assert the caller reaches the repository rather than only that the call
	// compiled with an extra argument.
	createdBy       []string
	patchedBy       []string
	tasksReplacedBy []string
	got             domain.Playbook
}

func (f *fakePlaybookRepo) Create(_ context.Context, req domain.CreatePlaybookRequest, actorID string) (string, error) {
	f.created = append(f.created, req)
	f.createdBy = append(f.createdBy, actorID)
	return "22222222-2222-2222-2222-222222222222", nil
}
func (f *fakePlaybookRepo) Get(_ context.Context, _ string) (*domain.Playbook, error) {
	p := f.got
	return &p, nil
}
func (f *fakePlaybookRepo) ListAll(context.Context) ([]domain.Playbook, error) { panic("not reached") }
func (f *fakePlaybookRepo) ListByProduct(context.Context, string) ([]domain.Playbook, error) {
	panic("not reached")
}
func (f *fakePlaybookRepo) ListForStage(context.Context, string, domain.LifecycleStage, []domain.PlaybookType) ([]domain.Playbook, error) {
	panic("not reached")
}
func (f *fakePlaybookRepo) Patch(_ context.Context, _ domain.PatchPlaybookRequest, actorID string) error {
	f.patchedBy = append(f.patchedBy, actorID)
	return nil
}
func (f *fakePlaybookRepo) ReplaceTasks(_ context.Context, _ domain.ReplacePlaybookTasksRequest, actorID string) error {
	f.tasksReplacedBy = append(f.tasksReplacedBy, actorID)
	return nil
}
func (f *fakePlaybookRepo) Delete(context.Context, string) error { panic("not reached") }

func createReq(mut func(*domain.CreatePlaybookRequest)) domain.CreatePlaybookRequest {
	req := domain.CreatePlaybookRequest{
		ProductCode:    "IAM",
		Name:           "Qualify the registration",
		LifecycleStage: domain.StageRegistration,
		PlaybookType:   domain.PlaybookProgressive,
	}
	mut(&req)
	return req
}

// Kind and stage are both required, and neither is defaulted. A default would
// be the most common real answer, which is exactly what makes it dangerous:
// forgetting to choose would produce a plausible playbook at the wrong stage
// rather than an obviously broken one.
func TestCreatePlaybookRequiresNameKindAndStage(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*domain.CreatePlaybookRequest)
		wantIn string
	}{
		{"no name", func(r *domain.CreatePlaybookRequest) { r.Name = "" }, "name"},
		{"a name of spaces", func(r *domain.CreatePlaybookRequest) { r.Name = "   " }, "name"},
		{"no kind", func(r *domain.CreatePlaybookRequest) { r.PlaybookType = "" }, "playbookType"},
		{"no stage", func(r *domain.CreatePlaybookRequest) { r.LifecycleStage = "" }, "lifecycleStage"},
		{"the retired kind", func(r *domain.CreatePlaybookRequest) {
			r.PlaybookType = domain.PlaybookType("RISK_INTERVENTION")
		}, "playbookType"},
		{"a stage that is not one", func(r *domain.CreatePlaybookRequest) {
			r.LifecycleStage = domain.LifecycleStage("AT_RISK")
		}, "lifecycleStage"},
		// Nothing progresses out of ABANDONED and a pairing there is gone, so a
		// playbook of any kind could never run.
		{"at ABANDONED", func(r *domain.CreatePlaybookRequest) {
			r.LifecycleStage = domain.StageAbandoned
		}, "ABANDONED"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakePlaybookRepo{}
			svc := NewPlaybookService(repo)
			_, err := svc.Create(context.Background(), createReq(tc.mutate), testActorID)
			if err == nil {
				t.Fatal("expected a validation error, got none")
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Errorf("error %q does not mention %q", err.Error(), tc.wantIn)
			}
			if len(repo.created) != 0 {
				t.Error("the playbook was created anyway")
			}
		})
	}
}

// All three kinds must be accepted. The bug this guards against is specific and
// has happened once already: a validator that tested for one list type rejected
// the other, and four of seven playbooks failed silently because the failure
// looked like an authoring mistake.
func TestCreatePlaybookAcceptsEveryKind(t *testing.T) {
	for _, kind := range domain.PlaybookTypeOrder {
		t.Run(string(kind), func(t *testing.T) {
			repo := &fakePlaybookRepo{}
			svc := NewPlaybookService(repo)
			_, err := svc.Create(context.Background(), createReq(func(r *domain.CreatePlaybookRequest) {
				r.PlaybookType = kind
			}), testActorID)
			if err != nil {
				t.Fatalf("%s was rejected: %v", kind, err)
			}
			if len(repo.created) != 1 {
				t.Fatalf("expected one create, got %d", len(repo.created))
			}
			if repo.created[0].PlaybookType != kind {
				t.Errorf("kind reached the repository as %s, want %s", repo.created[0].PlaybookType, kind)
			}
		})
	}
}

// The same shape, for task types. Both list types must be allowed to carry the
// options they require, and nothing else may carry any.
func TestTaskOptionsAreRequiredByListTypesAndRefusedByTheRest(t *testing.T) {
	opts := []domain.ChecklistOption{
		{Code: "EVAL", Label: "Evaluating"},
		{Code: "BUILD", Label: "Building now"},
	}

	for _, vt := range domain.TaskValueTypeOrder {
		needsOptions := domain.TypeNeedsOptions(vt)

		t.Run(string(vt)+" with options", func(t *testing.T) {
			repo := &fakePlaybookRepo{}
			svc := NewPlaybookService(repo)
			_, err := svc.Create(context.Background(), createReq(func(r *domain.CreatePlaybookRequest) {
				r.Tasks = []domain.PlaybookTaskInput{{Name: "A task", ValueType: vt, Options: opts}}
			}), testActorID)
			if needsOptions && err != nil {
				t.Fatalf("%s carries options by definition but was rejected: %v", vt, err)
			}
			if !needsOptions && err == nil {
				t.Fatalf("%s cannot hold a list, but options were accepted", vt)
			}
		})

		t.Run(string(vt)+" without options", func(t *testing.T) {
			repo := &fakePlaybookRepo{}
			svc := NewPlaybookService(repo)
			_, err := svc.Create(context.Background(), createReq(func(r *domain.CreatePlaybookRequest) {
				r.Tasks = []domain.PlaybookTaskInput{{Name: "A task", ValueType: vt}}
			}), testActorID)
			if needsOptions && err == nil {
				t.Fatalf("%s needs an answer list, but none was required", vt)
			}
			if !needsOptions && err != nil {
				t.Fatalf("%s needs no options but was rejected: %v", vt, err)
			}
		})
	}
}

// A list with one entry is not a choice.
func TestListTasksNeedAtLeastTwoOptions(t *testing.T) {
	for _, vt := range []domain.TaskValueType{domain.ValueChecklist, domain.ValueSingleSelect} {
		t.Run(string(vt), func(t *testing.T) {
			repo := &fakePlaybookRepo{}
			svc := NewPlaybookService(repo)
			_, err := svc.Create(context.Background(), createReq(func(r *domain.CreatePlaybookRequest) {
				r.Tasks = []domain.PlaybookTaskInput{{
					Name:      "A task",
					ValueType: vt,
					Options:   []domain.ChecklistOption{{Code: "ONLY", Label: "The only one"}},
				}}
			}), testActorID)
			if err == nil {
				t.Fatalf("%s was accepted with a single option", vt)
			}
		})
	}
}

// The bookends are structural: the API adds them, so an author never submits
// them and every run gets them.
func TestBookendsAreAddedAroundTheAuthorsTasks(t *testing.T) {
	repo := &fakePlaybookRepo{}
	svc := NewPlaybookService(repo)
	_, err := svc.Create(context.Background(), createReq(func(r *domain.CreatePlaybookRequest) {
		r.Tasks = []domain.PlaybookTaskInput{
			{Name: "Company profile", ValueType: domain.ValueString},
		}
	}), testActorID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tasks := repo.created[0].Tasks
	if len(tasks) != 3 {
		t.Fatalf("expected the author's task between two bookends, got %d tasks", len(tasks))
	}
	if !domain.IsBookendTask(tasks[0].Code) {
		t.Errorf("first task is %q, expected a bookend", tasks[0].Code)
	}
	if !domain.IsBookendTask(tasks[len(tasks)-1].Code) {
		t.Errorf("last task is %q, expected a bookend", tasks[len(tasks)-1].Code)
	}
	if tasks[1].Name != "Company profile" {
		t.Errorf("the author's task is not in the middle: %q", tasks[1].Name)
	}
}

// TestPlaybookWritesCarryTheActor pins that the caller reaches the repository.
//
// Threading an argument through four layers compiles whether or not the right
// value arrives: a handler passing "" , or a service passing its own parameter
// to the wrong position, both build. The repository is where the value stops
// being plumbing and becomes a column, so that is where this asserts.
func TestPlaybookWritesCarryTheActor(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		repo := &fakePlaybookRepo{}
		svc := NewPlaybookService(repo)
		if _, err := svc.Create(context.Background(), createReq(func(*domain.CreatePlaybookRequest) {}), testActorID); err != nil {
			t.Fatalf("create: %v", err)
		}
		if len(repo.createdBy) != 1 {
			t.Fatalf("expected one create, got %d", len(repo.createdBy))
		}
		if repo.createdBy[0] != testActorID {
			t.Errorf("repository received actor %q, want %q", repo.createdBy[0], testActorID)
		}
	})

	t.Run("patch", func(t *testing.T) {
		repo := &fakePlaybookRepo{}
		svc := NewPlaybookService(repo)
		name := "renamed"
		if _, err := svc.Patch(context.Background(),
			domain.PatchPlaybookRequest{ID: "11111111-1111-1111-1111-111111111111", Name: &name},
			testActorID); err != nil {
			t.Fatalf("patch: %v", err)
		}
		if len(repo.patchedBy) != 1 {
			t.Fatalf("expected one patch, got %d", len(repo.patchedBy))
		}
		if repo.patchedBy[0] != testActorID {
			t.Errorf("repository received actor %q, want %q", repo.patchedBy[0], testActorID)
		}
	})
}

// TestReplaceTasksCarriesTheActor covers the third write on a playbook.
//
// Tasks are edited far more often than the playbook row itself -- adding one,
// reordering, retiring one -- and any admin may do it to any playbook, so this
// is the attribution that answers "who changed the checklist".
func TestReplaceTasksCarriesTheActor(t *testing.T) {
	repo := &fakePlaybookRepo{}
	svc := NewPlaybookService(repo)
	_, err := svc.ReplaceTasks(context.Background(), domain.ReplacePlaybookTasksRequest{
		PlaybookID: "11111111-1111-1111-1111-111111111111",
		Tasks: []domain.PlaybookTaskInput{
			{Code: "INITIATE", Name: "Initiate", ValueType: domain.ValueBoolean},
			{Code: "CLOSE_PLAYBOOK", Name: "Close", ValueType: domain.ValueBoolean},
		},
	}, testActorID)
	if err != nil {
		t.Fatalf("replace tasks: %v", err)
	}
	if len(repo.tasksReplacedBy) != 1 {
		t.Fatalf("expected one call, got %d", len(repo.tasksReplacedBy))
	}
	if repo.tasksReplacedBy[0] != testActorID {
		t.Errorf("repository received actor %q, want %q", repo.tasksReplacedBy[0], testActorID)
	}
}
