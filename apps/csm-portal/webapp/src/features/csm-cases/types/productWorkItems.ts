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

/** Wire shapes of `GET /cases/{id}/product-work-items` and
 * `GET /product-work-items/{id}` (entity-service `ProductIssueSummary` /
 * `ProductIssueDetail`). */

export type ProductBoardStatus =
  | "OPEN"
  | "IN_PROGRESS"
  | "WAITING_ON_PRODUCT_TEAM"
  | "WAITING_ON_CS"
  | "REOPENED"
  | "RESOLVED"
  | "CLOSED"
  | "OTHER";

export type ProductLinkState = "ACTIVE" | "INACCESSIBLE" | "DELETED" | "TRANSFERRED";

export interface ProductIssueGitHub {
  owner: string;
  repository: string;
  issueNumber: number;
  htmlUrl?: string | null;
  state: "OPEN" | "CLOSED";
  stateReason?: string | null;
  labels?: string[];
}

export type ProductIssueSlaState = "NONE" | "OK" | "AT_RISK" | "PAUSED" | "MET" | "BREACHED";

/** Resolution-clock standing of a product issue, computed by entity-service at
 * read time. Always present; `state` NONE means no clock (numbers are 0 and
 * `pausedSince` / `policyName` / `schedule` are null). */
export interface ProductIssueSla {
  state: ProductIssueSlaState;
  budgetSeconds: number;
  consumedSeconds: number;
  remainingSeconds: number;
  percentage: number;
  running: boolean;
  pausedSince?: string | null;
  priority?: string | null;
  policyName?: string | null;
  schedule?: string | null;
}

export interface ProductIssueSummary {
  id: string;
  number: string;
  title: string;
  github: ProductIssueGitHub;
  boardStatus?: ProductBoardStatus | null;
  boardStatusRaw?: string | null;
  priority?: string | null;
  category?: string | null;
  case?: { id: string; number: string } | null;
  linkSource?: "CREATION" | "MANUAL" | "BACKFILL" | null;
  linkedOn?: string | null;
  lastActivityOn?: string | null;
  linkState: ProductLinkState;
  sla?: ProductIssueSla | null;
  /** True when the issue sits in a board column outside the standard set
   * (`boardStatus` OTHER; `boardStatusRaw` is the column name). */
  nonStandardColumn?: boolean;
}

export interface ProductIssueListResponse {
  items?: ProductIssueSummary[];
}

export type ProductIssueSearchSort = "created" | "pausedSince" | "slaPercentage";

/** Body of `POST /product-work-items/search`. */
export interface ProductIssueSearchRequest {
  filters: {
    boardStatuses?: ProductBoardStatus[];
    mine?: boolean;
    sort?: ProductIssueSearchSort;
  };
  pagination?: { limit: number; offset: number };
}

export interface ProductIssueSearchResponse {
  items?: ProductIssueSummary[];
  total?: number;
  limit?: number;
  offset?: number;
}

export interface ProductIssueComment {
  id: string;
  type?: string | null;
  author: string;
  body: string;
  createdOn: string;
  origin?: "PORTAL" | "GITHUB" | null;
  githubCommentId?: number | null;
  authorLogin?: string | null;
  /** Portal-origin comments only: whether GitHub has it yet. null for a GitHub comment. */
  delivery?: "PENDING" | "DELIVERED" | "FAILED" | "CANCELLED" | null;
  commandId?: string | null;
}

export type ProductCommandKind = "ADD_COMMENT" | "SET_BOARD_STATUS" | "SET_STATE";
export type ProductCommandStatus = "PENDING" | "DELIVERED" | "FAILED" | "CANCELLED";

/** A queued write to GitHub (`GitHubCommand` in entity-service). */
export interface ProductGitHubCommand {
  id: string;
  kind: ProductCommandKind;
  status: ProductCommandStatus;
  attempts: number;
  nextAttemptOn?: string | null;
  lastError?: string | null;
  requestedBy: string;
  createdOn: string;
  deliveredOn?: string | null;
  commentId?: string | null;
  targetStatus?: string | null;
}

export interface ProductAddCommentResponse {
  comment: ProductIssueComment;
  command: ProductGitHubCommand;
}

export interface ProductCommandResponse {
  command: ProductGitHubCommand;
}

export interface ProductIssueStatusEvent {
  id: string;
  fromStatus?: string | null;
  toStatus: string;
  waitsOn?: "CS" | "PRODUCT_TEAM" | "CUSTOMER" | "NOBODY";
  occurredOn: string;
  actor?: string | null;
  source: "WEBHOOK" | "RECONCILE" | "BACKFILL" | "PORTAL";
  rawStatus?: string | null;
}

export interface ProductIssueDetail extends ProductIssueSummary {
  openedByEmail?: string | null;
  abtTeam?: string | null;
  publicIssueUrl?: string | null;
  createdOn?: string;
  lastSyncedOn?: string | null;
  conversation?: ProductIssueComment[];
  conversationTruncated?: boolean;
  statusHistory?: ProductIssueStatusEvent[];
  /** Every PENDING or FAILED command plus the 10 newest others, newest first. */
  commands?: ProductGitHubCommand[];
}
