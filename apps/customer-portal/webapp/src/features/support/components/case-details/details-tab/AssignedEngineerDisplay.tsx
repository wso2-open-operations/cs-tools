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

import type { AssignedEngineerDisplayProps } from "@features/support/types/supportComponents";
import { Avatar, Stack, Typography, alpha, useTheme } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { formatValue, getInitials } from "@features/support/utils/support";

/**
 * Displays the assigned engineer with Avatar (initials) and name.
 *
 * @param {AssignedEngineerDisplayProps} props - Assigned engineer name.
 * @returns {JSX.Element} Avatar and name display.
 */
export default function AssignedEngineerDisplay({
  assignedEngineer,
}: AssignedEngineerDisplayProps): JSX.Element {
  const theme = useTheme();
  const initials = getInitials(assignedEngineer);

  return (
    <Stack spacing={0.5}>
      <Stack direction="row" alignItems="center" spacing={1}>
        <Avatar
          sx={{
            width: 32,
            height: 32,
            fontSize: "0.75rem",
            flexShrink: 0,
            bgcolor: alpha(theme.palette.info?.light ?? "#0288d1", 0.2),
            color: theme.palette.info?.main ?? "#0288d1",
          }}
        >
          {initials}
        </Avatar>
        <Typography variant="body2" color="text.primary">
          {formatValue(assignedEngineer)}
        </Typography>
      </Stack>
    </Stack>
  );
}
