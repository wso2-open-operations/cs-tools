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
  FormControl,
  IconButton,
  InputAdornment,
  ListSubheader,
  MenuItem,
  Select,
  Skeleton,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { Code, LayoutDashboard, Pencil, Plus, SearchIcon, Share2, Trash2 } from "@wso2/oxygen-ui-icons-react";
import { type JSX, useState } from "react";
import { Navigate, useNavigate, useParams } from "react-router";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import DashboardWidgetGrid from "@features/csm-dashboard/components/DashboardWidgetGrid";
import {
  useDeleteCustomDashboard,
  useGetCustomDashboard,
  useGetCustomDashboards,
  useRenameCustomDashboard,
} from "@features/csm-custom-dashboards/api/useCustomDashboards";
import {
  type CodeDashboardSummary,
  savedCodeDashboardPath,
  useDeleteCodeDashboard,
  useGetCodeDashboard,
  useGetCodeDashboards,
  useRenameCodeDashboard,
} from "@features/csm-custom-dashboards/api/useCodeDashboards";
import { CODE_DASHBOARD_SHARING } from "@features/csm-custom-dashboards/api/useCodeDashboardShares";
import CodeDashboardFrame from "@features/csm-custom-dashboards/components/CodeDashboardFrame";
import DashboardNameDialog from "@features/csm-custom-dashboards/components/DashboardNameDialog";
import DeleteCustomDashboardDialog from "@features/csm-custom-dashboards/components/DeleteCustomDashboardDialog";
import ShareCustomDashboardDialog from "@features/csm-custom-dashboards/components/ShareCustomDashboardDialog";
import { renameErrorMessage } from "@features/csm-custom-dashboards/utils/saveErrorMessage";
import { useCanGenerateAdvancedDashboards } from "@features/csm-custom-dashboards/utils/advancedDashboardAccess";

const BASE_PATH = "/dashboard/my";
// A switcher value that opens a saved advanced dashboard, not a custom one.
const CODE_VALUE_PREFIX = "code:";

/** A dashboard of either kind: the open one, or the one the rename or delete dialog is about. */
interface DialogTarget {
  id: string;
  name: string;
  advanced: boolean;
}

/**
 * Same props as Create case (CsmCasesPage). With dashboards it sits in the
 * header next to the switcher; with none it is the centred call to action of
 * the empty state. Opens the generate page (`/dashboard/my/new`).
 */
function GenerateDashboardButton(): JSX.Element {
  const navigate = useNavigate();
  return (
    <Button
      variant="contained"
      color="primary"
      size="small"
      startIcon={<Plus size={16} />}
      onClick={() => navigate(`${BASE_PATH}/new`)}
    >
      Generate Dashboard
    </Button>
  );
}

/**
 * One saved dashboard (`GET /dashboards/custom/{id}`), drawn with Dashboard's
 * own widget grid (reused unchanged), the same grid the generate page's
 * preview uses, so a saved dashboard looks exactly as it did when it was
 * saved. Not `AgentsLandingPagePilot`: that one loads its dashboard from the
 * system registry (`GET /dashboards/{id}`), which a saved dashboard is not in.
 *
 * A dashboard saved under rules that have since changed comes back as an
 * error whose message asks the user to generate it again; it is shown as is.
 */
function SavedDashboardView({ id }: { id: string }): JSX.Element {
  const saved = useGetCustomDashboard(id);

  const frame = { border: 1, borderColor: "divider", borderRadius: 1, p: 2 };
  if (saved.isLoading) {
    return (
      <Box sx={frame}>
        <Skeleton variant="rounded" height={200} />
      </Box>
    );
  }
  if (saved.isError || !saved.data) {
    return (
      <Box sx={frame}>
        <Typography variant="body2" color="text.secondary">
          {saved.isError
            ? saved.error.message || "Could not load this dashboard."
            : "This dashboard no longer exists."}
        </Typography>
      </Box>
    );
  }
  return (
    <Box sx={frame}>
      <DashboardWidgetGrid widgets={saved.data.dashboard.widgets} linePointClickThrough />
    </Box>
  );
}

