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
import { Route, HashRouter as Router, Routes } from "react-router-dom";

import AppProvider from "@app/providers";
import {
  ChatPage,
  CreateCasePage,
  HomePage,
  ItemPage,
  ItemsListPage,
  ProfileEditPage,
  ProfilePage,
  ProjectSelectPage,
  SupportPage,
  UserEditPage,
  UsersPage,
} from "@src/pages";

import { CASE_TYPES } from "@shared/constants";
import { useSafeAreaInsets, useScrollControl } from "@shared/hooks";

import { ErrorState } from "@components/common";
import MainLayout from "@components/layout/MainLayout";
import RequireProject from "@components/layout/RequireProject";

const App: React.FC = () => {
  useSafeAreaInsets();

  return (
    <Router>
      <AppProvider>
        <ScrollHandler />

        <Routes>
          <Route path="/select" element={<ProjectSelectPage />} />

          <Route element={<RequireProject />} errorElement={<ErrorState />}>
            <Route element={<MainLayout />} errorElement={<ErrorState />}>
              <Route path="/" element={<HomePage />} />

              <Route path="/users" element={<UsersPage />} />
              <Route path="/users/invite" element={<UserEditPage />} />
              <Route path="/users/edit" element={<UserEditPage />} />

              <Route path="/profile" element={<ProfilePage />} />
              <Route path="/profile/update" element={<ProfileEditPage />} />

              <Route path="/chat" element={<ChatPage />} />

              <Route path="/create" element={<CreateCasePage />} />

              <Route path="/support" element={<SupportPage />} />
              <Route path="/support/all" element={<ItemsListPage />} />

              <Route path="/cases/:id" element={<ItemPage type={CASE_TYPES.DEFAULT} />} />
              <Route path="/conversations/:id" element={<ItemPage type={CASE_TYPES.CHAT} />} />
              <Route path="/service-requests/:id" element={<ItemPage type={CASE_TYPES.SERVICE_REQUEST} />} />
              <Route path="/change-requests/:id" element={<ItemPage type={CASE_TYPES.CHANGE_REQUEST} />} />
              <Route path="/engagements/:id" element={<ItemPage type={CASE_TYPES.ENGAGEMENT} />} />
              <Route path="/announcements/:id" element={<ItemPage type={CASE_TYPES.ANNOUNCEMENT} />} />
              <Route
                path="/security-report-analysis/:id"
                element={<ItemPage type={CASE_TYPES.SECURITY_REPORT_ANALYSIS} />}
              />
            </Route>
          </Route>
        </Routes>
      </AppProvider>
    </Router>
  );
};

export default App;

const ScrollHandler: React.FC = () => {
  useScrollControl();
  return null;
};
