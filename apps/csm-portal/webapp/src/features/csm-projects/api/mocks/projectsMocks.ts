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

import { getMockCsmCases } from "@features/csm-cases/api/mocks/casesMocks";
import type { DashboardScope } from "@features/csm-dashboard/types/abtDashboard";
import type {
  CsmProjectRow,
  CsmProjectsListResponse,
  CsmProjectStatus,
  CsmProjectTier,
} from "@features/csm-projects/types/csmProjects";

interface ProjectMetaSeed {
  tier: CsmProjectTier;
  productType: string;
  status: CsmProjectStatus;
  updateLevel: string;
}

const PROJECT_META: Record<string, ProjectMetaSeed> = {
  "prj-acme-iam-prod": {
    tier: "Platinum",
    productType: "Identity Server",
    status: "Active",
    updateLevel: "IS-7.0.0 / U231",
  },
  "prj-acme-openbanking": {
    tier: "Platinum",
    productType: "Open Banking",
    status: "Active",
    updateLevel: "OB-4.0.0 / U118",
  },
  "prj-initech-apim": {
    tier: "Platinum",
    productType: "API Manager",
    status: "Active",
    updateLevel: "APIM-4.3.0 / U92",
  },
  "prj-initech-mi": {
    tier: "Platinum",
    productType: "Micro Integrator",
    status: "Active",
    updateLevel: "MI-4.3.0 / U67",
  },
  "prj-initech-si": {
    tier: "Platinum",
    productType: "Streaming Integrator",
    status: "Active",
    updateLevel: "SI-4.2.0 / U43",
  },
  "prj-initech-choreo": {
    tier: "Platinum",
    productType: "Choreo",
    status: "Active",
    updateLevel: "Choreo Cloud",
  },
  "prj-umbrella-choreo": {
    tier: "Gold",
    productType: "Choreo",
    status: "Active",
    updateLevel: "Choreo Cloud",
  },
  "prj-umbrella-asgardeo": {
    tier: "Gold",
    productType: "Asgardeo",
    status: "Active",
    updateLevel: "Asgardeo Cloud",
  },
  "prj-globex-choreo": {
    tier: "Gold",
    productType: "Choreo",
    status: "Active",
    updateLevel: "Choreo Cloud",
  },
  "prj-globex-iam": {
    tier: "Gold",
    productType: "Identity Server",
    status: "Active",
    updateLevel: "IS-6.1.0 / U195",
  },
  "prj-soylent-apim": {
    tier: "Silver",
    productType: "API Manager",
    status: "Active",
    updateLevel: "APIM-4.2.0 / U88",
  },
  "prj-soylent-iam": {
    tier: "Silver",
    productType: "Identity Server",
    status: "Onboarding",
    updateLevel: "IS-7.0.0 / U231",
  },
  // All-customer extras (only visible at scope=all_customers via the cases mocks)
  "prj-wayne-openbanking": {
    tier: "Platinum",
    productType: "Open Banking",
    status: "Active",
    updateLevel: "OB-4.0.0 / U118",
  },
  "prj-stark-choreo": {
    tier: "Gold",
    productType: "Choreo",
    status: "Active",
    updateLevel: "Choreo Cloud",
  },
  "prj-stark-iam": {
    tier: "Gold",
    productType: "Identity Server",
    status: "Active",
    updateLevel: "IS-7.0.0 / U231",
  },
  "prj-tyrell-apim": {
    tier: "Silver",
    productType: "API Manager",
    status: "Suspended",
    updateLevel: "APIM-4.1.0 / U54",
  },
};

const FALLBACK_META: ProjectMetaSeed = {
  tier: "Silver",
  productType: "—",
  status: "Active",
  updateLevel: "—",
};

interface Aggregates {
  openCaseCount: number;
  s0s1Count: number;
  breachedCount: number;
  lastActivityAt: string;
}

function projectAggregatesFromCases(
  scope: DashboardScope,
): Map<string, Aggregates & { name: string; customer: string; accountId: string }> {
  const cases = getMockCsmCases(scope).cases;
  const byProject = new Map<
    string,
    Aggregates & { name: string; customer: string; accountId: string }
  >();

  for (const c of cases) {
    let agg = byProject.get(c.projectId);
    if (!agg) {
      agg = {
        name: c.projectName,
        customer: c.customer,
        accountId: c.accountId,
        openCaseCount: 0,
        s0s1Count: 0,
        breachedCount: 0,
        lastActivityAt: c.updatedAt,
      };
      byProject.set(c.projectId, agg);
    }
    if (c.state !== "closed") agg.openCaseCount += 1;
    if ((c.severity === "S0" || c.severity === "S1") && c.state !== "closed") {
      agg.s0s1Count += 1;
    }
    if (c.minutesToBreach < 0 && c.state !== "closed") {
      agg.breachedCount += 1;
    }
    if (c.updatedAt > agg.lastActivityAt) {
      agg.lastActivityAt = c.updatedAt;
    }
  }

  return byProject;
}

/**
 * Look up a single project by id. Uses the broadest scope so direct URL
 * navigation works even when the project sits outside the engineer's ABT.
 */
export function getMockCsmProjectById(
  projectId: string,
): CsmProjectRow | undefined {
  return getMockCsmProjects("all_customers").projects.find(
    (p) => p.id === projectId,
  );
}

export function getMockCsmProjects(
  scope: DashboardScope,
): CsmProjectsListResponse {
  const aggregates = projectAggregatesFromCases(scope);
  const projects: CsmProjectRow[] = [];

  for (const [projectId, agg] of aggregates) {
    const meta = PROJECT_META[projectId] ?? FALLBACK_META;
    projects.push({
      id: projectId,
      name: agg.name,
      customer: agg.customer,
      accountId: agg.accountId,
      tier: meta.tier,
      productType: meta.productType,
      status: meta.status,
      updateLevel: meta.updateLevel,
      openCaseCount: agg.openCaseCount,
      s0s1Count: agg.s0s1Count,
      breachedCount: agg.breachedCount,
      lastActivityAt: agg.lastActivityAt,
    });
  }

  // Customer A → Z, then project name A → Z for stable display order.
  projects.sort((a, b) => {
    const byCustomer = a.customer.localeCompare(b.customer);
    return byCustomer !== 0 ? byCustomer : a.name.localeCompare(b.name);
  });

  return { scope, projects };
}
