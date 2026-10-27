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

// Service-catalog variable classification + value encoding for the service
// request form. ServiceNow returns the full raw variable set for a catalog
// item — including auto-populated "context" fields (project/deployment/product)
// and hidden system fields (case type, priority, ...) — so the portal must
// classify them client-side, exactly as the customer portal does. The shape
// (`id`/`questionText`/`order`/`type`) and these heuristics mirror the
// customer-portal reference (operations/utils/serviceRequestValidation.ts).

import { htmlToPlainText } from "@components/rich-text-editor/richTextEditor";
import type {
  BeCatalogItemVariable,
  BeCatalogItemVariableChoice,
} from "@api/backend/types";

// Variable `type` strings ServiceNow emits (matched case-insensitively).
export const VARIABLE_TYPE_SINGLE_LINE = "Single Line Text";
export const VARIABLE_TYPE_MULTI_LINE = "Multi Line Text";
export const VARIABLE_TYPE_SELECT = "Select Box";
export const VARIABLE_TYPE_CHECKBOX = "Checkbox";
export const VARIABLE_TYPE_RADIO = "Radio Buttons";

/**
 * A choice whose `value` can actually be submitted, paired with the label to
 * show for it. The contract makes every key inside a choice nullable, so an
 * option with no `value` is dropped: it would submit nothing and render as a
 * blank row. A blank `value` is dropped for the same reason — in a select it
 * is the "nothing selected" sentinel, so it cannot be told apart from an
 * unanswered field. `text` falls back to `value` when absent.
 */
export interface UsableChoice {
  value: string;
  label: string;
}

/**
 * The submittable options on a variable, in the order the backing data source
 * returned them (never re-sorted). Empty when the variable carries no choice
 * list, or when every option in it is malformed — in which case the form
 * falls back to a plain text input rather than showing an empty dropdown.
 */
export function usableChoices(variable: BeCatalogItemVariable): UsableChoice[] {
  const choices: BeCatalogItemVariableChoice[] = variable.choices ?? [];
  return choices
    .filter((c): c is BeCatalogItemVariableChoice & { value: string } =>
      typeof c?.value === "string" && c.value !== "",
    )
    .map((c) => ({ value: c.value, label: c.text ?? c.value }));
}

/** Boolean-ish field that renders a Yes/No dropdown (used only when the
 *  backing data source supplied no choice list for the field). */
export function isChoiceField(variable: BeCatalogItemVariable): boolean {
  // Case-insensitive, matching the documented contract and the other classifiers.
  const t = (variable.type ?? "").trim().toLowerCase();
  return (
    t === VARIABLE_TYPE_SELECT.toLowerCase() ||
    t === VARIABLE_TYPE_RADIO.toLowerCase() ||
    t === VARIABLE_TYPE_CHECKBOX.toLowerCase()
  );
}

/** Multi-line free text (but not the Description field, which is rich text). */
export function isMultiLineField(variable: BeCatalogItemVariable): boolean {
  const t = (variable.type ?? "").trim().toLowerCase();
  return (
    (t === VARIABLE_TYPE_MULTI_LINE.toLowerCase() || t.includes("multi")) &&
    !isDescriptionField(variable.questionText ?? "")
  );
}

/** The Description field — rendered with the rich-text editor; value sent as HTML. */
export function isDescriptionField(questionText: string): boolean {
  const normalized = (questionText ?? "")
    .replace(/^\s*\*?\s*/, "")
    .trim()
    .toLowerCase();
  return normalized === "description";
}

/** File Copy Path field — a plain (optional) text input, not an upload. */
export function isFileCopyPathField(variable: BeCatalogItemVariable): boolean {
  const q = (variable.questionText ?? "").trim();
  const t = (variable.type ?? "").trim();
  return /file\s*copy\s*path/i.test(q) || /file\s*copy\s*path/i.test(t);
}

function isAttachmentType(type: string): boolean {
  const t = (type ?? "").trim().toLowerCase();
  return (
    t === "attachment" ||
    t === "file" ||
    t === "file upload" ||
    t.includes("attachment") ||
    t.includes("file upload") ||
    (t.includes("file") && !t.includes("configuration")) ||
    t.includes("attach") ||
    t.includes("document") ||
    t.includes("upload")
  );
}

