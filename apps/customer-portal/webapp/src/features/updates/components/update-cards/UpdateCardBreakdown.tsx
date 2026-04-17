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

import { Grid, Paper, Typography, alpha, useTheme } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import type { UpdateCardBreakdownProps } from "@features/updates/types/updates";

/**
 * Component to display installed and pending update breakdowns.
 *
 * @param {UpdateCardBreakdownProps} props - Component props.
 * @returns {JSX.Element} The rendered component.
 */
export function UpdateCardBreakdown({
  installedRegular,
  installedSecurity,
  pendingRegular,
  pendingSecurity,
  totalPending,
}: UpdateCardBreakdownProps): JSX.Element {
  const theme = useTheme();

  return (
    <Grid container spacing={1} sx={{ mb: 2 }}>
      <Grid size={{ xs: 6 }}>
        <Paper
          sx={{
            p: 1,
            textAlign: "center",
            borderColor: "success.light",
            bgcolor: alpha(theme.palette["success"].light, 0.1),
          }}
        >
          <Typography variant="caption" color="text.secondary" display="block">
            Installed
          </Typography>
          <Typography variant="body2" color="success.main" fontWeight="bold">
            {installedRegular + installedSecurity}
          </Typography>
          <Typography variant="caption" color="text.secondary">
            {installedRegular}R • {installedSecurity}S
          </Typography>
        </Paper>
      </Grid>
      <Grid size={{ xs: 6 }}>
        <Paper
          sx={{
            p: 1,
            textAlign: "center",
            borderColor: "warning.light",
            bgcolor: alpha(theme.palette["warning"].light, 0.1),
          }}
        >
          <Typography variant="caption" color="text.secondary" display="block">
            Pending
          </Typography>
          <Typography variant="body2" color="warning.main" fontWeight="bold">
            {totalPending}
          </Typography>
          <Typography variant="caption" color="text.secondary">
            {pendingRegular}R • {pendingSecurity}S
          </Typography>
        </Paper>
      </Grid>
    </Grid>
  );
}
