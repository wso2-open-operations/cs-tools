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
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/engineeringentity"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	pwCaseID = "0190a4b0-0000-7000-8000-000000000001"
	pwEmail  = "engineer@wso2.com"
)

type fakePWEng struct {
	createCalls []fakePWCreate
	createRef   engineeringentity.IssueRef
	createErr   error
	detail      engineeringentity.IssueDetail
	detailErr   error
	detailCalls int
	list        []engineeringentity.IssueDetail
	listErr     error
	listCalls   []engineeringentity.ListIssueDetailsQuery
}

type fakePWCreate struct {
	org, owner, repo, title, body string
	labels                        []string
}

func (f *fakePWEng) CreateIssue(_ context.Context, org, owner, repo, title, body string, labels []string) (engineeringentity.IssueRef, error) {
	f.createCalls = append(f.createCalls, fakePWCreate{org, owner, repo, title, body, labels})
	return f.createRef, f.createErr
}
func (f *fakePWEng) GetIssueDetail(context.Context, string, string, string, int) (engineeringentity.IssueDetail, error) {
	f.detailCalls++
	return f.detail, f.detailErr
}
func (f *fakePWEng) ListIssueDetails(_ context.Context, _, _, _ string, q engineeringentity.ListIssueDetailsQuery) ([]engineeringentity.IssueDetail, error) {
	f.listCalls = append(f.listCalls, q)
	if q.Page > 1 {
		return nil, f.listErr
	}
	return f.list, f.listErr
}

type fakePWRepo struct {
	kase      repository.ProductWorkItemCase
	loadErr   error
	begin     repository.CreationRequest
	beginErr  error
	beginArgs []repository.BeginCreationParams
	claimed   map[int]bool

	recorded     []repository.GitHubIssueRef
	recordErr    error
	outcomes     []fakePWOutcome
	registered   []repository.RegisterParams
	registerKind string
	registerErr  error
	replayed     bool
	completed    []repository.RegisterParams
	completeErr  error
}

type fakePWOutcome struct {
	note     string
	definite bool
}

func (f *fakePWRepo) LoadCase(context.Context, string) (repository.ProductWorkItemCase, error) {
	return f.kase, f.loadErr
}
func (f *fakePWRepo) BeginCreationRequest(_ context.Context, p repository.BeginCreationParams) (repository.CreationRequest, error) {
	f.beginArgs = append(f.beginArgs, p)
	cr := f.begin
	if cr.RequestHash == "" {
		cr.RequestHash = p.Hash
	}
	return cr, f.beginErr
}
func (f *fakePWRepo) IssueClaimed(_ context.Context, _, _ string, n int) (bool, error) {
	return f.claimed[n], nil
}
func (f *fakePWRepo) RecordGitHubCreated(_ context.Context, _, _ string, ref repository.GitHubIssueRef) error {
	f.recorded = append(f.recorded, ref)
	return f.recordErr
}
func (f *fakePWRepo) RecordOutcome(_ context.Context, _, _, note string, definite bool) error {
	f.outcomes = append(f.outcomes, fakePWOutcome{note, definite})
	return nil
}
func (f *fakePWRepo) RegisterProductIssue(_ context.Context, p repository.RegisterParams) (repository.RegisterOutcome, error) {
	f.registerKind = domain.ProductWorkItemKindProductIssue
	f.registered = append(f.registered, p)
	if f.registerErr != nil {
		return repository.RegisterOutcome{}, f.registerErr
	}
	if f.replayed {
		b, _ := json.Marshal(p.Respond("0190a4b0-0000-7000-8000-0000000000aa", "PI0000001"))
		return repository.RegisterOutcome{Response: b, Replayed: true}, nil
	}
	return repository.RegisterOutcome{WorkItemID: "0190a4b0-0000-7000-8000-0000000000aa", Number: "PI0000001"}, nil
}
func (f *fakePWRepo) CompleteRequest(_ context.Context, p repository.RegisterParams) (repository.RegisterOutcome, error) {
	f.completed = append(f.completed, p)
	if f.completeErr != nil {
		return repository.RegisterOutcome{}, f.completeErr
	}
	id, num := "", ""
	if f.registerKind == domain.ProductWorkItemKindProductIssue {
		id, num = "0190a4b0-0000-7000-8000-0000000000aa", "PI0000001"
	}
	b, _ := json.Marshal(p.Respond(id, num))
	return repository.RegisterOutcome{Response: b}, nil
}

type fakePWCases struct {
	calls   []string
	notes   []string
	tags    []string
	tagErr  error
	noteErr error
	actors  []string
}