function isAttachmentFieldByQuestionText(questionText: string): boolean {
  const q = (questionText ?? "").trim().toLowerCase();
  if (!q) return false;
  const patterns = [
    /attachment/i,
    /attach\s/i,
    /attach$/i,
    /file\s*upload/i,
    /upload\s*file/i,
    /vulnerability\s*scan\s*report/i,
    /scan\s*report/i,
    /upload\s*document/i,
    /document\s*upload/i,
    /attach\s*report/i,
    /attach\s*document/i,
  ];
  return patterns.some((p) => p.test(q));
}

/** Attachment/file-upload field (by type or questionText). Optional + collected
 *  in the shared attachments section, so these are not rendered as text inputs. */
export function isAttachmentField(variable: BeCatalogItemVariable): boolean {
  // A File Copy Path field is a plain (optional) text input, not an upload —
  // exclude it explicitly since its type/label can contain "file" and would
  // otherwise be swallowed by isAttachmentType and dropped from the form.
  if (isFileCopyPathField(variable)) return false;
  return (
    isAttachmentType(variable.type ?? "") ||
    isAttachmentFieldByQuestionText(variable.questionText ?? "")
  );
}

const CONTEXT_FIELD_PATTERNS = [
  /^project$/i,
  /^deployments?$/i,
  /^product$/i,
  /^wso2\s*product$/i,
  /^environment$/i,
];

const HIDDEN_FIELD_PATTERNS = [
  /^case\s*type$/i,
  /^service\s*request\s*category$/i,
  /^classification$/i,
  /^class\s*fication$/i,
  /^srns$/i,
  /^state$/i,
  /^assignment\s*group$/i,
  /^assigned\s*to$/i,
  /^priority$/i,
  /^impact$/i,
];

/** Auto-populated from the project/deployment/product cascade — not shown. */
export function isContextField(questionText: string): boolean {
  const normalized = questionText?.trim().toLowerCase() ?? "";
  return CONTEXT_FIELD_PATTERNS.some((p) => p.test(normalized));
}

/** System field ServiceNow defaults — sent by the backend, not shown. */
export function isHiddenField(questionText: string): boolean {
  const normalized = questionText?.trim().toLowerCase() ?? "";
  return HIDDEN_FIELD_PATTERNS.some((p) => p.test(normalized));
}

const DATE_TIME_FIELD_PATTERNS: RegExp[] = [
  /start\s*(date|time)/i,
  /end\s*(date|time)/i,
  /scheduled\s*(date|time|start|end)/i,
  /implementation\s*(date|time|start|end)/i,
  /planned\s*(start|end|date|time)/i,
  /actual\s*(start|end|date|time)/i,
  /date\s*(and\s*)?\/?\s*time/i,
  /time\s*(and\s*)?\/?\s*date/i,
];

/** Renders a datetime-local picker (by type or questionText). */
export function isDateTimeField(variable: BeCatalogItemVariable): boolean {
  const t = (variable.type ?? "").trim();
  if (/date.*time|datetime/i.test(t) || /^date$/i.test(t)) return true;
  const q = (variable.questionText ?? "").trim();
  return DATE_TIME_FIELD_PATTERNS.some((p) => p.test(q));
}

/** Strip the leading `*`/whitespace ServiceNow prefixes onto required labels. */
export function variableLabel(variable: BeCatalogItemVariable): string {
  return (variable.questionText ?? "").replace(/^\s*\*?\s*/, "").trim() || variable.id;
}

/** Strips the same leading `*`/bullet ServiceNow prefix as {@link variableLabel},
 *  plus an optional "Required:" prefix some catalog items also carry, so a
 *  question text like "* Required: Start Date" still matches exactly
 *  "start date" below. The marker group is `*` (zero-or-more), not
 *  `+` (one-or-more): a label with no leading `*`/bullet at all, e.g. a bare
 *  "Required: Start Date", must still reach the "Required:" strip — `+`
 *  would require a marker character to be present before even attempting
 *  that strip, silently leaving "Required:" on and making the field fail to
 *  match "start date" below. */
function normalizeDateTimeFieldLabel(questionText: string): string {
  return (questionText ?? "").trim().replace(/^[\s*•]*\s*(?:required\s*:?\s*)?/i, "").trim();
}

