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
  Checkbox,
  Divider,
  IconButton,
  List,
  ListItem,
  ListItemButton,
  ListItemText,
  Popover,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { ColumnsSettings, GripVertical } from "@wso2/oxygen-ui-icons-react";
import { useState, type DragEvent, type JSX, type KeyboardEvent, type MouseEvent } from "react";
import type { ColumnOption } from "@hooks/useColumnPreferences";

export interface ColumnCustomizerButtonProps {
  /** All known columns for this table, in the user's current order —
   * `useColumnPreferences`'s `allColumns`. */
  allColumns: ColumnOption[];
  isVisible: (id: string) => boolean;
  onToggle: (id: string) => void;
  /** Keyboard reordering: the drag handle's own arrow-up/down fallback for
   * anyone not using a mouse/touch drag. */
  onMove: (id: string, direction: "up" | "down") => void;
  /** Drag-and-drop reordering: called once, on drop, with the dragged
   * column's id and the index of the row it was dropped onto — a single
   * gesture can move a column several slots, which is exactly what
   * `useColumnPreferences`'s `reorderColumn` (not `onMove`) is for. */
  onReorder: (id: string, targetIndex: number) => void;
  onReset: () => void;
  /** Accessible label for the trigger button; defaults to "Customise
   * columns". Override when a page has more than one table on screen. */
  label?: string;
}

/**
 * "Customise columns" trigger + popover: check/uncheck to add or remove a
 * column, drag the grip handle to reorder it (or, with the handle focused,
 * arrow up/down — native HTML5 drag-and-drop has no built-in keyboard path,
 * so this is the only way a keyboard-only user can reorder at all). Shared
 * across every table that adopts `useColumnPreferences` — the table itself
 * owns what each column id renders as; this component only edits the
 * visibility/order state.
 */
export default function ColumnCustomizerButton({
  allColumns,
  isVisible,
  onToggle,
  onMove,
  onReorder,
  onReset,
  label = "Customise columns",
}: ColumnCustomizerButtonProps): JSX.Element {
  const [anchorEl, setAnchorEl] = useState<HTMLElement | null>(null);
  const [draggedId, setDraggedId] = useState<string | null>(null);
  const [dragOverId, setDragOverId] = useState<string | null>(null);
  const open = Boolean(anchorEl);
  const visibleCount = allColumns.filter((c) => isVisible(c.id)).length;

  const handleOpen = (e: MouseEvent<HTMLElement>): void => setAnchorEl(e.currentTarget);
  const handleClose = (): void => setAnchorEl(null);

  const handleDragStart = (e: DragEvent<HTMLElement>, id: string): void => {
    setDraggedId(id);
    e.dataTransfer.effectAllowed = "move";
    // Firefox refuses to start a drag at all unless data is set.
    e.dataTransfer.setData("text/plain", id);
  };

  const handleDragOver = (e: DragEvent<HTMLElement>, id: string): void => {
    // Required for this element to become a valid drop target at all.
    e.preventDefault();
    if (id !== dragOverId) setDragOverId(id);
  };

  const handleDrop = (e: DragEvent<HTMLElement>, targetId: string): void => {
    e.preventDefault();
    if (draggedId && draggedId !== targetId) {
      const targetIndex = allColumns.findIndex((c) => c.id === targetId);
      if (targetIndex !== -1) onReorder(draggedId, targetIndex);
    }
    setDraggedId(null);
    setDragOverId(null);
  };

  const handleDragEnd = (): void => {
    setDraggedId(null);
    setDragOverId(null);
  };

  const handleHandleKeyDown = (e: KeyboardEvent<HTMLElement>, id: string): void => {
    if (e.key === "ArrowUp") {
      e.preventDefault();
      onMove(id, "up");
    } else if (e.key === "ArrowDown") {
      e.preventDefault();
      onMove(id, "down");
    }
  };

  return (
    <>
      <Tooltip title={label}>
        <IconButton size="small" aria-label={label} onClick={handleOpen}>
          <ColumnsSettings size={16} />
        </IconButton>
      </Tooltip>
      <Popover
        open={open}
        anchorEl={anchorEl}
        onClose={handleClose}
        anchorOrigin={{ vertical: "bottom", horizontal: "right" }}
        transformOrigin={{ vertical: "top", horizontal: "right" }}
      >
        <Box sx={{ width: 280, py: 1 }}>
          <Typography
            variant="caption"
            color="text.secondary"
            sx={{ px: 2, py: 0.5, display: "block", fontWeight: 600 }}
          >
            Columns
          </Typography>
          <List
            dense
            disablePadding
            aria-label={label}
            sx={{ maxHeight: 320, overflowY: "auto" }}
          >
            {allColumns.map((column) => {
              const checked = isVisible(column.id);
              const disableUncheck = checked && visibleCount <= 1;
              return (
                <ListItem
                  key={column.id}
                  disableGutters
                  onDragOver={(e) => handleDragOver(e, column.id)}
                  onDrop={(e) => handleDrop(e, column.id)}
                  sx={{
                    px: 2,
                    py: 0.25,
                    opacity: draggedId === column.id ? 0.4 : 1,
                    outline:
                      dragOverId === column.id && draggedId !== column.id
                        ? "2px solid"
                        : "2px solid transparent",
                    outlineColor:
                      dragOverId === column.id && draggedId !== column.id
                        ? "primary.main"
                        : "transparent",
                    outlineOffset: -2,
                    borderRadius: 1,
                  }}
                >
                  <Checkbox
                    size="small"
                    checked={checked}
                    disabled={disableUncheck}
                    onChange={() => !disableUncheck && onToggle(column.id)}
                    sx={{ mr: 1, p: 0.25 }}
                  />
                  <ListItemButton
                    disableRipple
                    disabled={disableUncheck}
                    onClick={() => !disableUncheck && onToggle(column.id)}
                    sx={{ py: 0, px: 0, borderRadius: 1 }}
                  >
                    <ListItemText
                      primary={column.label}
                      slotProps={{ primary: { style: { fontSize: 13 } } }}
                    />
                  </ListItemButton>
                  <Box
                    role="button"
                    tabIndex={0}
                    draggable
                    onDragStart={(e) => handleDragStart(e, column.id)}
                    onDragEnd={handleDragEnd}
                    onKeyDown={(e) => handleHandleKeyDown(e, column.id)}
                    aria-label={`Reorder ${column.label}. Drag, or focus and press the up/down arrow keys, to move.`}
                    sx={{
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "center",
                      width: 24,
                      height: 24,
                      ml: 1,
                      borderRadius: 1,
                      color: "text.secondary",
                      cursor: "grab",
                      "&:active": { cursor: "grabbing" },
                      "&:hover": { bgcolor: "action.hover" },
                      "&:focus-visible": {
                        outline: "2px solid",
                        outlineColor: "primary.main",
                        outlineOffset: 1,
                      },
                    }}
                  >
                    <GripVertical size={16} />
                  </Box>
                </ListItem>
              );
            })}
          </List>
          <Divider sx={{ my: 0.5 }} />
          <Box sx={{ px: 1.5 }}>
            <Button size="small" variant="text" onClick={onReset} sx={{ textTransform: "none" }}>
              Reset to default
            </Button>
          </Box>
        </Box>
      </Popover>
    </>
  );
}
