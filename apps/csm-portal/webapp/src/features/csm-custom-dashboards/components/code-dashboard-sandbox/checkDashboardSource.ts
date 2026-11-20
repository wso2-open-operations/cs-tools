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

/**
 * Names a code dashboard may not use anywhere outside strings and comments,
 * as a variable or a property: the ways out of the worker it runs in (the
 * worker, its messages and its address), the network, and the ways to reach
 * them indirectly (the global object, building code from text, prototypes).
 * The worker already has none of these working (codeDashboardWorker.js); this
 * refuses code that tries before it runs. The code catalogue's section 2
 * (csm-ai-integration-service) lists the same names for the model.
 */
export const FORBIDDEN_NAMES: readonly string[] = [
  "self", "globalThis", "window", "location", "postMessage", "importScripts", "import",
  "fetch", "XMLHttpRequest", "WebSocket", "EventSource", "WebTransport", "sendBeacon",
  "Worker", "SharedWorker", "BroadcastChannel",
  "eval", "Function", "constructor", "prototype", "__proto__",
  "__defineGetter__", "__defineSetter__", "__lookupGetter__", "__lookupSetter__",
];

const FORBIDDEN = new Set(FORBIDDEN_NAMES);

// After these words a `/` starts a regular expression, not a division.
const BEFORE_REGEX = new Set([
  "return", "typeof", "instanceof", "in", "of", "new", "delete", "void", "throw", "case", "do", "else", "yield", "await",
]);

const IDENT_START = /[A-Za-z_$]/;
const IDENT_PART = /[\w$]/;

/**
 * Checks a code dashboard's source before it runs. Returns why it is refused
 * (the first forbidden name, or a backslash outside a string, which could
 * spell one as `fetch`), or null when it may run. Strings, comments and
 * regular expressions are skipped; the code inside a template literal's
 * `${...}` is checked. Where a `/` is unclear it is read as a division, so its
 * text is checked rather than skipped.
 */
export function checkDashboardSource(source: string): string | null {
  let i = 0;
  // What the last token was, to tell a regular expression from a division.
  let slashDivides = false;
  // Brace depth of each open template literal's `${`, innermost last.
  const templates: number[] = [];
  let braces = 0;

  const skipString = (quote: string): void => {
    i++;
    while (i < source.length && source[i] !== quote && source[i] !== "\n") i += source[i] === "\\" ? 2 : 1;
    i++;
  };
  // Skips template text up to its end (returns false) or to a `${` (true).
  const skipTemplateText = (): boolean => {
    while (i < source.length) {
      const c = source[i];
      if (c === "\\") i += 2;
      else if (c === "`") {
        i++;
        return false;
      } else if (c === "$" && source[i + 1] === "{") {
        i += 2;
        return true;
      } else i++;
    }
    return false;
  };
  const skipRegex = (): void => {
    i++;
    let inClass = false;
    while (i < source.length && source[i] !== "\n") {
      const c = source[i];
      if (c === "\\") i += 2;
      else {
        i++;
        if (c === "[") inClass = true;
        else if (c === "]") inClass = false;
        else if (c === "/" && !inClass) break;
      }
    }
    while (i < source.length && IDENT_PART.test(source[i])) i++; // flags
  };

  while (i < source.length) {
    const c = source[i];
    const next = source[i + 1];
    if (/\s/.test(c)) {
      i++;
    } else if (c === "/" && next === "/") {
      while (i < source.length && source[i] !== "\n") i++;
    } else if (c === "/" && next === "*") {
      const end = source.indexOf("*/", i + 2);
      i = end < 0 ? source.length : end + 2;
    } else if (c === "'" || c === '"') {
      skipString(c);
      slashDivides = true;
    } else if (c === "`") {
      i++;
      if (skipTemplateText()) {
        templates.push(braces);
        slashDivides = false;
      } else slashDivides = true;
    } else if (c === "/" && !slashDivides) {
      skipRegex();
      slashDivides = true;
    } else if (IDENT_START.test(c)) {
      const start = i;
      while (i < source.length && IDENT_PART.test(source[i])) i++;
      const word = source.slice(start, i);
      if (FORBIDDEN.has(word)) return `the code uses \`${word}\`, which dashboards may not use`;
      slashDivides = !BEFORE_REGEX.has(word);
    } else if (/[0-9]/.test(c)) {
      while (i < source.length && /[\w.]/.test(source[i])) i++;
      slashDivides = true;
    } else if (c === "\\") {
      return "the code has a backslash outside a string, which dashboards may not use";
    } else if (c === "}" && templates.length > 0 && templates[templates.length - 1] === braces) {
      // The end of a template literal's `${...}`: back to its text.
      templates.pop();
      i++;
      if (skipTemplateText()) templates.push(braces);
      slashDivides = true;
    } else {
      if (c === "{") braces++;
      else if (c === "}") braces--;
      i++;
      // `a++ / b` and `a) / b` divide; after any other punctuation a `/` starts a regular expression.
      slashDivides = c === ")" || c === "]" || c === "}" || ((c === "+" || c === "-") && source[i - 2] === c);
    }
  }
  return null;
}
