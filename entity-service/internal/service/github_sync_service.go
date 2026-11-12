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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"log/slog"
	"strconv"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/github"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// GithubSyncService applies a GitHub issue webhook to a change request.
//
// Ported from ServiceNow's GitHubIssueContentProcessor, but the target schema
// is not the one it was written against, so several mappings are decisions
// rather than transcriptions. Each is named where it is made.
type GithubSyncService interface {
	// HandleWebhook applies one delivery. A delivery with nothing to do is not
	// an error -- most webhooks from a watched repository are not about a
	// change request at all.
	HandleWebhook(ctx context.Context, d Delivery) (Outcome, error)
	// CreateServiceRequestFromIssue is the same work HandleWebhook does for an
	// issues event, reachable directly. A repository that would rather call us
	// than wait for a webhook -- as servicenow_create_case.yml calls
	// ServiceNow today -- uses this.
	CreateServiceRequestFromIssue(ctx context.Context, req domain.CreateServiceRequestFromIssueRequest) (domain.CreateServiceRequestFromIssueResponse, error)
}

// Delivery is one webhook, already authenticated.
type Delivery struct {
	ID      string
	Event   string
	Payload IssuePayload
}

// Outcome says what a delivery did, for the response and the log.
type Outcome struct {
	Action          string
	ChangeRequestID string
	// Number is the created record's human-readable number, empty unless this
	// outcome created one.
	Number string
	// Skipped is why nothing happened, empty when something did.
	Skipped string
}

// IssuePayload is the part of GitHub's issues / issue_comment payload this
// reads. Everything else in a webhook body is ignored.
type IssuePayload struct {
	Action string `json:"action"`
	Issue  struct {
		Number  int    `json:"number"`
		Title   string `json:"title"`
		Body    string `json:"body"`
		State   string `json:"state"`
		HTMLURL string `json:"html_url"`
		Labels  []struct {
			Name string `json:"name"`
		} `json:"labels"`
		User github.User `json:"user"`
	} `json:"issue"`
	Label *struct {
		Name string `json:"name"`
	} `json:"label"`
	Comment *struct {
		Body    string      `json:"body"`
		HTMLURL string      `json:"html_url"`
		User    github.User `json:"user"`
	} `json:"comment"`
	Repository struct {
		Name  string `json:"name"`
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
	} `json:"repository"`
	Sender github.User `json:"sender"`
}

// LabelNames flattens the issue's labels.
func (p IssuePayload) LabelNames() []string {
	out := make([]string, 0, len(p.Issue.Labels))
	for _, l := range p.Issue.Labels {
		out = append(out, l.Name)
	}
	return out
}

// The label protocol. These are constants rather than configuration: the
// parsing logic depends on their shape, so an operator who changed them in
// config would break the code that reads them. ServiceNow kept them in two
// places -- hardcoded in the processor AND in github.label.* properties -- and
// the two could disagree.
const (
	labelChangeRequest  = "Type/ChangeRequest"
	labelPrefixCRType   = "CRType/"
	labelPrefixCRScope  = "CRScope/"
	labelScopeApp       = "CRScope/Application"
	labelScopeInfra     = "CRScope/Infrastructure"
	labelImpactHigh     = "Impact 1"
	labelImpactMedium   = "Impact 2"
	labelLikelihoodHigh = "Likelihood 1"
	labelLikelihoodMed  = "Likelihood 2"
)

// githubStateByLabel maps a state label onto change_request_state_enum.
//
// BY NAME, NOT BY NUMBER. ServiceNow mapped these through its numeric codes,
// and its own table disagreed with its own constants: stateAssessed was -3
// while stateMap called -3 "Authorize". Mapping the label directly to the
// state it names avoids importing that off-by-one.
//
// "Closed" and "Canceled" are absent on purpose. Closing is driven by the
// issue's closed action, which has a guard; a label should not be able to
// bypass it.
var githubStateByLabel = map[string]string{
	"Assessed":    "ASSESS",
	"Authorized":  "AUTHORIZE",
	"Scheduled":   "SCHEDULED",
	"Implemented": "IMPLEMENT",
	"Reviewed":    "REVIEW",
}

// githubImpactByLabel maps the impact labels onto change_request_impact_enum.
// ServiceNow used 1/2/3 where 1 was the most severe; the enum says so instead.
var githubImpactByLabel = map[string]string{
	labelImpactHigh:   "HIGH",
	labelImpactMedium: "MEDIUM",
}

var githubLikelihoodByLabel = map[string]string{
	labelLikelihoodHigh: "HIGH",
	labelLikelihoodMed:  "MEDIUM",
}

