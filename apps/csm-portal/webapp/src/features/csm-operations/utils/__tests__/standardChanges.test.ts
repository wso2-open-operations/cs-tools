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
import { CR_FILTER_PARAM_KEYS } from "@features/csm-operations/utils/changeRequestsFiltersUrl";
import {
  appendPlaceholderToHtml,
  canonicalStandardChangesSearch,
  CHANGE_REQUEST_LIST_PARAM_PREFIX,
  changeRequestListSearch,
  changeRequestsView,
  contentPlaceholderKeys,
  hasParameterIssues,
  highlightStandardChangeHtml,
  insertPlaceholderAt,
  isLegacyStandardChangesTab,
  isLegacyStandardChangesView,
  isStandardChangeTemplateOtherCustomer,
  isStandardChangeTemplateUnavailable,
  legacyStandardChangesHref,
  normalizeStandardChangeParameters,
  placeholderKeys,
  proposalScope,
  proposalStateFilter,
  renderStandardChangePlainText,
  splitPlaceholders,
  STANDARD_CHANGE_PROPOSALS_PATH,
  standardChangeProposalCustomerEditable,
  standardChangeProposalCustomerFixedReason,
  STANDARD_CHANGES_PATH,
  standardChangeParametersPayload,
  standardChangeProposalGaps,
  standardChangeProposalPath,
  standardChangeRecentEntry,
  standardChangesPath,
  standardChangesSearch,
  standardChangesShow,
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
    accountId: "acct-1",
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
        accountId: "",
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

  it("needs a customer for a new template, after the name and rationale (the server's order)", () => {
    expect(standardChangeProposalGaps("new", { ...complete, accountId: "" })).toEqual(["a customer"]);
    expect(standardChangeProposalGaps("new", { ...complete, accountId: "  " })).toEqual(["a customer"]);
    expect(standardChangeProposalGaps("new", { ...complete, name: "", rationale: "", accountId: "", testPlan: "" })).toEqual([
      "a name",
      "a rationale",
      "a customer",
      "a test plan",
    ]);
  });

  it("never asks a revision or retirement for a customer: it has its template's", () => {
    expect(standardChangeProposalGaps("modify", { ...complete, accountId: "" })).toEqual([]);
    expect(standardChangeProposalGaps("retire", { ...complete, accountId: "" })).toEqual([]);
  });
});

