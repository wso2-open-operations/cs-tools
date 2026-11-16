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

import drawScript from "./codeDashboardDraw.js?raw";
import workerScript from "./codeDashboardWorker.js?raw";

/** How long a dashboard's code may run before its worker is stopped. */
export const RENDER_LIMIT_MS = 60_000;

/**
 * The Content-Security-Policy of the frame's document, which its worker
 * inherits. Nothing may leave the frame: no fetch, XHR, WebSocket, images
 * from a URL, fonts, frames or forms (all fall back to `default-src 'none'`).
 * Workers only from `blob:`, which is how the runner starts the one that runs
 * the code.
 */
export const FRAME_CSP =
  "default-src 'none'; script-src 'unsafe-inline' 'unsafe-eval'; worker-src blob:; style-src 'unsafe-inline'; img-src data:; base-uri 'none'; form-action 'none'";

/** A script's text as a JS string literal that cannot end the inline `<script>`. */
const literal = (text: string): string => JSON.stringify(text).replace(/</g, "\\u003c");

/**
 * The document loaded into the sandboxed iframe. It runs no generated code
 * itself: on `run` it starts a Web Worker (codeDashboardWorker.js) with the
 * source, passes the worker's `portal.*` calls to the page and the answers
 * back, and draws the finished page (codeDashboardDraw.js) into `#root`. The
 * worker is stopped when the page is drawn, the code fails, or after
 * {@link RENDER_LIMIT_MS}.
 *
 * Messages with the page: in {type: "run", source}, {type: "result", ...};
 * out {type: "ready"}, {type: "call", id, fn, args}, {type: "error", message}.
 */
export const FRAME_DOCUMENT = `<!doctype html><html><head><meta charset="utf-8">
<meta http-equiv="Content-Security-Policy" content="${FRAME_CSP}">
<style>body{font:14px/1.45 system-ui,sans-serif;margin:12px;color:#1d2330;background:#fff}.loading{color:#8a93a3}</style></head>
<body><div id="root"><p class="loading">Loading the dashboard...</p></div><script>
const drawCodeDashboard = (() => { ${drawScript}
return drawCodeDashboard; })();
const WORKER_SCRIPT = ${literal(workerScript)};
const root = document.getElementById("root");
let worker = null, started = false, timer = 0;
function finish(tree, message) {
  clearTimeout(timer);
  if (worker) worker.terminate();
  worker = null;
  root.replaceChildren();
  try { drawCodeDashboard(tree, root); } catch (err) { message = message || String((err && err.message) || err); }
  if (message) parent.postMessage({ type: "error", message }, "*");
}
addEventListener("message", (e) => {
  if (e.source !== parent) return;
  const m = e.data || {};
  if (m.type === "result") {
    if (worker) worker.postMessage({ type: "result", id: m.id, value: m.value, error: m.error });
  } else if (m.type === "run" && !started) {
    started = true;
    worker = new Worker(URL.createObjectURL(new Blob([WORKER_SCRIPT], { type: "text/javascript" })));
    worker.onmessage = (w) => {
      const d = w.data || {};
      if (d.type === "call") parent.postMessage({ type: "call", id: d.id, fn: String(d.fn), args: d.args }, "*");
      else if (d.type === "done") finish(d.tree, null);
      else if (d.type === "error") finish(d.tree, String(d.message));
    };
    worker.onerror = (w) => { w.preventDefault(); finish([], w.message || "the dashboard code could not start"); };
    timer = setTimeout(() => finish([], "the dashboard took longer than ${RENDER_LIMIT_MS / 1000} seconds"), ${RENDER_LIMIT_MS});
    worker.postMessage({ type: "run", source: String(m.source) });
  }
});
parent.postMessage({ type: "ready" }, "*");
</script></body></html>`;
