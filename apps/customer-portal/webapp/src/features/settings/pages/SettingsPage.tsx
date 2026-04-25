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
import { useParams } from "react-router";
import { useState, useMemo, useEffect, type JSX } from "react";
import useGetUserDetails from "@features/settings/api/useGetUserDetails";
import useGetProjectDetails from "@api/useGetProjectDetails";
import TabBar from "@components/tab-bar/TabBar";
import SettingsAiAssistant from "@features/settings/components/SettingsAiAssistant";
import SettingsUserManagement from "@features/settings/components/SettingsUserManagement";
import SettingsRegistryTokens from "@features/settings/components/SettingsRegistryTokens";
import {
  SETTINGS_CUSTOMER_ADMIN_ROLE,
  SETTINGS_PAGE_TABS,
  SETTINGS_PROJECT_NOT_FOUND_MESSAGE,
} from "@features/settings/constants/settingsConstants";
import { SettingsPageTabId } from "@features/settings/types/settings";
import { resolveSettingsPageTabId } from "@features/settings/utils/settingsPage";
import { consumePendingSettingsTab } from "@features/settings/utils/settingsStorage";
import { ProjectType } from "@/types/permission";
import { isProjectRestricted } from "@utils/permission";

/**
 * Settings page with User Management and AI Assistant tabs.
 *
 * @returns {JSX.Element} The Settings page.
 */
export default function SettingsPage(): JSX.Element {
  const { projectId } = useParams<{ projectId: string }>();
  const [activeTab, setActiveTab] = useState<string>(SettingsPageTabId.USERS);
  const { data: userDetails } = useGetUserDetails();

  useEffect(() => {
    const pending = consumePendingSettingsTab();
    if (pending) setActiveTab(pending);
  }, []);
  const { data: projectDetails } = useGetProjectDetails(projectId || "");

  const isCustomerAdmin = useMemo(
    () => (userDetails?.roles ?? []).includes(SETTINGS_CUSTOMER_ADMIN_ROLE),
    [userDetails?.roles],
  );

  const isRestricted = isProjectRestricted(projectDetails?.closureState);

  const hideRestrictedTabs = useMemo(() => {
    const projectTypeLabel = projectDetails?.type?.label;
    return (
      projectTypeLabel === ProjectType.CLOUD_SUPPORT ||
      projectTypeLabel === ProjectType.CLOUD_EVALUATION_SUPPORT
    );
  }, [projectDetails?.type?.label]);

  const tabs = useMemo(
    () =>
      hideRestrictedTabs
        ? SETTINGS_PAGE_TABS.filter(
            (tab) =>
              tab.id !== SettingsPageTabId.USERS &&
              tab.id !== SettingsPageTabId.REGISTRY_TOKENS,
          )
        : [...SETTINGS_PAGE_TABS],
    [hideRestrictedTabs],
  );

  const safeActiveTab = useMemo(() => {
    if (
      hideRestrictedTabs &&
      (resolveSettingsPageTabId(activeTab) === SettingsPageTabId.USERS ||
        resolveSettingsPageTabId(activeTab) === SettingsPageTabId.REGISTRY_TOKENS)
    ) {
      return SettingsPageTabId.AI;
    }
    return activeTab;
  }, [activeTab, hideRestrictedTabs]);

  const displayTab = useMemo(
    () => resolveSettingsPageTabId(safeActiveTab),
    [safeActiveTab],
  );

  if (!projectId) {
    return (
      <Box sx={{ p: 3 }}>
        <Typography variant="body1" color="text.secondary">
          {SETTINGS_PROJECT_NOT_FOUND_MESSAGE}
        </Typography>
      </Box>
    );
  }

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <TabBar
        tabs={tabs.map((t) => ({
          id: t.id,
          label: t.label,
          icon: t.icon,
        }))}
        activeTab={displayTab}
        onTabChange={setActiveTab}
        sx={{ mb: 0 }}
      />

      {displayTab === SettingsPageTabId.USERS && (
        <SettingsUserManagement
          projectId={projectId}
          canAddOrRemoveUsers={isCustomerAdmin && !isRestricted}
        />
      )}
      {displayTab === SettingsPageTabId.AI && (
        <SettingsAiAssistant projectId={projectId} canEdit={isCustomerAdmin} />
      )}
      {displayTab === SettingsPageTabId.REGISTRY_TOKENS && (
        <SettingsRegistryTokens
          projectId={projectId}
          isAdmin={isCustomerAdmin}
          isRestricted={isRestricted}
        />
      )}
    </Box>
  );
}
