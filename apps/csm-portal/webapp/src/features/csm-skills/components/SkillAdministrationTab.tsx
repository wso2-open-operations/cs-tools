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

import { Box, Chip, Tab, Tabs } from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import AllSkillsTab from "@features/csm-skills/components/AllSkillsTab";
import SkillCategoriesAdmin from "@features/csm-skills/components/SkillCategoriesAdmin";
import SkillRequestsQueue from "@features/csm-skills/components/SkillRequestsQueue";
import type { SkillCapabilities } from "@features/csm-skills/types";

type AdminSection = "requests" | "skills" | "categories";

/**
 * Administration: one sub-tab per gated task, each shown only to a holder of
 * its capability. Requests (reviewers) carries the pending count; Skills and
 * Categories are the catalogue managers'.
 */
export default function SkillAdministrationTab({ capabilities }: { capabilities: SkillCapabilities }): JSX.Element {
  const sections: AdminSection[] = [
    ...(capabilities.canReviewRequests ? (["requests"] as const) : []),
    ...(capabilities.canManageCatalogue ? (["skills", "categories"] as const) : []),
  ];
  const [chosen, setChosen] = useState<AdminSection | null>(null);
  const section = chosen && sections.includes(chosen) ? chosen : sections[0];
  const pending = capabilities.pendingRequestCount ?? 0;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Tabs
        aria-label="Administration sections"
        value={section}
        onChange={(_, v: AdminSection) => setChosen(v)}
        sx={{ minHeight: 36, "& .MuiTab-root": { minHeight: 36 } }}
      >
        {sections.includes("requests") && (
          <Tab
            value="requests"
            id="skills-admin-requests"
            label={
              <Box component="span" sx={{ display: "inline-flex", alignItems: "center", gap: 1 }}>
                Requests
                {pending > 0 && <Chip size="small" color="warning" label={pending} aria-label={`${pending} pending`} />}
              </Box>
            }
          />
        )}
        {sections.includes("skills") && <Tab value="skills" id="skills-admin-skills" label="Skills" />}
        {sections.includes("categories") && <Tab value="categories" id="skills-admin-categories" label="Categories" />}
      </Tabs>
      <Box role="tabpanel" aria-labelledby={`skills-admin-${section}`}>
        {section === "requests" ? (
          <SkillRequestsQueue />
        ) : section === "skills" ? (
          <AllSkillsTab />
        ) : (
          <SkillCategoriesAdmin />
        )}
      </Box>
    </Box>
  );
}
