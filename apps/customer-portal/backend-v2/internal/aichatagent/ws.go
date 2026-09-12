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

package aichatagent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// eventTypeKey/eventPayloadKey/eventFinal/eventError mirror the upstream AI
// chat agent's WebSocket event envelope — see apps/customer-portal/backend's
// modules/ai_chat_agent/constants.bal.
const (
	eventTypeKey    = "type"
	eventPayloadKey = "payload"
	eventFinal      = "final"
	eventError      = "error"

	// eventFeedbackAck/eventTokenRequestAck terminate a side-channel exchange.
	// The agent answers a rating or a token-increase request with one of these
	// and never sends a "final" — see SendSideChannel.
	eventFeedbackAck     = "feedback_ack"
	eventTokenRequestAck = "token_request_ack"
)

// sideChannelTimeout bounds how long SendSideChannel waits for an
// acknowledgement. The upstream answers these in well under a second, so this
// is far shorter than idleTimeout — a side-channel exchange must never park an
// upstream connection for the length of a chat turn.
const sideChannelTimeout = 30 * time.Second

// maxMessageBytes bounds the size of a single frame read from the upstream
// AI chat agent — mirrors internal/handler/websocket.go's wsMaxMessageBytes
// for the browser-facing side of the same proxy.
const maxMessageBytes = 64 << 10 // 64 KiB

// idleTimeout bounds how long StreamChat waits for the next frame from an
// otherwise-healthy upstream connection.
const idleTimeout = 5 * time.Minute

// WSConfig holds the configuration for dialing the upstream AI chat agent's
// WebSocket endpoint. Kept separate from Config because the WebSocket
// connection uses its own distinct OAuth2 client-credentials configuration.
type WSConfig struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// WSClient dials the upstream AI chat agent's WebSocket endpoint.
type WSClient struct {
	baseURL string
	// tokens is a reusing token source, not a bare *clientcredentials.Config.
	// Config.Token builds and discards a fresh source per call, so every chat
	// turn and side-channel message paid a full token-endpoint round trip
	// before the handshake -- added first-byte latency on each message, and one
	// identity-provider grant per message across all users. Config.TokenSource
	// wraps the same grant in an oauth2.ReuseTokenSource, so a token is fetched
	// once and reused until it expires. Same reasoning as NewClient's cached
	// oauth client for the REST path.
	tokens oauth2.TokenSource
}

// NewWSClient constructs a WSClient authenticated via the OAuth2 client
// credentials grant.
func NewWSClient(cfg WSConfig) *WSClient {
	cc := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
		Scopes:       cfg.Scopes,
	}

	// The token source captures this context, not the per-dial caller's, so a
	// token refresh is bounded by tokenFetchTimeout rather than by whichever
	// request happened to trigger it -- the same trade-off NewClient makes.
	tokenCtx := context.WithValue(context.Background(), oauth2.HTTPClient,
		&http.Client{Timeout: tokenFetchTimeout, CheckRedirect: noRedirect})

	return &WSClient{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		tokens:  cc.TokenSource(tokenCtx),
	}
}

// dial opens a WebSocket connection to the upstream AI chat agent for the
// given session ID, authenticated with a bearer token obtained via OAuth2
// client credentials.
func (c *WSClient) dial(ctx context.Context, sessionID string) (*websocket.Conn, error) {
	token, err := c.tokens.Token()
	if err != nil {
		return nil, fmt.Errorf("aichatagent: fetch WS token: %w", err)
	}

	wsURL := strings.Replace(c.baseURL, "https://", "wss://", 1)
	wsURL = strings.Replace(wsURL, "http://", "ws://", 1)
	wsURL += "/ws?sessionId=" + url.QueryEscape(sessionID)

	header := http.Header{}
	header.Set("Authorization", "Bearer "+token.AccessToken)

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, header)
	if err != nil {
		return nil, fmt.Errorf("aichatagent: dial upstream WebSocket: %w", err)
	}
	return conn, nil
}

// BrowserConn abstracts the browser-facing WebSocket connection just enough
// for StreamChat to forward events to it, so the handler package's real
// *websocket.Conn doesn't need to be imported here.
type BrowserConn interface {
	WriteMessage(messageType int, data []byte) error
}

