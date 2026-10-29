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

import { isBlankHtml } from "@utils/sanitizeHtml";
import { backendErrorCode } from "@features/csm-operations/utils/changeRequests";
import type {
  BeChangeRequestRisk,
  BeStandardChangeContent,
  BeStandardChangeParameter,
  BeStandardChangeParameterType,
  BeStandardChangeProposalKind,
  BeStandardChangeProposalState,
  BeStandardChangeTemplateState,
} from "@api/backend/types";

// The pure rules of a standard change template as the web needs them: what a
// parameter may look like, which placeholders a text refers to, how a value fills
// one in (a preview), and what a change request raised from a template sends.
// The entity service's `standard_change_template.go` is the authority for all of
// it; these mirror it so the form can say so before a round trip.

type ChipColor = "default" | "primary" | "secondary" | "success" | "error" | "info" | "warning";

/** The most parameters a template may declare. */
export const MAX_STANDARD_CHANGE_PARAMETERS = 30;
/** A parameter key: a lower-case letter, then lower-case letters, digits and underscores (at most 50). */
export const STANDARD_CHANGE_PARAMETER_KEY_RE = /^[a-z][a-z0-9_]{0,49}$/;
const PARAMETER_LABEL_MAX = 120;
const PARAMETER_HELP_MAX = 500;
const SELECT_OPTIONS_MAX = 50;
const SELECT_OPTION_MAX = 200;
/** A text value is one line of at most this many characters. */
export const STANDARD_CHANGE_TEXT_VALUE_MAX = 500;
/** A textarea value is at most this many characters. */
export const STANDARD_CHANGE_TEXTAREA_VALUE_MAX = 4000;

/** `{{ key }}`; spaces inside the braces are allowed. A non-key inside braces is plain text. */
const PLACEHOLDER_SOURCE = String.raw`\{\{\s*([a-z][a-z0-9_]{0,49})\s*\}\}`;

/** A fresh global matcher (a shared `/g` regex keeps `lastIndex` between calls). */
function placeholderRe(): RegExp {
  return new RegExp(PLACEHOLDER_SOURCE, "g");
}

/** The error code of a create (or Request Approval) refused because the template is no longer usable as shown. */
export const STANDARD_CHANGE_TEMPLATE_UNAVAILABLE = "standard_change_template_unavailable";
/** The error code of a create or edit that sends a field the template specifies. */
export const CHANGE_REQUEST_STANDARD_TEMPLATE_FIELD = "change_request_standard_template_field";

/** True for a refusal that means the template was suspended, retired or revised since it was shown. */
export function isStandardChangeTemplateUnavailable(err: unknown): boolean {
  return backendErrorCode(err) === STANDARD_CHANGE_TEMPLATE_UNAVAILABLE;
}

/**
 * The error code of a change request raised from a template for no project or
 * for a project of another customer than the template's (or moved to one).
 */
export const STANDARD_CHANGE_TEMPLATE_OTHER_CUSTOMER = "standard_change_template_other_customer";

/** True for a refusal that means the project is not one of the template's customer's. */
export function isStandardChangeTemplateOtherCustomer(err: unknown): boolean {
  return backendErrorCode(err) === STANDARD_CHANGE_TEMPLATE_OTHER_CUSTOMER;
}

/** A customer's (account's) own page. */
export function customerAccountPath(accountId: string): string {
  return `/customers/accounts/${encodeURIComponent(accountId)}`;
}

/**
 * Whether a proposal's customer can be named or changed in its editor: only a
 * new proposal started from nothing (no source change request, not a copy). One
 * started from a change request takes that change request's customer, a copy
 * the copied proposal's, and a modification or retirement its template's; the
 * server refuses another. A copy of a proposal that had no customer yet names
 * one, once (the server accepts naming it while it has none).
 */
export function standardChangeProposalCustomerEditable(proposal: {
  kind: BeStandardChangeProposalKind;
  sourceChangeRequest?: { id: string } | null;
  copiedFrom?: { id: string } | null;
  customer?: { id: string } | null;
}): boolean {
  if (proposal.kind !== "new" || proposal.sourceChangeRequest) {
    return false;
  }
  return !proposal.copiedFrom || !proposal.customer;
}