/** The "Start Date"/"Start Time" field a start/end window comparison checks
 *  against (not just any field matching the broader `isDateTimeField`
 *  patterns — only one named exactly this). */
export function isStartDateTimeField(variable: BeCatalogItemVariable): boolean {
  return /^start\s*(date|time)$/i.test(
    normalizeDateTimeFieldLabel(variable.questionText ?? ""),
  );
}

/** The "End Date"/"End Time" field a start/end window comparison checks
 *  against. */
export function isEndDateTimeField(variable: BeCatalogItemVariable): boolean {
  return /^end\s*(date|time)$/i.test(
    normalizeDateTimeFieldLabel(variable.questionText ?? ""),
  );
}

/** "date" or "time" — a Start/End field's own kind, read off its label after
 *  stripping the "Start "/"End " prefix itself. Used to pair a catalog's
 *  "Start Date" with its "End Date" and, separately, "Start Time" with
 *  "End Time", rather than crossing the two or only ever looking at
 *  whichever Start/End field happens to be first. */
function dateTimeWindowKind(variable: BeCatalogItemVariable, prefix: "start" | "end"): string {
  return normalizeDateTimeFieldLabel(variable.questionText ?? "")
    .replace(new RegExp(`^${prefix}\\s*`, "i"), "")
    .toLowerCase();
}

/** The "End <kind>" field pairing with a given "Start <kind>" field (date
 *  pairs with date, time with time) among `variables`, or undefined if this
 *  catalog item has no matching end field. */
export function matchingEndDateTimeField(
  variables: BeCatalogItemVariable[],
  startField: BeCatalogItemVariable,
): BeCatalogItemVariable | undefined {
  const kind = dateTimeWindowKind(startField, "start");
  return variables.find((v) => isEndDateTimeField(v) && dateTimeWindowKind(v, "end") === kind);
}

/** The "Start <kind>" field pairing with a given "End <kind>" field. */
export function matchingStartDateTimeField(
  variables: BeCatalogItemVariable[],
  endField: BeCatalogItemVariable,
): BeCatalogItemVariable | undefined {
  const kind = dateTimeWindowKind(endField, "end");
  return variables.find((v) => isStartDateTimeField(v) && dateTimeWindowKind(v, "start") === kind);
}

/**
 * User-editable variables — excludes context and hidden fields, sorted by the
 * backend's display order. This is the set the form renders and validates.
 *
 * Also excludes a variable the backend itself marks `hidden: true` or
 * `active: false` (the additive metadata from `CHANGES-sr-variable-metadata.md`
 * — see `BeCatalogItemVariable`'s doc comment). Both are read as `undefined`
 * for a variable the backend hasn't tagged (or, in tests, a hand-built
 * fixture), which is treated as "not hidden"/"active" — the same as before
 * this metadata existed — so this is additive filtering, not a behaviour
 * change for untagged data.
 */
export function getUserEditableVariables(
  variables: BeCatalogItemVariable[],
): BeCatalogItemVariable[] {
  return variables
    .filter(
      (v) =>
        !isContextField(v.questionText ?? "") &&
        !isHiddenField(v.questionText ?? "") &&
        v.hidden !== true &&
        v.active !== false,
    )
    .sort(
      (a, b) =>
        (a.order ?? Number.MAX_SAFE_INTEGER) -
        (b.order ?? Number.MAX_SAFE_INTEGER),
    );
}

/**
 * Whether a variable must be filled in. Prefers the backend's own `mandatory`
 * flag (the correct one — see the warning on `BeCatalogItemVariable.hasMandatory`)
 * when present. Falls back to the pre-existing hot fix — every typable
 * (non-attachment, non-File-Copy-Path) field required — only for a variable
 * the backend hasn't tagged with `mandatory` at all, so this narrows rather
 * than replaces the old behaviour as real data becomes available.
 */
export function isVariableRequired(variable: BeCatalogItemVariable): boolean {
  if (typeof variable.mandatory === "boolean") return variable.mandatory;
  return !isAttachmentField(variable) && !isFileCopyPathField(variable);
}

/**
 * First empty required field label, or null if all are filled. Required-ness
 * comes from {@link isVariableRequired}; attachment fields are always
 * skipped here regardless of `mandatory` since they're collected by the
 * page's separate attachments section, not this check.
 */
