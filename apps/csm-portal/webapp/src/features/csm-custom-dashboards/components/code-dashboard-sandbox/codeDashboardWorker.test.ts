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

import { describe, expect, it, vi } from "vitest";
import workerScript from "./codeDashboardWorker.js?raw";

interface Posted {
  type: string;
  id?: number;
  fn?: string;
  args?: unknown;
  tree?: unknown[];
  message?: string;
}

/**
 * Starts the worker script against a stand-in worker scope (the script only
 * uses `self.postMessage` and `self.addEventListener`) and returns what it posts.
 */
function startWorker() {
  const posted: Posted[] = [];
  let onMessage: (event: { data: unknown }) => void = () => {};
  const scope = {
    postMessage: (m: Posted) => posted.push(m),
    addEventListener: (_type: string, fn: typeof onMessage) => {
      onMessage = fn;
    },
  };
  new Function("self", workerScript)(scope);
  const send = (data: unknown) => onMessage({ data });
  const finished = () => posted.find((m) => m.type === "done" || m.type === "error");
  const run = async (source: string): Promise<Posted> => {
    send({ type: "run", source });
    await vi.waitFor(() => expect(finished()).toBeDefined());
    return finished()!;
  };
  return { posted, send, run, scope };
}

// The catalogue's helpers that use the most of the DOM (section 7, shortened).
const HELPERS = `
function el(tag, cls, text) {
  const n = document.createElement(tag);
  if (cls) n.className = cls;
  if (text !== undefined && text !== null) n.textContent = String(text);
  return n;
}
function table(c, columns, items) {
  const t = el("table", "d-table");
  const head = el("tr");
  for (const col of columns) head.appendChild(el("th", null, col.title));
  t.appendChild(el("thead")).appendChild(head);
  const body = el("tbody");
  for (const item of items) {
    const tr = el("tr");
    for (const col of columns) {
      const td = el("td", col.cls);
      const v = col.value(item);
      if (v instanceof Node) td.appendChild(v); else td.textContent = v === null || v === undefined ? "-" : String(v);
      tr.appendChild(td);
    }
    body.appendChild(tr);
  }
  t.appendChild(body);
  c.appendChild(el("div", "d-scroll")).appendChild(t);
}
`;