/** Why a proposal's customer is fixed, for the read-only field's helper. */
export function standardChangeProposalCustomerFixedReason(proposal: {
  kind: BeStandardChangeProposalKind;
  sourceChangeRequest?: { id: string } | null;
  copiedFrom?: { id: string; name?: string } | null;
  customer?: { id: string } | null;
}): string | null {
  if (proposal.kind === "modify" || proposal.kind === "retire") {
    return "The template's customer: a revision or retirement is for the customer the template belongs to.";
  }
  if (proposal.sourceChangeRequest) {
    return "The customer of the change request it started from.";
  }
  if (proposal.copiedFrom && proposal.customer) {
    const from = proposal.copiedFrom.name ? ` (${proposal.copiedFrom.name})` : "";
    return `The customer of the proposal it was copied from${from}. To propose it for another customer, start a new proposal.`;
  }
  return null;
}

/**
 * The warning on a change request raised from a template whose project is no
 * longer the template's customer's (`standardChange.customerMismatch`).
 */
export function standardChangeCustomerMismatchWarning(customerName?: string | null): string {
  const whose = customerName ? `${customerName}'s` : "the template customer's";
  return (
    `This change request's Customer Project is no longer one of ${whose} projects: it has moved to another customer ` +
    "since the change was raised. The template is pre-authorised only for its own customer, so approval cannot be " +
    "requested. Cancel this change request and raise a new one from a template of the project's customer."
  );
}

/** Said where a template is picked or a project chosen for one. */
export const STANDARD_CHANGE_ONE_CUSTOMER_RULE =
  "A standard change template belongs to one customer and is used only for that customer's projects.";

export const STANDARD_CHANGE_PARAMETER_TYPE_OPTIONS: ReadonlyArray<{
  value: BeStandardChangeParameterType;
  label: string;
}> = [
  { value: "text", label: "Text (one line)" },
  { value: "textarea", label: "Text (several lines)" },
  { value: "number", label: "Number" },
  { value: "date", label: "Date" },
  { value: "select", label: "Choice" },
];

export function standardChangeParameterTypeLabel(type: string): string {
  return STANDARD_CHANGE_PARAMETER_TYPE_OPTIONS.find((o) => o.value === type)?.label ?? type;
}

/** A standard change is pre-authorised, so it cannot be high risk. */
export const STANDARD_CHANGE_RISK_OPTIONS: ReadonlyArray<{ value: BeChangeRequestRisk; label: string }> = [
  { value: "low", label: "Low" },
  { value: "moderate", label: "Moderate" },
];

export const STANDARD_CHANGE_HIGH_RISK_REASON =
  "A standard change is pre-authorised, so it cannot be high risk: lower the risk, or raise this change as a normal change.";

const RISK_LABEL: Record<string, string> = { low: "Low", moderate: "Moderate", high: "High" };

export function changeRequestRiskLabel(risk?: string | null): string {
  return risk ? (RISK_LABEL[risk] ?? risk) : "—";
}

const TEMPLATE_STATE: Record<BeStandardChangeTemplateState, { label: string; color: ChipColor }> = {
  active: { label: "Active", color: "success" },
  suspended: { label: "Suspended", color: "warning" },
  retired: { label: "Retired", color: "default" },
};

export function standardChangeTemplateStateLabel(state?: string | null): string {
  return TEMPLATE_STATE[state as BeStandardChangeTemplateState]?.label ?? (state || "—");
}

export function standardChangeTemplateStateColor(state?: string | null): ChipColor {
  return TEMPLATE_STATE[state as BeStandardChangeTemplateState]?.color ?? "default";
}

const PROPOSAL_STATE: Record<BeStandardChangeProposalState, { label: string; color: ChipColor }> = {
  draft: { label: "Draft", color: "default" },
  cab_review: { label: "CAB review", color: "info" },
  approved: { label: "Approved", color: "success" },
  rejected: { label: "Rejected", color: "error" },
  withdrawn: { label: "Withdrawn", color: "default" },
};

