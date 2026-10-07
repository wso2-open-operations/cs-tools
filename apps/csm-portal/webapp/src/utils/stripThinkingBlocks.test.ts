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
import { stripThinkingBlocks } from "./stripThinkingBlocks";

describe("stripThinkingBlocks", () => {
  it("removes a leading thinking block and keeps the answer", () => {
    const raw =
      "<thinking> The user asks about Widget 2.\n- Dev: Widget 1\nI should ask.\n</thinking>\n\nI don't see Widget 2 in your environments.";
    expect(stripThinkingBlocks(raw)).toBe(
      "I don't see Widget 2 in your environments.",
    );
  });

  it("removes every block, in any letter case", () => {
    expect(
      stripThinkingBlocks("A<thinking>x</thinking>B<Thinking>y</THINKING>C"),
    ).toBe("ABC");
  });

  it("drops a block that was never closed, and a half-written opening tag", () => {
    expect(stripThinkingBlocks("Done.\n<thinking>cut off mid")).toBe("Done.\n");
    expect(stripThinkingBlocks("Done. <thin")).toBe("Done. ");
  });

  it("keeps the author's indentation, whichever side of a block it is on", () => {
    expect(stripThinkingBlocks("    code\n<thinking>reason</thinking>")).toBe(
      "    code\n",
    );
    expect(stripThinkingBlocks("<thinking>x</thinking>\n\n    code")).toBe(
      "    code",
    );
    expect(stripThinkingBlocks("<thinking>x</thinking>   Answer")).toBe(
      "Answer",
    );
  });

  it("stays linear however many openers there are", () => {
    const manyOpeners = "<thinking>x".repeat(50_000);
    const started = performance.now();
    expect(stripThinkingBlocks(manyOpeners)).toBe("");
    // A rescan per unclosed opener takes several seconds here; a linear scan well
    // under a millisecond. The bound is ~1000x the real cost so load can't flake it.
    expect(performance.now() - started).toBeLessThan(1000);
  });

  it("returns text without a thinking tag untouched", () => {
    const plain = "Set `a < b` and use <b>bold</b> or <thead> markup.";
    expect(stripThinkingBlocks(plain)).toBe(plain);
    expect(stripThinkingBlocks("")).toBe("");
  });
});
