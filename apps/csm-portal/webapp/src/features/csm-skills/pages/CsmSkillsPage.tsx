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

import { Box, Skeleton, Tab, Tabs, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { usePortalAccess } from "@context/current-user/usePortalAccess";
import { useQueryParamTabs } from "@hooks/useSectionTabs";
import { useIsSkillManager } from "@features/csm-skills/api/useSkillManager";
import AllSkillsTab from "@features/csm-skills/components/AllSkillsTab";
import MySkillsTab from "@features/csm-skills/components/MySkillsTab";
import SkillManagerTab from "@features/csm-skills/components/SkillManagerTab";

const TAB_IDS = ["my", "all", "manager"] as const;
type SkillsTabId = (typeof TAB_IDS)[number];

/**
 * Skills: a skills profile for every CS engineer, the catalogue, and (for
 * managers) a people-by-skill search.
 *
 * Two controls are hidden from callers who lack them, a deliberate exception
 * to this app's "show it and let the backend reject it" default, by explicit
 * product decision: the Skill Manager tab (shown only when the manager search
 * answers instead of refusing with 403) and the catalogue's create/edit/
 * deactivate actions (admins only). The backend remains the real gate.
 */
export default function CsmSkillsPage(): JSX.Element {
  const { isAdmin } = usePortalAccess();
  const manager = useIsSkillManager();
  const { activeTab, setActiveTab } = useQueryParamTabs<SkillsTabId>(TAB_IDS, "my");

  const isManager = manager.data === true;
  // A link to ?tab=manager waits for the probe instead of flashing another tab.
  const waitingOnManager = activeTab === "manager" && manager.isPending;
  const shown: SkillsTabId = activeTab === "manager" && !isManager ? "my" : activeTab;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Typography variant="h5">Skills</Typography>

      <Box sx={{ borderBottom: 1, borderColor: "divider" }}>
        <Tabs
          aria-label="Skills tabs"
          value={waitingOnManager ? false : shown}
          onChange={(_, id: SkillsTabId) => setActiveTab(id)}
          variant="scrollable"
          scrollButtons="auto"
        >
          <Tab value="my" label="My Skills" id="skills-tab-my" aria-controls="skills-panel" />
          <Tab value="all" label="All Skills" id="skills-tab-all" aria-controls="skills-panel" />
          {isManager && (
            <Tab value="manager" label="Skill Manager" id="skills-tab-manager" aria-controls="skills-panel" />
          )}
        </Tabs>
      </Box>

      <Box role="tabpanel" id="skills-panel" aria-labelledby={`skills-tab-${shown}`}>
        {waitingOnManager ? (
          <Skeleton variant="rounded" height={160} aria-label="Loading" />
        ) : shown === "my" ? (
          <MySkillsTab />
        ) : shown === "all" ? (
          <AllSkillsTab isAdmin={isAdmin} />
        ) : (
          <SkillManagerTab />
        )}
      </Box>
    </Box>
  );
}
