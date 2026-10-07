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
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestStripThinkingBlocks(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "leading block and its gap are removed",
			in:   "<thinking> The user asks about Widget 2.\n- Dev: Widget 1\nI should ask.\n</thinking>\n\nI don't see Widget 2 in your environments.",
			want: "I don't see Widget 2 in your environments.",
		},
		{"every block, in any letter case", "A<thinking>x</thinking>B<Thinking>y</THINKING>C", "ABC"},
		{"block that is never closed", "<thinking>The user wants", ""},
		{"unclosed block after some answer", "Done.\n<thinking>more reasoning", "Done.\n"},
		{"half-written opening tag after a block", "<thinking>x</thinking>Hello <t", "Hello "},
		{"longer half-written opening tag after a block", "A<thinking>x</thinking>B <thin", "AB "},
		{"half-written opening tag with no block is the author's text", "Hello <t", "Hello <t"},
		{"opening tag without its bracket and no block is the author's text", "Hello <thinking", "Hello <thinking"},
		{"whitespace left after a leading block becomes empty", "<thinking>x</thinking>\n  ", ""},
		{"whitespace around a block that is the whole answer becomes empty", " \n<thinking>x</thinking>\n\n\n", ""},
		{"inline whitespace left after a leading block becomes empty", "<thinking>x</thinking>   ", ""},
		{"indentation kept when a later block is removed", "    code\n<thinking>reason</thinking>", "    code\n"},
		{"first real line keeps its indentation", "<thinking>x</thinking>\n\n    code", "    code"},
		{"inline gap after a leading block is dropped", "<thinking>x</thinking>   Answer", "Answer"},
		{"whitespace before a leading block is dropped", "\n <thinking>x</thinking>\nAnswer", "Answer"},
		{"non-ASCII text survives", "é<thinking>x</thinking>ü — héllo ✓", "éü — héllo ✓"},
		{"only reasoning", "<thinking>just this</thinking>", ""},
		{"markup that only looks similar is untouched", "Use <table><thead><tr><th>A</th></tr></thead></table> markup", "Use <table><thead><tr><th>A</th></tr></thead></table> markup"},
		{"a comparison is untouched", "if (a <t) {}", "if (a <t) {}"},
		{"a similarly named tag is untouched", "<thinking-time> custom tag</thinking-time>", "<thinking-time> custom tag</thinking-time>"},
		{"title tag is untouched", "<title>Doc</title>", "<title>Doc</title>"},
		{"no angle bracket at all", "plain answer", "plain answer"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripThinkingBlocks(tt.in); got != tt.want {
				t.Errorf("StripThinkingBlocks(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// A rescan of the rest of the text for every unclosed opener takes several
// seconds on this input; a single pass takes well under a millisecond. The bound
// is ~1000x the real cost so load cannot flake it.
func TestStripThinkingBlocks_LinearInTheNumberOfOpeners(t *testing.T) {
	in := strings.Repeat("<thinking>x", 50_000)
	start := time.Now()
	if got := StripThinkingBlocks(in); got != "" {
		t.Fatalf("got %d bytes, want none", len(got))
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("took %v, want a single linear pass", elapsed)
	}
}

func decodeEvent(t *testing.T, frame []byte) map[string]json.RawMessage {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(frame, &m); err != nil {
		t.Fatalf("frame is not a JSON object: %v\n%s", err, frame)
	}
	return m
}

func messageOf(t *testing.T, fields map[string]json.RawMessage) string {
	t.Helper()
	var s string
	if err := json.Unmarshal(fields["message"], &s); err != nil {
		t.Fatalf("message is not a string: %v", err)
	}
	return s
}

func TestScrubFinalEvent(t *testing.T) {
	const reasoning = "<thinking>internal notes</thinking>\n\n"

	t.Run("nested payload: answer cleaned, everything else kept", func(t *testing.T) {
		data := []byte(`{"type":"final","payload":{"message":"` + strings.ReplaceAll(reasoning, "\n", `\n`) + `Which <b>gateway</b>?","conversationId":"c1","resolved":false,"intent":{"intentId":"i1"}}}`)
		frame, target := scrubFinalEvent(decodeEvent(t, data), data)

		event := decodeEvent(t, frame)
		if string(event["type"]) != `"final"` {
			t.Errorf("type = %s", event["type"])
		}
		payload := decodeEvent(t, event["payload"])
		if got := messageOf(t, payload); got != "Which <b>gateway</b>?" {
			t.Errorf("forwarded message = %q", got)
		}
		if string(payload["conversationId"]) != `"c1"` || string(payload["resolved"]) != `false` {
			t.Errorf("other payload fields changed: %s", event["payload"])
		}
		if !strings.Contains(string(payload["intent"]), `"intentId":"i1"`) {
			t.Errorf("nested object lost: %s", payload["intent"])
		}
		if got := messageOf(t, target); got != "Which <b>gateway</b>?" {
			t.Errorf("persisted message = %q", got)
		}
		if bytes.Contains(frame, []byte{'\\', 'u', '0', '0', '3', 'c'}) { // a JSON \u003c escape
			t.Errorf("answer text was HTML-escaped on the wire: %s", frame)
		}
	})

	t.Run("an answer without reasoning is forwarded byte for byte", func(t *testing.T) {
		data := []byte(`{ "type": "final", "payload": { "message": "Which <b>gateway</b>?", "conversationId": "c1" } }`)
		frame, target := scrubFinalEvent(decodeEvent(t, data), data)
		if !bytes.Equal(frame, data) {
			t.Errorf("frame was rewritten:\n got %s\nwant %s", frame, data)
		}
		if got := messageOf(t, target); got != "Which <b>gateway</b>?" {
			t.Errorf("persisted message = %q", got)
		}
	})

	t.Run("older flat shape", func(t *testing.T) {
		data := []byte(`{"type":"final","message":"<thinking>x</thinking>Hello","conversationId":"c1"}`)
		parsed := decodeEvent(t, data)
		frame, target := scrubFinalEvent(parsed, data)
		if got := messageOf(t, decodeEvent(t, frame)); got != "Hello" {
			t.Errorf("forwarded message = %q", got)
		}
		if got := messageOf(t, target); got != "Hello" {
			t.Errorf("persisted message = %q", got)
		}
	})

	t.Run("an answer that was only reasoning becomes empty, not an error", func(t *testing.T) {
		data := []byte(`{"type":"final","payload":{"message":"<thinking>only this</thinking>"}}`)
		frame, target := scrubFinalEvent(decodeEvent(t, data), data)
		if got := messageOf(t, decodeEvent(t, decodeEvent(t, frame)["payload"])); got != "" {
			t.Errorf("forwarded message = %q", got)
		}
		if got := messageOf(t, target); got != "" {
			t.Errorf("persisted message = %q", got)
		}
	})

	untouched := map[string]string{
		"message that is not a string":  `{"type":"final","payload":{"message":{"message":"<thinking>x</thinking>Hi"}}}`,
		"no message at all":             `{"type":"final","payload":{"conversationId":"c1"}}`,
		"payload that is not an object": `{"type":"final","payload":"<thinking>x</thinking>Hi"}`,
		"null payload":                  `{"type":"final","payload":null}`,
	}
	for name, raw := range untouched {
		t.Run(name+" is left alone", func(t *testing.T) {
			data := []byte(raw)
			frame, _ := scrubFinalEvent(decodeEvent(t, data), data)
			if !bytes.Equal(frame, data) {
				t.Errorf("frame was rewritten:\n got %s\nwant %s", frame, data)
			}
		})
	}
}
