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

import { Box, Card, CardActionArea, Chip, Typography } from "@wso2/oxygen-ui";
import { LayoutGrid } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { useNavigate } from "react-router";
import { useRouteTabs } from "@hooks/useSectionTabs";

/**
 * Landing page for Settings → User management: a tile per directory
 * (Users/Roles/Groups/Teams/Permissions) instead of the tab strip these used
 * to share. Reuses `useRouteTabs("admin.user-management")` — the same
 * nav-tree-driven, `CSM_PORTAL_FEATURE_OVERRIDES`-filtered list the old
 * secondary tab strip rendered — so which tiles appear (and which show as
 * "WIP") is still decided in one place, not duplicated here.
 */
export default function CsmUserManagementLandingPage(): JSX.Element {
  const navigate = useNavigate();
  const { tabs } = useRouteTabs("admin.user-management");

  return (
    <Box
      sx={{
        display: "grid",
        // Fixed at 4 per row from "md" up rather than `auto-fill`, which
        // packed in as many 200px-min tiles as the viewport allowed (5 on a
        // typical desktop width) -- inconsistent with every other card grid
        // in the app, which caps at a specific column count per breakpoint.
        gridTemplateColumns: {
          xs: "1fr",
          sm: "repeat(2, minmax(0, 1fr))",
          md: "repeat(4, minmax(0, 1fr))",
        },
        gap: 2,
      }}
    >
      {tabs.map((tab) => {
        const Icon = tab.node.icon ?? LayoutGrid;
        const isWip = tab.state === "wip";
        return (
          <Card key={tab.key} variant="outlined">
            <CardActionArea
              disabled={isWip}
              onClick={() => navigate(tab.key)}
              sx={{
                p: 3,
                display: "flex",
                flexDirection: "column",
                alignItems: "flex-start",
                gap: 1.5,
                minHeight: 120,
              }}
            >
              <Icon size={24} />
              <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>
                  {tab.label}
                </Typography>
                {isWip && (
                  <Chip
                    size="small"
                    label="WIP"
                    color="warning"
                    variant="outlined"
                    sx={{ height: 18, fontSize: 10 }}
                  />
                )}
              </Box>
            </CardActionArea>
          </Card>
        );
      })}
    </Box>
  );
}