/**
 * One saved advanced dashboard (`GET /dashboards/code/{id}`): its saved source
 * run in CodeDashboardFrame (the source check and sandbox, with the viewer's
 * own data), in a panel as tall as the advanced page's preview, since the
 * frame fills its parent. Loading, gone and failed look as SavedDashboardView's.
 */
function SavedCodeDashboardView({ id }: { id: string }): JSX.Element {
  const saved = useGetCodeDashboard(id);

  const frame = { border: 1, borderColor: "divider", borderRadius: 1, p: 2 };
  if (saved.isLoading) {
    return (
      <Box sx={frame}>
        <Skeleton variant="rounded" height={200} />
      </Box>
    );
  }
  if (saved.isError || !saved.data) {
    return (
      <Box sx={frame}>
        <Typography variant="body2" color="text.secondary">
          {saved.isError
            ? saved.error.message || "Could not load this dashboard."
            : "This dashboard no longer exists."}
        </Typography>
      </Box>
    );
  }
  return (
    <Box
      data-testid="saved-code-dashboard-panel"
      sx={{ ...frame, height: "calc(100dvh - 240px)", minHeight: 520, display: "flex", flexDirection: "column" }}
    >
      <CodeDashboardFrame key={saved.data.source} source={saved.data.source} />
    </Box>
  );
}

/**
 * Dashboards > My Dashboards (`/dashboard/my`,
 * `/dashboard/my/:dashboardId`): the dashboards the user saved from the
 * generate page (`GET /dashboards/custom`, only their own), with a switcher
 * over them in the header. No id in the URL, or one that is not in the list,
 * opens the first one; while the list is being re-read (e.g. just after a
 * save) an unknown id waits for it instead, so a dashboard saved a moment ago
 * is opened rather than skipped.
 *
 * Each entry of the switcher has its own delete icon, as Saved views has
 * (SavedViewsMenu); clicking it closes the menu without switching and asks
 * first (DeleteCustomDashboardDialog), since a deleted dashboard cannot be
 * brought back. Deleting the open dashboard opens `/dashboard/my` (the first
 * one left, or the empty state); deleting another keeps the open one. A failed
 * delete shows the error banner and leaves the dashboard as it is.
 *
 * Beside it, a rename icon opens DashboardNameDialog with the entry's name
 * (`useRenameCustomDashboard`: the dashboard is saved again under the same id,
 * so its shares stay). A refused rename (a name the user already has, or a
 * dashboard saved under rules that have since changed) shows in the dialog,
 * which stays open; a rename that goes through closes it and shows the success
 * banner, and the switcher and the heading show the new name.
 *
 * The switcher's menu starts with a search box that filters the entries by
 * name as you type (case-insensitive; the list is the user's own, at most 50,
 * so it is filtered here, with no request). It takes focus when the menu
 * opens and is cleared when it closes. Typing there is not the menu's own
 * jump-to-letter, so keys stop at the box, except Escape, which closes.
 *
 * Share opens ShareCustomDashboardDialog for the open dashboard: pick people
 * (internal staff) to let them view it, view only, and remove them again.
 *
 * Edit with AI opens the open dashboard in the generate page
 * (`/dashboard/my/:id/edit`, EditCustomDashboardPage) to change it by chat.
 *
 * For a user who can use advanced dashboards (`canGenerateAdvancedDashboards`),
 * their saved advanced dashboards (`GET /dashboards/code`) follow in the
 * switcher under "Advanced", filtered by the same search, with the same
 * rename and delete icons, dialogs and banners (`useRenameCodeDashboard`,
 * `useDeleteCodeDashboard`). Choosing one opens it on this page
 * (`/dashboard/my/code/:codeDashboardId`) with the same header and buttons:
 * Share with the advanced dashboards' hooks (CODE_DASHBOARD_SHARING), Edit
 * with AI on the advanced page (`.../edit`, EditCodeDashboardPage), and
 * Generate Dashboard; SavedCodeDashboardView draws it. With no id the first
 * custom one opens, else the first advanced one; the empty state is for a
 * user with neither. That route is the 404 page for everyone else; nobody
 * else's list is read, and the page is otherwise unchanged for them.
 */
