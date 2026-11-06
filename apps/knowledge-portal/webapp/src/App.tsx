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
import { Route, Routes } from "react-router";
import NotFoundState from "@components/NotFoundState";
import KBHomePage from "@features/kb/pages/KBHomePage";
import KBBrowsePage from "@features/kb/pages/KBBrowsePage";
import KBArticlePage from "@features/kb/pages/KBArticlePage";
import { LegacyKbRedirect, PreviousArticleRedirect, PreviousBrowseRedirect } from "@components/LegacyRedirect";
import KBLayout from "@layouts/KBLayout";

// The whole app is public and read-only: no auth guard, no login, no
// protected route. Everything renders inside KBLayout.
export default function App(): JSX.Element {
  return (
    <Routes>
      <Route path="/" element={<KBLayout />}>
        <Route index element={<KBHomePage />} />
        <Route path="knowledge-bases" element={<KBBrowsePage />} />
        <Route path="knowledge-bases/:id" element={<KBBrowsePage />} />
        <Route path="knowledge-articles/:id" element={<KBArticlePage />} />
        <Route path="kb" element={<LegacyKbRedirect />} />
        <Route path="browse" element={<PreviousBrowseRedirect />} />
        <Route path="articles/:id" element={<PreviousArticleRedirect />} />
        <Route path="*" element={<NotFoundState />} />
      </Route>
    </Routes>
  );
}
