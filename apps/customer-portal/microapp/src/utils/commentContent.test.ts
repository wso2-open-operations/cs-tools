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
import { commentContentToHtml } from "./commentContent";

describe("commentContentToHtml", () => {
  it("turns plain text into escaped HTML with line breaks", () => {
    expect(commentContentToHtml("Hello,\nif a < b & c > d")).toBe("Hello,<br>if a &lt; b &amp; c &gt; d");
    expect(commentContentToHtml("a\r\nb")).toBe("a<br>b");
  });

  it("keeps HTML as it is", () => {
    expect(commentContentToHtml("<p>Hi <b>there</b></p>")).toBe("<p>Hi <b>there</b></p>");
  });

  it("shows a [code] block as code and the text around it as plain text", () => {
    expect(commentContentToHtml("Run:\n[code]a < b\nc[/code]\nthen retry")).toBe(
      "Run:<br><code>a &lt; b\nc</code><br>then retry",
    );
    expect(commentContentToHtml("x [\\code]y[\\/code]")).toBe("x <code>y</code>");
  });

  it("drops the system 'Customer comment added' label", () => {
    expect(commentContentToHtml("<p>Customer comment added</p><p>Real text</p>")).toBe("<p>Real text</p>");
    expect(commentContentToHtml("Customer comment added")).toBe("");
  });

  it("copes with nothing", () => {
    expect(commentContentToHtml(undefined)).toBe("");
    expect(commentContentToHtml(null)).toBe("");
    expect(commentContentToHtml("   ")).toBe("");
  });
});
