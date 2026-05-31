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

import { Box, Button, Chip, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import type {
  CsmDashboardEngineer,
  DashboardScope,
} from "@features/csm-dashboard/types/abtDashboard";

interface AbtDashboardHeaderProps {
  engineer?: CsmDashboardEngineer;
  scope: DashboardScope;
  onScopeChange: (scope: DashboardScope) => void;
}

export default function AbtDashboardHeader({
  engineer,
  scope,
  onScopeChange,
}: AbtDashboardHeaderProps): JSX.Element {
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
            {engineer ? engineer.name : "Loading engineer…"}
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
      <Box sx={{ display: "flex", gap: 1 }}>
        <Button
          size="small"
          variant={scope === "my_abt" ? "contained" : "outlined"}
          color="primary"
          onClick={() => onScopeChange("my_abt")}
        >
          My ABT
        </Button>
        <Button
          size="small"
          variant={scope === "all_customers" ? "contained" : "outlined"}
          color="primary"
          onClick={() => onScopeChange("all_customers")}
        >
          All customers
        </Button>
      </Box>
    </Box>
  );
}
