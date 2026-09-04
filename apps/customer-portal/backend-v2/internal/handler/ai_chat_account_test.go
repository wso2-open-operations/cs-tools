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

package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/aichatagent"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
)

// testProjectID is defined in project_scoped_search_test.go.
const (
	testAccountID      = "22222222-2222-2222-2222-222222222222"
	testConversationID = "33333333-3333-3333-3333-333333333333"
)

// chatSpy records the ChatPayload the handler sends to the agent, so a test can
// assert which accountId the per-account token budget would be billed to.
type chatSpy struct{ got aichatagent.ChatPayload }

func (c *chatSpy) CreateChat(_ context.Context, req aichatagent.ChatPayload) (aichatagent.ChatResponse, error) {
	c.got = req
	return aichatagent.ChatResponse{Message: "ok"}, nil
}

func (c *chatSpy) CreateCaseClassification(_ context.Context, _ aichatagent.CaseClassificationPayload) (aichatagent.CaseClassificationResponse, error) {
	return aichatagent.CaseClassificationResponse{}, nil
}

func (c *chatSpy) GetRecommendations(_ context.Context, _ aichatagent.RecommendationRequest) (aichatagent.RecommendationResponse, error) {
	return aichatagent.RecommendationResponse{}, nil
}

func (c *chatSpy) GetConversationSummary(_ context.Context, _, _ string) (aichatagent.ConversationSummaryResponse, error) {
	return aichatagent.ConversationSummaryResponse{}, nil
}

// convEntity is an entityConversationClient with a scripted GetProject, and a
// counter for how many conversations it was asked to create.
type convEntity struct {
	project     entity.ProjectDetailsView
	projectErr  error
	createCalls int
}

func (e *convEntity) GetProject(_ context.Context, _ string) (entity.ProjectDetailsView, error) {
	return e.project, e.projectErr
}

func (e *convEntity) CreateConversation(_ context.Context, _ entity.CreateConversationRequest) (entity.CreateConversationResponse, error) {
	e.createCalls++
	return entity.CreateConversationResponse{
		Conversation: entity.CreatedConversation{ID: testConversationID},
	}, nil
}

func (e *convEntity) SearchConversations(_ context.Context, _ entity.SearchConversationsRequest) (entity.SearchConversationsResponse, error) {
	return entity.SearchConversationsResponse{}, nil
}

func (e *convEntity) SearchComments(_ context.Context, _ entity.SearchCommentsRequest) (entity.SearchCommentsResponse, error) {
	return entity.SearchCommentsResponse{}, nil
}

func (e *convEntity) CreateComment(_ context.Context, _ entity.CreateCommentRequest) (entity.CreateCommentResponse, error) {
	return entity.CreateCommentResponse{}, nil
}

func (e *convEntity) GetConversation(_ context.Context, _ string) (entity.ConversationDetails, error) {
	return entity.ConversationDetails{Project: &entity.EntityRef{ID: testProjectID}}, nil
}

func (e *convEntity) UpdateConversation(_ context.Context, _ string, _ entity.UpdateConversationRequest) (entity.UpdateConversationResponse, error) {
	return entity.UpdateConversationResponse{}, nil
}

func authed(req *http.Request) *http.Request {
	return req.WithContext(middleware.WithUserInfo(req.Context(), &middleware.UserInfo{
		UserID: "user-1", Email: "someone@example.com",
	}))
}

// The REST routes must bill the same account the WebSocket path resolves. A
// project ID sent as the accountId gives one budget key per project (N projects
// → N+1 budgets for one account) and splits the analytics.
func TestRESTChatBillsTheResolvedAccount(t *testing.T) {
	withAccount := entity.ProjectDetailsView{ID: testProjectID}
	withAccount.Account.ID = testAccountID

	t.Run("CreateConversation sends the project's account", func(t *testing.T) {
		ai, ent := &chatSpy{}, &convEntity{project: withAccount}
		h := NewAIChatHandler(ai, ent)

		req := authed(httptest.NewRequest(http.MethodPost,
			"/projects/"+testProjectID+"/conversations", strings.NewReader(`{"message":"hi"}`)))
		req.SetPathValue("id", testProjectID)
		h.CreateConversation(httptest.NewRecorder(), req)

		if ai.got.AccountID != testAccountID {
			t.Errorf("accountId = %q, want the resolved account %q", ai.got.AccountID, testAccountID)
		}
	})

	t.Run("SendConversationMessage sends the project's account", func(t *testing.T) {
		ai, ent := &chatSpy{}, &convEntity{project: withAccount}
		h := NewAIChatHandler(ai, ent)

		req := authed(httptest.NewRequest(http.MethodPost,
			"/projects/"+testProjectID+"/conversations/"+testConversationID+"/messages",
			strings.NewReader(`{"message":"hi"}`)))
		req.SetPathValue("projectId", testProjectID)
		req.SetPathValue("conversationId", testConversationID)
		h.SendConversationMessage(httptest.NewRecorder(), req)

		if ai.got.AccountID != testAccountID {
			t.Errorf("accountId = %q, want the resolved account %q", ai.got.AccountID, testAccountID)
		}
	})

	// Mirrors HandleWebSocket's own fallback, and the frontend's
	// `projectDetails?.account?.id || projectId` — the agent rejects an empty
	// accountId outright, so an account-less project must not send one.
	t.Run("falls back to the project ID when there is no account", func(t *testing.T) {
		ai, ent := &chatSpy{}, &convEntity{project: entity.ProjectDetailsView{ID: testProjectID}}
		h := NewAIChatHandler(ai, ent)

		req := authed(httptest.NewRequest(http.MethodPost,
			"/projects/"+testProjectID+"/conversations", strings.NewReader(`{"message":"hi"}`)))
		req.SetPathValue("id", testProjectID)
		h.CreateConversation(httptest.NewRecorder(), req)

		if ai.got.AccountID != testProjectID {
			t.Errorf("accountId = %q, want the project ID fallback %q", ai.got.AccountID, testProjectID)
		}
	})

	// Resolution runs before the conversation is created, so a failure cannot
	// leave an orphaned conversation behind.
	t.Run("a resolution failure creates no conversation", func(t *testing.T) {
		ai, ent := &chatSpy{}, &convEntity{projectErr: errors.New("upstream down")}
		h := NewAIChatHandler(ai, ent)

		req := authed(httptest.NewRequest(http.MethodPost,
			"/projects/"+testProjectID+"/conversations", strings.NewReader(`{"message":"hi"}`)))
		req.SetPathValue("id", testProjectID)
		rec := httptest.NewRecorder()
		h.CreateConversation(rec, req)

		if ent.createCalls != 0 {
			t.Errorf("CreateConversation called %d times, want 0", ent.createCalls)
		}
		if ai.got.AccountID != "" {
			t.Errorf("agent was called with %q, want no call at all", ai.got.AccountID)
		}
		if rec.Code == http.StatusOK {
			t.Errorf("status = %d, want a non-OK status", rec.Code)
		}
	})
}
