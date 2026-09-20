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
} from "@wso2/oxygen-ui";
import {
  Bookmark,
  BookmarkPlus,
  Check,
  ChevronDown,
  ChevronUp,
  Trash2,
} from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import type { SavedFilterViewsStore } from "@features/csm-operations/utils/savedFilterViews";

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
  /** The tab-scoped saved-views store (its own `localStorage` key) — see
   * `changeRequestsSavedViews.ts` / `incidentsSavedViews.ts` /
   * `problemsSavedViews.ts`. */
  store: SavedFilterViewsStore;
}

/**
 * "Saved views" button + menu shared by the Change Requests, Incidents, and
 * Problems filter bars — a named, reusable filter set for high-volume
 * triage, same UI shape as the Cases list's own saved-views block
 * (`CasesFilterBar.tsx`, search for "Saved views") for consistency. Each
 * caller supplies its own tab-scoped `store` so views never leak across
 * tabs (or into/out of the separate Cases list feature, which keeps its own
 * inline implementation untouched).
 */
export default function SavedViewsMenu({
  currentQs,
  canonicalizeQs,
  activeCount,
  hasSearch,
  onApply,
  store,
}: SavedViewsMenuProps): JSX.Element {
  const savedViews = store.useSavedFilterViews();
  const currentCanonical = canonicalizeQs(currentQs);
  const isActiveView = (qs: string): boolean => canonicalizeQs(qs) === currentCanonical;
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);
  const [saveDialogOpen, setSaveDialogOpen] = useState(false);
  const [newViewName, setNewViewName] = useState("");

  const applyView = (qs: string): void => {
    setAnchor(null);
    onApply(qs);
  };

  const handleSaveView = (): void => {
    if (!newViewName.trim()) return;
    store.saveFilterView(newViewName, currentQs);
    setNewViewName("");
    setSaveDialogOpen(false);
    setAnchor(null);
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
            >
              <ListItemIcon>{isActiveView(v.qs) ? <Check size={16} /> : null}</ListItemIcon>
              <ListItemText primary={v.name} />
              <IconButton
                size="small"
                edge="end"
                aria-label={`Move saved view ${v.name} up`}
                disabled={i === 0}
                onClick={(e) => {
                  e.stopPropagation();
                  store.moveFilterView(v.name, "up");
                }}
                sx={{ ml: 1 }}
              >
                <ChevronUp size={15} />
              </IconButton>
              <IconButton
                size="small"
                edge="end"
                aria-label={`Move saved view ${v.name} down`}
                disabled={i === savedViews.length - 1}
                onClick={(e) => {
                  e.stopPropagation();
                  store.moveFilterView(v.name, "down");
                }}
              >
                <ChevronDown size={15} />
              </IconButton>
              <IconButton
                size="small"
                edge="end"
                aria-label={`Delete saved view ${v.name}`}
                onClick={(e) => {
                  e.stopPropagation();
                  store.deleteFilterView(v.name);
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
          <TextField
            autoFocus
            fullWidth
            size="small"
            margin="dense"
            label="View name"
            placeholder="e.g. My open records"
            value={newViewName}
            onChange={(e) => setNewViewName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                handleSaveView();
              }
            }}
            helperText={
              activeCount === 0 && !hasSearch
                ? "Tip: no filters are active — this view will show all records."
                : `Captures the ${activeCount} active filter${activeCount === 1 ? "" : "s"}${
                    hasSearch ? " and the current search" : ""
                  }.`
            }
          />
        </DialogContent>
        <DialogActions>
          <Button color="inherit" onClick={() => setSaveDialogOpen(false)}>
            Cancel
          </Button>
          <Button variant="contained" onClick={handleSaveView} disabled={!newViewName.trim()}>
            Save
          </Button>
        </DialogActions>
      </Dialog>
    </>
  );
}
