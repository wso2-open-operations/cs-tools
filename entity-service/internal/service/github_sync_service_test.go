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
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/github"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const ghIssueURL = "https://github.com/wso2/choreo/issues/42"

type fakeGhRepo struct {
	claimed    map[string]bool
	linked     map[string]string
	mappingErr error
	accountID  string
	caseID     string
	linkErr    error
	mapping    *repository.RepoMapping
	cr         *repository.GithubChangeRequest
}

func (f *fakeGhRepo) RepoMapping(context.Context, string, string) (*repository.RepoMapping, error) {
	return f.mapping, f.mappingErr
}
func (f *fakeGhRepo) ChangeRequestByGitReference(context.Context, string) (*repository.GithubChangeRequest, error) {
	return f.cr, nil
}

// Behaves like the table it stands in for: a second claim on the same id is
// refused. A fake that always returned nil is why the missing claim call went
// unnoticed.
func (f *fakeGhRepo) RepoForAccount(context.Context, string) (*repository.RepoMapping, error) {
	return f.mapping, f.mappingErr
}
func (f *fakeGhRepo) CaseByIssueNumber(context.Context, string, int) (string, error) {
	return f.caseID, nil
}
func (f *fakeGhRepo) AccountForCase(context.Context, string) (string, error) {
	return f.accountID, nil
}
func (f *fakeGhRepo) SetCaseGithubIssueNumber(_ context.Context, caseID string, n int) (bool, error) {
	if f.linkErr != nil {
		return false, f.linkErr
	}
	if f.linked == nil {
		f.linked = map[string]string{}
	}
	f.linked[caseID] = fmt.Sprint(n)
	return true, nil
}

func (f *fakeGhRepo) ClaimDelivery(_ context.Context, id, _, _ string) error {
	if f.claimed == nil {
		f.claimed = map[string]bool{}
	}
	if f.claimed[id] {
		return repository.ErrDeliverySeen
	}
	f.claimed[id] = true
	return nil
}

func (f *fakeGhRepo) ReleaseDelivery(_ context.Context, id string) error {
	delete(f.claimed, id)
	return nil
}

func (f *fakeGhRepo) LinkDelivery(_ context.Context, id, crID string) error {
	if f.linked == nil {
		f.linked = map[string]string{}
	}
	f.linked[id] = crID
	return nil
}

type fakeGhClient struct {
	comments []string
	states   []github.State
	labels   [][]string
	removed  []string
	added    []string
}

func (f *fakeGhClient) CreateComment(_ context.Context, _ github.Issue, body string) (*github.Comment, error) {
	f.comments = append(f.comments, body)
	return &github.Comment{}, nil
}
func (f *fakeGhClient) SetLabels(_ context.Context, _ github.Issue, l []string) error {
	f.labels = append(f.labels, l)
	return nil
}
func (f *fakeGhClient) AddLabel(_ context.Context, _ github.Issue, l string) error {
	f.added = append(f.added, l)
	return nil
}
func (f *fakeGhClient) RemoveLabel(_ context.Context, _ github.Issue, l string) error {
	f.removed = append(f.removed, l)
	return nil
}
func (f *fakeGhClient) SetState(_ context.Context, _ github.Issue, s github.State) error {
	f.states = append(f.states, s)
	return nil
}

// fakeGhMutations records what the sync would have written.
type fakeGhMutations struct {
	// srAlreadyExists makes the next service-request insert lose the race.
	srAlreadyExists bool
	created         int
	lastSR          repository.NewServiceRequestFromIssue
	lastCreate      repository.NewChangeRequestFromIssue
	lastUpdate      repository.NewChangeRequestFromIssue
	lastComment     string
	lastCommentOn   string
}

func (f *fakeGhMutations) CreateServiceRequestFromIssue(_ context.Context, in repository.NewServiceRequestFromIssue) (string, string, error) {
	f.created++
	f.lastSR = in
	// Stands in for the unique index catching a concurrent delivery: the
	// insert loses the race and reports the record that won.
	if f.srAlreadyExists {
		return "sr-winner", "", repository.ErrAlreadyExists()
	}
	return "sr-new", "CS-GH-000001", nil
}
func (f *fakeGhMutations) CreateFromIssue(_ context.Context, in repository.NewChangeRequestFromIssue) (string, string, error) {
	f.created++
	f.lastCreate = in
	return "cr-new", "CHG-GH-000001", nil
}
func (f *fakeGhMutations) UpdateFromIssue(_ context.Context, _ string, in repository.NewChangeRequestFromIssue) error {
	f.lastUpdate = in
	return nil
}
func (f *fakeGhMutations) SetState(context.Context, string, string) (bool, error) { return true, nil }
func (f *fakeGhMutations) AddComment(_ context.Context, target, content, _ string) error {
	f.lastCommentOn = target
	f.lastComment = content
	return nil
}
func (f *fakeGhMutations) SetAssignee(context.Context, string, string) (bool, error) {
	return true, nil
}
func (f *fakeGhMutations) UserIDForGithubLogin(context.Context, string) (string, error) {
	return "", nil
}

