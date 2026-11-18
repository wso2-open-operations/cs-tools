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

import type { ProductIssueSummary, ProductLinkState } from "@features/csm-cases/types/productWorkItems";

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
