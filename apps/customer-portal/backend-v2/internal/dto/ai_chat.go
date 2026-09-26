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

package dto

import (
	"errors"
	"fmt"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/aichatagent"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// CaseClassificationRequest is the portal's request shape for POST /cases/classify.
// Every field here is customer-context the AI agent needs — decoded directly
// into aichatagent.CaseClassificationPayload, no restriction needed.
type CaseClassificationRequest struct {
	ChatHistory   string              `json:"chatHistory"`
	EnvProducts   map[string][]string `json:"envProducts"`
	Region        string              `json:"region"`
	Tier          string              `json:"tier"`
	ProjectTypeID string              `json:"projectTypeId"`
}

// BuildCaseClassificationPayload converts the portal's request into the AI
// chat agent's request shape.
func BuildCaseClassificationPayload(req CaseClassificationRequest) aichatagent.CaseClassificationPayload {
	return aichatagent.CaseClassificationPayload{
		ChatHistory:   req.ChatHistory,
		EnvProducts:   req.EnvProducts,
		Region:        req.Region,
		Tier:          req.Tier,
		ProjectTypeID: req.ProjectTypeID,
	}
}

// CaseClassificationResponse is the portal's response for POST /cases/classify.
type CaseClassificationResponse struct {
	IssueType     string `json:"issueType"`
	SeverityLevel string `json:"severityLevel"`
	CaseInfo      struct {
		Description      string `json:"description"`
		ShortDescription string `json:"shortDescription"`
		ProductName      string `json:"productName"`
		ProductVersion   string `json:"productVersion"`
		Environment      string `json:"environment"`
		Tier             string `json:"tier"`
		Region           string `json:"region"`
	} `json:"caseInfo"`
}

// MapCaseClassification builds the portal response from the AI chat agent's
// CaseClassificationResponse.
func MapCaseClassification(r aichatagent.CaseClassificationResponse) CaseClassificationResponse {
	out := CaseClassificationResponse{
		IssueType:     r.IssueType,
		SeverityLevel: r.SeverityLevel,
	}
	out.CaseInfo.Description = r.CaseInfo.Description
	out.CaseInfo.ShortDescription = r.CaseInfo.ShortDescription
	out.CaseInfo.ProductName = r.CaseInfo.ProductName
	out.CaseInfo.ProductVersion = r.CaseInfo.ProductVersion
	out.CaseInfo.Environment = r.CaseInfo.Environment
	out.CaseInfo.Tier = r.CaseInfo.Tier
	out.CaseInfo.Region = r.CaseInfo.Region
	return out
}

// ChatHistoryMessage is one message of chat-history context for a recommendation request.
type ChatHistoryMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

// RecommendationRequest is the portal's request shape for
// POST /conversations/recommendations/search.
type RecommendationRequest struct {
	ChatHistory      []ChatHistoryMessage `json:"chatHistory"`
	ConversationData struct {
		ChatHistory string              `json:"chatHistory"`
		EnvProducts map[string][]string `json:"envProducts"`
		Region      string              `json:"region"`
		Tier        string              `json:"tier"`
	} `json:"conversationData"`
}

// BuildRecommendationRequest converts the portal's request into the AI chat
// agent's request shape.
func BuildRecommendationRequest(req RecommendationRequest) aichatagent.RecommendationRequest {
	history := make([]aichatagent.ChatMessage, 0, len(req.ChatHistory))
	for _, m := range req.ChatHistory {
		history = append(history, aichatagent.ChatMessage{Role: m.Role, Content: m.Content, Timestamp: m.Timestamp})
	}
	return aichatagent.RecommendationRequest{
		ChatHistory: history,
		ConversationData: aichatagent.ConversationData{
			ChatHistory: req.ConversationData.ChatHistory,
			EnvProducts: req.ConversationData.EnvProducts,
			Region:      req.ConversationData.Region,
			Tier:        req.ConversationData.Tier,
		},
	}
}

// RecommendationItem is a single matching KB article.
type RecommendationItem struct {
	Title     string  `json:"title"`
	ArticleID string  `json:"articleId"`
	Score     float64 `json:"score"`
}

// RecommendationResponse is the portal's response for
// POST /conversations/recommendations/search.
type RecommendationResponse struct {
	Query           string               `json:"query"`
	Recommendations []RecommendationItem `json:"recommendations"`
}

// MapRecommendationResponse builds the portal response from the AI chat
// agent's RecommendationResponse.
func MapRecommendationResponse(r aichatagent.RecommendationResponse) RecommendationResponse {
	items := make([]RecommendationItem, 0, len(r.Recommendations))
	for _, i := range r.Recommendations {
		items = append(items, RecommendationItem{Title: i.Title, ArticleID: i.ArticleID, Score: i.Score})
	}
	return RecommendationResponse{Query: r.Query, Recommendations: items}
}

// ChatMessageRequest is the portal's request shape for
// POST /projects/{projectId}/conversations/{conversationId}/messages.
type ChatMessageRequest struct {
	Message     string              `json:"message"`
	EnvProducts map[string][]string `json:"envProducts,omitempty"`
	Region      string              `json:"region,omitempty"`
	Tier        string              `json:"tier,omitempty"`
}

// KbReference is a single KB article referenced in a chat response.
type KbReference struct {
	SysKbID string `json:"sysKbId"`
	Title   string `json:"title"`
}

// ChatResponse is the portal's response for
// POST /projects/{projectId}/conversations/{conversationId}/messages.
type ChatResponse struct {
	Message         string                      `json:"message"`
	ConversationID  string                      `json:"conversationId"`
	Intent          *aichatagent.DetectedIntent `json:"intent,omitempty"`
	SlotState       *aichatagent.SlotState      `json:"slotState,omitempty"`
	Actions         []aichatagent.Action        `json:"actions,omitempty"`
	Recommendations *RecommendationResponse     `json:"recommendations,omitempty"`
	Resolved        *bool                       `json:"resolved,omitempty"`
	KbReferences    []KbReference               `json:"kbReferences,omitempty"`
}

// MapChatResponse builds the portal response from the AI chat agent's ChatResponse.
func MapChatResponse(r aichatagent.ChatResponse) ChatResponse {
	out := ChatResponse{
		Message:        r.Message,
		ConversationID: r.ConversationID,
		Intent:         r.Intent,
		SlotState:      r.SlotState,
		Actions:        r.Actions,
		Resolved:       r.Resolved,
	}
	if len(r.KbReferences) > 0 {
		out.KbReferences = make([]KbReference, 0, len(r.KbReferences))
		for _, kb := range r.KbReferences {
			out.KbReferences = append(out.KbReferences, KbReference{SysKbID: kb.SysKbID, Title: kb.Title})
		}
	}
	if r.Recommendations != nil {
		mapped := MapRecommendationResponse(*r.Recommendations)
		out.Recommendations = &mapped
	}
	return out
}

// ConversationSummaryResponse is the portal's response for
// GET /projects/{id}/conversations/{conversationId}/summary.
type ConversationSummaryResponse struct {
	ConversationID          string `json:"conversationId"`
	MessagesExchanged       int    `json:"messagesExchanged"`
	TroubleshootingAttempts int    `json:"troubleshootingAttempts"`
	KbArticlesReviewed      int    `json:"kbArticlesReviewed"`
}

// MapConversationSummary builds the portal response from the AI chat agent's
// ConversationSummaryResponse.
func MapConversationSummary(r aichatagent.ConversationSummaryResponse) ConversationSummaryResponse {
	return ConversationSummaryResponse{
		ConversationID:          r.ConversationID,
		MessagesExchanged:       r.MessagesExchanged,
		TroubleshootingAttempts: r.TroubleshootingAttempts,
		KbArticlesReviewed:      r.KbArticlesReviewed,
	}
}

// ConversationSummary is one item of the portal's response for
// POST /projects/{id}/conversations/search — shaped to match the frontend's
// own Conversation type (apps/customer-portal/webapp/src/features/support/
// types/conversations.ts) field-for-field: State and Project as IDLabelRef
// (not a plain string / not missing) — see conversation_enum_mapping.go for
// the State translation.
type ConversationSummary struct {
	ID             string      `json:"id"`
	Number         string      `json:"number,omitempty"`
	InitialMessage string      `json:"initialMessage,omitempty"`
	MessageCount   int         `json:"messageCount"`
	Project        *IDLabelRef `json:"project,omitempty"`
	Case           *IDLabelRef `json:"case,omitempty"`
	State          *IDLabelRef `json:"state,omitempty"`
	CreatedOn      string      `json:"createdOn"`
	CreatedBy      string      `json:"createdBy"`
}

// SearchConversationsResponse is the portal's response for
// POST /projects/{id}/conversations/search. TotalRecords (not Total) to
// match the frontend's shared pagination envelope.
type SearchConversationsResponse struct {
	Conversations []ConversationSummary `json:"conversations"`
	TotalRecords  int                   `json:"totalRecords"`
	Limit         int                   `json:"limit"`
	Offset        int                   `json:"offset"`
}

// MapSearchConversations builds the portal response from entity-service's
// SearchConversationsResponse.
func MapSearchConversations(r entity.SearchConversationsResponse) SearchConversationsResponse {
	items := make([]ConversationSummary, 0, len(r.Conversations))
	for _, c := range r.Conversations {
		summary := ConversationSummary{
			MessageCount: c.MessageCount,
			Project:      entityRefToIDLabel(c.Project),
			Case:         entityRefToIDLabel(c.Case),
			State:        conversationStateRef(c.State),
			CreatedOn:    c.CreatedOn,
			CreatedBy:    userRefIdentity(c.CreatedBy),
		}
		if c.ID != nil {
			summary.ID = *c.ID
		}
		if c.Number != nil {
			summary.Number = *c.Number
		}
		if c.InitialMessage != nil {
			summary.InitialMessage = *c.InitialMessage
		}
		items = append(items, summary)
	}
	return SearchConversationsResponse{
		Conversations: items,
		TotalRecords:  r.Total,
		Limit:         r.Limit,
		Offset:        r.Offset,
	}
}

// ErrUnsupportedConversationState reports a stateKeys value this backend
// cannot translate. Surfaced as a 400 rather than silently widening the search.
var ErrUnsupportedConversationState = errors.New("unsupported conversation state filter")

// ConversationSearchFilters holds the optional filter criteria for
// POST /projects/{id}/conversations/search — shaped to match the frontend's
// actual request body. StateKeys carries ServiceNow's numeric choice-list
// ids (the frontend was built against the old Ballerina backend and still
// sends these, not entity-service's own string enum) — see
// conversation_enum_mapping.go for the translation. No ProjectIDs field:
// project scoping comes exclusively from the {id} path parameter.
type ConversationSearchFilters struct {
	StateKeys   []int  `json:"stateKeys,omitempty"`
	SearchQuery string `json:"searchQuery,omitempty"`
	CreatedByMe bool   `json:"createdByMe,omitempty"`
}

// ConversationSearchRequest is the portal's request body for
// POST /projects/{id}/conversations/search.
type ConversationSearchRequest struct {
	Filters    ConversationSearchFilters `json:"filters"`
	SortBy     entity.ConversationSort   `json:"sortBy"`
	Pagination entity.Pagination         `json:"pagination"`
}

// BuildEntitySearchConversationsRequest translates the portal's request
// into entity-service's SearchConversationsRequest. projectID (the {id}
// path parameter) always populates Filters.ProjectIDs — never the request
// body.
//
// Returns an error when a supplied stateKey has no mapping. It must not fall
// back to an unfiltered search: entity-service reads an empty States as "no
// state filter", so a dropped id turns "show me Open conversations" into "show
// me everything" — which is what it did for ServiceNow's "Open" state (id 1).
func BuildEntitySearchConversationsRequest(projectID string, req ConversationSearchRequest) (entity.SearchConversationsRequest, error) {
	states, unmapped, ok := conversationIDsToEnums(req.Filters.StateKeys)
	if !ok {
		return entity.SearchConversationsRequest{}, fmt.Errorf("%w: %d", ErrUnsupportedConversationState, unmapped)
	}
	return entity.SearchConversationsRequest{
		Filters: entity.SearchConversationsFilters{
			ProjectIDs:  []string{projectID},
			States:      states,
			SearchQuery: req.Filters.SearchQuery,
			CreatedByMe: req.Filters.CreatedByMe,
		},
		SortBy:     req.SortBy,
		Pagination: req.Pagination,
	}, nil
}
