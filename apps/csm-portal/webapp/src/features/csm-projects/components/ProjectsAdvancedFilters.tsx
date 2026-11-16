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

import { Box, Chip } from "@wso2/oxygen-ui";
import { useMemo, type JSX } from "react";
import OperationsAdvancedFilters from "@features/csm-operations/components/OperationsAdvancedFilters";
import { projectAdvancedFields } from "@features/csm-projects/utils/projectAdvancedFields";
import { buildProjectFilterChips, type ProjectFilters } from "@features/csm-projects/utils/projectFilters";

interface ProjectsAdvancedFiltersProps {
  filters: ProjectFilters;
  onChange: (next: ProjectFilters) => void;
}

/** The projects list's Advanced filter panel: the shared Field / Operator /
 *  Value builder the change request and incident lists use. */
export default function ProjectsAdvancedFilters({
  filters,
  onChange,
}: ProjectsAdvancedFiltersProps): JSX.Element {
  const fields = useMemo(() => projectAdvancedFields(), []);
  return (
    <OperationsAdvancedFilters idPrefix="projects" fields={fields} filters={filters} onChange={onChange} />
  );
}

interface ProjectFilterChipsProps {
  filters: ProjectFilters;
  /** Whether the Advanced panel is open (and so already lists its filters). */
  panelOpen: boolean;
  onChange: (next: ProjectFilters) => void;
}

/** Active-filter chips under the filter bar, each removable on its own. */
export function ProjectFilterChips({
  filters,
  panelOpen,
  onChange,
}: ProjectFilterChipsProps): JSX.Element | null {
  const chips = buildProjectFilterChips(filters, !panelOpen);
  if (chips.length === 0) return null;
  return (
    <Box sx={{ display: "flex", flexWrap: "wrap", gap: 1 }}>
      {chips.map((chip) => (
        <Chip key={chip.key} size="small" label={chip.label} onDelete={() => onChange(chip.onRemove(filters))} />
      ))}
    </Box>
  );
}
