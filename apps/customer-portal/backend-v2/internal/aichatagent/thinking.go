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
	"unicode"
)

const (
	thinkingOpenTag  = "<thinking>"
	thinkingCloseTag = "</thinking>"
	messageKey       = "message"
)

// StripThinkingBlocks removes the model's <thinking>…</thinking> reasoning from
// a complete answer.
//
// The upstream agent can emit its reasoning inside the answer text itself. The
// browser would then show it, and the handler would store it as the
// conversation comment — which is also what the CSM portal later renders — so
// it is removed here, before the answer is forwarded or persisted.
//
// It removes complete blocks (the tag is matched case-insensitively), everything
// after an opening tag that is never closed (an answer that was cut off), and —
// once reasoning has been found — a half-written opening tag at the very end. An
// answer with no opening tag at all is returned untouched. A leading block takes
// the gap after it with it, but the first real line keeps its own indentation,
// and an answer that is nothing but whitespace afterwards becomes empty. Known
// limits: a complete block the answer merely mentions is removed too, an unclosed
// mention hides the rest of the text, and nested blocks or a stray closing tag
// are left as they are.
//
// The browser apps carry the same rule as a display-side fallback for answers
// stored before this ran; keep the behaviour in step with theirs.
func StripThinkingBlocks(text string) string {
	if !strings.Contains(text, "<") {
		return text
	}

	var kept strings.Builder
	pos := 0
	foundReasoning := false
	for {
		open := indexASCIIFold(text, thinkingOpenTag, pos)
		if open < 0 {
			kept.WriteString(text[pos:])
			break
		}
		foundReasoning = true
		kept.WriteString(text[pos:open])
		closeAt := indexASCIIFold(text, thinkingCloseTag, open+len(thinkingOpenTag))
		if closeAt < 0 {
			break // never closed: the rest is reasoning
		}
		pos = closeAt + len(thinkingCloseTag)
	}

	// Unlike the browser fallback, which sees an answer while it is still being
	// typed, this sees a complete one: a half-written opening tag is only reasoning
	// that was cut off if a reasoning block was found, otherwise it is the
	// author's own text and the answer is left exactly as it came.
	if !foundReasoning {
		return text
	}
	stripped := trimPartialOpenTag(kept.String())
	if stripped == text {
		return text
	}
	// Only a block at the very start leaves a gap to tidy; whitespace anywhere
	// else is the author's (an indented code line must stay indented).
	if !startsWithThinkingBlock(text) {
		return stripped
	}
	// Drop the gap but keep the first real line's own indentation: when the gap
	// spans lines, cut up to the last newline; when it is inline, cut it all.
	gapEnd := len(stripped) - len(strings.TrimLeftFunc(stripped, unicode.IsSpace))
	if gapEnd == len(stripped) {
		// Nothing but whitespace was left: empty, not blank, so a caller that
		// skips an empty answer skips this one too.
		return ""
	}
	if nl := strings.LastIndexByte(stripped[:gapEnd], '\n'); nl >= 0 {
		return stripped[nl+1:]
	}
	return stripped[gapEnd:]
}

// scrubFinalEvent removes <thinking> reasoning from the answer text of a
// "final" event.
//
// parsed is the event already decoded by the caller and data its raw frame. It
// returns the frame to forward and the payload map the caller persists from —
// the same map StreamChat has always returned: the nested "payload" object, or
// the event itself when the agent sends the older flat shape.
//
// Nothing is rewritten unless the answer actually carried reasoning: when it
// did not, data comes back byte-for-byte, so an ordinary answer is still
// forwarded verbatim. The same goes if anything unexpected is found (a message
// that is not a string, a value that will not re-encode) — the turn is never
// failed over this.
func scrubFinalEvent(parsed map[string]json.RawMessage, data []byte) ([]byte, map[string]json.RawMessage) {
	target := parsed
	var nested map[string]json.RawMessage
	if raw, ok := parsed[eventPayloadKey]; ok {
		if err := json.Unmarshal(raw, &nested); err == nil {
			target = nested
		} else {
			nested = nil
		}
	}

	raw, ok := target[messageKey]
	if !ok {
		return data, target
	}
	var message string
	if err := json.Unmarshal(raw, &message); err != nil {
		return data, target
	}
	cleaned := StripThinkingBlocks(message)
	if cleaned == message {
		return data, target
	}

	encoded, err := marshalNoHTMLEscape(cleaned)
	if err != nil {
		return data, target
	}
	target[messageKey] = encoded

	if nested != nil {
		reencoded, err := marshalNoHTMLEscape(nested)
		if err != nil {
			return data, target
		}
		parsed[eventPayloadKey] = reencoded
	}
	frame, err := marshalNoHTMLEscape(parsed)
	if err != nil {
		return data, target
	}
	return frame, target
}

// marshalNoHTMLEscape encodes v without turning <, > and & into <-style
// escapes, so the answer text reads the same on the wire as it did upstream.
func marshalNoHTMLEscape(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// startsWithThinkingBlock reports whether the first non-space thing in text is
// an opening tag.
func startsWithThinkingBlock(text string) bool {
	trimmed := strings.TrimLeftFunc(text, unicode.IsSpace)
	return len(trimmed) >= len(thinkingOpenTag) &&
		equalASCIIFold(trimmed[:len(thinkingOpenTag)], thinkingOpenTag)
}

// trimPartialOpenTag drops a half-written opening tag ("<t" … "<thinking") from
// the very end of text.
func trimPartialOpenTag(text string) string {
	longest := len(thinkingOpenTag) - 1
	if longest > len(text) {
		longest = len(text)
	}
	for n := longest; n >= 2; n-- {
		if equalASCIIFold(text[len(text)-n:], thinkingOpenTag[:n]) {
			return text[:len(text)-n]
		}
	}
	return text
}

// indexASCIIFold returns the index of the first occurrence of tag in s at or
// after from, ignoring ASCII case, or -1. tag must be lower case. The scan is
// linear in len(s); unlike a lazy regular expression it never rescans the rest
// of the text for each unclosed opener.
func indexASCIIFold(s, tag string, from int) int {
	for i := from; i+len(tag) <= len(s); i++ {
		if s[i] == '<' && equalASCIIFold(s[i:i+len(tag)], tag) {
			return i
		}
	}
	return -1
}

// equalASCIIFold compares s with the lower-case ASCII string lower, ignoring
// ASCII case. Both must be the same length.
func equalASCIIFold(s, lower string) bool {
	if len(s) != len(lower) {
		return false
	}
	for i := 0; i < len(lower); i++ {
		c := s[i]
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != lower[i] {
			return false
		}
	}
	return true
}
