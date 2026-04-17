// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License. You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

import type { CatalogItemVariable } from "@features/operations/types/serviceRequests";

/** Attachment/File type variants from API (case-insensitive). */
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

/** Detect attachment fields by questionText when API type is ambiguous (e.g. "Single Line Text"). */
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

/** True if variable is the Description field (uses rich text editor; value should be sent as HTML). */
export function isDescriptionField(questionText: string): boolean {
  const normalized = (questionText ?? "")
    .replace(/^\s*\*?\s*/, "")
    .trim()
    .toLowerCase();
  return normalized === "description";
}

/** True if variable is a File Copy Path field (use text input instead of upload). */
export function isFileCopyPathField(variable: CatalogItemVariable): boolean {
  const q = (variable.questionText ?? "").trim().toLowerCase();
  const t = (variable.type ?? "").trim().toLowerCase();
  return (
    /file\s*copy\s*path/i.test(q) ||
    /file\s*copy\s*path/i.test(t)
  );
}

/** True if variable is an attachment/file upload field (by type or questionText). */
export function isAttachmentField(variable: CatalogItemVariable): boolean {
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

export function isContextField(
  questionText: string,
  _contextValues?: { projectDisplay: string; deploymentDisplay: string; productDisplay: string },
): boolean {
  const normalized = questionText?.trim().toLowerCase() ?? "";
  return CONTEXT_FIELD_PATTERNS.some((p) => p.test(normalized));
}

function isHiddenField(questionText: string): boolean {
  const normalized = questionText?.trim().toLowerCase() ?? "";
  return HIDDEN_FIELD_PATTERNS.some((p) => p.test(normalized));
}

/**
 * Returns user-editable variables (excludes context and hidden).
 * Hot fix: all typable fields are mandatory.
 */
export function getUserEditableVariables(
  variables: CatalogItemVariable[],
  contextValues: {
    projectDisplay: string;
    deploymentDisplay: string;
    productDisplay: string;
  },
): CatalogItemVariable[] {
  return variables.filter(
    (v) =>
      !isContextField(v.questionText ?? "", contextValues) &&
      !isHiddenField(v.questionText ?? ""),
  );
}

/**
 * Returns the first empty required field label, or null if all filled.
 * Attachment variables are optional and are skipped.
 */
export function getFirstEmptyRequiredField(
  variables: CatalogItemVariable[],
  contextValues: {
    projectDisplay: string;
    deploymentDisplay: string;
    productDisplay: string;
  },
  variableValues: Record<string, string>,
): string | null {
  const userEditable = getUserEditableVariables(variables, contextValues);
  for (const v of userEditable) {
    if (isAttachmentField(v) || isFileCopyPathField(v)) continue; // Attachments and File Copy Path are optional
    const raw = (variableValues[v.id] ?? "").trim();
    const textContent = raw.replace(/<[^>]+>/g, "").trim();
    if (!textContent) {
      const label = (v.questionText ?? "").replace(/^\s*\*?\s*/, "").trim() || "Field";
      return label;
    }
  }
  return null;
}
