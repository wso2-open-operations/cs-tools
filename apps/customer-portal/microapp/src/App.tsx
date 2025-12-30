// Copyright (c) 2025 WSO2 LLC. (https://www.wso2.com).
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

import React from "react";
import { HashRouter as Router, Route, Routes } from "react-router-dom";
import SelectProjectPage from "@pages/SelectProjectPage";
import MainLayout from "@components/layout/MainLayout";
import AppProvider from "@context/AppProvider";

import HomePage from "@pages/HomePage";
import SupportPage from "@pages/SupportPage";
import UsersPage from "@pages/UsersPage";
import ProfilePage from "@pages/ProfilePage";
import NotificationsPage from "@pages/NotificationsPage";
import ChatPage from "@pages/ChatPage";
import CreateCasePage from "@pages/CreateCasePage";
import AllCasesPage from "@pages/AllCasesPage";
import DetailedPage from "./pages/DetailedPage";

const App: React.FC = () => {
  return (
    <Router>
      <AppProvider>
        <Routes>
          <Route path="/select" element={<SelectProjectPage />} />

          <Route element={<MainLayout />}>
            <Route path="/" element={<HomePage />} />
            <Route path="/support" element={<SupportPage />} />
            <Route path="/users" element={<UsersPage />} />
            <Route path="/profile" element={<ProfilePage />} />
            <Route path="/notifications" element={<NotificationsPage />} />
            <Route path="/chat" element={<ChatPage />} />
            <Route path="/create" element={<CreateCasePage />} />
            <Route path="/cases">
              <Route path="all" element={<AllCasesPage type="case" />} />
              <Route path=":id" element={<DetailedPage type="case" />} />
            </Route>
            <Route path="/chats">
              <Route path="all" element={<AllCasesPage type="chat" />} />
            </Route>
            <Route path="/services">
              <Route path="all" element={<AllCasesPage type="service" />} />
            </Route>
            <Route path="/changes">
              <Route path="all" element={<AllCasesPage type="change" />} />
            </Route>
          </Route>
        </Routes>
      </AppProvider>
    </Router>
  );
};

export default App;
