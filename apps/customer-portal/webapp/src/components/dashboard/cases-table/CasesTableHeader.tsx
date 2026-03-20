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
import { ListFilter, RotateCcw, ChevronDown, ChevronUp, Plus } from "@wso2/oxygen-ui-icons-react";
import { type JSX, useCallback } from "react";
import { useNavigate, useParams } from "react-router";

interface CasesTableHeaderProps {
  activeFiltersCount: number;
  isFiltersOpen: boolean;
  onFilterToggle: () => void;
  hasAgent?: boolean;
}

const CasesTableHeader = ({
  activeFiltersCount,
  isFiltersOpen,
  onFilterToggle,
  hasAgent = false,
}: CasesTableHeaderProps): JSX.Element => {
  const navigate = useNavigate();
  const { projectId: rawProjectId } = useParams<{ projectId?: string }>();
  const projectId = rawProjectId ?? "";
  const hasActiveFilters = activeFiltersCount > 0;

  const handleCreateCase = useCallback(() => {
    if (hasAgent) {
      navigate(`/projects/${projectId}/support/chat/describe-issue`);
    } else {
      navigate(`/projects/${projectId}/support/chat/create-case`, {
        state: { skipChat: true },
      });
    }
  }, [navigate, projectId, hasAgent]);

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
        }}
      >
        {/* Title and description */}
        <Box>
          <Typography variant="h6">Outstanding Support Cases</Typography>
          <Typography variant="body2" color="text.secondary">
            Track and manage all active support tickets
          </Typography>
        </Box>
        {/* Filter and Create buttons */}
        <Box sx={{ display: "flex", gap: 1 }}>
          <Button
            variant="outlined"
            color="warning"
            size="small"
            onClick={onFilterToggle}
            startIcon={hasActiveFilters ? <RotateCcw size={16} /> : <ListFilter size={16} />}
            endIcon={
              !hasActiveFilters &&
              (isFiltersOpen ? (
                <ChevronUp size={16} />
              ) : (
                <ChevronDown size={16} />
              ))
            }
          >
            {hasActiveFilters ? "Reset Filters" : "Filters"}
          </Button>
          <Button
            variant="contained"
            color="warning"
            startIcon={<Plus size={16} />}
            size="small"
            onClick={handleCreateCase}
          >
            Create
          </Button>
        </Box>
      </Box>
    </Box>
  );
};

export default CasesTableHeader;
