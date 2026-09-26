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
  Alert,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  IconButton,
  ListItemIcon,
  ListItemText,
  ListSubheader,
  Menu,
  MenuItem,
  TextField,
  Tooltip,
} from "@wso2/oxygen-ui";
import {
  Bookmark,
  BookmarkPlus,
  Check,
  ChevronDown,
  Copy,
  Menu as DragMenu,
  Trash2,
} from "@wso2/oxygen-ui-icons-react";
import { useEffect, useState, type DragEvent, type JSX, type KeyboardEvent } from "react";
import { useSavedFilterViews, type SavedFilterListKey } from "@features/saved-filter-views/useSavedFilterViews";
import { qsFromPastedFilter, shareUrl } from "@features/saved-filter-views/shareLink";

interface SavedViewsMenuProps {
  /** This tab's own serialized-filters query string right now (no leading
   * `?`) — what "Save current view…" captures, and what an already-saved
   * view is compared against for the active-view highlight. */
  currentQs: string;
  /**
   * Round-trip a saved view's `qs` through this tab's own filter codec
   * (read then write) so the "is this view currently active" check matches
   * regardless of param order/encoding differences between how a view's
   * `qs` was authored and what `writeXFiltersToUrl` itself emits — same
   * reasoning as `CasesFilterBar`'s own `canonicalQs`.
   */
  canonicalizeQs: (qs: string) => string;
  /** Active (non-search) filter count, shown in the save dialog's helper text. */
  activeCount: number;
  /** Whether a search term is currently active. `activeCount` deliberately
   * excludes search (see each tab's `countActive*Filters`), but a saved
   * view's `qs` still captures it — the save dialog's "will show all
   * records" message must only appear when neither is set, or applying that
   * view would silently restore a search the message said wasn't there. */
  hasSearch: boolean;
  /** Apply a saved view's `qs` — the caller parses it back into its own
   * filter shape and feeds it through the same `onChange` the filter bar
   * already has. */
  onApply: (qs: string) => void;
  /** Which CSM list this menu persists views for. */
  listKey: SavedFilterListKey;
}

/**
 * "Saved views" button + menu shared by the Cases, Change Requests,
 * Incidents, and Problems filter bars — a named, reusable filter set for
 * high-volume triage. Each caller supplies its own `listKey` so views never
 * leak across lists.
 */
