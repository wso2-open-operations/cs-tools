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

import { getMockCsmProjects } from "@features/csm-projects/api/mocks/projectsMocks";
import type { DashboardScope } from "@features/csm-dashboard/types/abtDashboard";
import type {
  CsmAccountRow,
  CsmAccountsListResponse,
} from "@features/csm-accounts/types/csmAccounts";
import type {
  CsmProjectRow,
  CsmProjectStatus,
  CsmProjectTier,
} from "@features/csm-projects/types/csmProjects";

const TIER_RANK: Record<CsmProjectTier, number> = {
  Platinum: 3,
  Gold: 2,
  Silver: 1,
  Bronze: 0,
};

function pickWorstStatus(statuses: CsmProjectStatus[]): CsmProjectStatus {
  if (statuses.includes("Suspended")) return "Suspended";
  if (statuses.includes("Onboarding")) return "Onboarding";
  return "Active";
}

function aggregateAccounts(projects: CsmProjectRow[]): CsmAccountRow[] {
  const byAccount = new Map<
    string,
    {
      id: string;
      name: string;
      tier: CsmProjectTier;
      statuses: CsmProjectStatus[];
      projectCount: number;
      openCaseCount: number;
      s0s1Count: number;
      breachedCount: number;
      lastActivityAt: string;
    }
  >();

  for (const p of projects) {
    let bucket = byAccount.get(p.accountId);
    if (!bucket) {
      bucket = {
        id: p.accountId,
        name: p.customer,
        tier: p.tier,
        statuses: [],
        projectCount: 0,
        openCaseCount: 0,
        s0s1Count: 0,
        breachedCount: 0,
        lastActivityAt: p.lastActivityAt,
      };
      byAccount.set(p.accountId, bucket);
    }
    bucket.projectCount += 1;
    bucket.openCaseCount += p.openCaseCount;
    bucket.s0s1Count += p.s0s1Count;
    bucket.breachedCount += p.breachedCount;
    if (TIER_RANK[p.tier] > TIER_RANK[bucket.tier]) bucket.tier = p.tier;
    if (p.lastActivityAt > bucket.lastActivityAt) {
      bucket.lastActivityAt = p.lastActivityAt;
    }
    bucket.statuses.push(p.status);
  }

  return Array.from(byAccount.values())
    .map((b) => ({
      id: b.id,
      name: b.name,
      tier: b.tier,
      status: pickWorstStatus(b.statuses),
      projectCount: b.projectCount,
      openCaseCount: b.openCaseCount,
      s0s1Count: b.s0s1Count,
      breachedCount: b.breachedCount,
      lastActivityAt: b.lastActivityAt,
    }))
    .sort((a, b) => a.name.localeCompare(b.name));
}

export function getMockCsmAccounts(
  scope: DashboardScope,
): CsmAccountsListResponse {
  const projects = getMockCsmProjects(scope).projects;
  return { scope, accounts: aggregateAccounts(projects) };
}

export function getMockCsmAccountById(
  accountId: string,
): CsmAccountRow | undefined {
  // Use the broadest scope so an account is still findable when navigated
  // directly via URL even if it falls outside the user's ABT.
  const accounts = getMockCsmAccounts("all_customers").accounts;
  return accounts.find((a) => a.id === accountId);
}

export function getMockProjectsForAccount(
  accountId: string,
): CsmProjectRow[] {
  return getMockCsmProjects("all_customers").projects.filter(
    (p) => p.accountId === accountId,
  );
}
