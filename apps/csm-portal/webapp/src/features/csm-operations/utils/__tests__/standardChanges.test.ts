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
import { sanitizeRichTextHtml } from "@utils/sanitizeHtml";
import {
  appendPlaceholderToHtml,
  changeRequestsView,
  changeRequestsViewPath,
  contentPlaceholderKeys,
  hasParameterIssues,
  highlightStandardChangeHtml,
  insertPlaceholderAt,
  isLegacyStandardChangesTab,
  isStandardChangeTemplateUnavailable,
  legacyStandardChangesHref,
  normalizeStandardChangeParameters,
  placeholderKeys,
  renderStandardChangePlainText,
  splitPlaceholders,
  STANDARD_CHANGE_CATALOGUE_PATH,
  STANDARD_CHANGE_PROPOSALS_PATH,
  standardChangeParametersPayload,
  standardChangeProposalGaps,
  standardChangeProposalPath,
  standardChangeTemplatePath,
  standardChangeValueError,
  standardChangeValuesValid,
  undeclaredPlaceholders,
  unusedParameters,
  validateStandardChangeParameters,
} from "@features/csm-operations/utils/standardChanges";
import type { BeStandardChangeParameter } from "@api/backend/types";

const text = (key: string, extra: Partial<BeStandardChangeParameter> = {}): BeStandardChangeParameter => ({
  key,
  label: key.toUpperCase(),
  type: "text",
  required: true,
  ...extra,
});

describe("placeholders", () => {
  it("finds {{key}} with or without spaces inside the braces, and ignores what is not a key", () => {
    expect(placeholderKeys("Patch {{host}} to {{ version }}, not {{Host}} or {{1x}} or {single}; again {{host}}")).toEqual([
      "host",
      "version",
    ]);
  });

  it("splits a text at its placeholders, in order", () => {
    expect(splitPlaceholders("On {{host}}: {{ env }}.")).toEqual([
      { text: "On " },
      { key: "host", raw: "{{host}}" },
      { text: ": " },
      { key: "env", raw: "{{ env }}" },
      { text: "." },
    ]);
  });

  it("collects the keys of every text field the server renders, sorted", () => {
    expect(
      contentPlaceholderKeys({
        subject: "{{z}}",
        description: "<p>{{a}}</p>",
        rollbackDurationText: "{{m}} mins",
        testPlan: null,
      }),
    ).toEqual(["a", "m", "z"]);
  });

  it("names the placeholders no parameter declares, and the parameters nothing uses", () => {
    const content = { subject: "{{host}} {{port}}", implementationPlan: "<p>{{host}}</p>" };
    expect(undeclaredPlaceholders(content, [text("host"), text("unused")])).toEqual(["port"]);
    expect(unusedParameters(content, [text("host"), text("unused")])).toEqual(["unused"]);
  });
});

describe("parameter declarations", () => {
  it("accepts well-formed declarations", () => {
    const issues = validateStandardChangeParameters([
      text("host"),
      { key: "env", label: "Environment", type: "select", required: true, options: ["a", "b"] },
    ]);
    expect(issues).toEqual([{}, {}]);
    expect(hasParameterIssues(issues)).toBe(false);
  });

  it.each([
    ["Host", "key"],
    ["1st", "key"],
    ["has-dash", "key"],
    ["a".repeat(51), "key"],
  ])("refuses the key %j", (key, field) => {
    const [issue] = validateStandardChangeParameters([text(key)]);
    expect(issue).toHaveProperty(field);
  });

  it("refuses a key declared twice, on both rows", () => {
    const issues = validateStandardChangeParameters([text("host"), text("host ")]);
    expect(issues[0]!.key).toMatch(/more than once/);
    expect(issues[1]!.key).toMatch(/more than once/);
  });

  it("needs a label", () => {
    expect(validateStandardChangeParameters([text("host", { label: "  " })])[0]!.label).toMatch(/required/);
  });

  it("needs options on a choice, each once and none blank", () => {
    const choice = (options: string[]): BeStandardChangeParameter => ({ key: "env", label: "Env", type: "select", required: true, options });
    expect(validateStandardChangeParameters([choice([])])[0]!.options).toMatch(/1 to 50 options/);
    expect(validateStandardChangeParameters([choice(["a", "x".repeat(201)])])[0]!.options).toMatch(/1 to 200 characters/);
    // A blank line is ignored (it is not sent), not an option.
    expect(validateStandardChangeParameters([choice(["a", " ", "b"])])[0]).toEqual({});
    expect(validateStandardChangeParameters([choice([" ", ""])])[0]!.options).toMatch(/1 to 50 options/);
    expect(validateStandardChangeParameters([choice(["a", "a "])])[0]!.options).toMatch(/twice/);
  });

  it("sends them trimmed, without blank help, with options only on a choice", () => {
    expect(
      normalizeStandardChangeParameters([
        { key: " host ", label: " Host ", type: "text", required: true, helpText: "  ", options: ["stale"] },
        { key: "env", label: "Env", type: "select", required: false, helpText: " Pick one ", options: [" a", "", "b "] },
      ]),
    ).toEqual([
      { key: "host", label: "Host", type: "text", required: true },
      { key: "env", label: "Env", type: "select", required: false, helpText: "Pick one", options: ["a", "b"] },
    ]);
  });
});

