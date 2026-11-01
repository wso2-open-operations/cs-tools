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
import { isExternalHref, sanitizeArticleHtml } from "@utils/sanitizeHtml";

const render = (html: string): HTMLElement => {
  const el = document.createElement("div");
  el.innerHTML = sanitizeArticleHtml(html);
  return el;
};

describe("sanitizeArticleHtml", () => {
  it("removes scripts, event handlers and javascript: URLs", () => {
    const el = render(
      '<p onclick="x()">hi</p><script>alert(1)</script><a href="javascript:alert(1)">l</a>',
    );
    expect(el.querySelector("script")).toBeNull();
    expect(el.querySelector("p")?.hasAttribute("onclick")).toBe(false);
    expect(el.querySelector("a")?.hasAttribute("href")).toBe(false);
  });

  it.each(["style", "form", "input", "button", "textarea", "select", "iframe", "object", "embed"])(
    "forbids <%s>",
    (tag) => {
      const el = render(`<p>keep</p><${tag}></${tag}>`);
      expect(el.querySelector(tag)).toBeNull();
      expect(el.textContent).toContain("keep");
    },
  );

  it("strips inline style attributes", () => {
    const el = render('<p style="position:fixed;inset:0">x</p>');
    expect(el.querySelector("p")?.hasAttribute("style")).toBe(false);
  });

  it("opens external links in a new tab with noopener noreferrer", () => {
    const a = render('<a href="https://example.com/doc" target="_self">x</a>').querySelector("a")!;
    expect(a.getAttribute("target")).toBe("_blank");
    expect(a.getAttribute("rel")).toBe("noopener noreferrer");
  });

  it("leaves internal, fragment and mailto links in the same tab", () => {
    const el = render(
      '<a href="/articles/abc">a</a><a href="#top">b</a><a href="mailto:a@example.com">c</a>',
    );
    el.querySelectorAll("a").forEach((a) => {
      expect(a.hasAttribute("target")).toBe(false);
    });
  });

  it("keeps ordinary content such as headings, code and tables", () => {
    const el = render("<h2>T</h2><pre><code>x</code></pre><table><tr><td>1</td></tr></table>");
    expect(el.querySelector("h2")).not.toBeNull();
    expect(el.querySelector("pre code")).not.toBeNull();
    expect(el.querySelector("td")).not.toBeNull();
  });
});

describe("isExternalHref", () => {
  const origin = "https://kb.example.com";
  it("classifies hrefs", () => {
    expect(isExternalHref("https://other.example.com/x", origin)).toBe(true);
    expect(isExternalHref("//other.example.com/x", origin)).toBe(true);
    expect(isExternalHref("https://kb.example.com/x", origin)).toBe(false);
    expect(isExternalHref("/x", origin)).toBe(false);
    expect(isExternalHref("#x", origin)).toBe(false);
    expect(isExternalHref("mailto:a@example.com", origin)).toBe(false);
    expect(isExternalHref(null, origin)).toBe(false);
  });
});

describe("heading demotion", () => {
  it("moves every heading down one level when the body has an h1", () => {
    const el = render('<h1 id="t" class="c">One</h1><h2>Two</h2><h6>Six</h6><p>x</p>');
    expect(el.querySelector("h1")).toBeNull();
    expect(el.querySelector("h2")?.textContent).toBe("One");
    expect(el.querySelector("h2")?.getAttribute("id")).toBe("t");
    expect(el.querySelector("h3")?.textContent).toBe("Two");
    expect(el.querySelector("h6")?.textContent).toBe("Six");
  });

  it("leaves headings alone when there is no h1", () => {
    const el = render("<h2>Two</h2><h3>Three</h3>");
    expect(el.querySelector("h2")?.textContent).toBe("Two");
    expect(el.querySelector("h3")?.textContent).toBe("Three");
  });

  it("keeps formatting inside a demoted heading", () => {
    const el = render("<h1>A <em>b</em></h1>");
    expect(el.querySelector("h2 em")?.textContent).toBe("b");
  });
});