// githubTypeByScope maps the scope label onto change_request_type_enum.
//
// A DECISION, NOT A TRANSCRIPTION. ServiceNow stored scope in u_crscope
// (Application / Infrastructure) and a separate u_type (normal / standard /
// emergency). This schema has neither: it has change_request_type, whose
// values are INFRA and GENERAL. Scope maps onto it cleanly; the normal /
// standard / emergency distinction has nowhere to go and is dropped, which is
// recorded as an open question in docs/github-cr-sync-spec.md rather than
// silently discarded.
var githubTypeByScope = map[string]string{
	labelScopeApp:   "GENERAL",
	labelScopeInfra: "INFRA",
}

// stateReview is the only state from which the issue may be closed.
const stateReview = "REVIEW"

// stateClosed is where a successful close lands.
const stateClosed = "CLOSED"

type githubSyncService struct {
	repo repository.GithubSyncRepository
	gh   githubIssueClient
	// labels is the vocabulary this deployment recognises. Defaults to
	// ServiceNow's values; overridable because a repository's labels are a
	// deployment's business and dev need not match production.
	labels GithubLabels
	// mutate writes the change request. Nil leaves the sync read-only, which
	// is how it behaved before the mutation layer existed and is still useful
	// for a dry run against a live repository.
	mutate repository.GithubMutationRepository
	// srNotices runs the service-request automation for an SR created from an
	// issue, exactly as caseService does for one created in the portal:
	// assignment and acknowledgement for an automated SRE team, and sr.created
	// for the Chat card. Nil (no SRE_EVENT_HUB_TOPIC) runs none. Set with
	// WithGithubSRNotices.
	srNotices srNotifier
	// caseStates moves a case that is Awaiting Info to Waiting on WSO2 once a GitHub comment lands on it. Set with WithGithubCaseStates.
	caseStates githubCaseStateMover
	// integrationLogin is our own GitHub account. Events it sent are our own
	// writes coming back and are dropped -- identity, not string-matching the
	// comment body the way the case webhook does.
	integrationLogins []string
	// issueAuthorLogin is the account this service creates issues under
	// (GITHUB_INTEGRATION_LOGIN). Distinct from github-actions[bot], which runs
	// the repository's workflows and only ever labels and comments.
	issueAuthorLogin string
	// repoConfigs reads each repository's own mapping file. Nil (the
	// default) leaves account_github_repo the only source, as before.
	repoConfigs *githubRepoConfigs
	// issueTemplates reads each repository's issue form labels. Set with WithGithubRepoConfig; nil splits on every ### heading.
	issueTemplates *githubIssueTemplates
	// snCases creates a service request in ServiceNow first, under
	// dual-write, so it takes ServiceNow's CS number and WSO2 Case ID as a
	// portal-created one does (github_sn_case.go). Nil writes Postgres only
	// and numbers the record SR-GH-<n>. Set with WithServiceNowGithubCases.
	snCases snGithubCaseAPI
	// nativeNumbers numbers a new service request from the ServiceNow-style
	// series (CS<7 digits>, <project key>-<n>), as a portal-created SR is on
	// plain Postgres (#3363), instead of the GitHub series. Set with
	// WithNativeGithubSRNumbers, for DATA_SOURCE=postgres only: under
	// dual-write ServiceNow still allocates that series.
	nativeNumbers bool
	// slaEngine registers the CSM SLA clocks for a service request created
	// from an issue, as caseService.CreateCase does for one created in the
	// portal. Nil (CSM_SLA_ENGINE_ENABLED off, or no database) registers
	// none. Set with WithGithubSLAEngine.
	slaEngine SLAEngineService
}

// WithGithubSLAEngine gives the sync the SLA clock engine, so a service
// request created from a GitHub issue gets the same clocks one created in the
// portal does. A no-op if svc is not a *githubSyncService; a nil engine (the
// engine is disabled) leaves the sync registering no clocks.
func WithGithubSLAEngine(svc GithubSyncService, e SLAEngineService) GithubSyncService {
	if s, ok := svc.(*githubSyncService); ok && e != nil {
		s.slaEngine = e
	}
	return svc
}

// registerServiceRequestClocks registers the SLA clocks for a service request
// the sync just inserted, or found already inserted by an earlier or
// concurrent delivery. Best-effort, as on every other creation path: the
// record exists, so a failure is logged by the engine and never fails the
// delivery. Idempotent: the engine registers a clock only when no active
// (or already-completed) one exists for that record and policy, so running it
// again on a redelivery adds nothing and repairs a registration that failed
// the first time. It runs before the SR automation so a consumer reacting to
// the sr.created event finds the clocks already there, the order the portal's
// create path keeps for cases.
func (s *githubSyncService) registerServiceRequestClocks(ctx context.Context, id string) {
	if s.slaEngine == nil || id == "" {
		return
	}
	// The engine writes the sla table as the system: a webhook delivery
	// carries no user identity of its own.
	ctx, cancel := context.WithTimeout(repository.WithSystemIdentity(ctx), applyCaseStateSLATimeout)
	defer cancel()
	s.slaEngine.RegisterCaseClocks(ctx, id, nil, "")
}