describe("values", () => {
  it("needs a required value and accepts a blank optional one", () => {
    expect(standardChangeValueError(text("host"), "  ")).toBe("HOST is required.");
    expect(standardChangeValueError(text("host", { required: false }), "")).toBeNull();
  });

  it.each([
    [{ type: "text" }, "one\ntwo", /single line/],
    [{ type: "text" }, "x".repeat(501), /At most 500/],
    [{ type: "textarea" }, "x".repeat(4001), /At most 4000/],
    [{ type: "number" }, "12abc", /must be a number/],
    // The server reads decimal notation only (strconv.ParseFloat).
    [{ type: "number" }, "0x1F", /must be a number/],
    [{ type: "number" }, "0b101", /must be a number/],
    [{ type: "number" }, "0o17", /must be a number/],
    [{ type: "number" }, "Infinity", /must be a number/],
    [{ type: "date" }, "31/12/2026", /YYYY-MM-DD/],
    [{ type: "select", options: ["a", "b"] }, "c", /one of: a, b/],
  ] as const)("refuses a malformed value for %o", (extra, value, message) => {
    expect(standardChangeValueError(text("p", extra as Partial<BeStandardChangeParameter>), value)).toMatch(message);
  });

  it.each([
    [{ type: "textarea" }, "one\ntwo"],
    [{ type: "number" }, "-1.5"],
    [{ type: "number" }, "1e3"],
    [{ type: "number" }, ".5"],
    [{ type: "date" }, "2026-12-31"],
    [{ type: "select", options: ["a", "b"] }, "b"],
  ] as const)("accepts a well-formed value for %o", (extra, value) => {
    expect(standardChangeValueError(text("p", extra as Partial<BeStandardChangeParameter>), value)).toBeNull();
  });

  it("sends the declared keys only, trimmed, leaving blank optional values out", () => {
    const params = [text("host"), text("note", { required: false })];
    const values = { host: " gw-1 ", note: "  ", undeclared: "x" };
    expect(standardChangeParametersPayload(params, values)).toEqual({ host: "gw-1" });
    expect(standardChangeValuesValid(params, values)).toBe(true);
    expect(standardChangeValuesValid(params, { note: "x" })).toBe(false);
  });
});

describe("rendering, as the server renders", () => {
  it("fills a plain field with the value's whitespace collapsed and leaves an undeclared placeholder alone", () => {
    expect(renderStandardChangePlainText("Patch {{host}} ({{other}})", { host: "  gw-1 \n  east " }, new Set(["host"]))).toBe(
      "Patch gw-1 east ({{other}})",
    );
  });

  it("highlights the placeholders of a rich-text field, sanitized", () => {
    const html = highlightStandardChangeHtml('<p>On {{host}}<script>alert(1)</script></p><a href="https://x/{{host}}">x</a>', sanitizeRichTextHtml);
    expect(html).toContain('<mark class="std-param" data-param="host">{{host}}</mark>');
    expect(html).not.toContain("<script");
    // A placeholder inside an attribute is not touched.
    expect(html).toContain('href="https://x/{{host}}"');
  });

  it("fills a rich-text field with values as text, never markup, and a textarea's lines as <br>", () => {
    const params = [text("host"), text("notes", { type: "textarea", required: false }), text("empty", { required: false })];
    const html = highlightStandardChangeHtml(
      "<p>{{host}} / {{notes}} / {{empty}}</p>",
      sanitizeRichTextHtml,
      { parameters: params, values: { host: "<img src=x onerror=alert(1)>", notes: "one\ntwo" } },
    );
    expect(html).toContain('data-filled="true">&lt;img src=x onerror=alert(1)&gt;</mark>');
    expect(html).toContain('data-filled="true">one<br>two</mark>');
    // No value yet: the placeholder stays visible.
    expect(html).toContain('data-param="empty">{{empty}}</mark>');
  });
});

