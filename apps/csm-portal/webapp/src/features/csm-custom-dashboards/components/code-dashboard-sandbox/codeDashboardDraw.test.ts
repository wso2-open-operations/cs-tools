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
import drawScript from "./codeDashboardDraw.js?raw";

const drawCodeDashboard = new Function(`${drawScript}\nreturn drawCodeDashboard;`)() as (
  nodes: unknown,
  root: Element,
) => void;

/** Draws the tree into a fresh element and returns it. */
function draw(tree: unknown): HTMLElement {
  const root = document.createElement("div");
  drawCodeDashboard(tree, root);
  return root;
}

const div = (attrs: string[][], children: unknown[] = []) => ({ tag: "div", ns: "html", attrs, children });

describe("drawCodeDashboard", () => {
  it("draws layout, text, styles and SVG shapes", () => {
    const root = draw([
      { tag: "style", ns: "html", attrs: [], children: [".d{color:red}"] },
      div([["class", "d-kpi"], ["style", "--c: #c62828"], ["title", "Open"]], ["7 <b>open</b>"]),
      {
        tag: "svg",
        ns: "svg",
        attrs: [["viewBox", "0 0 10 10"]],
        children: [{ tag: "polyline", ns: "svg", attrs: [["points", "0,0 10,10"], ["stroke", "#1565c0"]], children: [] }],
      },
    ]);
    expect(root.innerHTML).toBe(
      '<style>.d{color:red}</style><div class="d-kpi" style="--c: #c62828" title="Open">7 &lt;b&gt;open&lt;/b&gt;</div>' +
        '<svg viewBox="0 0 10 10"><polyline points="0,0 10,10" stroke="#1565c0"></polyline></svg>',
    );
    expect(root.querySelector("polyline")?.namespaceURI).toBe("http://www.w3.org/2000/svg");
  });

  it("leaves out elements that run script, load a URL or navigate, with their content", () => {
    const root = draw(
      ["script", "a", "img", "iframe", "form", "meta", "link", "base", "object", "embed", "input", "button"].map(
        (tag) => ({ tag, ns: "html", attrs: [["href", "https://example.com"]], children: ["x"] }),
      ),
    );
    expect(root.innerHTML).toBe("");
    const svg = draw(
      ["a", "use", "image", "animate", "set", "foreignObject", "script", "style"].map((tag) => ({
        tag,
        ns: "svg",
        attrs: [],
        children: ["x"],
      })),
    );
    expect(svg.innerHTML).toBe("");
  });

  it("keeps only presentation attributes", () => {
    const root = draw([
      div([
        ["class", "d"],
        ["onclick", "location='https://example.com'"],
        ["ONMOUSEOVER", "x()"],
        ["href", "https://example.com"],
        ["src", "https://example.com"],
        ["srcdoc", "<script></script>"],
        ["aria-label", "Cases"],
        ["data-id", "7"],
      ]),
      { tag: "rect", ns: "svg", attrs: [], children: [] }, // SVG shape outside an svg: still SVG
      {
        tag: "svg",
        ns: "svg",
        attrs: [["href", "#x"], ["xlink:href", "#x"], ["onload", "x()"], ["width", "4"]],
        children: [],
      },
    ]);
    expect(root.innerHTML).toBe(
      '<div class="d" aria-label="Cases" data-id="7"></div><rect></rect><svg width="4"></svg>',
    );
  });

  it("leaves out anything that is not a node it knows", () => {
    const root = draw([
      { tag: "div", ns: "math", attrs: [], children: [] },
      { tag: "div", attrs: [], children: [] },
      { tag: "div", ns: "html", attrs: [["class"], ["title", 7]], children: [null, 5, "ok"] },
      null,
      42,
    ]);
    expect(root.innerHTML).toBe("<div>ok</div>");
    expect(draw("not a list").innerHTML).toBe("");
  });

  it("draws innerHTML markup through the same lists, without running or loading anything", () => {
    const root = draw([
      div(
        [],
        [
          {
            html:
              '<b class="x" onclick="go()">bold</b><img src="https://example.com/i" onerror="location=1">' +
              '<script>location=1</script><a href="https://example.com">link</a><!-- note -->' +
              '<svg viewBox="0 0 4 4"><linearGradient id="g"></linearGradient>' +
              '<rect width="4" fill="url(#g)"></rect><use href="#g"></use><foreignObject><p>x</p></foreignObject></svg>',
          },
        ],
      ),
    ]);
    expect(root.innerHTML).toBe(
      '<div><b class="x">bold</b><svg viewBox="0 0 4 4"><linearGradient id="g"></linearGradient>' +
        '<rect width="4" fill="url(#g)"></rect></svg></div>',
    );
    expect(root.querySelector("linearGradient")?.namespaceURI).toBe("http://www.w3.org/2000/svg");
  });

  it("leaves out CSS that could load a URL", () => {
    const root = draw([
      div([["style", "color: red; background: url(https://example.com/x); --c: #c62828; b: \\75 rl(x)"]]),
      div([["style", "background-image: image-set('https://example.com/x' 1x)"]]),
      { tag: "rect", ns: "svg", attrs: [["fill", "url(https://example.com/x)"], ["stroke", "url( #line )"]], children: [] },
      { tag: "style", ns: "html", attrs: [], children: [".d{color:red}"] },
      { tag: "style", ns: "html", attrs: [], children: ["@import 'https://example.com/x.css';"] },
      { tag: "style", ns: "html", attrs: [], children: [".d{background:URL(https://example.com/x)}"] },
      { tag: "style", ns: "html", attrs: [], children: [".d{background:\\75 rl(x)}"] },
    ]);
    expect(root.innerHTML).toBe(
      '<div style="color: red; --c: #c62828"></div><div></div><rect stroke="url( #line )"></rect>' +
        "<style>.d{color:red}</style>",
    );
  });

  it("stops at the size limits with an error", () => {
    const many = Array.from({ length: 20001 }, () => "x");
    expect(() => draw(many)).toThrow("the dashboard is too large to draw (over 20000 elements)");
    let deep: unknown = "x";
    for (let i = 0; i < 70; i++) deep = div([], [deep]);
    expect(() => draw([deep])).toThrow("the dashboard is nested too deeply to draw");
    expect(() => draw([{ html: "x".repeat(500001) }])).toThrow("the dashboard's innerHTML markup is too large to draw");
    expect(draw(Array.from({ length: 20000 }, () => "x")).textContent).toHaveLength(20000);
  });

  it("can be put inline in the frame's script", () => {
    expect(drawScript).not.toMatch(/<\/|<!--/);
  });
});
