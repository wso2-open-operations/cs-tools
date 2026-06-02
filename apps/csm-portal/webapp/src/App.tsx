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

import { type JSX } from "react";
import { Route, Routes, Navigate } from "react-router";
import AuthGuard from "@layouts/AuthGuard";
import ProjectGuard from "@layouts/ProjectGuard";
import ProjectDetailsPage from "@features/project-details/pages/ProjectDetails";
import DashboardPage from "@features/dashboard/pages/DashboardPage";
import DashboardItemsPage from "@features/dashboard/pages/DashboardItemsPage";
import CsmDashboardPage from "@features/csm-dashboard/pages/CsmDashboardPage";
import CsmCasesPage from "@features/csm-cases/pages/CsmCasesPage";
import CsmCaseDetailPage from "@features/csm-cases/pages/CsmCaseDetailPage";
import CsmProjectsPage from "@features/csm-projects/pages/CsmProjectsPage";
import CsmProjectDetailPage from "@features/csm-projects/pages/CsmProjectDetailPage";
import CsmAccountsPage from "@features/csm-accounts/pages/CsmAccountsPage";
import CsmAccountDetailPage from "@features/csm-accounts/pages/CsmAccountDetailPage";
import CsmAdminLayout from "@features/csm-admin/pages/CsmAdminLayout";
import CsmUsersPage from "@features/csm-users/pages/CsmUsersPage";
import CsmAdminUserDetailPage from "@features/csm-admin/pages/CsmAdminUserDetailPage";
import CsmAdminRolesPage from "@features/csm-admin/pages/CsmAdminRolesPage";
import CsmAdminRoleDetailPage from "@features/csm-admin/pages/CsmAdminRoleDetailPage";
import CsmAdminGroupsPage from "@features/csm-admin/pages/CsmAdminGroupsPage";
import CsmAdminGroupDetailPage from "@features/csm-admin/pages/CsmAdminGroupDetailPage";
import CsmAdminTeamsPage from "@features/csm-admin/pages/CsmAdminTeamsPage";
import CsmAdminSkillsPage from "@features/csm-admin/pages/CsmAdminSkillsPage";
import CsmAdminTemplatesPage from "@features/csm-admin/pages/CsmAdminTemplatesPage";
import CsmAdminCatalogPage from "@features/csm-admin/pages/CsmAdminCatalogPage";
import CsmAdminSlaPage from "@features/csm-admin/pages/CsmAdminSlaPage";
import CsmAdminReportsPage from "@features/csm-admin/pages/CsmAdminReportsPage";
import CsmAdminClosurePage from "@features/csm-admin/pages/CsmAdminClosurePage";
import CsmComingSoonPage from "@features/csm-coming-soon/pages/CsmComingSoonPage";
import SupportPage from "@features/support/pages/SupportPage";
import AllCasesPage from "@features/support/pages/AllCasesPage";
import CaseDetailsPage from "@features/support/pages/CaseDetailsPage";
import AllConversationsPage from "@features/support/pages/AllConversationsPage";
import ConversationDetailsPage from "@features/support/pages/ConversationDetailsPage";
import ServiceRequestsPage from "@features/operations/pages/ServiceRequestsPage";
import ServiceRequestDetailsPage from "@features/operations/pages/ServiceRequestDetailsPage";
import CreateServiceRequestPage from "@features/operations/pages/CreateServiceRequestPage";
import NoveraChatPage from "@features/support/pages/NoveraChatPage";
import DescribeIssuePage from "@features/support/pages/DescribeIssuePage";
import CreateCasePage from "@features/support/pages/CreateCasePage";
import ChangeRequestsPage from "@features/operations/pages/ChangeRequestsPage";
import ChangeRequestDetailsPage from "@features/operations/pages/ChangeRequestDetailsPage";
import CsmUpdatesPage from "@features/updates/pages/CsmUpdatesPage";
import UpdatesPage from "@features/updates/pages/UpdatesPage";
import PendingUpdatesPage from "@features/updates/pages/PendingUpdatesPage";
import UpdateLevelDetailsPage from "@features/updates/pages/UpdateLevelDetailsPage";
import AnnouncementsPage from "@features/announcements/pages/AnnouncementsPage";
import AnnouncementDetailsPage from "@features/announcements/pages/AnnouncementDetailsPage";
import OperationsPage from "@features/operations/pages/OperationsPage";
import SecurityPage from "@features/security/pages/SecurityPage";
import VulnerabilityDetailsPage from "@features/security/pages/VulnerabilityDetailsPage";
import EngagementsPage from "@features/engagements/pages/EngagementsPage";
import CsmEngagementsPage from "@features/csm-engagements/pages/CsmEngagementsPage";
import CsmEngagementDetailPage from "@features/csm-engagements/pages/CsmEngagementDetailPage";
import UsageMetricsPage from "@features/usage-metrics/pages/UsageMetricsPage";
import SettingsPage from "@features/settings/pages/SettingsPage";
import ServiceNowCaseRedirectPage from "@features/project-hub/pages/ServiceNowCaseRedirectPage";
import Error401Page from "@components/error/Error401Page";
import Error403Page from "@components/error/Error403Page";
import Error404Page from "@components/error/Error404Page";
import ErrorLayout from "@layouts/ErrorLayout";
import { ErrorBannerProvider } from "@context/error-banner/ErrorBannerContext";
import { SuccessBannerProvider } from "@context/success-banner/SuccessBannerContext";
import { LoaderProvider } from "@context/linear-loader/LoaderContext";
import { ErrorPageProvider } from "@context/error-page/ErrorPageContext";