// WithServiceNowGithubCases makes the sync create each new service request in
// ServiceNow first, through the ServiceNow integration service's
// /github-cases (ServiceNow's GitHub case API, as its M2M user), and keep
// ServiceNow in step with the issue's edits and comments. For DATA_SOURCE=
// postgres-servicenow-dual-write only. A no-op if svc is not a
// *githubSyncService or api is nil.
func WithServiceNowGithubCases(svc GithubSyncService, api snGithubCaseAPI) GithubSyncService {
	if s, ok := svc.(*githubSyncService); ok && api != nil {
		s.snCases = api
	}
	return svc
}

// WithNativeGithubSRNumbers makes a new service request raised from an issue
// take the ServiceNow-style number and WSO2 id (next_work_item_number,
// next_wso2_id) a portal-created SR takes on DATA_SOURCE=postgres, so the two
// read alike. Only for that data source: under dual-write ServiceNow still
// allocates the same series and the ids would collide. A repository mapped
// without a project keeps the GitHub series, as next_wso2_id needs one. A
// no-op if svc is not a *githubSyncService.
func WithNativeGithubSRNumbers(svc GithubSyncService) GithubSyncService {
	if s, ok := svc.(*githubSyncService); ok {
		s.nativeNumbers = true
	}
	return svc
}

// githubIssueClient is the slice of *github.Client this service needs.
type githubIssueClient interface {
	CreateComment(ctx context.Context, issue github.Issue, body string) (*github.Comment, error)
	SetLabels(ctx context.Context, issue github.Issue, labels []string) error
	RemoveLabel(ctx context.Context, issue github.Issue, label string) error
	AddLabel(ctx context.Context, issue github.Issue, label string) error
	SetState(ctx context.Context, issue github.Issue, state github.State) error
}

// NewGithubSyncService constructs the webhook policy.
func NewGithubSyncService(repo repository.GithubSyncRepository, gh githubIssueClient, integrationLogin string) GithubSyncService {
	return NewGithubSyncServiceWithLabels(repo, gh, integrationLogin, DefaultGithubLabels())
}

// NewGithubSyncServiceWithLabels is NewGithubSyncService with an explicit
// label vocabulary.
func NewGithubSyncServiceWithLabels(repo repository.GithubSyncRepository, gh githubIssueClient, integrationLogin string, labels GithubLabels) GithubSyncService {
	return &githubSyncService{repo: repo, gh: gh, integrationLogins: integrationLoginSet(integrationLogin), issueAuthorLogin: strings.TrimSpace(integrationLogin), labels: labels}
}

// WithMutations returns the service able to write change requests. Without it
// the sync recognises and reports but changes nothing.
func (s *githubSyncService) WithMutations(m repository.GithubMutationRepository) GithubSyncService {
	s.mutate = m
	return s
}

// WithGithubSRNotices gives the sync the service-request automation the
// portal's create path runs (caseService's srNotices), so an SR created from a
// GitHub issue is assigned, acknowledged and announced in Chat the same way.
// A no-op if svc is not a *githubSyncService or n is nil.
func WithGithubSRNotices(svc GithubSyncService, n *SRNoticeService) GithubSyncService {
	if s, ok := svc.(*githubSyncService); ok && n != nil {
		s.srNotices = n
	}
	return svc
}

// githubCaseStateMover is the part of the case service a relayed GitHub comment needs.
type githubCaseStateMover interface {
	MoveAwaitingInfoToWaitingOnWSO2(ctx context.Context, caseID string)
}

// WithGithubCaseStates lets a relayed GitHub comment move its Awaiting Info case to Waiting on WSO2. A no-op unless both sides are the Postgres services.
func WithGithubCaseStates(svc GithubSyncService, cases CaseService) GithubSyncService {
	s, ok := svc.(*githubSyncService)
	if !ok {
		return svc
	}
	if m, ok := cases.(githubCaseStateMover); ok {
		s.caseStates = m
	}
	return svc
}

// NewGithubSyncServiceWriting is the full service: recognises, writes, and
// pushes the resulting label changes back to the issue.
func NewGithubSyncServiceWriting(repo repository.GithubSyncRepository, mutate repository.GithubMutationRepository, gh githubIssueClient, integrationLogin string, labels GithubLabels) GithubSyncService {
	return &githubSyncService{repo: repo, gh: gh, integrationLogins: integrationLoginSet(integrationLogin), issueAuthorLogin: strings.TrimSpace(integrationLogin), labels: labels, mutate: mutate}
}

