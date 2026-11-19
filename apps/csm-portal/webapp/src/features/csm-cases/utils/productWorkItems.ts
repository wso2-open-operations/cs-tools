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

import type {
  ProductGitHubCommand,
  ProductIssueSla,
  ProductIssueSlaState,
  ProductIssueSummary,
  ProductLinkState,
} from "@features/csm-cases/types/productWorkItems";

export type ProductStatusColor = "default" | "info" | "warning" | "success" | "error";

const BOARD_STATUS_LABELS: Record<string, string> = {
  OPEN: "Open",
  IN_PROGRESS: "In progress",
  WAITING_ON_PRODUCT_TEAM: "Waiting on product team",
  WAITING_ON_CS: "Waiting on CS",
  REOPENED: "Reopened",
  RESOLVED: "Resolved",
  CLOSED: "Closed",
};

/** Readable status of a product issue: the board status when there is one
 * (`OTHER` shows the raw column name), otherwise the GitHub open/closed state. */
export function productIssueStatusLabel(
  item: Pick<ProductIssueSummary, "boardStatus" | "boardStatusRaw" | "github">,
): string {
  const board = item.boardStatus;
  if (board === "OTHER") return item.boardStatusRaw?.trim() || "Other";
  if (board && BOARD_STATUS_LABELS[board]) return BOARD_STATUS_LABELS[board];
  return item.github.state === "CLOSED" ? "Closed" : "Open";
}

export function productIssueStatusColor(
  item: Pick<ProductIssueSummary, "boardStatus" | "github">,
): ProductStatusColor {
  switch (item.boardStatus) {
    case "IN_PROGRESS":
      return "info";
    case "WAITING_ON_PRODUCT_TEAM":
    case "WAITING_ON_CS":
    case "REOPENED":
      return "warning";
    case "RESOLVED":
    case "CLOSED":
      return "success";
    case "OPEN":
    case "OTHER":
      return "default";
    default:
      return item.github.state === "CLOSED" ? "success" : "default";
  }
}

const LINK_STATE_LABELS: Record<string, string> = {
  INACCESSIBLE: "No longer accessible",
  DELETED: "Deleted on GitHub",
  TRANSFERRED: "Transferred to another repository",
};

/** Label for a link that is not ACTIVE; null for an active link. */
export function productLinkStateLabel(state: ProductLinkState | undefined): string | null {
  if (!state || state === "ACTIVE") return null;
  return LINK_STATE_LABELS[state] ?? state;
}

/** Display name of a status-history value (board status key or raw column). */
export function productHistoryStatusLabel(value: string | null | undefined): string {
  if (!value) return "—";
  return BOARD_STATUS_LABELS[value] ?? value;
}

const SOURCE_LABELS: Record<string, string> = {
  WEBHOOK: "GitHub webhook",
  RECONCILE: "Reconciliation",
  BACKFILL: "Backfill",
  PORTAL: "Portal",
};

export function productHistorySourceLabel(source: string): string {
  return SOURCE_LABELS[source] ?? source;
}

const SLA_STATE_LABELS: Record<ProductIssueSlaState, string> = {
  NONE: "No SLA",
  OK: "OK",
  AT_RISK: "At risk",
  PAUSED: "Paused (waiting on CS)",
  MET: "Met",
  BREACHED: "Breached",
};

// Same colours the case page's SLA table uses: running info, paused warning,
// completed success, breached error; at risk (>= 75% consumed) is warning there too.
const SLA_STATE_ROLES: Record<ProductIssueSlaState, ProductStatusColor> = {
  NONE: "default",
  OK: "info",
  AT_RISK: "warning",
  PAUSED: "warning",
  MET: "success",
  BREACHED: "error",
};

export function productSlaLabel(state: ProductIssueSlaState | undefined): string {
  return (state && SLA_STATE_LABELS[state]) || SLA_STATE_LABELS.NONE;
}

export function productSlaRole(state: ProductIssueSlaState | undefined): ProductStatusColor {
  return (state && SLA_STATE_ROLES[state]) || "default";
}

/** Whole seconds as "10h", "1h 30m", "45m"; under a minute is "0m". */
export function formatSlaDuration(seconds: number): string {
  const total = Math.max(0, Math.floor(seconds));
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  if (h > 0 && m > 0) return `${h}h ${m}m`;
  if (h > 0) return `${h}h`;
  return `${m}m`;
}

/** "10h of 24h", plus ", business hours, IST" for the 12x5 schedule. Null for NONE. */
export function productSlaConsumedText(sla: ProductIssueSla | null | undefined): string | null {
  if (!sla || sla.state === "NONE") return null;
  const base = `${formatSlaDuration(sla.consumedSeconds)} of ${formatSlaDuration(sla.budgetSeconds)}`;
  return sla.schedule === "CS 12x5 IST" ? `${base} (business hours, IST)` : base;
}

/** How long after GitHub accepted a move/close we still wait for its webhook to
 * change the stored state before offering the action again. */
export const AWAITING_GITHUB_GRACE_MS = 2 * 60 * 1000;

/** A move/close delivered to GitHub whose confirmation has not come back yet. */
export function isAwaitingGithub(
  command: Pick<ProductGitHubCommand, "kind" | "status" | "deliveredOn">,
  now: number,
): boolean {
  if (command.status !== "DELIVERED" || !command.deliveredOn) return false;
  if (command.kind !== "SET_BOARD_STATUS" && command.kind !== "SET_STATE") return false;
  const at = Date.parse(command.deliveredOn);
  return !Number.isNaN(at) && now - at < AWAITING_GITHUB_GRACE_MS;
}

/** A request id that also works outside a secure context, where
 * `crypto.randomUUID` is missing. */
export function newRequestId(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    try {
      return crypto.randomUUID();
    } catch {
      /* fall through */
    }
  }
  const bytes = new Uint8Array(16);
  if (typeof crypto !== "undefined" && typeof crypto.getRandomValues === "function") {
    crypto.getRandomValues(bytes);
  } else {
    for (let i = 0; i < bytes.length; i++) bytes[i] = Math.floor(Math.random() * 256);
  }
  bytes[6] = (bytes[6] & 0x0f) | 0x40;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;
  const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}

/** True when the server (or gateway) answered; false for a network failure,
 * where the same idempotency key must be kept so a true retry dedupes. */
export function requestWasAnswered(error: unknown): boolean {
  return typeof (error as { status?: unknown } | null)?.status === "number";
}
