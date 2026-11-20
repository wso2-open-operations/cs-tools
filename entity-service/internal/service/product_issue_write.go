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
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/validate"
)

const (
	maxProductIssueCommentChars = 60000

	msgPIWriteSignedIn      = "A signed-in user is required to act on a product issue."
	msgPICommentInvalid     = "A comment of 1 to 60000 characters is required."
	msgPIKeyInvalid         = "The Idempotency-Key must be 1 to 200 characters."
	msgPIKeyReused          = "This idempotency key was already used for a different request."
	msgPILinkInactive       = "The GitHub issue is no longer linked to this product issue."
	msgPINotWaitingOnCS     = "The issue is not waiting on customer support."
	msgPICommandPending     = "A request of this kind is already waiting to be delivered to GitHub."
	msgPINoTrackingBoard    = "No project board is configured for this issue's repository."
	msgPIAlreadyClosed      = "The GitHub issue is already closed."
	msgPICommandNotFailed   = "Only a failed request can be retried."
	msgPIUnsupportedStatus  = "Only WAITING_ON_PRODUCT_TEAM can be set."
	productIssueCommentNote = "(CS) wrote:"
)

// ProductIssueWriteService queues the portal's write actions on a product issue
// for delivery to GitHub (the worker, ProductIssueCommandWorker, delivers them).
// Every method is for internal callers only, and none changes the issue's stored
// board or GitHub state: that changes only when GitHub confirms it through the
// webhook or the reconciliation.
//
// Each method returns replay true when the Idempotency-Key matched an earlier
// request on the item: the first command is returned and nothing new is queued.
type ProductIssueWriteService interface {
	AddComment(ctx context.Context, workItemID, key string, req domain.AddProductIssueCommentRequest) (resp domain.AddProductIssueCommentResponse, replay bool, err error)
	SetBoardStatus(ctx context.Context, workItemID, key string, req domain.SetProductIssueBoardStatusRequest) (cmd domain.GitHubCommand, replay bool, err error)
	Close(ctx context.Context, workItemID, key string) (cmd domain.GitHubCommand, replay bool, err error)
	Retry(ctx context.Context, workItemID, commandID string) (domain.GitHubCommand, error)
}

type productIssueWriteService struct {
	repo   repository.GitHubCommandRepository
	access AccessService
}

// NewProductIssueWriteService constructs a ProductIssueWriteService.
func NewProductIssueWriteService(repo repository.GitHubCommandRepository, access AccessService) ProductIssueWriteService {
	return &productIssueWriteService{repo: repo, access: access}
}

// productIssueCommentBody is the text sent to GitHub for a portal comment (P6).
func productIssueCommentBody(displayName, text string) string {
	return "**" + displayName + "** " + productIssueCommentNote + "\n\n" + text
}

// cleanDisplayName makes a name safe inside the bold attribution line: one line,
// no emphasis, mention or link markup.
func cleanDisplayName(name string) string {
	name = strings.NewReplacer("*", "", "_", "", "`", "", "@", "", "[", "", "]", "", "<", "", ">", "",
		"\r", " ", "\n", " ", "\t", " ").Replace(name)
	return strings.Join(strings.Fields(name), " ")
}

func (s *productIssueWriteService) caller(ctx context.Context, workItemID, key string) (email string, cleanKey string, err error) {
	if err := RequireInternalCaller(ctx, s.access, productWorkItemForbidden); err != nil {
		return "", "", err
	}
	if !validate.IsUUID(workItemID) {
		return "", "", &apierror.ValidationError{Msg: fmt.Sprintf("id contains invalid UUID: %q", workItemID)}
	}
	email = strings.TrimSpace(auth.IdentityFromContext(ctx).UserEmail)
	if email == "" {
		return "", "", &apierror.ValidationError{Msg: msgPIWriteSignedIn}
	}
	cleanKey = strings.TrimSpace(key)
	if cleanKey != "" && utf8.RuneCountInString(cleanKey) > maxIdempotencyKeyChars {
		return "", "", &apierror.ValidationError{Msg: msgPIKeyInvalid}
	}
	return email, cleanKey, nil
}

// link reads the issue's link, 404 for a work item that is not a product issue.
func (s *productIssueWriteService) link(ctx context.Context, workItemID string) (*repository.CommandLink, error) {
	l, err := s.repo.LinkForCommand(ctx, workItemID)
	if errors.Is(err, repository.ErrProductIssueLinkNotFound) {
		return nil, &apierror.NotFoundError{Msg: msgProductIssueNotFound}
	}
	return l, err
}

