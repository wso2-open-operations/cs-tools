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
  Paper,
  TextField,
  InputAdornment,
} from "@wso2/oxygen-ui";
import { Search } from "@wso2/oxygen-ui-icons-react";
import type { JSX, ChangeEvent } from "react";
import type { ServiceRequestStatusFilter } from "@pages/ServiceRequestsPage";

export interface ServiceRequestStats {
  pending: number;
  inProgress: number;
  completed: number;
  rejected: number;
}

export interface ServiceRequestsSearchBarProps {
  searchTerm: string;
  onSearchChange: (value: string) => void;
  statusFilter: ServiceRequestStatusFilter;
  onStatusFilterChange: (value: ServiceRequestStatusFilter) => void;
  stats?: ServiceRequestStats;
}

const STATUS_TABS: { value: ServiceRequestStatusFilter; label: string }[] = [
  { value: "all", label: "All" },
  { value: "pending", label: "Pending" },
  { value: "in_progress", label: "In Progress" },
  { value: "completed", label: "Completed" },
];

const TAB_VALUE_TO_STATS_KEY: Record<
  Exclude<ServiceRequestStatusFilter, "all">,
  keyof ServiceRequestStats
> = {
  pending: "pending",
  in_progress: "inProgress",
  completed: "completed",
};

/**
 * Search bar with status filter tabs for Service Requests.
 *
 * @param {ServiceRequestsSearchBarProps} props - Search and filter props.
 * @returns {JSX.Element} The rendered search bar.
 */
export default function ServiceRequestsSearchBar({
  searchTerm,
  onSearchChange,
  statusFilter,
  onStatusFilterChange,
  stats,
}: ServiceRequestsSearchBarProps): JSX.Element {
  const handleSearchChange = (event: ChangeEvent<HTMLInputElement>) => {
    onSearchChange(event.target.value);
  };

  return (
    <Paper sx={{ p: 2 }}>
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          gap: 2,
        }}
      >
        <Box sx={{ flex: 1 }}>
          <TextField
            fullWidth
            size="small"
            placeholder="Search service requests..."
            value={searchTerm}
            onChange={handleSearchChange}
            slotProps={{
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <Search size={16} />
                  </InputAdornment>
                ),
              },
            }}
          />
        </Box>
        <Box sx={{ display: "flex", gap: 0.5 }}>
          {STATUS_TABS.map((tab) => {
            const count =
              tab.value === "all"
                ? (stats
                    ? stats.pending +
                      stats.inProgress +
                      stats.completed +
                      stats.rejected
                    : 0)
                : stats?.[TAB_VALUE_TO_STATS_KEY[tab.value]] ?? 0;
            const label = stats ? `${tab.label} (${count})` : tab.label;

            return (
              <Button
                key={tab.value}
                size="small"
                variant={statusFilter === tab.value ? "contained" : "text"}
                color={statusFilter === tab.value ? "primary" : "inherit"}
                onClick={() => onStatusFilterChange(tab.value)}
                sx={{
                  textTransform: "none",
                  fontWeight: statusFilter === tab.value ? 600 : 400,
                  minWidth: "auto",
                  px: 2,
                }}
              >
                {label}
              </Button>
            );
          })}
        </Box>
      </Box>
    </Paper>
  );
}
