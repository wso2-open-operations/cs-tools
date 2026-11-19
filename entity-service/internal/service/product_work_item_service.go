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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/engineeringentity"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/validate"
)

const (
	// GitHub's own limits: a title is at most 256 characters and a body at most 65,536.
	maxProductIssueTitleChars = 256
	maxProductIssueBodyChars  = 65536
	maxIdempotencyKeyChars    = 200

	msgProductRepoNotMapped = "No GitHub repository is mapped for this product."
	msgProductTitleInvalid  = "A title of up to 256 characters is required."
	msgProductBodyTooLong   = "The issue description is too long."
	msgProductUpstream      = "The GitHub issue could not be created. Please try again later."
	msgProductRateLimited   = "GitHub is limiting requests right now. Please try again shortly."
	msgProductNotRecorded   = "The issue was created on GitHub but could not be recorded yet. Please retry the same request."

	productWorkItemForbidden = "product issues are available to WSO2 staff only"

	// productRequestLease is how long a PENDING attempt that has recorded no
	// outcome counts as still running. It must exceed productGitHubPhaseTimeout,
	// the longest an attempt can spend between claiming the row and recording
	// GITHUB_CREATED, or a second attempt could start while the first is mid-call.
	productRequestLease = 2 * time.Minute
	// productAmbiguousLease is the least time since its last write that a PENDING
	// row carrying an ambiguity note (an earlier attempt may have created the
	// issue) must have before another attempt may resume it. GitHub's issue list
	// can lag a create by a few seconds, so an immediate retry's earlier-issue
	// search could miss the new issue and create a second one.
	productAmbiguousLease = 60 * time.Second
	// productGitHubPhaseTimeout bounds the earlier-issue search plus the create.
	productGitHubPhaseTimeout = 90 * time.Second
	// productDetailTimeout bounds the best-effort read of the created issue.
	productDetailTimeout = 15 * time.Second
	// productRecordTimeout bounds the database steps that must run even when the
	// caller has gone away.
	productRecordTimeout = 10 * time.Second

	// adoptionMaxPages bounds the earlier-issue search: a page holds up to 100
	// issues updated since the request was first made.
	adoptionMaxPages = 5
	adoptionPageSize = 100
)

// UpstreamFailureError is a refusal that is the engineering entity's (and so
// GitHub's) rather than the caller's: 502, or 503 with a Retry-After when GitHub
// is rate limiting. Msg is caller-safe; nothing from the upstream body is in it.
type UpstreamFailureError struct {
	Status     int
	Msg        string
	RetryAfter time.Duration
}

func (e *UpstreamFailureError) Error() string { return e.Msg }

// ProductWorkItemService creates the internal counterpart of a case's GitHub issue.
type ProductWorkItemService interface {
	// CreateProductWorkItem files the GitHub issue for a case and registers it.
	// created is false when the answer is the stored result of an earlier,
	// completed request with the same idempotency key.
	CreateProductWorkItem(ctx context.Context, req domain.CreateProductWorkItemRequest) (resp domain.CreateProductWorkItemResponse, created bool, err error)
}

// productWorkItemEngineering is the engineering entity calls this service makes.
type productWorkItemEngineering interface {
	CreateIssue(ctx context.Context, org, owner, repo, title, body string, labels []string) (engineeringentity.IssueRef, error)
	GetIssueDetail(ctx context.Context, org, owner, repo string, number int) (engineeringentity.IssueDetail, error)
	ListIssueDetails(ctx context.Context, org, owner, repo string, q engineeringentity.ListIssueDetailsQuery) ([]engineeringentity.IssueDetail, error)
}

// productRepoMappingLister is the catalogue of product repositories.
type productRepoMappingLister interface {
	ListActive(ctx context.Context) ([]domain.ProductRepoMapping, error)
}

