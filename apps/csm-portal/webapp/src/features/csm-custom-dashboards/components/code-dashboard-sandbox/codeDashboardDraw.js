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

// Draws the page a code dashboard's worker sent (codeDashboardWorker.js) into the
// sandboxed frame (loaded as text, see codeDashboardRunner.ts). The tree comes
// from generated code, so only layout, text, table and SVG shape elements are
// drawn, with presentation attributes only: nothing that runs script, loads a
// URL or navigates (script, a, img, iframe, form, meta, link, use, image,
// animate, on* handlers, href, src) gets into the frame. Anything else is
// left out with its content. Markup the code set through innerHTML is parsed
// inert (a <template>) and goes through the same lists; CSS that could load a
// URL is left out; and a page over the size limits is not drawn past them.

function drawCodeDashboard(nodes, root) {
  const SVG_NS = "http://www.w3.org/2000/svg";
  const HTML_TAGS = new Set([
    "div", "span", "p", "h1", "h2", "h3", "h4", "h5", "h6", "section", "header", "footer", "article", "aside",
    "strong", "b", "em", "i", "u", "s", "small", "sub", "sup", "br", "hr", "code", "pre", "blockquote", "mark",
    "abbr", "time", "ul", "ol", "li", "dl", "dt", "dd", "table", "caption", "thead", "tbody", "tfoot", "tr", "th",
    "td", "colgroup", "col", "figure", "figcaption", "details", "summary", "progress", "meter", "label", "style",
  ]);
  const SVG_TAGS = new Set([
    "svg", "g", "path", "rect", "circle", "ellipse", "line", "polyline", "polygon", "text", "tspan", "title",
    "desc", "defs", "linearGradient", "radialGradient", "stop", "clipPath",
  ]);
  const HTML_ATTRS = new Set([
    "class", "style", "title", "id", "colspan", "rowspan", "scope", "dir", "lang", "role", "width", "height",
    "value", "min", "max", "low", "high", "optimum", "open", "datetime", "span", "align",
  ]);
  const SVG_ATTRS = new Set([
    "class", "style", "id", "viewBox", "width", "height", "x", "y", "x1", "x2", "y1", "y2", "cx", "cy", "r", "rx",
    "ry", "dx", "dy", "d", "points", "fill", "fill-opacity", "fill-rule", "stroke", "stroke-width",
    "stroke-opacity", "stroke-dasharray", "stroke-linecap", "stroke-linejoin", "opacity", "transform",
    "text-anchor", "dominant-baseline", "font-size", "font-weight", "font-family", "preserveAspectRatio",
    "offset", "stop-color", "stop-opacity", "gradientUnits", "gradientTransform", "clip-path", "clipPathUnits",
    "role", "vector-effect", "shape-rendering",
  ]);
  const allowedAttr = (svg, name) =>
    (svg ? SVG_ATTRS : HTML_ATTRS).has(name) || /^aria-[a-z]+$/.test(name) || /^data-[a-z0-9-]+$/.test(name);
  const MAX_NODES = 20000;
  const MAX_DEPTH = 64;
  const MAX_MARKUP = 500000;
  let drawn = 0;

  // CSS that could load a URL: url(), image-set(), @import, and escapes, which
  // can spell any of them. A local reference such as fill="url(#fade)" is fine.
  const LOADS = /url\s*\(|image-set\s*\(|@import|\\/i;
  const LOCAL_REF = /^\s*url\(\s*#[\w-]+\s*\)\s*$/;
  const safeValue = (value) => !LOADS.test(value) || LOCAL_REF.test(value);
  const safeStyle = (css) =>
    css
      .split(";")
      .filter((d) => d.trim() !== "" && !LOADS.test(d))
      .join(";");

  /** innerHTML markup, parsed inert, as tree nodes. */
  function fromMarkup(html) {
    if (html.length > MAX_MARKUP) throw new Error("the dashboard's innerHTML markup is too large to draw");
    const t = document.createElement("template");
    t.innerHTML = html;
    const toTree = (n) => {
      if (n.nodeType === 3) return n.data;
      if (n.nodeType !== 1) return null;
      const ns = n.namespaceURI === SVG_NS ? "svg" : n.namespaceURI === "http://www.w3.org/1999/xhtml" ? "html" : "";
      return {
        tag: n.localName,
        ns,
        attrs: [...n.attributes].map((a) => [a.name, a.value]),
        children: [...n.childNodes].map(toTree),
      };
    };
    return [...t.content.childNodes].map(toTree);
  }

  function draw(node, parent, depth) {
    if (++drawn > MAX_NODES) throw new Error("the dashboard is too large to draw (over " + MAX_NODES + " elements)");
    if (depth > MAX_DEPTH) throw new Error("the dashboard is nested too deeply to draw");
    if (typeof node === "string") {
      parent.appendChild(document.createTextNode(node));
      return;
    }
    if (node && typeof node === "object" && typeof node.html === "string") {
      for (const n of fromMarkup(node.html)) draw(n, parent, depth);
      return;
    }
    if (!node || typeof node !== "object" || typeof node.tag !== "string" || !Array.isArray(node.children)) return;
    const svg = node.ns === "svg";
    if (!(svg ? SVG_TAGS : HTML_TAGS).has(node.tag) || (!svg && node.ns !== "html")) return;
    const el = svg ? document.createElementNS(SVG_NS, node.tag) : document.createElement(node.tag);
    for (const pair of Array.isArray(node.attrs) ? node.attrs : []) {
      const [name, value] = Array.isArray(pair) ? pair : [];
      if (typeof name !== "string" || typeof value !== "string" || !allowedAttr(svg, name)) continue;
      const safe = name === "style" ? safeStyle(value) : safeValue(value) ? value : "";
      if (safe === "") continue;
      try {
        el.setAttribute(name, safe);
      } catch {
        // not a valid attribute name for this element
      }
    }
    if (node.tag === "style") {
      // A stylesheet is drawn whole or not at all.
      const css = node.children.filter((c) => typeof c === "string").join("");
      if (LOADS.test(css)) return;
      el.textContent = css;
    } else {
      for (const child of node.children) draw(child, el, depth + 1);
    }
    parent.appendChild(el);
  }

  for (const node of Array.isArray(nodes) ? nodes : []) draw(node, root, 0);
}
