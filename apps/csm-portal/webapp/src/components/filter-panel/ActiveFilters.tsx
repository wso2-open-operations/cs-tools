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

import { Box, Chip, Button, Menu, MenuItem } from "@wso2/oxygen-ui";
import { ChevronDown } from "@wso2/oxygen-ui-icons-react";
import { type JSX, type MouseEvent, useState } from "react";

export interface ActiveFilterConfig {
  id: string;
  label: string;
  enableMenu?: boolean;
  options?: (string | { label: string; value: string })[];
}

interface ActiveFiltersProps<T> {
  appliedFilters: T;
  filterFields: ActiveFilterConfig[];
  onRemoveFilter: (field: keyof T) => void;
  onClearAll: () => void;
  onUpdateFilter?: (field: keyof T, value: any) => void;
}

const ActiveFilters = <T extends Record<string, any>>({
  appliedFilters,
  filterFields,
  onRemoveFilter,
  onClearAll,
  onUpdateFilter,
}: ActiveFiltersProps<T>): JSX.Element => {
  const [anchorEl, setAnchorEl] = useState<null | HTMLElement>(null);
  const [menuFieldId, setMenuFieldId] = useState<string | null>(null);

  const activeFiltersCount =
    Object.values(appliedFilters).filter(Boolean).length;

  if (activeFiltersCount === 0) {
    return <></>;
  }

  const handleChipClick = (fieldId: string, event: MouseEvent<HTMLElement>) => {
    setAnchorEl(event.currentTarget);
    setMenuFieldId(fieldId);
  };

  const handleMenuClose = () => {
    setAnchorEl(null);
    setMenuFieldId(null);
  };

  const handleMenuItemClick = (value: string) => {
    if (menuFieldId && onUpdateFilter) {
      onUpdateFilter(menuFieldId as keyof T, value);
    }
    handleMenuClose();
  };

  const activeField = filterFields.find((f) => f.id === menuFieldId);

  return (
    <Box
      sx={{
        display: "flex",
        gap: 1,
        flexWrap: "wrap",
        alignItems: "center",
      }}
    >
      {filterFields.map((field) => {
        const value = appliedFilters[field.id];
        const isActive = Boolean(value);

        const hasOptions = field.options && field.options.length > 0;

        return (
          <Box key={field.id} sx={{ display: "flex", alignItems: "center" }}>
            {/* active filters chip */}
            <Chip
              label={value || field.label}
              onClick={
                hasOptions ? (e) => handleChipClick(field.id, e) : undefined
              }
              onDelete={isActive ? () => onRemoveFilter(field.id) : undefined}
              icon={hasOptions ? <ChevronDown size={14} /> : undefined}
              size="small"
              variant="outlined"
              color={isActive ? "warning" : "default"}
              sx={{
                "& .MuiChip-label": { order: 1, pr: 1 },
                "& .MuiChip-icon": { order: 2, mr: 0.5, ml: 0 },
                "& .MuiChip-deleteIcon": { order: 3, ml: 0.5 },
                backgroundColor: "background.paper",
                cursor: hasOptions ? "pointer" : "default",
              }}
            />
          </Box>
        );
      })}

      {/* active filters menu */}
      <Menu
        anchorEl={anchorEl}
        open={Boolean(anchorEl) && Boolean(menuFieldId)}
        onClose={handleMenuClose}
      >
        {activeField?.options?.map((option) => {
          const optionLabel =
            typeof option === "string" ? option : option.label;
          const optionValue =
            typeof option === "string" ? option : option.value;
          const isSelected =
            appliedFilters[activeField.id] === optionLabel ||
            appliedFilters[activeField.id] === optionValue;

          return (
            <MenuItem
              key={optionValue}
              selected={isSelected}
              onClick={() => handleMenuItemClick(optionValue)}
            >
              {optionLabel}
            </MenuItem>
          );
        })}
      </Menu>

      {/* clear all button */}
      <Button
        size="small"
        color="inherit"
        onClick={onClearAll}
        sx={{
          textTransform: "none",
          color: "text.secondary",
        }}
      >
        Clear filters
      </Button>
    </Box>
  );
};

export default ActiveFilters;
