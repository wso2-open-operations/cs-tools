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
  AlertTitle,
  Box,
  Button,
  Checkbox,
  Chip,
  Collapse,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  InputAdornment,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  Skeleton,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { Check, ChevronDown, ChevronRight, Pencil, Search } from "@wso2/oxygen-ui-icons-react";
import { useQueryClient } from "@tanstack/react-query";
import { type JSX, useMemo, useState } from "react";
import { configOverridesAsStored, type StoredFeatureState } from "@config/featureFlags";
import {
  executeFeatureSave,
  changedFeatureIds,
  planFeatureSave,
} from "@features/csm-admin/feature-visibility/featureSave";
import {
  STATE_HELP,
  STATE_LABEL,
  allFeatureNodes,
  effectiveFeatureStates,
  filterFeatureTree,
  type FeatureFilter,
  type FeatureMap,
} from "@features/csm-admin/feature-visibility/featureTree";
import {
  FEATURE_OVERRIDES_KEY,
  useFeatureOverridesQuery,
  usePutFeatureOverride,
  type FeatureOverridesData,
} from "@hooks/useFeatureOverrides";

const CHOICES: StoredFeatureState[] = ["enabled", "preview"];

/** What a bulk choice does. The ticked rows can be sections and pages together, so the words fit either. */
const BULK_HELP: Record<StoredFeatureState, string> = {
  enabled: "Everyone can open the selected items.",
  preview: "Admins can open them, with a Preview tag. Everyone else sees them greyed out.",
};

/** One saved change, as the popup after a save lists it. */
interface SavedChange {
  id: string;
  label: string;
  from: StoredFeatureState;
  to: StoredFeatureState;
}

/** The saved rows as a map the page can compare against, keeping only pages that exist and states it knows. */
function rowsToMap(rows: Record<string, string>, knownIds: ReadonlySet<string>): FeatureMap {
  const out: FeatureMap = {};
  for (const [id, state] of Object.entries(rows)) {
    if (knownIds.has(id) && (state === "enabled" || state === "preview")) out[id] = state;
  }
  return out;
}

/**
 * Settings > Feature visibility. An admin sets each section and page to
 * Available or Preview. A Preview page opens for admins with a Preview tag and
 * is greyed out for everyone else.
 *
 * One list: every row has a checkbox (several can be changed at once), and a
 * section's pages sit in an accordion under it. Each row has a pencil to pick
 * its state, and a changed row also gets Undo. Nothing is saved until Save
 * changes, which sends one write per changed page. Choosing Available keeps the
 * page's row as `enabled`: a page that was saved once keeps its own tag, and
 * there is no way to delete one.
 */
