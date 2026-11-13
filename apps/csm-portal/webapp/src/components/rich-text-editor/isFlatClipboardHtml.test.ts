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
import { isFlatClipboardHtml } from "./isFlatClipboardHtml";

describe("isFlatClipboardHtml", () => {
  it("treats empty or blank HTML as flat", () => {
    expect(isFlatClipboardHtml("")).toBe(true);
    expect(isFlatClipboardHtml("  \n")).toBe(true);
  });

  it("treats a VS Code-style div/span/br snippet as flat", () => {
    const html =
      "<meta charset='utf-8'><div style=\"color:#ccc;font-family:Menlo\"><div><span style=\"color:#569cd6\"># Title</span></div><div><br></div><div><span>- item</span></div></div>";
    expect(isFlatClipboardHtml(html)).toBe(true);
  });

  it("treats pre/code as flat", () => {
    expect(isFlatClipboardHtml("<pre><code>a **b**</code></pre>")).toBe(true);
  });

  it.each([
    ["a Word-style ordered list", "<ol><li>a</li><li>b</li></ol>"],
    ["a ChatGPT-style rendered list with bold", "<ul><li><strong>x</strong> y</li></ul>"],
    ["a Gmail/Outlook quote", "<blockquote>quoted</blockquote>"],
    ["a table", "<table><tr><td>a</td></tr></table>"],
    ["a heading", "<h2>Title</h2>"],
    ["a link", '<p><a href="https://x.com">x</a></p>'],
    ["bold", "<p><b>x</b></p>"],
    ["italic", "<p><em>x</em></p>"],
    ["an image", '<img src="https://x.com/a.png">'],
  ])("treats %s as rendered (not flat)", (_name, html) => {
    expect(isFlatClipboardHtml(html)).toBe(false);
  });
});
