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
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/aichatagent"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
)

// wsStreamer abstracts the AI chat agent's WebSocket proxy operation used by
// WebSocketHandler.
type wsStreamer interface {
	StreamChat(ctx context.Context, sessionID, payload string, caller aichatagent.BrowserConn) (map[string]json.RawMessage, error)
	SendSideChannel(ctx context.Context, sessionID, payload string, caller aichatagent.BrowserConn) error
}

// wsMaxMessageBytes bounds the size of a single WebSocket frame this handler
// will read, on both the browser connection (here) and the upstream AI agent
// connection (internal/aichatagent/ws.go) — protects against a peer forcing
// a large allocation via an oversized frame.
const wsMaxMessageBytes = 64 << 10 // 64 KiB

// wsIdleTimeout bounds how long this handler waits for the next frame from
// an idle peer before closing the connection.
const wsIdleTimeout = 5 * time.Minute

// entityCommentCreator is the subset of the entity client needed to create
// conversations, persist conversation messages as comments, and auto-resolve a
// conversation.
// GetProject resolves the project's owning account (and doubles as the
// caller's access-control gate for that project) — see HandleWebSocket.
type entityCommentCreator interface {
	CreateComment(ctx context.Context, req entity.CreateCommentRequest) (entity.CreateCommentResponse, error)
	CreateConversation(ctx context.Context, req entity.CreateConversationRequest) (entity.CreateConversationResponse, error)
	UpdateConversation(ctx context.Context, id string, req entity.UpdateConversationRequest) (entity.UpdateConversationResponse, error)
	GetConversation(ctx context.Context, id string) (entity.ConversationDetails, error)
	GetProject(ctx context.Context, id string) (entity.ProjectDetailsView, error)
}

// wsAutoResolveState is entity-service's ConversationState value used to
// auto-resolve a conversation when the AI agent reports the issue solved.
const wsAutoResolveState = "RESOLVED"

// msgTypeFeedback/msgTypeTokenIncreaseRequest are the inbound side-channel
// message types — a thumbs up/down on an answer, and a request asking support to
// raise a token limit. Mirrors MSG_TYPE_FEEDBACK/MSG_TYPE_TOKEN_INCREASE_REQUEST
// in apps/customer-portal/backend's modules/ai_chat_agent/constants.bal.
const (
	msgTypeFeedback             = "feedback"
	msgTypeTokenIncreaseRequest = "token_increase_request"
)

// requestedByField names the actor on a token-increase request. It is always
// rewritten server-side from the authenticated session — never trusted from the
// browser — because it lands in a durable audit trail. See handleSideChannel.
const requestedByField = "requestedBy"

// wsSubprotocol is the WebSocket subprotocol this endpoint negotiates. The
// frontend offers it (see WS_CUSTOMER_PORTAL in the webapp's apiConstants.ts)
// and the upgrade echoes it back, matching the Ballerina backend's
// `@websocket:ServiceConfig { subProtocols: ["cs-customer-portal"] }`.
const wsSubprotocol = "cs-customer-portal"

// userIDTokenHeader carries the caller's user-ID token on ordinary HTTP
// requests. A browser cannot set it on a WebSocket handshake — see
// userIDTokenFromRequest.
const userIDTokenHeader = "x-user-id-token" // #nosec G101 -- HTTP header name, not a credential

// wsTokenValidator abstracts middleware.TokenValidator so tests can inject a
// fake identity without minting real JWTs.
//
// DecodeUnverified, not Validate: the token arriving here is the browser's
// Asgardeo ID token, not the Choreo-injected x-jwt-assertion the validator is
// configured for, so full validation always rejects it. See that method's doc
// comment for the trust model that makes this safe.
type wsTokenValidator interface {
	DecodeUnverified(token string) (*middleware.UserInfo, error)
}

// WebSocketHandler proxies real-time chat messages between the browser and
// the upstream AI chat agent. When an incoming message does not carry an
// existing conversationId, a new conversation is dynamically created via
// entity-service and its ID is returned to the client in a conversation_created
// event before streaming begins.
// The AI agent's own reply IS persisted as a comment here (see
// handleMessage), attributed to the assistant rather than to the customer
// whose token relayed it, via entity.CreatedByAgent — the same as
// AIChatHandler.SendConversationMessage.
type WebSocketHandler struct {
	ai      wsStreamer
	entity  entityCommentCreator
	auth    wsTokenValidator
	upgrade websocket.Upgrader
}