export default function SavedViewsMenu({
  currentQs,
  canonicalizeQs,
  activeCount,
  hasSearch,
  onApply,
  listKey,
}: SavedViewsMenuProps): JSX.Element {
  const {
    views: savedViews,
    saveFilterView,
    deleteFilterView,
    moveFilterView,
    reorderFilterView,
    isSaving,
    saveError,
    resetSaveError,
  } = useSavedFilterViews(listKey);
  const currentCanonical = canonicalizeQs(currentQs);
  const isActiveView = (qs: string): boolean => canonicalizeQs(qs) === currentCanonical;
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);
  const [saveDialogOpen, setSaveDialogOpen] = useState(false);
  const [newViewName, setNewViewName] = useState("");
  const [pastedLink, setPastedLink] = useState("");
  const [pasteError, setPasteError] = useState<string | null>(null);
  const [copiedName, setCopiedName] = useState<string | null>(null);
  const [copyError, setCopyError] = useState<string | null>(null);
  const [draggedName, setDraggedName] = useState<string | null>(null);
  const [dragOverName, setDragOverName] = useState<string | null>(null);

  useEffect(() => {
    if (!copiedName) return;
    const timer = setTimeout(() => setCopiedName(null), 1500);
    return () => clearTimeout(timer);
  }, [copiedName]);

  const applyView = (qs: string): void => {
    setAnchor(null);
    onApply(qs);
  };

  const handleSaveView = (): void => {
    if (!newViewName.trim()) return;
    const pasted = pastedLink.trim();
    let qs = currentQs;
    if (pasted) {
      const parsed = qsFromPastedFilter(pasted, listKey);
      if (!parsed.ok) {
        setPasteError(parsed.error);
        return;
      }
      qs = parsed.qs;
    }
    void (async () => {
      try {
        await saveFilterView(newViewName, qs);
        setNewViewName("");
        setPastedLink("");
        setPasteError(null);
        setSaveDialogOpen(false);
        setAnchor(null);
      } catch {
        // Keep the dialog and name so the caller can retry after saveError.
      }
    })();
  };

  const copyLink = (name: string, qs: string): void => {
    const url = shareUrl(listKey, qs);
    if (!navigator.clipboard?.writeText) {
      setCopiedName(null);
      setCopyError(name);
      return;
    }
    navigator.clipboard.writeText(url).then(
      () => {
        setCopyError(null);
        setCopiedName(name);
      },
      () => {
        setCopiedName(null);
        setCopyError(name);
      },
    );
  };

  const handleDragStart = (e: DragEvent<HTMLElement>, name: string): void => {
    setDraggedName(name);
    if (!e.dataTransfer) return;
    e.dataTransfer.effectAllowed = "move";
    e.dataTransfer.setData("text/plain", name);
  };

  const handleDragOver = (e: DragEvent<HTMLElement>, name: string): void => {
    e.preventDefault();
    if (name !== dragOverName) setDragOverName(name);
  };

  const handleDrop = (e: DragEvent<HTMLElement>, targetName: string): void => {
    e.preventDefault();
    if (draggedName && draggedName !== targetName) {
      const targetIndex = savedViews.findIndex((v) => v.name === targetName);
      if (targetIndex !== -1) void reorderFilterView(draggedName, targetIndex);
    }
    setDraggedName(null);
    setDragOverName(null);
  };

  const handleDragEnd = (): void => {
    setDraggedName(null);
    setDragOverName(null);
  };

  const handleHandleKeyDown = (e: KeyboardEvent<HTMLElement>, name: string, index: number): void => {
    if (e.key === "ArrowUp") {
      e.preventDefault();
      e.stopPropagation();
      if (index > 0) void moveFilterView(name, "up");
    } else if (e.key === "ArrowDown") {
      e.preventDefault();
      e.stopPropagation();
      if (index < savedViews.length - 1) void moveFilterView(name, "down");
    }
  };

  return (
    <>
      <Button
        variant="outlined"
        size="small"
        color="inherit"
        onClick={(e) => setAnchor(e.currentTarget)}
        startIcon={<Bookmark size={16} />}
        endIcon={<ChevronDown size={16} />}
        aria-haspopup="true"
        aria-expanded={Boolean(anchor)}
      >
        Saved views
      </Button>
      <Menu
        anchorEl={anchor}
        open={Boolean(anchor)}
        onClose={() => setAnchor(null)}
        anchorOrigin={{ vertical: "bottom", horizontal: "left" }}
      >
        <MenuItem
          onClick={() => {
            setAnchor(null);
            resetSaveError();
            setPasteError(null);
            setSaveDialogOpen(true);
          }}
        >
          <ListItemIcon>
            <BookmarkPlus size={16} />
          </ListItemIcon>
          <ListItemText primary="Save current view…" />
        </MenuItem>
        <Divider />
        <ListSubheader sx={{ lineHeight: "32px" }}>Saved</ListSubheader>
        {savedViews.length === 0 ? (
          <MenuItem disabled>
            <ListItemText
              primary="No saved views yet"
              slotProps={{ primary: { variant: "body2" } }}
            />
          </MenuItem>
        ) : (
          savedViews.map((v, i) => (
            <MenuItem
              key={`saved-${v.name}`}
              selected={isActiveView(v.qs)}
              onClick={() => applyView(v.qs)}
              onDragOver={(e) => handleDragOver(e, v.name)}
              onDrop={(e) => handleDrop(e, v.name)}
              sx={{
                outline:
                  dragOverName === v.name && draggedName !== v.name
                    ? "2px solid"
                    : "2px solid transparent",
                outlineColor:
                  dragOverName === v.name && draggedName !== v.name
                    ? "primary.main"
                    : "transparent",
                outlineOffset: -2,
              }}
            >
              <ListItemIcon>{isActiveView(v.qs) ? <Check size={16} /> : null}</ListItemIcon>
              <ListItemText primary={v.name} />
              <Tooltip
                title={
                  copyError === v.name
                    ? "Couldn't copy link"
                    : copiedName === v.name
                      ? "Copied!"
                      : "Copy filter link"
                }
              >
                <IconButton
                  size="small"
                  edge="end"
                  aria-label={
                    copyError === v.name
                      ? `Couldn't copy filter link for ${v.name}`
                      : copiedName === v.name
                        ? `Copied filter link for ${v.name}`
                        : `Copy filter link for ${v.name}`
                  }
                  onClick={(e) => {
                    e.stopPropagation();
                    copyLink(v.name, v.qs);
                  }}
                  sx={{ ml: 1 }}
                >
                  {copiedName === v.name ? <Check size={15} /> : <Copy size={15} />}
                </IconButton>
              </Tooltip>
              <Tooltip title="Drag to reorder">
                <IconButton
                  size="small"
                  edge="end"
                  draggable
                  aria-label={`Drag to reorder saved view ${v.name}`}
                  onDragStart={(e) => handleDragStart(e, v.name)}
                  onDragEnd={handleDragEnd}
                  onKeyDown={(e) => handleHandleKeyDown(e, v.name, i)}
                  onClick={(e) => e.stopPropagation()}
                  sx={{
                    cursor: "grab",
                    opacity: draggedName === v.name ? 0.4 : 1,
                    "&:active": { cursor: "grabbing" },
                  }}
                >
                  <DragMenu size={15} />
                </IconButton>
              </Tooltip>
              <IconButton
                size="small"
                edge="end"
                aria-label={`Delete saved view ${v.name}`}
                onClick={(e) => {
                  e.stopPropagation();
                  deleteFilterView(v.name);
                }}
              >
                <Trash2 size={15} />
              </IconButton>
            </MenuItem>
          ))
        )}
      </Menu>

      <Dialog
        open={saveDialogOpen}
        onClose={() => setSaveDialogOpen(false)}
        maxWidth="xs"
        fullWidth
      >
        <DialogTitle>Save current view</DialogTitle>
        <DialogContent>
          {saveError ? (
            <Alert severity="error" sx={{ mb: 1 }}>
              Couldn&apos;t save this view. Try again.
            </Alert>
          ) : null}
          <TextField
            autoFocus
            fullWidth
            size="small"
            margin="dense"
            label="View name"
            placeholder="e.g. My open records"
            value={newViewName}
            onChange={(e) => {
              resetSaveError();
              setNewViewName(e.target.value);
            }}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                handleSaveView();
              }
            }}
            helperText={
              pastedLink.trim()
                ? "Saves the filter from the pasted link, not the filters on screen."
                : activeCount === 0 && !hasSearch
                  ? "Tip: no filters are active — this view will show all records."
                  : `Captures the ${activeCount} active filter${activeCount === 1 ? "" : "s"}${
                      hasSearch ? " and the current search" : ""
                    }.`
            }
          />
          <TextField
            fullWidth
            size="small"
            margin="dense"
            label="Filter link"
            placeholder="Paste a filter link"
            value={pastedLink}
            error={Boolean(pasteError)}
            helperText={
              pasteError ??
              "Optional. Paste a copied filter link to save that filter instead of the one on screen."
            }
            onChange={(e) => {
              setPasteError(null);
              setPastedLink(e.target.value);
            }}
          />
        </DialogContent>
        <DialogActions>
          <Button color="inherit" onClick={() => setSaveDialogOpen(false)}>
            Cancel
          </Button>
          <Button variant="contained" onClick={handleSaveView} disabled={!newViewName.trim() || isSaving}>
            Save
          </Button>
        </DialogActions>
      </Dialog>
    </>
  );
}