// productWorkItemCases is the slice of CaseService the create uses: the same
// tag and comment paths POST /cases/{id}/tags and /comments take.
type productWorkItemCases interface {
	AddCaseTagAs(ctx context.Context, caseID, label, actorEmail string) (domain.Tag, error)
	CreateCaseCommentAs(ctx context.Context, req domain.CreateCaseCommentRequest, actorEmail string) (domain.CreateCaseCommentResponse, error)
}

// patchCaseTag is the case tag an untracked patch adds: billing reads it.
const patchCaseTag = "patch"

type productWorkItemService struct {
	eng      productWorkItemEngineering
	repo     repository.ProductWorkItemRepository
	mappings productRepoMappingLister
	cases    productWorkItemCases
	access   AccessService
	tagRule  ProductIssueCaseTagRule // nil: s_dip is not maintained (WithProductCaseTagRule)
	sla      WorkItemSLAEngine       // nil: no SLA clock at creation (WithProductWorkItemSLA)
}

// NewProductWorkItemService constructs a ProductWorkItemService.
func NewProductWorkItemService(eng productWorkItemEngineering, repo repository.ProductWorkItemRepository, mappings productRepoMappingLister, cases productWorkItemCases, access AccessService) ProductWorkItemService {
	return &productWorkItemService{eng: eng, repo: repo, mappings: mappings, cases: cases, access: access}
}

// productRequestFingerprint is the canonical form of a request, without its
// idempotency key (and without the ignored repoOverride), that the request hash
// is taken over. Field order is the struct's, so the encoding is stable.
type productRequestFingerprint struct {
	Title                string `json:"title"`
	Description          string `json:"description"`
	Reason               string `json:"reason"`
	UpdateLevel          string `json:"updateLevel"`
	PublicIssueURL       string `json:"publicIssueUrl"`
	Regression           bool   `json:"regression"`
	HotFixRequired       bool   `json:"hotFixRequired"`
	IssueTypeLabel       string `json:"issueTypeLabel"`
	PriorityLevel        string `json:"priorityLevel"`
	OnboardingInProgress bool   `json:"onboardingInProgress"`
}

