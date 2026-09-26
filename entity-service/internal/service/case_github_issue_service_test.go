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
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/github"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const cgiCaseID = "e0048c8e-1bd5-4a10-8d4e-64a2604bcb6e"

type fakeIssueCreator struct {
	owner, repo, title, body string
	labels                   []string
	calls                    int
	err                      error
}

func (f *fakeIssueCreator) CreateIssue(_ context.Context, owner, repo, title, body string, labels []string) (*github.CreatedIssue, error) {
	f.calls++
	f.owner, f.repo, f.title, f.body, f.labels = owner, repo, title, body, labels
	if f.err != nil {
		return nil, f.err
	}
	return &github.CreatedIssue{Number: 42, HTMLURL: "https://github.com/" + owner + "/" + repo + "/issues/42"}, nil
}

type fakeCGIRepo struct {
	accountID string
	mapping   *repository.RepoMapping
	linkErr   error
	linkedTo  int
	linkCalls int
}

func (f *fakeCGIRepo) AccountForCase(context.Context, string) (string, error) {
	return f.accountID, nil
}
func (f *fakeCGIRepo) RepoForAccount(context.Context, string) (*repository.RepoMapping, error) {
	return f.mapping, nil
}
func (f *fakeCGIRepo) SetCaseGithubIssueNumber(_ context.Context, _ string, n int) (bool, error) {
	f.linkCalls++
	if f.linkErr != nil {
		return false, f.linkErr
	}
	f.linkedTo = n
	return true, nil
}

// fakeCaseSvcForIssue supplies only what this service reads: the case's state.
type fakeCaseSvcForIssue struct {
	state domain.CaseState
	err   error
}

func (f *fakeCaseSvcForIssue) GetCaseByID(context.Context, string) (domain.CaseView, error) {
	if f.err != nil {
		return domain.CaseView{}, f.err
	}
	st := f.state
	return domain.CaseView{State: &st}, nil
}

func cgiRequest() domain.CreateCaseGithubIssueRequest {
	return domain.CreateCaseGithubIssueRequest{
		CaseID:      cgiCaseID,
		Reason:      domain.CaseGithubIssueReasonDefault,
		Title:       "Gateway returns 502 under load",
		Description: "Sustained 502s above 400 rps.",
	}
}

func cgiService(gh githubIssueCreator, repo caseGithubIssueRepo, state domain.CaseState) CaseGithubIssueService {
	return NewCaseGithubIssueService(gh, repo, &fakeCaseSvcForIssue{state: state}, DefaultGithubLabels())
}

func cgiMappedRepo() *fakeCGIRepo {
	return &fakeCGIRepo{
		accountID: "a1",
		mapping:   &repository.RepoMapping{AccountID: "a1", AccountName: "Acme", Owner: "wso2", Repository: "choreo"},
	}
}

func TestCreateCaseGithubIssue_FilesAndLinks(t *testing.T) {
	gh, repo := &fakeIssueCreator{}, cgiMappedRepo()
	resp, err := cgiService(gh, repo, domain.CaseStateOpen).
		CreateCaseGithubIssue(context.Background(), cgiRequest())
	if err != nil {
		t.Fatalf("CreateCaseGithubIssue: %v", err)
	}
	if gh.owner != "wso2" || gh.repo != "choreo" {
		t.Errorf("filed in %s/%s, want wso2/choreo", gh.owner, gh.repo)
	}
	if resp.Issue.Number != 42 || resp.Issue.Repo != "wso2/choreo" {
		t.Errorf("response = %+v", resp.Issue)
	}
	// The whole point: the case is now linked, which is what opens gate 2.
	if repo.linkedTo != 42 {
		t.Errorf("case linked to issue %d, want 42", repo.linkedTo)
	}
}

// An issue filed from a case must be recognisable by our own inbound sync,
// which keys on the [CR]: title prefix and a CR class label -- not on a type
// label, which no product repository applies.
func TestCreateCaseGithubIssue_IsRecognisableByOurOwnInbound(t *testing.T) {
	gh := &fakeIssueCreator{}
	if _, err := cgiService(gh, cgiMappedRepo(), domain.CaseStateOpen).
		CreateCaseGithubIssue(context.Background(), cgiRequest()); err != nil {
		t.Fatalf("CreateCaseGithubIssue: %v", err)
	}
	if !IsChangeRequestTitle(gh.title) {
		t.Errorf("title %q carries no [CR]: prefix, so our webhook would ignore it", gh.title)
	}
	if _, ok := DefaultGithubLabels().ClassOf(gh.labels); !ok {
		t.Errorf("labels %v carry no single CR class label", gh.labels)
	}
}