export const STANDARD_CHANGE_PROPOSAL_STATES = Object.keys(PROPOSAL_STATE) as BeStandardChangeProposalState[];

export function standardChangeProposalStateLabel(state?: string | null): string {
  return PROPOSAL_STATE[state as BeStandardChangeProposalState]?.label ?? (state || "—");
}

export function standardChangeProposalStateColor(state?: string | null): ChipColor {
  return PROPOSAL_STATE[state as BeStandardChangeProposalState]?.color ?? "default";
}

const PROPOSAL_KIND: Record<BeStandardChangeProposalKind, string> = {
  new: "New template",
  modify: "Revision",
  retire: "Retirement",
};

export function standardChangeProposalKindLabel(kind?: string | null): string {
  return PROPOSAL_KIND[kind as BeStandardChangeProposalKind] ?? (kind || "—");
}

/** A proposal that can no longer change (approved, rejected, withdrawn). */
export function isStandardChangeProposalFinal(state?: string | null): boolean {
  return state === "approved" || state === "rejected" || state === "withdrawn";
}

/** One piece of a text split at its placeholders. */
export type PlaceholderSegment = { text: string } | { key: string; raw: string };

/** Splits `text` into its plain runs and its `{{key}}` placeholders, in order. */
export function splitPlaceholders(text: string): PlaceholderSegment[] {
  const out: PlaceholderSegment[] = [];
  let last = 0;
  for (const m of text.matchAll(placeholderRe())) {
    const at = m.index ?? 0;
    if (at > last) out.push({ text: text.slice(last, at) });
    out.push({ key: m[1]!, raw: m[0] });
    last = at + m[0].length;
  }
  if (last < text.length) out.push({ text: text.slice(last) });
  return out;
}

/** The keys `text` refers to, in order of first use. */
export function placeholderKeys(text: string | null | undefined): string[] {
  if (!text) return [];
  const seen = new Set<string>();
  for (const m of text.matchAll(placeholderRe())) seen.add(m[1]!);
  return [...seen];
}

/** The content's text fields a placeholder may appear in (the server's `standardChangeTextFields`). */
export const STANDARD_CHANGE_TEXT_FIELDS = [
  "subject",
  "description",
  "justification",
  "implementationPlan",
  "riskImpactAnalysis",
  "backoutPlan",
  "testPlan",
  "rollbackDurationText",
] as const;

export type StandardChangeTextField = (typeof STANDARD_CHANGE_TEXT_FIELDS)[number];

/** Every key the content's text fields refer to, sorted. */
export function contentPlaceholderKeys(content: Partial<Pick<BeStandardChangeContent, StandardChangeTextField>>): string[] {
  const seen = new Set<string>();
  for (const field of STANDARD_CHANGE_TEXT_FIELDS) {
    for (const key of placeholderKeys(content[field])) seen.add(key);
  }
  return [...seen].sort();
}

/** The placeholders the content uses that no parameter declares (the CAB is never asked while there are any). */
export function undeclaredPlaceholders(
  content: Partial<Pick<BeStandardChangeContent, StandardChangeTextField>>,
  parameters: readonly BeStandardChangeParameter[],
): string[] {
  const declared = new Set(parameters.map((p) => p.key.trim()));
  return contentPlaceholderKeys(content).filter((k) => !declared.has(k));
}

/** Declared parameters the content never refers to (allowed, but probably a slip). */
export function unusedParameters(
  content: Partial<Pick<BeStandardChangeContent, StandardChangeTextField>>,
  parameters: readonly BeStandardChangeParameter[],
): string[] {
  const used = new Set(contentPlaceholderKeys(content));
  return parameters.map((p) => p.key.trim()).filter((k) => k && !used.has(k));
}

/** The problems with one parameter declaration, by field, as the server would refuse it. */
export interface ParameterIssues {
  key?: string;
  label?: string;
  helpText?: string;
  options?: string;
}

/**
 * Checks a declaration list the way the server's `normalizeStandardChangeParameters`
 * does (keys unique and key-shaped, labels present, options only and always on a
 * choice), returning the issues per row (an empty object for a valid row).
 */
