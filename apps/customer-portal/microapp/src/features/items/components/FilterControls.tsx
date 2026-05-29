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
import { CASE_TYPES } from "@root/src/shared/constants";
import { SearchBar, Stack, Tab, Tabs } from "@wso2/oxygen-ui";

import { useFilters } from "@features/items/hooks";

export type FilterControlsVariant = "full" | "tabs-only" | "search-only";

interface FilterTab {
  label: string;
  value: string;
}

export interface FilterControlsProps {
  tabs: FilterTab[];
  placeholder?: string;
  variant?: FilterControlsVariant;
}

export function FilterControls({ variant = "full", tabs, placeholder }: FilterControlsProps) {
  const { filters, set } = useFilters();
  const showSearch = variant !== "tabs-only"; /** Hide the search bar */
  const showTabs = variant !== "search-only"; /** Hide the filter tabs section */

  return (
    <Stack p={1} pb={0} mt={-1}>
      {showSearch && (
        <SearchBar
          fullWidth
          size="small"
          placeholder={placeholder}
          onChange={(e) => set({ search: e.target.value })}
          sx={{
            mt: 1,
            mb: 2,
            bgcolor: "background.paper",
          }}
        />
      )}

      {showTabs && (
        <Tabs
          value={
            (filters.types?.[0] === CASE_TYPES.CHANGE_REQUEST ? filters.states?.[0] : filters.statuses?.[0]) ?? "all"
          }
          scrollButtons={false}
          variant="scrollable"
          onChange={(_, value) =>
            set({
              [filters.types?.[0] === CASE_TYPES.CHANGE_REQUEST ? "states" : "statuses"]:
                value === "all" ? [] : [value],
            })
          }
          sx={{
            "& .MuiTabs-flexContainer": {
              gap: 1.2,
            },

            "& .MuiTab-root": {
              minHeight: 0,
              minWidth: 0,
              padding: "6px 12px",
              borderRadius: 999,
              textTransform: "none",
              fontWeight: "medium",
              color: "text.secondary",
              backgroundColor: "background.paper",
            },

            "& .MuiTab-root.Mui-selected": {
              color: "primary.contrastText",
              backgroundColor: "primary.main",
              fontWeight: "bold",
            },

            "& .MuiTabs-indicator": {
              display: "none",
            },
          }}
        >
          <Tab label="All" value="all" disableRipple />

          {tabs.map((tab, index) => (
            <Tab key={index} label={tab.label} value={tab.value} disableRipple />
          ))}
        </Tabs>
      )}
    </Stack>
  );
}
