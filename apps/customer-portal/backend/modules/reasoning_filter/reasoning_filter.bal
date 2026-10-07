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

// Removes the model's <thinking>…</thinking> reasoning from AI chat agent answers.
//
// Kept as its own module, with no network dependencies, so the logic can be tested
// without the AI chat agent module's OAuth2 clients (which contact a token endpoint
// when the module initialises).

# Opening tag of the model's reasoning block.
const string THINKING_OPEN_TAG = "<thinking>";

# Closing tag of the model's reasoning block.
const string THINKING_CLOSE_TAG = "</thinking>";

# JSON key holding the answer text in a "final" event payload.
const string MESSAGE_KEY = "message";

# JSON key of the payload object in a "final" event (the same key the AI chat agent module reads).
const string EVENT_PAYLOAD_KEY = "payload";

# Remove the model's <thinking>…</thinking> reasoning from a complete answer.
#
# The upstream agent can emit its reasoning inside the answer text itself. The
# browser would then show it, and the portal would store it as the conversation
# comment (which the CSM portal later renders), so it is removed here, before the
# answer is forwarded or persisted.
#
# It removes complete blocks (the tag is matched case-insensitively), everything
# after an opening tag that is never closed (an answer that was cut off), and, once
# reasoning has been found, a half-written opening tag at the very end. An answer
# with no opening tag at all is returned untouched. A leading block takes the gap
# after it with it, but the first real line keeps its own indentation, and an
# answer that is nothing but whitespace afterwards becomes empty. Known limits: a
# complete block the answer merely mentions is removed too, an unclosed mention
# hides the rest of the text, and nested blocks or a stray closing tag are left
# as they are.
#
# The browser apps carry the same rule as a display-side fallback for answers
# stored before this ran; keep the behaviour in step with theirs.
#
# + text - The answer text
# + return - The answer without its reasoning (the same string when it had none)
public isolated function stripThinkingBlocks(string text) returns string {
    if !text.includes("<") {
        return text;
    }

    // Lower-casing only touches ASCII, so indexes into `lower` are valid in `text`.
    string lower = text.toLowerAscii();
    string[] kept = [];
    int pos = 0;
    boolean foundReasoning = false;
    while true {
        int? open = lower.indexOf(THINKING_OPEN_TAG, pos);
        if open is () {
            kept.push(text.substring(pos));
            break;
        }
        foundReasoning = true;
        kept.push(text.substring(pos, open));
        int? closeAt = lower.indexOf(THINKING_CLOSE_TAG, open + THINKING_OPEN_TAG.length());
        if closeAt is () {
            break; // never closed: the rest is reasoning
        }
        pos = closeAt + THINKING_CLOSE_TAG.length();
    }

    // Unlike the browser fallback, which sees an answer while it is still being typed, this sees a
    // complete one: a half-written opening tag is only reasoning that was cut off if a reasoning
    // block was found, otherwise it is the author's own text and the answer is left exactly as it came.
    if !foundReasoning {
        return text;
    }
    string stripped = trimPartialOpenTag(string:'join("", ...kept));
    if stripped == text {
        return text;
    }
    // Only a block at the very start leaves a gap to tidy; whitespace anywhere
    // else is the author's (an indented code line must stay indented).
    if !startsWithThinkingBlock(lower) {
        return stripped;
    }
    // Drop the gap but keep the first real line's own indentation: when the gap
    // spans lines, cut up to the last newline; when it is inline, cut it all.
    int gapEnd = leadingWhitespaceEnd(stripped);
    if gapEnd == stripped.length() {
        // Nothing but whitespace was left: empty, not blank, so a caller that skips an empty
        // answer skips this one too.
        return "";
    }
    int? lastNewline = stripped.substring(0, gapEnd).lastIndexOf("\n");
    return lastNewline is () ? stripped.substring(gapEnd) : stripped.substring(lastNewline + 1);
}

# Remove <thinking> reasoning from the answer text of a "final" event.
#
# Nothing is rewritten unless the answer actually carried reasoning: when it did
# not, the frame comes back exactly as the upstream sent it, so an ordinary answer
# is still forwarded verbatim. The same goes for anything unexpected (a message
# that is not a string): the turn is never failed over this.
#
# + parsed - The "final" event, already decoded (it is updated in place when the answer is cleaned)
# + rawFrame - The frame exactly as the upstream sent it
# + return - The frame to forward, and the payload map to persist from: the
# nested "payload" object, or the event itself for the older flat shape
public isolated function scrubFinalEvent(map<json> parsed, string rawFrame) returns [string, map<json>] {
    json eventPayload = parsed[EVENT_PAYLOAD_KEY] ?: parsed;
    map<json> target = eventPayload is map<json> ? eventPayload : parsed;

    json message = target[MESSAGE_KEY] ?: ();
    if message !is string {
        return [rawFrame, target];
    }
    string cleaned = stripThinkingBlocks(message);
    if cleaned == message {
        return [rawFrame, target];
    }
    target[MESSAGE_KEY] = cleaned;
    return [parsed.toJsonString(), target];
}

# Drop a half-written opening tag ("<t" … "<thinking") from the very end of the text.
#
# + text - The text
# + return - The text without such a suffix
isolated function trimPartialOpenTag(string text) returns string {
    int length = text.length();
    int n = int:min(THINKING_OPEN_TAG.length() - 1, length);
    while n >= 2 {
        if text.substring(length - n).toLowerAscii() == THINKING_OPEN_TAG.substring(0, n) {
            return text.substring(0, length - n);
        }
        n -= 1;
    }
    return text;
}

# Whether the first non-space thing in the (already lower-cased) text is an opening tag.
#
# + lower - The lower-cased text
# + return - True when the text starts with an opening tag, ignoring leading whitespace
isolated function startsWithThinkingBlock(string lower) returns boolean {
    int tagStart = leadingWhitespaceEnd(lower);
    int tagLength = THINKING_OPEN_TAG.length();
    return tagStart + tagLength <= lower.length()
        && lower.substring(tagStart, tagStart + tagLength) == THINKING_OPEN_TAG;
}

# Index of the first non-whitespace character of the text (its length when it is all whitespace).
#
# + text - The text
# + return - The index
isolated function leadingWhitespaceEnd(string text) returns int {
    int i = 0;
    int length = text.length();
    while i < length && isWhitespace(text.getCodePoint(i)) {
        i += 1;
    }
    return i;
}

# Whether a code point is whitespace (the same set a regular expression `\s` covers).
#
# + codePoint - The code point
# + return - True for whitespace
isolated function isWhitespace(int codePoint) returns boolean {
    return codePoint == 32 || (codePoint >= 9 && codePoint <= 13) || codePoint == 0xA0
        || codePoint == 0x1680 || (codePoint >= 0x2000 && codePoint <= 0x200A)
        || codePoint == 0x2028 || codePoint == 0x2029 || codePoint == 0x202F
        || codePoint == 0x205F || codePoint == 0x3000 || codePoint == 0xFEFF;
}
