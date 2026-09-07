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

// Routes: / (dashboard), /issues, catch-all -> redirect to / (SPEC §11).
// Eagerly imported pages (csm-portal's no-lazy rule) — this app ships as one
// small bundle, so there is no route-chunking cost to avoid in the first
// place.
import { Navigate, Route, Routes } from "react-router";
import AuthGuard from "@layouts/AuthGuard";
import DashboardPage from "@features/dashboard/DashboardPage";
import IssuesPage from "@features/issues/IssuesPage";

export default function App() {
  return (
    <Routes>
      <Route element={<AuthGuard />}>
        <Route path="/" element={<DashboardPage />} />
        <Route path="/issues" element={<IssuesPage />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
