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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/aichatagent"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
)

// TestUserIDTokenFromRequest covers the Sec-WebSocket-Protocol token smuggling
// the frontend relies on, because a browser cannot set a custom header on a
// WebSocket handshake. The "via Choreo" case is the one that actually runs in
// a deployed environment — the gateway strips the leading
// "choreo-oauth2-token, <accessToken>" pair before the request reaches here.
func TestUserIDTokenFromRequest(t *testing.T) {
	tests := map[string]struct {
		header    string
		protocols string
		want      string
	}{
		"header wins when present": {
			header:    "header-token",
			protocols: "cs-customer-portal, protocol-token",
			want:      "header-token",
		},
		"via Choreo — gateway already stripped the oauth2 pair": {
			protocols: "cs-customer-portal, user-id-token",
			want:      "user-id-token",
		},
		"direct connection — full offer from the browser": {
			protocols: "choreo-oauth2-token, access-token, cs-customer-portal, user-id-token",
			want:      "user-id-token",
		},
		"no padding around the separator": {
			protocols: "cs-customer-portal,user-id-token",
			want:      "user-id-token",
		},
		"subprotocol alone carries no token": {
			protocols: "cs-customer-portal",
			want:      "",
		},
		"nothing at all": {
			want: "",
		},
		"blank header falls through to the subprotocol": {
			header:    "   ",
			protocols: "cs-customer-portal, user-id-token",
			want:      "user-id-token",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/ws?sessionId=x", nil)
			if tc.header != "" {
				r.Header.Set(userIDTokenHeader, tc.header)
			}
			if tc.protocols != "" {
				r.Header.Set("Sec-WebSocket-Protocol", tc.protocols)
			}
			if got := userIDTokenFromRequest(r); got != tc.want {
				t.Errorf("userIDTokenFromRequest() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestUserIDTokenFromRequest_RepeatedHeaders proves a client that splits its
// offer across repeated headers is read the same as a single comma-separated
// one — the two forms are equivalent on the wire.
func TestUserIDTokenFromRequest_RepeatedHeaders(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/ws?sessionId=x", nil)
	r.Header.Add("Sec-WebSocket-Protocol", "cs-customer-portal")
	r.Header.Add("Sec-WebSocket-Protocol", "user-id-token")

	if got := userIDTokenFromRequest(r); got != "user-id-token" {
		t.Errorf("userIDTokenFromRequest() = %q, want %q", got, "user-id-token")
	}
}

// TestBuildUpstreamPayload guards the fields the AI agent reads straight off
// the message. accountId in particular is required by the agent — it uses it
// for the session, the per-account token budget, and analytics — and an
// earlier field whitelist dropped it, which made every chat message fail with
// "accountId is required".
func TestBuildUpstreamPayload(t *testing.T) {
	const serverConvID = "11111111-1111-1111-1111-111111111111"
	const serverAccountID = "22222222-2222-2222-2222-222222222222"

	t.Run("forwards type, message and envProducts untouched", func(t *testing.T) {
		parsed := map[string]any{
			"type":        "user_message",
			"message":     "hi",
			"envProducts": map[string]any{"dep-1": []any{"apim"}},
		}

		raw, err := buildUpstreamPayload(parsed, serverConvID, serverAccountID)
		if err != nil {
			t.Fatalf("buildUpstreamPayload returned error: %v", err)
		}

		var got map[string]any
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("result is not valid JSON: %v", err)
		}
		if got["type"] != "user_message" {
			t.Errorf("type = %v, want user_message", got["type"])
		}
		if got["message"] != "hi" {
			t.Errorf("message = %v, want hi", got["message"])
		}
		if _, ok := got["envProducts"]; !ok {
			t.Error("envProducts was dropped")
		}
		// accountId must be present even though the client sent none — the
		// agent rejects the message outright without it.
		if got["accountId"] != serverAccountID {
			t.Errorf("accountId = %v, want the server-resolved %v", got["accountId"], serverAccountID)
		}
	})

	// The security-relevant case: a caller naming someone else's account must
	// not have that account's token budget or analytics charged.
	t.Run("server accountId overrides a client-supplied one", func(t *testing.T) {
		raw, err := buildUpstreamPayload(map[string]any{
			"accountId": "someone-elses-account",
			"message":   "hi",
		}, serverConvID, serverAccountID)
		if err != nil {
			t.Fatalf("buildUpstreamPayload returned error: %v", err)
		}

		var got map[string]any
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("result is not valid JSON: %v", err)
		}
		if got["accountId"] != serverAccountID {
			t.Errorf("accountId = %v, want the server-resolved %v — a client value must never win",
				got["accountId"], serverAccountID)
		}
	})

	t.Run("server conversationId overrides the client's", func(t *testing.T) {
		raw, err := buildUpstreamPayload(map[string]any{
			"conversationId": "whatever-the-client-said",
			"message":        "hi",
		}, serverConvID, serverAccountID)
		if err != nil {
			t.Fatalf("buildUpstreamPayload returned error: %v", err)
		}

		var got map[string]any
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("result is not valid JSON: %v", err)
		}
		if got["conversationId"] != serverConvID {
			t.Errorf("conversationId = %v, want the server-resolved %v", got["conversationId"], serverConvID)
		}
	})

	t.Run("does not mutate the caller's map", func(t *testing.T) {
		parsed := map[string]any{"conversationId": "client-value", "message": "hi"}

		if _, err := buildUpstreamPayload(parsed, serverConvID, serverAccountID); err != nil {
			t.Fatalf("buildUpstreamPayload returned error: %v", err)
		}
		if parsed["conversationId"] != "client-value" {
			t.Errorf("input map was mutated: conversationId = %v", parsed["conversationId"])
		}
	})

	t.Run("nil input does not panic", func(t *testing.T) {
		raw, err := buildUpstreamPayload(nil, serverConvID, serverAccountID)
		if err != nil {
			t.Fatalf("buildUpstreamPayload returned error: %v", err)
		}

		var got map[string]any
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("result is not valid JSON: %v", err)
		}
		if got["conversationId"] != serverConvID {
			t.Errorf("conversationId = %v, want %v", got["conversationId"], serverConvID)
		}
	})
}

// stubValidator is a wsTokenValidator that accepts exactly one token.
type stubValidator struct {
	accept string
	called int
}

func (s *stubValidator) DecodeUnverified(token string) (*middleware.UserInfo, error) {
	s.called++
	if token != s.accept {
		return nil, errors.New("invalid token")
	}
	return &middleware.UserInfo{UserID: "user-1", Email: "u@example.com"}, nil
}

// TestHandleWebSocket_RejectsUnauthenticated asserts the upgrade never happens
// without a valid token — the handler must answer with a normal HTTP error
// rather than completing the handshake, since it runs on a listener that has
// no Auth middleware in front of it.
func TestHandleWebSocket_RejectsUnauthenticated(t *testing.T) {
	tests := map[string]struct {
		protocols  string
		wantStatus int
		wantCalls  int
	}{
		"no token at all": {
			wantStatus: http.StatusUnauthorized,
			wantCalls:  0,
		},
		"token present but not valid": {
			protocols:  "cs-customer-portal, wrong-token",
			wantStatus: http.StatusUnauthorized,
			wantCalls:  1,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			v := &stubValidator{accept: "good-token"}
			h := NewWebSocketHandler(nil, nil, v, nil)

			r := httptest.NewRequest(http.MethodGet, "/ws?sessionId=11111111-1111-1111-1111-111111111111", nil)
			if tc.protocols != "" {
				r.Header.Set("Sec-WebSocket-Protocol", tc.protocols)
			}
			w := httptest.NewRecorder()

			h.HandleWebSocket(w, r)

			if w.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tc.wantStatus)
			}
			if v.called != tc.wantCalls {
				t.Errorf("DecodeUnverified called %d times, want %d", v.called, tc.wantCalls)
			}
		})
	}
}

