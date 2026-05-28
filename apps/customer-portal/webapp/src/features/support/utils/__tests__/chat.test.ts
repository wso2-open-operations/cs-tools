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

import { describe, expect, it } from "vitest";
import {
  displayTextFromConversationContent,
  getFinalMessageFromPayload,
  sanitizeStreamToken,
  splitTokenForTyping,
} from "@features/support/utils/chat";

describe("splitTokenForTyping", () => {
  it("splits text into equal chunks up to max chars", () => {
    expect(splitTokenForTyping("abcdefgh", 3)).toEqual(["abc", "def", "gh"]);
  });

  it("guards against non-positive cap with minimum chunk size of one", () => {
    expect(splitTokenForTyping("abc", 0)).toEqual(["a", "b", "c"]);
  });
});

describe("sanitizeStreamToken", () => {
  it("removes description fragments, markdown bold, and excessive newlines", () => {
    const raw = '**{"description":"ignore me","message":"Hello"}\n\n\n\nnext**';
    expect(sanitizeStreamToken(raw)).toBe('{"message":"Hello"}\n\nnext');
  });
});

describe("displayTextFromConversationContent", () => {
  it("returns parsed message for bot JSON payloads", () => {
    const raw = '{"message":"Final answer","description":"hidden"}';
    expect(displayTextFromConversationContent(raw, true)).toBe("Final answer");
  });

  it("returns raw text for invalid JSON payloads", () => {
    expect(displayTextFromConversationContent("{bad-json", true)).toBe("{bad-json");
  });
});

describe("getFinalMessageFromPayload", () => {
  it("supports nested message object", () => {
    expect(
      getFinalMessageFromPayload({
        message: { message: "Nested content" },
      }),
    ).toBe("Nested content");
  });
});