func skip(reason string) (Outcome, error) { return Outcome{Skipped: reason}, nil }

// HandleWebhook implements GithubSyncService.
//
// THE DELIVERY CLAIM WRAPS EVERY WRITE. GitHub retries any delivery it did not
// get a 2xx for, and an issue must not become two change requests.
func (s *githubSyncService) HandleWebhook(ctx context.Context, d Delivery) (Outcome, error) {
	p := d.Payload

	// The cheap guards first, outside the claim: they touch nothing, so
	// recording a delivery we are going to ignore protects nothing.
	//
	// Our own writes come back as webhooks. Dropping them by sender identity is
	// what stops a comment we posted being synced back as a new one.
	//
	// THE IDENTITY IS THE WORKFLOW'S, NOT OURS. We dispatch; a GitHub Actions
	// workflow does the posting, so the sender on the way back is
	// github-actions[bot] and never the integration account. Before the
	// dispatch rewrite this service posted directly and the two were the same,
	// which is why the default outlived its meaning: a CSM comment went out,
	// came back as a bot comment, and was written onto the case as a new one.
	//
	// The text marker below is the second line of defence, matching
	// sn_comment_to_github.yml's own guard. Identity alone should be enough;
	// the marker catches a repository whose workflow posts under some other
	// account.
	// ONLY A COMMENT CAN LOOP, SO ONLY A COMMENT IS GUARDED ON IDENTITY.
	//
	// Applying the identity check to every event broke issue creation
	// outright: the repository's validation workflow applies
	// validation-passed, GitHub reports that as issues/labeled sent by
	// github-actions[bot], and the guard discarded exactly the event the
	// validation gate waits for. The issue passed validation and no record was
	// ever created -- observed on a live delivery, invisible to the unit tests
	// because their fixtures send as a human.
	//
	// An issues event carries the reporter's own text no matter who triggered
	// the delivery, so re-reading it cannot echo anything we wrote; the unique
	// index and the update path make a repeat harmless. A comment is the only
	// thing we put back onto the issue, so it is the only thing that can
	// return to us as new content.
	if d.Event != "issues" && d.Event != "issue_comment" {
		return skip("event " + d.Event + " is not handled")
	}
	if d.Event == "issue_comment" {
		// Any of our identities: a comment is the only thing we put back onto
		// an issue, so it is the only thing that can return as new content.
		if s.isOwnEvent(p) {
			return skip("event was sent by the integration account")
		}
	} else if s.isOwnIssue(p) {
		return skip("issue was raised by this service")
	}

	if err := s.repo.ClaimDelivery(ctx, d.ID, d.Event, p.Action); err != nil {
		// ErrDeliverySeen travels up to the handler, which answers 200 so
		// GitHub stops retrying something already applied.
		return Outcome{}, err
	}

	out, err := s.handleClaimed(ctx, d)
	if err != nil {
		if rerr := s.repo.ReleaseDelivery(ctx, d.ID); rerr != nil {
			slog.ErrorContext(ctx, "github: could not release delivery claim",
				"delivery", d.ID, "err", rerr)
		}
		return Outcome{}, err
	}

	if out.ChangeRequestID != "" {
		if lerr := s.repo.LinkDelivery(ctx, d.ID, out.ChangeRequestID); lerr != nil {
			slog.WarnContext(ctx, "github: could not link delivery to change request",
				"delivery", d.ID, "err", lerr)
		}
	}
	return out, nil
}

// handleClaimed is the body of HandleWebhook, run with the delivery claimed.
func (s *githubSyncService) handleClaimed(ctx context.Context, d Delivery) (Outcome, error) {
	p := d.Payload

	// An unmapped repository is not ours: one with neither its own mapping
	// file nor an account_github_repo row is skipped (resolveMapping).
	mapping, reason, err := s.resolveMapping(ctx, p.Repository.Owner.Login, p.Repository.Name)
	if err != nil {
		return Outcome{}, err
	}
	if reason != "" {
		return skip(reason)
	}

	// What this issue already produced, if anything. Looked up by issue number
	// within the repository rather than by git_reference: the record is a
	// service request now, and service_request has no git_reference column.
	existing, err := s.repo.CaseByIssue(ctx, mapping.AccountID, p.Repository.Owner.Login, p.Repository.Name, p.Issue.Number)
	if err != nil {
		return Outcome{}, err
	}

	if d.Event == "issue_comment" {
		return s.handleComment(ctx, p, mapping, existing)
	}
	return s.handleIssue(ctx, p, mapping, existing)
}