// stubEntity is an entityCommentCreator whose GetProject result is scripted.
type stubEntity struct {
	mu sync.Mutex

	project entity.ProjectDetailsView
	err     error
	calls   int

	// conversation/conversationErr script GetConversation, which handleMessage
	// consults before downgrading a CONVERTED conversation to RESOLVED.
	conversation    entity.ConversationDetails
	conversationErr error
	// updatedStates records every state UpdateConversation was asked to set, so
	// a test can assert the transition was skipped rather than merely reordered.
	updatedStates []string

	createdConversation     entity.CreateConversationResponse
	createdConversationErr  error
	createConversationCalls int
	lastCreateReq           entity.CreateConversationRequest

	commentsCreated []entity.CreateCommentRequest
}

func (s *stubEntity) GetProject(_ context.Context, _ string) (entity.ProjectDetailsView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	return s.project, s.err
}

func (s *stubEntity) CreateComment(_ context.Context, req entity.CreateCommentRequest) (entity.CreateCommentResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.commentsCreated = append(s.commentsCreated, req)
	return entity.CreateCommentResponse{}, nil
}

func (s *stubEntity) CreateConversation(_ context.Context, req entity.CreateConversationRequest) (entity.CreateConversationResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.createConversationCalls++
	s.lastCreateReq = req
	if s.createdConversationErr != nil {
		return entity.CreateConversationResponse{}, s.createdConversationErr
	}
	if s.createdConversation.Conversation.ID == "" {
		return entity.CreateConversationResponse{
			Conversation: entity.CreatedConversation{ID: "33333333-3333-3333-3333-333333333333"},
		}, nil
	}
	return s.createdConversation, nil
}

