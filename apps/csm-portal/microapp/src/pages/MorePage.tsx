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

import { Link } from "react-router-dom";
import { List, ListItemButton, ListItemIcon, ListItemText, Stack, Typography } from "@wso2/oxygen-ui";
import {
  Briefcase,
  ChevronRight,
  Clock,
  Megaphone,
  RefreshCw,
  Shield,
  type LucideIcon,
} from "@wso2/oxygen-ui-icons-react";

interface MoreItem {
  label: string;
  path: string;
  icon: LucideIcon;
}

// Secondary sections that don't get their own tab, mirroring a subset of the webapp's
// CSM_NAV_ITEMS (apps/csm-portal/webapp/src/config/csmNavItems.ts) — Customers and Settings are
// deliberately left out here.
const MORE_ITEMS: MoreItem[] = [
  { label: "Announcements", path: "/more/announcements", icon: Megaphone },
  { label: "Time Cards", path: "/more/time-cards", icon: Clock },
  { label: "Security Center", path: "/more/security-center", icon: Shield },
  { label: "Updates", path: "/more/updates", icon: RefreshCw },
  { label: "Engagements", path: "/more/engagements", icon: Briefcase },
];

export default function MorePage() {
  return (
    <Stack gap={2}>
      <Typography variant="h5">More</Typography>

      <List disablePadding>
        {MORE_ITEMS.map((item) => (
          <ListItemButton key={item.path} component={Link} to={item.path} sx={{ borderRadius: 1 }}>
            <ListItemIcon sx={{ minWidth: 40 }}>
              <item.icon size={20} />
            </ListItemIcon>
            <ListItemText primary={item.label} />
            <ChevronRight size={18} />
          </ListItemButton>
        ))}
      </List>
    </Stack>
  );
}