// ghDelivery builds one webhook. The title carries the [CR]: prefix because
// that -- not a label -- is what marks an issue as a change request.
//
// validation-passed is added unless the caller already supplied it: nothing is
// created before the repository's validation workflow has passed the issue, so
// every fixture that expects a record needs it. Tests for the gate itself build
// their payload without going through here.
func ghDelivery(event, action, title string, labels []string) Delivery {
	if !slices.Contains(labels, DefaultGithubLabels().ValidationPassed) {
		labels = append(labels, DefaultGithubLabels().ValidationPassed)
	}
	var p IssuePayload
	p.Action = action
	p.Issue.HTMLURL = ghIssueURL
	p.Issue.Number = 42
	p.Issue.Title = title
	p.Issue.Body = "Certificates expire on the 30th."
	p.Repository.Name = "choreo"
	p.Repository.Owner.Login = "wso2"
	p.Sender.Login = "a-human"
	for _, l := range labels {
		p.Issue.Labels = append(p.Issue.Labels, struct {
			Name string `json:"name"`
		}{Name: l})
	}
	return Delivery{ID: "d1", Event: event, Payload: p}
}

// commentDelivery builds an issue_comment webhook with the comment object
// actually present -- it is a pointer on the payload, so a test that only sets
// fields on it dereferences nil.
func commentDelivery(body, author string) Delivery {
	d := ghDelivery("issue_comment", "created", "[CR]: x", []string{"CR/NormalChange"})
	d.Payload.Comment = &struct {
		Body    string      `json:"body"`
		HTMLURL string      `json:"html_url"`
		User    github.User `json:"user"`
	}{Body: body}
	d.Payload.Comment.User.Login = author
	return d
}

func crDelivery() Delivery {
	return ghDelivery("issues", "labeled", "[CR]: rotate gateway certificates",
		[]string{"CR/NormalChange"})
}

func newGhSvc(r *fakeGhRepo, c *fakeGhClient) GithubSyncService {
	return NewGithubSyncService(r, c, "wso2-integration-bot")
}

func writingSvc(r *fakeGhRepo, m *fakeGhMutations, c *fakeGhClient) GithubSyncService {
	return NewGithubSyncServiceWriting(r, m, c, "wso2-integration-bot", DefaultGithubLabels())
}

func mapped() *repository.RepoMapping {
	return &repository.RepoMapping{
		AccountID: "a1", AccountName: "Choreo Customer",
		CredentialRef: "gh-choreo", Owner: "wso2", Repository: "choreo",
	}
}

// RECOGNITION IS BY TITLE. issue_servicenow.yml: "Change requests carry no
// template label -- the [CR]:/[ECR]: title is the only signal." An earlier
// version of this service gated on a Type/ChangeRequest label that nothing in
// any product repository applies, so it would never have matched a real issue.
func TestHandleWebhook_RecognisesByTitlePrefix(t *testing.T) {
	cases := map[string]struct {
		title   string
		labels  []string
		creates bool
	}{
		"[CR]: is a change request":     {"[CR]: rotate certs", []string{"CR/NormalChange"}, true},
		"[ECR]: is one too":             {"[ECR]: restore the gateway", []string{"CR/EmergencyChange"}, true},
		"no prefix is not":              {"rotate certs", []string{"CR/NormalChange"}, false},
		"an incident is not, yet":       {"gateway down", []string{"Type/Incident"}, false},
		"a service request IS one":      {"please add a user", []string{"Type/ServiceRequest"}, true},
		"prefix without a class waits":  {"[CR]: rotate certs", nil, false},
		"prefix with two classes waits": {"[CR]: rotate certs", []string{"CR/NormalChange", "CR/StandardChange"}, false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r := &fakeGhRepo{mapping: mapped()}
			m := &fakeGhMutations{}
			_, err := writingSvc(r, m, &fakeGhClient{}).
				HandleWebhook(context.Background(), ghDelivery("issues", "labeled", c.title, c.labels))
			if err != nil {
				t.Fatalf("HandleWebhook: %v", err)
			}
			if got := m.created > 0; got != c.creates {
				t.Errorf("created = %v, want %v", got, c.creates)
			}
		})
	}
}