// handleIssue creates or updates the record an issue represents.
//
// A GITHUB ISSUE BECOMES A SERVICE REQUEST, NOT A CHANGE REQUEST.
// issue_servicenow.yml maps a [CR]:/[ECR]: title onto case_type
// "Service Request" with catalog "Generic Requests", carrying the CR/*Change
// label as sr_type; a Type/ServiceRequest label onto "General Requests". The
// change request proper is raised later, by a person, with the approval path
// and planned window an issue cannot supply -- which is why 759 change
// requests hang off a service request in the data and only 301 off a case.
//
// An earlier version of this created a change request directly. Those records
// had no parent and no account, because there was nothing to parent them to.
func (s *githubSyncService) handleIssue(ctx context.Context, p IssuePayload, mapping *repository.RepoMapping, existing string) (Outcome, error) {
	if !s.validated(p) {
		return skip("issue has not passed template validation yet")
	}

	catalog, srType, ok := s.classify(p)
	if !ok {
		return skip("issue is neither a [CR]: change request nor labelled as a service request")
	}

	if s.mutate == nil {
		return Outcome{Action: "creation_prepared"}, nil
	}
	issue := snGithubIssueOf(p)
	issue.Headings = s.issueHeadingsFor(ctx, p.Repository.Owner.Login, p.Repository.Name)
	if existing != "" {
		// The issue already produced a record. Its text is the source of
		// truth, so an edit overwrites -- servicenow_update_case.yml re-sends
		// the whole body rather than a diff for the same reason.
		update := repository.NewChangeRequestFromIssue{
			Subject:      p.Issue.Title,
			Description:  p.Issue.Body,
			GitReference: p.Issue.HTMLURL,
			Catalog:      catalog,
			SRType:       srType,
			Fields:       extractTemplateFields(p.Issue.Body, issue.Headings),
		}
		if s.snCases != nil || s.nativeNumbers {
			// Used only for a record outside the GitHub series (see UpdateFromIssue).
			update.ServiceNowDescription = snGithubCaseDescription(extractSNIssueFields(p.Issue.Body, issue.Headings), issue)
		}
		if err := s.mutate.UpdateFromIssue(ctx, existing, update); err != nil {
			return Outcome{}, err
		}
		// An edit reaches ServiceNow as servicenow_update_case.yml sent it --
		// only an edit, not every label the workflows apply. After the Postgres
		// write and best effort, as any dual-write update: a failure leaves
		// ServiceNow's copy stale, never this one.
		if s.snCases != nil && p.Action == "edited" {
			if err := s.snCases.UpdateCase(ctx, snGithubCaseUpdate(issue)); err != nil {
				logSNGithubCaseError(ctx, "github: ServiceNow did not take the issue edit", err,
					"serviceRequestId", existing, "issue", p.Issue.Number)
			}
		}
		return Outcome{Action: "updated", ChangeRequestID: existing}, nil
	}

	in := repository.NewServiceRequestFromIssue{
		Subject:      p.Issue.Title,
		Description:  p.Issue.Body,
		GitReference: p.Issue.HTMLURL,
		IssueNumber:  p.Issue.Number,
		AccountID:    mapping.AccountID,
		ProjectID:    mapping.ProjectID,
		Owner:        p.Repository.Owner.Login,
		Repository:   p.Repository.Name,
		Catalog:      catalog,
		SRType:       srType,
		Fields:       extractTemplateFields(p.Issue.Body, issue.Headings),
		CreatedBy:    p.Issue.User.Login,
	}
	// ServiceNow first, as a portal-created record under dual-write: the
	// record takes ServiceNow's sys_id, CS number and WSO2 Case ID. A
	// repository mapped without a project (account_github_repo) cannot be, as
	// ServiceNow's API requires one; it keeps the GitHub series.
	snFirst := s.snCases != nil && mapping.ProjectID != ""
	var snSysID string
	if snFirst {
		res, err := s.snCases.CreateCase(ctx, snGithubCaseCreate(issue, mapping.AccountID, mapping.ProjectID,
			mapping.Category, mapping.AnnouncementType, catalog, githubCatalogPrefix(p, catalog)))
		if err != nil {
			// Nothing is written here: ServiceNow has no case, and a Postgres
			// row without one would never get its numbers. The delivery is
			// released and can be redelivered.
			return Outcome{}, fmt.Errorf("github: create the service request in ServiceNow: %w", err)
		}
		if res.WSO2CaseID == "" {
			slog.WarnContext(ctx, "github: ServiceNow returned no WSO2 Case ID; the record takes the GitHub series id",
				"snNumber", res.Number, "issue", p.Issue.Number)
		}
		snSysID = res.SysID
		in.ServiceNowID = sysidToUUID(res.SysID)
		in.ServiceNowNumber = res.Number
		in.ServiceNowWSO2ID = res.WSO2CaseID
		// The same text ServiceNow holds, so both stores read alike and the
		// portal renders it as it renders every ServiceNow-numbered record.
		in.Description = snGithubCaseDescription(extractSNIssueFields(p.Issue.Body, issue.Headings), issue)
	} else if s.nativeNumbers && mapping.ProjectID != "" {
		// Plain Postgres: the series a portal-created SR takes (CS..., <key>-<n>),
		// with the description ServiceNow would have held, so the record reads
		// like every other CS-numbered one.
		in.NativeNumbers = true
		in.Description = snGithubCaseDescription(extractSNIssueFields(p.Issue.Body, issue.Headings), issue)
	}

	id, number, err := s.mutate.CreateServiceRequestFromIssue(ctx, in)
	if err != nil && snFirst && !errors.Is(err, repository.ErrAlreadyExists()) {
		// ServiceNow has the case; Postgres does not. A redelivery recovers it:
		// ServiceNow answers "Case already exists" with the same ids.
		slog.ErrorContext(ctx, "github: ServiceNow case created but the Postgres insert failed",
			"snId", in.ServiceNowID, "snNumber", in.ServiceNowNumber, "issue", p.Issue.Number, "error", err)
	}
	if err != nil {
		// A concurrent delivery for the same issue won the insert. Nothing to
		// do, and not a failure -- the record the caller wanted exists.
		if errors.Is(err, repository.ErrAlreadyExists()) {
			slog.InfoContext(ctx, "github: issue already had a service request",
				"serviceRequestId", id, "issue", p.Issue.Number)
			// Idempotent: repairs a registration an earlier delivery missed.
			s.registerServiceRequestClocks(ctx, id)
			return Outcome{Action: "exists", ChangeRequestID: id}, nil
		}
		return Outcome{}, err
	}
	slog.InfoContext(ctx, "github: service request created from issue",
		"serviceRequestId", id, "number", number, "issue", p.Issue.Number, "catalog", catalog)
	s.registerServiceRequestClocks(ctx, id)
	// Only for a new record: an issue that already had one ("exists" above)
	// was announced when that record was created.
	//
	// Ours in every case; ServiceNow's "SR New Request - Acknowledge & Chat
	// Alert" flow is not relied on. For a record created in ServiceNow first,
	// the acknowledgement comment is written to ServiceNow's case as well.
	if s.srNotices != nil {
		mirrored, canMirror := s.srNotices.(interface {
			OnCreatedMirrored(ctx context.Context, caseID string, mirror srAckMirror)
		})
		if snFirst && canMirror {
			mirrored.OnCreatedMirrored(ctx, id, func(ctx context.Context, comment string) {
				if err := s.snCases.AddComment(ctx, snSysID, comment); err != nil {
					logSNGithubCaseError(ctx, "github: ServiceNow did not take the SR acknowledgement", err,
						"serviceRequestId", id, "issue", p.Issue.Number)
				}
			})
		} else {
			s.srNotices.OnCreated(ctx, id)
		}
	}
	return Outcome{Action: "created", ChangeRequestID: id, Number: number}, nil
}

