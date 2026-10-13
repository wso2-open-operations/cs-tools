// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.

import { type JSX } from "react";
import { Navigate, Route, Routes } from "react-router";
import KBLayout from "@features/kb/components/KBLayout";
import KBHomePage from "@features/kb/pages/KBHomePage";
import KBBrowsePage from "@features/kb/pages/KBBrowsePage";
import KBArticlePage from "@features/kb/pages/KBArticlePage";

// The whole app is public and unauthenticated -- there is no AuthGuard,
// no login, no protected route. Everything lives under KBLayout.
export default function App(): JSX.Element {
  return (
    <Routes>
      <Route path="/" element={<KBLayout />}>
        <Route index element={<KBHomePage />} />
        <Route path="browse" element={<KBBrowsePage />} />
        <Route path="articles/:id" element={<KBArticlePage />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
