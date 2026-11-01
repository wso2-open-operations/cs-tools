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
import { buildArticleOutline, slugify } from "@utils/articleOutline";

describe("slugify", () => {
  it("lower-cases, strips accents and collapses separators", () => {
    expect(slugify("Step 1:  Vérify the URL!")).toBe("step-1-verify-the-url");
  });
  it("falls back when nothing is left", () => {
    expect(slugify("***")).toBe("section");
  });
});

describe("buildArticleOutline", () => {
  it("assigns prefixed ids to h2 and h3 and collects them in order", () => {
    const { html, headings } = buildArticleOutline("<h2>Symptoms</h2><p>x</p><h3>Logs</h3><h4>Deep</h4>");
    expect(headings).toEqual([
      { id: "kb-symptoms", text: "Symptoms", level: 2 },
      { id: "kb-logs", text: "Logs", level: 3 },
    ]);
    expect(html).toContain('<h2 id="kb-symptoms">');
    expect(html).toContain('<h3 id="kb-logs">');
    expect(html).toContain("<h4>Deep</h4>");
  });

  it("keeps an existing id and de-duplicates repeats", () => {
    const { headings } = buildArticleOutline('<h2 id="custom">A</h2><h2>Steps</h2><h2>Steps</h2>');
    expect(headings.map((h) => h.id)).toEqual(["custom", "kb-steps", "kb-steps-2"]);
  });

  it("skips headings with no text", () => {
    expect(buildArticleOutline("<h2> </h2>").headings).toEqual([]);
  });

  it("wraps each code block with a copy button, once", () => {
    const once = buildArticleOutline("<pre><code>ls -la</code></pre>");
    expect(once.html).toBe(
      '<div class="kb-code"><button type="button" class="kb-code-copy" data-kb-copy="" aria-label="Copy code to clipboard">Copy</button><pre><code>ls -la</code></pre></div>',
    );
    expect(buildArticleOutline(once.html).html).toBe(once.html);
  });
});