// classify decides what an issue is, in the order issue_servicenow.yml does.
// The title is checked first because a change request carries no type label.
// validated reports whether the repository's own validation workflow has
// passed this issue. Nothing is created before it has: the template decides
// which catalog and fields a record gets, and an unchecked template produces a
// record nobody can act on. The workflow applies the label on open and on every
// edit, so an issue fixed after a failure arrives here as a labeled event.
func (s *githubSyncService) validated(p IssuePayload) bool {
	for _, l := range p.LabelNames() {
		if l == s.labels.ValidationPassed {
			return true
		}
	}
	return false
}

func (s *githubSyncService) classify(p IssuePayload) (catalog, srType string, ok bool) {
	labels := p.LabelNames()
	if IsChangeRequestTitle(p.Issue.Title) {
		class, found := s.labels.ClassOf(labels)
		if !found {
			// The class label is applied by the repository's validation
			// workflow. Acting before it lands would make a record from a
			// template nobody has checked.
			return "", "", false
		}
		return CatalogGenericRequests, class, true
	}
	for _, l := range labels {
		if l == s.labels.TypeServiceRequest {
			return CatalogGeneralRequests, "", true
		}
	}
	return "", "", false
}

// handleComment mirrors a GitHub comment onto the change request.
func (s *githubSyncService) handleComment(ctx context.Context, p IssuePayload, mapping *repository.RepoMapping, caseID string) (Outcome, error) {
	// CREATED ONLY. AddComment always inserts, and nothing links a CSM comment
	// back to the GitHub comment it came from, so treating an edit as new
	// content appends a second copy to the case every time someone fixes a
	// typo. Relaying the first version and ignoring later edits loses less
	// than duplicating the thread does.
	if p.Action != "created" {
		return skip("comment action " + p.Action + " is not handled")
	}
	// Comment is a pointer: an issue_comment delivery that carries no comment
	// object is malformed, and dereferencing it would panic the handler rather
	// than answer GitHub.
	if p.Comment == nil {
		return skip("issue_comment delivery carried no comment")
	}
	// THE COMMENT GOES ON THE CASE, NOT THE CHANGE REQUEST.
	//
	// github_comment_to_sn.yml PATCHes sn_customerservice_case, and the
	// outbound trigger only watches case comments. Attaching a relayed comment
	// to the change request instead produced a conversation that could not be
	// answered: a reply on the change request synced nowhere, and a reply on
	// the case reached GitHub detached from the thread that started it.
	if caseID == "" {
		return skip("no case is linked to this issue")
	}
	body := strings.TrimSpace(p.Comment.Body)
	if body == "" {
		return skip("comment is empty")
	}
	// A command, not a note. The repository's own workflows own these.
	if strings.HasPrefix(body, "/") {
		return skip("comment is a slash command")
	}
	if s.mutate == nil {
		return Outcome{Action: "comment_pending_write"}, nil
	}

	// The "(GitHub Comment)" marker is what github_comment_to_sn.yml stamps on
	// what it relays, and what sn_comment_to_github.yml checks for before
	// posting back. Keeping the same marker keeps the same loop closed.
	author := p.Comment.User.Login
	if author == "" {
		author = "a GitHub user"
	}
	text := fmt.Sprintf("@%s (GitHub Comment) %s\n\n%s", author, p.Issue.HTMLURL, body)

	if err := s.mutate.AddComment(ctx, caseID, text, author); err != nil {
		return Outcome{}, err
	}
	if s.caseStates != nil {
		s.caseStates.MoveAwaitingInfoToWaitingOnWSO2(ctx, caseID)
	}
	// The case's ServiceNow journal gets it too, as github_comment_to_sn.yml
	// wrote it. After the Postgres write and best effort; a record ServiceNow
	// never had (the GitHub series) answers not found, logged at info.
	if s.snCases != nil {
		if err := s.snCases.AddComment(ctx, uuidToSysid(caseID), snGithubCommentText(author, p.Comment.HTMLURL, body)); err != nil {
			logSNGithubCaseError(ctx, "github: ServiceNow did not take the issue comment", err,
				"serviceRequestId", caseID, "issue", p.Issue.Number)
		}
	}
	return Outcome{Action: "comment_relayed", ChangeRequestID: caseID}, nil
}

