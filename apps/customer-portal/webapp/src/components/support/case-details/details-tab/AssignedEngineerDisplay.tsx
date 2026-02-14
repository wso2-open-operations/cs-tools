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

import { Avatar, Stack, Typography, colors } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { formatValue, getInitials } from "@utils/support";

export interface AssignedEngineerDisplayProps {
  assignedEngineer: string | null | undefined;
}

/**
 * Displays the assigned engineer with Avatar (initials) and name.
 *
 * @param {AssignedEngineerDisplayProps} props - Assigned engineer name.
 * @returns {JSX.Element} Avatar and name display.
 */
export default function AssignedEngineerDisplay({
  assignedEngineer,
}: AssignedEngineerDisplayProps): JSX.Element {
  const initials = getInitials(assignedEngineer);

  return (
    <Stack direction="row" alignItems="center" spacing={1.5}>
      <Avatar
        sx={{
          width: 24,
          height: 24,
          fontSize: "0.75rem",
          bgcolor: colors.blue[100],
          color: colors.blue[700],
        }}
      >
        {initials}
      </Avatar>
      <Typography variant="body2" color="text.primary">
        {formatValue(assignedEngineer)}
      </Typography>
    </Stack>
  );
}