export function validateStandardChangeParameters(parameters: readonly BeStandardChangeParameter[]): ParameterIssues[] {
  const counts = new Map<string, number>();
  for (const p of parameters) counts.set(p.key.trim(), (counts.get(p.key.trim()) ?? 0) + 1);
  return parameters.map((p) => {
    const issues: ParameterIssues = {};
    const key = p.key.trim();
    if (!STANDARD_CHANGE_PARAMETER_KEY_RE.test(key)) {
      issues.key = "Start with a lower-case letter; use only lower-case letters, digits and underscores (at most 50).";
    } else if ((counts.get(key) ?? 0) > 1) {
      issues.key = `"${key}" is declared more than once.`;
    }
    const label = p.label.trim();
    if (!label) issues.label = "A label is required.";
    else if (label.length > PARAMETER_LABEL_MAX) issues.label = `At most ${PARAMETER_LABEL_MAX} characters.`;
    if ((p.helpText ?? "").trim().length > PARAMETER_HELP_MAX) {
      issues.helpText = `At most ${PARAMETER_HELP_MAX} characters.`;
    }
    // Blank lines are what the options box holds while it is typed in; they are not sent.
    const options = (p.options ?? []).map((o) => o.trim()).filter(Boolean);
    if (p.type === "select") {
      if (options.length === 0 || options.length > SELECT_OPTIONS_MAX) {
        issues.options = `A choice needs 1 to ${SELECT_OPTIONS_MAX} options.`;
      } else if (options.some((o) => o.length > SELECT_OPTION_MAX)) {
        issues.options = `Every option needs 1 to ${SELECT_OPTION_MAX} characters.`;
      } else if (new Set(options).size !== options.length) {
        issues.options = "An option is listed twice.";
      }
    }
    return issues;
  });
}

export function hasParameterIssues(issues: readonly ParameterIssues[]): boolean {
  return issues.some((i) => Object.keys(i).length > 0);
}

/** The declarations as they are sent: trimmed, empty help dropped, options only on a choice. */
export function normalizeStandardChangeParameters(parameters: readonly BeStandardChangeParameter[]): BeStandardChangeParameter[] {
  return parameters.map((p) => {
    const out: BeStandardChangeParameter = {
      key: p.key.trim(),
      label: p.label.trim(),
      type: p.type,
      required: p.required,
    };
    const help = (p.helpText ?? "").trim();
    if (help) out.helpText = help;
    if (p.type === "select") out.options = (p.options ?? []).map((o) => o.trim()).filter(Boolean);
    return out;
  });
}

/**
 * What is wrong with one value for its parameter (the server's
 * `normalizeStandardChangeValue`), or `null` when it is acceptable. A blank
 * optional value is acceptable.
 */
export function standardChangeValueError(p: BeStandardChangeParameter, raw: string | undefined): string | null {
  const v = (raw ?? "").trim();
  if (!v) return p.required ? `${p.label} is required.` : null;
  switch (p.type) {
    case "text":
      if (/[\r\n]/.test(v)) return `${p.label} must be a single line.`;
      if (v.length > STANDARD_CHANGE_TEXT_VALUE_MAX) return `At most ${STANDARD_CHANGE_TEXT_VALUE_MAX} characters.`;
      return null;
    case "textarea":
      return v.length > STANDARD_CHANGE_TEXTAREA_VALUE_MAX ? `At most ${STANDARD_CHANGE_TEXTAREA_VALUE_MAX} characters.` : null;
    case "number":
      // Decimal notation only, as the server reads it (strconv.ParseFloat): no 0x / 0b / 0o
      // prefixes, no Infinity or NaN.
      return /^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$/.test(v) && Number.isFinite(Number(v))
        ? null
        : `${p.label} must be a number.`;
    case "date":
      return /^\d{4}-\d{2}-\d{2}$/.test(v) && !Number.isNaN(Date.parse(`${v}T00:00:00Z`))
        ? null
        : `${p.label} must be a date (YYYY-MM-DD).`;
    case "select":
      return (p.options ?? []).includes(v) ? null : `${p.label} must be one of: ${(p.options ?? []).join(", ")}.`;
    default:
      return null;
  }
}

