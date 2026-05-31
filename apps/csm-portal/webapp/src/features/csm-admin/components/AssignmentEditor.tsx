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
  Chip,
  Menu,
  MenuItem,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { Plus, X } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useRef, useState, type JSX } from "react";

export interface AssignmentOption {
  id: string;
  label: string;
  /** Optional secondary label shown in the picker dropdown. */
  description?: string;
}

interface AssignmentEditorProps {
  /** Currently-selected ids. */
  value: string[];
  /** All possible options (selected + unselected). */
  options: AssignmentOption[];
  onChange: (next: string[]) => void;
  /** Empty-state copy when no chips are selected. */
  emptyLabel?: string;
  /** Plural noun used in the add picker (e.g. "roles", "groups", "permissions"). */
  itemNoun?: string;
  disabled?: boolean;
}

/**
 * Inline editor for many-to-many assignments. Shows current selections as
 * removable chips; an "Add" button opens a menu of unselected options that
 * can be filtered by typing.
 */
export default function AssignmentEditor({
  value,
  options,
  onChange,
  emptyLabel = "Nothing assigned.",
  itemNoun = "items",
  disabled = false,
}: AssignmentEditorProps): JSX.Element {
  const [anchorEl, setAnchorEl] = useState<HTMLElement | null>(null);
  const [filter, setFilter] = useState("");
  const buttonRef = useRef<HTMLButtonElement | null>(null);

  const selectedSet = useMemo(() => new Set(value), [value]);

  const selectedOptions = useMemo(
    () => options.filter((o) => selectedSet.has(o.id)),
    [options, selectedSet],
  );

  const availableOptions = useMemo(() => {
    const q = filter.trim().toLowerCase();
    return options
      .filter((o) => !selectedSet.has(o.id))
      .filter((o) => {
        if (!q) return true;
        return (
          o.label.toLowerCase().includes(q) ||
          (o.description ?? "").toLowerCase().includes(q)
        );
      });
  }, [options, selectedSet, filter]);

  const handleRemove = (id: string) => {
    if (disabled) return;
    onChange(value.filter((v) => v !== id));
  };
  const handleAdd = (id: string) => {
    if (disabled) return;
    onChange([...value, id]);
    setFilter("");
    // Keep menu open so the user can add several in a row.
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          gap: 0.75,
          flexWrap: "wrap",
          minHeight: 32,
        }}
      >
        {selectedOptions.length === 0 && (
          <Typography variant="body2" color="text.secondary">
            {emptyLabel}
          </Typography>
        )}
        {selectedOptions.map((o) => (
          <Chip
            key={o.id}
            size="small"
            label={o.label}
            onDelete={disabled ? undefined : () => handleRemove(o.id)}
            deleteIcon={<X size={12} />}
            sx={{ "& .MuiChip-deleteIcon": { fontSize: "1rem" } }}
          />
        ))}
      </Box>
      <Box>
        <Button
          ref={buttonRef}
          size="small"
          variant="outlined"
          startIcon={<Plus size={14} />}
          disabled={disabled || availableOptions.length === 0}
          onClick={(e) => {
            setAnchorEl(e.currentTarget);
            setFilter("");
          }}
        >
          Add {itemNoun}
        </Button>
        <Menu
          anchorEl={anchorEl}
          open={!!anchorEl}
          onClose={() => setAnchorEl(null)}
          slotProps={{ paper: { sx: { width: 320, maxHeight: 360 } } }}
        >
          <Box sx={{ px: 1, py: 0.5 }}>
            <TextField
              size="small"
              fullWidth
              autoFocus
              placeholder={`Filter ${itemNoun}…`}
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              onKeyDown={(e) => {
                // Prevent the Menu from intercepting type-to-jump shortcuts.
                e.stopPropagation();
              }}
            />
          </Box>
          {availableOptions.length === 0 && (
            <Box sx={{ px: 2, py: 1.5 }}>
              <Typography variant="caption" color="text.secondary">
                {filter ? `No ${itemNoun} match "${filter}".` : `No ${itemNoun} left to add.`}
              </Typography>
            </Box>
          )}
          {availableOptions.map((o) => (
            <MenuItem key={o.id} onClick={() => handleAdd(o.id)} sx={{ alignItems: "flex-start" }}>
              <Box>
                <Typography variant="body2">{o.label}</Typography>
                {o.description && (
                  <Typography variant="caption" color="text.secondary">
                    {o.description}
                  </Typography>
                )}
              </Box>
            </MenuItem>
          ))}
        </Menu>
      </Box>
    </Box>
  );
}
