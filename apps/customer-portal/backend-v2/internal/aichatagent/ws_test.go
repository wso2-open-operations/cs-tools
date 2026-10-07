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
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/oauth2"
)

// recordingConn is a BrowserConn that keeps every frame StreamChat forwards.
type recordingConn struct {
	mu     sync.Mutex
	frames []string
}

func (c *recordingConn) WriteMessage(_ int, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.frames = append(c.frames, string(data))
	return nil
}

func (c *recordingConn) all() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.frames...)
}

// fakeAgent serves one chat turn: it reads the user's message, then plays back
// the given frames and closes.
func fakeAgent(t *testing.T, frames ...string) *WSClient {
	t.Helper()
	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		if _, _, err := conn.ReadMessage(); err != nil {
			t.Errorf("read user message: %v", err)
			return
		}
		for _, f := range frames {
			if err := conn.WriteMessage(websocket.TextMessage, []byte(f)); err != nil {
				return
			}
		}
		_ = conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
	}))
	t.Cleanup(srv.Close)
	return &WSClient{
		baseURL: srv.URL,
		tokens:  oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"}),
	}
}

func TestStreamChat_RemovesReasoningFromTheFinalAnswer(t *testing.T) {
	const (
		token = `{"type":"token","content":"<thinking>so far"}`
		final = `{"type":"final","payload":{"message":"<thinking>internal notes</thinking>\n\nWhich gateway?","conversationId":"c1"}}`
	)
	client := fakeAgent(t, token, final)
	browser := &recordingConn{}

	result, err := client.StreamChat(context.Background(), "p1:c1", `{"type":"user_message","message":"hi"}`, browser)
	if err != nil {
		t.Fatalf("StreamChat: %v", err)
	}

	frames := browser.all()
	if len(frames) != 2 {
		t.Fatalf("browser got %d frames, want 2: %q", len(frames), frames)
	}
	if frames[0] != token {
		t.Errorf("token frame was altered: %s", frames[0])
	}
	finalEvent := decodeEvent(t, []byte(frames[1]))
	if got := messageOf(t, decodeEvent(t, finalEvent["payload"])); got != "Which gateway?" {
		t.Errorf("answer sent to the browser = %q", got)
	}
	// What the handler persists as the conversation comment.
	if got := messageOf(t, result); got != "Which gateway?" {
		t.Errorf("answer returned for persisting = %q", got)
	}
	if string(result["conversationId"]) != `"c1"` {
		t.Errorf("other payload fields lost: %v", result)
	}
}

func TestStreamChat_ForwardsAnOrdinaryAnswerUnchanged(t *testing.T) {
	const final = `{"type":"final","payload":{"message":"Which <b>gateway</b>?","conversationId":"c1"}}`
	client := fakeAgent(t, `{"type":"thinking_start"}`, `not json at all`, final)
	browser := &recordingConn{}

	result, err := client.StreamChat(context.Background(), "p1:c1", `{"type":"user_message","message":"hi"}`, browser)
	if err != nil {
		t.Fatalf("StreamChat: %v", err)
	}

	want := []string{`{"type":"thinking_start"}`, `not json at all`, final}
	got := browser.all()
	if len(got) != len(want) {
		t.Fatalf("browser got %d frames, want %d: %q", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("frame %d was altered:\n got %s\nwant %s", i, got[i], want[i])
		}
	}
	if got := messageOf(t, result); got != "Which <b>gateway</b>?" {
		t.Errorf("answer returned for persisting = %q", got)
	}
}

func TestStreamChat_StillReportsAnUpstreamError(t *testing.T) {
	client := fakeAgent(t, `{"type":"error","message":"boom"}`)
	browser := &recordingConn{}

	if _, err := client.StreamChat(context.Background(), "p1:c1", `{"type":"user_message","message":"hi"}`, browser); err == nil {
		t.Fatal("want an error for an upstream error event")
	}
	if got := browser.all(); len(got) != 1 || got[0] != `{"type":"error","message":"boom"}` {
		t.Errorf("error frame not forwarded as it came: %q", got)
	}
}

func TestStreamChat_CleansTheOlderFlatShapeToo(t *testing.T) {
	client := fakeAgent(t, `{"type":"final","message":"<thinking>x</thinking>Hello","conversationId":"c1"}`)
	browser := &recordingConn{}

	result, err := client.StreamChat(context.Background(), "p1:c1", `{"type":"user_message","message":"hi"}`, browser)
	if err != nil {
		t.Fatalf("StreamChat: %v", err)
	}

	frames := browser.all()
	if len(frames) != 1 {
		t.Fatalf("browser got %d frames, want 1: %q", len(frames), frames)
	}
	if got := messageOf(t, decodeEvent(t, []byte(frames[0]))); got != "Hello" {
		t.Errorf("answer sent to the browser = %q", got)
	}
	if got := messageOf(t, result); got != "Hello" {
		t.Errorf("answer returned for persisting = %q", got)
	}
}

// A final event with a null payload has always returned a nil payload map and
// the frame untouched; cleaning must not turn that into a panic or a rewrite.
func TestStreamChat_ANullPayloadIsForwardedAsItCame(t *testing.T) {
	const final = `{"type":"final","payload":null}`
	client := fakeAgent(t, final)
	browser := &recordingConn{}

	result, err := client.StreamChat(context.Background(), "p1:c1", `{"type":"user_message","message":"hi"}`, browser)
	if err != nil {
		t.Fatalf("StreamChat: %v", err)
	}
	if got := browser.all(); len(got) != 1 || got[0] != final {
		t.Errorf("frame was altered: %q", got)
	}
	if len(result) != 0 {
		t.Errorf("result = %v, want an empty payload", result)
	}
}