// StreamChat opens a dedicated upstream WebSocket connection for sessionID,
// sends payload, then forwards every event verbatim to caller until a
// "final" event arrives, an "error" event arrives, a read/write fails, or
// the upstream connection closes normally. Mirrors
// apps/customer-portal/backend's ai_chat_agent:streamChat, except it also
// reports failure to the caller (below) via a non-nil error, so a failed
// turn is never mistaken for a successful one.
func (c *WSClient) StreamChat(ctx context.Context, sessionID, payload string, caller BrowserConn) (map[string]json.RawMessage, error) {
	conn, err := c.dial(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	conn.SetReadLimit(maxMessageBytes)

	if err := conn.WriteMessage(websocket.TextMessage, []byte(payload)); err != nil {
		return nil, fmt.Errorf("aichatagent: write initial message: %w", err)
	}

	finalPayload := map[string]json.RawMessage{}
	for {
		deadline := time.Now().Add(idleTimeout)
		if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
			deadline = dl
		}
		_ = conn.SetReadDeadline(deadline)

		_, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				break
			}
			// Never forward err.Error() to the browser — it can contain the
			// upstream host/URL/TLS detail of this internal Python service.
			// The caller (internal/handler/websocket.go) logs the real error
			// returned here and sends its own fixed-text error event.
			return nil, fmt.Errorf("aichatagent: read upstream message: %w", err)
		}

		if writeErr := caller.WriteMessage(websocket.TextMessage, data); writeErr != nil {
			return nil, fmt.Errorf("aichatagent: forward message to browser: %w", writeErr)
		}

		var parsed map[string]json.RawMessage
		if err := json.Unmarshal(data, &parsed); err != nil {
			continue
		}
		var evtType string
		if raw, ok := parsed[eventTypeKey]; ok {
			_ = json.Unmarshal(raw, &evtType)
		}
		if evtType == eventFinal {
			if raw, ok := parsed[eventPayloadKey]; ok {
				var nested map[string]json.RawMessage
				if err := json.Unmarshal(raw, &nested); err == nil {
					finalPayload = nested
					break
				}
			}
			finalPayload = parsed
			break
		}
		if evtType == eventError {
			var upstreamMsg string
			if raw, ok := parsed["message"]; ok {
				_ = json.Unmarshal(raw, &upstreamMsg)
			}
			return nil, fmt.Errorf("aichatagent: upstream reported an error: %s", upstreamMsg)
		}
	}

	_ = conn.WriteControl(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, "session complete"),
		time.Now().Add(2*time.Second))

	return finalPayload, nil
}

// SendSideChannel forwards a side-channel message — an answer rating, or a
// request to raise a token limit — to the upstream agent and pipes events back
// to caller until it acknowledges.
//
// Deliberately separate from StreamChat, because these are not chat turns: the
// agent answers them with a "*_ack" and never sends a "final". Routing them
// through StreamChat left it reading for an event that could not arrive until
// the read deadline expired, and because the browser-facing read loop is
// strictly sequential (it does not read the next frame until the current one is
// handled) the whole connection stalled for that long — the customer's next
// message was not even read off the socket, so the chat looked dead while the
// socket was fine. Mirrors apps/customer-portal/backend's
// ai_chat_agent:sendSideChannelMessage.
//
// A read timeout or a mid-exchange failure is reported as a nil error: the write
// succeeded, so the rating may well have been stored, and telling the customer
// their rating failed would be worse than saying nothing. Only a failure to
// open or write to the upstream connection returns an error.
func (c *WSClient) SendSideChannel(ctx context.Context, sessionID, payload string, caller BrowserConn) error {
	conn, err := c.dial(ctx, sessionID)
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetReadLimit(maxMessageBytes)

	if err := conn.WriteMessage(websocket.TextMessage, []byte(payload)); err != nil {
		return fmt.Errorf("aichatagent: write side-channel message: %w", err)
	}

	acknowledged := false
	for {
		deadline := time.Now().Add(sideChannelTimeout)
		if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
			deadline = dl
		}
		_ = conn.SetReadDeadline(deadline)

		_, data, err := conn.ReadMessage()
		if err != nil {
			// Includes the read timeout and a normal upstream close.
			break
		}
		if err := caller.WriteMessage(websocket.TextMessage, data); err != nil {
			// The browser went away; nothing left to forward to.
			break
		}

		var parsed map[string]json.RawMessage
		if err := json.Unmarshal(data, &parsed); err != nil {
			continue
		}
		var evtType string
		if raw, ok := parsed[eventTypeKey]; ok {
			_ = json.Unmarshal(raw, &evtType)
		}
		if evtType == eventFeedbackAck || evtType == eventTokenRequestAck || evtType == eventError {
			acknowledged = true
			break
		}
	}

	if acknowledged {
		_ = conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "acknowledged"),
			time.Now().Add(2*time.Second))
	}
	return nil
}
