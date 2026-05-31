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

import { Box, Button, Chip, TextField, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import type {
  CaseState,
  DashboardScope,
  Severity,
} from "@features/csm-dashboard/types/abtDashboard";
import { STATE_LABEL } from "@features/csm-dashboard/utils/abtDashboard";

export type SlaFilter = "any" | "breached" | "at_risk";
export type OwnerFilter = "anyone" | "me" | "unassigned";

export interface CasesFilters {
  scope: DashboardScope;
  search: string;
  severities: Severity[];
  states: CaseState[];
  sla: SlaFilter;
  owner: OwnerFilter;
}

interface CasesFilterBarProps {
  filters: CasesFilters;
  onChange: (next: CasesFilters) => void;
  onReset: () => void;
}

const ALL_SEVERITIES: Severity[] = ["S0", "S1", "S2", "S3", "S4"];
const PRIMARY_STATES: CaseState[] = [
  "open",
  "work_in_progress",
  "awaiting_info",
  "solution_proposed",
  "waiting_on_wso2",
  "reopen",
  "closed",
];

function toggle<T>(arr: T[], value: T): T[] {
  return arr.includes(value) ? arr.filter((v) => v !== value) : [...arr, value];
}

export default function CasesFilterBar({
  filters,
  onChange,
  onReset,
}: CasesFilterBarProps): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
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
            placeholder="Search case # / subject / customer…"
            value={filters.search}
            onChange={(e) =>
              onChange({ ...filters, search: e.target.value })
            }
          />
        </Box>
        <Button size="small" variant="text" onClick={onReset}>
          Reset
        </Button>
      </Box>

      <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
        <Typography variant="caption" color="text.secondary" sx={{ mr: 0.5 }}>
          Severity:
        </Typography>
        {ALL_SEVERITIES.map((sev) => {
          const active = filters.severities.includes(sev);
          return (
            <Chip
              key={sev}
              size="small"
              label={sev}
              clickable
              color={active ? (sev === "S0" || sev === "S1" ? "error" : sev === "S2" ? "warning" : "primary") : "default"}
              variant={active ? "filled" : "outlined"}
              onClick={() =>
                onChange({ ...filters, severities: toggle(filters.severities, sev) })
              }
            />
          );
        })}

        <Box sx={{ width: 12 }} />

        <Typography variant="caption" color="text.secondary" sx={{ mr: 0.5 }}>
          SLA:
        </Typography>
        {(["any", "at_risk", "breached"] as SlaFilter[]).map((s) => (
          <Chip
            key={s}
            size="small"
            label={s === "any" ? "Any" : s === "at_risk" ? "At risk" : "Breached"}
            clickable
            color={filters.sla === s ? (s === "breached" ? "error" : s === "at_risk" ? "warning" : "primary") : "default"}
            variant={filters.sla === s ? "filled" : "outlined"}
            onClick={() => onChange({ ...filters, sla: s })}
          />
        ))}

        <Box sx={{ width: 12 }} />

        <Typography variant="caption" color="text.secondary" sx={{ mr: 0.5 }}>
          Owner:
        </Typography>
        {(["anyone", "me", "unassigned"] as OwnerFilter[]).map((o) => (
          <Chip
            key={o}
            size="small"
            label={o === "anyone" ? "Anyone" : o === "me" ? "Me" : "Unassigned"}
            clickable
            color={filters.owner === o ? "primary" : "default"}
            variant={filters.owner === o ? "filled" : "outlined"}
            onClick={() => onChange({ ...filters, owner: o })}
          />
        ))}
      </Box>

      <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
        <Typography variant="caption" color="text.secondary" sx={{ mr: 0.5 }}>
          State:
        </Typography>
        {PRIMARY_STATES.map((st) => {
          const active = filters.states.includes(st);
          return (
            <Chip
              key={st}
              size="small"
              label={STATE_LABEL[st]}
              clickable
              color={active ? "primary" : "default"}
              variant={active ? "filled" : "outlined"}
              onClick={() =>
                onChange({ ...filters, states: toggle(filters.states, st) })
              }
            />
          );
        })}
      </Box>
    </Box>
  );
}
