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
import { checkDashboardSource, FORBIDDEN_NAMES } from "./checkDashboardSource";

const refused = (name: string) => `the code uses \`${name}\`, which dashboards may not use`;

describe("checkDashboardSource", () => {
  it("lets a normal dashboard run", () => {
    const source = `
      const STYLE = \`.d{color:#1d2330}\`; // the stylesheet
      function el(tag, cls, text) { const n = document.createElement(tag); if (cls) n.className = cls; return n; }
      async function render(portal, root) {
        const [open, me] = await Promise.all([portal.countCases({ groupBy: "state" }), portal.currentUser()]);
        const parent = el("div", "d");
        const pct = open.total / (me.total || 1) * 100, half = pct / 2;
        parent.textContent = \`\${open.total} open (\${Math.round(pct)}%) for \${me.name.replace(/_/g, " ")}\`;
        root.appendChild(parent);
      }`;
    expect(checkDashboardSource(source)).toBeNull();
  });

  it("refuses every forbidden name used as a name or a property", () => {
    for (const name of FORBIDDEN_NAMES) {
      expect(checkDashboardSource(`function render(portal, root) { ${name}; }`)).toBe(refused(name));
      expect(checkDashboardSource(`function render(portal, root) { root.x.${name}; }`)).toBe(refused(name));
    }
    expect(checkDashboardSource('function render() { const F = (() => {}).constructor; F("return 1")(); }')).toBe(
      refused("constructor"),
    );
    expect(checkDashboardSource('async function render() { await import("https://example.com/x.js"); }')).toBe(
      refused("import"),
    );
  });

  it("ignores the names in strings, comments and regular expressions", () => {
    const source = `
      // location = "x"; fetch()
      /* self.postMessage(1) */
      const a = "location", b = 'fetch("x")', c = \`self\`, r = /constructor|location/g;
      function render(portal, root) { return typeof /window/; }`;
    expect(checkDashboardSource(source)).toBeNull();
  });

  it("checks the code inside a template literal's ${...}, nested ones too", () => {
    expect(checkDashboardSource("const a = `x ${location.href} y`;")).toBe(refused("location"));
    expect(checkDashboardSource("const a = `x ${ { k: `in ${ fetch } side` }.k } y`;")).toBe(refused("fetch"));
    expect(checkDashboardSource("const a = `x ${ { k: 1 }.k } y ${ 2 } location`;")).toBeNull();
    expect(checkDashboardSource("const a = `x ${ { k: 1 }.k } y ${ fetch } z`;")).toBe(refused("fetch"));
  });

  it("reads an unclear slash as a division, so the text after it is checked", () => {
    expect(checkDashboardSource("const a = b / location / c;")).toBe(refused("location"));
    expect(checkDashboardSource("const a = (b) / location / c;")).toBe(refused("location"));
    expect(checkDashboardSource("let b = 1; const a = b++ / location / 2;")).toBe(refused("location"));
    expect(checkDashboardSource("const a = {} / location / 2;")).toBe(refused("location"));
  });

  it("refuses a backslash outside a string, which could spell a name", () => {
    expect(checkDashboardSource("function render() { \\u0066etch('x'); }")).toBe(
      "the code has a backslash outside a string, which dashboards may not use",
    );
    expect(checkDashboardSource("const a = 'it\\'s', b = /a\\/b/;")).toBeNull();
  });

  it("does not refuse names that only contain a forbidden one", () => {
    expect(checkDashboardSource("const selfServe = 1, locations = 2, fetched = 3, myWindow = 4;")).toBeNull();
  });
});
