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


import { Box } from "@wso2/oxygen-ui";
import { type JSX, Suspense, useMemo } from "react";
import { Outlet } from "react-router";
import RouteSuspenseFallback from "@components/route-fallback/RouteSuspenseFallback";
import SectionTabs from "@components/section-tabs/SectionTabs";
import { useRouteTabs } from "@hooks/useSectionTabs";
import { isFeatureVisible } from "@config/featureFlags";
import { usePortalAccess } from "@context/current-user/usePortalAccess";

/**
 * Settings ▸ Product catalogue: the data behind the public Support Matrix and
 * Release Matrix, under its own second-level tab strip (Catalogue, Release
 * lines), driven by the navigation tree. `CsmAdminLayout` renders the
 * "Settings" title and the first-level strip above it. `useRouteTabs` resolves
 * tabs for the deployment only, so a tab whose nav node `requires` a
 * capability is narrowed here by the caller's access, the same way
 * `CsmAdminLayout` narrows its tabs.
 */
export default function ProductsLayout(): JSX.Element {
  const access = usePortalAccess();
  const allTabs = useRouteTabs("admin.product-catalog");
  const tabs = useMemo(() => {
    const visible = allTabs.tabs.filter((tab) => isFeatureVisible(tab.node.id, access));
    const activeKey = visible.some((tab) => tab.key === allTabs.activeKey)
      ? allTabs.activeKey
      : (visible[0]?.key ?? "");
    return { ...allTabs, tabs: visible, activeKey };
  }, [allTabs, access]);

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <SectionTabs {...tabs} ariaLabel="Product catalogue tabs" variant="secondary" />

      <Suspense fallback={<RouteSuspenseFallback />}>
        <Outlet />
      </Suspense>
    </Box>
  );
}
