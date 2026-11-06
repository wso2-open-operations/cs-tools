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

import { severityFromBe } from "@api/backend/mappers";
import type { BeCaseType, BeResponseWarning } from "@api/backend/types";
import type { SeverityOrUnset } from "@features/csm-dashboard/types/abtDashboard";
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
    // A service request has its own (internal) severity, so a case moving to
    // one keeps its severity rather than losing it.
    lostFields:
      to === "service_request"
        ? fromMeta.specificFields.filter((f) => f.key !== "severity")
        : fromMeta.specificFields,
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

/**
 * What happens to the severity when a case is moved to a service request.
 * A service request holds S1-S4 only, so an S0 case is stored as S1 and a
 * source with no severity (including every non-case type) becomes S4. The
 * transfer never fails for this; the dialog tells the engineer up front.
 */
export interface SeverityTransferNote {
  kind: "downgrade" | "default";
  message: string;
}

/**
 * The severity note to show before confirming a transfer, or `null` when the
 * target is not a service request or the severity carries over unchanged.
 *
 * + to - The type being transferred to
 * + severity - The source case's severity (`"unset"` when it has none)
 * + return - The note to display, or null when there is nothing to warn about
 */
export function serviceRequestSeverityNote(
  to: BeCaseType,
  severity: SeverityOrUnset,
): SeverityTransferNote | null {
  if (to !== "service_request") return null;
  if (severity === "S0") {
    return {
      kind: "downgrade",
      message: "S0 is not available for service requests; severity will be set to S1.",
    };
  }
  if (severity === "unset") {
    return {
      kind: "default",
      message: "This case has no severity; the service request will be given S4.",
    };
  }
  return null;
}

/**
 * Turns the warnings on a successful transfer response into text for the
 * engineer, or `null` when there is nothing to show. A `SEVERITY_DOWNGRADED`
 * warning is rendered from its `from`/`to` values in the UI's S-scale; any
 * other code falls back to the backend's own `message`.
 *
 * + warnings - `warnings` from the PATCH response (may be absent)
 * + return - One sentence per warning joined by a space, or null when none
 */
export function describeTransferWarnings(
  warnings: BeResponseWarning[] | undefined,
): string | null {
  if (!warnings || warnings.length === 0) return null;
  const parts = warnings
    .map((w) => {
      if (w.code === "SEVERITY_DOWNGRADED") {
        const from = severityFromBe(w.from);
        const to = severityFromBe(w.to);
        if (from !== "unset" && to !== "unset") {
          return `${from} is not available for service requests; severity was set to ${to}.`;
        }
      }
      return w.message;
    })
    .filter((m) => !!m);
  return parts.length > 0 ? parts.join(" ") : null;
}