func hashProductRequest(req domain.CreateProductWorkItemRequest) string {
	b, _ := json.Marshal(productRequestFingerprint{
		Title: req.Title, Description: req.Description, Reason: strings.TrimSpace(req.Reason),
		UpdateLevel: req.UpdateLevel, PublicIssueURL: req.PublicIssueURL,
		Regression: req.Regression, HotFixRequired: req.HotFixRequired,
		IssueTypeLabel: strings.TrimSpace(req.IssueTypeLabel), PriorityLevel: strings.TrimSpace(req.PriorityLevel),
		OnboardingInProgress: req.OnboardingInProgress,
	})
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// productWorkItemKind is the kind a request creates: only Type/Patch is
// untracked (patches have no work item yet); every other issue type is a product issue.
func productWorkItemKind(req domain.CreateProductWorkItemRequest) string {
	if strings.TrimSpace(req.IssueTypeLabel) == patchIssueTypeLabel {
		return domain.ProductWorkItemKindUntrackedPatch
	}
	return domain.ProductWorkItemKindProductIssue
}

// CreateProductWorkItem implements ProductWorkItemService.
//
// The create is idempotent on (case, idempotencyKey), recorded in
// work_item_creation_request, because it spans two systems that cannot share a
// transaction: GitHub (a created issue cannot be rolled back) and Postgres.
// The order is: record PENDING, create on GitHub, record GITHUB_CREATED at once,
// then register in one database transaction. Whatever stops in between, a retry
// with the same key finishes the job instead of filing a second issue:
//
//   - COMPLETED: the stored response is returned (200).
//   - GITHUB_CREATED: GitHub is skipped; registration is finished.
//   - FAILED (a 4xx: GitHub did not create it): the create is retried.
//   - PENDING with a recorded ambiguous outcome (timeout, 5xx, network; once
//     productAmbiguousLease has passed) or an expired lease: an earlier attempt may have created the issue, so the
//     repository's recent Origin/CS issues are searched for one with this
//     request's title and footer lines; a match is adopted, else the issue is
//     created.
//   - PENDING with no outcome and a live lease: another attempt is running; 409
//     request_in_progress, nothing is done.
//
// REMAINING WINDOW. The database steps of every attempt for one (case, key) are
// serialised by a transaction-scoped advisory lock and a fresh read of the row,
// but the lock cannot be held across the GitHub call. What stops two attempts
// both creating is the lease: a claimed row is "in flight" for
// productRequestLease (2 minutes), longer than productGitHubPhaseTimeout (90 s),
// the most one attempt can spend before it records GITHUB_CREATED. A row an
// attempt left ambiguous (timeout, 5xx, a failed GITHUB_CREATED write) may be
// resumed only productAmbiguousLease (60 s) after its last write, so GitHub's
// issue list has had time to show an issue the earlier attempt created. A
// duplicate issue is therefore possible only if an attempt outlives its lease
// (its process is frozen for over 30 s past the deadline) or GitHub's list still
// does not show an issue a minute after it was created, when the adoption search
// runs. Such a duplicate carries the same title and footer, so it is easy to
// find and close.
func (s *productWorkItemService) CreateProductWorkItem(ctx context.Context, req domain.CreateProductWorkItemRequest) (domain.CreateProductWorkItemResponse, bool, error) {
	if err := RequireInternalCaller(ctx, s.access, productWorkItemForbidden); err != nil {
		return domain.CreateProductWorkItemResponse{}, false, err
	}
	email := strings.TrimSpace(auth.IdentityFromContext(ctx).UserEmail)
	if email == "" {
		return domain.CreateProductWorkItemResponse{}, false, &apierror.ValidationError{Msg: "A signed-in user is required to file a GitHub issue."}
	}
	if err := validateProductWorkItemRequest(&req); err != nil {
		return domain.CreateProductWorkItemResponse{}, false, err
	}

	kase, err := s.repo.LoadCase(ctx, req.CaseID)
	if err != nil {
		return domain.CreateProductWorkItemResponse{}, false, err
	}
	kind := productWorkItemKind(req)
	cr, err := s.repo.BeginCreationRequest(ctx, repository.BeginCreationParams{
		CaseID: req.CaseID, Key: req.IdempotencyKey, Hash: hashProductRequest(req), Kind: kind, Actor: email,
		Lease: productRequestLease, AmbiguousLease: productAmbiguousLease,
	})
	if err != nil {
		return domain.CreateProductWorkItemResponse{}, false, err
	}

	switch {
	case cr.RequestHash != hashProductRequest(req):
		return domain.CreateProductWorkItemResponse{}, false, &apierror.ConflictError{
			Msg: "This idempotency key was already used for a different request.", Code: apierror.CodeIdempotencyKeyReused}
	case cr.Status == repository.CreationStatusCompleted:
		var stored domain.CreateProductWorkItemResponse
		if err := json.Unmarshal(cr.Response, &stored); err != nil {
			return domain.CreateProductWorkItemResponse{}, false, fmt.Errorf("stored response of creation request %s: %w", cr.ID, err)
		}
		return stored, false, nil
	case cr.InFlight:
		return domain.CreateProductWorkItemResponse{}, false, &apierror.ConflictError{
			Msg: "A request with this idempotency key is still being processed. Please retry shortly.", Code: apierror.CodeRequestInProgress}
	}

	mapping, mappingLabel, mapErr := s.resolveRepository(ctx, kase)
	err = mapErr
	var ref repository.GitHubIssueRef
	if cr.Status == repository.CreationStatusGitHubCreated && cr.GitHubNumber != nil && cr.GitHubOwner != nil && cr.GitHubRepository != nil {
		// GitHub already has the issue: finish registration, whatever the case or
		// mapping look like now.
		ref = repository.GitHubIssueRef{Owner: *cr.GitHubOwner, Repository: *cr.GitHubRepository, Number: *cr.GitHubNumber,
			NodeID: derefString(cr.GitHubNodeID), HTMLURL: derefString(cr.GitHubHTMLURL)}
	} else {
		if err == nil {
			err = checkCaseState(kase)
		}
		if err != nil {
			s.recordOutcome(ctx, cr.ID, email, "refused before GitHub: "+err.Error(), ownsRow(cr))
			return domain.CreateProductWorkItemResponse{}, false, err
		}
		ref, err = s.fileIssue(ctx, req, kase, mapping, mappingLabel, cr, email)
		if err != nil {
			return domain.CreateProductWorkItemResponse{}, false, err
		}
	}

	// Best effort: node id, canonical URL and timestamp. A failed read costs
	// nothing that matters; a later sync fills node_id.
	var createdOn *time.Time
	var detailLabels []string
	detailCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), productDetailTimeout)
	detail, derr := s.eng.GetIssueDetail(detailCtx, ref.Owner, ref.Owner, ref.Repository, ref.Number)
	cancel()
	if derr == nil {
		ref.NodeID = detail.NodeID
		detailLabels = detail.Labels
		if detail.HTMLURL != "" {
			ref.HTMLURL = detail.HTMLURL
		}
		if t, perr := time.Parse(time.RFC3339, detail.CreatedAt); perr == nil {
			createdOn = &t
		}
	} else {
		slog.WarnContext(ctx, "product work item: reading the created issue failed; using the built URL",
			"caseId", req.CaseID, "repo", ref.Owner+"/"+ref.Repository, "number", ref.Number, "err", derr)
	}
	if ref.HTMLURL == "" {
		ref.HTMLURL = fmt.Sprintf("https://github.com/%s/%s/issues/%d", ref.Owner, ref.Repository, ref.Number)
	}

	params := repository.RegisterParams{
		RequestID: cr.ID, CaseID: req.CaseID, CaseKey: req.IdempotencyKey, Actor: email, ActorEmail: email,
		Issue: ref, GitHubCreatedOn: createdOn,
		Respond: func(workItemID, number string) any {
			resp := domain.CreateProductWorkItemResponse{
				Message: "GitHub issue created.",
				Issue:   domain.ProductWorkItemIssue{URL: ref.HTMLURL, Number: ref.Number, Repo: ref.Owner + "/" + ref.Repository},
				Kind:    kind,
			}
			if workItemID != "" {
				resp.WorkItem = &domain.ProductWorkItemRef{ID: workItemID, Number: number}
			}
			return resp
		},
	}
	// The rest runs even if the caller has gone away: the issue exists.
	regCtx, regCancel := context.WithTimeout(context.WithoutCancel(ctx), productRecordTimeout)
	defer regCancel()
	notRecorded := func(step string, err error) error {
		var nfe *apierror.NotFoundError
		var ce *apierror.ConflictError
		if errors.As(err, &nfe) || errors.As(err, &ce) {
			return err
		}
		slog.ErrorContext(ctx, "product work item: the issue was created but a later step failed; a retry with the same key resumes there",
			"step", step, "caseId", req.CaseID, "repo", ref.Owner+"/"+ref.Repository, "number", ref.Number, "url", ref.HTMLURL, "err", err)
		return &apierror.ServiceUnavailableError{Msg: msgProductNotRecorded}
	}

	// 1. Registration (product issue only): rows in one transaction, work_item_id
	//    on the request, status still GITHUB_CREATED. A retry that finds a work
	//    item already recorded skips this and never inserts a second one.
	if kind == domain.ProductWorkItemKindProductIssue {
		priority := ""
		if strings.TrimSpace(req.IssueTypeLabel) == discussionIssueTypeLabel {
			priority = productIssuePriority(githubPriorityLabel(req.PriorityLevel))
		}
		params.Title = req.Title
		params.Priority = priority
		params.Category = strings.TrimSpace(req.IssueTypeLabel)
		params.ABTTeam = kase.Team
		params.PublicIssueURL = strings.TrimSpace(req.PublicIssueURL)
		// The labels as sent; if the mapping can no longer be resolved (a
		// resumed registration), the issue's own.
		if mapErr == nil {
			params.Labels = buildProductIssueLabels(mappingLabelForLabels(mapping), req)
		} else {
			params.Labels = detailLabels
		}
		out, err := s.repo.RegisterProductIssue(regCtx, params)
		if err != nil {
			return domain.CreateProductWorkItemResponse{}, false, notRecorded("register", err)
		}
		if out.Replayed {
			return s.storedResponse(out, false, cr.ID)
		}
	}

	// 2. The case's patch tag (idempotent by label) and the "GitHub issue filed"
	//    work note, through the case service so its follow-ups (comment_added
	//    event, dual-write ServiceNow mirror, watch list) run as for any note.
	if kind == domain.ProductWorkItemKindUntrackedPatch {
		if _, err := s.cases.AddCaseTagAs(regCtx, req.CaseID, patchCaseTag, email); err != nil {
			return domain.CreateProductWorkItemResponse{}, false, notRecorded("patch tag", err)
		}
	}
	if _, err := s.cases.CreateCaseCommentAs(regCtx, domain.CreateCaseCommentRequest{
		CaseID: req.CaseID, Type: domain.CommentTypeWorkNote, Content: "GitHub issue filed: " + ref.HTMLURL,
	}, email); err != nil {
		return domain.CreateProductWorkItemResponse{}, false, notRecorded("work note", err)
	}

	// 3. COMPLETED with the stored response.
	//
	// REMAINING DUPLICATE-NOTE WINDOW: the note is written by the case service in
	// its own transaction, so it cannot commit together with this write. If the
	// note succeeds and this COMPLETED write then fails (or the process dies
	// between them), the retry sees GITHUB_CREATED and writes the note a second
	// time. The tag cannot duplicate (it is idempotent by label); a second
	// "GitHub issue filed" note on the case is the whole cost.
	out, err := s.repo.CompleteRequest(regCtx, params)
	if err != nil {
		return domain.CreateProductWorkItemResponse{}, false, notRecorded("complete", err)
	}

	// 4. s_dip: the new issue is open, so the rule adds it. After COMPLETED, so
	//    a slow rule cannot use up regCtx and fail a request whose issue was
	//    filed; best effort (logged, never returned). A request that dies before
	//    this point is converged by its retry or by the next rule run.
	//    The SLA clock is registered here too (back-dated to the issue's GitHub
	//    creation), so a board that does not auto-add new issues still gets a
	//    clock before the first status event or the daily reconciliation.
	resp, created, err := s.storedResponse(out, !out.Replayed, cr.ID)
	if err == nil && kind == domain.ProductWorkItemKindProductIssue {
		if s.sla != nil && resp.WorkItem != nil && resp.WorkItem.ID != "" {
			slaCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), productCaseTagRuleTimeout)
			if regErr := s.sla.RegisterWorkItemClocks(slaCtx, resp.WorkItem.ID); regErr != nil {
				slog.ErrorContext(ctx, "product issue: SLA clock not registered at creation; the next status event or reconciliation registers it",
					"workItemId", resp.WorkItem.ID, "err", regErr)
			}
			cancel()
		}
		applyCaseTagRule(ctx, s.tagRule, req.CaseID)
	}
	return resp, created, err
}

