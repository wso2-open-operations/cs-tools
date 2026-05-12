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
import ProjectHubPage from "@features/project-hub/pages/ProjectHub";
import ProjectDetailsPage from "@features/project-details/pages/ProjectDetails";
import DashboardPage from "@features/dashboard/pages/DashboardPage";
import DashboardItemsPage from "@features/dashboard/pages/DashboardItemsPage";
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
import UpdatesPage from "@features/updates/pages/UpdatesPage";
import PendingUpdatesPage from "@features/updates/pages/PendingUpdatesPage";
import UpdateLevelDetailsPage from "@features/updates/pages/UpdateLevelDetailsPage";
import AnnouncementsPage from "@features/announcements/pages/AnnouncementsPage";
import AnnouncementDetailsPage from "@features/announcements/pages/AnnouncementDetailsPage";
import OperationsPage from "@features/operations/pages/OperationsPage";
import SecurityPage from "@features/security/pages/SecurityPage";
import VulnerabilityDetailsPage from "@features/security/pages/VulnerabilityDetailsPage";
import EngagementsPage from "@features/engagements/pages/EngagementsPage";
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

export default function App(): JSX.Element {
  return (
    <LoaderProvider>
      <ErrorBannerProvider>
        <SuccessBannerProvider>
          <ErrorPageProvider>
            <Routes>
              {/* Error Routes */}
              <Route path="/401" element={<ErrorLayout><Error401Page /></ErrorLayout>} />
              <Route path="/403" element={<ErrorLayout><Error403Page /></ErrorLayout>} />
              <Route path="/404" element={<ErrorLayout><Error404Page /></ErrorLayout>} />

            <Route element={<AuthGuard />}>
              {/* ProjectHub Page */}
              <Route path="/" element={<ProjectHubPage />} />

              {/* ServiceNow deep-link redirect */}
              <Route path="support" element={<ServiceNowCaseRedirectPage />} />

              {/* Project Specific Routes */}
              <Route path="projects/:projectId" element={<ProjectGuard />}>
                {/* Dashboard */}
                <Route index element={<Navigate to="dashboard" replace />} />
                <Route path="dashboard">
                  <Route index element={<DashboardPage />} />
                  <Route path="action-required" element={<DashboardItemsPage mode="action-required" />} />
                  <Route path="outstanding-interactions" element={<DashboardItemsPage mode="outstanding-interactions" />} />
                  <Route path="closed-last-30d" element={<DashboardItemsPage mode="closed-last-30d" />} />
                </Route>
                {/* Project Details */}
                <Route path="project-details" element={<ProjectDetailsPage />} />
                {/* Operations */}
                <Route path="operations">
                  <Route index element={<OperationsPage />} />
                  <Route path="service-requests">
                    <Route index element={<ServiceRequestsPage />} />
                    <Route
                      path="create"
                      element={<CreateServiceRequestPage />}
                    />
                    <Route
                      path=":serviceRequestId"
                      element={<ServiceRequestDetailsPage />}
                    />
                  </Route>
                  <Route path="change-requests">
                    <Route index element={<ChangeRequestsPage />} />
                    <Route
                      path=":changeRequestId"
                      element={<ChangeRequestDetailsPage />}
                    />
                  </Route>
                </Route>
                {/* Support */}
                <Route path="support">
                  <Route index element={<SupportPage />} />
                  <Route path="cases">
                    <Route index element={<AllCasesPage />} />
                    <Route path=":caseId" element={<CaseDetailsPage />} />
                  </Route>
                  <Route path="change-requests">
                    <Route index element={<ChangeRequestsPage />} />
                    <Route
                      path=":changeRequestId"
                      element={<ChangeRequestDetailsPage />}
                    />
                  </Route>
                  <Route path="conversations">
                    <Route index element={<AllConversationsPage />} />
                    <Route
                      path=":conversationId"
                      element={<ConversationDetailsPage />}
                    />
                  </Route>
                  <Route path="service-requests">
                    <Route index element={<ServiceRequestsPage />} />
                    <Route
                      path="create"
                      element={<CreateServiceRequestPage />}
                    />
                    <Route
                      path=":serviceRequestId"
                      element={<ServiceRequestDetailsPage />}
                    />
                  </Route>
                  <Route path="chat">
                    <Route index element={<NoveraChatPage />} />
                    <Route
                      path=":conversationId"
                      element={<NoveraChatPage />}
                    />
                    <Route
                      path="describe-issue"
                      element={<DescribeIssuePage />}
                    />
                    <Route path="create-case" element={<CreateCasePage />} />
                    <Route
                      path="create-related-case"
                      element={<CreateCasePage />}
                    />
                  </Route>
                  <Route path="security-report">
                    <Route path="create" element={<CreateCasePage />} />
                  </Route>
                </Route>
                {/* Updates */}
                <Route path="updates">
                  <Route index element={<UpdatesPage />} />
                  <Route path="pending">
                    <Route index element={<PendingUpdatesPage />} />
                    <Route
                      path="level/:levelKey"
                      element={<UpdateLevelDetailsPage />}
                    />
                  </Route>
                </Route>
                {/* SecurityCenter */}
                <Route path="security-center">
                  <Route index element={<SecurityPage />} />
                  <Route
                    path="security-report-analysis/:caseId"
                    element={<CaseDetailsPage />}
                  />
                  <Route
                    path=":vulnerabilityId"
                    element={<VulnerabilityDetailsPage />}
                  />
                </Route>
                {/* Engagements */}
                <Route path="engagements">
                  <Route index element={<EngagementsPage />} />
                  <Route path=":caseId" element={<CaseDetailsPage />} />
                </Route>
                <Route path="usage-metrics" element={<UsageMetricsPage />} />
                {/* Announcements */}
                <Route path="announcements">
                  <Route index element={<AnnouncementsPage />} />
                  <Route path=":caseId" element={<AnnouncementDetailsPage />} />
                </Route>
                {/* Settings */}
                <Route path="settings" element={<SettingsPage />} />
              </Route>
            </Route>

            {/* Fallback */}
            <Route path="*" element={<ErrorLayout><Error404Page /></ErrorLayout>} />
          </Routes>
          </ErrorPageProvider>
        </SuccessBannerProvider>
      </ErrorBannerProvider>
    </LoaderProvider>
  );
}
