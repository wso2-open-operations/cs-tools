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
  onInstalledClick,
  onPendingClick,
}: UpdateCardBreakdownProps): JSX.Element {
  const theme = useTheme();

  return (
    <Grid container spacing={1} sx={{ mb: 2 }}>
      <Grid size={{ xs: 6 }}>
        <Paper
          onClick={onInstalledClick}
          variant="outlined"
          sx={{
            p: 1.5,
            textAlign: "center",
            border: "1px solid",
            borderColor: alpha(theme.palette.success.main, 0.35),
            bgcolor: alpha(theme.palette.success.light, 0.08),
            cursor: onInstalledClick ? "pointer" : "default",
            transition: "all 0.2s ease",
            ...(onInstalledClick && {
              "&:hover": {
                bgcolor: alpha(theme.palette.success.light, 0.2),
                borderColor: "success.main",
                boxShadow: `0 2px 8px ${alpha(theme.palette.success.main, 0.25)}`,
                transform: "translateY(-1px)",
              },
              "&:active": {
                transform: "translateY(0)",
                boxShadow: "none",
              },
            }),
          }}
        >
          <Typography variant="caption" color="text.secondary" display="block" sx={{ mb: 0.25 }}>
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
          onClick={onPendingClick}
          variant="outlined"
          sx={{
            p: 1.5,
            textAlign: "center",
            border: "1px solid",
            borderColor: alpha(theme.palette.warning.main, 0.35),
            bgcolor: alpha(theme.palette.warning.light, 0.08),
            cursor: onPendingClick ? "pointer" : "default",
            transition: "all 0.2s ease",
            ...(onPendingClick && {
              "&:hover": {
                bgcolor: alpha(theme.palette.warning.light, 0.2),
                borderColor: "warning.main",
                boxShadow: `0 2px 8px ${alpha(theme.palette.warning.main, 0.25)}`,
                transform: "translateY(-1px)",
              },
              "&:active": {
                transform: "translateY(0)",
                boxShadow: "none",
              },
            }),
          }}
        >
          <Typography variant="caption" color="text.secondary" display="block" sx={{ mb: 0.25 }}>
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
