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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/aichatagent"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// replyAI is an agent client whose chat reply is scripted.
type replyAI struct {
	chatSpy
	reply aichatagent.ChatResponse
}

func (a *replyAI) CreateChat(_ context.Context, _ aichatagent.ChatPayload) (aichatagent.ChatResponse, error) {
	return a.reply, nil
}

// commentLog is a convEntity that records every comment and state change, and
// refuses a comment with no content with the 400 entity-service returns for it
// ("content is required").
type commentLog struct {
	*convEntity
	comments []entity.CreateCommentRequest
	resolved int
}

func (c *commentLog) CreateComment(_ context.Context, req entity.CreateCommentRequest) (entity.CreateCommentResponse, error) {
	if req.Content == "" {
		return entity.CreateCommentResponse{}, apierror.NewUpstreamError(http.StatusBadRequest, []byte(`{"message":"content is required"}`))
	}
	c.comments = append(c.comments, req)
	return entity.CreateCommentResponse{}, nil
}

func (c *commentLog) UpdateConversation(_ context.Context, _ string, req entity.UpdateConversationRequest) (entity.UpdateConversationResponse, error) {
	if req.State == createEntityStateResolved {
		c.resolved++
	}
	return entity.UpdateConversationResponse{}, nil
}

func agentComments(c *commentLog) []entity.CreateCommentRequest {
	var out []entity.CreateCommentRequest
	for _, cm := range c.comments {
		if cm.CreatedBy == entity.CreatedByAgent {
			out = append(out, cm)
		}
	}
	return out
}

func replyingWith(message string, resolved bool) (*AIChatHandler, *commentLog) {
	ent := &commentLog{convEntity: &convEntity{project: entity.ProjectDetailsView{ID: testProjectID}}}
	ai := &replyAI{reply: aichatagent.ChatResponse{Message: message, Resolved: &resolved}}
	return NewAIChatHandler(ai, ent), ent
}

func firstMessage(h *AIChatHandler) *httptest.ResponseRecorder {
	req := authed(httptest.NewRequest(http.MethodPost,
		"/projects/"+testProjectID+"/conversations", strings.NewReader(`{"message":"hi"}`)))
	req.SetPathValue("id", testProjectID)
	rec := httptest.NewRecorder()
	h.CreateConversation(rec, req)
	return rec
}

func followUp(h *AIChatHandler) *httptest.ResponseRecorder {
	req := authed(httptest.NewRequest(http.MethodPost,
		"/projects/"+testProjectID+"/conversations/"+testConversationID+"/messages",
		strings.NewReader(`{"message":"hi"}`)))
	req.SetPathValue("projectId", testProjectID)
	req.SetPathValue("conversationId", testConversationID)
	rec := httptest.NewRecorder()
	h.SendConversationMessage(rec, req)
	return rec
}

// An answer that was only <thinking> reasoning is empty once aichatagent strips
// it, and entity-service rejects a comment with no content. The turn must still
// succeed — nothing to store, but the conversation state is still handled —
// rather than turning a stored comment into a 400 for the customer.
func TestRESTChatAnEmptyAgentReplyIsNotAFailure(t *testing.T) {
	routes := map[string]func(*AIChatHandler) *httptest.ResponseRecorder{
		"CreateConversation":      firstMessage,
		"SendConversationMessage": followUp,
	}
	for name, call := range routes {
		t.Run(name+": stores no agent comment and still auto-resolves", func(t *testing.T) {
			h, ent := replyingWith("", true)
			rec := call(h)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body)
			}
			if got := agentComments(ent); len(got) != 0 {
				t.Errorf("stored %d agent comments for an empty reply, want none", len(got))
			}
			if ent.resolved != 1 {
				t.Errorf("auto-resolve ran %d times, want 1", ent.resolved)
			}
			var body map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("response is not JSON: %v", err)
			}
			if body["message"] != "" {
				t.Errorf("message = %v, want empty", body["message"])
			}
		})

		t.Run(name+": an ordinary reply is still stored once, as the agent", func(t *testing.T) {
			h, ent := replyingWith("Which gateway?", false)
			rec := call(h)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body)
			}
			got := agentComments(ent)
			if len(got) != 1 || got[0].Content != "Which gateway?" {
				t.Errorf("agent comments = %+v, want exactly the reply", got)
			}
			if ent.resolved != 0 {
				t.Errorf("auto-resolve ran %d times for an unresolved reply", ent.resolved)
			}
		})
	}
}