// The class label decides the change request's type.
func TestHandleWebhook_ClassLabelSetsTheType(t *testing.T) {
	for label, want := range map[string]string{
		"CR/NormalChange":    "Normal Change",
		"CR/StandardChange":  "Standard Change",
		"CR/EmergencyChange": "Emergency Change",
	} {
		t.Run(label, func(t *testing.T) {
			r := &fakeGhRepo{mapping: mapped()}
			m := &fakeGhMutations{}
			_, err := writingSvc(r, m, &fakeGhClient{}).HandleWebhook(context.Background(),
				ghDelivery("issues", "labeled", "[CR]: x", []string{label}))
			if err != nil {
				t.Fatalf("HandleWebhook: %v", err)
			}
			if m.lastSR.SRType != want {
				t.Errorf("sr_type = %q, want %q", m.lastSR.SRType, want)
			}
			if m.lastSR.Catalog != CatalogGenericRequests {
				t.Errorf("catalog = %q, want %q", m.lastSR.Catalog, CatalogGenericRequests)
			}
		})
	}
}

// An unmapped repository is not ours. The mapping table is the allow-list.
func TestHandleWebhook_UnmappedRepositoryIsIgnored(t *testing.T) {
	r := &fakeGhRepo{mapping: nil}
	m := &fakeGhMutations{}
	out, err := writingSvc(r, m, &fakeGhClient{}).HandleWebhook(context.Background(), crDelivery())
	if err != nil {
		t.Fatalf("HandleWebhook: %v", err)
	}
	if out.Skipped == "" || m.created != 0 {
		t.Errorf("acted on an unmapped repository: %+v", out)
	}
}

// Our own writes come back as webhooks; dropping them by sender identity is
// what stops a comment we posted syncing back as a new one.
// An issue this service raised from a case must not come back as a new service
// request. The guard is the issue's AUTHOR, not whoever triggered the delivery:
// see isOwnIssue for why the two differ and what the difference cost.
func TestHandleWebhook_OwnEventsAreDropped(t *testing.T) {
	d := crDelivery()
	d.Payload.Issue.User.Login = "wso2-integration-bot"
	d.Payload.Sender.Login = "wso2-integration-bot"
	r := &fakeGhRepo{mapping: mapped()}
	m := &fakeGhMutations{}
	out, err := writingSvc(r, m, &fakeGhClient{}).HandleWebhook(context.Background(), d)
	if err != nil {
		t.Fatalf("HandleWebhook: %v", err)
	}
	if out.Skipped == "" || m.created != 0 {
		t.Errorf("acted on an issue we raised ourselves: %+v", out)
	}
}

func TestHandleWebhook_ReplayedDeliveryIsRefused(t *testing.T) {
	r := &fakeGhRepo{mapping: mapped()}
	svc := writingSvc(r, &fakeGhMutations{}, &fakeGhClient{})
	if _, err := svc.HandleWebhook(context.Background(), crDelivery()); err != nil {
		t.Fatalf("first delivery: %v", err)
	}
	if _, err := svc.HandleWebhook(context.Background(), crDelivery()); !errors.Is(err, repository.ErrDeliverySeen) {
		t.Fatalf("replay was processed again; err = %v", err)
	}
}

func TestHandleWebhook_FailedDeliveryReleasesItsClaim(t *testing.T) {
	r := &fakeGhRepo{mappingErr: errors.New("database is down")}
	d := crDelivery()
	if _, err := writingSvc(r, &fakeGhMutations{}, &fakeGhClient{}).
		HandleWebhook(context.Background(), d); err == nil {
		t.Fatal("want the underlying failure")
	}
	if r.claimed[d.ID] {
		t.Error("the claim survived a failure, so GitHub's retry would be refused as a replay")
	}
}

