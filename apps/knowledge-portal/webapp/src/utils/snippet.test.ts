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
import { toSnippet } from "@utils/snippet";

describe("toSnippet", () => {
  it("drops style and script text", () => {
    expect(toSnippet("<style>p{color:red}</style><script>alert(1)</script><p>Hello</p>")).toBe("Hello");
  });

  it("decodes entities", () => {
    expect(toSnippet("<p>a &amp; b &lt;c&gt; &quot;d&quot; &#39;e&#39;</p>")).toBe(`a & b <c> "d" 'e'`);
  });

  it("separates adjacent blocks with a space and collapses whitespace", () => {
    expect(toSnippet("<p>one</p><p>two</p><ul><li>three</li><li>four</li></ul>\n\n  five")).toBe(
      "one two three four five",
    );
  });

  it("keeps markdown-looking characters", () => {
    expect(toSnippet("<p>use #1 and (x) [y] *z*</p>")).toBe("use #1 and (x) [y] *z*");
  });

  it("truncates with an ellipsis", () => {
    const out = toSnippet(`<p>${"a".repeat(50)}</p>`, 10);
    expect(out).toBe(`${"a".repeat(10)}...`);
  });

  it("does not split a surrogate pair when truncating", () => {
    const out = toSnippet("<p>" + "😀".repeat(20) + "</p>", 5);
    expect(out).toBe(`${"😀".repeat(5)}...`);
  });

  it("returns an empty string for an empty body", () => {
    expect(toSnippet("")).toBe("");
  });
});
