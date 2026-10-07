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

import ballerina/test;
import ballerina/time;

function stripCases() returns map<[string, string]> {
    return {
        "leading block and its gap are removed": [
            "<thinking> The user asks about Widget 2.\n- Dev: Widget 1\nI should ask.\n</thinking>\n\nI don't see Widget 2 in your environments.",
            "I don't see Widget 2 in your environments."
        ],
        "every block, in any letter case": ["A<thinking>x</thinking>B<Thinking>y</THINKING>C", "ABC"],
        "block that is never closed": ["<thinking>The user wants", ""],
        "unclosed block after some answer": ["Done.\n<thinking>more reasoning", "Done.\n"],
        "half-written opening tag after a block": ["<thinking>x</thinking>Hello <t", "Hello "],
        "longer half-written opening tag after a block": ["A<thinking>x</thinking>B <thin", "AB "],
        "half-written opening tag with no block is the author's text": ["Hello <t", "Hello <t"],
        "opening tag without its bracket and no block is the author's text": ["Hello <thinking", "Hello <thinking"],
        "whitespace left after a leading block becomes empty": ["<thinking>x</thinking>\n  ", ""],
        "whitespace around a block that is the whole answer becomes empty": [" \n<thinking>x</thinking>\n\n\n", ""],
        "inline whitespace left after a leading block becomes empty": ["<thinking>x</thinking>   ", ""],
        "indentation kept when a later block is removed": ["    code\n<thinking>reason</thinking>", "    code\n"],
        "first real line keeps its indentation": ["<thinking>x</thinking>\n\n    code", "    code"],
        "inline gap after a leading block is dropped": ["<thinking>x</thinking>   Answer", "Answer"],
        "whitespace before a leading block is dropped": ["\n <thinking>x</thinking>\nAnswer", "Answer"],
        "non-ASCII text survives": ["é<thinking>x</thinking>ü — héllo ✓", "éü — héllo ✓"],
        "only reasoning": ["<thinking>just this</thinking>", ""],
        "markup that only looks similar is untouched": [
            "Use <table><thead><tr><th>A</th></tr></thead></table> markup",
            "Use <table><thead><tr><th>A</th></tr></thead></table> markup"
        ],
        "a comparison is untouched": ["if (a <t) {}", "if (a <t) {}"],
        "a similarly named tag is untouched": [
            "<thinking-time> custom tag</thinking-time>",
            "<thinking-time> custom tag</thinking-time>"
        ],
        "title tag is untouched": ["<title>Doc</title>", "<title>Doc</title>"],
        "no angle bracket at all": ["plain answer", "plain answer"],
        "empty": ["", ""]
    };
}

@test:Config {dataProvider: stripCases}
function testStripThinkingBlocks(string input, string expected) {
    test:assertEquals(stripThinkingBlocks(input), expected);
}

