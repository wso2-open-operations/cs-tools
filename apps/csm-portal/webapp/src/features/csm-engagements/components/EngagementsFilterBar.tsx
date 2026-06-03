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
  Card,
  Chip,
  MenuItem,
  TextField,
} from "@wso2/oxygen-ui";
import { Search, X } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import {
  ENGAGEMENT_STATES_ALL,
  ENGAGEMENT_STATE_LABEL,
  ENGAGEMENT_TYPES_ALL,
  ENGAGEMENT_TYPE_LABEL,
} from "@features/csm-engagements/utils/engagements";
import type {
  CsmEngagementListFilters,
  CsmEngagementState,
  CsmEngagementType,
} from "@features/csm-engagements/types/csmEngagements";

interface EngagementsFilterBarProps {
  filters: CsmEngagementListFilters;
  onChange: (next: CsmEngagementListFilters) => void;
  onReset: () => void;
}

function toggle<T>(arr: T[], v: T): T[] {
  return arr.includes(v) ? arr.filter((x) => x !== v) : [...arr, v];
}

export default function EngagementsFilterBar({
  filters,
  onChange,
  onReset,
}: EngagementsFilterBarProps): JSX.Element {
  const hasRefinement =
    filters.search.trim() !== "" ||
    filters.types.length > 0 ||
    filters.states.length > 0 ||
    filters.stages.length > 0 ||
    filters.assignee !== "all" ||
    filters.health !== "all";

  return (
    <Card variant="outlined" sx={{ p: 1.5, display: "flex", flexDirection: "column", gap: 1.25 }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
        <TextField
          size="small"
          placeholder="Search by name, reference, customer…"
          value={filters.search}
          onChange={(e) => onChange({ ...filters, search: e.target.value })}
          InputProps={{
            startAdornment: (
              <Box sx={{ display: "flex", color: "text.secondary", mr: 0.75 }}>
                <Search size={16} />
              </Box>
            ),
          }}
          sx={{ flex: 1, minWidth: 240 }}
        />
        <TextField
          select
          size="small"
          label="Assignee"
          value={filters.assignee}
          onChange={(e) =>
            onChange({ ...filters, assignee: e.target.value as CsmEngagementListFilters["assignee"] })
          }
          sx={{ minWidth: 140 }}
        >
          <MenuItem value="all">Any assignee</MenuItem>
          <MenuItem value="me">Owned by me</MenuItem>
          <MenuItem value="unassigned">Unassigned</MenuItem>
        </TextField>
        <TextField
          select
          size="small"
          label="Health"
          value={filters.health}
          onChange={(e) =>
            onChange({ ...filters, health: e.target.value as CsmEngagementListFilters["health"] })
          }
          sx={{ minWidth: 130 }}
        >
          <MenuItem value="all">Any health</MenuItem>
          <MenuItem value="green">On track</MenuItem>
          <MenuItem value="amber">At risk</MenuItem>
          <MenuItem value="red">Off track</MenuItem>
        </TextField>
        {hasRefinement && (
          <Button
            size="small"
            variant="text"
            startIcon={<X size={14} />}
            onClick={onReset}
            sx={{ ml: "auto" }}
          >
            Clear
          </Button>
        )}
      </Box>

      <Box sx={{ display: "flex", flexWrap: "wrap", gap: 0.75, alignItems: "center" }}>
        <Box component="span" sx={{ fontSize: "0.75rem", color: "text.secondary", mr: 0.5 }}>
          Type:
        </Box>
        {ENGAGEMENT_TYPES_ALL.map((t) => {
          const active = filters.types.includes(t);
          return (
            <Chip
              key={t}
              size="small"
              label={ENGAGEMENT_TYPE_LABEL[t]}
              variant={active ? "filled" : "outlined"}
              color={active ? "primary" : "default"}
              onClick={() =>
                onChange({ ...filters, types: toggle<CsmEngagementType>(filters.types, t) })
              }
            />
          );
        })}
      </Box>
      <Box sx={{ display: "flex", flexWrap: "wrap", gap: 0.75, alignItems: "center" }}>
        <Box component="span" sx={{ fontSize: "0.75rem", color: "text.secondary", mr: 0.5 }}>
          State:
        </Box>
        {ENGAGEMENT_STATES_ALL.map((s) => {
          const active = filters.states.includes(s);
          return (
            <Chip
              key={s}
              size="small"
              label={ENGAGEMENT_STATE_LABEL[s]}
              variant={active ? "filled" : "outlined"}
              color={active ? "primary" : "default"}
              onClick={() =>
                onChange({ ...filters, states: toggle<CsmEngagementState>(filters.states, s) })
              }
            />
          );
        })}
      </Box>
    </Card>
  );
}
