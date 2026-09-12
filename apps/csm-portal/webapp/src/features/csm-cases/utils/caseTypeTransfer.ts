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

import type { BeCaseType } from "@api/backend/types";
import { CASE_TYPE_LABEL } from "@features/csm-cases/utils/caseType";

/**
 * The 4 case types a case can be transferred between.
 * `announcement` is deliberately excluded — it's system-managed (produced by
 * the security-advisory automation writing directly to the backing data
 * source, see the create-payload validator on the entity-service side), with
 * no create path and no reason a case would ever need to become or stop
 * being one.
 */
export const TRANSFERABLE_CASE_TYPES: readonly BeCaseType[] = [
  "case",
  "security_report_analysis",
  "engagement",
  "service_request",
];

/**
 * Transfer targets the backend accepts. All four, as of the entity-service
 * validator accepting every type on update and the backing data source's
 * support for a mid-life catalog-item assignment being confirmed — the two
 * preconditions that previously kept `service_request` and
 * `security_report_analysis` visible-but-disabled here.
 *
 * Kept as a named export rather than inlined: it is the single place a target
 * gets withdrawn again if one ever needs to be.
 */
export const SUPPORTED_TRANSFER_TARGETS: readonly BeCaseType[] = TRANSFERABLE_CASE_TYPES;

/** One field this type collects beyond the shared fields every case has. */
export interface CaseTypeSpecificField {
  /** Stable identifier — not a wire field name, just a key for list rendering. */
  key: "severity" | "issueType" | "engagementType";
  label: string;
  /** One-line reason shown next to the field when it's about to be lost. */
  lostReason: string;
}

interface CaseTypeMeta {
  specificFields: CaseTypeSpecificField[];
  /** `security_report_analysis` requires at least one attachment. */
  requiresAttachment?: boolean;
  /** `service_request` is driven by a ServiceNow catalog item + its answers,
   * not a fixed field list — rendered as its own picker, not a specific field. */
  requiresCatalog?: boolean;
}

const TYPE_META: Partial<Record<BeCaseType, CaseTypeMeta>> = {
  case: {
    specificFields: [
      {
        key: "severity",
        label: "Severity",
        lostReason: "Query cases are triaged S0–S4; other types have no severity concept.",
      },
      {
        key: "issueType",
        label: "Issue type",
        lostReason: "Issue type only classifies plain support queries.",
      },
    ],
  },
  security_report_analysis: {
    specificFields: [],
    requiresAttachment: true,
  },
  engagement: {
    specificFields: [
      {
        key: "engagementType",
        label: "Engagement type",
        lostReason: "Engagements are categorized by type instead of severity.",
      },
    ],
  },
  service_request: {
    specificFields: [],
    requiresCatalog: true,
  },
};

/** Fields every case type carries regardless of type — always retained across a transfer. */
export const ALWAYS_RETAINED_FIELDS: string[] = [
  "Subject",
  "Description",
  "Project, deployment, and product",
  "Watchers",
  "Tags",
  "Comments and activity history",
  "Existing attachments",
];

export interface TransferPreview {
  /** The source type's own fields that don't carry over to the target type. */
  lostFields: CaseTypeSpecificField[];
  /** The target type's own fields not already satisfied by the source case. */
  neededFields: CaseTypeSpecificField[];
  /** Target requires an attachment and the case currently has none. */
  attachmentNeeded: boolean;
  /** Target is driven by a ServiceNow catalog item, not a fixed field. */
  catalogNeeded: boolean;
}

/**
 * Computes what's retained, lost, and newly required when transferring a
 * case from `from` to `to`. Table-driven off `TYPE_META` rather than
 * hardcoded per pair, so it covers all 12 ordered pairs among the 4
 * transferable types uniformly.
 *
 * + from - The case's current type
 * + to - The type being transferred to
 * + hasAttachments - Whether the case already has at least one attachment
 * + return - What's lost from `from` and what's newly needed for `to`
 */
export function computeTransferPreview(
  from: BeCaseType,
  to: BeCaseType,
  hasAttachments: boolean,
): TransferPreview {
  const fromMeta = TYPE_META[from] ?? { specificFields: [] };
  const toMeta = TYPE_META[to] ?? { specificFields: [] };

  return {
    lostFields: fromMeta.specificFields,
    neededFields: toMeta.specificFields,
    attachmentNeeded: !!toMeta.requiresAttachment && !hasAttachments,
    catalogNeeded: !!toMeta.requiresCatalog,
  };
}

/** Display label for a case type, falling back to the raw value for a type
 * this module doesn't otherwise know about (kept in sync with `ALL_CASE_TYPES`). */
export function caseTypeTransferLabel(type: BeCaseType): string {
  return CASE_TYPE_LABEL[type] ?? type;
}
