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
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The inbound webhook branch for product issues (git internals). It sits in front
// of the customer-repository sync: a delivery about an issue that has a
// github_issue_link row (or about a board item whose content is one) is applied
// here and the old path never sees it; every other delivery goes to the old path
// exactly as it arrived.

// projectAutomationLogin is GitHub's board automation account. It sets the
// initial Open status of every new issue, so it is never treated as the portal's
// own bot even if it is listed in GIT_INTERNALS_BOT_LOGINS.
const projectAutomationLogin = "github-project-automation[bot]"

// maxIssueSubject is work_item.subject's length; a GitHub title can be longer.
const maxIssueSubject = 255

// webhookPayload is the part of a delivery the product issue branch reads. It is
// decoded from the same bytes as IssuePayload, which has no node ids, issue
// timestamps or project fields.
type webhookPayload struct {
	Action string `json:"action"`
	Issue  struct {
		Number      int        `json:"number"`
		NodeID      string     `json:"node_id"`
		Title       string     `json:"title"`
		State       string     `json:"state"`
		StateReason *string    `json:"state_reason"`
		UpdatedAt   *time.Time `json:"updated_at"`
		ClosedAt    *time.Time `json:"closed_at"`
		Labels      []struct {
			Name string `json:"name"`
		} `json:"labels"`
	} `json:"issue"`
	Comment *struct {
		ID        int64      `json:"id"`
		Body      string     `json:"body"`
		CreatedAt *time.Time `json:"created_at"`
		UpdatedAt *time.Time `json:"updated_at"`
		User      struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"comment"`
	Item *struct {
		ID            int64      `json:"id"`
		ProjectNodeID string     `json:"project_node_id"`
		ContentNodeID string     `json:"content_node_id"`
		ContentType   string     `json:"content_type"`
		UpdatedAt     *time.Time `json:"updated_at"`
	} `json:"projects_v2_item"`
	Changes struct {
		FieldValue *struct {
			FieldName     string          `json:"field_name"`
			FieldType     string          `json:"field_type"`
			ProjectNumber *int            `json:"project_number"`
			To            json.RawMessage `json:"to"`
		} `json:"field_value"`
	} `json:"changes"`
	Organization struct {
		Login string `json:"login"`
	} `json:"organization"`
	Repository struct {
		Name  string `json:"name"`
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
	} `json:"repository"`
	Sender struct {
		Login string `json:"login"`
	} `json:"sender"`
}

func (p webhookPayload) labelNames() []string {
	out := make([]string, 0, len(p.Issue.Labels))
	for _, l := range p.Issue.Labels {
		out = append(out, l.Name)
	}
	return out
}

// ProductIssueDeliveries applies deliveries about linked product issues.
type ProductIssueDeliveries struct {
	repo repository.ProductIssueSyncRepository
	bots map[string]bool

	// boardNumbers remembers which project node id is which project number. A
	// Status edit carries both; a board item "created" event carries only the node
	// id. In memory and per process: a cold cache costs nothing but the created
	// event, since the Status edit that follows records the item id too.
	mu           sync.Mutex
	boardNumbers map[string]int
}

// NewProductIssueDeliveries builds the branch. botLogins are the portal's own
// GitHub accounts (GIT_INTERNALS_BOT_LOGINS): comments they post are not mirrored
// back. Nothing is ignored unless listed.
func NewProductIssueDeliveries(repo repository.ProductIssueSyncRepository, botLogins []string) *ProductIssueDeliveries {
	bots := map[string]bool{}
	for _, l := range botLogins {
		if l = strings.ToLower(strings.TrimSpace(l)); l != "" && l != projectAutomationLogin {
			bots[l] = true
		}
	}
	return &ProductIssueDeliveries{repo: repo, bots: bots, boardNumbers: map[string]int{}}
}

func (p *ProductIssueDeliveries) isBot(login string) bool {
	return p.bots[strings.ToLower(strings.TrimSpace(login))]
}

func (p *ProductIssueDeliveries) learnBoard(nodeID string, number int) {
	if nodeID == "" {
		return
	}
	p.mu.Lock()
	p.boardNumbers[nodeID] = number
	p.mu.Unlock()
}

func (p *ProductIssueDeliveries) knownBoard(nodeID string) (int, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	n, ok := p.boardNumbers[nodeID]
	return n, ok
}

// Handle applies the delivery when it concerns a linked product issue. handled is
// false when it does not (the caller runs the old path); an error is returned
// only for a delivery that was ours and failed, or replayed (repository.ErrDeliverySeen).
func (p *ProductIssueDeliveries) Handle(ctx context.Context, d Delivery) (out Outcome, handled bool, err error) {
	switch d.Event {
	case "issues", "issue_comment", "projects_v2_item":
	default:
		return Outcome{}, false, nil
	}
	if len(d.Raw) == 0 {
		return Outcome{}, false, nil
	}
	var wp webhookPayload
	if err := json.Unmarshal(d.Raw, &wp); err != nil {
		return Outcome{}, false, nil // the old path has its own view of a malformed body
	}

	var link *repository.LinkedIssue
	if d.Event == "projects_v2_item" {
		if wp.Item == nil || wp.Item.ContentNodeID == "" || (wp.Item.ContentType != "" && wp.Item.ContentType != "Issue") {
			return Outcome{}, false, nil
		}
		link, err = p.repo.LinkByNodeID(ctx, wp.Item.ContentNodeID)
	} else {
		if wp.Repository.Owner.Login == "" || wp.Repository.Name == "" || wp.Issue.Number <= 0 {
			return Outcome{}, false, nil
		}
		link, err = p.repo.LinkByIssue(ctx, wp.Repository.Owner.Login, wp.Repository.Name, wp.Issue.Number)
	}
	if err != nil {
		// Not knowing whether the delivery is ours must not break the customer
		// repository sync: treat it as unlinked.
		slog.WarnContext(ctx, "github: product issue link lookup failed; the delivery goes to the customer repository path",
			"delivery", d.ID, "event", d.Event, "err", err)
		return Outcome{}, false, nil
	}
	if link == nil {
		return Outcome{}, false, nil
	}

	run, skipped, err := p.plan(ctx, d, wp, link)
	if err != nil {
		return Outcome{}, true, err
	}
	if run == nil {
		return Outcome{Skipped: skipped}, true, nil
	}

	// Only a delivery that will write is claimed: ignored ones leave no row, as on
	// the customer path. The claim is inserted by the write's own transaction, so a
	// failed write leaves none and GitHub's retry runs; a replay of a committed
	// delivery comes back as repository.ErrDeliverySeen.
	out, err = run(repository.WithDeliveryClaim(ctx, repository.DeliveryClaim{ID: d.ID, Event: d.Event, Action: wp.Action}))
	if err != nil {
		return Outcome{}, true, err
	}
	return out, true, nil
}

// plan decides what a linked delivery needs. It returns the write to run after
// the delivery's claim, or the reason there is nothing to do.
func (p *ProductIssueDeliveries) plan(ctx context.Context, d Delivery, wp webhookPayload, link *repository.LinkedIssue) (func(ctx context.Context) (Outcome, error), string, error) {
	if link.LinkState != repository.LinkStateActive {
		return nil, "the issue's link is " + strings.ToLower(link.LinkState), nil
	}
	switch d.Event {
	case "issues":
		return p.planIssue(d, wp, link)
	case "issue_comment":
		return p.planComment(d, wp, link)
	default:
		return p.planBoardItem(ctx, d, wp, link)
	}
}

func (p *ProductIssueDeliveries) planIssue(d Delivery, wp webhookPayload, link *repository.LinkedIssue) (func(ctx context.Context) (Outcome, error), string, error) {
	id := link.WorkItemID
	switch wp.Action {
	case "deleted":
		return func(ctx context.Context) (Outcome, error) {
			return Outcome{Action: "product_issue_deleted"}, p.repo.SetLinkState(ctx, id, repository.LinkStateDeleted)
		}, "", nil
	case "transferred":
		// No transferred delivery was captured and GitHub's changes.new_issue shape is
		// not confirmed from a fixture, so the link is only marked and the move left
		// to a person: the issue's later events arrive under its new address, which
		// has no link, and take the customer path.
		return func(ctx context.Context) (Outcome, error) {
			slog.Warn("github: a product issue was transferred; its link is marked TRANSFERRED and needs a manual re-link",
				"workItemId", id, "delivery", d.ID)
			return Outcome{Action: "product_issue_transferred"}, p.repo.SetLinkState(ctx, id, repository.LinkStateTransferred)
		}, "", nil
	case "opened", "edited", "labeled", "unlabeled", "closed", "reopened":
	default:
		return nil, "issue action " + wp.Action + " is not handled", nil
	}
	labels := wp.labelNames()
	state := strings.ToUpper(wp.Issue.State)
	reason := ""
	if wp.Issue.StateReason != nil {
		reason = *wp.Issue.StateReason
	}
	occurred := time.Now().UTC()
	switch {
	case wp.Action == "closed" && wp.Issue.ClosedAt != nil:
		occurred = *wp.Issue.ClosedAt
	case wp.Issue.UpdatedAt != nil:
		occurred = *wp.Issue.UpdatedAt
	}
	snap := repository.IssueSnapshot{
		WorkItemID: id, Title: truncateRunes(wp.Issue.Title, maxIssueSubject), State: state, StateReason: reason,
		Labels: labels, Priority: issuePriorityFromLabels(labels), Category: issueCategoryFromLabels(labels),
		UpdatedOn: wp.Issue.UpdatedAt, ClosedOn: wp.Issue.ClosedAt,
		RecordStateEvent: wp.Action == "closed" || wp.Action == "reopened",
		DedupeKey:        d.ID, Actor: wp.Sender.Login, OccurredAt: occurred,
	}
	return func(ctx context.Context) (Outcome, error) {
		res, err := p.repo.ApplyIssueSnapshot(ctx, snap)
		if err != nil {
			return Outcome{}, err
		}
		if res.Stale {
			return Outcome{Action: "product_issue_stale"}, nil
		}
		return Outcome{Action: "product_issue_" + wp.Action}, nil
	}, "", nil
}

func (p *ProductIssueDeliveries) planComment(d Delivery, wp webhookPayload, link *repository.LinkedIssue) (func(ctx context.Context) (Outcome, error), string, error) {
	// Only a comment can come back as new content, so only a comment is guarded on
	// the sender, as on the customer path.
	if p.isBot(wp.Sender.Login) {
		return nil, "comment was sent by the portal's own account", nil
	}
	if wp.Comment == nil || wp.Comment.ID == 0 {
		return nil, "delivery carries no comment", nil
	}
	author := wp.Comment.User.Login
	if author == "" {
		author = wp.Sender.Login
	}
	in := repository.GitHubCommentInput{
		WorkItemID: link.WorkItemID, GitHubCommentID: wp.Comment.ID, Author: author, Body: wp.Comment.Body,
		UpdatedOn: wp.Comment.UpdatedAt, IssueUpdatedOn: wp.Issue.UpdatedAt, Editor: wp.Sender.Login,
	}
	if wp.Comment.CreatedAt != nil {
		in.CreatedOn = *wp.Comment.CreatedAt
	}
	switch wp.Action {
	case "created":
		return func(ctx context.Context) (Outcome, error) {
			res, err := p.repo.MirrorGitHubComment(ctx, in)
			return commentOutcome("product_issue_comment_created", res), err
		}, "", nil
	case "edited":
		return func(ctx context.Context) (Outcome, error) {
			res, err := p.repo.UpdateGitHubComment(ctx, in)
			return commentOutcome("product_issue_comment_edited", res), err
		}, "", nil
	case "deleted":
		return func(ctx context.Context) (Outcome, error) {
			res, err := p.repo.DeleteGitHubComment(ctx, in, wp.Sender.Login)
			return commentOutcome("product_issue_comment_deleted", res), err
		}, "", nil
	}
	return nil, "comment action " + wp.Action + " is not handled", nil
}

func commentOutcome(action string, r repository.CommentOutcome) Outcome {
	if r.Skipped {
		return Outcome{Action: action, Skipped: "comment already recorded or unchanged"}
	}
	return Outcome{Action: action}
}

func (p *ProductIssueDeliveries) planBoardItem(ctx context.Context, d Delivery, wp webhookPayload, link *repository.LinkedIssue) (func(ctx context.Context) (Outcome, error), string, error) {
	item := wp.Item
	itemID := strconv.FormatInt(item.ID, 10)
	switch wp.Action {
	case "created":
		number, ok := p.knownBoard(item.ProjectNodeID)
		if !ok {
			return nil, "the item's board is not known yet; its Status edit records it", nil
		}
		return p.boardGate(ctx, d, link, wp, number, func(ctx context.Context) (Outcome, error) {
			return Outcome{Action: "product_issue_board_item"}, p.repo.RecordBoardItem(ctx, link.WorkItemID, itemID, number)
		})
	case "edited":
		fv := wp.Changes.FieldValue
		if fv == nil || fv.FieldName != "Status" || fv.ProjectNumber == nil {
			return nil, "not a Status field change", nil
		}
		// A Status column is a single select; a field of any other type that happens to
		// be named Status is not the board column.
		if fv.FieldType != "" && fv.FieldType != "single_select" {
			return nil, "the Status field is not a single select", nil
		}
		number := *fv.ProjectNumber
		p.learnBoard(item.ProjectNodeID, number)
		var to struct {
			Name string `json:"name"`
		}
		if len(fv.To) > 0 && string(fv.To) != "null" {
			_ = json.Unmarshal(fv.To, &to)
		}
		raw := strings.TrimSpace(to.Name)
		if raw == "" {
			return nil, "the Status was cleared", nil
		}
		occurred := time.Now().UTC()
		if item.UpdatedAt != nil {
			occurred = *item.UpdatedAt
		}
		return p.boardGate(ctx, d, link, wp, number, func(ctx context.Context) (Outcome, error) {
			_, err := p.repo.RecordBoardStatus(ctx, repository.BoardStatusParams{
				WorkItemID: link.WorkItemID, ProjectNumber: number, ItemID: itemID, RawStatus: raw,
				DedupeKey: d.ID, Source: repository.StatusSourceWebhook, Actor: wp.Sender.Login, OccurredAt: occurred,
			})
			return Outcome{Action: "product_issue_board_status"}, err
		})
	}
	return nil, "board item action " + wp.Action + " is not handled", nil
}

// boardGate lets run through only when number is the tracking board of the
// issue's product: the board number of a product_repo_mapping row for the
// link's repository, in the link's organisation.
func (p *ProductIssueDeliveries) boardGate(ctx context.Context, d Delivery, link *repository.LinkedIssue, wp webhookPayload, number int, run func(ctx context.Context) (Outcome, error)) (func(ctx context.Context) (Outcome, error), string, error) {
	// A board event is applied only when its organisation is the link's: without the
	// organisation the check cannot be made, so the event is refused.
	org := wp.Organization.Login
	if org == "" {
		slog.WarnContext(ctx, "github: a board event names no organisation; refused", "delivery", d.ID, "workItemId", link.WorkItemID)
		return nil, "the board event names no organisation", nil
	}
	if !strings.EqualFold(org, link.Owner) {
		return nil, "the board belongs to another organisation", nil
	}
	ok, err := p.repo.IsTrackingBoard(ctx, link.Owner, link.Repository, number)
	if err != nil {
		return nil, "", err
	}
	if !ok {
		return nil, "board " + strconv.Itoa(number) + " is not the product's tracking board", nil
	}
	return run, "", nil
}

// productIssueFrontDoor puts the product issue branch in front of the existing
// GithubSyncService.
type productIssueFrontDoor struct {
	inner GithubSyncService // nil when the customer-repository sync is off
	front *ProductIssueDeliveries
}

// WithProductIssueDeliveries returns a GithubSyncService that offers every
// delivery to front first. inner may be nil (the customer sync is off): a delivery
// that is not about a linked product issue is then skipped.
func WithProductIssueDeliveries(inner GithubSyncService, front *ProductIssueDeliveries) GithubSyncService {
	return &productIssueFrontDoor{inner: inner, front: front}
}

// HandleWebhook implements GithubSyncService.
func (f *productIssueFrontDoor) HandleWebhook(ctx context.Context, d Delivery) (Outcome, error) {
	out, handled, err := f.front.Handle(ctx, d)
	if handled || err != nil {
		return out, err
	}
	if f.inner == nil {
		return skip("not a linked product issue and the customer repository sync is off")
	}
	return f.inner.HandleWebhook(ctx, d)
}

var errCustomerSyncOff = errors.New("github: the customer repository sync is not enabled")

// CreateServiceRequestFromIssue implements GithubSyncService.
func (f *productIssueFrontDoor) CreateServiceRequestFromIssue(ctx context.Context, req domain.CreateServiceRequestFromIssueRequest) (domain.CreateServiceRequestFromIssueResponse, error) {
	if f.inner == nil {
		return domain.CreateServiceRequestFromIssueResponse{}, errCustomerSyncOff
	}
	return f.inner.CreateServiceRequestFromIssue(ctx, req)
}

// SetServiceRequestSeverityFromIssue implements GithubSyncService.
func (f *productIssueFrontDoor) SetServiceRequestSeverityFromIssue(ctx context.Context, owner, repository string, issueNumber int, severity domain.CaseSeverity) (domain.SetIssueSeverityResponse, error) {
	if f.inner == nil {
		return domain.SetIssueSeverityResponse{}, errCustomerSyncOff
	}
	return f.inner.SetServiceRequestSeverityFromIssue(ctx, owner, repository, issueNumber, severity)
}