describe("codeDashboardWorker", () => {
  it("draws with the catalogue's helpers and sends the page as a tree", async () => {
    const { run } = startWorker();
    const done = await run(`${HELPERS}
      async function render(portal, root) {
        const style = document.createElement("style");
        style.textContent = ".d{color:red}";
        root.appendChild(style);
        const box = el("div", "d-kpi");
        box.style.setProperty("--c", "#c62828");
        box.style.width = 50 + "%";
        root.appendChild(box);
        table(root, [
          { title: "Number", value: (c) => c.number, cls: "d-nowrap" },
          { title: "State", value: (c) => el("span", "d-chip", c.state) },
        ], [{ number: "CS-1", state: "Open" }]);
      }`);

    expect(done).toEqual({
      type: "done",
      tree: [
        { tag: "style", ns: "html", attrs: [], children: [".d{color:red}"] },
        { tag: "div", ns: "html", attrs: [["class", "d-kpi"], ["style", "--c: #c62828; width: 50%"]], children: [] },
        {
          tag: "div",
          ns: "html",
          attrs: [["class", "d-scroll"]],
          children: [
            {
              tag: "table",
              ns: "html",
              attrs: [["class", "d-table"]],
              children: [
                {
                  tag: "thead",
                  ns: "html",
                  attrs: [],
                  children: [
                    {
                      tag: "tr",
                      ns: "html",
                      attrs: [],
                      children: [
                        { tag: "th", ns: "html", attrs: [], children: ["Number"] },
                        { tag: "th", ns: "html", attrs: [], children: ["State"] },
                      ],
                    },
                  ],
                },
                {
                  tag: "tbody",
                  ns: "html",
                  attrs: [],
                  children: [
                    {
                      tag: "tr",
                      ns: "html",
                      attrs: [],
                      children: [
                        { tag: "td", ns: "html", attrs: [["class", "d-nowrap"]], children: ["CS-1"] },
                        {
                          tag: "td",
                          ns: "html",
                          attrs: [],
                          children: [{ tag: "span", ns: "html", attrs: [["class", "d-chip"]], children: ["Open"] }],
                        },
                      ],
                    },
                  ],
                },
              ],
            },
          ],
        },
      ],
    });
  });

  it("draws SVG in the SVG namespace and keeps innerHTML markup for the frame to check", async () => {
    const { run } = startWorker();
    const done = await run(`async function render(portal, root) {
      const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
      svg.setAttribute("viewBox", "0 0 10 10");
      const line = document.createElementNS("http://www.w3.org/2000/svg", "polyline");
      line.setAttribute("points", "0,0 10,10");
      svg.appendChild(line);
      root.appendChild(svg);
      const box = document.createElement("div");
      box.innerHTML = '<svg><rect width="4"/></svg>';
      root.appendChild(box);
    }`);
    expect(done.tree).toEqual([
      {
        tag: "svg",
        ns: "svg",
        attrs: [["viewBox", "0 0 10 10"]],
        children: [{ tag: "polyline", ns: "svg", attrs: [["points", "0,0 10,10"]], children: [] }],
      },
      { tag: "div", ns: "html", attrs: [], children: [{ html: '<svg><rect width="4"/></svg>' }] },
    ]);
  });

  it("passes portal calls out and their answers back", async () => {
    const { posted, send } = startWorker();
    send({
      type: "run",
      source: `async function render(portal, root) {
        const [a, b] = await Promise.all([portal.countCases({ groupBy: "state" }), portal.searchCases({}).catch((e) => e.message)]);
        root.textContent = a.total + " / " + b;
      }`,
    });
    await vi.waitFor(() => expect(posted.filter((m) => m.type === "call")).toHaveLength(2));
    expect(posted).toEqual([
      { type: "call", id: 1, fn: "countCases", args: { groupBy: "state" } },
      { type: "call", id: 2, fn: "searchCases", args: {} },
    ]);
    send({ type: "result", id: 2, error: "searchCases: bad filter" });
    send({ type: "result", id: 1, value: { total: 7 } });
    await vi.waitFor(() => expect(posted.at(-1)?.type).toBe("done"));
    expect(posted.at(-1)).toEqual({ type: "done", tree: ["7 / searchCases: bad filter"] });
  });

  it("waits for a call the code did not await before sending the page", async () => {
    const { posted, send } = startWorker();
    send({
      type: "run",
      source: `function render(portal, root) {
        portal.countCases({}).then((r) => { root.textContent = "Total " + r.total; });
      }`,
    });
    await vi.waitFor(() => expect(posted).toHaveLength(1));
    await new Promise((r) => setTimeout(r, 50));
    expect(posted).toHaveLength(1); // still waiting for the call
    send({ type: "result", id: 1, value: { total: 4 } });
    await vi.waitFor(() => expect(posted.at(-1)).toEqual({ type: "done", tree: ["Total 4"] }));
  });

  it("reports an error with what was drawn before it", async () => {
    const { run } = startWorker();
    const failed = await run(`async function render(portal, root) {
      root.appendChild(document.createTextNode("Header"));
      throw new Error("no data");
    }`);
    expect(failed).toEqual({ type: "error", message: "no data", tree: ["Header"] });
  });

  it("reports code without a render function, and code that does not parse", async () => {
    expect((await startWorker().run("const x = 1;")).message).toBe(
      "the code does not define a render(portal, root) function",
    );
    expect((await startWorker().run("function render( {")).message).toMatch(/Unexpected|Syntax/i);
  });

  it("gives the code no way to the network, the frame or the worker itself", async () => {
    const { run } = startWorker();
    const done = await run(`async function render(portal, root) {
      root.textContent = [typeof self, typeof globalThis, typeof window, typeof location, typeof fetch,
        typeof XMLHttpRequest, typeof WebSocket, typeof importScripts, typeof postMessage, typeof Worker].join(",");
    }`);
    expect(done.tree).toEqual([Array(10).fill("undefined").join(",")]);
  });

  it("removes the network globals from the worker scope for good", () => {
    const { scope } = startWorker();
    for (const name of ["fetch", "XMLHttpRequest", "WebSocket", "importScripts", "postMessage"]) {
      expect((scope as Record<string, unknown>)[name]).toBeUndefined();
      expect(Object.getOwnPropertyDescriptor(scope, name)).toMatchObject({ writable: false, configurable: false });
    }
  });

  it("refuses to put a node inside itself", async () => {
    const { run } = startWorker();
    const failed = await run(`function render(portal, root) {
      const a = document.createElement("div"), b = document.createElement("div");
      a.appendChild(b);
      b.appendChild(a);
    }`);
    expect(failed.message).toBe("a node cannot be put inside itself");
  });

  it("supports the usual element and style properties", async () => {
    const { run } = startWorker();
    const done = await run(`function render(portal, root) {
      const td = document.createElement("TD");
      td.colSpan = 2;
      td.classList.add("a", "b");
      td.classList.remove("a");
      td.style.cssText = "color: red; font-weight: 600";
      td.style.color = "";
      td.setAttribute("title", "Tip");
      td.onclick = () => {};
      td.append("x", document.createElement("br"));
      td.prepend("first ");
      root.append(td);
      root.appendChild(document.createTextNode(td.tagName + " " + td.className + " " + td.textContent + " " + td.innerHTML));
    }`);
    expect(done.tree).toEqual([
      {
        tag: "td",
        ns: "html",
        attrs: [["colspan", "2"], ["class", "b"], ["title", "Tip"], ["style", "font-weight: 600"]],
        children: ["first ", "x", { tag: "br", ns: "html", attrs: [], children: [] }],
      },
      "TD b first x first x<br>",
    ]);
  });
});