func (s *productWorkItemService) storedResponse(out repository.RegisterOutcome, created bool, requestID string) (domain.CreateProductWorkItemResponse, bool, error) {
	var resp domain.CreateProductWorkItemResponse
	if err := json.Unmarshal(out.Response, &resp); err != nil {
		return domain.CreateProductWorkItemResponse{}, false, fmt.Errorf("response of creation request %s: %w", requestID, err)
	}
	return resp, created, nil
}

func mappingLabelForLabels(m domain.ProductRepoMapping) string {
	return strings.TrimSpace(m.GithubLabel)
}

// validateProductWorkItemRequest checks the body the way the portal backend does
// and normalises the text fields it writes into the issue and the hash.
func validateProductWorkItemRequest(req *domain.CreateProductWorkItemRequest) error {
	if !validate.IsUUID(req.CaseID) {
		return &apierror.ValidationError{Msg: fmt.Sprintf("id contains invalid UUID: %q", req.CaseID)}
	}
	req.IdempotencyKey = strings.TrimSpace(req.IdempotencyKey)
	if n := utf8.RuneCountInString(req.IdempotencyKey); n < 1 || n > maxIdempotencyKeyChars {
		return &apierror.ValidationError{Msg: "idempotencyKey is required and must be 1 to 200 characters."}
	}
	req.Title = strings.TrimSpace(req.Title)
	if n := utf8.RuneCountInString(req.Title); n < 1 || n > maxProductIssueTitleChars {
		return &apierror.ValidationError{Msg: msgProductTitleInvalid}
	}
	req.Description = strings.TrimSpace(req.Description)
	if utf8.RuneCountInString(req.Description) > maxProductIssueBodyChars {
		return &apierror.ValidationError{Msg: msgProductBodyTooLong}
	}
	return nil
}