func (s *stubEntity) UpdateConversation(_ context.Context, _ string, req entity.UpdateConversationRequest) (entity.UpdateConversationResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updatedStates = append(s.updatedStates, req.State)
	return entity.UpdateConversationResponse{}, nil
}

func (s *stubEntity) GetConversation(_ context.Context, _ string) (entity.ConversationDetails, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conversation, s.conversationErr
}

func (s *stubEntity) getCreateConversationCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.createConversationCalls
}

func (s *stubEntity) getLastCreateReq() entity.CreateConversationRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastCreateReq
}

// stubStreamer is a mock wsStreamer for WebSocket tests.
type stubStreamer struct {
	mu sync.Mutex

	lastSessionID string
	lastPayload   string
	streamCalls   int
	streamErr     error
	result        map[string]json.RawMessage
}

func (s *stubStreamer) StreamChat(_ context.Context, sessionID, payload string, caller aichatagent.BrowserConn) (map[string]json.RawMessage, error) {
	s.mu.Lock()
	s.streamCalls++
	s.lastSessionID = sessionID
	s.lastPayload = payload
	err := s.streamErr
	res := s.result
	s.mu.Unlock()

	if err != nil {
		return nil, err
	}
	// Emit a final event so the client knows the turn is complete
	_ = caller.WriteMessage(websocket.TextMessage, []byte(`{"type":"final","payload":{"message":"response text"}}`))

	if res != nil {
		return res, nil
	}
	return map[string]json.RawMessage{
		"message": json.RawMessage(`"response text"`),
	}, nil
}

func (s *stubStreamer) SendSideChannel(_ context.Context, _ string, _ string, _ aichatagent.BrowserConn) error {
	return nil
}

func (s *stubStreamer) getCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.streamCalls
}

func (s *stubStreamer) getLastSessionID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastSessionID
}