func conflict(msg, code string) error { return &apierror.ConflictError{Msg: msg, Code: code} }

// replayOf returns the command an Idempotency-Key was used for, when it is one
// of the given kind; a key used for another kind is refused.
func (s *productIssueWriteService) replayOf(ctx context.Context, workItemID, key, kind string) (*repository.GitHubCommand, error) {
	prev, err := s.repo.FindCommandByKey(ctx, workItemID, key)
	if err != nil || prev == nil {
		return nil, err
	}
	if prev.Kind != kind {
		return nil, conflict(msgPIKeyReused, apierror.CodeIdempotencyKeyReused)
	}
	return prev, nil
}

func (s *productIssueWriteService) AddComment(ctx context.Context, workItemID, key string, req domain.AddProductIssueCommentRequest) (domain.AddProductIssueCommentResponse, bool, error) {
	var zero domain.AddProductIssueCommentResponse
	email, key, err := s.caller(ctx, workItemID, key)
	if err != nil {
		return zero, false, err
	}
	text := strings.TrimSpace(req.Body)
	if n := utf8.RuneCountInString(text); n < 1 || n > maxProductIssueCommentChars {
		return zero, false, &apierror.ValidationError{Msg: msgPICommentInvalid}
	}
	l, err := s.link(ctx, workItemID)
	if err != nil {
		return zero, false, err
	}
	prev, err := s.replayOf(ctx, workItemID, key, repository.CommandAddComment)
	if err != nil {
		return zero, false, err
	}
	if prev == nil && l.LinkState != "ACTIVE" {
		return zero, false, conflict(msgPILinkInactive, apierror.CodeLinkInactive)
	}
	name := email
	if prev == nil {
		n, nerr := s.repo.UserDisplayName(ctx, email)
		if nerr != nil {
			// A name is a nicety: the email names the author just as well.
			slog.WarnContext(ctx, "product issue comment: reading the user's name failed; using the email", "err", nerr)
		}
		if n = cleanDisplayName(n); n != "" {
			name = n
		}
	}
	res, err := s.repo.InsertPortalComment(ctx, repository.PortalCommentInput{
		WorkItemID: workItemID, Content: text, GitHubBody: productIssueCommentBody(name, text),
		IdempotencyKey: key, RequestedBy: email,
	})
	if errors.Is(err, repository.ErrIdempotencyKeyReused) {
		return zero, false, conflict(msgPIKeyReused, apierror.CodeIdempotencyKeyReused)
	}
	if err != nil {
		return zero, false, err
	}
	cmd := res.Command.Domain()
	delivery := res.Command.Status
	origin := repository.CommentOriginPortal
	typ := "WORK_NOTE"
	comment := domain.ProductIssueComment{
		ID: res.Comment.ID, Type: &typ, Author: res.Comment.Author, Body: res.Comment.Body, CreatedOn: res.Comment.CreatedOn,
		Origin: &origin, CommandID: &cmd.ID,
	}
	if delivery != "" {
		comment.Delivery = &delivery
	}
	return domain.AddProductIssueCommentResponse{Comment: comment, Command: cmd}, res.Existed, nil
}

// checkMove is the state a move to WOW needs.
func checkMove(l *repository.CommandLink) error {
	switch {
	case l.LinkState != "ACTIVE":
		return conflict(msgPILinkInactive, apierror.CodeLinkInactive)
	case l.BoardStatus == nil || *l.BoardStatus != "WAITING_ON_CS":
		return conflict(msgPINotWaitingOnCS, apierror.CodeNotWaitingOnCS)
	case l.BoardProjectNumber == nil:
		return conflict(msgPINoTrackingBoard, apierror.CodeNoTrackingBoard)
	}
	return nil
}

// checkClose is the state a close needs.
func checkClose(l *repository.CommandLink) error {
	switch {
	case l.LinkState != "ACTIVE":
		return conflict(msgPILinkInactive, apierror.CodeLinkInactive)
	case strings.EqualFold(l.GitHubState, domain.ProductIssueStateClosed):
		return conflict(msgPIAlreadyClosed, apierror.CodeAlreadyClosed)
	}
	return nil
}

// insert queues a move or close; ErrCommandPending is the 409 command_pending.
func (s *productIssueWriteService) insert(ctx context.Context, in repository.NewGitHubCommand) (domain.GitHubCommand, bool, error) {
	c, existed, err := s.repo.InsertCommand(ctx, in)
	if errors.Is(err, repository.ErrCommandPending) {
		return domain.GitHubCommand{}, false, conflict(msgPICommandPending, apierror.CodeCommandPending)
	}
	if errors.Is(err, repository.ErrIdempotencyKeyReused) {
		return domain.GitHubCommand{}, false, conflict(msgPIKeyReused, apierror.CodeIdempotencyKeyReused)
	}
	if err != nil {
		return domain.GitHubCommand{}, false, err
	}
	return c.Domain(), existed, nil
}

