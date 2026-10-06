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

// Ported verbatim from apps/support-portal-lite/webapp's own
// customerHealthTypes.ts — these field names are already verified
// against the real Go backend (internal/handler/spl_customer_health*.go,
// internal/risk/*.go, internal/servicenow/customer_health.go).

export interface AccountSummary {
  accountSysId: string;
  accountName: string | null;
  hasNoGoLive: { status: string; isRisk: boolean; state: string };
  hasRecentCases: boolean;
  hasEolProduct: boolean;
  hasAbandonedMigrations: boolean;
  hasMigrationDelays: boolean;
  hasRecentEscalations: boolean;
  noSupportCases6mo: boolean;
  healthStatus?: HealthStatusType;
}

export interface GoLiveStatus {
  status: string;
  isRisk: boolean;
}

export interface CaseInfo {
  sysId: string;
  number?: string;
}

export interface CaseGroup {
  priority: string;
  count: number;
  cases?: CaseInfo[];
}

export interface Deployment {
  sysId: string;
  name: string;
}

export interface EolProduct {
  name: string;
  eolDate: string;
  deployments: Deployment[];
}

export interface CaseLink {
  sysId?: string;
  number?: string;
  escalationSysId?: string;
  escalationNumber?: string;
}

export interface ProjectDetail {
  sysId: string;
  name: string;
  hasRecentCases: boolean;
  detailedRecentCases: CaseGroup[];
  totalRecentCases: number;
  hasAbandonedCases: boolean;
  detailedAbandonedCases: CaseLink[];
  isUsingEolProduct: boolean;
  softwareModel: EolProduct[];
  deployments?: Deployment[];
  hasMigrationDelays: boolean;
  detailedMigrationDelays: CaseLink[];
  hasEscalatedCases: boolean;
  detailedEscalatedCases: CaseLink[];
  goLiveStatus: GoLiveStatus;
}

export interface AccountDetail {
  accountName: string;
  customerProjects: ProjectDetail[];
}

export type RiskFilterKey =
  | "hasNoGoLive"
  | "noSupportCases6mo"
  | "hasEolProduct"
  | "hasAbandonedMigrations"
  | "hasMigrationDelays"
  | "hasRecentEscalations";

export type HealthStatusType = "to_be_reviewed" | "healthy" | "at_risk";

export interface RiskActionItem {
  id: number;
  riskId: number;
  projectSysId: string;
  accountSysId: string;
  title: string;
  description: string | null;
  priority: string;
  status: string;
  assignedToEmail: string | null;
  dueDate: string | null;
  resolutionComment: string | null;
  resolvedByEmail: string | null;
  resolvedOn: string | null;
  createdByEmail: string;
  createdOn: string;
  updatedOn: string;
  commentCount?: number;
}

export interface ActionItemComment {
  id: number;
  actionItemId: number;
  comment: string;
  createdByEmail: string;
  createdOn: string;
}

export interface ProjectRisk {
  id: number;
  projectSysId: string;
  accountSysId: string;
  status: string;
  openedComment: string;
  openedByEmail: string;
  openedOn: string;
  closedComment: string | null;
  closedByEmail?: string | null;
  closedOn?: string | null;
  actionItems: RiskActionItem[];
}

export interface HealthStatusRecord {
  id: number;
  projectSysId: string;
  accountSysId: string;
  status: HealthStatusType;
  reviewedByEmail: string | null;
  reviewedOn: string | null;
}

export interface ProjectHealthStatus {
  projectSysId: string;
  healthStatus: HealthStatusRecord;
  openRisk: ProjectRisk | null;
}

// Ported from constants/constants.tsx — the subset this domain needs.
export const TOOLTIP_TEXT = {
  goLive: "Risk if the project has not gone live within the expected timeframe (6 Months).",
  support: "Risk if the customer has not raised any support cases in the last 6 months.",
  eol: "Risk if the customer is currently using a software version that is End-of-Life.",
  abandoned: "Risk if there are abandoned product migration cases.",
  delays: "Risk if there are delayed migration cases.",
  escalations: "Risk if the account has more than 2 escalations in the last 3 months.",
  healthStatus:
    "Manually set from the account detail page. If any single project is marked At Risk, the account is considered At Risk.",
};

export const DEBOUNCE_DELAY = 500;