func (f *fakePWCases) AddCaseTagAs(_ context.Context, _, label, actor string) (domain.Tag, error) {
	f.calls = append(f.calls, "tag")
	f.tags = append(f.tags, label)
	f.actors = append(f.actors, actor)
	return domain.Tag{}, f.tagErr
}
func (f *fakePWCases) CreateCaseCommentAs(_ context.Context, r domain.CreateCaseCommentRequest, actor string) (domain.CreateCaseCommentResponse, error) {
	f.calls = append(f.calls, "note")
	if r.Type != domain.CommentTypeWorkNote {
		panic("not a work note")
	}
	f.notes = append(f.notes, r.Content)
	f.actors = append(f.actors, actor)
	return domain.CreateCaseCommentResponse{}, f.noteErr
}

type fakePWMappings struct{ rows []domain.ProductRepoMapping }

func (f fakePWMappings) ListActive(context.Context) ([]domain.ProductRepoMapping, error) {
	return f.rows, nil
}

type pwEnv struct {
	svc   ProductWorkItemService
	eng   *fakePWEng
	repo  *fakePWRepo
	cases *fakePWCases
}

func newPWEnv() *pwEnv {
	eng := &fakePWEng{
		createRef: engineeringentity.IssueRef{ID: 1, NodeID: "I_kw1", Number: 42},
		detail: engineeringentity.IssueDetail{NodeID: "I_kw1", Number: 42, HTMLURL: "https://github.com/example-org/is/issues/42",
			CreatedAt: "2026-11-18T10:00:00Z", Labels: []string{"Origin/CS"}},
	}
	repo := &fakePWRepo{
		kase: repository.ProductWorkItemCase{ID: pwCaseID, Number: "CS0000001", InternalID: "ACME-1", State: domain.CaseStateOpen,
			ProductName: "WSO2 Identity Server", DisplayName: "WSO2 Identity Server 6.1.0", ProductSlug: "wso2is-6.1.0", Team: "Sirius"},
		begin: repository.CreationRequest{ID: "req-1", Status: repository.CreationStatusPending, Inserted: true, CreatedOn: time.Now()},
	}
	maps := fakePWMappings{rows: []domain.ProductRepoMapping{{ProductName: "WSO2 Identity Server", Owner: "example-org", Repository: "is", GithubLabel: "product/is"}}}
	cases := &fakePWCases{}
	return &pwEnv{svc: NewProductWorkItemService(eng, repo, maps, cases, internalAccess{}), eng: eng, repo: repo, cases: cases}
}

func pwCtx() context.Context {
	return auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: pwEmail})
}

func pwReq() domain.CreateProductWorkItemRequest {
	return domain.CreateProductWorkItemRequest{CaseID: pwCaseID, IdempotencyKey: "key-1", Title: "It fails", Description: "Details.",
		IssueTypeLabel: "Type/Discussion", PriorityLevel: "Priority/High", UpdateLevel: "U12"}
}

func (e *pwEnv) create(t *testing.T, req domain.CreateProductWorkItemRequest) (domain.CreateProductWorkItemResponse, bool, error) {
	t.Helper()
	return e.svc.CreateProductWorkItem(pwCtx(), req)
}

func wantStatus(t *testing.T, err error, status int) *UpstreamFailureError {
	t.Helper()
	var ufe *UpstreamFailureError
	if !errors.As(err, &ufe) || ufe.Status != status {
		t.Fatalf("err = %v, want an upstream failure with status %d", err, status)
	}
	return ufe
}

