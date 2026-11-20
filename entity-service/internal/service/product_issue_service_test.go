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
	"net/http"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/engineeringentity"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	piItem1 = "0190a4b0-0000-7000-8000-0000000000b1"
	piItem2 = "0190a4b0-0000-7000-8000-0000000000b2"
)

type fakePIRepo struct {
	kase    repository.ProductWorkItemCase
	loadErr error

	byIssue    []string // successive FindByGitHubIssue answers; the last repeats
	findCalls  int
	mappedOK   bool
	mappedErr  error
	mappedArgs [][2]string

	beingCreated    bool
	beingCreatedErr error

	linkCreated bool
	linkErr     error
	linkCalls   []string // "workItemID->caseID"
	registerErr error
	registered  []repository.ManualRegisterParams
	registerID  string
	unlinkOK    bool
	unlinkArgs  []string
}

func (f *fakePIRepo) LoadCase(context.Context, string) (repository.ProductWorkItemCase, error) {
	return f.kase, f.loadErr
}
func (f *fakePIRepo) ListForCase(context.Context, string) ([]domain.ProductIssueSummary, error) {
	return []domain.ProductIssueSummary{{ID: piItem1}}, nil
}
func (f *fakePIRepo) GetSummary(_ context.Context, id string) (domain.ProductIssueSummary, error) {
	return domain.ProductIssueSummary{ID: id, Number: "PI0000001"}, nil
}
func (f *fakePIRepo) GetDetail(context.Context, string) (domain.ProductIssueDetail, error) {
	return domain.ProductIssueDetail{}, nil
}
func (f *fakePIRepo) Search(context.Context, domain.SearchProductIssuesFilters, domain.Pagination) ([]domain.ProductIssueSummary, int, error) {
	return nil, 0, nil
}
func (f *fakePIRepo) FindByGitHubIssue(context.Context, string, string, int) (string, error) {
	i := f.findCalls
	f.findCalls++
	if len(f.byIssue) == 0 {
		return "", nil
	}
	if i >= len(f.byIssue) {
		i = len(f.byIssue) - 1
	}
	return f.byIssue[i], nil
}
func (f *fakePIRepo) MappedRepository(_ context.Context, o, r string) (string, string, bool, error) {
	f.mappedArgs = append(f.mappedArgs, [2]string{o, r})
	return "example-org", "is", f.mappedOK, f.mappedErr
}
func (f *fakePIRepo) IssueBeingCreated(context.Context, string, string, int) (bool, error) {
	return f.beingCreated, f.beingCreatedErr
}
func (f *fakePIRepo) LinkExisting(_ context.Context, wi, caseID, _ string) (bool, error) {
	f.linkCalls = append(f.linkCalls, wi+"->"+caseID)
	return f.linkCreated, f.linkErr
}
func (f *fakePIRepo) RegisterManualProductIssue(_ context.Context, p repository.ManualRegisterParams) (string, error) {
	f.registered = append(f.registered, p)
	return f.registerID, f.registerErr
}
func (f *fakePIRepo) Unlink(_ context.Context, caseID, wi, actor string) (bool, error) {
	f.unlinkArgs = []string{caseID, wi, actor}
	return f.unlinkOK, nil
}

type piEnv struct {
	svc  ProductIssueService
	eng  *fakePWEng
	repo *fakePIRepo
}

func newPIEnv() *piEnv {
	body := "Details.\n\nProduct : is-6.1.0\nPublic Git Issue : https://github.com/o/r/issues/1\nABT Team : Sirius\nOpened by : opener@wso2.com"
	eng := &fakePWEng{detail: engineeringentity.IssueDetail{
		NodeID: "I_n1", Number: 42, Title: " Broken thing ", Body: &body, State: "open",
		Labels: []string{"Origin/CS", "Type/Incident", "Priority/High(P2)"}, HTMLURL: "https://github.com/example-org/is/issues/42",
		CreatedAt: "2026-11-01T10:00:00Z", UpdatedAt: "2026-11-02T10:00:00Z",
	}}
	repo := &fakePIRepo{
		kase:     repository.ProductWorkItemCase{ID: pwCaseID, Number: "CS1", State: domain.CaseStateOpen},
		mappedOK: true, registerID: piItem1,
	}
	return &piEnv{svc: NewProductIssueService(eng, repo, internalAccess{}), eng: eng, repo: repo}
}

