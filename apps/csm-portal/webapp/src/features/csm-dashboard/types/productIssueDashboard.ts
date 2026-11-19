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

import type { ProductIssueSla } from "@features/csm-cases/types/productWorkItems";

/** Wire shape of `GET /product-work-items/dashboard` (entity-service
 * `ProductIssueDashboard`, passed through unchanged by the backend). */

export type ProductIssueDashboardBucket = "week" | "month";

export interface ProductIssueOpenBySla {
  ok: number;
  atRisk: number;
  breached: number;
  paused: number;
  none: number;
}

export interface ProductIssueDashboardTotals {
  open: number;
  openedInPeriod: number;
  resolvedInPeriod: number;
  waitingOnCs: number;
  waitingOnProductTeam: number;
  unmatchedOpen: number;
  nonStandardColumnOpen: number;
  openBySla: ProductIssueOpenBySla;
  slaMet: number;
  slaBreached: number;
  /** `null` when no clock finished in the period. */
  compliancePercentage: number | null;
}

export interface ProductIssueTrendBucket {
  start: string;
  end: string;
  opened: number;
  resolved: number;
}

export interface ProductIssueProductRow {
  /** `null`: no product mapped to the issue's repository ("Unmapped"). */
  productName: string | null;
  owner: string;
  repository: string;
  open: number;
  openedInPeriod: number;
  resolvedInPeriod: number;
  waitingOnCs: number;
  waitingOnProductTeam: number;
  openBySla: ProductIssueOpenBySla;
  slaMet: number;
  slaBreached: number;
  compliancePercentage: number | null;
}

export interface ProductIssueDashboardItem {
  id: string;
  number: string;
  title: string;
  productName: string | null;
  owner: string;
  repository: string;
  issueNumber: number;
  htmlUrl?: string | null;
  boardStatus?: string | null;
  priority?: string | null;
  openedOn: string;
  wocSince?: string | null;
  case?: { id: string; number: string } | null;
  openedByEmail?: string | null;
  sla: ProductIssueSla;
}

export interface ProductIssueWocBucket {
  label: string;
  count: number;
}

export interface ProductIssueBlockedCaseIssue {
  id: string;
  number: string;
  title: string;
  productName: string | null;
  boardStatus?: string | null;
  openedOn: string;
}

export interface ProductIssueBlockedCase {
  caseId: string;
  caseNumber: string;
  subject: string;
  issues: ProductIssueBlockedCaseIssue[];
  oldestOpenedOn: string;
}

export interface ProductIssueDashboard {
  periodStart: string;
  periodEnd: string;
  generatedOn: string;
  bucket: ProductIssueDashboardBucket;
  products: string[];
  totals: ProductIssueDashboardTotals;
  trend: ProductIssueTrendBucket[];
  byProduct: ProductIssueProductRow[];
  wocAgeing: {
    buckets: ProductIssueWocBucket[];
    oldest: ProductIssueDashboardItem[];
  };
  blockedCases: {
    total: number;
    byProduct: { productName: string | null; owner: string; repository: string; cases: number }[];
    cases: ProductIssueBlockedCase[];
  };
  breachedOpen: ProductIssueDashboardItem[];
}

/** Query of the dashboard request. `from`/`to` are `YYYY-MM-DD` (Asia/Kolkata,
 * `to` exclusive); `products` are product names, `Unmapped` for none. */
export interface ProductIssueDashboardParams {
  from: string;
  to: string;
  bucket: ProductIssueDashboardBucket;
  products: string[];
}