func TestProductWorkItem_ProductIssueHappyPath(t *testing.T) {
	e := newPWEnv()
	resp, created, err := e.create(t, pwReq())
	if err != nil || !created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	if resp.Message != "GitHub issue created." || resp.Kind != "PRODUCT_ISSUE" ||
		resp.Issue.URL != "https://github.com/example-org/is/issues/42" || resp.Issue.Number != 42 || resp.Issue.Repo != "example-org/is" ||
		resp.WorkItem == nil || resp.WorkItem.Number != "PI0000001" {
		t.Fatalf("resp = %+v", resp)
	}
	if len(e.eng.createCalls) != 1 {
		t.Fatalf("create calls = %d", len(e.eng.createCalls))
	}
	c := e.eng.createCalls[0]
	if c.org != "example-org" || c.owner != "example-org" || c.repo != "is" || c.title != "It fails" {
		t.Errorf("create = %+v", c)
	}
	wantLabels := []string{"Origin/CS", "product/is", "Type/Incident", "Priority/High(P2)"}
	if !slices.Equal(c.labels, wantLabels) {
		t.Errorf("labels = %v, want %v", c.labels, wantLabels)
	}
	wantBody := "Details.\n\n" + infoBlock("wso2is-6.1.0", "U12", "ACME-1", "CS0000001", "", "Sirius", pwEmail)
	if c.body != wantBody {
		t.Errorf("body = %q, want %q", c.body, wantBody)
	}
	if len(e.repo.recorded) != 1 || e.repo.recorded[0].Number != 42 {
		t.Errorf("GITHUB_CREATED not recorded: %+v", e.repo.recorded)
	}
	if !slices.Equal(e.cases.notes, []string{"GitHub issue filed: https://github.com/example-org/is/issues/42"}) || len(e.cases.tags) != 0 ||
		!slices.Equal(e.cases.actors, []string{pwEmail}) || len(e.repo.completed) != 1 {
		t.Errorf("note=%v tags=%v actors=%v completed=%d", e.cases.notes, e.cases.tags, e.cases.actors, len(e.repo.completed))
	}
	p := e.repo.registered[0]
	if e.repo.registerKind != "PRODUCT_ISSUE" || p.Priority != "P2" || p.Category != "Type/Discussion" || p.ABTTeam != "Sirius" ||
		p.Title != "It fails" ||
		p.Issue.NodeID != "I_kw1" || p.GitHubCreatedOn == nil || !slices.Equal(p.Labels, wantLabels) {
		t.Errorf("registered = %+v", p)
	}
	if e.repo.beginArgs[0].Kind != "PRODUCT_ISSUE" || e.repo.beginArgs[0].Actor != pwEmail {
		t.Errorf("begin = %+v", e.repo.beginArgs[0])
	}
}

func TestProductWorkItem_KindSelectionAndLabels(t *testing.T) {
	for _, tc := range []struct {
		name       string
		mutate     func(*domain.CreateProductWorkItemRequest)
		wantKind   string
		wantLabels []string
		wantPrio   string
	}{
		{"patch is untracked and adds Type/Patch only", func(r *domain.CreateProductWorkItemRequest) {
			r.IssueTypeLabel = "Type/Patch"
			r.PriorityLevel = "Priority/High"
		},
			"UNTRACKED_PATCH", []string{"Origin/CS", "product/is", "Type/Patch"}, ""},
		{"discussion is a product issue with its priority", func(r *domain.CreateProductWorkItemRequest) { r.PriorityLevel = "Priority/Critical" },
			"PRODUCT_ISSUE", []string{"Origin/CS", "product/is", "Type/Incident", "Priority/Critical(P1)"}, "P1"},
		{"onboarding adds Affected/Onboarding", func(r *domain.CreateProductWorkItemRequest) { r.IssueTypeLabel = ""; r.OnboardingInProgress = true },
			"PRODUCT_ISSUE", []string{"Origin/CS", "product/is", "Affected/Onboarding"}, ""},
		{"a query type without a label is a product issue", func(r *domain.CreateProductWorkItemRequest) { r.IssueTypeLabel = "Type/Query"; r.Regression = true },
			"PRODUCT_ISSUE", []string{"Origin/CS", "product/is", "regression"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newPWEnv()
			req := pwReq()
			tc.mutate(&req)
			resp, _, err := e.create(t, req)
			if err != nil {
				t.Fatal(err)
			}
			if resp.Kind != tc.wantKind {
				t.Errorf("kind = %s, want %s", resp.Kind, tc.wantKind)
			}
			if patch := tc.wantKind == "UNTRACKED_PATCH"; patch != (len(e.repo.registered) == 0) || patch != slices.Equal(e.cases.calls, []string{"tag", "note"}) ||
				(!patch && !slices.Equal(e.cases.calls, []string{"note"})) {
				t.Errorf("registered=%d case calls=%v for %s (a patch: tag then note, no registration)", len(e.repo.registered), e.cases.calls, tc.wantKind)
			}
			if !slices.Equal(e.eng.createCalls[0].labels, tc.wantLabels) {
				t.Errorf("labels = %v, want %v", e.eng.createCalls[0].labels, tc.wantLabels)
			}
			if len(e.repo.registered) == 1 {
				if got := e.repo.registered[0].Priority; got != tc.wantPrio {
					t.Errorf("priority = %q, want %q", got, tc.wantPrio)
				}
			}
			if tc.wantKind == "UNTRACKED_PATCH" && resp.WorkItem != nil {
				t.Errorf("an untracked patch registers no work item: %+v", resp.WorkItem)
			}
			if e.repo.beginArgs[0].Kind != tc.wantKind {
				t.Errorf("request kind = %s", e.repo.beginArgs[0].Kind)
			}
		})
	}
}