// Consume the saved post-login redirect with a short-window cache.
//
// Two competing constraints:
//   1. StrictMode invokes state initializers / function-component bodies twice
//      in dev. A non-idempotent reader (read + remove from sessionStorage)
//      loses the value on the second call → user lands on /dashboard instead
//      of the saved deep link. So we cache the resolved value.
//   2. Clicking the logo navigates to `/`, which re-renders this component.
//      A permanent cache would keep redirecting the user to the original
//      deep link forever, defeating the logo. So the cache must expire.
//
// 500ms is comfortably longer than StrictMode's microsecond double-init and
// far shorter than any user-initiated click.
let cachedPostLoginRedirect: string | undefined;
let cachedAt = 0;
const CACHE_TTL_MS = 500;

function consumePostLoginRedirect(): string {
  const now = Date.now();
  if (
    cachedPostLoginRedirect !== undefined &&
    now - cachedAt < CACHE_TTL_MS
  ) {
    return cachedPostLoginRedirect;
  }
  cachedPostLoginRedirect = undefined;
  try {
    if (typeof sessionStorage !== "undefined") {
      const saved = sessionStorage.getItem("post_login_redirect");
      if (saved && saved !== "/") {
        sessionStorage.removeItem("post_login_redirect");
        cachedPostLoginRedirect = saved;
        cachedAt = now;
        return saved;
      }
    }
  } catch {
    // ignore — sessionStorage access can throw in sandboxed contexts
  }
  cachedPostLoginRedirect = "/dashboard";
  cachedAt = now;
  return cachedPostLoginRedirect;
}

/**
 * Renders at the `/` route inside AuthGuard. If the IdP redirect dropped a
 * deep-link in sessionStorage (the URL the user was originally trying to
 * reach before being sent to the IdP), restore that URL — including its
 * fragment. Otherwise fall back to the dashboard.
 */
function PostLoginRedirect(): JSX.Element {
  return <Navigate to={consumePostLoginRedirect()} replace />;
}