// NewWebSocketHandler creates a WebSocketHandler backed by the given AI chat
// agent WebSocket client and entity client. auth validates the caller's token
// (see userIDTokenFromRequest for why this route authenticates itself instead
// of going through middleware.Auth). allowedOrigins restricts which browser
// Origins may open this connection (defense in depth against cross-site
// WebSocket hijacking) — pass nil/empty to allow any origin, e.g. for local
// development.
func NewWebSocketHandler(ai wsStreamer, entityClient entityCommentCreator, auth wsTokenValidator, allowedOrigins []string) *WebSocketHandler {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = true
	}
	return &WebSocketHandler{
		ai:     ai,
		entity: entityClient,
		auth:   auth,
		upgrade: websocket.Upgrader{
			// Negotiating the subprotocol the frontend offers is required, not
			// cosmetic: the handshake carries the caller's token as a
			// subprotocol value (see userIDTokenFromRequest), and a browser
			// aborts the connection if the server selects a protocol it never
			// offered. Selecting this one echoes back a value the client sent.
			Subprotocols: []string{wsSubprotocol},
			// Authorization is the token check in HandleWebSocket; this Origin
			// check is defense in depth against cross-site WebSocket
			// hijacking. A non-browser caller (e.g. a server-to-server client)
			// sends no Origin header at all and is allowed through either way.
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				return origin == "" || len(allowed) == 0 || allowed[origin]
			},
		},
	}
}

// userIDTokenFromRequest recovers the caller's user-ID token from a WebSocket
// upgrade request.
//
// A browser cannot set custom headers on a WebSocket handshake, so the
// frontend smuggles its tokens through Sec-WebSocket-Protocol as
//
//	"choreo-oauth2-token, <accessToken>, cs-customer-portal, <userIdToken>"
//
// (see the webapp's useChatWebSocket.ts). Choreo's gateway consumes the
// leading "choreo-oauth2-token, <accessToken>" pair for its own authorization
// and forwards only the remainder, so the token this backend needs is always
// the last comma-separated value — which holds whether or not the gateway is
// in the path, so a direct local connection works the same way.
//
// The x-user-id-token header is tried first, for non-browser callers and for
// any deployment where the gateway injects it. This mirrors the Ballerina
// backend's ws upgrade resource in apps/customer-portal/backend/service.bal.
func userIDTokenFromRequest(r *http.Request) string {
	if h := strings.TrimSpace(r.Header.Get(userIDTokenHeader)); h != "" {
		return h
	}
	// Values (not Get) because a client may split the offer across repeated
	// headers; both forms are equivalent on the wire.
	raw := strings.Join(r.Header.Values("Sec-WebSocket-Protocol"), ",")
	parts := strings.Split(raw, ",")
	last := strings.TrimSpace(parts[len(parts)-1])
	// A handshake offering only the subprotocol name carries no token.
	if last == wsSubprotocol {
		return ""
	}
	return last
}

// buildUpstreamPayload prepares a client chat message for the AI agent: the
// payload is forwarded **as the client sent it**, with conversationId replaced
// by the server-resolved value.
//
// Forwarding verbatim is required, not lazy. The agent reads several fields
// straight off this object and rejects the message outright if they're missing
// — most importantly accountId ("accountId is required"), which it uses to
// create the session, enforce the per-account token budget, and attribute
// analytics. An earlier version of this function forwarded only
// message/conversationId/envProducts, which made every chat message fail.
//
// This mirrors the Ballerina backend's ws onMessage
// (apps/customer-portal/backend/service.bal), which likewise sets
// parsed["conversationId"] and forwards the rest unchanged.
//
// Two fields are NOT taken from the client, and both are overwritten here even
// when the client supplied its own value:
//
//   - conversationID — already validated as a UUID by the caller, and it keys
//     the agent session this message belongs to.
//   - accountID — resolved server-side from the project the connection is
//     scoped to (see HandleWebSocket). The agent charges its per-account token
//     budget and attributes analytics to this value, so trusting the client
//     would let a caller bill another account. The Ballerina backend does
//     forward the client's accountId, but that is a weakness to not reproduce,
//     not a contract to match.
//
// Everything else — message, type, envProducts, and any field the frontend adds
// later — passes through untouched. Do not reintroduce a field whitelist here:
// that is what dropped accountId and broke every message before.
func buildUpstreamPayload(parsed map[string]any, conversationID, accountID string) ([]byte, error) {
	upstream := make(map[string]any, len(parsed)+2)
	for k, v := range parsed {
		upstream[k] = v
	}
	upstream["conversationId"] = conversationID
	upstream["accountId"] = accountID
	return json.Marshal(upstream)
}

