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

import type { JSX } from "react";
import { BrowserRouter, Route, Routes, Navigate } from "react-router";
import ProjectHub from "@/pages/ProjectHub";
import ProjectPage from "@/pages/ProjectPage";
import ProjectDetails from "@/pages/ProjectDetails";
import DashboardPage from "@/pages/DashboardPage";
import SupportPage from "@/pages/SupportPage";
import NoveraChatPage from "@/pages/NoveraChatPage";
import CreateCasePage from "@/pages/CreateCasePage";
import AppLayout from "@/layouts/AppLayout";
import { LoaderProvider } from "@/context/linearLoader/LoaderContext";

export default function App(): JSX.Element {
  return (
    <BrowserRouter>
      <LoaderProvider>
        <Routes>
          {/* AppLayout component */}
          <Route element={<AppLayout />}>
            {/* ProjectHub Page */}
            <Route path="/" element={<ProjectHub />} />

            {/* Project Specific Routes */}
            <Route path="/:projectId">
              {/* Dashboard */}
              <Route index element={<Navigate to="dashboard" replace />} />
              <Route path="dashboard" element={<DashboardPage />} />
              {/* Project Details */}
              <Route path="project-details" element={<ProjectDetails />} />
              {/* Support */}
              <Route path="support">
                <Route index element={<SupportPage />} />
                <Route path="chat">
                  <Route index element={<NoveraChatPage />} />
                  <Route path="create-case" element={<CreateCasePage />} />
                </Route>
              </Route>
              {/* Updates */}
              <Route path="updates" element={<ProjectPage title="Updates" />} />
              {/* SecurityCenter */}
              <Route
                path="security-center"
                element={<ProjectPage title="Security Center" />}
              />
              {/* Engagements */}
              <Route
                path="engagements"
                element={<ProjectPage title="Engagements" />}
              />
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
              <Route
                path="announcements"
                element={<ProjectPage title="Announcements" />}
              />
              {/* Settings */}
              <Route
                path="settings"
                element={<ProjectPage title="Settings" />}
              />
            </Route>
          </Route>

          {/* Fallback */}
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </LoaderProvider>
    </BrowserRouter>
  );
}