// Many CLOSED blocks is the input that tells a linear pass from a quadratic one:
// rebuilding the result by repeated string concatenation copies the whole result
// each time, which is far slower, while a single pass is quick (this whole module's
// tests run in well under a second). The 1s bound is a guard against a gross
// regression, not a benchmark, and leaves wide room for a loaded machine. (Many
// unclosed openers stop at the first one, so they would not discriminate.)
@test:Config {}
function testStripThinkingBlocksIsLinearInTheNumberOfBlocks() {
    string[] blocks = [];
    string[] expected = [];
    foreach int _ in 0 ..< 50000 {
        blocks.push("<thinking>x</thinking>a");
        expected.push("a");
    }
    string input = string:'join("", ...blocks);

    time:Utc started = time:utcNow();
    string result = stripThinkingBlocks(input);
    decimal elapsed = time:utcDiffSeconds(time:utcNow(), started);

    test:assertEquals(result, string:'join("", ...expected));
    test:assertTrue(elapsed < 1.0d, string `took ${elapsed}s, want a single linear pass`);
}

@test:Config {}
function testStripThinkingBlocksHandlesManyUnclosedOpeners() {
    string[] openers = [];
    foreach int _ in 0 ..< 50000 {
        openers.push("<thinking>x");
    }
    test:assertEquals(stripThinkingBlocks(string:'join("", ...openers)), "");
}

@test:Config {}
function testScrubFinalEventCleansTheNestedPayload() returns error? {
    map<json> event = {
        "type": "final",
        "payload": {
            "message": "<thinking>internal notes</thinking>\n\nWhich <b>gateway</b>?",
            "conversationId": "c1",
            "resolved": false
        }
    };
    string raw = event.toJsonString();

    [string, map<json>] result = scrubFinalEvent(event.clone(), raw);

    map<json> forwarded = <map<json>>check result[0].fromJsonString();
    map<json> forwardedPayload = <map<json>>forwarded["payload"];
    test:assertEquals(forwarded["type"], "final");
    test:assertEquals(forwardedPayload["message"], "Which <b>gateway</b>?");
    test:assertEquals(forwardedPayload["conversationId"], "c1");
    test:assertEquals(forwardedPayload["resolved"], false);
    // What the portal persists as the conversation comment.
    test:assertEquals(result[1]["message"], "Which <b>gateway</b>?");
    test:assertEquals(result[1]["conversationId"], "c1");
}

@test:Config {}
function testScrubFinalEventForwardsAnOrdinaryAnswerByteForByte() {
    map<json> event = {"type": "final", "payload": {"message": "Which <b>gateway</b>?", "conversationId": "c1"}};
    string raw = "{ \"type\": \"final\", \"payload\": { \"message\": \"Which <b>gateway</b>?\" } }";

    [string, map<json>] result = scrubFinalEvent(event.clone(), raw);

    test:assertEquals(result[0], raw);
    test:assertEquals(result[1]["message"], "Which <b>gateway</b>?");
}

@test:Config {}
function testScrubFinalEventHandlesTheOlderFlatShape() {
    map<json> event = {"type": "final", "message": "<thinking>x</thinking>Hello", "conversationId": "c1"};

    [string, map<json>] result = scrubFinalEvent(event.clone(), event.toJsonString());

    test:assertEquals(result[1]["message"], "Hello");
    test:assertTrue(result[0].includes("\"message\":\"Hello\""));
}

@test:Config {}
function testScrubFinalEventLeavesUnexpectedShapesAlone() {
    map<json>[] events = [
        {"type": "final", "payload": {"message": {"message": "<thinking>x</thinking>Hi"}}},
        {"type": "final", "payload": {"conversationId": "c1"}},
        {"type": "final", "payload": "<thinking>x</thinking>Hi"},
        {"type": "final", "payload": ()}
    ];
    foreach map<json> event in events {
        string raw = event.toJsonString();
        map<json> original = event.clone();
        [string, map<json>] result = scrubFinalEvent(event.clone(), raw);
        test:assertEquals(result[0], raw);
        // The payload map handed back for persisting is the same one the base code produced:
        // the nested object when there is one, the whole event otherwise.
        json payload = original["payload"] ?: original;
        map<json> expectedTarget = payload is map<json> ? payload : original;
        test:assertEquals(result[1], expectedTarget);
    }
}

@test:Config {}
function testScrubFinalEventTurnsAReasoningOnlyAnswerIntoAnEmptyOne() returns error? {
    map<json> event = {"type": "final", "payload": {"message": "<thinking>only this</thinking>", "resolved": true}};

    [string, map<json>] result = scrubFinalEvent(event.clone(), event.toJsonString());

    map<json> forwarded = <map<json>>check result[0].fromJsonString();
    map<json> forwardedPayload = <map<json>>forwarded["payload"];
    test:assertEquals(forwardedPayload["message"], "");
    test:assertEquals(forwardedPayload["resolved"], true);
    test:assertEquals(result[1]["message"], "");
}