// checkCaseState refuses a case that is not open for filing (the gate the
// existing github-issue route applies).
func checkCaseState(c repository.ProductWorkItemCase) error {
	if _, ok := caseGithubIssueActionableStates[c.State]; !ok {
		return &apierror.ConflictError{Msg: "Case is not in a state that allows filing a GitHub issue"}
	}
	return nil
}

// resolveRepository finds the product's repository: the deployed product's
// catalogue name first, then its versioned display name (the lookup's prefix
// match takes a trailing version). No match is a 400. The second result is the
// label the footer falls back to when the case has no product label.
func (s *productWorkItemService) resolveRepository(ctx context.Context, c repository.ProductWorkItemCase) (domain.ProductRepoMapping, string, error) {
	names := []string{c.ProductName, c.DisplayName}
	if c.ProductName == "" && c.DisplayName == "" {
		return domain.ProductRepoMapping{}, "", &apierror.ValidationError{Msg: msgProductRepoNotMapped}
	}
	rows, err := s.mappings.ListActive(ctx)
	if err != nil {
		return domain.ProductRepoMapping{}, "", err
	}
	for _, name := range names {
		if name == "" {
			continue
		}
		if m, ok := MatchProductRepo(rows, name); ok {
			if strings.TrimSpace(m.Owner) == "" || strings.TrimSpace(m.Repository) == "" {
				break
			}
			m.Owner, m.Repository = strings.TrimSpace(m.Owner), strings.TrimSpace(m.Repository)
			// github_label is NOT NULL but may be blank; the footer then falls back
			// to the case's own product name rather than dropping the Product line.
			label := strings.TrimSpace(m.GithubLabel)
			if label == "" {
				label = names[0]
				if label == "" {
					label = names[1]
				}
			}
			return m, label, nil
		}
	}
	return domain.ProductRepoMapping{}, "", &apierror.ValidationError{Msg: msgProductRepoNotMapped}
}

