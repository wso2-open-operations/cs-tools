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

import { Box, Button, TextField } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import type { DashboardScope } from "@features/csm-dashboard/types/abtDashboard";

export interface ProjectsFilters {
  scope: DashboardScope;
  search: string;
}

interface ProjectsFilterBarProps {
  filters: ProjectsFilters;
  onChange: (next: ProjectsFilters) => void;
  onReset: () => void;
}

export default function ProjectsFilterBar({
  filters,
  onChange,
  onReset,
}: ProjectsFilterBarProps): JSX.Element {
  return (
    <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
      <Button
        size="small"
        variant={filters.scope === "my_abt" ? "contained" : "outlined"}
        color="primary"
        onClick={() => onChange({ ...filters, scope: "my_abt" })}
      >
        My ABT
      </Button>
      <Button
        size="small"
        variant={filters.scope === "all_customers" ? "contained" : "outlined"}
        color="primary"
        onClick={() => onChange({ ...filters, scope: "all_customers" })}
      >
        All customers
      </Button>
      <Box sx={{ flex: 1, minWidth: 240 }}>
        <TextField
          size="small"
          fullWidth
          placeholder="Search project / customer / product…"
          value={filters.search}
          onChange={(e) => onChange({ ...filters, search: e.target.value })}
        />
      </Box>
      <Button size="small" variant="text" onClick={onReset}>
        Reset
      </Button>
    </Box>
  );
}