describe("a standard change is one customer's", () => {
  it("lets the customer be named only on a new proposal that did not start from a change request", () => {
    expect(standardChangeProposalCustomerEditable({ kind: "new", sourceChangeRequest: null })).toBe(true);
    expect(standardChangeProposalCustomerEditable({ kind: "new" })).toBe(true);
    expect(standardChangeProposalCustomerEditable({ kind: "new", sourceChangeRequest: { id: "chg-1" } })).toBe(false);
    expect(standardChangeProposalCustomerEditable({ kind: "modify", sourceChangeRequest: null })).toBe(false);
    expect(standardChangeProposalCustomerEditable({ kind: "retire", sourceChangeRequest: null })).toBe(false);
  });

  it("keeps a copy's customer: only a copy of a proposal that had none names one", () => {
    const copiedFrom = { id: "scp-1", name: "SCP0000001" };
    expect(standardChangeProposalCustomerEditable({ kind: "new", copiedFrom, customer: { id: "acct-1" } })).toBe(false);
    expect(standardChangeProposalCustomerEditable({ kind: "new", copiedFrom, customer: null })).toBe(true);
    expect(standardChangeProposalCustomerFixedReason({ kind: "new", copiedFrom, customer: { id: "acct-1" } })).toBe(
      "The customer of the proposal it was copied from (SCP0000001). To propose it for another customer, start a new proposal.",
    );
    expect(standardChangeProposalCustomerFixedReason({ kind: "new", copiedFrom, customer: null })).toBeNull();
  });

  it("says why a fixed customer is fixed", () => {
    expect(standardChangeProposalCustomerFixedReason({ kind: "modify" })).toMatch(/template's customer/i);
    expect(standardChangeProposalCustomerFixedReason({ kind: "retire" })).toMatch(/template's customer/i);
    expect(standardChangeProposalCustomerFixedReason({ kind: "new", sourceChangeRequest: { id: "chg-1" } })).toMatch(
      /change request it started from/i,
    );
    expect(standardChangeProposalCustomerFixedReason({ kind: "new", sourceChangeRequest: null })).toBeNull();
  });

  it("recognises the refusal of another customer's project by its code, not its words", () => {
    expect(
      isStandardChangeTemplateOtherCustomer({ status: 400, payload: { errorCode: "standard_change_template_other_customer" } }),
    ).toBe(true);
    expect(
      isStandardChangeTemplateOtherCustomer({ status: 400, payload: { message: "standard_change_template_other_customer" } }),
    ).toBe(false);
    expect(
      isStandardChangeTemplateOtherCustomer({ status: 409, payload: { errorCode: "standard_change_template_unavailable" } }),
    ).toBe(false);
    expect(isStandardChangeTemplateOtherCustomer(new Error("x"))).toBe(false);
  });
});

it("recognises the refusal of a template that moved on by its code, not its words", () => {
  expect(isStandardChangeTemplateUnavailable({ status: 409, payload: { errorCode: "standard_change_template_unavailable" } })).toBe(true);
  expect(isStandardChangeTemplateUnavailable({ status: 409, payload: { message: "standard_change_template_unavailable" } })).toBe(false);
  expect(isStandardChangeTemplateUnavailable(new Error("x"))).toBe(false);
});

describe("standard changes inside Operations > Change requests", () => {
  const q = (search: string): URLSearchParams => new URLSearchParams(search);

  it("reads Standard changes from ?view=standard, and from the two old sub-view values", () => {
    expect(changeRequestsView(q(""))).toBe("list");
    expect(changeRequestsView(q("crQ=x"))).toBe("list");
    expect(changeRequestsView(q("view=standard"))).toBe("standard");
    expect(changeRequestsView(q("view=standard&show=proposals"))).toBe("standard");
    expect(changeRequestsView(q("view=catalogue"))).toBe("standard");
    expect(changeRequestsView(q("view=proposals&crQ=x"))).toBe("standard");
    expect(changeRequestsView(q("view=standard_changes"))).toBe("list");
    expect(changeRequestsView(q("show=proposals"))).toBe("list");
  });

  it("opens on the catalogue, and on the proposals for show=proposals or the old view=proposals", () => {
    expect(standardChangesShow(q("view=standard"))).toBe("templates");
    expect(standardChangesShow(q("view=standard&show=nonsense"))).toBe("templates");
    expect(standardChangesShow(q("view=catalogue"))).toBe("templates");
    expect(standardChangesShow(q("view=standard&show=proposals"))).toBe("proposals");
    expect(standardChangesShow(q("view=proposals"))).toBe("proposals");
  });

  it("knows the old two-sub-view form", () => {
    expect(isLegacyStandardChangesView(q("view=catalogue"))).toBe(true);
    expect(isLegacyStandardChangesView(q("view=proposals"))).toBe(true);
    expect(isLegacyStandardChangesView(q("view=standard"))).toBe(false);
    expect(isLegacyStandardChangesView(q(""))).toBe(false);
  });

  it("reads the proposals' filters, falling back to all proposals in any state for what it does not know", () => {
    expect(proposalScope(q(""))).toBe("all");
    expect(proposalScope(q("scope=awaiting"))).toBe("awaiting");
    expect(proposalScope(q("scope=mine"))).toBe("mine");
    expect(proposalScope(q("scope=all"))).toBe("all");
    expect(proposalScope(q("scope=everyone"))).toBe("all");
    expect(proposalStateFilter(q(""))).toBe("");
    expect(proposalStateFilter(q("state=draft"))).toBe("draft");
    expect(proposalStateFilter(q("state=cab_review"))).toBe("cab_review");
    expect(proposalStateFilter(q("state=open"))).toBe("");
  });

  it("links Standard changes, its proposals (filtered or not), the template and the proposal pages", () => {
    expect(STANDARD_CHANGES_PATH).toBe("/operations/change-requests?view=standard");
    expect(STANDARD_CHANGE_PROPOSALS_PATH).toBe("/operations/change-requests?view=standard&show=proposals");
    expect(standardChangesPath("proposals", { scope: "awaiting" })).toBe(
      "/operations/change-requests?view=standard&show=proposals&scope=awaiting",
    );
    expect(standardChangesPath("proposals", { scope: "mine", state: "draft" })).toBe(
      "/operations/change-requests?view=standard&show=proposals&scope=mine&state=draft",
    );
    // All proposals is the default, so its URL is the plain one.
    expect(standardChangesPath("proposals", { scope: "all" })).toBe(STANDARD_CHANGE_PROPOSALS_PATH);
    // The proposals' filters mean nothing on the catalogue.
    expect(standardChangesPath("templates", { scope: "awaiting", state: "draft" })).toBe(STANDARD_CHANGES_PATH);
    expect(standardChangeTemplatePath("tpl 1")).toBe("/operations/standard-changes/tpl%201");
    expect(standardChangeProposalPath("scp-1")).toBe("/operations/standard-change-proposals/scp-1");
  });

  it("keeps the change request list's filters on the way into Standard changes, and anything else", () => {
    expect(standardChangesSearch(q("crQ=gateway&crStates=new"), "templates").toString()).toBe(
      "view=standard&crQ=gateway&crStates=new",
    );
    expect(standardChangesSearch(q("crQ=gateway&other=1"), "proposals").toString()).toBe(
      "view=standard&show=proposals&crQ=gateway&other=1",
    );
  });

  it("brings the list back exactly as it was after a trip through Standard changes", () => {
    const list = q("crQ=gateway&crStates=new%2Cassess&crAnyOf=1");
    const templates = standardChangesSearch(list, "templates");
    const proposals = standardChangesSearch(templates, "proposals", { scope: "awaiting", state: "draft" });
    const back = standardChangesSearch(proposals, "templates");
    expect(changeRequestListSearch(proposals).toString()).toBe(list.toString());
    expect(changeRequestListSearch(back).toString()).toBe(list.toString());
  });

  it("names the list's parameter prefix after every one of its filters", () => {
    CR_FILTER_PARAM_KEYS.forEach((key) => expect(key.startsWith(CHANGE_REQUEST_LIST_PARAM_PREFIX)).toBe(true));
  });

  it("drops the proposals' filters on the way to the catalogue, and sets the ones it is given", () => {
    const proposals = q("view=standard&show=proposals&scope=all&state=rejected");
    expect(standardChangesSearch(proposals, "templates").toString()).toBe("view=standard");
    expect(standardChangesSearch(proposals, "proposals", { scope: "awaiting" }).toString()).toBe(
      "view=standard&show=proposals&scope=awaiting",
    );
    expect(standardChangesSearch(proposals, "proposals", { scope: "all", state: "rejected" }).toString()).toBe(
      "view=standard&show=proposals&state=rejected",
    );
  });

  it("drops Standard changes' own parameters on the way back to the list, and keeps the rest", () => {
    expect(changeRequestListSearch(q("view=standard&show=proposals&scope=all&state=draft&other=1")).toString()).toBe(
      "other=1",
    );
    expect(changeRequestListSearch(q("crQ=x")).toString()).toBe("crQ=x");
  });

  it("rewrites the old sub-view form to the current one, keeping the proposals' filters and the list's", () => {
    expect(canonicalStandardChangesSearch(q("view=catalogue")).toString()).toBe("view=standard");
    // The old proposals link lands on the plain proposals URL: all proposals.
    expect(canonicalStandardChangesSearch(q("view=proposals")).toString()).toBe("view=standard&show=proposals");
    expect(canonicalStandardChangesSearch(q("view=proposals&scope=awaiting&crQ=x")).toString()).toBe(
      "view=standard&show=proposals&scope=awaiting&crQ=x",
    );
  });

  it("sends an old standard changes link to the catalogue, or to the proposals it named", () => {
    expect(isLegacyStandardChangesTab("standard_changes")).toBe(true);
    expect(isLegacyStandardChangesTab("standard-changes")).toBe(true);
    expect(isLegacyStandardChangesTab("change_requests")).toBe(false);
    expect(isLegacyStandardChangesTab(null)).toBe(false);
    expect(legacyStandardChangesHref("?tab=standard_changes")).toBe("/operations/change-requests?view=standard");
    expect(legacyStandardChangesHref("?tab=standard_changes&view=proposals")).toBe(
      "/operations/change-requests?view=standard&show=proposals",
    );
    expect(legacyStandardChangesHref("")).toBe("/operations/change-requests?view=standard");
    expect(legacyStandardChangesHref("?view=other&x=1", "#h")).toBe("/operations/change-requests?view=standard&x=1#h");
  });

  it("names a template's or a proposal's Recent entry after the record, number first", () => {
    expect(
      standardChangeRecentEntry("template", { id: "tpl-3", number: "STD0000003", name: "Apply a WSO2 product update level" }),
    ).toEqual({
      kind: "standard_change",
      id: "tpl-3",
      title: "STD0000003 · Apply a WSO2 product update level",
      subtitle: "Standard change template",
      href: "/operations/standard-changes/tpl-3",
    });
    expect(standardChangeRecentEntry("proposal", { id: "scp-5", number: "SCP0000005", name: "Restart a node" })).toEqual({
      kind: "standard_change",
      id: "scp-5",
      title: "SCP0000005 · Restart a node",
      subtitle: "Standard change proposal",
      href: "/operations/standard-change-proposals/scp-5",
    });
    expect(standardChangeRecentEntry("proposal", { id: "scp-6", number: "SCP0000006", name: "  " }).title).toBe(
      "SCP0000006 · Untitled proposal",
    );
  });
});