// TestHandleWebSocket_ProjectAccessGatesTheUpgrade asserts the account lookup
// doubles as an authorization gate: a project the caller can't see must be
// rejected with an HTTP status *before* any upgrade happens, not closed after.
func TestHandleWebSocket_ProjectAccessGatesTheUpgrade(t *testing.T) {
	v := &stubValidator{accept: "good-token"}
	e := &stubEntity{err: apierror.NewUpstreamError(http.StatusForbidden, nil)}
	h := NewWebSocketHandler(nil, e, v, nil)

	r := httptest.NewRequest(http.MethodGet, "/ws?sessionId=11111111-1111-1111-1111-111111111111", nil)
	r.Header.Set("Sec-WebSocket-Protocol", "cs-customer-portal, good-token")
	w := httptest.NewRecorder()

	h.HandleWebSocket(w, r)

	if e.calls != 1 {
		t.Errorf("GetProject called %d times, want 1", e.calls)
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
	// A completed upgrade would have written 101 and hijacked the connection.
	if w.Code == http.StatusSwitchingProtocols {
		t.Error("connection was upgraded despite the project being inaccessible")
	}
}

// TestHandleWebSocket_ValidatesSessionIDAfterAuth pins the ordering: an
// authenticated caller with a malformed sessionId gets 400, not 401.
func TestHandleWebSocket_ValidatesSessionIDAfterAuth(t *testing.T) {
	v := &stubValidator{accept: "good-token"}
	h := NewWebSocketHandler(nil, nil, v, nil)

	r := httptest.NewRequest(http.MethodGet, "/ws?sessionId=not-a-uuid", nil)
	r.Header.Set("Sec-WebSocket-Protocol", "cs-customer-portal, good-token")
	w := httptest.NewRecorder()

	h.HandleWebSocket(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// TestHandleWebSocket_CreateConversation asserts that an incoming message with an
// empty conversationId dynamically creates a new conversation, returns a
// conversation_created event, and streams the AI agent's response.
func TestHandleWebSocket_CreateConversation(t *testing.T) {
	const projectID = "11111111-1111-1111-1111-111111111111"
	const createdConvID = "33333333-3333-3333-3333-333333333333"

	v := &stubValidator{accept: "good-token"}
	e := &stubEntity{
		project: entity.ProjectDetailsView{
			ID:      projectID,
			Account: entity.ProjectAccountRef{ID: "acc-1"},
		},
		createdConversation: entity.CreateConversationResponse{
			Conversation: entity.CreatedConversation{ID: createdConvID},
		},
	}
	ai := &stubStreamer{}
	h := NewWebSocketHandler(ai, e, v, nil)

	s := httptest.NewServer(http.HandlerFunc(h.HandleWebSocket))
	defer s.Close()

	wsURL := "ws" + strings.TrimPrefix(s.URL, "http") + "/ws?sessionId=" + projectID
	dialer := websocket.Dialer{
		Subprotocols: []string{"cs-customer-portal", "good-token"},
	}

	conn, resp, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	defer func() { _ = conn.Close() }()

	// 1. Send user_message with empty conversationId
	msg := map[string]any{
		"type":           "user_message",
		"conversationId": "",
		"message":        "Hello, I need help",
	}
	msgBytes, _ := json.Marshal(msg)
	if err := conn.WriteMessage(websocket.TextMessage, msgBytes); err != nil {
		t.Fatalf("write message failed: %v", err)
	}

	// 2. Expect conversation_created event
	_, respData, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read message failed: %v", err)
	}
	var evt wsEvent
	if err := json.Unmarshal(respData, &evt); err != nil {
		t.Fatalf("unmarshal event failed: %v", err)
	}
	if evt.Type != "conversation_created" {
		t.Errorf("type = %q, want conversation_created", evt.Type)
	}
	if evt.ConversationID != createdConvID {
		t.Errorf("conversationId = %q, want %q", evt.ConversationID, createdConvID)
	}

	// Read the stream response from turn 1 so turn 1 completes
	_, respData2, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read turn 1 final message failed: %v", err)
	}
	var evtFinal wsEvent
	if err := json.Unmarshal(respData2, &evtFinal); err != nil {
		t.Fatalf("unmarshal turn 1 final failed: %v", err)
	}
	if evtFinal.Type != "final" {
		t.Errorf("type = %q, want final", evtFinal.Type)
	}

	// Verify CreateConversation was called with projectID and initial message
	if e.getCreateConversationCalls() != 1 {
		t.Errorf("CreateConversation called %d times, want 1", e.getCreateConversationCalls())
	}
	lastReq := e.getLastCreateReq()
	if lastReq.ProjectID != projectID {
		t.Errorf("CreateConversation ProjectID = %q, want %q", lastReq.ProjectID, projectID)
	}
	if lastReq.InitialMessage != "Hello, I need help" {
		t.Errorf("CreateConversation InitialMessage = %q, want 'Hello, I need help'", lastReq.InitialMessage)
	}

	// Verify StreamChat was called with sessionID = projectID + ":" + createdConvID
	expectedSessionID := projectID + ":" + createdConvID
	if ai.getLastSessionID() != expectedSessionID {
		t.Errorf("StreamChat sessionID = %q, want %q", ai.getLastSessionID(), expectedSessionID)
	}

	// 3. Send follow-up turn omitting conversationId (connection reuses active conversation)
	msg2 := map[string]any{
		"type":           "user_message",
		"conversationId": "",
		"message":        "Second question",
	}
	msg2Bytes, _ := json.Marshal(msg2)
	if err := conn.WriteMessage(websocket.TextMessage, msg2Bytes); err != nil {
		t.Fatalf("write follow-up failed: %v", err)
	}

	// Read turn 2 stream response
	_, respData3, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read turn 2 final message failed: %v", err)
	}
	var evtFinal2 wsEvent
	if err := json.Unmarshal(respData3, &evtFinal2); err != nil {
		t.Fatalf("unmarshal turn 2 final failed: %v", err)
	}
	if evtFinal2.Type != "final" {
		t.Errorf("type = %q, want final", evtFinal2.Type)
	}

	// Verify CreateConversation was NOT called again (calls remain 1)
	if e.getCreateConversationCalls() != 1 {
		t.Errorf("CreateConversation called %d times on follow-up, want 1", e.getCreateConversationCalls())
	}
	if ai.getCalls() != 2 {
		t.Errorf("StreamChat called %d times, want 2", ai.getCalls())
	}
	if ai.getLastSessionID() != expectedSessionID {
		t.Errorf("StreamChat sessionID on follow-up = %q, want %q", ai.getLastSessionID(), expectedSessionID)
	}
}