/** True when every value is acceptable for its parameter. */
export function standardChangeValuesValid(
  parameters: readonly BeStandardChangeParameter[],
  values: Readonly<Record<string, string>>,
): boolean {
  return parameters.every((p) => standardChangeValueError(p, values[p.key]) === null);
}

/**
 * The `standardChangeParameters` of a create: the declared keys only (an
 * undeclared key is refused), trimmed, blank optional ones left out.
 */
export function standardChangeParametersPayload(
  parameters: readonly BeStandardChangeParameter[],
  values: Readonly<Record<string, string>>,
): Record<string, string> {
  const out: Record<string, string> = {};
  for (const p of parameters) {
    const v = (values[p.key] ?? "").trim();
    if (v) out[p.key] = v;
  }
  return out;
}

/** A plain text field (subject, rollback duration) as the server renders it: a value's whitespace collapses to single spaces. */
export function renderStandardChangePlainText(
  text: string,
  values: Readonly<Record<string, string>>,
  declared: ReadonlySet<string>,
): string {
  return text.replace(placeholderRe(), (raw, key: string) =>
    declared.has(key) ? (values[key] ?? "").trim().split(/\s+/).filter(Boolean).join(" ") : raw,
  );
}

/** Where a filled-in or still-open placeholder sits in a preview: wrapped in `<mark>` so it stands out. */
const MARK_CLASS = "std-param";

/**
 * Renders a rich-text field for display, with every `{{key}}` placeholder
 * highlighted. With `values` it is the preview of a change request raised from
 * the template: a declared key with a value shows the value (as the server puts
 * it in: plain text, a textarea's line breaks as `<br>`), and one without shows
 * the placeholder, highlighted, so what is still to fill in is visible.
 *
 * The replacement is made in the sanitized DOM's text nodes, so a value can
 * never become markup (the server HTML-escapes it for the same reason) and a
 * placeholder inside an attribute is left alone. The result is sanitized HTML.
 */
export function highlightStandardChangeHtml(
  html: string,
  sanitize: (html: string) => string,
  fill?: { parameters: readonly BeStandardChangeParameter[]; values: Readonly<Record<string, string>> },
): string {
  const template = document.createElement("template");
  template.innerHTML = sanitize(html);
  const byKey = new Map((fill?.parameters ?? []).map((p) => [p.key, p]));
  const walker = document.createTreeWalker(template.content, NodeFilter.SHOW_TEXT);
  const textNodes: Text[] = [];
  for (let n = walker.nextNode(); n; n = walker.nextNode()) textNodes.push(n as Text);
  for (const node of textNodes) {
    const segments = splitPlaceholders(node.data);
    if (!segments.some((s) => "key" in s)) continue;
    const frag = document.createDocumentFragment();
    for (const s of segments) {
      if ("text" in s) {
        frag.append(s.text);
        continue;
      }
      const mark = document.createElement("mark");
      mark.className = MARK_CLASS;
      mark.setAttribute("data-param", s.key);
      const param = byKey.get(s.key);
      const value = (fill?.values[s.key] ?? "").trim();
      if (fill && param && value) {
        mark.setAttribute("data-filled", "true");
        const lines = param.type === "textarea" ? value.replace(/\r\n/g, "\n").split("\n") : [value];
        lines.forEach((line, i) => {
          if (i > 0) mark.append(document.createElement("br"));
          mark.append(line);
        });
      } else {
        mark.textContent = s.raw;
      }
      frag.append(mark);
    }
    node.replaceWith(frag);
  }
  return template.innerHTML;
}

/** Puts `{{key}}` at the end of a rich-text field's last paragraph (or in a new one when it is blank). */
export function appendPlaceholderToHtml(html: string, key: string, isBlank: boolean): string {
  const token = `{{${key}}}`;
  if (isBlank) return `<p>${token}</p>`;
  const at = html.lastIndexOf("</p>");
  if (at !== -1 && at + 4 === html.trimEnd().length) return `${html.slice(0, at)} ${token}</p>`;
  return `${html}<p>${token}</p>`;
}

