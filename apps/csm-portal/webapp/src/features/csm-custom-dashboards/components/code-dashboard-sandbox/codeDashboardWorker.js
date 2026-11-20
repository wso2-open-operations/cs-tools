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

// The script of the Web Worker a code dashboard's `render(portal, root)` runs in
// (loaded as text, see codeDashboardRunner.ts). A worker has no page to navigate
// and no DOM, and it inherits the frame's Content-Security-Policy, so it cannot
// send the viewer's data anywhere. The code draws into a small stand-in DOM
// (`document`, `Node`, ...); when `render` is done the page goes to the frame as
// a tree, and the frame draws it through an allow-list (codeDashboardDraw.js).
//
// Messages: in {type: "run", source}, {type: "result", id, value | error};
// out {type: "call", id, fn, args}, {type: "done", tree}, {type: "error", message, tree}.
"use strict";
(() => {
  const scope = self;
  const post = scope.postMessage.bind(scope);
  const listen = scope.addEventListener.bind(scope);

  // Nothing the code could send data with is left on the worker itself. The
  // CSP already blocks every request; this is a second wall.
  const REMOVED = [
    "fetch", "XMLHttpRequest", "WebSocket", "EventSource", "WebTransport", "importScripts",
    "Worker", "SharedWorker", "BroadcastChannel", "postMessage",
  ];
  for (const name of REMOVED) {
    for (let o = Object.getPrototypeOf(scope); o; o = Object.getPrototypeOf(o)) {
      try { delete o[name]; } catch { /* not configurable */ }
    }
    try { Object.defineProperty(scope, name, { value: undefined, writable: false, configurable: false }); } catch { /* already locked */ }
  }

  const HTML_NS = "http://www.w3.org/1999/xhtml";
  const SVG_NS = "http://www.w3.org/2000/svg";

  class Node {
    constructor() { this.childNodes = []; this.parentNode = null; }
    get firstChild() { return this.childNodes[0] ?? null; }
    get lastChild() { return this.childNodes[this.childNodes.length - 1] ?? null; }
    get children() { return this.childNodes.filter((n) => n instanceof Element); }
    get parentElement() { return this.parentNode instanceof Element ? this.parentNode : null; }
    contains(node) {
      for (let n = node; n; n = n.parentNode) if (n === this) return true;
      return false;
    }
    insertBefore(child, ref) {
      if (!(child instanceof Node)) throw new TypeError("insertBefore needs a node");
      if (child instanceof DocumentFragment) {
        for (const c of [...child.childNodes]) this.insertBefore(c, ref);
        return child;
      }
      if (child.contains(this)) throw new Error("a node cannot be put inside itself");
      child.remove();
      const at = ref ? this.childNodes.indexOf(ref) : -1;
      if (ref && at < 0) throw new Error("insertBefore: the reference node is not a child");
      this.childNodes.splice(at < 0 ? this.childNodes.length : at, 0, child);
      child.parentNode = this;
      return child;
    }
    appendChild(child) { return this.insertBefore(child, null); }
    removeChild(child) {
      if (child?.parentNode !== this) throw new Error("removeChild: not a child");
      child.remove();
      return child;
    }
    append(...nodes) { for (const n of nodes) this.appendChild(n instanceof Node ? n : new Text(n)); }
    prepend(...nodes) { const first = this.firstChild; for (const n of nodes) this.insertBefore(n instanceof Node ? n : new Text(n), first); }
    replaceChildren(...nodes) { for (const c of [...this.childNodes]) c.remove(); this.append(...nodes); }
    remove() {
      const p = this.parentNode;
      if (!p) return;
      p.childNodes.splice(p.childNodes.indexOf(this), 1);
      this.parentNode = null;
    }
    get textContent() { return this.childNodes.map((c) => c.textContent).join(""); }
    set textContent(value) {
      for (const c of [...this.childNodes]) c.remove();
      if (value !== null && value !== undefined && String(value) !== "") this.appendChild(new Text(value));
    }
    // Dashboards draw once and are not interactive: listeners never fire.
    addEventListener() {}
    removeEventListener() {}
  }

  class Text extends Node {
    constructor(data) { super(); this.data = String(data); }
    get nodeType() { return 3; }
    get textContent() { return this.data; }
    set textContent(value) { this.data = value === null || value === undefined ? "" : String(value); }
    get nodeValue() { return this.data; }
    set nodeValue(value) { this.textContent = value; }
    toTree() { return this.data; }
  }

  class DocumentFragment extends Node {
    get nodeType() { return 11; }
  }

  /** Markup set through innerHTML; the frame parses and checks it. */
  class Markup extends Node {
    constructor(html) { super(); this.html = html; }
    get textContent() { return this.html.replace(/<[^>]*>/g, ""); }
    toTree() { return { html: this.html }; }
  }

  const kebab = (name) => (name.startsWith("--") ? name : name === "cssFloat" ? "float" : name.replace(/[A-Z]/g, (c) => "-" + c.toLowerCase()));

  /** A stand-in for CSSStyleDeclaration: `style.x = v`, setProperty, cssText. */
  function makeStyle() {
    const decl = new Map();
    const api = {
      setProperty(name, value) {
        const v = value === null || value === undefined ? "" : String(value);
        if (v === "") decl.delete(String(name)); else decl.set(String(name), v);
      },
      getPropertyValue(name) { return decl.get(String(name)) ?? ""; },
      removeProperty(name) { const old = decl.get(String(name)) ?? ""; decl.delete(String(name)); return old; },
      get cssText() { return [...decl].map(([k, v]) => k + ": " + v).join("; "); },
      set cssText(text) {
        decl.clear();
        for (const part of String(text ?? "").split(";")) {
          const i = part.indexOf(":");
          if (i > 0) api.setProperty(part.slice(0, i).trim(), part.slice(i + 1).trim());
        }
      },
    };
    return new Proxy(api, {
      get(target, key) {
        if (typeof key !== "string" || key in target) return Reflect.get(target, key);
        return decl.get(kebab(key)) ?? "";
      },
      set(target, key, value) {
        if (typeof key !== "string") return false;
        if (key === "cssText") target.cssText = value; else target.setProperty(kebab(key), value);
        return true;
      },
    });
  }

  const VOID = new Set(["area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "source", "track", "wbr"]);
  const escapeText = (s) => s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
  const escapeAttr = (s) => escapeText(s).replace(/"/g, "&quot;");

  function serialize(node) {
    if (node instanceof Text) return escapeText(node.data);
    if (node instanceof Markup) return node.html;
    if (node instanceof Element) {
      const attrs = node.attributeList().map(([k, v]) => " " + k + '="' + escapeAttr(v) + '"').join("");
      const open = "<" + node.localName + attrs + ">";
      if (node.namespaceURI === HTML_NS && VOID.has(node.localName)) return open;
      return open + node.innerHTML + "</" + node.localName + ">";
    }
    return node.childNodes.map(serialize).join("");
  }

  class Element extends Node {
    constructor(tag, ns) {
      super();
      this.namespaceURI = ns;
      this.localName = ns === HTML_NS ? tag.toLowerCase() : tag;
      this.attrs = new Map();
      this.style = makeStyle();
      const el = this;
      const classes = () => (el.className ? el.className.split(/\s+/).filter(Boolean) : []);
      this.classList = {
        add: (...names) => { el.className = [...new Set([...classes(), ...names.map(String)])].join(" "); },
        remove: (...names) => { el.className = classes().filter((c) => !names.map(String).includes(c)).join(" "); },
        toggle: (name, force) => {
          const on = force ?? !classes().includes(String(name));
          if (on) el.classList.add(name); else el.classList.remove(name);
          return on;
        },
        contains: (name) => classes().includes(String(name)),
      };
    }
    get nodeType() { return 1; }
    get tagName() { return this.namespaceURI === HTML_NS ? this.localName.toUpperCase() : this.localName; }
    attrName(name) { return this.namespaceURI === HTML_NS ? String(name).toLowerCase() : String(name); }
    setAttribute(name, value) {
      const n = this.attrName(name);
      if (n === "style") this.style.cssText = value; else this.attrs.set(n, String(value));
    }
    setAttributeNS(_ns, name, value) { this.setAttribute(name, value); }
    getAttribute(name) {
      const n = this.attrName(name);
      if (n === "style") return this.style.cssText || null;
      return this.attrs.get(n) ?? null;
    }
    hasAttribute(name) { return this.getAttribute(name) !== null; }
    removeAttribute(name) {
      const n = this.attrName(name);
      if (n === "style") this.style.cssText = ""; else this.attrs.delete(n);
    }
    /** The attributes as [name, value] pairs, style included. */
    attributeList() {
      const list = [...this.attrs];
      if (this.style.cssText) list.push(["style", this.style.cssText]);
      return list;
    }
    get className() { return this.attrs.get("class") ?? ""; }
    set className(value) { this.attrs.set("class", String(value)); }
    get id() { return this.attrs.get("id") ?? ""; }
    set id(value) { this.attrs.set("id", String(value)); }
    get title() { return this.attrs.get("title") ?? ""; }
    set title(value) { this.attrs.set("title", String(value)); }
    get colSpan() { return Number(this.attrs.get("colspan") ?? 1); }
    set colSpan(value) { this.attrs.set("colspan", String(value)); }
    get rowSpan() { return Number(this.attrs.get("rowspan") ?? 1); }
    set rowSpan(value) { this.attrs.set("rowspan", String(value)); }
    get innerHTML() { return this.childNodes.map(serialize).join(""); }
    set innerHTML(html) {
      this.textContent = "";
      const s = html === null || html === undefined ? "" : String(html);
      if (s !== "") this.appendChild(new Markup(s));
    }
    get outerHTML() { return serialize(this); }
    insertAdjacentHTML(position, html) {
      const m = new Markup(String(html));
      if (position === "beforeend") this.appendChild(m);
      else if (position === "afterbegin") this.insertBefore(m, this.firstChild);
      else throw new Error('insertAdjacentHTML: only "beforeend" and "afterbegin" are supported');
    }
    toTree() {
      return {
        tag: this.localName,
        ns: this.namespaceURI === SVG_NS ? "svg" : this.namespaceURI === HTML_NS ? "html" : String(this.namespaceURI),
        attrs: this.attributeList(),
        children: this.childNodes.map((c) => c.toTree()),
      };
    }
  }
  class HTMLElement extends Element {}
  class SVGElement extends Element {}

  const document = {
    createElement: (tag) => new HTMLElement(String(tag), HTML_NS),
    createElementNS: (ns, tag) => (ns === SVG_NS ? new SVGElement(String(tag), SVG_NS) : new Element(String(tag), String(ns))),
    createTextNode: (text) => new Text(text),
    createDocumentFragment: () => new DocumentFragment(),
  };

  // The names the code gets, and the globals it should not reach, shadowed as
  // parameters of the function the source becomes.
  const GIVEN = { document, Node, Element, HTMLElement, SVGElement, Text, DocumentFragment };
  const SHADOWED = [
    "self", "globalThis", "window", "location", "close", "addEventListener", "onmessage", ...REMOVED,
  ];

  let next = 0;
  const pending = new Map();
  const call = (fn, args) => new Promise((resolve, reject) => {
    const id = ++next;
    pending.set(id, { resolve, reject });
    post({ type: "call", id, fn, args: JSON.parse(JSON.stringify(args ?? null)) });
  });
  const portal = new Proxy({}, { get: (_, fn) => (args) => call(String(fn), args) });

  const settled = async () => {
    // A call the code started but did not await may still draw when it ends.
    while (pending.size > 0) await new Promise((r) => setTimeout(r, 20));
    await new Promise((r) => setTimeout(r, 0));
  };

  let started = false;
  async function run(source) {
    const root = new HTMLElement("div", HTML_NS);
    const tree = () => root.childNodes.map((c) => c.toTree());
    try {
      const make = new Function(
        ...Object.keys(GIVEN), ...SHADOWED,
        source + "\nreturn typeof render === 'function' ? render : undefined;",
      );
      const render = make(...Object.values(GIVEN));
      if (!render) throw new Error("the code does not define a render(portal, root) function");
      await render(portal, root);
      await settled();
      post({ type: "done", tree: tree() });
    } catch (err) {
      let partial = [];
      try { partial = tree(); } catch { /* nothing drawn */ }
      post({ type: "error", message: String((err && err.message) || err), tree: partial });
    }
  }

  listen("message", (event) => {
    const m = event.data || {};
    if (m.type === "result") {
      const p = pending.get(m.id);
      if (!p) return;
      pending.delete(m.id);
      if (m.error) p.reject(new Error(m.error)); else p.resolve(m.value);
    } else if (m.type === "run" && !started) {
      started = true;
      run(String(m.source));
    }
  });
})();