// isOwnIssue reports whether an issues event concerns an issue this service
// raised itself, from a case. Only the configured integration login creates
// issues; github-actions[bot] never does -- it runs the repository's workflows,
// which label and comment.
//
// THE DISTINCTION IS NOT COSMETIC. Treating every integration identity as
// disqualifying here discarded the issues/labeled event that the validation
// workflow produces when it applies validation-passed -- the exact delivery the
// validation gate waits for. The issue passed validation and no record was ever
// created. Guarding on the issue's author instead keeps the real loop closed
// (an issue we raised from a case must not come back as a new service request)
// without silencing the workflow.
func (s *githubSyncService) isOwnIssue(p IssuePayload) bool {
	return s.issueAuthorLogin != "" &&
		strings.EqualFold(p.Issue.User.Login, s.issueAuthorLogin)
}

// isOwnEvent reports whether this delivery is an echo of something we caused.
func (s *githubSyncService) isOwnEvent(p IssuePayload) bool {
	for _, login := range s.integrationLogins {
		if login != "" && strings.EqualFold(p.Sender.Login, login) {
			return true
		}
	}
	// A comment we relayed carries this marker, and so does one the workflow
	// posted on our behalf. Either way it originated here.
	if p.Comment != nil && strings.Contains(p.Comment.Body, "(GitHub Comment)") {
		return true
	}
	return false
}

// integrationLoginSet is every account whose events are our own coming back.
//
// github-actions[bot] is always included: with the dispatch architecture it is
// the account that actually posts, whatever the configured integration login
// is. Leaving it to configuration would make a loop the default.
func integrationLoginSet(configured string) []string {
	out := []string{"github-actions[bot]"}
	if c := strings.TrimSpace(configured); c != "" {
		out = append(out, c)
	}
	return out
}