func TestProductWorkItem_Validation(t *testing.T) {
	long := strings.Repeat("x", 257)
	for name, mutate := range map[string]func(*domain.CreateProductWorkItemRequest){
		"blank title":          func(r *domain.CreateProductWorkItemRequest) { r.Title = "  " },
		"title over 256 runes": func(r *domain.CreateProductWorkItemRequest) { r.Title = long },
		"no key":               func(r *domain.CreateProductWorkItemRequest) { r.IdempotencyKey = "" },
		"key over 200":         func(r *domain.CreateProductWorkItemRequest) { r.IdempotencyKey = strings.Repeat("k", 201) },
		"description too long": func(r *domain.CreateProductWorkItemRequest) { r.Description = strings.Repeat("d", 65537) },
		"bad case id":          func(r *domain.CreateProductWorkItemRequest) { r.CaseID = "nope" },
	} {
		e := newPWEnv()
		req := pwReq()
		mutate(&req)
		_, _, err := e.create(t, req)
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("%s: err = %v, want a validation error", name, err)
		}
		if len(e.repo.beginArgs) != 0 || len(e.eng.createCalls) != 0 {
			t.Errorf("%s: reached the database or GitHub", name)
		}
	}
	// A title of exactly 256 runes (multi-byte) is fine.
	e := newPWEnv()
	req := pwReq()
	req.Title = strings.Repeat("é", 256)
	if _, _, err := e.create(t, req); err != nil {
		t.Errorf("256 runes: %v", err)
	}
}

func TestProductWorkItem_Callers(t *testing.T) {
	e := newPWEnv()
	// No user email: 400.
	_, _, err := e.svc.CreateProductWorkItem(auth.WithIdentity(context.Background(), auth.Identity{Validated: true}), pwReq())
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("no email: %v", err)
	}
	// A customer: 403.
	cust := NewProductWorkItemService(e.eng, e.repo, fakePWMappings{}, e.cases, customerAccess{})
	_, _, err = cust.CreateProductWorkItem(pwCtx(), pwReq())
	var fe *apierror.ForbiddenError
	if !errors.As(err, &fe) {
		t.Errorf("customer: %v", err)
	}
	if len(e.repo.beginArgs) != 0 {
		t.Error("a refused caller reached the database")
	}
}

