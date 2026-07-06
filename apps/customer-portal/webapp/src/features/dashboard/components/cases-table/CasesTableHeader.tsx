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

import { Box, Typography, Button } from "@wso2/oxygen-ui";
import TabBar from "@components/tab-bar/TabBar";
import {
  ListFilter,
  ChevronDown,
  ChevronUp,
  X,
} from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";
import {
  CASES_TABLE_BUTTON_FILTERS,
  CASES_TABLE_HEADER_SUBTITLE,
  CASES_TABLE_HEADER_TITLE,
} from "@/features/dashboard/constants/casesTable";
import type { CasesTableHeaderProps } from "@/features/dashboard/types/casesTable";
import { formatCasesTableClearFiltersLabel } from "@features/dashboard/utils/dashboard";

/**
 * Header row for the dashboard cases table (title, filter toggle).
 *
 * @returns {JSX.Element} Header UI
 */
const CasesTableHeader = ({
  activeFiltersCount,
  isFiltersOpen,
  onFilterToggle,
  viewTabs,
  activeViewMode,
  onViewModeChange,
}: CasesTableHeaderProps): JSX.Element => {
  const hasActiveFilters = activeFiltersCount > 0;

  return (
    <Box>
      <Box
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "flex-start",
          mb: 3,
          flexWrap: "wrap",
          gap: 2,
          width: "100%",
          minWidth: 0,
        }}
      >
        <Box sx={{ minWidth: 0, flex: 1 }}>
          <Typography variant="h6">{CASES_TABLE_HEADER_TITLE}</Typography>
          <Typography variant="body2" color="text.secondary">
            {CASES_TABLE_HEADER_SUBTITLE}
          </Typography>
        </Box>
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            gap: 1.5,
            flexWrap: "wrap",
            flexShrink: 0,
          }}
        >
          <TabBar
            tabs={viewTabs}
            activeTab={activeViewMode}
            onTabChange={onViewModeChange}
            sx={{ mb: 0, height: 32 }}
          />
          <Button
            variant="outlined"
            color="warning"
            size="small"
            onClick={onFilterToggle}
            startIcon={
              hasActiveFilters ? <X size={16} /> : <ListFilter size={16} />
            }
            endIcon={
              !hasActiveFilters &&
              (isFiltersOpen ? (
                <ChevronUp size={16} />
              ) : (
                <ChevronDown size={16} />
              ))
            }
          >
            {hasActiveFilters
              ? formatCasesTableClearFiltersLabel(activeFiltersCount)
              : CASES_TABLE_BUTTON_FILTERS}
          </Button>
        </Box>
      </Box>
    </Box>
  );
};

export default CasesTableHeader;
