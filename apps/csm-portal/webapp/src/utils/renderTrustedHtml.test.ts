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
import { linkifyBareUrlsInDom } from "@features/csm-cases/utils/commentContent";
import { replaceCallRequestLinksInDom } from "@features/csm-cases/utils/callRequestLinks";
import { replaceSnLinksInDom } from "@features/csm-cases/utils/snLinkRegistry";
import { renderTrustedHtml, transformHtml } from "./renderTrustedHtml";

const TRANSFORMS = [
  replaceCallRequestLinksInDom,
  replaceSnLinksInDom,
  linkifyBareUrlsInDom,
] as const;

function parse(html: string): HTMLElement {
  const doc = new DOMParser().parseFromString(`<body>${html}</body>`, "text/html");
  return doc.body;
}

function hasEventHandlerAttribute(root: HTMLElement): boolean {
  return [...root.querySelectorAll("*")].some((el) =>
    [...el.attributes].some((attr) => /^on/i.test(attr.name)),
  );
}

describe("renderTrustedHtml", () => {
  it.each([
    ["src", '<img src="https://evil.example/x/onerror=alert(1)//">'],
    ["alt", '<img src="x" alt="https://a.test//onerror=alert(1)//">'],
    ["title", '<p title="https://a.test//onmouseover=alert(1)//">hi</p>'],
  ])("never introduces an event-handler attribute from a URL inside %s", (_name, payload) => {
    const out = renderTrustedHtml(payload, TRANSFORMS);
    expect(hasEventHandlerAttribute(parse(out))).toBe(false);
  });

  it("keeps a benign remote image src exactly as written", () => {
    const out = renderTrustedHtml('<img src="https://cdn.example/sig.png">', TRANSFORMS);
    const img = parse(out).querySelector("img");
    expect(img?.getAttribute("src")).toBe("https://cdn.example/sig.png");
    expect(parse(out).querySelector("a")).toBeNull();
  });

  it("does not rewrite a URL held in an attribute value while linkifying text", () => {
    const out = renderTrustedHtml(
      '<p title="https://attr.example/a">see https://text.example/b</p>',
      TRANSFORMS,
    );
    const body = parse(out);
    expect(body.querySelector("p")?.getAttribute("title")).toBe("https://attr.example/a");
    const anchors = body.querySelectorAll("a");
    expect(anchors).toHaveLength(1);
    expect(anchors[0].getAttribute("href")).toBe("https://text.example/b");
    expect(anchors[0].getAttribute("target")).toBe("_blank");
    expect(anchors[0].getAttribute("rel")).toBe("noopener noreferrer");
  });

  it("sanitises last: markup injected by a transform is still stripped", () => {
    const evil = (body: HTMLElement) => {
      const img = body.ownerDocument.createElement("img");
      img.setAttribute("src", "x");
      img.setAttribute("onerror", "alert(1)");
      body.append(img);
    };
    const out = renderTrustedHtml("<p>hi</p>", [evil]);
    expect(out).not.toContain("onerror");
  });

  it("strips script content and javascript: hrefs", () => {
    const out = renderTrustedHtml(
      '<p>hi</p><script>alert(1)</script><a href="javascript:alert(1)">x</a>',
      TRANSFORMS,
    );
    expect(out).not.toContain("<script");
    expect(out).not.toContain("javascript:");
  });

  it("does not linkify text inside existing anchors, code, or pre", () => {
    const out = renderTrustedHtml(
      '<a href="https://a.example">https://a.example</a><code>https://c.example</code><pre>https://p.example</pre>',
      TRANSFORMS,
    );
    expect(parse(out).querySelectorAll("a")).toHaveLength(1);
  });

  it("escapes URL text so it can only ever be text content of the anchor", () => {
    const out = renderTrustedHtml("go https://x.example/?a=1&b=2", TRANSFORMS);
    const anchor = parse(out).querySelector("a");
    expect(anchor?.getAttribute("href")).toBe("https://x.example/?a=1&b=2");
    expect(anchor?.textContent).toBe("https://x.example/?a=1&b=2");
  });

  it("keeps the in-app marker attributes through the final sanitise", () => {
    const out = renderTrustedHtml(
      "see https://host.example/sn_customerservice_customer_call.do?sys_id=7a43e2d43b2a4b5091404c6aa5e45a41",
      TRANSFORMS,
    );
    const marker = parse(out).querySelector("[data-call-request-sysid]");
    expect(marker?.getAttribute("data-call-request-sysid")).toBe(
      "7a43e2d4-3b2a-4b50-9140-4c6aa5e45a41",
    );
    expect(marker?.getAttribute("role")).toBe("button");
    expect(marker?.getAttribute("tabindex")).toBe("0");
  });

  it("returns empty input unchanged", () => {
    expect(renderTrustedHtml("", TRANSFORMS)).toBe("");
    expect(transformHtml("", TRANSFORMS)).toBe("");
  });
});
