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
import { ListFilter, Plus } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";
import ActiveFilters, {
  type ActiveFilterConfig,
} from "@/components/common/filterPanel/ActiveFilters";

interface CasesTableHeaderProps {
  activeFiltersCount: number;
  appliedFilters: Record<string, string>;
  filterFields: ActiveFilterConfig[];
  onRemoveFilter: (field: string) => void;
  onClearAll: () => void;
  onUpdateFilter: (field: string, value: any) => void;
  onFilterClick: () => void;
  onCreateCase: () => void;
  projectId: string;
}

const CasesTableHeader = ({
  activeFiltersCount,
  appliedFilters,
  filterFields,
  onRemoveFilter,
  onClearAll,
  onUpdateFilter,
  onFilterClick,
  onCreateCase,
}: CasesTableHeaderProps): JSX.Element => {
  return (
    <Box
      sx={{
        display: "flex",
        justifyContent: "space-between",
        alignItems: "flex-start",
        mb: 3,
        flexWrap: "wrap",
        gap: 2,
      }}
    >
      {/* Title and description */}
      <Box sx={{ flexGrow: 1 }}>
        <Box
          sx={{
            display: "flex",
            flexDirection: "column",
            width: "100%",
            mb: activeFiltersCount > 0 ? 2 : 0,
          }}
        >
          <Box>
            <Typography variant="h6">Outstanding cases</Typography>
            <Typography variant="body2" color="text.secondary">
              Manage and track all your open support cases
            </Typography>
          </Box>
        </Box>

        {/* Active filters */}
        <ActiveFilters
          appliedFilters={appliedFilters}
          filterFields={filterFields}
          onRemoveFilter={onRemoveFilter}
          onClearAll={onClearAll}
          onUpdateFilter={onUpdateFilter}
        />
      </Box>
      {/* Buttons */}
      <Box sx={{ display: "flex", gap: 1, pt: 0.5 }}>
        <Button
          variant="outlined"
          color="warning"
          size="small"
          startIcon={<ListFilter size={16} />}
          onClick={onFilterClick}
        >
          Filters
        </Button>
        <Button variant="outlined" size="small" color="warning">
          All cases
        </Button>
        <Button
          variant="contained"
          color="warning"
          startIcon={<Plus size={16} />}
          size="small"
          onClick={onCreateCase}
        >
          Create case
        </Button>
      </Box>
    </Box>
  );
};

export default CasesTableHeader;