// A comment is relayed with the same "(GitHub Comment)" marker
// github_comment_to_sn.yml uses, which is what sn_comment_to_github.yml checks
// before posting back. Same marker, same loop closed.
func TestHandleWebhook_CommentIsRelayedWithTheLoopMarker(t *testing.T) {
	r := &fakeGhRepo{mapping: mapped(), caseID: "case-1", cr: &repository.GithubChangeRequest{ID: "cr-1"}}
	m := &fakeGhMutations{}
	d := commentDelivery("Scheduled for Friday.", "nimal")

	if _, err := writingSvc(r, m, &fakeGhClient{}).HandleWebhook(context.Background(), d); err != nil {
		t.Fatalf("HandleWebhook: %v", err)
	}
	if !strings.Contains(m.lastComment, "(GitHub Comment)") {
		t.Errorf("loop marker missing: %q", m.lastComment)
	}
	if !strings.Contains(m.lastComment, "Scheduled for Friday.") {
		t.Errorf("comment body lost: %q", m.lastComment)
	}
}

// Slash commands belong to the repository's own workflows.
func TestHandleWebhook_SlashCommandsAreNotRelayed(t *testing.T) {
	r := &fakeGhRepo{mapping: mapped(), caseID: "case-1", cr: &repository.GithubChangeRequest{ID: "cr-1"}}
	m := &fakeGhMutations{}
	d := commentDelivery("/close", "nimal")
	if _, err := writingSvc(r, m, &fakeGhClient{}).HandleWebhook(context.Background(), d); err != nil {
		t.Fatalf("HandleWebhook: %v", err)
	}
	if m.lastComment != "" {
		t.Errorf("relayed a slash command: %q", m.lastComment)
	}
}

// THE ROUND TRIP. A comment from GitHub must land on the CASE, because that is
// the only record the outbound trigger watches. Attaching it to the change
// request left a conversation that could not be answered: a reply on the
// change request synced nowhere.
func TestHandleWebhook_RelayedCommentLandsOnTheCase(t *testing.T) {
	r := &fakeGhRepo{mapping: mapped(), caseID: "case-1",
		cr: &repository.GithubChangeRequest{ID: "cr-1"}}
	m := &fakeGhMutations{}
	if _, err := writingSvc(r, m, &fakeGhClient{}).
		HandleWebhook(context.Background(), commentDelivery("from github", "nimal")); err != nil {
		t.Fatalf("HandleWebhook: %v", err)
	}
	if m.lastCommentOn != "case-1" {
		t.Errorf("comment landed on %q, want the case -- outbound will never see it otherwise", m.lastCommentOn)
	}
}

// An issue nobody linked to a case has nowhere to put the comment.
func TestHandleWebhook_CommentWithoutALinkedCaseIsSkipped(t *testing.T) {
	r := &fakeGhRepo{mapping: mapped(), caseID: ""}
	m := &fakeGhMutations{}
	out, err := writingSvc(r, m, &fakeGhClient{}).
		HandleWebhook(context.Background(), commentDelivery("from github", "nimal"))
	if err != nil {
		t.Fatalf("HandleWebhook: %v", err)
	}
	if out.Skipped == "" || m.lastComment != "" {
		t.Errorf("wrote a comment with no linked case: %+v", out)
	}
}

// THE LOOP THIS CLOSES. A CSM comment is dispatched, a workflow posts it on
// the issue as github-actions[bot], and GitHub sends that back as an
// issue_comment. Relaying it onto the case would make it a new case comment,
// which the outbound trigger dispatches again.
//
// Before the dispatch rewrite this service posted comments itself, so the
// sender coming back was the configured integration account and the existing
// guard caught it. Now the workflow posts, and the sender is never that
// account -- observed live: three comment_added rows in thirteen seconds.
func TestHandleWebhook_WorkflowsOwnCommentsAreDropped(t *testing.T) {
	for _, sender := range []string{"github-actions[bot]", "GitHub-Actions[bot]", "wso2-integration-bot"} {
		t.Run(sender, func(t *testing.T) {
			r := &fakeGhRepo{mapping: mapped(), caseID: "case-1"}
			m := &fakeGhMutations{}
			d := commentDelivery("[#CS-1](https://csm/cases/1)\n\nsomething", "x")
			d.Payload.Sender.Login = sender

			out, err := writingSvc(r, m, &fakeGhClient{}).HandleWebhook(context.Background(), d)
			if err != nil {
				t.Fatalf("HandleWebhook: %v", err)
			}
			if out.Skipped == "" || m.lastComment != "" {
				t.Errorf("relayed our own echo back onto the case: %+v", out)
			}
		})
	}
}