func (e *piEnv) link(req domain.LinkProductIssueRequest) (domain.ProductIssueSummary, bool, error) {
	req.CaseID = pwCaseID
	return e.svc.Link(pwCtx(), req)
}

func byIssue() domain.LinkProductIssueRequest {
	return domain.LinkProductIssueRequest{Owner: "WSO2-Enterprise", Repository: "IS", IssueNumber: 42}
}

func TestProductIssueLink_ByWorkItemID(t *testing.T) {
	t.Run("new link is created", func(t *testing.T) {
		e := newPIEnv()
		e.repo.linkCreated = true
		sum, created, err := e.link(domain.LinkProductIssueRequest{WorkItemID: piItem1})
		if err != nil || !created || sum.ID != piItem1 {
			t.Fatalf("created=%v err=%v sum=%+v", created, err, sum)
		}
		if len(e.repo.linkCalls) != 1 || e.repo.linkCalls[0] != piItem1+"->"+pwCaseID || e.eng.detailCalls != 0 {
			t.Errorf("link calls %v, github calls %d", e.repo.linkCalls, e.eng.detailCalls)
		}
	})
	t.Run("already linked to this case is idempotent", func(t *testing.T) {
		e := newPIEnv()
		e.repo.linkCreated = false
		_, created, err := e.link(domain.LinkProductIssueRequest{WorkItemID: piItem1})
		if err != nil || created {
			t.Fatalf("created=%v err=%v", created, err)
		}
	})
	t.Run("linked to another case is a 409 already_linked", func(t *testing.T) {
		e := newPIEnv()
		e.repo.linkErr = &apierror.ConflictError{Msg: "This product issue is already linked to case CS9.", Code: apierror.CodeAlreadyLinked}
		_, _, err := e.link(domain.LinkProductIssueRequest{WorkItemID: piItem1})
		var ce *apierror.ConflictError
		if !errors.As(err, &ce) || ce.Code != apierror.CodeAlreadyLinked || !strings.Contains(ce.Msg, "CS9") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("not a product issue is a 404", func(t *testing.T) {
		e := newPIEnv()
		e.repo.linkErr = &apierror.NotFoundError{Msg: "product issue not found"}
		_, _, err := e.link(domain.LinkProductIssueRequest{WorkItemID: piItem1})
		var nfe *apierror.NotFoundError
		if !errors.As(err, &nfe) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestProductIssueLink_ByIssue_KnownIssue(t *testing.T) {
	e := newPIEnv()
	e.repo.byIssue = []string{piItem2}
	e.repo.linkCreated = true
	sum, created, err := e.link(byIssue())
	if err != nil || !created || sum.ID != piItem2 {
		t.Fatalf("created=%v err=%v sum=%+v", created, err, sum)
	}
	if e.eng.detailCalls != 0 || len(e.repo.registered) != 0 || e.repo.linkCalls[0] != piItem2+"->"+pwCaseID {
		t.Errorf("known issue must not call GitHub or register: github=%d registered=%d links=%v", e.eng.detailCalls, len(e.repo.registered), e.repo.linkCalls)
	}
}

func TestProductIssueLink_RegistersFromGitHub(t *testing.T) {
	e := newPIEnv()
	sum, created, err := e.link(byIssue())
	if err != nil || !created || sum.ID != piItem1 {
		t.Fatalf("created=%v err=%v sum=%+v", created, err, sum)
	}
	if len(e.repo.registered) != 1 || len(e.repo.linkCalls) != 0 {
		t.Fatalf("registered=%d linkCalls=%v", len(e.repo.registered), e.repo.linkCalls)
	}
	p := e.repo.registered[0]
	if p.Issue.Owner != "example-org" || p.Issue.Repository != "is" || p.Issue.Number != 42 || p.Issue.NodeID != "I_n1" {
		t.Errorf("issue = %+v (the mapping's spelling is used)", p.Issue)
	}
	if p.CaseID != pwCaseID || p.Actor != pwEmail || p.Title != "Broken thing" || p.Priority != "P2" || p.Category != "Type/Incident" ||
		p.OpenedByEmail != "opener@wso2.com" || p.ABTTeam != "Sirius" || p.PublicIssueURL != "https://github.com/o/r/issues/1" {
		t.Errorf("params = %+v", p)
	}
	if p.State != "OPEN" || p.GitHubCreatedOn == nil || p.GitHubUpdatedOn == nil || p.GitHubClosedOn != nil {
		t.Errorf("state/timestamps = %s %v %v %v", p.State, p.GitHubCreatedOn, p.GitHubUpdatedOn, p.GitHubClosedOn)
	}
	if len(e.repo.mappedArgs) != 1 || e.repo.mappedArgs[0] != [2]string{"WSO2-Enterprise", "IS"} {
		t.Errorf("mapping lookup = %v", e.repo.mappedArgs)
	}
}

func TestProductIssueLink_RegisterRefusals(t *testing.T) {
	wantStatus400 := func(t *testing.T, err error, contains string) {
		t.Helper()
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) || !strings.Contains(ve.Msg, contains) {
			t.Fatalf("err = %v, want a 400 containing %q", err, contains)
		}
	}
	t.Run("Type/Patch", func(t *testing.T) {
		e := newPIEnv()
		e.eng.detail.Labels = []string{"Origin/CS", "Type/Patch"}
		_, _, err := e.link(byIssue())
		wantStatus400(t, err, "Patch")
		if len(e.repo.registered) != 0 {
			t.Error("registered a patch")
		}
	})
	t.Run("legacy patch label", func(t *testing.T) {
		e := newPIEnv()
		e.eng.detail.Labels = []string{"patch"}
		_, _, err := e.link(byIssue())
		wantStatus400(t, err, "Patch")
	})
	t.Run("pull request", func(t *testing.T) {
		e := newPIEnv()
		e.eng.detail.HTMLURL = "https://github.com/example-org/is/pull/42"
		_, _, err := e.link(byIssue())
		wantStatus400(t, err, "pull request")
	})
	t.Run("unmapped repository", func(t *testing.T) {
		e := newPIEnv()
		e.repo.mappedOK = false
		_, _, err := e.link(byIssue())
		wantStatus400(t, err, "This repository is not mapped to a product.")
		if e.eng.detailCalls != 0 {
			t.Error("called GitHub for an unmapped repository")
		}
	})
	t.Run("GitHub 404", func(t *testing.T) {
		e := newPIEnv()
		e.eng.detailErr = &engineeringentity.Error{StatusCode: http.StatusNotFound, Message: "x"}
		_, _, err := e.link(byIssue())
		var nfe *apierror.NotFoundError
		if !errors.As(err, &nfe) || nfe.Msg != "GitHub issue not found." {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("GitHub 5xx is a generic 502", func(t *testing.T) {
		e := newPIEnv()
		e.eng.detailErr = &engineeringentity.Error{StatusCode: 500, Message: "secret detail"}
		_, _, err := e.link(byIssue())
		var ufe *UpstreamFailureError
		if !errors.As(err, &ufe) || ufe.Status != http.StatusBadGateway || strings.Contains(ufe.Msg, "secret") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("GitHub rate limit is a 503", func(t *testing.T) {
		e := newPIEnv()
		e.eng.detailErr = &engineeringentity.Error{StatusCode: http.StatusTooManyRequests}
		_, _, err := e.link(byIssue())
		var ufe *UpstreamFailureError
		if !errors.As(err, &ufe) || ufe.Status != http.StatusServiceUnavailable {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestProductIssueLink_ClosedIssueKeepsItsState(t *testing.T) {
	e := newPIEnv()
	reason, closed := "completed", "2026-11-03T10:00:00Z"
	e.eng.detail.State, e.eng.detail.StateReason, e.eng.detail.ClosedAt = "closed", &reason, &closed
	if _, _, err := e.link(byIssue()); err != nil {
		t.Fatal(err)
	}
	p := e.repo.registered[0]
	if p.State != "CLOSED" || p.StateReason != "completed" || p.GitHubClosedOn == nil {
		t.Errorf("params = %+v", p)
	}
}

// Another request registered the issue between our lookup and our insert: the
// unique index refuses, and the link is made to the row that won, not a 500.
func TestProductIssueLink_RaceReReads(t *testing.T) {
	e := newPIEnv()
	e.repo.byIssue = []string{"", piItem2} // first lookup: unknown; the re-read finds the winner
	e.repo.registerErr = repository.ErrGitHubIssueAlreadyLinked
	e.repo.linkCreated = true
	sum, created, err := e.link(byIssue())
	if err != nil || !created || sum.ID != piItem2 {
		t.Fatalf("created=%v err=%v sum=%+v", created, err, sum)
	}
	if len(e.repo.linkCalls) != 1 || e.repo.linkCalls[0] != piItem2+"->"+pwCaseID {
		t.Errorf("link calls = %v", e.repo.linkCalls)
	}
	// The winner is linked to another case: that is the already_linked answer.
	e = newPIEnv()
	e.repo.byIssue = []string{"", piItem2}
	e.repo.registerErr = repository.ErrGitHubIssueAlreadyLinked
	e.repo.linkErr = &apierror.ConflictError{Msg: "linked to CS9", Code: apierror.CodeAlreadyLinked}
	if _, _, err := e.link(byIssue()); err == nil || !strings.Contains(err.Error(), "CS9") {
		t.Errorf("err = %v", err)
	}
	// Refused but nothing to re-read (node id clash): a 409, never a 500.
	e = newPIEnv()
	e.repo.registerErr = repository.ErrGitHubIssueAlreadyLinked
	_, _, err = e.link(byIssue())
	var ce *apierror.ConflictError
	if !errors.As(err, &ce) {
		t.Errorf("err = %v", err)
	}
}

func TestProductIssueLink_CaseAndBodyValidation(t *testing.T) {
	var ve *apierror.ValidationError
	var nfe *apierror.NotFoundError
	var ce *apierror.ConflictError

	e := newPIEnv()
	e.repo.kase.State = domain.CaseStateClosed
	if _, _, err := e.link(domain.LinkProductIssueRequest{WorkItemID: piItem1}); !errors.As(err, &ce) {
		t.Errorf("closed case: %v", err)
	}
	// Every other state may be linked.
	for _, st := range []domain.CaseState{domain.CaseStateAwaitingInfo, domain.CaseStateSolutionProposed, domain.CaseStateWaitingOnWSO2} {
		e = newPIEnv()
		e.repo.kase.State = st
		e.repo.linkCreated = true
		if _, _, err := e.link(domain.LinkProductIssueRequest{WorkItemID: piItem1}); err != nil {
			t.Errorf("state %s: %v", st, err)
		}
	}
	e = newPIEnv()
	e.repo.loadErr = &apierror.NotFoundError{Msg: "case not found"}
	if _, _, err := e.link(domain.LinkProductIssueRequest{WorkItemID: piItem1}); !errors.As(err, &nfe) {
		t.Errorf("not a case: %v", err)
	}
	for name, req := range map[string]domain.LinkProductIssueRequest{
		"neither":              {},
		"both":                 {WorkItemID: piItem1, Owner: "o", Repository: "r", IssueNumber: 1},
		"partial triple":       {Owner: "o", Repository: "r"},
		"zero issue number":    {Owner: "o", Repository: "r", IssueNumber: 0, WorkItemID: ""},
		"negative issue":       {Owner: "o", Repository: "r", IssueNumber: -3},
		"malformed workItemId": {WorkItemID: "nope"},
	} {
		e = newPIEnv()
		if _, _, err := e.link(req); !errors.As(err, &ve) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestProductIssueUnlink(t *testing.T) {
	e := newPIEnv()
	e.repo.unlinkOK = true
	if err := e.svc.Unlink(pwCtx(), pwCaseID, piItem1); err != nil {
		t.Fatal(err)
	}
	if len(e.repo.unlinkArgs) != 3 || e.repo.unlinkArgs[0] != pwCaseID || e.repo.unlinkArgs[1] != piItem1 || e.repo.unlinkArgs[2] != pwEmail {
		t.Errorf("unlink args = %v", e.repo.unlinkArgs)
	}
	e.repo.unlinkOK = false
	var nfe *apierror.NotFoundError
	if err := e.svc.Unlink(pwCtx(), pwCaseID, piItem1); !errors.As(err, &nfe) {
		t.Errorf("no active link: %v", err)
	}
}

func TestProductIssueReads_CallersAndFilters(t *testing.T) {
	e := newPIEnv()
	custSvc := NewProductIssueService(e.eng, e.repo, customerAccess{})
	var fe *apierror.ForbiddenError
	if _, err := custSvc.ListForCase(pwCtx(), pwCaseID); !errors.As(err, &fe) {
		t.Errorf("customer list: %v", err)
	}
	if _, _, err := custSvc.Link(pwCtx(), domain.LinkProductIssueRequest{CaseID: pwCaseID, WorkItemID: piItem1}); !errors.As(err, &fe) {
		t.Errorf("customer link: %v", err)
	}
	if err := custSvc.Unlink(pwCtx(), pwCaseID, piItem1); !errors.As(err, &fe) {
		t.Errorf("customer unlink: %v", err)
	}

	var ve *apierror.ValidationError
	bad := map[string]domain.SearchProductIssuesFilters{
		"githubState":    {GitHubState: "MERGED"},
		"boardStatus":    {BoardStatuses: []string{"OPEN", "NOPE"}},
		"caseId":         {CaseID: "x"},
		"case+unmatched": {CaseID: pwCaseID, Unmatched: true},
		"long query":     {Query: strings.Repeat("a", 201)},
	}
	for name, f := range bad {
		if _, err := e.svc.Search(pwCtx(), domain.SearchProductIssuesRequest{Filters: f}); !errors.As(err, &ve) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := e.svc.Search(pwCtx(), domain.SearchProductIssuesRequest{Pagination: domain.Pagination{Limit: 51}}); !errors.As(err, &ve) {
		t.Errorf("limit 51: %v", err)
	}
	resp, err := e.svc.Search(pwCtx(), domain.SearchProductIssuesRequest{Filters: domain.SearchProductIssuesFilters{GitHubState: "closed"}})
	if err != nil || resp.Limit != 20 || resp.Offset != 0 {
		t.Errorf("defaults: %+v %v", resp, err)
	}
}

// An issue a case's create request has recorded (PENDING or GITHUB_CREATED) but not
// finished registering must not be registered by a manual link: that would steal it.
func TestProductIssueLink_RefusesAnIssueBeingCreatedFromACase(t *testing.T) {
	e := newPIEnv()
	e.repo.beingCreated = true
	_, _, err := e.link(byIssue())
	var ce *apierror.ConflictError
	if !errors.As(err, &ce) || ce.Code != apierror.CodeRequestInProgress || !strings.Contains(ce.Msg, "retry shortly") {
		t.Fatalf("err = %v, want a 409 request_in_progress", err)
	}
	if len(e.repo.registered) != 0 || e.eng.detailCalls != 0 {
		t.Errorf("registered=%d github reads=%d: nothing may happen", len(e.repo.registered), e.eng.detailCalls)
	}
	// A failed check is an error, not a registration.
	e = newPIEnv()
	e.repo.beingCreatedErr = errors.New("db down")
	if _, _, err := e.link(byIssue()); err == nil || len(e.repo.registered) != 0 {
		t.Fatalf("err = %v registered=%d", err, len(e.repo.registered))
	}
}
