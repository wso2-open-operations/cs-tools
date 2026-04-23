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
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Typography,
} from "@wso2/oxygen-ui";
import type { JSX, ReactNode } from "react";
import { SortOrder } from "@/types/common";

export interface SortFieldOption {
  value: string;
  label: string;
  kind?: "chronological" | "ordinal";
}

export interface ListResultsBarProps {
  shownCount: number;
  totalCount: number;
  entityLabel: string;
  /** Sort field dropdown options — omit to hide the sort dropdowns */
  sortFieldOptions?: SortFieldOption[];
  sortField?: string;
  onSortFieldChange?: (value: string) => void;
  sortOrder?: SortOrder;
  onSortOrderChange?: (value: SortOrder) => void;
  /** Optional right-side slot — e.g. TabBar in ChangeRequestsPage */
  rightContent?: ReactNode;
}

/**
 * ListResultsBar renders the "Showing X of Y items" count on the left and
 * sort controls (or custom right content) on the right.
 *
 * @param {ListResultsBarProps} props - Results count and sort configuration.
 * @returns {JSX.Element} The rendered results bar.
 */
export default function ListResultsBar({
  shownCount,
  totalCount,
  entityLabel,
  sortFieldOptions,
  sortField,
  onSortFieldChange,
  sortOrder,
  onSortOrderChange,
  rightContent,
}: ListResultsBarProps): JSX.Element {
  const showSortDropdowns =
    sortFieldOptions &&
    sortFieldOptions.length > 0 &&
    sortField !== undefined &&
    onSortFieldChange !== undefined &&
    sortOrder !== undefined &&
    onSortOrderChange !== undefined;

  const isOrdinalSort =
    sortFieldOptions?.find((o) => o.value === sortField)?.kind === "ordinal";

  return (
    <Box
      sx={{
        display: "flex",
        justifyContent: "space-between",
        alignItems: "center",
        flexWrap: "wrap",
        gap: 2,
      }}
    >
      <Typography variant="body2" color="text.secondary">
        Showing {shownCount} of {totalCount} {entityLabel}
      </Typography>

      {(showSortDropdowns || rightContent) && (
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          {showSortDropdowns && (
            <>
              <FormControl size="small" sx={{ minWidth: 180 }}>
                <InputLabel id="list-sort-field-label">Sort by</InputLabel>
                <Select
                  labelId="list-sort-field-label"
                  id="list-sort-field"
                  value={sortField}
                  label="Sort by"
                  onChange={(e) => onSortFieldChange!(e.target.value as string)}
                >
                  {sortFieldOptions!.map((opt) => (
                    <MenuItem key={opt.value} value={opt.value}>
                      <Typography variant="body2">{opt.label}</Typography>
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
              <FormControl size="small" sx={{ minWidth: 180 }}>
                <InputLabel id="list-sort-order-label">Order by</InputLabel>
                <Select
                  labelId="list-sort-order-label"
                  id="list-sort-order"
                  value={sortOrder}
                  label="Order by"
                  onChange={(e) =>
                    onSortOrderChange!(e.target.value as SortOrder)
                  }
                >
                  <MenuItem value={SortOrder.DESC}>
                    <Typography variant="body2">{isOrdinalSort ? "Descending" : "Newest first"}</Typography>
                  </MenuItem>
                  <MenuItem value={SortOrder.ASC}>
                    <Typography variant="body2">{isOrdinalSort ? "Ascending" : "Oldest first"}</Typography>
                  </MenuItem>
                </Select>
              </FormControl>
            </>
          )}
          {rightContent}
        </Box>
      )}
    </Box>
  );
}
