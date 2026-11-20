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
import type { JSX } from "react";
import TechnicalTeamBadge from "@components/TechnicalTeamBadge";
import { hasStaffNameMarker, stripStaffNameMarker } from "@utils/staffNameMarker";

interface StaffNameProps {
  /** A person's display name as stored, possibly carrying the staff marker. */
  name: string | null | undefined;
  /** Shown when there is no name (default "—"). */
  fallback?: string;
}

/**
 * A person's name in a list cell: the name without the circled-W staff
 * marker, followed by the Technical Team badge when the marker was there.
 * One line, truncated with an ellipsis, the full name on hover.
 */
export default function StaffName({ name, fallback = "—" }: StaffNameProps): JSX.Element {
  const display = stripStaffNameMarker(name);
  const staff = hasStaffNameMarker(name);
  return (
    <Box sx={{ display: "flex", alignItems: "center", gap: 0.5, minWidth: 0 }}>
      <Typography variant="body2" noWrap title={display || undefined}>
        {display || fallback}
      </Typography>
      {staff && <TechnicalTeamBadge />}
    </Box>
  );
}
