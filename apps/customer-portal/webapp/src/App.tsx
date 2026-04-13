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
import ProjectHub from "@pages/ProjectHub";
import ProjectPage from "@pages/ProjectPage";
import ProjectDetails from "@pages/ProjectDetails";
import ProjectGuard from "@layouts/ProjectGuard";
import DashboardPage from "@pages/DashboardPage";
import SupportPage from "@pages/SupportPage";
import UpdatesPage from "@pages/UpdatesPage";
import PendingUpdatesPage from "@pages/PendingUpdatesPage";
import UpdateLevelDetailsPage from "@pages/UpdateLevelDetailsPage";
import AllCasesPage from "@pages/AllCasesPage";
import ChangeRequestsPage from "@pages/support/change-requests/ChangeRequestsPage";
import ChangeRequestDetailsPage from "@pages/support/change-requests/ChangeRequestDetailsPage";
import AnnouncementsPage from "@pages/AnnouncementsPage";
import AnnouncementDetailsPage from "@pages/AnnouncementDetailsPage";
import AllConversationsPage from "@pages/AllConversationsPage";
import ConversationDetailsPage from "@pages/ConversationDetailsPage";
import CaseDetailsPage from "@pages/CaseDetailsPage";
import ServiceRequestsPage from "@pages/ServiceRequestsPage";
import ServiceRequestDetailsPage from "@pages/ServiceRequestDetailsPage";
import CreateServiceRequestPage from "@pages/CreateServiceRequestPage";
import NoveraChatPage from "@pages/NoveraChatPage";
import DescribeIssuePage from "@pages/DescribeIssuePage";
import CreateCasePage from "@pages/CreateCasePage";
import { ErrorBannerProvider } from "@context/error-banner/ErrorBannerContext";
import { SuccessBannerProvider } from "@context/success-banner/SuccessBannerContext";
import { LoaderProvider } from "@context/linear-loader/LoaderContext";
import SecurityPage from "@pages/SecurityPage";
import SettingsPage from "@pages/SettingsPage";
import VulnerabilityDetailsPage from "@pages/VulnerabilityDetailsPage";
import OperationsPage from "@pages/OperationsPage";
import EngagementsPage from "@pages/EngagementsPage";
import UsageMetricsPage from "@pages/UsageMetricsPage";
import ServiceNowCaseRedirectPage from "@pages/ServiceNowCaseRedirectPage";
import {
  Error401Page,
  Error403Page,
  Error404Page,
} from "@components/common/error";

export default function App(): JSX.Element {
  return (
    <LoaderProvider>
      <ErrorBannerProvider>
        <SuccessBannerProvider>
          <Routes>
            {/* Error Routes */}
            <Route path="/401" element={<Error401Page />} />
            <Route path="/403" element={<Error403Page />} />
            <Route path="/404" element={<Error404Page />} />

            <Route element={<AuthGuard />}>
              {/* ProjectHub Page */}
              <Route path="/" element={<ProjectHub />} />

              {/* ServiceNow deep-link redirect */}
              <Route path="support" element={<ServiceNowCaseRedirectPage />} />

              {/* Project Specific Routes */}
              <Route path="projects/:projectId" element={<ProjectGuard />}>
                {/* Dashboard */}
                <Route index element={<Navigate to="dashboard" replace />} />
                <Route path="dashboard" element={<DashboardPage />} />
                {/* Project Details */}
                <Route path="project-details" element={<ProjectDetails />} />
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
                {/* LegalContracts */}
                <Route
                  path="legal-contracts"
                  element={<ProjectPage title="Legal Contracts" />}
                />
                {/* Community */}
                <Route
                  path="community"
                  element={<ProjectPage title="Community" />}
                />
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
            <Route path="*" element={<Error404Page />} />
          </Routes>
        </SuccessBannerProvider>
      </ErrorBannerProvider>
    </LoaderProvider>
  );
}
