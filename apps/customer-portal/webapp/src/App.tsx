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
import { ProtectedRoute } from "@asgardeo/react-router";
import ProjectHub from "@pages/ProjectHub";
import ProjectPage from "@pages/ProjectPage";
import ProjectDetails from "@pages/ProjectDetails";
import DashboardPage from "@pages/DashboardPage";
import SupportPage from "@pages/SupportPage";
import UpdatesPage from "@pages/UpdatesPage";
import PendingUpdatesPage from "@pages/PendingUpdatesPage";
import UpdateLevelDetailsPage from "@pages/UpdateLevelDetailsPage";
import AllCasesPage from "@pages/AllCasesPage";
import ChangeRequestsPage from "@pages/ChangeRequestsPage";
import ChangeRequestDetailsPage from "@pages/ChangeRequestDetailsPage";
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
import AppLayout from "@layouts/AppLayout";
import { ErrorBannerProvider } from "@context/error-banner/ErrorBannerContext";
import { SuccessBannerProvider } from "@context/success-banner/SuccessBannerContext";
import { LoaderProvider } from "@context/linear-loader/LoaderContext";
import LoginPage from "@pages/LoginPage";
import SecurityPage from "@pages/SecurityPage";
import SettingsPage from "@pages/SettingsPage";
import VulnerabilityDetailsPage from "@pages/VulnerabilityDetailsPage";
import OperationsPage from "@pages/OperationsPage";
import EngagementsPage from "@pages/EngagementsPage";


export default function App(): JSX.Element {
  return (
    <LoaderProvider>
      <ErrorBannerProvider>
        <SuccessBannerProvider>
          <Routes>
            {/* Public Route */}
            <Route path="/login" element={<LoginPage />} />

            {/* Protected Routes - All routes inside ProtectedRoute are automatically protected */}
            <Route
              element={
                <ProtectedRoute redirectTo="/login">
                  <AppLayout />
                </ProtectedRoute>
              }
            >
              {/* ProjectHub Page */}
              <Route path="/" element={<ProjectHub />} />

              {/* Project Specific Routes */}
              <Route path="projects/:projectId">
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
                    <Route path="create" element={<CreateServiceRequestPage />} />
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
                    <Route
                      path="create"
                      element={<CreateCasePage />}
                    />
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
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </SuccessBannerProvider>
      </ErrorBannerProvider>
    </LoaderProvider>
  );
}