func TestProductWorkItem_CaseAndMapping(t *testing.T) {
	t.Run("case not found", func(t *testing.T) {
		e := newPWEnv()
		e.repo.loadErr = &apierror.NotFoundError{Msg: "case not found"}
		_, _, err := e.create(t, pwReq())
		var nfe *apierror.NotFoundError
		if !errors.As(err, &nfe) || len(e.repo.beginArgs) != 0 {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("closed case is a 409 and the new request is marked failed", func(t *testing.T) {
		e := newPWEnv()
		e.repo.kase.State = domain.CaseStateClosed
		_, _, err := e.create(t, pwReq())
		var ce *apierror.ConflictError
		if !errors.As(err, &ce) || len(e.eng.createCalls) != 0 || len(e.repo.outcomes) != 1 || !e.repo.outcomes[0].definite {
			t.Fatalf("err = %v outcomes=%+v", err, e.repo.outcomes)
		}
	})
	for _, st := range []domain.CaseState{domain.CaseStateOpen, domain.CaseStateWorkInProgress, domain.CaseStateWaitingOnWSO2, domain.CaseStateReopened} {
		e := newPWEnv()
		e.repo.kase.State = st
		if _, _, err := e.create(t, pwReq()); err != nil {
			t.Errorf("state %s: %v", st, err)
		}
	}
	t.Run("no mapped repository is a 400", func(t *testing.T) {
		e := newPWEnv()
		e.repo.kase.ProductName, e.repo.kase.DisplayName = "Other Product", "Other Product 1.0"
		_, _, err := e.create(t, pwReq())
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) || ve.Msg != "No GitHub repository is mapped for this product." || len(e.eng.createCalls) != 0 {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("a case with no product is a 400", func(t *testing.T) {
		e := newPWEnv()
		e.repo.kase.ProductName, e.repo.kase.DisplayName, e.repo.kase.ProductSlug = "", "", ""
		_, _, err := e.create(t, pwReq())
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("the display name is tried when the catalogue name has no mapping", func(t *testing.T) {
		e := newPWEnv()
		e.repo.kase.ProductName = "Unmapped"
		if _, _, err := e.create(t, pwReq()); err != nil {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("the Product line falls back through the display name, the catalogue name and the mapping label", func(t *testing.T) {
		for _, tc := range []struct{ slug, display, name, want string }{
			{"wso2is-6.1.0", "WSO2 Identity Server 6.1.0", "WSO2 Identity Server", "wso2is-6.1.0"},
			{"", "WSO2 Identity Server 6.1.0", "WSO2 Identity Server", "WSO2 Identity Server 6.1.0"},
			{"", "", "WSO2 Identity Server", "WSO2 Identity Server"},
		} {
			e := newPWEnv()
			e.repo.kase.ProductSlug, e.repo.kase.DisplayName, e.repo.kase.ProductName = tc.slug, tc.display, tc.name
			if _, _, err := e.create(t, pwReq()); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(e.eng.createCalls[0].body, "Product : "+tc.want+"\n") {
				t.Errorf("body = %q, want Product : %s", e.eng.createCalls[0].body, tc.want)
			}
		}
	})
}

func TestProductWorkItem_IdempotencyBranches(t *testing.T) {
	t.Run("a different request under the same key is a 409 and nothing happens", func(t *testing.T) {
		e := newPWEnv()
		e.repo.begin = repository.CreationRequest{ID: "r", RequestHash: "something-else", Status: repository.CreationStatusCompleted}
		_, _, err := e.create(t, pwReq())
		var ce *apierror.ConflictError
		if !errors.As(err, &ce) || ce.Code != apierror.CodeIdempotencyKeyReused || len(e.eng.createCalls) != 0 || len(e.repo.registered) != 0 {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("the hash ignores the key and repoOverride but not the content", func(t *testing.T) {
		a, b := pwReq(), pwReq()
		b.IdempotencyKey = "other"
		b.RepoOverride = &struct {
			Owner string `json:"owner"`
			Repo  string `json:"repo"`
		}{"x", "y"}
		if hashProductRequest(a) != hashProductRequest(b) {
			t.Error("key and repoOverride must not change the hash")
		}
		b.Title = "different"
		if hashProductRequest(a) == hashProductRequest(b) {
			t.Error("a different title must change the hash")
		}
	})
	t.Run("a completed request returns the stored response with created=false", func(t *testing.T) {
		e := newPWEnv()
		stored, _ := json.Marshal(domain.CreateProductWorkItemResponse{Message: "GitHub issue created.", Kind: "PRODUCT_ISSUE",
			Issue: domain.ProductWorkItemIssue{URL: "u", Number: 7, Repo: "o/r"}, WorkItem: &domain.ProductWorkItemRef{ID: "i", Number: "PI0000009"}})
		e.repo.begin = repository.CreationRequest{ID: "r", Status: repository.CreationStatusCompleted, Response: stored}
		resp, created, err := e.create(t, pwReq())
		if err != nil || created || resp.Issue.Number != 7 || resp.WorkItem.Number != "PI0000009" {
			t.Fatalf("resp=%+v created=%v err=%v", resp, created, err)
		}
		if len(e.eng.createCalls)+e.eng.detailCalls+len(e.eng.listCalls)+len(e.repo.registered) != 0 {
			t.Error("a completed request touched GitHub or the database again")
		}
	})
	t.Run("another attempt in flight is a 409 request_in_progress", func(t *testing.T) {
		e := newPWEnv()
		e.repo.begin = repository.CreationRequest{ID: "r", Status: repository.CreationStatusPending, InFlight: true}
		_, _, err := e.create(t, pwReq())
		var ce *apierror.ConflictError
		if !errors.As(err, &ce) || ce.Code != apierror.CodeRequestInProgress || len(e.eng.createCalls)+len(e.eng.listCalls) != 0 {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("GITHUB_CREATED skips GitHub's create and finishes registration, even if the case has since closed", func(t *testing.T) {
		e := newPWEnv()
		owner, repo, num, node := "example-org", "is", 42, "I_kw1"
		e.repo.begin = repository.CreationRequest{ID: "r", Status: repository.CreationStatusGitHubCreated, GitHubOwner: &owner, GitHubRepository: &repo, GitHubNumber: &num, GitHubNodeID: &node}
		e.repo.kase.State = domain.CaseStateClosed
		resp, created, err := e.create(t, pwReq())
		if err != nil || !created || resp.Issue.Number != 42 {
			t.Fatalf("resp=%+v created=%v err=%v", resp, created, err)
		}
		if len(e.eng.createCalls)+len(e.eng.listCalls) != 0 || len(e.repo.registered) != 1 || len(e.repo.recorded) != 0 {
			t.Errorf("create=%d list=%d registered=%d recorded=%d", len(e.eng.createCalls), len(e.eng.listCalls), len(e.repo.registered), len(e.repo.recorded))
		}
	})
	t.Run("a FAILED request is retried: it creates without searching for an earlier issue", func(t *testing.T) {
		e := newPWEnv()
		e.repo.begin = repository.CreationRequest{ID: "r", Status: repository.CreationStatusPending, RetryOfFailed: true}
		if _, created, err := e.create(t, pwReq()); err != nil || !created {
			t.Fatalf("created=%v err=%v", created, err)
		}
		if len(e.eng.createCalls) != 1 || len(e.eng.listCalls) != 0 {
			t.Errorf("create=%d list=%d", len(e.eng.createCalls), len(e.eng.listCalls))
		}
	})
	t.Run("a registration that was already completed concurrently returns the winner's response with created=false", func(t *testing.T) {
		e := newPWEnv()
		e.repo.replayed = true
		_, created, err := e.create(t, pwReq())
		if err != nil || created {
			t.Fatalf("created=%v err=%v", created, err)
		}
	})
}

func earlierIssue(num int, title, body string) engineeringentity.IssueDetail {
	return engineeringentity.IssueDetail{Number: num, NodeID: "I_old", Title: title, Body: &body, HTMLURL: "https://github.com/example-org/is/issues/" + string(rune('0'+num%10))}
}

func TestProductWorkItem_AmbiguousEarlierAttempt(t *testing.T) {
	pending := func(e *pwEnv) {
		note := "create ended without a clear answer"
		e.repo.begin = repository.CreationRequest{ID: "r", Status: repository.CreationStatusPending, Error: &note, CreatedOn: time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)}
	}
	footerBody := "Details.\n\n" + infoBlock("wso2is-6.1.0", "U12", "ACME-1", "CS0000001", "", "Sirius", pwEmail)

	t.Run("a matching issue is adopted, nothing is created", func(t *testing.T) {
		e := newPWEnv()
		pending(e)
		e.eng.list = []engineeringentity.IssueDetail{
			earlierIssue(40, "Another issue", footerBody),
			earlierIssue(41, "It fails", strings.Replace(footerBody, "Opened by : "+pwEmail, "Opened by : someone@else.com", 1)),
			earlierIssue(42, "It fails", footerBody),
		}
		e.eng.detail.Number = 42
		resp, created, err := e.create(t, pwReq())
		if err != nil || !created || resp.Issue.Number != 42 {
			t.Fatalf("resp=%+v created=%v err=%v", resp, created, err)
		}
		if len(e.eng.createCalls) != 0 || len(e.repo.recorded) != 1 || e.repo.recorded[0].Number != 42 || len(e.repo.registered) != 1 {
			t.Errorf("create=%d recorded=%+v registered=%d", len(e.eng.createCalls), e.repo.recorded, len(e.repo.registered))
		}
		q := e.eng.listCalls[0]
		if q.State != "all" || !slices.Equal(q.Labels, []string{"Origin/CS"}) || q.Since != "2026-11-18T09:59:00Z" {
			t.Errorf("search = %+v", q)
		}
	})
	t.Run("no match: the issue is created", func(t *testing.T) {
		e := newPWEnv()
		pending(e)
		e.eng.list = []engineeringentity.IssueDetail{earlierIssue(40, "Another issue", footerBody)}
		if _, _, err := e.create(t, pwReq()); err != nil {
			t.Fatal(err)
		}
		if len(e.eng.createCalls) != 1 {
			t.Errorf("create calls = %d", len(e.eng.createCalls))
		}
	})
	t.Run("an issue already recorded against another request is not adopted", func(t *testing.T) {
		e := newPWEnv()
		pending(e)
		e.eng.list = []engineeringentity.IssueDetail{earlierIssue(42, "It fails", footerBody)}
		e.repo.claimed = map[int]bool{42: true}
		if _, _, err := e.create(t, pwReq()); err != nil {
			t.Fatal(err)
		}
		if len(e.eng.createCalls) != 1 {
			t.Errorf("create calls = %d, want a new issue", len(e.eng.createCalls))
		}
	})
	t.Run("the footer lines must match exactly (a longer value or a quoted line is not the footer)", func(t *testing.T) {
		if bodyHasLines("WSO2 case id : ACME-12\nOpened by : a@b.c", "WSO2 case id : ACME-1", "Opened by : a@b.c") {
			t.Error("a prefix of the id matched")
		}
		if !bodyHasLines("x\r\nWSO2 case id : ACME-1\r\nOpened by : a@b.c", "WSO2 case id : ACME-1", "Opened by : a@b.c") {
			t.Error("CRLF lines did not match")
		}
	})
	t.Run("a failed search is a 502 and nothing is created (it could duplicate)", func(t *testing.T) {
		e := newPWEnv()
		pending(e)
		e.eng.listErr = &engineeringentity.Error{StatusCode: 502, Message: "boom"}
		_, _, err := e.create(t, pwReq())
		wantStatus(t, err, http.StatusBadGateway)
		if len(e.eng.createCalls) != 0 || len(e.repo.outcomes) != 1 || e.repo.outcomes[0].definite {
			t.Errorf("create=%d outcomes=%+v", len(e.eng.createCalls), e.repo.outcomes)
		}
	})
	t.Run("a request this call inserted never searches", func(t *testing.T) {
		e := newPWEnv()
		if _, _, err := e.create(t, pwReq()); err != nil {
			t.Fatal(err)
		}
		if len(e.eng.listCalls) != 0 {
			t.Error("searched for an earlier issue on a first attempt")
		}
	})
}

func TestProductWorkItem_EngineeringErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		name       string
		err        error
		wantStatus int
		definite   bool
		retryAfter time.Duration
	}{
		{"422 is a definite failure, 502", &engineeringentity.Error{StatusCode: 422, Message: "secret upstream words"}, 502, true, 0},
		{"403 is a definite failure, 502", &engineeringentity.Error{StatusCode: 403}, 502, true, 0},
		{"429 passes Retry-After through as 503 but is ambiguous for the row", &engineeringentity.Error{StatusCode: 429, RetryAfter: 12 * time.Second}, 503, false, 12 * time.Second},
		{"409 is ambiguous: 502, stays pending", &engineeringentity.Error{StatusCode: 409}, 502, false, 0},
		{"500 is ambiguous: 502, stays pending", &engineeringentity.Error{StatusCode: 500}, 502, false, 0},
		{"a timeout is ambiguous: 502, stays pending", &engineeringentity.Error{StatusCode: 0, Message: "request failed: deadline"}, 502, false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newPWEnv()
			e.eng.createErr = tc.err
			_, _, err := e.create(t, pwReq())
			ufe := wantStatus(t, err, tc.wantStatus)
			if ufe.RetryAfter != tc.retryAfter {
				t.Errorf("RetryAfter = %v", ufe.RetryAfter)
			}
			if strings.Contains(ufe.Msg, "secret") || strings.Contains(ufe.Error(), "upstream") {
				t.Errorf("the message carries upstream text: %q", ufe.Msg)
			}
			if len(e.repo.outcomes) != 1 || e.repo.outcomes[0].definite != tc.definite {
				t.Errorf("outcomes = %+v, want definite=%v", e.repo.outcomes, tc.definite)
			}
			if len(e.repo.recorded) != 0 || len(e.repo.registered) != 0 {
				t.Error("a failed create recorded or registered something")
			}
		})
	}
	t.Run("404 means the mapped repository is missing: a 400, definite", func(t *testing.T) {
		e := newPWEnv()
		e.eng.createErr = &engineeringentity.Error{StatusCode: 404}
		_, _, err := e.create(t, pwReq())
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) || len(e.repo.outcomes) != 1 || !e.repo.outcomes[0].definite {
			t.Fatalf("err=%v outcomes=%+v", err, e.repo.outcomes)
		}
	})
}

func TestProductWorkItem_AfterTheIssueExists(t *testing.T) {
	t.Run("a failed detail read builds the URL and leaves node_id to the create answer", func(t *testing.T) {
		e := newPWEnv()
		e.eng.detailErr = &engineeringentity.Error{StatusCode: 502}
		e.eng.createRef.NodeID = ""
		resp, _, err := e.create(t, pwReq())
		if err != nil {
			t.Fatal(err)
		}
		if resp.Issue.URL != "https://github.com/example-org/is/issues/42" || e.repo.registered[0].Issue.NodeID != "" || e.repo.registered[0].GitHubCreatedOn != nil {
			t.Errorf("resp=%+v params=%+v", resp, e.repo.registered[0])
		}
	})
	t.Run("a failed GITHUB_CREATED write is a 503 that says to retry; nothing is registered", func(t *testing.T) {
		e := newPWEnv()
		e.repo.recordErr = errors.New("db down")
		_, _, err := e.create(t, pwReq())
		var sue *apierror.ServiceUnavailableError
		if !errors.As(err, &sue) || len(e.repo.registered) != 0 {
			t.Fatalf("err = %v", err)
		}
		// The row must not keep the in-flight lease: it is marked ambiguous so the
		// retry runs the earlier-issue search.
		if len(e.repo.outcomes) != 1 || e.repo.outcomes[0].definite || !strings.Contains(e.repo.outcomes[0].note, "created but not recorded") {
			t.Fatalf("outcomes = %+v, want one ambiguous (not definite) note", e.repo.outcomes)
		}
	})
	t.Run("a failed registration is a 503 that says to retry", func(t *testing.T) {
		e := newPWEnv()
		e.repo.registerErr = errors.New("db down")
		_, _, err := e.create(t, pwReq())
		var sue *apierror.ServiceUnavailableError
		if !errors.As(err, &sue) {
			t.Fatalf("err = %v", err)
		}
		if len(e.repo.recorded) != 1 {
			t.Error("GITHUB_CREATED must be recorded before registration is tried")
		}
	})
	t.Run("a registration conflict (issue already linked) is passed through", func(t *testing.T) {
		e := newPWEnv()
		e.repo.registerErr = &apierror.ConflictError{Msg: "This GitHub issue is already linked to another work item."}
		_, _, err := e.create(t, pwReq())
		var ce *apierror.ConflictError
		if !errors.As(err, &ce) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestProductWorkItem_CaseServiceFailureLeavesTheRequestResumable(t *testing.T) {
	t.Run("a failed patch tag is a 503; no note, not completed", func(t *testing.T) {
		e := newPWEnv()
		e.cases.tagErr = errors.New("boom")
		req := pwReq()
		req.IssueTypeLabel = "Type/Patch"
		_, _, err := e.create(t, req)
		var sue *apierror.ServiceUnavailableError
		if !errors.As(err, &sue) || len(e.cases.notes) != 0 || len(e.repo.completed) != 0 || len(e.repo.recorded) != 1 {
			t.Fatalf("err=%v notes=%v completed=%d", err, e.cases.notes, len(e.repo.completed))
		}
	})
	t.Run("a failed note on a product issue is a 503; the work item stays registered, not completed", func(t *testing.T) {
		e := newPWEnv()
		e.cases.noteErr = errors.New("boom")
		_, _, err := e.create(t, pwReq())
		var sue *apierror.ServiceUnavailableError
		if !errors.As(err, &sue) || len(e.repo.registered) != 1 || len(e.repo.completed) != 0 {
			t.Fatalf("err=%v registered=%d completed=%d", err, len(e.repo.registered), len(e.repo.completed))
		}
	})
}

// A row inherited as PENDING (this call neither inserted it nor retried a FAILED
// one) may stand for an earlier attempt that created an issue. A refusal now must
// leave it PENDING (definite=false) so a later retry still searches before
// creating.
func TestProductWorkItem_InheritedAmbiguousRowIsNeverMarkedFailed(t *testing.T) {
	inherited := func(e *pwEnv) {
		note := "create ended without a clear answer"
		e.repo.begin = repository.CreationRequest{ID: "r", Status: repository.CreationStatusPending, Error: &note, CreatedOn: time.Now()}
	}
	for _, tc := range []struct {
		name  string
		setup func(e *pwEnv)
	}{
		{"case closed", func(e *pwEnv) { e.repo.kase.State = domain.CaseStateClosed }},
		{"repository not mapped", func(e *pwEnv) { e.repo.kase.ProductName, e.repo.kase.DisplayName = "Other", "Other 1" }},
		{"create answers 404", func(e *pwEnv) { e.eng.createErr = &engineeringentity.Error{StatusCode: 404} }},
		{"create answers 422", func(e *pwEnv) { e.eng.createErr = &engineeringentity.Error{StatusCode: 422} }},
		{"create answers 429", func(e *pwEnv) { e.eng.createErr = &engineeringentity.Error{StatusCode: 429} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newPWEnv()
			inherited(e)
			tc.setup(e)
			if _, _, err := e.create(t, pwReq()); err == nil {
				t.Fatal("want a refusal")
			}
			if len(e.repo.outcomes) == 0 {
				t.Fatal("no outcome recorded")
			}
			for _, o := range e.repo.outcomes {
				if o.definite {
					t.Fatalf("an inherited row was marked FAILED: %+v", o)
				}
			}
		})
	}
	t.Run("a row retried after FAILED is still marked FAILED by a definite refusal", func(t *testing.T) {
		e := newPWEnv()
		e.repo.begin = repository.CreationRequest{ID: "r", Status: repository.CreationStatusPending, RetryOfFailed: true, CreatedOn: time.Now()}
		e.repo.kase.State = domain.CaseStateClosed
		_, _, _ = e.create(t, pwReq())
		if len(e.repo.outcomes) != 1 || !e.repo.outcomes[0].definite {
			t.Fatalf("outcomes = %+v", e.repo.outcomes)
		}
	})
}

func TestProductWorkItem_BeginsWithBothLeases(t *testing.T) {
	e := newPWEnv()
	if _, _, err := e.create(t, pwReq()); err != nil {
		t.Fatal(err)
	}
	p := e.repo.beginArgs[0]
	if p.Lease != productRequestLease || p.AmbiguousLease != productAmbiguousLease || productAmbiguousLease != 60*time.Second || productRequestLease != 2*time.Minute {
		t.Fatalf("leases = %v / %v", p.Lease, p.AmbiguousLease)
	}
}
