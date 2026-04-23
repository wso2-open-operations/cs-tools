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

import { useLocation, useNavigate, useSearchParams } from "react-router-dom";
import { SearchBar, Stack, Tab, Tabs } from "@wso2/oxygen-ui";
import type { NotificationFilter } from "@pages/NotificationsPage";
import type { Status } from "./ItemCard";

export interface FilterTab {
  label: string;
  value: Status | NotificationFilter | "all";
}

export interface FilterSlotBuilderProps {
  tabs: FilterTab[];
  searchPlaceholder?: string;
}

export function FilterSlotBuilder({ tabs, searchPlaceholder }: FilterSlotBuilderProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const [searchParams] = useSearchParams();

  const searchParamName = "search",
    filterParamName = "filter";

  const baseRoute = location.pathname;
  const activeFilter = searchParams.get(filterParamName) ?? "all";
  const searchValue = searchParams.get(searchParamName) ?? "";

  const updateParams = (updates: Record<string, string | null>) => {
    const next = new URLSearchParams(searchParams);

    Object.entries(updates).forEach(([key, value]) => {
      if (!value) {
        next.delete(key);
      } else {
        next.set(key, value);
      }
    });

    navigate(`${baseRoute}?${next.toString()}`, { replace: true });
  };

  return (
    <Stack gap={2} pb={1}>
      <SearchBar
        size="small"
        placeholder={searchPlaceholder}
        value={searchValue}
        onChange={(e) => updateParams({ [searchParamName]: e.target.value || null })}
        sx={{
          mt: 1,
          bgcolor: "background.paper",
        }}
        fullWidth
      />
      <Tabs
        value={activeFilter}
        scrollButtons={false}
        variant="scrollable"
        onChange={(_, value) => updateParams({ [filterParamName]: value })}
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
    </Stack>
  );
}
