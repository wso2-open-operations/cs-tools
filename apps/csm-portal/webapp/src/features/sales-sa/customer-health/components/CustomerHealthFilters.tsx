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

import { Typography, Box, FormControl, Select, MenuItem, ListItemText, type SelectChangeEvent } from "@wso2/oxygen-ui";
import { useTheme } from "@mui/material/styles";
import CustomerHealthToggle, { type AccountScopeState } from "./CustomerHealthToggle";
import RiskIndicatorSelect from "./RiskIndicatorSelect";
import RegionFilterSelect from "./RegionFilterSelect";
import ProductFilterSelect from "./ProductFilterSelect";
import AbtTeamFilterSelect from "./AbtTeamFilterSelect";
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import type { RiskFilterKey } from "../api/customerHealthTypes";

export interface HealthFilters {
  accountScope: "my-accounts" | "all-accounts";
  userEmail: string | null;
  riskIndicators: RiskFilterKey[];
  region: string[];
  product: string | null;
  abtTeam: string | null;
  healthStatus: string | null;
}

function HealthStatusFilterSelect({
  selectedStatus,
  onStatusChange,
}: {
  selectedStatus: string | null;
  onStatusChange: (newStatus: string | null) => void;
}) {
  const handleChange = (event: SelectChangeEvent<string>) => {
    onStatusChange(event.target.value || null);
  };
  return (
    <FormControl sx={{ m: 1, width: 180 }} size="small">
      <Select
        value={selectedStatus || ""}
        onChange={handleChange}
        displayEmpty
        renderValue={(selected) => {
          if (!selected) return "All Statuses";
          return selected === "at_risk" ? "At Risk" : "Healthy";
        }}
      >
        <MenuItem value="">
          <ListItemText primary="All Statuses" />
        </MenuItem>
        <MenuItem value="at_risk">
          <ListItemText primary="At Risk" />
        </MenuItem>
        <MenuItem value="healthy">
          <ListItemText primary="Healthy" />
        </MenuItem>
      </Select>
    </FormControl>
  );
}

interface CustomerHealthFiltersProps {
  onFilterChange: (filters: HealthFilters) => void;
  currentFilters: HealthFilters;
}

export default function CustomerHealthFilters({ onFilterChange, currentFilters }: CustomerHealthFiltersProps) {
  // Source read the current user's email via a dedicated useAsgardeoUser
  // hook; this app's equivalent is useIdTokenClaims (decodes the same
  // id_token claim), already used elsewhere in this codebase.
  const userEmail = useIdTokenClaims()?.email;
  const theme = useTheme();

  const handleAccountScopeToggleClick = (newState: AccountScopeState) => {
    onFilterChange({ ...currentFilters, accountScope: newState, userEmail: userEmail ?? null });
  };
  const handleRiskChange = (newRisks: RiskFilterKey[]) => {
    onFilterChange({ ...currentFilters, riskIndicators: newRisks });
  };
  const handleRegionChange = (newRegions: string[]) => {
    onFilterChange({ ...currentFilters, region: newRegions });
  };
  const handleProductChange = (newProduct: string | null) => {
    onFilterChange({ ...currentFilters, product: newProduct });
  };
  const handleABTTeamChange = (newTeam: string | null) => {
    onFilterChange({ ...currentFilters, abtTeam: newTeam });
  };
  const handleHealthStatusChange = (newStatus: string | null) => {
    onFilterChange({ ...currentFilters, healthStatus: newStatus });
  };

  return (
    <Box sx={{ my: 2, p: 2, border: `1px solid ${theme.palette.divider}`, borderRadius: "4px", display: "flex", gap: 2, alignItems: "center", flexWrap: "wrap" }}>
      <Typography variant="h6">Filters</Typography>
      <CustomerHealthToggle page={currentFilters.accountScope || "all-accounts"} onSwitchClick={handleAccountScopeToggleClick} />
      <RiskIndicatorSelect selectedRisks={currentFilters.riskIndicators || []} onRiskChange={handleRiskChange} />
      <RegionFilterSelect selectedRegions={currentFilters.region || []} onRegionChange={handleRegionChange} />
      <ProductFilterSelect selectedProduct={currentFilters.product || null} onProductChange={handleProductChange} />
      <AbtTeamFilterSelect selectedTeam={currentFilters.abtTeam || null} onTeamChange={handleABTTeamChange} />
      <HealthStatusFilterSelect selectedStatus={currentFilters.healthStatus || null} onStatusChange={handleHealthStatusChange} />
    </Box>
  );
}