export function getFirstEmptyRequiredField(
  variables: BeCatalogItemVariable[],
  values: Record<string, string>,
): string | null {
  for (const v of getUserEditableVariables(variables)) {
    if (isAttachmentField(v) || !isVariableRequired(v)) continue;
    const raw = (values[v.id] ?? "").trim();
    const textContent = raw.replace(/<[^>]+>/g, "").trim();
    if (!textContent) return variableLabel(v);
  }
  return null;
}

/**
 * First field whose current value exceeds the backend-declared `maxLength`,
 * or null if none do. Description (rich text) and datetime fields are
 * skipped — `maxLength` is documented as a single-line-text attribute
 * (`CHANGES-sr-variable-metadata.md`) and HTML markup would make a raw
 * character-count comparison meaningless for the former.
 */
export function getFirstFieldExceedingMaxLength(
  variables: BeCatalogItemVariable[],
  values: Record<string, string>,
): { label: string; maxLength: number } | null {
  for (const v of getUserEditableVariables(variables)) {
    if (!v.maxLength || isDescriptionField(v.questionText ?? "") || isDateTimeField(v)) continue;
    const raw = values[v.id] ?? "";
    if (raw.length > v.maxLength) return { label: variableLabel(v), maxLength: v.maxLength };
  }
  return null;
}

/**
 * First field whose current (non-empty) value fails its declared
 * `validation.regex`, or null if all pass. Empty values are left to
 * {@link getFirstEmptyRequiredField} — a required-but-empty field shouldn't
 * also report a pattern mismatch.
 */
export function getFirstFieldFailingValidation(
  variables: BeCatalogItemVariable[],
  values: Record<string, string>,
): { label: string; message: string } | null {
  for (const v of getUserEditableVariables(variables)) {
    if (!v.validation?.regex) continue;
    const raw = (values[v.id] ?? "").trim();
    if (!raw) continue;
    let matches: boolean;
    try {
      matches = new RegExp(v.validation.regex).test(raw);
    } catch {
      // A malformed regex from the backend must not brick the form.
      continue;
    }
    if (!matches) return { label: variableLabel(v), message: v.validation.message };
  }
  return null;
}

/**
 * Error when a catalog item has both a "Start Date"/"Start Time" field and an
 * "End Date"/"End Time" field and the current values put the end at or before
 * the start — an impossible window (e.g. a log-retrieval request) that
 * shouldn't be submittable. Only fires once both fields have a value; an
 * empty one is left to {@link getFirstEmptyRequiredField} instead of also
 * reporting a window error on nothing. Values are the "YYYY-MM-DDTHH:MM"
 * strings the datetime-local picker stores (`formatDateTimeLocal`), which
 * compare correctly with a plain string `>=` since every field is fixed-width.
 */
export function getDateTimeWindowError(
  variables: BeCatalogItemVariable[],
  values: Record<string, string>,
): string | null {
  const editable = getUserEditableVariables(variables);
  // Every Start field is checked against its own matching End field, not
  // just the first Start/End pair found — a catalog item can carry more
  // than one pair (e.g. Start Date/End Date alongside Start Time/End Time),
  // and an invalid second pair must not be hidden behind a valid first one.
  for (const startField of editable.filter(isStartDateTimeField)) {
    const endField = matchingEndDateTimeField(editable, startField);
    if (!endField) continue;
    const startVal = values[startField.id] ?? "";
    const endVal = values[endField.id] ?? "";
    if (startVal && endVal && startVal >= endVal) {
      return "Start time must be before end time.";
    }
  }
  return null;
}

/**
 * Encode a variable's raw input into the value the payload carries:
 * description stays HTML, datetime becomes an ISO-UTC string, everything else
 * is reduced to trimmed plain text.
 */
export function encodeVariableValue(
  variable: BeCatalogItemVariable,
  raw: string,
): string {
  const trimmed = (raw ?? "").trim();
  if (!trimmed) return "";
  if (isDescriptionField(variable.questionText ?? "")) return trimmed;
  if (isDateTimeField(variable)) {
    const ms = Date.parse(trimmed);
    return Number.isNaN(ms) ? trimmed : new Date(ms).toISOString();
  }
  return htmlToPlainText(trimmed).trim();
}
