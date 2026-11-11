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

import { Box, Chip, Skeleton, Tab, Tabs, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { useQueryParamTabs } from "@hooks/useSectionTabs";
import { useSkillCapabilities } from "@features/csm-skills/api/useSkillManagement";
import MySkillsTab from "@features/csm-skills/components/MySkillsTab";
import SkillCatalogueBrowser from "@features/csm-skills/components/SkillCatalogueBrowser";
import SkillAdministrationTab from "@features/csm-skills/components/SkillAdministrationTab";
import SkillDirectoryTab from "@features/csm-skills/components/SkillDirectoryTab";

// "manager" is the old Skill Manager tab's id: a link to it lands on the directory.
const TAB_IDS = ["my", "all", "directory", "admin", "manager"] as const;
type SkillsTabId = (typeof TAB_IDS)[number];

/**
 * Skills: a skills profile for every CS engineer (with requests for skills the
 * catalogue lacks), the read-only catalogue, and, for those who hold the
 * capability, the Skill Directory (people by skill) and Administration
 * (reviewing requests, managing skills and categories).
 *
 * The Directory and Administration tabs are hidden from callers who lack them,
 * a deliberate exception to this app's "show it and let the backend reject it"
 * default, by explicit product decision. Which tabs show comes from
 * GET /skills/me/capabilities; the backend remains the real gate on every call.
 */
export default function CsmSkillsPage(): JSX.Element {
  const caps = useSkillCapabilities();
  const { activeTab, setActiveTab } = useQueryParamTabs<SkillsTabId>(TAB_IDS, "my");

  const capabilities = caps.data;
  const canDirectory = capabilities?.canViewDirectory === true;
  const canAdmin = capabilities?.canReviewRequests === true || capabilities?.canManageCatalogue === true;
  const wanted: SkillsTabId = activeTab === "manager" ? "directory" : activeTab;
  // A link to a gated tab waits for the capabilities instead of flashing another tab.
  const waiting = (wanted === "directory" || wanted === "admin") && caps.isPending;
  const shown: SkillsTabId =
    (wanted === "directory" && !canDirectory) || (wanted === "admin" && !canAdmin) ? "my" : wanted;
  const pending = capabilities?.pendingRequestCount ?? 0;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Typography variant="h5">Skills</Typography>

      <Box sx={{ borderBottom: 1, borderColor: "divider" }}>
        <Tabs
          aria-label="Skills tabs"
          value={waiting ? false : shown}
          onChange={(_, id: SkillsTabId) => setActiveTab(id)}
          variant="scrollable"
          scrollButtons="auto"
        >
          <Tab value="my" label="My Skills" id="skills-tab-my" aria-controls="skills-panel" />
          <Tab value="all" label="All Skills" id="skills-tab-all" aria-controls="skills-panel" />
          {canDirectory && (
            <Tab value="directory" label="Skill Directory" id="skills-tab-directory" aria-controls="skills-panel" />
          )}
          {canAdmin && (
            <Tab
              value="admin"
              id="skills-tab-admin"
              aria-controls="skills-panel"
              label={
                <Box component="span" sx={{ display: "inline-flex", alignItems: "center", gap: 1 }}>
                  Administration
                  {pending > 0 && <Chip size="small" color="warning" label={pending} aria-label={`${pending} pending requests`} />}
                </Box>
              }
            />
          )}
        </Tabs>
      </Box>

      <Box role="tabpanel" id="skills-panel" aria-labelledby={`skills-tab-${shown}`}>
        {waiting ? (
          <Skeleton variant="rounded" height={160} aria-label="Loading" />
        ) : shown === "my" ? (
          <MySkillsTab />
        ) : shown === "all" ? (
          <SkillCatalogueBrowser canViewPeople={canDirectory} />
        ) : shown === "directory" ? (
          <SkillDirectoryTab />
        ) : capabilities ? (
          <SkillAdministrationTab capabilities={capabilities} />
        ) : null}
      </Box>
    </Box>
  );
}