func (s *productIssueWriteService) SetBoardStatus(ctx context.Context, workItemID, key string, req domain.SetProductIssueBoardStatusRequest) (domain.GitHubCommand, bool, error) {
	var zero domain.GitHubCommand
	email, key, err := s.caller(ctx, workItemID, key)
	if err != nil {
		return zero, false, err
	}
	if strings.TrimSpace(req.Status) != domain.ProductIssueBoardStatusWaitingOnProductTeam {
		return zero, false, &apierror.ValidationError{Msg: msgPIUnsupportedStatus, Code: apierror.CodeUnsupportedStatus}
	}
	l, err := s.link(ctx, workItemID)
	if err != nil {
		return zero, false, err
	}
	prev, err := s.replayOf(ctx, workItemID, key, repository.CommandSetBoardStatus)
	if err != nil {
		return zero, false, err
	}
	if prev != nil {
		return prev.Domain(), true, nil
	}
	if err := checkMove(l); err != nil {
		return zero, false, err
	}
	payload, _ := json.Marshal(map[string]string{"status": domain.ProductIssueBoardStatusWaitingOnProductTeam, "rawStatus": "WOW"})
	return s.insert(ctx, repository.NewGitHubCommand{
		WorkItemID: workItemID, Kind: repository.CommandSetBoardStatus, Payload: payload, IdempotencyKey: key, RequestedBy: email,
	})
}

func (s *productIssueWriteService) Close(ctx context.Context, workItemID, key string) (domain.GitHubCommand, bool, error) {
	var zero domain.GitHubCommand
	email, key, err := s.caller(ctx, workItemID, key)
	if err != nil {
		return zero, false, err
	}
	l, err := s.link(ctx, workItemID)
	if err != nil {
		return zero, false, err
	}
	prev, err := s.replayOf(ctx, workItemID, key, repository.CommandSetState)
	if err != nil {
		return zero, false, err
	}
	if prev != nil {
		return prev.Domain(), true, nil
	}
	if err := checkClose(l); err != nil {
		return zero, false, err
	}
	payload, _ := json.Marshal(map[string]string{"state": "closed", "stateReason": "completed"})
	return s.insert(ctx, repository.NewGitHubCommand{
		WorkItemID: workItemID, Kind: repository.CommandSetState, Payload: payload, IdempotencyKey: key, RequestedBy: email,
	})
}

func (s *productIssueWriteService) Retry(ctx context.Context, workItemID, commandID string) (domain.GitHubCommand, error) {
	var zero domain.GitHubCommand
	if _, _, err := s.caller(ctx, workItemID, ""); err != nil {
		return zero, err
	}
	if !validate.IsUUID(commandID) {
		return zero, &apierror.NotFoundError{Msg: "command not found"}
	}
	l, err := s.link(ctx, workItemID)
	if err != nil {
		return zero, err
	}
	cmd, err := s.repo.Get(ctx, commandID)
	if err != nil {
		return zero, err
	}
	if cmd == nil || cmd.WorkItemID != workItemID {
		return zero, &apierror.NotFoundError{Msg: "command not found"}
	}
	if cmd.Status != repository.CommandFailed {
		return zero, conflict(msgPICommandNotFailed, apierror.CodeCommandNotFailed)
	}
	switch cmd.Kind {
	case repository.CommandAddComment:
		if l.LinkState != "ACTIVE" {
			return zero, conflict(msgPILinkInactive, apierror.CodeLinkInactive)
		}
	case repository.CommandSetBoardStatus:
		err = checkMove(l)
	case repository.CommandSetState:
		err = checkClose(l)
	}
	if err != nil {
		return zero, err
	}
	out, err := s.repo.Retry(ctx, workItemID, commandID)
	switch {
	case errors.Is(err, repository.ErrCommandNotFound):
		return zero, &apierror.NotFoundError{Msg: "command not found"}
	case errors.Is(err, repository.ErrCommandNotFailed):
		return zero, conflict(msgPICommandNotFailed, apierror.CodeCommandNotFailed)
	case errors.Is(err, repository.ErrCommandPending):
		return zero, conflict(msgPICommandPending, apierror.CodeCommandPending)
	case err != nil:
		return zero, err
	}
	return out.Domain(), nil
}
