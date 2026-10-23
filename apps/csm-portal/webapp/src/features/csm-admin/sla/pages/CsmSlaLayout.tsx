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

import { Box, Tab, Tabs, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { Outlet, useLocation, useNavigate } from "react-router";

/** The SLA admin area's own four sub-sections, under one `admin.sla` nav
 *  entry — mirroring how Dashboard Builder (list/new/:draftId/shared) uses
 *  several sub-pages under one top-level nav node rather than registering a
 *  separate one per sub-area. */
const SLA_TABS = [
  { path: "/admin/sla/definitions", label: "Definitions" },
  { path: "/admin/sla/calendars", label: "Calendars" },
  { path: "/admin/sla/fields", label: "Field Registry" },
  { path: "/admin/sla/notification-rules", label: "Notification Rules" },
];

export default function CsmSlaLayout(): JSX.Element {
  const location = useLocation();
  const navigate = useNavigate();
  const current = SLA_TABS.find((t) => location.pathname.startsWith(t.path))?.path ?? SLA_TABS[0].path;

  return (
    <Box>
      <Typography variant="h5" sx={{ mb: 1 }}>
        SLA
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
        Define the calendars, field registry, SLA definitions and notification rules the SLA module tracks cases
        and incidents against. Nothing here takes effect until a definition is published.
      </Typography>
      <Tabs value={current} onChange={(_e, value) => navigate(value)} sx={{ mb: 2 }}>
        {SLA_TABS.map((t) => (
          <Tab key={t.path} value={t.path} label={t.label} />
        ))}
      </Tabs>
      <Outlet />
    </Box>
  );
}