// wsEvent is the JSON envelope used for events this handler sends directly
// to the browser (ping/pong, errors) — matches the upstream AI chat agent's
// own event shape so the frontend handles both uniformly.
type wsEvent struct {
	Type           string `json:"type"`
	Message        string `json:"message,omitempty"`
	ConversationID string `json:"conversationId,omitempty"`
	TS             string `json:"ts,omitempty"`
}

// HandleWebSocket handles GET /ws?sessionId={projectId}. The query parameter
// is named sessionId for wire compatibility, but it actually carries the
// project ID — the AI agent's own per-conversation session key is derived
// below as "{projectId}:{conversationId}".
func (h *WebSocketHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// This route runs on its own listener, without middleware.Auth — the
	// x-jwt-assertion header that middleware depends on cannot be set by a
	// browser opening a WebSocket, so the token arrives as a subprotocol value
	// instead. See userIDTokenFromRequest.
	token := userIDTokenFromRequest(r)
	if token == "" {
		slog.WarnContext(r.Context(), "websocket auth: no token on upgrade request")
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	// Decode, not validate: this is the browser's Asgardeo ID token, signed by a
	// different issuer for a different audience than the x-jwt-assertion the
	// validator is configured for. Choreo's gateway has already authenticated
	// the caller's access token by this point — see
	// middleware.TokenValidator.DecodeUnverified.
	user, err := h.auth.DecodeUnverified(token)
	if err != nil {
		// The error is deliberately not logged: some jwt/v5 error paths embed
		// parts of the offending token.
		slog.WarnContext(r.Context(), "websocket auth: could not decode user ID token")
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	projectID := r.URL.Query().Get("sessionId")
	if projectID == "" || !uuidRe.MatchString(projectID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	// Rebuild the context the Auth middleware would normally have populated,
	// so downstream entity-service calls authenticate as this caller.
	ctx := middleware.WithUserInfo(r.Context(), user)
	ctx = entity.WithUserIDToken(ctx, token)
	r = r.WithContext(ctx)

	// Resolve the account this connection may act on, server-side, before the
	// upgrade. Two jobs in one call:
	//
	//  1. Access control — entity-service rejects a project this caller can't
	//     see, so an unauthorized sessionId never reaches an upgraded socket.
	//     Same gate ProductConsumptionHandler uses for its own project scoping.
	//  2. Supplies accountId for every message on this connection, so the agent
	//     bills the per-account token budget to an account the caller actually
	//     belongs to rather than to whichever one it claimed.
	//
	// Resolved once here, not per message: projectID is fixed for the lifetime
	// of the connection. Failing before the upgrade also means the caller gets a
	// real HTTP status instead of an immediate WebSocket close.
	project, err := h.entity.GetProject(ctx, projectID)
	if err != nil {
		slog.ErrorContext(ctx, "websocket: failed to resolve project account",
			"userID", user.UserID, "projectID", projectID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to open the chat connection.")
		return
	}
	// Fall back to the project ID when entity-service reports no account, which
	// is what the frontend does too (`projectDetails?.account?.id || projectId`).
	// The agent rejects an empty accountId outright.
	accountID := project.Account.ID
	if accountID == "" {
		accountID = projectID
	}

	conn, err := h.upgrade.Upgrade(w, r, nil)
	if err != nil {
		slog.ErrorContext(r.Context(), "websocket upgrade failed", "userID", user.UserID, "err", summarizeErr(err))
		return
	}
	defer conn.Close()

	// The server's ReadTimeout/WriteTimeout (see cmd/server/main.go) can leave
	// deadlines on the connection Hijack handed off for this upgrade; clear
	// them so they don't kill an otherwise-idle-but-healthy chat session, and
	// rely on wsIdleTimeout below instead.
	underlying := conn.UnderlyingConn()
	_ = underlying.SetReadDeadline(time.Time{})
	_ = underlying.SetWriteDeadline(time.Time{})

	conn.SetReadLimit(wsMaxMessageBytes)

	var activeConvID string
	for {
		if err := conn.SetReadDeadline(time.Now().Add(wsIdleTimeout)); err != nil {
			slog.WarnContext(r.Context(), "websocket set read deadline failed", "userID", user.UserID, "err", summarizeErr(err))
			return
		}
		_, data, err := conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				slog.WarnContext(r.Context(), "websocket read error", "userID", user.UserID, "err", summarizeErr(err))
			}
			return
		}
		h.handleMessage(r.Context(), conn, user, projectID, accountID, &activeConvID, data)
	}
}

func (h *WebSocketHandler) handleMessage(ctx context.Context, conn *websocket.Conn, user *middleware.UserInfo, projectID, accountID string, activeConvID *string, data []byte) {
	trimmed := strings.TrimSpace(strings.ToLower(string(data)))
	var parsed map[string]any
	_ = json.Unmarshal(data, &parsed)

	isPing := trimmed == "ping"
	if !isPing {
		if t, _ := parsed["type"].(string); t == "ping" {
			isPing = true
		}
	}
	if isPing {
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		_ = writeWSJSON(conn, wsEvent{Type: "pong", TS: ts})
		return
	}

	// Side-channel messages — an answer rating, or a request to raise a token
	// limit. Dispatched before everything below, because none of it applies to
	// them and all of it causes harm:
	//
	//   - StreamChat reads until a "final" event, which the agent never sends
	//     for these (it answers with a "*_ack"), so the turn stalled until the
	//     read deadline. The browser-facing read loop is strictly sequential, so
	//     that stalled the whole connection: the customer's next message was not
	//     even read off the socket. The chat looked dead while it was fine.
	//   - persisting the "user query" makes no sense for a rating, which carries
	//     no message at all.
	//   - the resolved/auto-resolve bookkeeping likewise does not apply.
	//
	// Called synchronously, on purpose. It blocks the read loop for as long as
	// the acknowledgement takes, which is bounded by sideChannelTimeout and in
	// practice well under a second. Forwarding it in a goroutine instead would
	// mean two writers on one *websocket.Conn — gorilla permits only one at a
	// time for WriteMessage — so it would need a write mutex *and* coordination
	// with HandleWebSocket's deferred conn.Close(), or the loop could close the
	// connection underneath an in-flight write. That trades a bounded stall for
	// a write-after-close race, and it would diverge from the Ballerina backend,
	// whose onMessage also forwards these inline.
	if msgType, _ := parsed["type"].(string); msgType == msgTypeFeedback || msgType == msgTypeTokenIncreaseRequest {
		h.handleSideChannel(ctx, conn, user, projectID, activeConvID, msgType, parsed)
		return
	}

	conversationID, _ := parsed["conversationId"].(string)
	userMessage, _ := parsed["message"].(string)

	if conversationID != "" {
		if !uuidRe.MatchString(conversationID) {
			_ = writeWSJSON(conn, wsEvent{
				Type:    "error",
				Message: "Invalid conversation ID.",
			})
			return
		}
		if activeConvID != nil {
			*activeConvID = conversationID
		}
	} else if activeConvID != nil && *activeConvID != "" {
		conversationID = *activeConvID
	} else {
		// New conversation: create conversation dynamically in entity-service
		if strings.TrimSpace(userMessage) == "" {
			_ = writeWSJSON(conn, wsEvent{
				Type:    "error",
				Message: "Message is required to start a new conversation.",
			})
			return
		}
		convResp, err := h.entity.CreateConversation(ctx, entity.CreateConversationRequest{
			ProjectID:      projectID,
			InitialMessage: userMessage,
		})
		if err != nil {
			slog.ErrorContext(ctx, "entity CreateConversation failed", "userID", user.UserID, "projectID", projectID, "err", summarizeErr(err))
			_ = writeWSJSON(conn, wsEvent{
				Type:    "error",
				Message: "Failed to create a new conversation.",
			})
			return
		}
		conversationID = convResp.Conversation.ID
		if activeConvID != nil {
			*activeConvID = conversationID
		}
		if err := writeWSJSON(conn, wsEvent{
			Type:           "conversation_created",
			ConversationID: conversationID,
		}); err != nil {
			slog.WarnContext(ctx, "failed to send conversation_created event", "userID", user.UserID, "conversationID", conversationID, "err", summarizeErr(err))
			return
		}
	}
	enriched, err := buildUpstreamPayload(parsed, conversationID, accountID)
	if err != nil {
		_ = writeWSJSON(conn, wsEvent{Type: "error", Message: "Failed to process message."})
		return
	}

	agentSessionID := projectID + ":" + conversationID
	result, err := h.ai.StreamChat(ctx, agentSessionID, string(enriched), conn)
	if err != nil {
		slog.ErrorContext(ctx, "aichatagent StreamChat failed", "userID", user.UserID, "conversationID", conversationID, "err", summarizeErr(err))
		_ = writeWSJSON(conn, wsEvent{Type: "error", Message: "Failed to process message."})
		return
	}

	if userMessage != "" {
		_, err := h.entity.CreateComment(ctx, entity.CreateCommentRequest{
			ReferenceID:   conversationID,
			ReferenceType: entity.ReferenceTypeConversation,
			Type:          entity.CommentTypeComment,
			Content:       userMessage,
		})
		if err != nil {
			slog.ErrorContext(ctx, "entity CreateComment failed for conversation message", "userID", user.UserID, "conversationID", conversationID, "err", summarizeErr(err))
		}
	}

	var agentMessageText string
	if raw, ok := result["message"]; ok {
		_ = json.Unmarshal(raw, &agentMessageText)
	}
	if agentMessageText != "" {
		// Attributed to the assistant, not the customer whose token relayed it
		// — see entity.CreatedByAgent.
		if _, err := h.entity.CreateComment(ctx, entity.CreateCommentRequest{
			ReferenceID:   conversationID,
			ReferenceType: entity.ReferenceTypeConversation,
			Type:          entity.CommentTypeComment,
			Content:       agentMessageText,
			CreatedBy:     entity.CreatedByAgent,
		}); err != nil {
			slog.ErrorContext(ctx, "entity CreateComment failed for chat response", "userID", user.UserID, "conversationID", conversationID, "err", summarizeErr(err))
		}
	}

	var resolved bool
	if raw, ok := result["resolved"]; ok {
		_ = json.Unmarshal(raw, &resolved)
	}
	if resolved {
		// Never downgrade a conversation a case was created from: CONVERTED
		// outranks RESOLVED. Without this check the agent reporting the issue
		// solved would silently overwrite the converted state, and the chat
		// stopped being attributable to the case it produced.
		//
		// A failed lookup deliberately does not block the transition — the
		// pre-existing behaviour (resolve) is the safer default when the current
		// state is unknown, and this whole branch is best-effort bookkeeping.
		//
		// This read-then-write is NOT atomic, and cannot be made so from here:
		// entity.UpdateConversationRequest carries a state and nothing else, and
		// entity-service's conversation API has no conditional-update
		// precondition (no expected-state field, no If-Match/ETag). So a case
		// created from this chat in the window between the read and the write
		// still gets downgraded to RESOLVED. The window is short and one-
		// directional — a CONVERTED write landing after this one wins correctly —
		// and the result is a mis-stated state, recoverable via
		// PATCH /conversations/{id}, which accepts "converted". Closing it
		// properly needs a conditional update upstream; do not simulate one with
		// a re-read-and-correct loop here, which can lose the same way and would
		// fight a legitimate concurrent update.
		current, err := h.entity.GetConversation(ctx, conversationID)
		if err != nil {
			slog.WarnContext(ctx, "entity GetConversation failed before auto-resolve; proceeding", "userID", user.UserID, "conversationID", conversationID, "err", summarizeErr(err))
		} else if isConvertedState(current.State) {
			slog.DebugContext(ctx, "conversation already converted; skipping the resolved transition", "conversationID", conversationID)
			return
		}
		if _, err := h.entity.UpdateConversation(ctx, conversationID, entity.UpdateConversationRequest{State: wsAutoResolveState}); err != nil {
			slog.ErrorContext(ctx, "entity UpdateConversation failed to auto-resolve", "userID", user.UserID, "conversationID", conversationID, "err", summarizeErr(err))
		}
	}
}

// handleSideChannel forwards a rating or token-increase request to the AI agent
// and pipes the acknowledgement back, bypassing the chat-turn path entirely.
//
// The requester is stamped from the authenticated session rather than trusted
// from the browser, and rewritten unconditionally: when there is no session
// email to stamp, any requestedBy the client supplied is *removed* rather than
// forwarded. Otherwise that one path would let a caller write someone else's
// name into a durable audit row. So the field is either this session's user or
// absent, never client-supplied — the upstream falls back to the account when it
// is absent. Mirrors the Ballerina backend's onMessage side-channel branch.
func (h *WebSocketHandler) handleSideChannel(ctx context.Context, conn *websocket.Conn, user *middleware.UserInfo, projectID string, activeConvID *string, msgType string, parsed map[string]any) {
	conversationID, _ := parsed["conversationId"].(string)
	if conversationID == "" && activeConvID != nil && *activeConvID != "" {
		conversationID = *activeConvID
	}
	if conversationID == "" || !uuidRe.MatchString(conversationID) {
		// The client knows which answer it is rating even when this connection
		// has not carried a turn yet, so a missing id is a client bug, not a
		// resumable state. Dropped rather than surfaced: a failed rating must not
		// interrupt the chat.
		slog.ErrorContext(ctx, "discarding side-channel message: no usable conversation id", "userID", user.UserID, "msgType", msgType)
		return
	}

	payload, dropped, err := stampRequestedBy(parsed, user.Email)
	if err != nil {
		slog.ErrorContext(ctx, "failed to re-encode side-channel payload", "userID", user.UserID, "msgType", msgType, "err", summarizeErr(err))
		return
	}
	if dropped {
		// Should not happen: an authenticated session always carries an email.
		// Logged because the difference is a named requester versus an anonymous
		// audit row.
		slog.WarnContext(ctx, "dropping client-supplied requestedBy: no authenticated email on this session", "userID", user.UserID, "msgType", msgType)
	}

	agentSessionID := projectID + ":" + conversationID
	if err := h.ai.SendSideChannel(ctx, agentSessionID, string(payload), conn); err != nil {
		slog.ErrorContext(ctx, "aichatagent SendSideChannel failed", "userID", user.UserID, "conversationID", conversationID, "msgType", msgType, "err", summarizeErr(err))
	}
}

// stampRequestedBy rewrites the requester on a side-channel payload from the
// authenticated session, returning the re-encoded bytes and whether a
// client-supplied value had to be discarded.
//
// The rewrite is unconditional: when email is empty, any requestedBy the client
// supplied is *removed* rather than forwarded. Otherwise that one path would let
// a caller write someone else's name into a durable audit row. So the field is
// either this session's user or absent, never client-supplied — the upstream
// falls back to the account when it is absent.
func stampRequestedBy(parsed map[string]any, email string) (payload []byte, dropped bool, err error) {
	if email != "" {
		parsed[requestedByField] = email
	} else if _, present := parsed[requestedByField]; present {
		delete(parsed, requestedByField)
		dropped = true
	}
	payload, err = json.Marshal(parsed)
	return payload, dropped, err
}

// isConvertedState reports whether a conversation has already been converted
// into a case. Compared case-insensitively because the state is a plain string
// on the wire, and nil-safe because entity-service types it as optional.
func isConvertedState(state *string) bool {
	return state != nil && strings.EqualFold(strings.TrimSpace(*state), conversationStateConverted)
}

func writeWSJSON(conn *websocket.Conn, v any) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, payload)
}