/** Inserts `{{key}}` into a plain text at the caret (or the end). */
export function insertPlaceholderAt(text: string, key: string, caret?: number | null): string {
  const token = `{{${key}}}`;
  const at = caret == null || caret < 0 || caret > text.length ? text.length : caret;
  return `${text.slice(0, at)}${token}${text.slice(at)}`;
}

/**
 * The create form with a template preselected (`?standardChangeTemplateId=`), so
 * the link is shareable and survives a reload.
 */
export function raiseFromTemplatePath(templateId: string): string {
  return `/operations/change-requests/new?standardChangeTemplateId=${encodeURIComponent(templateId)}`;
}

/**
 * Standard changes are templated change requests, so they live inside
 * Operations > Change requests (`/operations/change-requests`) as ONE sub-view
 * next to the change request list, chosen by `?view=standard`: no `view` (or
 * one this page does not know) is the list, exactly as before. Inside it the
 * template catalogue is the default and the proposals to the CAB a secondary
 * view (`&show=proposals`), whose filters are `scope` (all proposals by
 * default; mine / awaiting my decision when picked) and `state`. Everything is
 * in the URL so each place is linkable and Back from a template or a proposal
 * returns to it. The change request list's own filters are all `cr`-prefixed
 * (`CHANGE_REQUEST_LIST_PARAM_PREFIX`), so they never collide with these: they
 * do not apply to Standard changes, but they are kept in the URL while there,
 * so going back to the list finds it exactly as it was; these are dropped on
 * the way back to the list.
 *
 * Earlier links had two sub-views, `?view=catalogue` and `?view=proposals`;
 * both are still read (`changeRequestsView`, `standardChangesShow`) and
 * rewritten to the current form (`canonicalStandardChangesSearch`).
 */
export const CHANGE_REQUESTS_PATH = "/operations/change-requests";
export const CHANGE_REQUESTS_VIEW_PARAM = "view";
/** The `view` value of Standard changes. */
export const STANDARD_CHANGES_VIEW = "standard";
/** Which part of Standard changes: absent for the catalogue, `proposals` for the proposals. */
export const STANDARD_CHANGES_SHOW_PARAM = "show";
/** The proposals view's "whose proposals" filter. */
export const PROPOSAL_SCOPE_PARAM = "scope";
/** The proposals view's state filter. */
export const PROPOSAL_STATE_PARAM = "state";
/**
 * Every parameter of the change request list's own filters starts with this
 * (`CR_FILTER_PARAM_KEYS`). They are kept, unread, while on Standard changes,
 * and a pin of Standard changes ignores them (the Change requests nav node's
 * `views.ignoreParamPrefix`, which a test keeps equal to this).
 */
export const CHANGE_REQUEST_LIST_PARAM_PREFIX = "cr";

/** The `view` values of the two sub-views Standard changes replaced; still read. */
const LEGACY_VIEWS = ["catalogue", "proposals"] as const;

export type ChangeRequestsView = "list" | "standard";
export type StandardChangesShow = "templates" | "proposals";
/** Whose proposals the proposals view lists. */
export type ProposalScope = "all" | "mine" | "awaiting";
export const PROPOSAL_SCOPES: readonly ProposalScope[] = ["all", "mine", "awaiting"];
/** The scope the proposals view opens on, and the one its URL leaves out. */
export const DEFAULT_PROPOSAL_SCOPE: ProposalScope = "all";

/** What Recent, "Pin this page" and the breadcrumbs call the two places. */
export const STANDARD_CHANGES_LABELS: Readonly<Record<StandardChangesShow, string>> = {
  templates: "Standard changes",
  proposals: "Standard change proposals",
};

function isLegacyView(view: string | null): view is (typeof LEGACY_VIEWS)[number] {
  return (LEGACY_VIEWS as readonly string[]).includes(view ?? "");
}

/** The Change requests sub-view a URL's query selects. */
export function changeRequestsView(params: URLSearchParams): ChangeRequestsView {
  const view = params.get(CHANGE_REQUESTS_VIEW_PARAM);
  return view === STANDARD_CHANGES_VIEW || isLegacyView(view) ? "standard" : "list";
}

