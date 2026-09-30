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

import { Box, Typography } from "@wso2/oxygen-ui";
import { type JSX, Suspense } from "react";
import { Outlet } from "react-router";
import RouteSuspenseFallback from "@components/route-fallback/RouteSuspenseFallback";
import SectionTabs from "@components/section-tabs/SectionTabs";
import { useRouteTabs } from "@hooks/useSectionTabs";

/**
 * Knowledge Base shell — "My articles" (engineer view) and "Review queue"
 * (manager view) under one tab strip. Both tabs are visible to every
 * authenticated user regardless of role — this codebase's convention is to
 * show the action and let the backend reject it (see PatchKBArticleState's
 * role check), not to hide navigation based on role. The tabs come from the
 * navigation tree, so a deployment can restrict one through
 * `CSM_PORTAL_FEATURE_OVERRIDES` without touching this layout.
 */
export default function CsmKBArticlesLayout(): JSX.Element {
  const tabs = useRouteTabs("kb-articles");

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Typography variant="h5">Knowledge Base</Typography>

      <SectionTabs {...tabs} ariaLabel="Knowledge Base tabs" scrollable />

      <Suspense fallback={<RouteSuspenseFallback />}>
        <Outlet />
      </Suspense>
    </Box>
  );
}