// A title that already has the prefix is not given a second one.
func TestCreateCaseGithubIssue_DoesNotDoublePrefix(t *testing.T) {
	gh := &fakeIssueCreator{}
	req := cgiRequest()
	req.Title = "[CR]: already prefixed"
	if _, err := cgiService(gh, cgiMappedRepo(), domain.CaseStateOpen).
		CreateCaseGithubIssue(context.Background(), req); err != nil {
		t.Fatalf("CreateCaseGithubIssue: %v", err)
	}
	if gh.title != "[CR]: already prefixed" {
		t.Errorf("title = %q", gh.title)
	}
}

func TestCreateCaseGithubIssue_RepoOverrideWins(t *testing.T) {
	gh, repo := &fakeIssueCreator{}, cgiMappedRepo()
	req := cgiRequest()
	req.RepoOverride = &domain.CaseGithubIssueRepoOverride{Owner: "wso2-enterprise", Repo: "apim-internal"}
	if _, err := cgiService(gh, repo, domain.CaseStateOpen).
		CreateCaseGithubIssue(context.Background(), req); err != nil {
		t.Fatalf("CreateCaseGithubIssue: %v", err)
	}
	if gh.owner != "wso2-enterprise" || gh.repo != "apim-internal" {
		t.Errorf("override ignored: filed in %s/%s", gh.owner, gh.repo)
	}
}

// An account nobody has seeded has nowhere to file. Rejected before GitHub is
// called, so a misconfiguration cannot leave a stray issue behind.
func TestCreateCaseGithubIssue_NoMappingIsRejectedBeforeFiling(t *testing.T) {
	gh := &fakeIssueCreator{}
	repo := &fakeCGIRepo{accountID: "a1", mapping: nil}
	_, err := cgiService(gh, repo, domain.CaseStateOpen).
		CreateCaseGithubIssue(context.Background(), cgiRequest())
	if err == nil {
		t.Fatal("want an error")
	}
	if gh.calls != 0 {
		t.Errorf("filed an issue despite having no repository: %d calls", gh.calls)
	}
}

// THE PARTIAL FAILURE. The issue exists; the link does not. The error must
// carry the issue URL and number, or the issue is orphaned and a retry files a
// duplicate.
func TestCreateCaseGithubIssue_LinkFailureNamesTheOrphanedIssue(t *testing.T) {
	gh := &fakeIssueCreator{}
	repo := cgiMappedRepo()
	repo.linkErr = errors.New("connection refused")

	_, err := cgiService(gh, repo, domain.CaseStateOpen).
		CreateCaseGithubIssue(context.Background(), cgiRequest())
	if err == nil {
		t.Fatal("want an error")
	}
	msg := err.Error()
	for _, want := range []string{"issues/42", "#42"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error does not name the orphaned issue (%q missing): %s", want, msg)
		}
	}
	if !strings.Contains(msg, "duplicate") {
		t.Errorf("error does not warn that retrying duplicates the issue: %s", msg)
	}
}

// Validation parity with the ServiceNow implementation: the caller must not be
// able to tell which one answered.
func TestCreateCaseGithubIssue_ValidationMatchesTheServiceNowPath(t *testing.T) {
	cases := map[string]func(*domain.CreateCaseGithubIssueRequest){
		"no case id":       func(r *domain.CreateCaseGithubIssueRequest) { r.CaseID = "" },
		"case id not uuid": func(r *domain.CreateCaseGithubIssueRequest) { r.CaseID = "nope" },
		"bad reason":       func(r *domain.CreateCaseGithubIssueRequest) { r.Reason = "whatever" },
		"no title":         func(r *domain.CreateCaseGithubIssueRequest) { r.Title = "" },
		"no description":   func(r *domain.CreateCaseGithubIssueRequest) { r.Description = "" },
		"half an override": func(r *domain.CreateCaseGithubIssueRequest) {
			r.RepoOverride = &domain.CaseGithubIssueRepoOverride{Owner: "wso2"}
		},
	}
	for name, mangle := range cases {
		t.Run(name, func(t *testing.T) {
			gh := &fakeIssueCreator{}
			req := cgiRequest()
			mangle(&req)
			if _, err := cgiService(gh, cgiMappedRepo(), domain.CaseStateOpen).
				CreateCaseGithubIssue(context.Background(), req); err == nil {
				t.Fatal("want a validation error")
			}
			if gh.calls != 0 {
				t.Errorf("an invalid request reached GitHub")
			}
		})
	}
}

// The state gate, same set the ServiceNow API enforces.
func TestCreateCaseGithubIssue_ClosedCaseIsRefused(t *testing.T) {
	gh := &fakeIssueCreator{}
	_, err := cgiService(gh, cgiMappedRepo(), domain.CaseStateClosed).
		CreateCaseGithubIssue(context.Background(), cgiRequest())
	if err == nil {
		t.Fatal("want a conflict for a closed case")
	}
	if gh.calls != 0 {
		t.Errorf("filed an issue for a closed case")
	}
}