/** Which part of Standard changes a URL's query selects: the catalogue unless it asks for the proposals. */
export function standardChangesShow(params: URLSearchParams): StandardChangesShow {
  return params.get(STANDARD_CHANGES_SHOW_PARAM) === "proposals" || params.get(CHANGE_REQUESTS_VIEW_PARAM) === "proposals"
    ? "proposals"
    : "templates";
}

/** True for a query in the old two-sub-view form (`?view=catalogue|proposals`), which `canonicalStandardChangesSearch` rewrites. */
export function isLegacyStandardChangesView(params: URLSearchParams): boolean {
  return isLegacyView(params.get(CHANGE_REQUESTS_VIEW_PARAM));
}

/** The proposals view's scope filter; "all" without one (or for a value it does not know). */
export function proposalScope(params: URLSearchParams): ProposalScope {
  const scope = params.get(PROPOSAL_SCOPE_PARAM);
  return (PROPOSAL_SCOPES as readonly string[]).includes(scope ?? "") ? (scope as ProposalScope) : DEFAULT_PROPOSAL_SCOPE;
}

/** The proposals view's state filter; "" (any state) without one or for a state that does not exist. */
export function proposalStateFilter(params: URLSearchParams): BeStandardChangeProposalState | "" {
  const state = params.get(PROPOSAL_STATE_PARAM);
  return (STANDARD_CHANGE_PROPOSAL_STATES as readonly string[]).includes(state ?? "")
    ? (state as BeStandardChangeProposalState)
    : "";
}

/** The parameters only Standard changes reads; dropped on the way back to the change request list. */
const STANDARD_CHANGES_PARAMS = [
  CHANGE_REQUESTS_VIEW_PARAM,
  STANDARD_CHANGES_SHOW_PARAM,
  PROPOSAL_SCOPE_PARAM,
  PROPOSAL_STATE_PARAM,
] as const;

/**
 * The query of a place inside Standard changes, built from the current one:
 * `view=standard` first, then `show=proposals` and the proposals' filters when
 * that is where it goes (the default scope, all, is left out). The proposals'
 * filters are dropped on the way to the catalogue. Anything else in the query
 * is kept, the change request list's own `cr*` filters included: they do not
 * apply here, but switching back to the list restores them from it (and a pin
 * of the page ignores them, so it reads "Standard changes", not "Standard
 * changes: 3 filters").
 */
export function standardChangesSearch(
  current: URLSearchParams,
  show: StandardChangesShow,
  filters: { scope?: ProposalScope; state?: BeStandardChangeProposalState | "" } = {},
): URLSearchParams {
  const next = new URLSearchParams();
  next.set(CHANGE_REQUESTS_VIEW_PARAM, STANDARD_CHANGES_VIEW);
  if (show === "proposals") {
    next.set(STANDARD_CHANGES_SHOW_PARAM, "proposals");
    if (filters.scope && filters.scope !== DEFAULT_PROPOSAL_SCOPE) next.set(PROPOSAL_SCOPE_PARAM, filters.scope);
    if (filters.state) next.set(PROPOSAL_STATE_PARAM, filters.state);
  }
  current.forEach((value, key) => {
    if ((STANDARD_CHANGES_PARAMS as readonly string[]).includes(key)) return;
    next.append(key, value);
  });
  return next;
}

/**
 * The current form of a Standard changes query: an old `?view=catalogue` or
 * `?view=proposals` becomes `?view=standard[&show=proposals]`, keeping the
 * proposals' filters and everything else (the list's `cr*` filters too).
 */
export function canonicalStandardChangesSearch(current: URLSearchParams): URLSearchParams {
  const show = standardChangesShow(current);
  return standardChangesSearch(current, show, {
    scope: proposalScope(current),
    state: proposalStateFilter(current),
  });
}

/**
 * The query of the change request list, built from the current one: Standard
 * changes' own parameters dropped, everything else (the list's own `cr*`
 * filters, kept while on Standard changes) kept, so the list comes back as it
 * was left.
 */
export function changeRequestListSearch(current: URLSearchParams): URLSearchParams {
  const next = new URLSearchParams(current);
  STANDARD_CHANGES_PARAMS.forEach((k) => next.delete(k));
  return next;
}

