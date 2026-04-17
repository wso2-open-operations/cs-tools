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
import { useState, useMemo, type JSX } from "react";
import useGetUserDetails from "@features/settings/api/useGetUserDetails";
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

/**
 * Settings page with User Management and AI Assistant tabs.
 *
 * @returns {JSX.Element} The Settings page.
 */
export default function SettingsPage(): JSX.Element {
  const { projectId } = useParams<{ projectId: string }>();
  const [activeTab, setActiveTab] = useState<string>(SettingsPageTabId.USERS);
  const { data: userDetails } = useGetUserDetails();

  const isCustomerAdmin = useMemo(
    () => (userDetails?.roles ?? []).includes(SETTINGS_CUSTOMER_ADMIN_ROLE),
    [userDetails?.roles],
  );

  const tabs = useMemo(() => [...SETTINGS_PAGE_TABS], []);

  const displayTab = useMemo(
    () => resolveSettingsPageTabId(activeTab),
    [activeTab],
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
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
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
          canAddOrRemoveUsers={isCustomerAdmin}
        />
      )}
      {displayTab === SettingsPageTabId.AI && (
        <SettingsAiAssistant projectId={projectId} canEdit={isCustomerAdmin} />
      )}
      {displayTab === SettingsPageTabId.REGISTRY_TOKENS && (
        <SettingsRegistryTokens
          projectId={projectId}
          isAdmin={isCustomerAdmin}
        />
      )}
    </Box>
  );
}