// CreateServiceRequestFromIssue implements GithubSyncService.
//
// Built on the same handleIssue the webhook uses, so the two entry points
// cannot drift: one recognition rule, one catalog mapping, one field parser.
// The only difference is what carries the issue in.
func (s *githubSyncService) CreateServiceRequestFromIssue(ctx context.Context, req domain.CreateServiceRequestFromIssueRequest) (domain.CreateServiceRequestFromIssueResponse, error) {
	if strings.TrimSpace(req.Owner) == "" || strings.TrimSpace(req.Repository) == "" {
		return domain.CreateServiceRequestFromIssueResponse{}, &apierror.ValidationError{Msg: "owner and repository are required"}
	}
	if req.IssueNumber <= 0 {
		return domain.CreateServiceRequestFromIssueResponse{}, &apierror.ValidationError{Msg: "issueNumber is required"}
	}
	if strings.TrimSpace(req.Title) == "" {
		return domain.CreateServiceRequestFromIssueResponse{}, &apierror.ValidationError{Msg: "title is required"}
	}

	mapping, reason, err := s.resolveMapping(ctx, req.Owner, req.Repository)
	if err != nil {
		return domain.CreateServiceRequestFromIssueResponse{}, err
	}
	if reason != "" {
		return domain.CreateServiceRequestFromIssueResponse{}, &apierror.ConflictError{Msg: reason}
	}

	// Already created is success, not a conflict: the caller's retry after a
	// timeout must not produce a second record, and servicenow_create_case.yml
	// answers its own repeat with "Case Already Exists" for the same reason.
	existing, err := s.repo.CaseByIssue(ctx, mapping.AccountID, req.Owner, req.Repository, req.IssueNumber)
	if err != nil {
		return domain.CreateServiceRequestFromIssueResponse{}, err
	}
	if existing != "" {
		return domain.CreateServiceRequestFromIssueResponse{
			Message: "a service request already exists for this issue", ID: existing,
		}, nil
	}

	p := IssuePayload{}
	p.Issue.Number = req.IssueNumber
	p.Issue.Title = req.Title
	p.Issue.Body = req.Body
	p.Issue.HTMLURL = "https://github.com/" + req.Owner + "/" + req.Repository + "/issues/" + strconv.Itoa(req.IssueNumber)
	p.Issue.User.Login = req.Author
	p.Repository.Owner.Login = req.Owner
	p.Repository.Name = req.Repository
	for _, l := range req.Labels {
		p.Issue.Labels = append(p.Issue.Labels, struct {
			Name string `json:"name"`
		}{Name: l})
	}

	out, err := s.handleIssue(ctx, p, mapping, "")
	if err != nil {
		return domain.CreateServiceRequestFromIssueResponse{}, err
	}
	if out.Skipped != "" {
		return domain.CreateServiceRequestFromIssueResponse{}, &apierror.ValidationError{Msg: out.Skipped}
	}
	if out.Action == "exists" {
		return domain.CreateServiceRequestFromIssueResponse{
			Message: "a service request already exists for this issue", ID: out.ChangeRequestID,
		}, nil
	}
	return domain.CreateServiceRequestFromIssueResponse{
		Message: "service request created", ID: out.ChangeRequestID,
		Number: out.Number, Created: true,
	}, nil
}

// snGithubIssueOf is the issue as the ServiceNow payloads describe it.
func snGithubIssueOf(p IssuePayload) snGithubIssue {
	return snGithubIssue{
		Number:   p.Issue.Number,
		Title:    p.Issue.Title,
		Body:     p.Issue.Body,
		URL:      p.Issue.HTMLURL,
		User:     p.Issue.User.Login,
		Labels:   p.LabelNames(),
		RepoFull: p.Repository.Owner.Login + "/" + p.Repository.Name,
	}
}

// githubCatalogPrefix is the catalog_prefix issue_servicenow.yml hands
// servicenow_create_case.yml: the issue's CatalogueItem/ label when it has
// one (an emergency change), Type/ServiceRequest for a general service
// request, nothing for a change request.
func githubCatalogPrefix(p IssuePayload, catalog string) string {
	for _, l := range p.LabelNames() {
		if strings.HasPrefix(l, "CatalogueItem/") {
			return l
		}
	}
	if catalog == CatalogGeneralRequests {
		return "Type/ServiceRequest"
	}
	return ""
}

// logSNGithubCaseError logs a best-effort ServiceNow write that failed: at
// info when ServiceNow has no such case (a record of the GitHub series), as an
// error otherwise.
func logSNGithubCaseError(ctx context.Context, msg string, err error, args ...any) {
	args = append(args, "error", err)
	if errors.Is(err, errSNGithubCaseNotFound) {
		slog.InfoContext(ctx, msg+" (no such case in ServiceNow)", args...)
		return
	}
	slog.ErrorContext(ctx, msg, args...)
}
