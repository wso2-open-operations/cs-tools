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
	"log/slog"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type conversationService struct {
	repo repository.ConversationRepository
	// snWriteback/snMirror are set only under
	// DATA_SOURCE=postgres-servicenow-dual-write (via
	// NewConversationServiceWithSNWriteback) and are nil in every other mode.
	// CreateConversation calls snMirror synchronously, ServiceNow-first, so
	// the Postgres row carries ServiceNow's own id and every later mirror
	// write (UpdateConversation here, the conversation's comments in
	// commentService) targets a record ServiceNow knows. UpdateConversation
	// mirrors asynchronously through snWriteback.
	snWriteback *SNWritebackDispatcher
	snMirror    ConversationService
}

// NewConversationService constructs a ConversationService backed by Postgres.
func NewConversationService(repo repository.ConversationRepository) ConversationService {
	return &conversationService{repo: repo}
}

// NewConversationServiceWithSNWriteback is NewConversationService plus the
// wiring DATA_SOURCE=postgres-servicenow-dual-write needs: UpdateConversation
// dispatches a best-effort, asynchronous ServiceNow mirror write onto mirror
// after the Postgres write commits -- see conversationService's own
// snWriteback/snMirror doc comment.
func NewConversationServiceWithSNWriteback(repo repository.ConversationRepository, dispatcher *SNWritebackDispatcher, mirror ConversationService) ConversationService {
	return &conversationService{repo: repo, snWriteback: dispatcher, snMirror: mirror}
}

// SearchConversations implements ConversationService.
func (s *conversationService) SearchConversations(ctx context.Context, req domain.SearchConversationsRequest) (domain.SearchConversationsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchConversationsResponse{}, err
	}
	if err := validateUUIDs("filters.projectIds", req.Filters.ProjectIDs); err != nil {
		return domain.SearchConversationsResponse{}, err
	}
	if start, end := req.Filters.StartUpdatedDate, req.Filters.EndUpdatedDate; start != nil && end != nil && end.Before(*start) {
		return domain.SearchConversationsResponse{}, &apierror.ValidationError{Msg: "filters.endUpdatedDate must not be before filters.startUpdatedDate"}
	}

	// callerEmail is only needed to resolve filters.createdByMe -- a request
	// that doesn't set it works fine with no caller identity at all, so a
	// missing/unparsable token isn't an error here the way it is for a
	// write (e.g. UpdateConversation).
	var callerEmail string
	if req.Filters.CreatedByMe {
		token := middleware.UserIDTokenFromContext(ctx)
		email, err := emailFromJWT(token)
		if err != nil {
			return domain.SearchConversationsResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required to filter by createdByMe"}
		}
		callerEmail = email
	}

	views, total, err := s.repo.SearchConversations(ctx, req, callerEmail)
	if err != nil {
		return domain.SearchConversationsResponse{}, err
	}

	return domain.SearchConversationsResponse{
		Conversations: views,
		Total:         total,
		Limit:         req.Pagination.Limit,
		Offset:        req.Pagination.Offset,
	}, nil
}

// GetConversation implements ConversationService.
func (s *conversationService) GetConversation(ctx context.Context, id string) (domain.ConversationDetails, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.ConversationDetails{}, err
	}
	return s.repo.GetConversation(ctx, id)
}

// conversationSubjectLength is how much of the first message becomes
// work_item.subject -- the same 100-character truncation csm-sync-service's
// u_chat_conversation mapping applies to u_initial_message.
const conversationSubjectLength = 100

// CreateConversation implements ConversationService. The conversation starts
// ACTIVE: a chat is live from its first message. Under dual-write ServiceNow
// creates it first (see conversationService's snMirror doc comment); a
// ServiceNow failure then writes nothing to Postgres.
func (s *conversationService) CreateConversation(ctx context.Context, req domain.CreateConversationRequest) (domain.CreateConversationResponse, error) {
	if err := validateUUIDs("projectId", []string{req.ProjectID}); err != nil {
		return domain.CreateConversationResponse{}, err
	}
	if req.InitialMessage == "" {
		return domain.CreateConversationResponse{}, &apierror.ValidationError{Msg: "initialMessage is required"}
	}

	token := middleware.UserIDTokenFromContext(ctx)
	callerEmail, err := emailFromJWT(token)
	if err != nil {
		return domain.CreateConversationResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

	in := repository.CreateConversationInput{
		ProjectID:      req.ProjectID,
		Subject:        truncateRunes(req.InitialMessage, conversationSubjectLength),
		InitialMessage: req.InitialMessage,
		CreatedBy:      callerEmail,
		State:          domain.ConversationStateActive,
	}

	if s.snMirror != nil {
		snResp, err := s.snMirror.CreateConversation(ctx, req)
		if err != nil {
			return domain.CreateConversationResponse{}, err
		}
		in.ID = snResp.Conversation.ID
		in.Number = snResp.Conversation.Number
		if snResp.Conversation.State != nil {
			in.State = domain.ConversationState(*snResp.Conversation.State)
		}
	}

	created, err := s.repo.CreateConversation(ctx, in)
	if err != nil {
		if s.snMirror != nil {
			// ServiceNow already has the conversation: this is drift needing
			// operator attention, same as createProblemSNFirst's equivalent.
			slog.ErrorContext(ctx, "sn create conversation: ServiceNow conversation created but the Postgres insert failed",
				"conversationID", in.ID, "number", in.Number, "err", err)
		}
		return domain.CreateConversationResponse{}, err
	}
	return domain.CreateConversationResponse{Message: "Conversation created successfully", Conversation: created}, nil
}

// UpdateConversation implements ConversationService.
func (s *conversationService) UpdateConversation(ctx context.Context, id string, req domain.UpdateConversationRequest) (domain.UpdateConversationResponse, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.UpdateConversationResponse{}, err
	}
	if !validConversationUpdateState[req.State] {
		return domain.UpdateConversationResponse{}, &apierror.ValidationError{Msg: "state contains invalid value: " + string(req.State)}
	}

	token := middleware.UserIDTokenFromContext(ctx)
	callerEmail, err := emailFromJWT(token)
	if err != nil {
		return domain.UpdateConversationResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

	updated, err := s.repo.UpdateConversation(ctx, id, req.State, callerEmail)
	if err != nil {
		return domain.UpdateConversationResponse{}, err
	}

	// Best-effort ServiceNow mirror write, DATA_SOURCE=postgres-servicenow-dual-write
	// only (snWriteback/snMirror are both nil otherwise -- see
	// conversationService's own doc comment). Postgres has already committed
	// by this point.
	if s.snWriteback != nil {
		s.snWriteback.Dispatch(ctx, "conversation", id, "update", req,
			func(writeCtx context.Context) error {
				_, err := s.snMirror.UpdateConversation(writeCtx, id, req)
				return err
			},
		)
	}

	return domain.UpdateConversationResponse{
		Message:      "Conversation updated successfully",
		Conversation: updated,
	}, nil
}

// truncateRunes returns at most n runes of v, never splitting a multi-byte
// character.
func truncateRunes(v string, n int) string {
	r := []rune(v)
	if len(r) <= n {
		return v
	}
	return string(r[:n])
}
