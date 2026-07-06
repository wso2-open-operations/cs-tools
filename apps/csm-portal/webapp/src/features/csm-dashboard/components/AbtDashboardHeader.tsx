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

import {
  Box,
  Button,
  Chip,
  FormControl,
  MenuItem,
  Select,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import type { JSX } from "react";
import {
  DASHBOARD_OPTIONS,
  type CsmDashboardEngineer,
  type DashboardKey,
  type DashboardScope,
} from "@features/csm-dashboard/types/abtDashboard";

// ABT (Account-Based Team) scoping is not implemented yet, so the My ABT / All
// customers toggle is disabled and the dashboard is locked to "all customers".
// Flip this to re-enable the toggle once ABT membership data is available.
const ABT_SCOPING_ENABLED = false;

// Only the Engineer dashboard is live; the others are mock placeholders, so the
// switcher is disabled and locked to Engineer. Flip this to re-enable the
// dropdown once the other dashboards are real.
const DASHBOARD_SWITCHER_ENABLED = false;

interface AbtDashboardHeaderProps {
  engineer?: CsmDashboardEngineer;
  scope: DashboardScope;
  onScopeChange: (scope: DashboardScope) => void;
  dashboardKey: DashboardKey;
  onDashboardChange: (key: DashboardKey) => void;
  isError?: boolean;
}

export default function AbtDashboardHeader({
  engineer,
  scope,
  onScopeChange,
  dashboardKey,
  onDashboardChange,
  isError,
}: AbtDashboardHeaderProps): JSX.Element {
  const currentOption = DASHBOARD_OPTIONS.find((o) => o.key === dashboardKey);
  const showScopeButtons = currentOption?.scopeBased ?? false;

  return (
    <Box
      sx={{
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        gap: 2,
        flexWrap: "wrap",
      }}
    >
      <Box>
        <Typography variant="h5">Dashboard</Typography>
        <Box
          sx={{ display: "flex", alignItems: "center", gap: 1, mt: 0.5 }}
        >
          <Typography variant="body2" color="text.secondary">
            {engineer
              ? engineer.name
              : isError
                ? "Engineer overview"
                : "Loading engineer…"}
          </Typography>
          {engineer?.abtName && (
            <Chip
              size="small"
              label={engineer.abtName}
              variant="outlined"
              color="primary"
            />
          )}
        </Box>
      </Box>
      <Box sx={{ display: "flex", gap: 1, alignItems: "center", flexWrap: "wrap" }}>
        {showScopeButtons && (
          <Tooltip
            title={
              ABT_SCOPING_ENABLED
                ? ""
                : "ABT scoping is not available yet — showing all customers."
            }
          >
            <Box sx={{ display: "flex", gap: 1 }}>
              <Button
                size="small"
                variant={scope === "my_abt" ? "contained" : "outlined"}
                color="primary"
                disabled={!ABT_SCOPING_ENABLED}
                onClick={() => onScopeChange("my_abt")}
              >
                My ABT
              </Button>
              <Button
                size="small"
                variant={scope === "all_customers" ? "contained" : "outlined"}
                color="primary"
                disabled={!ABT_SCOPING_ENABLED}
                onClick={() => onScopeChange("all_customers")}
              >
                All customers
              </Button>
            </Box>
          </Tooltip>
        )}
        <Tooltip
          title={
            DASHBOARD_SWITCHER_ENABLED
              ? ""
              : "Other dashboards are not available yet — showing the Engineer dashboard."
          }
        >
          <FormControl size="small" sx={{ minWidth: 200 }}>
            <Select
              value={dashboardKey}
              onChange={(e) =>
                onDashboardChange(e.target.value as DashboardKey)
              }
              disabled={!DASHBOARD_SWITCHER_ENABLED}
              displayEmpty
            >
              {DASHBOARD_OPTIONS.map((o) => (
                <MenuItem key={o.key} value={o.key}>
                  {o.name}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
        </Tooltip>
      </Box>
    </Box>
  );
}