// fileIssue returns the GitHub issue for the request: an earlier attempt's, when
// there may be one, else a new one. It records GITHUB_CREATED before returning.
func (s *productWorkItemService) fileIssue(ctx context.Context, req domain.CreateProductWorkItemRequest, kase repository.ProductWorkItemCase,
	mapping domain.ProductRepoMapping, footerLabel string, cr repository.CreationRequest, email string) (repository.GitHubIssueRef, error) {

	footer := productIssueFooterContext{
		Product: firstNonEmpty(kase.ProductSlug, kase.DisplayName, kase.ProductName),
		Number:  kase.Number, InternalID: kase.InternalID, Team: kase.Team, OpenedBy: email,
	}
	body := buildProductIssueBody(req, footerLabel, footer)
	if utf8.RuneCountInString(body) > maxProductIssueBodyChars {
		s.recordOutcome(ctx, cr.ID, email, "refused before GitHub: body too long", ownsRow(cr))
		return repository.GitHubIssueRef{}, &apierror.ValidationError{Msg: msgProductBodyTooLong}
	}
	labels := buildProductIssueLabels(mappingLabelForLabels(mapping), req)
	owner, repoName := mapping.Owner, mapping.Repository

	phaseCtx, cancel := context.WithTimeout(ctx, productGitHubPhaseTimeout)
	defer cancel()

	// A PENDING row that this call did not insert and that did not fail
	// definitely may have an issue behind it: look before creating again.
	if !cr.Inserted && !cr.RetryOfFailed {
		found, err := s.findEarlierIssue(phaseCtx, owner, repoName, cr, req.Title, kase.InternalID, email)
		if err != nil {
			// Cannot tell whether the issue exists: creating could duplicate it.
			s.recordOutcome(ctx, cr.ID, email, "earlier issue search failed: "+err.Error(), false)
			return repository.GitHubIssueRef{}, upstreamFailure(err)
		}
		if found != nil {
			slog.InfoContext(ctx, "product work item: adopted the issue an earlier attempt created",
				"caseId", req.CaseID, "repo", owner+"/"+repoName, "number", found.Number)
			return s.recordCreated(ctx, cr.ID, email, repository.GitHubIssueRef{Owner: owner, Repository: repoName, Number: found.Number, NodeID: found.NodeID, HTMLURL: found.HTMLURL}, req.CaseID)
		}
	}

	// The mapping's owner is the GitHub organisation, so it is passed as both the
	// organisation and the owner the engineering entity asks for (as the portal
	// backend does); it picks its GitHub token by that organisation name.
	issue, err := s.eng.CreateIssue(phaseCtx, owner, owner, repoName, req.Title, body, labels)
	if err != nil {
		var ee *engineeringentity.Error
		switch {
		case errors.As(err, &ee) && ee.StatusCode == http.StatusNotFound:
			s.recordOutcome(ctx, cr.ID, email, "engineering entity: repository not found", ownsRow(cr))
			return repository.GitHubIssueRef{}, &apierror.ValidationError{Msg: msgProductRepoNotMapped}
		case errors.As(err, &ee) && ee.StatusCode == http.StatusTooManyRequests:
			s.recordOutcome(ctx, cr.ID, email, "engineering entity: rate limited", false)
			return repository.GitHubIssueRef{}, &UpstreamFailureError{Status: http.StatusServiceUnavailable, Msg: msgProductRateLimited, RetryAfter: ee.RetryAfter}
		case errors.As(err, &ee) && ee.Definite():
			s.recordOutcome(ctx, cr.ID, email, fmt.Sprintf("engineering entity refused the create (%d)", ee.StatusCode), ownsRow(cr))
		default:
			// Timeout, 5xx, network: GitHub may have created the issue. Stay PENDING.
			s.recordOutcome(ctx, cr.ID, email, "create ended without a clear answer: "+err.Error(), false)
		}
		slog.ErrorContext(ctx, "product work item: engineering entity create failed", "caseId", req.CaseID, "repo", owner+"/"+repoName, "err", err)
		return repository.GitHubIssueRef{}, upstreamFailure(err)
	}
	return s.recordCreated(ctx, cr.ID, email, repository.GitHubIssueRef{Owner: owner, Repository: repoName, Number: issue.Number, NodeID: issue.NodeID}, req.CaseID)
}