export default function App(): JSX.Element {
  return (
    <LoaderProvider>
      <ErrorBannerProvider>
        <SuccessBannerProvider>
          <ErrorPageProvider>
            <Routes>
              <Route path="/401" element={<ErrorLayout><Error401Page /></ErrorLayout>} />
              <Route path="/403" element={<ErrorLayout><Error403Page /></ErrorLayout>} />
              <Route path="/404" element={<ErrorLayout><Error404Page /></ErrorLayout>} />

              <Route element={<AuthGuard />}>
                <Route path="/" element={<PostLoginRedirect />} />

                {/* CSM top-level pages — mocked while csm-portal/backend catches up */}
                <Route path="dashboard" element={<CsmDashboardPage />} />

                <Route path="cases" element={<CsmCasesPage />} />
                {/* Real form (v2). Wip routes this to CsmComingSoonPage; we keep ours. */}
                <Route path="cases/new" element={<CreateCasePage />} />
                <Route path="cases/:caseId" element={<CsmCaseDetailPage />} />

                <Route path="projects" element={<CsmProjectsPage />} />

                <Route
                  path="operations"
                  element={
                    <CsmComingSoonPage
                      title="Operations"
                      description="Service requests and change requests across customers."
                      blockedOn="csm-portal/backend operations endpoints"
                    />
                  }
                />
                <Route path="engagements" element={<CsmEngagementsPage />} />
                <Route
                  path="engagements/:engagementId"
                  element={<CsmEngagementDetailPage />}
                />

                <Route path="updates" element={<CsmUpdatesPage />} />

                <Route
                  path="security-center"
                  element={
                    <CsmComingSoonPage
                      title="Security center"
                      description="Vulnerability posture across customer deployments."
                      blockedOn="csm-portal/backend security endpoint"
                    />
                  }
                />

                <Route
                  path="time-cards"
                  element={
                    <CsmComingSoonPage
                      title="Time cards"
                      description="Engineer time-tracking entries against cases, with approval flow."
                      blockedOn="csm-portal/backend time-cards endpoints"
                    />
                  }
                />

                <Route path="accounts" element={<CsmAccountsPage />} />
                <Route path="accounts/:accountId" element={<CsmAccountDetailPage />} />

                {/* Administration — v2 has CsmAdminLayout with real Users tab.
                    Roles/Groups detail pages come from wip as mocks. */}
                <Route path="admin" element={<CsmAdminLayout />}>
                  <Route index element={<Navigate to="/admin/users" replace />} />
                  <Route path="users" element={<CsmUsersPage />} />
                  <Route path="users/:userId" element={<CsmAdminUserDetailPage />} />
                  <Route path="roles" element={<CsmAdminRolesPage />} />
                  <Route path="roles/:roleId" element={<CsmAdminRoleDetailPage />} />
                  <Route path="groups" element={<CsmAdminGroupsPage />} />
                  <Route path="groups/:groupId" element={<CsmAdminGroupDetailPage />} />
                  <Route path="teams" element={<CsmAdminTeamsPage />} />
                  <Route path="skills" element={<CsmAdminSkillsPage />} />
                  <Route path="templates" element={<CsmAdminTemplatesPage />} />
                  <Route path="catalog" element={<CsmAdminCatalogPage />} />
                  <Route path="sla" element={<CsmAdminSlaPage />} />
                  <Route path="reports" element={<CsmAdminReportsPage />} />
                  <Route path="closure" element={<CsmAdminClosurePage />} />
                  <Route
                    path="permissions"
                    element={
                      <CsmComingSoonPage
                        title="Permissions"
                        description="Fine-grained permission catalog and assignment view."
                        blockedOn="csm-portal/backend permissions endpoints"
                      />
                    }
                  />
                </Route>

                {/* ServiceNow deep-link redirect */}
                <Route path="support" element={<ServiceNowCaseRedirectPage />} />

                {/* Project Specific Routes (mocked, project-scoped).
                    - `/projects/:projectId` (index) → CSM-native project detail (no ProjectGuard).
                    - Nested routes stay under ProjectGuard. */}
                <Route path="projects/:projectId">
                  <Route index element={<CsmProjectDetailPage />} />
                  <Route element={<ProjectGuard />}>
                    <Route path="dashboard">
                      <Route index element={<DashboardPage />} />
                      <Route path="action-required" element={<DashboardItemsPage mode="action-required" />} />
                      <Route path="outstanding-interactions" element={<DashboardItemsPage mode="outstanding-interactions" />} />
                      <Route path="closed-last-30d" element={<DashboardItemsPage mode="closed-last-30d" />} />
                    </Route>
                    <Route path="project-details" element={<ProjectDetailsPage />} />
                    <Route path="operations">
                      <Route index element={<OperationsPage />} />
                      <Route path="service-requests">
                        <Route index element={<ServiceRequestsPage />} />
                        <Route path="create" element={<CreateServiceRequestPage />} />
                        <Route path=":serviceRequestId" element={<ServiceRequestDetailsPage />} />
                      </Route>
                      <Route path="change-requests">
                        <Route index element={<ChangeRequestsPage />} />
                        <Route path=":changeRequestId" element={<ChangeRequestDetailsPage />} />
                      </Route>
                    </Route>
                    <Route path="support">
                      <Route index element={<SupportPage />} />
                      <Route path="cases">
                        <Route index element={<AllCasesPage />} />
                        <Route path=":caseId" element={<CaseDetailsPage />} />
                      </Route>
                      <Route path="change-requests">
                        <Route index element={<ChangeRequestsPage />} />
                        <Route path=":changeRequestId" element={<ChangeRequestDetailsPage />} />
                      </Route>
                      <Route path="conversations">
                        <Route index element={<AllConversationsPage />} />
                        <Route path=":conversationId" element={<ConversationDetailsPage />} />
                      </Route>
                      <Route path="service-requests">
                        <Route index element={<ServiceRequestsPage />} />
                        <Route path="create" element={<CreateServiceRequestPage />} />
                        <Route path=":serviceRequestId" element={<ServiceRequestDetailsPage />} />
                      </Route>
                      <Route path="chat">
                        <Route index element={<NoveraChatPage />} />
                        <Route path=":conversationId" element={<NoveraChatPage />} />
                        <Route path="describe-issue" element={<DescribeIssuePage />} />
                        <Route path="create-case" element={<CreateCasePage />} />
                        <Route path="create-related-case" element={<CreateCasePage />} />
                      </Route>
                      <Route path="security-report">
                        <Route path="create" element={<CreateCasePage />} />
                      </Route>
                    </Route>
                    <Route path="updates">
                      <Route index element={<UpdatesPage />} />
                      <Route path="pending">
                        <Route index element={<PendingUpdatesPage />} />
                        <Route path="level/:levelKey" element={<UpdateLevelDetailsPage />} />
                      </Route>
                    </Route>
                    <Route path="security-center">
                      <Route index element={<SecurityPage />} />
                      <Route path="security-report-analysis/:caseId" element={<CaseDetailsPage />} />
                      <Route path=":vulnerabilityId" element={<VulnerabilityDetailsPage />} />
                    </Route>
                    <Route path="engagements">
                      <Route index element={<EngagementsPage />} />
                      <Route path=":caseId" element={<CaseDetailsPage />} />
                    </Route>
                    <Route path="usage-metrics" element={<UsageMetricsPage />} />
                    <Route path="announcements">
                      <Route index element={<AnnouncementsPage />} />
                      <Route path=":caseId" element={<AnnouncementDetailsPage />} />
                    </Route>
                    <Route path="settings" element={<SettingsPage />} />
                  </Route>
                </Route>
              </Route>

              <Route path="*" element={<ErrorLayout><Error404Page /></ErrorLayout>} />
            </Routes>
          </ErrorPageProvider>
        </SuccessBannerProvider>
      </ErrorBannerProvider>
    </LoaderProvider>
  );
}