// TestHandleWebSocket_EmptyMessageOnNewConversation asserts that sending an empty
// message when starting a conversation returns an error event.
func TestHandleWebSocket_EmptyMessageOnNewConversation(t *testing.T) {
	const projectID = "11111111-1111-1111-1111-111111111111"

	v := &stubValidator{accept: "good-token"}
	e := &stubEntity{
		project: entity.ProjectDetailsView{
			ID:      projectID,
			Account: entity.ProjectAccountRef{ID: "acc-1"},
		},
	}
	ai := &stubStreamer{}
	h := NewWebSocketHandler(ai, e, v, nil)

	s := httptest.NewServer(http.HandlerFunc(h.HandleWebSocket))
	defer s.Close()

	wsURL := "ws" + strings.TrimPrefix(s.URL, "http") + "/ws?sessionId=" + projectID
	dialer := websocket.Dialer{
		Subprotocols: []string{"cs-customer-portal", "good-token"},
	}

	conn, resp, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	defer func() { _ = conn.Close() }()

	msg := map[string]any{
		"type":           "user_message",
		"conversationId": "",
		"message":        "   ",
	}
	msgBytes, _ := json.Marshal(msg)
	if err := conn.WriteMessage(websocket.TextMessage, msgBytes); err != nil {
		t.Fatalf("write message failed: %v", err)
	}

	_, respData, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read message failed: %v", err)
	}
	var evt wsEvent
	if err := json.Unmarshal(respData, &evt); err != nil {
		t.Fatalf("unmarshal event failed: %v", err)
	}
	if evt.Type != "error" {
		t.Errorf("type = %q, want error", evt.Type)
	}
	if evt.Message != "Message is required to start a new conversation." {
		t.Errorf("message = %q, want 'Message is required to start a new conversation.'", evt.Message)
	}
	if got := e.getCreateConversationCalls(); got != 0 {
		t.Errorf("CreateConversation was called %d times, want 0", got)
	}
}