// recordCreated writes GITHUB_CREATED at once, whether or not the caller is
// still there. If it fails the issue exists unrecorded; the retry's earlier-issue
// search adopts it.
func (s *productWorkItemService) recordCreated(ctx context.Context, requestID, actor string, ref repository.GitHubIssueRef, caseID string) (repository.GitHubIssueRef, error) {
	dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), productRecordTimeout)
	defer cancel()
	if err := s.repo.RecordGitHubCreated(dbCtx, requestID, actor, ref); err != nil {
		slog.ErrorContext(ctx, "product work item: the GitHub issue exists but its creation could not be recorded; a retry will adopt it",
			"caseId", caseID, "repo", ref.Owner+"/"+ref.Repository, "number", ref.Number, "err", err)
		// Best effort: mark the row ambiguous so the retry runs the earlier-issue
		// search (after the short ambiguous lease) instead of waiting out the
		// in-flight lease.
		s.recordOutcome(ctx, requestID, actor, fmt.Sprintf("issue %s/%s#%d created but not recorded: %v", ref.Owner, ref.Repository, ref.Number, err), false)
		return repository.GitHubIssueRef{}, &apierror.ServiceUnavailableError{Msg: msgProductNotRecorded}
	}
	return ref, nil
}

// ownsRow reports whether this attempt owns the row's history: it inserted it
// or claimed it after a definite failure. Only then can "GitHub certainly
// created nothing" be recorded (FAILED). A row inherited as PENDING may stand
// for an earlier attempt that created an issue, so a refusal now must leave it
// PENDING with a note, for a later retry's earlier-issue search.
func ownsRow(cr repository.CreationRequest) bool { return cr.Inserted || cr.RetryOfFailed }