// The text marker is the second line of defence, for a repository whose
// workflow posts under some other account.
func TestHandleWebhook_RelayedMarkerIsDroppedWhoeverSentIt(t *testing.T) {
	r := &fakeGhRepo{mapping: mapped(), caseID: "case-1"}
	m := &fakeGhMutations{}
	d := commentDelivery("@someone (GitHub Comment) https://github.com/x/y/issues/1\n\nechoed", "a-human")

	out, err := writingSvc(r, m, &fakeGhClient{}).HandleWebhook(context.Background(), d)
	if err != nil {
		t.Fatalf("HandleWebhook: %v", err)
	}
	if out.Skipped == "" || m.lastComment != "" {
		t.Errorf("a comment already carrying the relay marker was relayed again: %+v", out)
	}
}

// A person's comment still gets through.
func TestHandleWebhook_HumanCommentStillRelays(t *testing.T) {
	r := &fakeGhRepo{mapping: mapped(), caseID: "case-1"}
	m := &fakeGhMutations{}
	if _, err := writingSvc(r, m, &fakeGhClient{}).
		HandleWebhook(context.Background(), commentDelivery("a real question", "nimal")); err != nil {
		t.Fatalf("HandleWebhook: %v", err)
	}
	if m.lastCommentOn != "case-1" {
		t.Errorf("a human comment was dropped")
	}
}

// The catalog an issue routes to, per issue_servicenow.yml.
func TestHandleWebhook_CatalogFollowsTheIssueKind(t *testing.T) {
	cases := map[string]struct{ title, label, catalog string }{
		"[CR]: goes to Generic Requests": {"[CR]: rotate certs", "CR/NormalChange", CatalogGenericRequests},
		"a service request to General":   {"please add a user", "Type/ServiceRequest", CatalogGeneralRequests},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r := &fakeGhRepo{mapping: mapped()}
			m := &fakeGhMutations{}
			if _, err := writingSvc(r, m, &fakeGhClient{}).HandleWebhook(context.Background(),
				ghDelivery("issues", "labeled", c.title, []string{c.label})); err != nil {
				t.Fatalf("HandleWebhook: %v", err)
			}
			if m.lastSR.Catalog != c.catalog {
				t.Errorf("catalog = %q, want %q", m.lastSR.Catalog, c.catalog)
			}
		})
	}
}

// The template's fields travel with the record, as the u_-prefixed keys
// service_request.json_data already holds for records raised elsewhere.
func TestHandleWebhook_TemplateFieldsAreCaptured(t *testing.T) {
	r := &fakeGhRepo{mapping: mapped()}
	m := &fakeGhMutations{}
	d := ghDelivery("issues", "labeled", "[CR]: rotate certs", []string{"CR/NormalChange"})
	d.Payload.Issue.Body = "### Priority\n\nCritical\n\n### Environment Details\n\nProduction"
	if _, err := writingSvc(r, m, &fakeGhClient{}).HandleWebhook(context.Background(), d); err != nil {
		t.Fatalf("HandleWebhook: %v", err)
	}
	if m.lastSR.Fields["u_priority"] != "Critical" {
		t.Errorf("u_priority = %q", m.lastSR.Fields["u_priority"])
	}
	if m.lastSR.Fields["u_environment_details"] != "Production" {
		t.Errorf("u_environment_details = %q", m.lastSR.Fields["u_environment_details"])
	}
}

// The link is set when the record is created, not afterwards: the outbound
// trigger fires on the INSERT, so a link written later loses the first event.
func TestHandleWebhook_IssueNumberTravelsWithCreation(t *testing.T) {
	r := &fakeGhRepo{mapping: mapped()}
	m := &fakeGhMutations{}
	if _, err := writingSvc(r, m, &fakeGhClient{}).HandleWebhook(context.Background(),
		ghDelivery("issues", "labeled", "[CR]: x", []string{"CR/NormalChange"})); err != nil {
		t.Fatalf("HandleWebhook: %v", err)
	}
	if m.lastSR.IssueNumber != 42 {
		t.Errorf("issue number = %d, want 42", m.lastSR.IssueNumber)
	}
	if m.lastSR.AccountID != "a1" {
		t.Errorf("account = %q, want the mapped one", m.lastSR.AccountID)
	}
}