export default function MyDashboardsPage(): JSX.Element {
  const { dashboardId, codeDashboardId } = useParams<{ dashboardId?: string; codeDashboardId?: string }>();
  const navigate = useNavigate();
  const dashboardList = useGetCustomDashboards();
  const customDashboards = dashboardList.data ?? [];
  const canUseAdvanced = useCanGenerateAdvancedDashboards();
  const codeList = useGetCodeDashboards(canUseAdvanced);
  const codeDashboards: CodeDashboardSummary[] = codeList.data ?? [];
  const deleteDashboard = useDeleteCustomDashboard();
  const renameDashboard = useRenameCustomDashboard();
  const deleteCodeDashboard = useDeleteCodeDashboard();
  const renameCodeDashboard = useRenameCodeDashboard();
  const { showSuccess } = useSuccessBanner();
  const { showError } = useErrorBanner();
  const [menuOpen, setMenuOpen] = useState(false);
  const [search, setSearch] = useState("");
  // The dashboard the delete dialog asks about. Kept after the dialog closes,
  // so its name does not vanish during the closing animation.
  const [deleteTarget, setDeleteTarget] = useState<DialogTarget | null>(null);
  const [deleteOpen, setDeleteOpen] = useState(false);
  // The same for the rename dialog, with the reason the last rename was refused.
  const [renameTarget, setRenameTarget] = useState<DialogTarget | null>(null);
  const [renameOpen, setRenameOpen] = useState(false);
  const [renameError, setRenameError] = useState<string | null>(null);
  const [shareOpen, setShareOpen] = useState(false);
  const currentUserEmail = useIdTokenClaims()?.email;

  // As `/dashboard/my/new-code`: the backend's routes do not exist for them either.
  if (codeDashboardId !== undefined && !canUseAdvanced) return <Navigate to="/404" replace />;

  if (dashboardList.isError) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Typography variant="h5">My Dashboards</Typography>
        <Typography variant="body2" color="text.secondary">
          Could not load the dashboard list.
        </Typography>
      </Box>
    );
  }

  // At most one of the two ids is in the route.
  const selectedCustom = customDashboards.find((d) => d.id === dashboardId?.toLowerCase());
  const selectedCode = codeDashboards.find((d) => d.id === codeDashboardId?.toLowerCase());
  const selected: DialogTarget | undefined = selectedCustom
    ? { id: selectedCustom.id, name: selectedCustom.name, advanced: false }
    : selectedCode && { id: selectedCode.id, name: selectedCode.name, advanced: true };
  // Shown wherever the advanced list would be; the custom ones still work.
  const codeListError = codeList.isError && (
    <Typography variant="body2" color="text.secondary">
      Could not load your advanced dashboards.
    </Typography>
  );

  if (
    dashboardList.isLoading ||
    codeList.isLoading ||
    (dashboardId && !selectedCustom && dashboardList.isFetching) ||
    (codeDashboardId && !selectedCode && codeList.isFetching)
  ) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Skeleton variant="rounded" height={32} width={240} />
        <Skeleton variant="rounded" height={200} />
      </Box>
    );
  }

  if (customDashboards.length === 0 && codeDashboards.length === 0) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Typography variant="h5">My Dashboards</Typography>
        <Box
          sx={{
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            gap: 1,
            py: 8,
            textAlign: "center",
          }}
        >
          <LayoutDashboard size={32} />
          <Typography variant="h6">You have no dashboards yet</Typography>
          <Typography variant="body2" color="text.secondary">
            Dashboards you generate from a description will appear here. Only you can see them.
          </Typography>
          <Box sx={{ mt: 2, display: "flex", gap: 1 }}>
            <GenerateDashboardButton />
          </Box>
          {codeListError}
        </Box>
      </Box>
    );
  }

  if (!selected) {
    // No id, or one that is not in its list: open the first custom one, else the first advanced one.
    return (
      <Navigate
        to={
          customDashboards.length > 0
            ? `${BASE_PATH}/${encodeURIComponent(customDashboards[0].id)}`
            : savedCodeDashboardPath(codeDashboards[0].id)
        }
        replace
      />
    );
  }

  const query = search.trim().toLowerCase();
  const matches = (d: { name: string }): boolean => !query || d.name.toLowerCase().includes(query);
  const noMatch = query !== "" && !customDashboards.some(matches) && !codeDashboards.some(matches);

  const handleDelete = async (target: DialogTarget): Promise<void> => {
    try {
      await (target.advanced ? deleteCodeDashboard : deleteDashboard).mutateAsync(target.id);
      setDeleteOpen(false);
      showSuccess(`"${target.name}" was deleted.`);
      if (target.id === selected.id) void navigate(BASE_PATH);
    } catch (err) {
      setDeleteOpen(false);
      showError("The dashboard couldn't be deleted. Try again in a moment.", err);
    }
  };

  /** Renames `target`; a failure keeps the Rename dialog open with the error (a name already taken says so). */
  const handleRename = async (target: DialogTarget, name: string): Promise<void> => {
    setRenameError(null);
    try {
      await (target.advanced ? renameCodeDashboard : renameDashboard).mutateAsync({ id: target.id, name });
      setRenameOpen(false);
      showSuccess(`"${target.name}" was renamed to "${name}".`);
    } catch (err) {
      setRenameError(renameErrorMessage(err, name));
    }
  };

  /** A switcher entry's rename and delete icons, for either kind. */
  const entryActions = (target: DialogTarget): JSX.Element => (
    <Box sx={{ display: "flex", flexShrink: 0 }}>
      <Tooltip title="Rename dashboard">
        <IconButton
          size="small"
          aria-label={`Rename dashboard ${target.name}`}
          onClick={(e) => {
            // Not a selection: keep the open dashboard.
            e.stopPropagation();
            setMenuOpen(false);
            setRenameTarget(target);
            setRenameError(null);
            setRenameOpen(true);
          }}
        >
          <Pencil size={15} />
        </IconButton>
      </Tooltip>
      <Tooltip title="Delete dashboard">
        <IconButton
          size="small"
          edge="end"
          aria-label={`Delete dashboard ${target.name}`}
          onClick={(e) => {
            // Not a selection: keep the open dashboard.
            e.stopPropagation();
            setMenuOpen(false);
            setDeleteTarget(target);
            setDeleteOpen(true);
          }}
        >
          <Trash2 size={15} />
        </IconButton>
      </Tooltip>
    </Box>
  );

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 2,
          flexWrap: "wrap",
        }}
      >
        <Box>
          <Typography variant="h5">My Dashboards</Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
            {selected.name}
          </Typography>
        </Box>
        <Box sx={{ display: "flex", gap: 1, alignItems: "center", flexWrap: "wrap" }}>
          <FormControl size="small" sx={{ minWidth: 200 }}>
            <Select
              value={selected.advanced ? `${CODE_VALUE_PREFIX}${selected.id}` : selected.id}
              open={menuOpen}
              onOpen={() => setMenuOpen(true)}
              onClose={() => {
                setMenuOpen(false);
                setSearch("");
              }}
              // The search box takes focus, not the first entry.
              MenuProps={{ autoFocus: false, slotProps: { paper: { sx: { maxHeight: 400 } } } }}
              onChange={(e) => {
                const value = e.target.value;
                void navigate(
                  value.startsWith(CODE_VALUE_PREFIX)
                    ? savedCodeDashboardPath(value.slice(CODE_VALUE_PREFIX.length))
                    : `${BASE_PATH}/${encodeURIComponent(value)}`,
                );
              }}
              // The closed field shows the name only, not the entry's icons.
              renderValue={() => selected.name}
              // On the element screen readers announce: a plain aria-label
              // lands on Select's hidden input instead.
              SelectDisplayProps={{ "aria-label": "Select dashboard" }}
            >
              <ListSubheader sx={{ pt: 1, pb: 0.5, px: 1, bgcolor: "background.paper" }}>
                <TextField
                  size="small"
                  fullWidth
                  autoFocus
                  placeholder="Search dashboards"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key !== "Escape") e.stopPropagation();
                  }}
                  slotProps={{
                    htmlInput: { "aria-label": "Search dashboards" },
                    input: {
                      startAdornment: (
                        <InputAdornment position="start">
                          <SearchIcon size={16} />
                        </InputAdornment>
                      ),
                    },
                  }}
                />
              </ListSubheader>
              {noMatch && (
                <MenuItem disabled>
                  <Typography component="span" variant="inherit" color="text.secondary">
                    No dashboards match &ldquo;{search.trim()}&rdquo;
                  </Typography>
                </MenuItem>
              )}
              {customDashboards.map((d) => (
                <MenuItem
                  key={d.id}
                  value={d.id}
                  // An entry that does not match stays in the list, hidden and
                  // skipped by the arrow keys, so the open dashboard is always
                  // one of the Select's options.
                  disabled={!matches(d)}
                  sx={{ justifyContent: "space-between", gap: 2, ...(matches(d) ? {} : { display: "none" }) }}
                >
                  <Typography component="span" variant="inherit" noWrap>
                    {d.name}
                  </Typography>
                  {entryActions({ id: d.id, name: d.name, advanced: false })}
                </MenuItem>
              ))}
              {codeDashboards.length > 0 && codeDashboards.some(matches) && (
                <ListSubheader sx={{ lineHeight: 2, bgcolor: "background.paper" }}>Advanced</ListSubheader>
              )}
              {codeDashboards.map((d) => (
                <MenuItem
                  key={`${CODE_VALUE_PREFIX}${d.id}`}
                  value={`${CODE_VALUE_PREFIX}${d.id}`}
                  disabled={!matches(d)}
                  sx={{ justifyContent: "space-between", gap: 2, ...(matches(d) ? {} : { display: "none" }) }}
                >
                  <Box sx={{ display: "flex", alignItems: "center", gap: 1, minWidth: 0 }}>
                    <Code size={15} />
                    <Typography component="span" variant="inherit" noWrap>
                      {d.name}
                    </Typography>
                  </Box>
                  {entryActions({ id: d.id, name: d.name, advanced: true })}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
          <Button variant="outlined" size="small" startIcon={<Share2 size={16} />} onClick={() => setShareOpen(true)}>
            Share
          </Button>
          <Button
            variant="outlined"
            size="small"
            startIcon={<Pencil size={16} />}
            onClick={() =>
              void navigate(
                selected.advanced
                  ? `${savedCodeDashboardPath(selected.id)}/edit`
                  : `${BASE_PATH}/${encodeURIComponent(selected.id)}/edit`,
              )
            }
          >
            Edit with AI
          </Button>
          <GenerateDashboardButton />
        </Box>
      </Box>
      {codeListError}
      {selected.advanced ? <SavedCodeDashboardView id={selected.id} /> : <SavedDashboardView id={selected.id} />}
      <ShareCustomDashboardDialog
        // Keyed by kind: the dialog calls the hooks it is given, so a new kind is a new dialog.
        key={selected.advanced ? "advanced" : "custom"}
        open={shareOpen}
        dashboard={{ id: selected.id, name: selected.name }}
        currentUserEmail={currentUserEmail}
        onClose={() => setShareOpen(false)}
        hooks={selected.advanced ? CODE_DASHBOARD_SHARING : undefined}
      />
      <DashboardNameDialog
        open={renameOpen}
        title="Rename dashboard"
        initialName={renameTarget?.name ?? ""}
        confirmLabel="Rename"
        busyLabel="Renaming…"
        busy={(renameTarget?.advanced ? renameCodeDashboard : renameDashboard).isPending}
        error={renameError}
        requireChange
        onCancel={() => setRenameOpen(false)}
        onConfirm={(name) => {
          if (renameTarget) void handleRename(renameTarget, name);
        }}
      />
      <DeleteCustomDashboardDialog
        open={deleteOpen}
        name={deleteTarget?.name ?? ""}
        deleting={(deleteTarget?.advanced ? deleteCodeDashboard : deleteDashboard).isPending}
        onCancel={() => setDeleteOpen(false)}
        onConfirm={() => {
          if (deleteTarget) void handleDelete(deleteTarget);
        }}
      />
    </Box>
  );
}
