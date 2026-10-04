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

import { lazy, useLayoutEffect } from "react";
import { HashRouter as Router, Route, Routes } from "react-router-dom";
import MainLayout from "@components/layout/MainLayout";
import { requestDeviceSafeAreaInsets } from "@components/microapp-bridge";
import { refreshToken } from "@src/services/auth";
import { Logger } from "@utils/logger";

// Each route loads on demand so the first paint does not pay for every screen.
const HomePage = lazy(() => import("@pages/HomePage"));
const SupportPage = lazy(() => import("@pages/SupportPage"));
const CaseDetailPage = lazy(() => import("@pages/CaseDetailPage"));
const NewCasePage = lazy(() => import("@pages/NewCasePage"));
const OperationsPage = lazy(() => import("@pages/OperationsPage"));
const NewChangeRequestPage = lazy(() => import("@pages/NewChangeRequestPage"));
const NewServiceRequestPage = lazy(() => import("@pages/NewServiceRequestPage"));
const NewIncidentPage = lazy(() => import("@pages/NewIncidentPage"));
const ChangeRequestDetailPage = lazy(() => import("@pages/ChangeRequestDetailPage"));
const IncidentDetailPage = lazy(() => import("@pages/IncidentDetailPage"));
const MorePage = lazy(() => import("@pages/MorePage"));
const AnnouncementsPage = lazy(() => import("@pages/AnnouncementsPage"));
const TimeCardsPage = lazy(() => import("@pages/TimeCardsPage"));
const SecurityCenterPage = lazy(() => import("@pages/SecurityCenterPage"));
const NewSecurityReportPage = lazy(() => import("@pages/NewSecurityReportPage"));
const UpdatesPage = lazy(() => import("@pages/UpdatesPage"));
const EngagementsPage = lazy(() => import("@pages/EngagementsPage"));
const CustomersPage = lazy(() => import("@pages/CustomersPage"));
const AccountDetailPage = lazy(() => import("@pages/AccountDetailPage"));
const ProjectDetailPage = lazy(() => import("@pages/ProjectDetailPage"));
const VulnerabilityDetailPage = lazy(() => import("@pages/VulnerabilityDetailPage"));
const SettingsPage = lazy(() => import("@pages/SettingsPage"));
const ProfilePage = lazy(() => import("@pages/ProfilePage"));

export default function App() {
  useLayoutEffect(() => {
    requestDeviceSafeAreaInsets((data) => {
      if (!data?.insets) return;

      const { top, right, bottom, left } = data.insets;
      const root = document.documentElement;
      root.style.setProperty("--safe-top", `${top}px`);
      root.style.setProperty("--safe-right", `${right}px`);
      root.style.setProperty("--safe-bottom", `${bottom}px`);
      root.style.setProperty("--safe-left", `${left}px`);
    });

    // Eagerly authenticate on launch so the user's name/avatar (shown in the TopBar on every
    // page) is populated regardless of which page loads first — several pages (Home, Operations,
    // More) don't make any API call themselves, and the user store was otherwise only ever
    // populated as a side effect of apiClient's request interceptor refreshing the token.
    refreshToken().catch((error) => {
      Logger.warn("Failed to eagerly authenticate on launch", error);
    });
  }, []);

  return (
    <Router>
      <Routes>
        <Route element={<MainLayout />}>
          <Route path="/" element={<HomePage />} />
          <Route path="/support" element={<SupportPage />} />
          <Route path="/cases/new" element={<NewCasePage />} />
          <Route path="/cases/:id" element={<CaseDetailPage />} />
          <Route path="/operations" element={<OperationsPage />} />
          <Route path="/operations/change-requests/new" element={<NewChangeRequestPage />} />
          <Route path="/operations/service-requests/new" element={<NewServiceRequestPage />} />
          <Route path="/operations/change-requests/:id" element={<ChangeRequestDetailPage />} />
          <Route path="/operations/incidents/new" element={<NewIncidentPage />} />
          <Route path="/operations/incidents/:id" element={<IncidentDetailPage />} />
          <Route path="/more" element={<MorePage />} />
          <Route path="/more/announcements" element={<AnnouncementsPage />} />
          <Route path="/more/time-cards" element={<TimeCardsPage />} />
          <Route path="/more/security-center" element={<SecurityCenterPage />} />
          <Route path="/more/security-center/reports/new" element={<NewSecurityReportPage />} />
          <Route path="/more/security-center/vulnerabilities/:id" element={<VulnerabilityDetailPage />} />
          <Route path="/more/updates" element={<UpdatesPage />} />
          <Route path="/more/engagements" element={<EngagementsPage />} />
          <Route path="/more/customers" element={<CustomersPage />} />
          <Route path="/more/customers/accounts/:id" element={<AccountDetailPage />} />
          <Route path="/more/customers/projects/:id" element={<ProjectDetailPage />} />
          <Route path="/more/settings" element={<SettingsPage />} />
          <Route path="/profile" element={<ProfilePage />} />
        </Route>
      </Routes>
    </Router>
  );
}