func (s *productWorkItemService) recordOutcome(ctx context.Context, requestID, actor, note string, definite bool) {
	dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), productRecordTimeout)
	defer cancel()
	if err := s.repo.RecordOutcome(dbCtx, requestID, actor, note, definite); err != nil {
		slog.ErrorContext(ctx, "product work item: recording the outcome failed", "requestId", requestID, "err", err)
	}
}

// upstreamFailure maps an engineering entity failure to the caller-safe answer:
// a 502 with a generic message, whatever the upstream said.
func upstreamFailure(err error) error {
	if errors.Is(err, context.Canceled) {
		return err
	}
	return &UpstreamFailureError{Status: http.StatusBadGateway, Msg: msgProductUpstream}
}

// findEarlierIssue looks, among the repository's Origin/CS issues updated since a
// minute before the request was first made, for the issue an earlier attempt
// created: the same title, and the footer's exact "WSO2 case id" and "Opened by"
// lines. An issue already recorded against another request is never adopted.
func (s *productWorkItemService) findEarlierIssue(ctx context.Context, owner, repoName string, cr repository.CreationRequest, title, caseInternalID, email string) (*engineeringentity.IssueDetail, error) {
	since := cr.CreatedOn.Add(-time.Minute).UTC().Format(time.RFC3339)
	wantCase := footerLine("WSO2 case id", caseInternalID)
	wantBy := footerLine("Opened by", email)
	for page := 1; page <= adoptionMaxPages; page++ {
		issues, err := s.eng.ListIssueDetails(ctx, owner, owner, repoName, engineeringentity.ListIssueDetailsQuery{
			State: "all", Labels: []string{originLabel}, Since: since, PerPage: adoptionPageSize, Page: page,
		})
		if err != nil {
			return nil, err
		}
		if len(issues) == 0 {
			return nil, nil
		}
		for i := range issues {
			d := issues[i]
			if strings.TrimSpace(d.Title) != title || d.Body == nil || !bodyHasLines(*d.Body, wantCase, wantBy) {
				continue
			}
			claimed, err := s.repo.IssueClaimed(ctx, owner, repoName, d.Number)
			if err != nil {
				return nil, err
			}
			if !claimed {
				return &d, nil
			}
		}
	}
	return nil, nil
}

// bodyHasLines reports whether every wanted line appears in body as a whole line.
func bodyHasLines(body string, want ...string) bool {
	have := map[string]bool{}
	for _, l := range strings.Split(body, "\n") {
		have[strings.TrimRight(l, "\r")] = true
	}
	for _, w := range want {
		if !have[w] {
			return false
		}
	}
	return true
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
