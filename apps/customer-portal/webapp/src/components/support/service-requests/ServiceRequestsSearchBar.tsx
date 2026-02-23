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

import { Box, Paper, TextField, InputAdornment, Tabs, Tab } from "@wso2/oxygen-ui";
import { Search } from "@wso2/oxygen-ui-icons-react";
import type { JSX, ChangeEvent, SyntheticEvent } from "react";
import type { ServiceRequestStatusFilter } from "@pages/ServiceRequestsPage";

export interface ServiceRequestsSearchBarProps {
  searchTerm: string;
  onSearchChange: (value: string) => void;
  statusFilter: ServiceRequestStatusFilter;
  onStatusFilterChange: (value: ServiceRequestStatusFilter) => void;
}

const STATUS_TABS: { value: ServiceRequestStatusFilter; label: string }[] = [
  { value: "all", label: "All" },
  { value: "pending", label: "Pending" },
  { value: "in_progress", label: "In Progress" },
  { value: "completed", label: "Completed" },
];

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
}: ServiceRequestsSearchBarProps): JSX.Element {
  const handleSearchChange = (event: ChangeEvent<HTMLInputElement>) => {
    onSearchChange(event.target.value);
  };

  const handleTabChange = (
    _event: SyntheticEvent,
    newValue: ServiceRequestStatusFilter,
  ) => {
    onStatusFilterChange(newValue);
  };

  const currentTabIndex = STATUS_TABS.findIndex((t) => t.value === statusFilter);

  return (
    <Paper sx={{ p: 2 }}>
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          gap: 2,
          flexWrap: "wrap",
        }}
      >
        <Box sx={{ flex: 1, minWidth: 200 }}>
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
        <Tabs
          value={currentTabIndex >= 0 ? currentTabIndex : 0}
          onChange={(_e, idx) => handleTabChange(_e, STATUS_TABS[idx].value)}
          sx={{
            minHeight: 36,
            "& .MuiTabs-indicator": {
              display: "none",
            },
          }}
        >
          {STATUS_TABS.map((tab) => (
            <Tab
              key={tab.value}
              label={tab.label}
              sx={{
                minHeight: 36,
                px: 2,
                py: 0.5,
                textTransform: "none",
                fontSize: "0.875rem",
                fontWeight: statusFilter === tab.value ? 600 : 400,
                color:
                  statusFilter === tab.value
                    ? "primary.main"
                    : "text.secondary",
                bgcolor:
                  statusFilter === tab.value ? "action.selected" : "transparent",
              }}
            />
          ))}
        </Tabs>
      </Box>
    </Paper>
  );
}
