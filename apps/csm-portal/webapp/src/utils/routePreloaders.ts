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

/**
 * Eagerly starts downloading the JS chunk(s) for a route section when the user
 * hovers its sidebar link or a row that leads to it. By the time the click
 * registers the chunk is already in-flight (or cached), eliminating the
 * first-navigation pause that comes from React.lazy on-demand loading.
 *
 * Each key is a path prefix matching one of the CSM_NAV_ITEMS paths.
 */
const PRELOADERS: Record<string, () => void> = {
  "/dashboard": () => {
    void import("@features/csm-dashboard/pages/CsmDashboardPage");
  },
  "/cases": () => {
    void import("@features/csm-cases/pages/CsmCasesPage");
    void import("@features/csm-cases/pages/CsmCaseDetailPage");
    void import("@features/csm-cases/pages/CsmCaseCreatePage");
  },
  "/operations": () => {
    void import("@features/csm-operations/pages/OperationsPage");
    void import("@features/csm-operations/pages/CsmChangeRequestDetailPage");
    void import("@features/csm-operations/pages/CreateServiceRequestPage");
  },
  "/security-center": () => {
    void import("@features/csm-security-center/pages/CsmSecurityCenterPage");
    void import("@features/csm-security-center/pages/ProductVulnerabilityDetailPage");
    void import("@features/csm-security-center/pages/CreateSecurityReportPage");
  },
  "/updates": () => {
    void import("@features/updates/pages/CsmUpdatesPage");
  },
  "/time-cards": () => {
    void import("@features/csm-timecards/pages/CsmTimeCardsPage");
  },
  "/customers": () => {
    void import("@features/csm-customers/pages/CsmCustomersLayout");
    void import("@features/csm-accounts/pages/CsmAccountsPage");
    void import("@features/csm-accounts/pages/CsmAccountDetailPage");
    void import("@features/csm-projects/pages/CsmProjectsPage");
    void import("@features/csm-projects/pages/CsmProjectDetailPage");
  },
  "/admin": () => {
    void import("@features/csm-admin/pages/CsmAdminLayout");
    void import("@features/csm-users/pages/CsmUsersPage");
  },
  "/engagements": () => {
    void import("@features/csm-engagements/pages/CsmEngagementsPage");
  },
};

/**
 * Call this on mouseenter of any link/row that navigates to `path`.
 * Fires the matching chunk download; safe to call multiple times (the browser
 * deduplicates in-flight requests and caches completed ones).
 */
export function preloadRoute(path: string): void {
  const match = Object.keys(PRELOADERS)
    .filter((prefix) => path === prefix || path.startsWith(`${prefix}/`))
    .sort((a, b) => b.length - a.length)[0];
  if (match) PRELOADERS[match]();
}
