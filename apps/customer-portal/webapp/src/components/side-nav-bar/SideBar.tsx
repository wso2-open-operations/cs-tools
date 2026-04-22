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

import { Link, Sidebar } from "@wso2/oxygen-ui";
import { Settings } from "@wso2/oxygen-ui-icons-react";
import { type JSX, useMemo } from "react";
import { useLocation, useParams, Link as NavigateLink } from "react-router";
import useInfiniteProjects, { flattenProjectPages } from "@api/useGetProjects";
import useGetProjectDetails from "@api/useGetProjectDetails";
import useGetProjectFeatures from "@api/useGetProjectFeatures";
import useGetMetadata from "@api/useGetMetadata";
import { APP_SHELL_NAV_ITEMS } from "@features/project-hub/constants/appLayoutConstants";
import type { AppShellNavItem } from "@features/project-hub/types/appLayout";
import { getProjectPermissions } from "@utils/permission";

// Props for the SideBar component.
interface SideBarProps {
  collapsed: boolean;
  expandedMenus?: Record<string, boolean>;
  onSelect?: (id: string) => void;
  onToggleExpand?: (id: string) => void;
}

export default function SideBar({
  collapsed,
  expandedMenus,
  onSelect,
  onToggleExpand,
}: SideBarProps): JSX.Element {
  const location = useLocation();
  const { projectId } = useParams<{ projectId?: string }>();

  // Get the active item from the location pathname.
  const pathSegments: string[] = location.pathname.split("/").filter(Boolean);
  const projectsIndex = pathSegments.indexOf("projects");
  const projectIdIndex: number =
    projectsIndex !== -1 && projectId ? pathSegments.indexOf(projectId) : -1;
  const activeItem: string =
    projectIdIndex !== -1 && pathSegments[projectIdIndex + 1]
      ? pathSegments[projectIdIndex + 1]
      : "dashboard";

  const projectsQuery = useInfiniteProjects({ enabled: !!projectId });
  const projects = flattenProjectPages(projectsQuery.data);
  const selectedProject = projects.find((project) => project.id === projectId);
  const { data: projectDetails } = useGetProjectDetails(projectId || "");
  const { data: projectFeatures } = useGetProjectFeatures(projectId || "");
  const { data: portalMetadata } = useGetMetadata();
  const usageMetricsEnabled =
    portalMetadata?.featureFlags?.usageMetricsEnabled === true;

  const projectTypeLabel =
    selectedProject?.type?.label ?? projectDetails?.type?.label;
  const isProjectTypeResolved =
    projectTypeLabel != null && String(projectTypeLabel).trim() !== "";
  const permissions = useMemo(
    () =>
      getProjectPermissions(projectTypeLabel, {
        projectFeatures,
      }),
    [
      projectTypeLabel,
      projectFeatures,
    ],
  );

  const navItems = useMemo(() => {
    let items = APP_SHELL_NAV_ITEMS;

    if (!usageMetricsEnabled) {
      items = items.filter((item: AppShellNavItem) => item.id !== "usage-metrics");
    }

    if (
      !isProjectTypeResolved ||
      !permissions.hasOperations ||
      (!permissions.hasSR && !permissions.hasCR)
    ) {
      items = items.filter((item: AppShellNavItem) => item.id !== "operations");
    }

    if (!permissions.hasEngagements) {
      items = items.filter((item: AppShellNavItem) => item.id !== "engagements");
    }

    if (!permissions.hasUpdates) {
      items = items.filter((item: AppShellNavItem) => item.id !== "updates");
    }

    return items;
  }, [
    isProjectTypeResolved,
    permissions.hasOperations,
    permissions.hasSR,
    permissions.hasCR,
    permissions.hasEngagements,
    permissions.hasUpdates,
    usageMetricsEnabled,
  ]);

  return (
    <Sidebar
      collapsed={collapsed}
      activeItem={activeItem}
      expandedMenus={expandedMenus}
      onSelect={onSelect}
      onToggleExpand={onToggleExpand}
    >
      {/* sidebar navigation items */}
      <Sidebar.Nav>
        <Sidebar.Category>
          {navItems.map((item: AppShellNavItem) => (
            <Link
              key={item.id}
              component={NavigateLink}
              to={`/projects/${projectId}/${item.path}`}
              color="inherit"
              underline="none"
            >
              <Sidebar.Item id={item.id}>
                <Sidebar.ItemIcon>
                  <item.icon />
                </Sidebar.ItemIcon>
                <Sidebar.ItemLabel>{item.label}</Sidebar.ItemLabel>
              </Sidebar.Item>
            </Link>
          ))}
        </Sidebar.Category>
      </Sidebar.Nav>

      {/* sidebar footer items */}
      <Sidebar.Footer>
        {/* <SubscriptionWidget collapsed={collapsed} /> */}
        <Sidebar.Category>
          <Link
            component={NavigateLink}
            to={`/projects/${projectId}/settings`}
            color="inherit"
            underline="none"
          >
            <Sidebar.Item id="settings">
              <Sidebar.ItemIcon>
                <Settings size={20} />
              </Sidebar.ItemIcon>
              <Sidebar.ItemLabel>Settings</Sidebar.ItemLabel>
            </Sidebar.Item>
          </Link>
        </Sidebar.Category>
      </Sidebar.Footer>
    </Sidebar>
  );
}