export default function CsmFeatureVisibilityPage(): JSX.Element {
  const query = useFeatureOverridesQuery(true);
  const queryClient = useQueryClient();
  const put = usePutFeatureOverride();

  const nodes = useMemo(() => allFeatureNodes(), []);
  const knownIds = useMemo(() => new Set(nodes.map((node) => node.id)), [nodes]);
  const data: FeatureOverridesData | undefined = query.data;

  const [edits, setEdits] = useState<FeatureMap>({});
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const [search, setSearch] = useState("");
  const [filter, setFilter] = useState<FeatureFilter>("all");
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [stateMenu, setStateMenu] = useState<{ id: string; anchor: HTMLElement } | null>(null);
  const [bulkAnchor, setBulkAnchor] = useState<HTMLElement | null>(null);
  const [savedChanges, setSavedChanges] = useState<SavedChange[] | null>(null);

  const rows = useMemo(() => rowsToMap(data?.rows ?? {}, knownIds), [data, knownIds]);
  // What the page shows as saved: the saved rows, or before the first save the
  // config file's lines with any rows laid over them.
  const saved: FeatureMap = useMemo(
    () => (data?.savedOnce ? rows : { ...configOverridesAsStored(), ...rows }),
    [data?.savedOnce, rows],
  );
  const draft: FeatureMap = useMemo(() => ({ ...saved, ...edits }), [saved, edits]);

  const changedIds = useMemo(() => changedFeatureIds(saved, edits), [saved, edits]);
  const changedSet = useMemo(() => new Set(changedIds), [changedIds]);
  const dirty = changedIds.length > 0;
  const states = useMemo(() => effectiveFeatureStates(nodes, draft), [nodes, draft]);
  const nodeById = useMemo(() => new Map(nodes.map((node) => [node.id, node])), [nodes]);

  const needle = search.trim();
  const filtering = needle !== "" || filter !== "all";
  const sections = useMemo(
    () => filterFeatureTree(nodes, draft, changedSet, search, filter),
    [nodes, draft, changedSet, search, filter],
  );

  // Every row on screen: each section, and the pages of the sections that are open.
  const displayedIds = sections.flatMap(({ section, rows: below }) => {
    const open = section.childIds.length > 0 && (filtering || expanded.has(section.id));
    return open ? [section.id, ...below.map((page) => page.id)] : [section.id];
  });
  const allSelected = displayedIds.length > 0 && displayedIds.every((id) => selected.has(id));
  const someSelected = displayedIds.some((id) => selected.has(id));

  const setMany = (ids: readonly string[], state: StoredFeatureState): void =>
    setEdits((current) => {
      const next = { ...current };
      for (const id of ids) next[id] = state;
      return next;
    });

  const toggle = (set: Set<string>, id: string): Set<string> => {
    const next = new Set(set);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    return next;
  };

  const discard = (): void => {
    setEdits({});
    setSelected(new Set());
    setSaveError(null);
  };

  const save = async (): Promise<void> => {
    if (!data) return;
    const writes = planFeatureSave({ saved, edits, rows: data.rows, savedOnce: data.savedOnce });
    // What each changed page looked like before, and will look like after: the
    // popup after a save lists these.
    const before = effectiveFeatureStates(nodes, saved);
    const pathLabel = (id: string): string => {
      const node = nodeById.get(id);
      const parent = node?.parentId ? nodeById.get(node.parentId) : undefined;
      return parent && node ? `${parent.label} › ${node.label}` : (node?.label ?? id);
    };
    const changes: SavedChange[] = changedIds.map((id) => ({
      id,
      label: pathLabel(id),
      from: before.get(id) ?? "enabled",
      to: states.get(id) ?? "enabled",
    }));
    setSaving(true);
    setSaveError(null);
    const result = await executeFeatureSave(writes, (write) => put.mutateAsync(write));
    // Wait for the refreshed rows before dropping the edits, so a saved page
    // never flashes back to its old state in between.
    await queryClient.invalidateQueries({ queryKey: FEATURE_OVERRIDES_KEY });
    const failed = new Set(result.failed);
    // Drop a write only when it landed and nobody has changed that page since.
    // An edit made while the save was running, or a write that failed, stays.
    setEdits((current) => {
      const next: FeatureMap = { ...current };
      for (const write of writes) {
        if (!failed.has(write.navNodeId) && next[write.navNodeId] === write.state) {
          delete next[write.navNodeId];
        }
      }
      return next;
    });
    setSelected(new Set());
    setSaving(false);
    const done = changes.filter((change) => !failed.has(change.id));
    if (done.length > 0) setSavedChanges(done);
    if (result.failed.length > 0) {
      setSaveError(
        `${result.failed.length} of ${writes.length} could not be saved. They are still marked as unsaved, so you can try again.`,
      );
    }
  };

  const filterOptions: { value: FeatureFilter; label: string }[] = [
    { value: "all", label: "All" },
    { value: "enabled", label: "Available" },
    { value: "preview", label: "Preview" },
    { value: "changed", label: "Unsaved changes" },
  ];

  const menuNode = stateMenu ? nodeById.get(stateMenu.id) : undefined;
  const menuCurrent = menuNode ? (states.get(menuNode.id) ?? "enabled") : "enabled";
  const menuKind = menuNode && menuNode.depth === 0 ? "section" : "page";

  const rowControls = (id: string, label: string): JSX.Element => {
    const state = states.get(id) ?? "enabled";
    return (
      <Box sx={{ display: "flex", alignItems: "center", gap: 0.5, flexShrink: 0 }}>
        <Chip
          size="small"
          variant="outlined"
          color={state === "enabled" ? "success" : "info"}
          label={STATE_LABEL[state]}
          sx={{ mr: 1 }}
        />
        <Tooltip title={`Edit ${label}`}>
          <IconButton
            size="small"
            aria-label={`Edit ${label}`}
            aria-haspopup="menu"
            onClick={(event) => setStateMenu({ id, anchor: event.currentTarget })}
          >
            <Pencil size={16} />
          </IconButton>
        </Tooltip>
      </Box>
    );
  };

  const rowText = (id: string, label: string, bold: boolean): JSX.Element => {
    const changed = changedSet.has(id);
    const was = saved[id];
    return (
      <Box sx={{ minWidth: 0 }}>
        <Typography variant="body1" sx={{ fontWeight: bold ? 600 : 400 }}>
          {label}
        </Typography>
        {changed && (
          <Typography variant="caption" color="warning.main">
            Unsaved. Was {was ? STATE_LABEL[was] : "not set"}.
          </Typography>
        )}
      </Box>
    );
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Box sx={{ minWidth: 0 }}>
        <Typography variant="h6">Feature Visibility</Typography>
        <Typography variant="body2" color="text.secondary">
          Preview pages open for admins with a Preview tag, and are greyed out for everyone else. A page follows its
          section unless you set that page on its own. The menu and tabs follow your changes once you save.
        </Typography>
      </Box>

      {/* Shown while there is something to save, and kept in view as the list
          scrolls. The content area has 24px of padding, so the banner sticks that
          far up to sit flush with the top of the scroll area instead of leaving a
          strip of rows showing above it. */}
      {dirty && (
        <Box
          sx={{
            position: "sticky",
            top: (theme) => theme.spacing(-3),
            zIndex: 3,
            borderRadius: 1,
            bgcolor: "background.paper",
            // The theme's paper is translucent, so blur what scrolls behind the
            // banner or the rows underneath show through as faint text.
            backdropFilter: "blur(12px)",
            boxShadow: 2,
          }}
        >
          <Alert
            severity="warning"
            sx={{ alignItems: "center" }}
            action={
              <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                <Button color="inherit" disabled={saving} onClick={discard}>
                  Discard
                </Button>
                {/* Same as every other Save in the portal: contained, default primary colour. */}
                <Button variant="contained" disabled={saving || !data} onClick={() => void save()}>
                  {saving ? "Saving…" : "Save changes"}
                </Button>
              </Box>
            }
          >
            <AlertTitle sx={{ mb: 0 }}>
              {changedIds.length} unsaved {changedIds.length === 1 ? "change" : "changes"}
            </AlertTitle>
            Save to update the menu for everyone, or discard to go back to what is saved.
          </Alert>
        </Box>
      )}

      {query.isError && (
        <Alert
          severity="error"
          action={
            <Button color="inherit" size="small" onClick={() => void query.refetch()}>
              Retry
            </Button>
          }
        >
          Failed to load feature visibility.
        </Alert>
      )}
      {saveError && <Alert severity="error">{saveError}</Alert>}
      {data && !data.savedOnce && (
        <Alert severity="info">
          Nothing has been saved here yet, so the list below is what this environment&apos;s config file sets today (an
          older work-in-progress or hidden line shows as Preview). Your first save keeps those lines and adds your changes.
          After that the config file is no longer read.
        </Alert>
      )}

      {query.isPending && !query.isError && <Skeleton variant="rounded" height={320} />}

      {data && (
        <>
          <Box sx={{ display: "flex", flexWrap: "wrap", alignItems: "center", gap: 2 }}>
            <TextField
              size="small"
              placeholder="Search sections and pages"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              sx={{ minWidth: 280, flex: "1 1 280px", maxWidth: 420 }}
              slotProps={{
                input: {
                  startAdornment: (
                    <InputAdornment position="start">
                      <Search size={16} />
                    </InputAdornment>
                  ),
                },
                htmlInput: { "aria-label": "Search sections and pages" },
              }}
            />
            <TextField
              select
              size="small"
              label="Show"
              value={filter}
              onChange={(event) => setFilter(event.target.value as FeatureFilter)}
              sx={{ minWidth: 220 }}
            >
              {filterOptions.map((option) => (
                <MenuItem key={option.value} value={option.value}>
                  {option.label}
                </MenuItem>
              ))}
            </TextField>
          </Box>

          {selected.size > 0 && (
            <Box sx={{ display: "flex", alignItems: "center", gap: 2, flexWrap: "wrap" }}>
              <Typography variant="body2" color="text.secondary">
                {selected.size} {selected.size === 1 ? "item" : "items"} selected
              </Typography>
              <Button size="small" variant="text" onClick={() => setSelected(new Set())}>
                Clear selection
              </Button>
              <Button
                variant="outlined"
                endIcon={<ChevronDown size={16} />}
                aria-haspopup="menu"
                aria-expanded={bulkAnchor !== null}
                onClick={(event) => setBulkAnchor(event.currentTarget)}
              >
                Change status
              </Button>
            </Box>
          )}

          <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, bgcolor: "background.paper", overflow: "hidden" }}>
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                gap: 1,
                px: 2,
                py: 1.25,
                bgcolor: "action.hover",
                borderBottom: 1,
                borderColor: "divider",
              }}
            >
              <Box sx={{ width: 40, display: "flex", justifyContent: "center" }}>
                <Checkbox
                  size="small"
                  checked={allSelected}
                  indeterminate={someSelected && !allSelected}
                  onChange={() => setSelected(allSelected ? new Set() : new Set(displayedIds))}
                  inputProps={{ "aria-label": "Select all" }}
                  sx={{ p: 0 }}
                />
              </Box>
              <Typography variant="caption" color="text.secondary" sx={{ fontWeight: 600 }}>
                Section or page
              </Typography>
            </Box>

            {sections.length === 0 && (
              <Box sx={{ p: 3, textAlign: "center" }}>
                <Typography variant="body1">Nothing matches.</Typography>
                <Button
                  variant="text"
                  onClick={() => {
                    setSearch("");
                    setFilter("all");
                  }}
                >
                  Clear search and filter
                </Button>
              </Box>
            )}

            {sections.map(({ section, rows: below }, index) => {
              const hasPages = section.childIds.length > 0;
              const open = hasPages && (filtering || expanded.has(section.id));
              return (
                <Box key={section.id} sx={{ borderTop: index === 0 ? 0 : 1, borderColor: "divider" }}>
                  <Box
                    sx={{
                      display: "flex",
                      alignItems: "center",
                      gap: 1,
                      px: 2,
                      py: 1.25,
                      borderLeft: 3,
                      borderLeftColor: changedSet.has(section.id) ? "warning.main" : "transparent",
                      bgcolor: selected.has(section.id) ? "action.selected" : "transparent",
                    }}
                  >
                    <Box sx={{ width: 40, display: "flex", justifyContent: "center", flexShrink: 0 }}>
                      <Checkbox
                        size="small"
                        checked={selected.has(section.id)}
                        onChange={() => setSelected((current) => toggle(current, section.id))}
                        inputProps={{ "aria-label": `Select ${section.label}` }}
                        sx={{ p: 0 }}
                      />
                    </Box>
                    {hasPages ? (
                      <IconButton
                        size="small"
                        aria-label={`${open ? "Collapse" : "Expand"} ${section.label}`}
                        aria-expanded={open}
                        disabled={filtering}
                        onClick={() => setExpanded((current) => toggle(current, section.id))}
                      >
                        {open ? <ChevronDown size={18} /> : <ChevronRight size={18} />}
                      </IconButton>
                    ) : (
                      <Box sx={{ width: 30, flexShrink: 0 }} />
                    )}
                    <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 2, flex: 1, minWidth: 0 }}>
                      {rowText(section.id, section.label, true)}
                      {rowControls(section.id, section.label)}
                    </Box>
                  </Box>
                  {hasPages && (
                    <Collapse in={open} unmountOnExit>
                      {below.map((page) => (
                        <Box
                          key={page.id}
                          sx={{
                            display: "flex",
                            alignItems: "center",
                            gap: 1,
                            px: 2,
                            py: 1.25,
                            borderTop: 1,
                            borderColor: "divider",
                            borderLeft: 3,
                            borderLeftColor: changedSet.has(page.id) ? "warning.main" : "transparent",
                            bgcolor: selected.has(page.id) ? "action.selected" : "action.hover",
                          }}
                        >
                          <Box sx={{ width: 40, display: "flex", justifyContent: "center", flexShrink: 0 }}>
                            <Checkbox
                              size="small"
                              checked={selected.has(page.id)}
                              onChange={() => setSelected((current) => toggle(current, page.id))}
                              inputProps={{ "aria-label": `Select ${page.label}` }}
                              sx={{ p: 0 }}
                            />
                          </Box>
                          {/* Lines the page up under its section's name, one step further for each level down. */}
                          <Box sx={{ width: 30 + 24 * page.depth, flexShrink: 0 }} />
                          <Box
                            sx={{
                              display: "flex",
                              alignItems: "center",
                              justifyContent: "space-between",
                              gap: 2,
                              flex: 1,
                              minWidth: 0,
                            }}
                          >
                            {rowText(page.id, page.label, false)}
                            {rowControls(page.id, page.label)}
                          </Box>
                        </Box>
                      ))}
                    </Collapse>
                  )}
                </Box>
              );
            })}
          </Box>
        </>
      )}

      {/* After a save: what changed, from what to what. */}
      <Dialog open={savedChanges !== null} onClose={() => setSavedChanges(null)} fullWidth maxWidth="sm">
        <DialogTitle>Changes saved</DialogTitle>
        <DialogContent>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            {savedChanges?.length === 1 ? "This change is" : "These changes are"} now in effect for everyone.
          </Typography>
          <Box component="ul" aria-label="Saved changes" sx={{ listStyle: "none", m: 0, p: 0, display: "flex", flexDirection: "column", gap: 1.5 }}>
            {savedChanges?.map((change) => (
              <Box
                component="li"
                key={change.id}
                sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 2 }}
              >
                <Typography variant="body1" sx={{ fontWeight: 600 }}>
                  {change.label}
                </Typography>
                <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexShrink: 0 }}>
                  <Chip
                    size="small"
                    variant="outlined"
                    color={change.from === "enabled" ? "success" : "info"}
                    label={STATE_LABEL[change.from]}
                  />
                  <Typography variant="body2" color="text.secondary" aria-label="changed to">
                    →
                  </Typography>
                  <Chip
                    size="small"
                    variant="outlined"
                    color={change.to === "enabled" ? "success" : "info"}
                    label={STATE_LABEL[change.to]}
                  />
                </Box>
              </Box>
            ))}
          </Box>
        </DialogContent>
        <DialogActions>
          <Button variant="contained" onClick={() => setSavedChanges(null)}>
            Close
          </Button>
        </DialogActions>
      </Dialog>

      {/* Change status: one choice for every ticked row. */}
      <Menu
        anchorEl={bulkAnchor}
        open={bulkAnchor !== null}
        onClose={() => setBulkAnchor(null)}
        slotProps={{ paper: { sx: { minWidth: 340, maxWidth: 400 } } }}
      >
        {CHOICES.map((choice) => (
          <MenuItem
            key={choice}
            onClick={() => {
              setMany([...selected], choice);
              setBulkAnchor(null);
            }}
            sx={{ alignItems: "flex-start", whiteSpace: "normal" }}
          >
            <ListItemText primary={`Set to ${STATE_LABEL[choice]}`} secondary={BULK_HELP[choice]} />
          </MenuItem>
        ))}
      </Menu>

      {/* Pencil: pick the state for one row. */}
      <Menu
        anchorEl={stateMenu?.anchor ?? null}
        open={stateMenu !== null}
        onClose={() => setStateMenu(null)}
        slotProps={{ paper: { sx: { minWidth: 340, maxWidth: 400 } } }}
      >
        {stateMenu &&
          CHOICES.map((choice) => (
            <MenuItem
              key={choice}
              selected={menuCurrent === choice}
              onClick={() => {
                setMany([stateMenu.id], choice);
                setStateMenu(null);
              }}
              sx={{ alignItems: "flex-start", whiteSpace: "normal", gap: 1 }}
            >
              <ListItemIcon sx={{ minWidth: 24, mt: 0.5 }}>
                {menuCurrent === choice ? <Check size={16} /> : null}
              </ListItemIcon>
              <ListItemText
                primary={STATE_LABEL[choice]}
                secondary={STATE_HELP[menuKind][choice]}
                slotProps={{ primary: { sx: { fontWeight: menuCurrent === choice ? 600 : 400 } } }}
              />
            </MenuItem>
          ))}
      </Menu>
    </Box>
  );
}
