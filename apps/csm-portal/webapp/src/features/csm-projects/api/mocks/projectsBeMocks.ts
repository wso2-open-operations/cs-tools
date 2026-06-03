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
import type {
  Project,
  SearchProjectsRequest,
  SearchProjectsResponse,
  SubscriptionType,
} from "@features/csm-projects/types/csmProjects";

const PROJECT_BASE = new Date("2023-01-01T00:00:00Z").getTime();
const YEAR_MS = 365 * 86_400_000;

function hashOf(s: string): number {
  let h = 0;
  for (let i = 0; i < s.length; i += 1) h = (h * 31 + s.charCodeAt(i)) | 0;
  return Math.abs(h);
}

function startDateFor(id: string): string {
  return new Date(PROJECT_BASE + (hashOf(id) % 600) * 86_400_000).toISOString();
}

function endDateFor(id: string): string {
  return new Date(
    PROJECT_BASE + (hashOf(id) % 600) * 86_400_000 + 3 * YEAR_MS,
  ).toISOString();
}

function projectKeyFor(id: string, name: string): string {
  const base = name
    .split(/\s+/)
    .map((p) => p[0]?.toUpperCase() ?? "")
    .join("")
    .slice(0, 4);
  const tail = id.replace(/^prj-/, "").slice(0, 6).toUpperCase();
  return `${base || "PRJ"}-${tail}`;
}

function subscriptionFor(productType: string): SubscriptionType {
  const p = productType.toLowerCase();
  if (p.includes("choreo")) return "cloud_support";
  if (p.includes("asgardeo")) return "cloud_support";
  if (p.includes("evaluation")) return "evaluation_subscription";
  return "subscription";
}

function buildBeProjects(): Project[] {
  const rows = getMockCsmProjects("all_customers").projects;
  return rows.map((r) => ({
    id: r.id,
    accountId: r.accountId,
    sfId: `SF-${r.id.replace(/^prj-/, "").slice(0, 8).toUpperCase()}`,
    name: r.name,
    projectKey: projectKeyFor(r.id, r.name),
    subscriptionType: subscriptionFor(r.productType),
    startDate: startDateFor(r.id),
    endDate: endDateFor(r.id),
    createdAt: startDateFor(r.id),
    updatedAt: r.lastActivityAt,
  }));
}

function matchesQuery(p: Project, q: string): boolean {
  const needle = q.toLowerCase();
  return (
    p.name.toLowerCase().includes(needle) ||
    p.projectKey.toLowerCase().includes(needle) ||
    p.subscriptionType.toLowerCase().includes(needle)
  );
}

export function getMockBackendProjectsResponse(
  req: SearchProjectsRequest,
): SearchProjectsResponse {
  const all = buildBeProjects();
  const q = (req.searchQuery ?? "").trim();
  const filtered = q ? all.filter((p) => matchesQuery(p, q)) : all;
  const offset = req.pagination?.offset ?? 0;
  const limit = req.pagination?.limit ?? 20;
  const page = filtered.slice(offset, offset + limit);
  return {
    projects: page,
    total: filtered.length,
    limit,
    offset,
    hasMore: offset + page.length < filtered.length,
  };
}