describe("inserting a placeholder", () => {
  it("appends to a rich-text field's last paragraph, or starts one", () => {
    expect(appendPlaceholderToHtml("<p>Run on</p>", "host", false)).toBe("<p>Run on {{host}}</p>");
    expect(appendPlaceholderToHtml("<ul><li>a</li></ul>", "host", false)).toBe("<ul><li>a</li></ul><p>{{host}}</p>");
    expect(appendPlaceholderToHtml("<p><br></p>", "host", true)).toBe("<p>{{host}}</p>");
  });

  it("inserts into a one-line field at the caret, or at the end", () => {
    expect(insertPlaceholderAt("Patch  now", "host", 6)).toBe("Patch {{host}} now");
    expect(insertPlaceholderAt("Patch", "host", null)).toBe("Patch{{host}}");
  });
});

describe("what a proposal still needs before the CAB", () => {
  const complete = {
    name: "Rotate",
    rationale: "<p>Done often.</p>",
    subject: "Rotate {{host}}",
    implementationPlan: "<p>a</p>",
    backoutPlan: "<p>b</p>",
    testPlan: "<p>c</p>",
    impact: "low",
    risk: "low",
  };

  it("needs nothing when complete", () => {
    expect(standardChangeProposalGaps("new", complete)).toEqual([]);
  });

  it("names every missing part of a new template", () => {
    expect(
      standardChangeProposalGaps("modify", {
        name: " ",
        rationale: "<p><br></p>",
        subject: "",
        implementationPlan: "",
        backoutPlan: "<p></p>",
        testPlan: "",
        impact: "",
        risk: "",
      }),
    ).toEqual([
      "a name",
      "a rationale",
      "a subject",
      "an implementation plan",
      "a backout plan",
      "a test plan",
      "an impact",
      "a risk",
    ]);
  });

  it("asks a retirement only for its name and rationale", () => {
    expect(standardChangeProposalGaps("retire", { ...complete, subject: "", implementationPlan: "", impact: "", risk: "" })).toEqual([]);
    expect(standardChangeProposalGaps("retire", { ...complete, rationale: "" })).toEqual(["a rationale"]);
  });
});

it("recognises the refusal of a template that moved on by its code, not its words", () => {
  expect(isStandardChangeTemplateUnavailable({ status: 409, payload: { errorCode: "standard_change_template_unavailable" } })).toBe(true);
  expect(isStandardChangeTemplateUnavailable({ status: 409, payload: { message: "standard_change_template_unavailable" } })).toBe(false);
  expect(isStandardChangeTemplateUnavailable(new Error("x"))).toBe(false);
});

describe("standard changes inside Operations > Change requests", () => {
  it("reads the sub-view from ?view=, the change request list without one or for a value it does not know", () => {
    expect(changeRequestsView(new URLSearchParams(""))).toBe("list");
    expect(changeRequestsView(new URLSearchParams("crQ=x"))).toBe("list");
    expect(changeRequestsView(new URLSearchParams("view=catalogue"))).toBe("catalogue");
    expect(changeRequestsView(new URLSearchParams("view=proposals&crQ=x"))).toBe("proposals");
    expect(changeRequestsView(new URLSearchParams("view=standard_changes"))).toBe("list");
  });

  it("links each sub-view, the template and the proposal pages", () => {
    expect(changeRequestsViewPath("list")).toBe("/operations/change-requests");
    expect(STANDARD_CHANGE_CATALOGUE_PATH).toBe("/operations/change-requests?view=catalogue");
    expect(STANDARD_CHANGE_PROPOSALS_PATH).toBe("/operations/change-requests?view=proposals");
    expect(standardChangeTemplatePath("tpl 1")).toBe("/operations/standard-changes/tpl%201");
    expect(standardChangeProposalPath("scp-1")).toBe("/operations/standard-change-proposals/scp-1");
  });

  it("sends an old standard changes link to the catalogue, or to the proposals it named", () => {
    expect(isLegacyStandardChangesTab("standard_changes")).toBe(true);
    expect(isLegacyStandardChangesTab("standard-changes")).toBe(true);
    expect(isLegacyStandardChangesTab("change_requests")).toBe(false);
    expect(isLegacyStandardChangesTab(null)).toBe(false);
    expect(legacyStandardChangesHref("?tab=standard_changes")).toBe("/operations/change-requests?view=catalogue");
    expect(legacyStandardChangesHref("?tab=standard_changes&view=proposals")).toBe(
      "/operations/change-requests?view=proposals",
    );
    expect(legacyStandardChangesHref("")).toBe("/operations/change-requests?view=catalogue");
    expect(legacyStandardChangesHref("?view=other&x=1", "#h")).toBe("/operations/change-requests?view=catalogue&x=1#h");
  });
});
