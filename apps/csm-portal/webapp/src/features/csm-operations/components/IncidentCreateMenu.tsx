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

import { Box, Menu, MenuItem, Button } from "@wso2/oxygen-ui";
import { ChevronDown, Plus } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX, type ReactNode } from "react";

interface CreateMenuItemConfig {
  key: string;
  label: string;
  icon: ReactNode;
  onSelect: () => void;
}

interface IncidentCreateMenuProps {
  items: CreateMenuItemConfig[];
}

/**
 * Groups every "Create X…" action available from the incident detail page's
 * action row under one split button — mirrors ServiceNow's own incident form,
 * whose "Create change request ▾" control expands into Create incident task /
 * Create problem / Create request / Create child incident. This app doesn't
 * carry over "Create incident task" or "Create request" (see
 * `CsmIncidentDetailPage.tsx`'s own doc comments on why — no tasks concept in
 * this platform, and no create-service-request-from-incident flow exists
 * yet), so the menu here covers only the actions this app actually supports:
 * outage, change request, problem, and child incident. Same hand-rolled
 * `Button` + `Menu`/`MenuItem` split-button shape as `IncidentActionBar`'s
 * "Change state" control — there's no dedicated oxygen-ui split-button
 * component to reuse instead.
 */
const CREATE_MENU_ID = "incident-create-menu";

export default function IncidentCreateMenu({
  items,
}: IncidentCreateMenuProps): JSX.Element {
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);

  return (
    <Box sx={{ display: "flex" }}>
      <Button
        variant="outlined"
        size="small"
        startIcon={<Plus size={14} />}
        endIcon={<ChevronDown size={14} />}
        aria-haspopup="menu"
        aria-expanded={!!anchor}
        aria-controls={anchor ? CREATE_MENU_ID : undefined}
        onClick={(e) => setAnchor(e.currentTarget)}
      >
        Create
      </Button>
      <Menu
        id={CREATE_MENU_ID}
        anchorEl={anchor}
        open={!!anchor}
        onClose={() => setAnchor(null)}
        MenuListProps={{ "aria-label": "Create" }}
      >
        {items.map((item) => (
          <MenuItem
            key={item.key}
            onClick={() => {
              setAnchor(null);
              item.onSelect();
            }}
            sx={{ gap: 1.25, minHeight: 36 }}
          >
            <Box sx={{ color: "text.secondary", display: "flex" }}>{item.icon}</Box>
            {item.label}
          </MenuItem>
        ))}
      </Menu>
    </Box>
  );
}