/** The URL of a place inside Standard changes (the catalogue by default). */
export function standardChangesPath(
  show: StandardChangesShow = "templates",
  filters: { scope?: ProposalScope; state?: BeStandardChangeProposalState | "" } = {},
): string {
  return `${CHANGE_REQUESTS_PATH}?${standardChangesSearch(new URLSearchParams(), show, filters).toString()}`;
}

/** The catalogue: `/operations/change-requests?view=standard`. */
export const STANDARD_CHANGES_PATH = standardChangesPath();
/** The proposals (all of them): `/operations/change-requests?view=standard&show=proposals`. */
export const STANDARD_CHANGE_PROPOSALS_PATH = standardChangesPath("proposals");

/**
 * Where a link to the old, separate Operations tab lands: the standard change
 * catalogue, or its proposals for `view=proposals` (the old tab's own switch).
 * Covers both forms that tab had, `/operations?tab=standard_changes` and
 * `/operations/standard-changes`; every other query parameter and the hash are
 * kept, `tab` is dropped.
 */
export function legacyStandardChangesHref(search: string, hash = ""): string {
  const params = new URLSearchParams(search);
  params.delete("tab");
  return `${CHANGE_REQUESTS_PATH}?${canonicalStandardChangesSearch(params).toString()}${hash}`;
}

/** True for an old `?tab=` value that named the standard changes tab. */
export function isLegacyStandardChangesTab(tab: string | null): boolean {
  return tab === "standard_changes" || tab === "standard-changes";
}

export function standardChangeTemplatePath(id: string): string {
  return `/operations/standard-changes/${encodeURIComponent(id)}`;
}

export function standardChangeProposalPath(id: string): string {
  return `/operations/standard-change-proposals/${encodeURIComponent(id)}`;
}

/**
 * The Recent entry a template's or a proposal's own page records on a visit
 * (so "Pin this page" there reads "STD0000003 · Apply a product update"
 * rather than "Operations"), shaped as `useRecordRecentView` takes it; the
 * same "number · name" title a change request's page records.
 */
export function standardChangeRecentEntry(
  kind: "template" | "proposal",
  record: { id: string; number?: string | null; name?: string | null },
): { kind: "standard_change"; id: string; title: string; subtitle: string; href: string } {
  const name = record.name?.trim() || (kind === "proposal" ? "Untitled proposal" : "");
  return {
    kind: "standard_change",
    id: record.id,
    title: [record.number?.trim(), name].filter((s): s is string => !!s).join(" · ") || record.id,
    subtitle: kind === "template" ? "Standard change template" : "Standard change proposal",
    href: kind === "template" ? standardChangeTemplatePath(record.id) : standardChangeProposalPath(record.id),
  };
}

/** What a proposal still needs before the CAB can be asked (the server's `checkStandardChangeProposalComplete`), worded for the form. */
export function standardChangeProposalGaps(
  kind: BeStandardChangeProposalKind,
  draft: {
    name: string;
    rationale: string;
    /** The customer (account) id the proposal is for; "" when none is named. */
    accountId: string;
    subject: string;
    implementationPlan: string;
    backoutPlan: string;
    testPlan: string;
    impact: string;
    risk: string;
  },
): string[] {
  const gaps: string[] = [];
  if (!draft.name.trim()) gaps.push("a name");
  if (isBlankHtml(draft.rationale)) gaps.push("a rationale");
  // A new template belongs to one customer, which the proposal names; a
  // modification or retirement has its template's.
  if (kind === "new" && !draft.accountId.trim()) gaps.push("a customer");
  if (kind === "retire") return gaps;
  if (!draft.subject.trim()) gaps.push("a subject");
  if (isBlankHtml(draft.implementationPlan)) gaps.push("an implementation plan");
  if (isBlankHtml(draft.backoutPlan)) gaps.push("a backout plan");
  if (isBlankHtml(draft.testPlan)) gaps.push("a test plan");
  if (!draft.impact) gaps.push("an impact");
  if (!draft.risk) gaps.push("a risk");
  return gaps;
}
