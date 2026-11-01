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
import { browseUrl, articleUrl } from "@utils/browseUrl";
import { normalizeSearchQuery } from "@utils/searchQuery";

describe("normalizeSearchQuery", () => {
  it("trims, strips control characters and caps at 200 code points", () => {
    expect(normalizeSearchQuery("  hello\u0000world\n ")).toBe("hello world");
    expect(normalizeSearchQuery(null)).toBe("");
    expect(Array.from(normalizeSearchQuery("😀".repeat(300)))).toHaveLength(200);
  });
});

describe("url builders", () => {
  it("percent-encodes values", () => {
    expect(browseUrl({ q: "a b&c=d/é", knowledgeBaseId: "kb_1" })).toBe(
      "/knowledge-bases/kb_1?q=a%20b%26c%3Dd%2F%C3%A9",
    );
    expect(browseUrl()).toBe("/knowledge-bases");
    expect(browseUrl({ q: "" })).toBe("/knowledge-bases");
    expect(articleUrl("a/b")).toBe("/knowledge-articles/a%2Fb");
  });
});