// Opening one GitHub issue produces several deliveries (opened, then labeled),
// and they arrive close enough together that both can pass an existence check
// before either insert lands -- observed live as two service requests 480ms
// apart for a single issue. The unique index is what actually prevents the
// duplicate; this asserts the losing writer reports the winner rather than
// failing the caller.
func TestCreateServiceRequestFromIssue_ConcurrentDeliveryReportsWinner(t *testing.T) {
	mut := &fakeGhMutations{srAlreadyExists: true}
	svc := writingSvc(&fakeGhRepo{mapping: mapped()}, mut, &fakeGhClient{})

	resp, err := svc.CreateServiceRequestFromIssue(context.Background(),
		domain.CreateServiceRequestFromIssueRequest{
			Owner: "acme", Repository: "widgets", IssueNumber: 42,
			Title: "[CR]: something", Labels: []string{"CR/NormalChange", "validation-passed"},
		})
	if err != nil {
		t.Fatalf("losing the insert race must not fail the caller: %v", err)
	}
	if resp.Created {
		t.Error("created must be false when another delivery created the record")
	}
	if resp.ID != "sr-winner" {
		t.Errorf("must report the winning record, got id %q", resp.ID)
	}
	if resp.Number != "" {
		t.Errorf("number is unknown on this path, got %q", resp.Number)
	}
}

// A record must not exist before the repository's validation workflow has
// passed the issue. The workflow decides catalog and type from the template, so
// acting first produces a record built from fields nobody checked -- and the
// announcement comment would land above the validation result, which is how
// this was noticed.
func TestHandleWebhook_UnvalidatedIssueCreatesNothing(t *testing.T) {
	d := ghDelivery("issues", "opened", "[CR]: rotate gateway certificates",
		[]string{"CR/NormalChange"})
	// Drop the label ghDelivery adds, leaving only the class label.
	d.Payload.Issue.Labels = d.Payload.Issue.Labels[:1]

	mut := &fakeGhMutations{}
	svc := writingSvc(&fakeGhRepo{mapping: mapped()}, mut, &fakeGhClient{})

	out, err := svc.HandleWebhook(context.Background(), d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mut.created != 0 {
		t.Errorf("created %d record(s) before validation passed", mut.created)
	}
	if !strings.Contains(out.Skipped, "validation") {
		t.Errorf("skip reason should name validation, got %q", out.Skipped)
	}
}

// The same issue, once the workflow has labelled it, is created. Without this
// the gate above could pass simply by never creating anything.
func TestHandleWebhook_ValidatedIssueIsCreated(t *testing.T) {
	d := ghDelivery("issues", "labeled", "[CR]: rotate gateway certificates",
		[]string{"CR/NormalChange"})

	mut := &fakeGhMutations{}
	svc := writingSvc(&fakeGhRepo{mapping: mapped()}, mut, &fakeGhClient{})

	if _, err := svc.HandleWebhook(context.Background(), d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mut.created != 1 {
		t.Errorf("a validated issue must create exactly one record, got %d", mut.created)
	}
}

// The repository's validation workflow applies validation-passed, so GitHub
// reports that label change as issues/labeled sent by github-actions[bot] --
// the integration account. Guarding every event on sender identity discarded
// exactly the delivery the validation gate waits for, and the issue passed
// validation while no record was ever created. Observed on a live delivery;
// the existing fixtures missed it because they send as a human.
func TestHandleWebhook_ValidationLabelFromTheBotStillCreates(t *testing.T) {
	d := ghDelivery("issues", "labeled", "[CR]: rotate gateway certificates",
		[]string{"CR/NormalChange"})
	d.Payload.Sender.Login = "github-actions[bot]"

	mut := &fakeGhMutations{}
	svc := writingSvc(&fakeGhRepo{mapping: mapped()}, mut, &fakeGhClient{})

	out, err := svc.HandleWebhook(context.Background(), d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Skipped != "" {
		t.Fatalf("the validation label must not be discarded as our own event, got skip %q", out.Skipped)
	}
	if mut.created != 1 {
		t.Errorf("expected the record to be created, got %d creations", mut.created)
	}
}

// The other half of the same rule: a comment from the integration account is
// still an echo of something we posted, and must never become a second CSM
// comment.
func TestHandleWebhook_CommentFromTheBotIsStillIgnored(t *testing.T) {
	d := commentDelivery("relayed text", "github-actions[bot]")
	d.Payload.Sender.Login = "github-actions[bot]"

	mut := &fakeGhMutations{}
	svc := writingSvc(&fakeGhRepo{mapping: mapped(), caseID: "case-1"}, mut, &fakeGhClient{})

	out, err := svc.HandleWebhook(context.Background(), d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Skipped == "" {
		t.Error("a comment from the integration account must be skipped")
	}
}