// TestHandleWebSocket_InvalidConversationID asserts that an invalid UUID conversationId
// returns an error event.
func TestHandleWebSocket_InvalidConversationID(t *testing.T) {
	const projectID = "11111111-1111-1111-1111-111111111111"

	v := &stubValidator{accept: "good-token"}
	e := &stubEntity{
		project: entity.ProjectDetailsView{
			ID:      projectID,
			Account: entity.ProjectAccountRef{ID: "acc-1"},
		},
	}
	ai := &stubStreamer{}
	h := NewWebSocketHandler(ai, e, v, nil)

	s := httptest.NewServer(http.HandlerFunc(h.HandleWebSocket))
	defer s.Close()

	wsURL := "ws" + strings.TrimPrefix(s.URL, "http") + "/ws?sessionId=" + projectID
	dialer := websocket.Dialer{
		Subprotocols: []string{"cs-customer-portal", "good-token"},
	}

	conn, resp, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	defer func() { _ = conn.Close() }()

	msg := map[string]any{
		"type":           "user_message",
		"conversationId": "not-a-valid-uuid",
		"message":        "Hello",
	}
	msgBytes, _ := json.Marshal(msg)
	if err := conn.WriteMessage(websocket.TextMessage, msgBytes); err != nil {
		t.Fatalf("write message failed: %v", err)
	}

	_, respData, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read message failed: %v", err)
	}
	var evt wsEvent
	if err := json.Unmarshal(respData, &evt); err != nil {
		t.Fatalf("unmarshal event failed: %v", err)
	}
	if evt.Type != "error" {
		t.Errorf("type = %q, want error", evt.Type)
	}
	if evt.Message != "Invalid conversation ID." {
		t.Errorf("message = %q, want 'Invalid conversation ID.'", evt.Message)
	}
}

// TestHandleWebSocket_CreateConversationError asserts that a failure in entity
// CreateConversation emits an error event to the client.
func TestHandleWebSocket_CreateConversationError(t *testing.T) {
	const projectID = "11111111-1111-1111-1111-111111111111"

	v := &stubValidator{accept: "good-token"}
	e := &stubEntity{
		project: entity.ProjectDetailsView{
			ID:      projectID,
			Account: entity.ProjectAccountRef{ID: "acc-1"},
		},
		createdConversationErr: errors.New("upstream entity service down"),
	}
	ai := &stubStreamer{}
	h := NewWebSocketHandler(ai, e, v, nil)

	s := httptest.NewServer(http.HandlerFunc(h.HandleWebSocket))
	defer s.Close()

	wsURL := "ws" + strings.TrimPrefix(s.URL, "http") + "/ws?sessionId=" + projectID
	dialer := websocket.Dialer{
		Subprotocols: []string{"cs-customer-portal", "good-token"},
	}

	conn, resp, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	defer func() { _ = conn.Close() }()

	msg := map[string]any{
		"type":           "user_message",
		"conversationId": "",
		"message":        "Hello, I need help",
	}
	msgBytes, _ := json.Marshal(msg)
	if err := conn.WriteMessage(websocket.TextMessage, msgBytes); err != nil {
		t.Fatalf("write message failed: %v", err)
	}

	_, respData, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read message failed: %v", err)
	}
	var evt wsEvent
	if err := json.Unmarshal(respData, &evt); err != nil {
		t.Fatalf("unmarshal event failed: %v", err)
	}
	if evt.Type != "error" {
		t.Errorf("type = %q, want error", evt.Type)
	}
	if evt.Message != "Failed to create a new conversation." {
		t.Errorf("message = %q, want 'Failed to create a new conversation.'", evt.Message)
	}
}
