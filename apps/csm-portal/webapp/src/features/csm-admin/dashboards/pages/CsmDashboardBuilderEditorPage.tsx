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
  Box,
  Button,
  Checkbox,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  FormControlLabel,
  IconButton,
  MenuItem,
  Select,
  Skeleton,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowLeft, Copy, Plus, Settings, Trash2 } from "@wso2/oxygen-ui-icons-react";
import { useEffect, useMemo, useRef, useState, type JSX } from "react";
import { useNavigate, useParams } from "react-router";
import type { BeDashboardWidget } from "@api/backend/types";
import { useDashboard } from "@features/csm-dashboard/api/useDashboard";
import { abtFamilyForDashboardType, useTeams } from "@features/csm-dashboard/api/useTeams";
import { ALL_TEAMS_SENTINEL } from "@features/csm-dashboard/utils/teamFilterPlaceholder";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import DashboardWidgetGrid from "@features/csm-dashboard/components/DashboardWidgetGrid";
import SectionCard from "@features/csm-dashboard/components/SectionCard";
import { WIDGET_GRID_SX } from "@features/csm-dashboard/utils/dashboardWidgetGridLayout";
import WidgetEditorDialog from "@features/csm-admin/dashboards/components/WidgetEditorDialog";
import {
  useDashboardFilterPresets,
  useDashboardSharedSections,
} from "@features/csm-admin/dashboards/api/useDashboardSharedConfig";
import { deployableDashboardFromDraft } from "@features/csm-admin/dashboards/utils/sharedConfigDraftsStorage";
import { isDraftDrifted } from "@features/csm-admin/dashboards/utils/dashboardDrift";
import {
  newDraftId,
  saveDashboardDraft,
  useDashboardDraft,
  type DashboardDraft,
  type SectionInclude,
} from "@features/csm-admin/dashboards/utils/dashboardDraftsStorage";

const DASHBOARD_TYPES = ["cre", "sre", "cs"] as const;

function emptyDraft(id: string): Omit<DashboardDraft, "updatedAt"> {
  return {
    id,
    displayName: "",
    isDefault: false,
    isTeamBased: false,
    widgets: [],
    emptySections: [],
  };
}

function draftFromLive(id: string, live: NonNullable<ReturnType<typeof useDashboard>["data"]>): Omit<DashboardDraft, "updatedAt"> {
  return {
    id,
    sourceDashboardId: id,
    displayName: live.displayName,
    isDefault: live.isDefault,
    isTeamBased: live.isTeamBased,
    targetTeam: live.targetTeam,
    widgets: live.widgets,
    emptySections: [],
  };
}

/**
 * Dashboard builder editor: create-or-edit one dashboard, entirely against
 * `localStorage` — there is no dashboard CRUD API, and none is added here
 * (see this module's own `dashboardDraftsStorage.ts`). The draft's widgets
 * render through the exact same `DashboardWidgetGrid`/`DashboardWidgetTile`
 * the live dashboard page uses (see `DashboardWidgetGrid.tsx`), with a
 * settings-gear overlay per tile and add/remove affordances for both
 * widgets and sections layered on top via that component's own optional
 * `renderWidgetAction`/`renderSectionActions`/`trailingContent` props.
 *
 * Deploying a draft is a manual, out-of-band step: a maintainer takes the
 * dashboard's JSON (see the "Copy as JSON" action below) and lands it in
 * the config-driven dashboard registry's own repo, then redeploys that
 * Choreo component. This page never talks to a write endpoint.
 */
export default function CsmDashboardBuilderEditorPage(): JSX.Element {
  const params = useParams<{ draftId: string }>();
  const navigate = useNavigate();

  // "/admin/dashboards/new" has no :draftId of its own — mint one now and
  // immediately replace the URL with it, so a reload (or a bookmark/share)
  // resumes the SAME draft instead of minting a second one every visit.
  const [generatedId] = useState(() => newDraftId());
  const draftId = params.draftId ?? generatedId;
  useEffect(() => {
    if (!params.draftId) navigate(`/admin/dashboards/${generatedId}`, { replace: true });
  }, [params.draftId, generatedId, navigate]);

  const localDraft = useDashboardDraft(draftId);
  // Always attempted, even for a draft id that was never deployed — a 404
  // resolves to `null` (see `BackendApi.get`), which both seeds a brand-new
  // draft correctly (nothing to copy from) and later reports "not deployed"
  // from the drift check without any special-casing of which kind of id
  // this is.
  const live = useDashboard(draftId);

  // Seed a first-ever local draft exactly once per draftId, only after the
  // live fetch has settled — seeding an empty draft while `live` is still
  // in flight would (for a real deployed dashboard id) briefly blank the
  // editor before the real content arrives, and would race a redundant
  // "empty" save against the real seed a moment later.
  const seededFor = useRef<string | undefined>(undefined);
  useEffect(() => {
    if (localDraft || live.isLoading || seededFor.current === draftId) return;
    seededFor.current = draftId;
    const seed = live.data ? draftFromLive(draftId, live.data) : emptyDraft(draftId);
    saveDashboardDraft(seed);
  }, [draftId, localDraft, live.data, live.isLoading]);

  // The editor's own working copy — bound to form controls directly, then
  // debounce-persisted to storage (see the effect below). Re-seeded from
  // `localDraft` whenever the ROUTE's draftId changes (a real navigation
  // between two different drafts), never merely because storage emitted a
  // change event for the save this same effect below just made. Adjusted
  // during render (React's own "you might not need an effect" pattern for
  // state that must re-derive when a prop/id changes), not inside a
  // `useEffect`, so this never causes an extra committed render just to
  // catch up.
  const [working, setWorking] = useState<DashboardDraft | undefined>(localDraft);
  const [loadedDraftId, setLoadedDraftId] = useState<string | undefined>(localDraft?.id);
  if (localDraft && loadedDraftId !== localDraft.id) {
    setLoadedDraftId(localDraft.id);
    setWorking(localDraft);
  }

  // True while `working`'s NEXT change is a re-seed from storage (a draft
  // load, or switching to a different draftId) rather than something the
  // admin actually edited — re-armed by the effect below every time
  // `loadedDraftId` changes, and cleared once the autosave effect further
  // down has consumed it. Refs (not render-time mutation, which the
  // `react-hooks/refs` lint rule rejects) so setting it never itself
  // triggers a render. Without this, the autosave effect re-stamps
  // `updatedAt` (and bumps this draft to the top of the local-drafts list)
  // on every single open of an existing, completely unmodified draft — the
  // two effects below run in declaration order within the same commit (the
  // `loadedDraftId` change and the resulting `working` change land
  // together), so this is always armed before the autosave effect below
  // ever gets a chance to see it.
  const skipNextAutosaveRef = useRef(true);
  useEffect(() => {
    skipNextAutosaveRef.current = true;
  }, [loadedDraftId]);

  const [savedAt, setSavedAt] = useState<string | undefined>(localDraft?.updatedAt);
  // Both refs, not state — read only from the unmount-flush effect below,
  // so keeping them current never itself triggers a render. Kept current
  // via its own effect (never mutated during render — React's own
  // `react-hooks/refs` lint rule rejects that) rather than folded into the
  // debounce effect below, so it's always current even on the render where
  // `skipNextAutosaveRef` causes that effect to bail out early.
  const workingRef = useRef(working);
  useEffect(() => {
    workingRef.current = working;
  }, [working]);
  const pendingSaveRef = useRef(false);
  useEffect(() => {
    if (!working) return;
    if (skipNextAutosaveRef.current) {
      skipNextAutosaveRef.current = false;
      return;
    }
    pendingSaveRef.current = true;
    const timer = setTimeout(() => {
      pendingSaveRef.current = false;
      const saved = saveDashboardDraft(working);
      setSavedAt(saved.updatedAt);
    }, 300);
    // Ordinary debounce cleanup: fires on every `working` change (each
    // keystroke) to cancel the stale timer, NOT a flush — flushing here
    // too would write to storage on every keystroke and defeat the
    // debounce entirely. The real "did this edit ever get persisted?"
    // flush is the mount-once effect below, which only runs its own
    // cleanup on actual unmount.
    return () => clearTimeout(timer);
  }, [working]);

  // Runs its cleanup exactly once, on unmount (empty deps) — catches the
  // case a debounce window above never got to fire naturally because the
  // admin navigated away (e.g. "Back to dashboards") within 300ms of their
  // last keystroke. This feature has no backend of its own (see this
  // page's own doc comment) — `working` is the only copy of that edit, so
  // losing it here means losing it for good, not "it'll autosave again
  // later".
  useEffect(() => {
    return () => {
      if (pendingSaveRef.current && workingRef.current) {
        saveDashboardDraft(workingRef.current);
      }
    };
  }, []);

  // Fetched here, not in the widget dialog, and the dialog is not mounted
  // until it has settled: the dialog seeds its filter rows in a useState
  // initializer that needs the catalogue on the very first render (see
  // WidgetEditorDialogProps.presets). Starting the query at page mount means
  // it is long cached by the time an admin opens a widget.
  const {
    data: filterPresets,
    isError: filterPresetsFailed,
    refetch: refetchFilterPresets,
  } = useDashboardFilterPresets();
  const {
    data: sharedSections,
    isPending: sharedSectionsPending,
    isError: sharedSectionsFailed,
    refetch: refetchSharedSections,
  } = useDashboardSharedSections();

  // Exporting without the preset catalogue can only LOSE something when the
  // draft came from a deployed dashboard: those widgets hold the API's
  // already-expanded filters, and collapse-back needs the catalogue to turn
  // them into references again. A draft that was authored here has its
  // references stored as {"preset": ...} in the draft already (the editor
  // wrote them that way), so its export is lossless with or without the
  // catalogue — blocking that case would break the builder for no gain, the
  // same reasoning the widget-editor gate uses.
  const exportCouldLoseReferences =
    working?.sourceDashboardId !== undefined && filterPresets === undefined;
  const [editingWidget, setEditingWidget] = useState<
    { widget: BeDashboardWidget | undefined; defaultSection?: string } | undefined
  >(undefined);
  const [pendingRemoval, setPendingRemoval] = useState<
    { kind: "widget"; widgetId: string; label: string } | { kind: "section"; section: string } | undefined
  >(undefined);
  const [copyFeedback, setCopyFeedback] = useState(false);

  // The drift check needs `GET /dashboards/{draftId}` to have actually
  // resolved: `live.data` is `undefined` both "definitely not deployed" and
  // "haven't heard back yet", and `isDraftDrifted` treats an `undefined`
  // live value as "drifted" either way (see its own doc comment) — without
  // gating on `live.isLoading`, the "not yet deployed" banner flashes on
  // every single open of an existing, completely unmodified dashboard while
  // the fetch is still in flight. A failed fetch (`isError`) is its own
  // distinct state, not silently treated as "no drift" (which would hide a
  // real drift) or "always drifted" (which would false-alarm a dashboard
  // that's actually fine) — surfaced as its own "couldn't check" notice
  // below instead. Memoized (bug: this used to recompute a full recursive
  // canonicalize+stringify of the whole draft/live shape on every render,
  // including every keystroke).
  const driftStatus = useMemo<"checking" | "clean" | "drifted" | "check-failed">(() => {
    if (!working) return "checking";
    if (live.isLoading) return "checking";
    if (live.isError) return "check-failed";
    return isDraftDrifted(working, live.data ?? undefined) ? "drifted" : "clean";
  }, [working, live.data, live.isLoading, live.isError]);

  // Team context for the Preview tile (and this page's own widget grid),
  // threaded through exactly the way `CsmDashboardPage` resolves it for the
  // live dashboard (see that page's own doc comment): the signed-in user's
  // own team defaults the selector once their profile resolves, "All ABTs"
  // otherwise, and the admin can override it here to preview any team.
  // Unlike `CsmDashboardPage`, the team list itself is fetched already
  // scoped to `working.type`'s family (see `abtFamilyForDashboardType`) —
  // there's no separate unscoped query for resolving a default team outside
  // that family the way the live page needs (a signed-in admin's own team
  // being outside the dashboard's family is an edge case not worth the
  // extra query here; the selector simply starts blank in that case until
  // the admin picks one).
  const isTeamBased = working?.isTeamBased ?? false;
  const currentUser = useCurrentUser();
  const teamFamily = abtFamilyForDashboardType(working?.type);
  const teams = useTeams(isTeamBased, teamFamily);
  const [previewTeamOverride, setPreviewTeamOverride] = useState<string | undefined>(undefined);
  const userHasTeam = Boolean(currentUser.user?.team);
  const defaultPreviewTeamId = isTeamBased
    ? userHasTeam
      ? currentUser.user?.team?.teamKey
      : ALL_TEAMS_SENTINEL
    : undefined;
  const previewTeamId = previewTeamOverride ?? defaultPreviewTeamId;
  const previewTeam = teams.data?.find((t) => t.id === previewTeamId);
  const selectedTeamCreGroupId: string | string[] | undefined =
    previewTeamId === ALL_TEAMS_SENTINEL
      ? (teams.data ?? []).map((t) => t.creGroupId).filter((g): g is string => Boolean(g))
      : previewTeam?.creGroupId;
  const selectedTeamSreGroupId: string | string[] | undefined =
    previewTeamId === ALL_TEAMS_SENTINEL
      ? (teams.data ?? []).map((t) => t.sreGroupId).filter((g): g is string => Boolean(g))
      : previewTeam?.sreGroupId;
  const selectedTeamLabel: string | undefined =
    previewTeamId === ALL_TEAMS_SENTINEL ? "All ABTs" : previewTeam?.name;

  const sectionNames = useMemo(
    () =>
      working
        ? [
            ...new Set([
              ...working.widgets.map((w) => w.section).filter((s): s is string => Boolean(s)),
              ...working.emptySections,
            ]),
          ]
        : [],
    [working],
  );

  if (!working) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <Skeleton variant="rounded" height={32} width={280} />
        <Box sx={WIDGET_GRID_SX}>
          {Array.from({ length: 3 }, (_, i) => (
            <Skeleton key={i} variant="rounded" height={90} sx={{ gridColumn: "span 4" }} />
          ))}
        </Box>
      </Box>
    );
  }

  const updateWorking = (patch: Partial<DashboardDraft>): void =>
    setWorking((prev) => (prev ? { ...prev, ...patch } : prev));

  const handleSaveWidget = (widget: BeDashboardWidget): void => {
    const exists = working.widgets.some((w) => w.widgetId === widget.widgetId);
    const nextWidgets = exists
      ? working.widgets.map((w) => (w.widgetId === widget.widgetId ? widget : w))
      : [...working.widgets, widget];
    // A widget that lands in a real section is no longer an empty
    // scaffold — drop any matching placeholder so it isn't rendered twice
    // (once as the widget's own group, once as a leftover empty shell).
    const nextEmptySections = working.emptySections.filter((s) => s !== widget.section);
    updateWorking({ widgets: nextWidgets, emptySections: nextEmptySections });
    setEditingWidget(undefined);
  };

  const handleConfirmRemoval = (): void => {
    if (!pendingRemoval) return;
    if (pendingRemoval.kind === "widget") {
      updateWorking({ widgets: working.widgets.filter((w) => w.widgetId !== pendingRemoval.widgetId) });
    } else {
      updateWorking({
        widgets: working.widgets.filter((w) => w.section !== pendingRemoval.section),
        emptySections: working.emptySections.filter((s) => s !== pendingRemoval.section),
      });
    }
    setPendingRemoval(undefined);
  };

  const handleAddSection = (): void => {
    const name = window.prompt("New section name");
    const trimmed = name?.trim();
    if (!trimmed) return;
    if (sectionNames.includes(trimmed)) return;
    updateWorking({ emptySections: [...working.emptySections, trimmed] });
  };

  const handleCopyJson = async (): Promise<void> => {
    // Guarded here as well as on the button: a disabled button is a UI
    // affordance, not an invariant, and this must not be reachable from
    // anywhere else while the catalogue is missing.
    if (exportCouldLoseReferences) return;
    const deployable = deployableDashboardFromDraft(working, filterPresets);
    try {
      await navigator.clipboard.writeText(JSON.stringify(deployable, null, 2));
      setCopyFeedback(true);
      setTimeout(() => setCopyFeedback(false), 2000);
    } catch {
      // Clipboard access denied/unavailable — nothing else to fall back to
      // here; the admin can still read the JSON via devtools if needed.
    }
  };

  const emptySectionsToRender = working.emptySections.filter(
    (s) => !working.widgets.some((w) => w.section === s),
  );

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
        <Button
          variant="text"
          size="small"
          startIcon={<ArrowLeft size={16} />}
          onClick={() => navigate("/admin/dashboards")}
        >
          Back to dashboards
        </Button>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
          <Typography variant="caption" color="text.secondary">
            {savedAt ? `Saved locally ${new Date(savedAt).toLocaleTimeString()}` : "Not saved yet"}
          </Typography>
          <Tooltip
            title={
              exportCouldLoseReferences
                ? "Unavailable while the shared filter presets cannot be loaded: exporting now would turn this dashboard's shared preset references into fixed copies."
                : "Copy this dashboard's deployable JSON to the clipboard"
            }
          >
            <Button
              size="small"
              variant="outlined"
              startIcon={<Copy size={14} />}
              disabled={exportCouldLoseReferences}
              onClick={() => void handleCopyJson()}
            >
              {copyFeedback ? "Copied!" : "Copy as JSON"}
            </Button>
          </Tooltip>
        </Box>
      </Box>

      {driftStatus === "drifted" && (
        <Alert severity="warning">
          {working.sourceDashboardId
            ? "This local draft has changes that are not yet deployed — it differs from what GET /dashboards currently returns. Use “Copy as JSON” and hand it to a maintainer to redeploy."
            : "This dashboard has never been deployed — it exists only in this browser's local storage until a maintainer ships its JSON."}
        </Alert>
      )}
      {driftStatus === "check-failed" && (
        <Alert severity="info">
          Couldn't check this draft against what's currently deployed (GET /dashboards/{draftId}{" "}
          failed) — edits are still saved locally regardless.
        </Alert>
      )}

      {filterPresetsFailed && (
        // The widget editor stays shut while this is showing (see the
        // editingWidget gate below). That is deliberate: without the preset
        // catalogue, an existing widget's already-expanded filters cannot be
        // recognised as the presets they came from, so opening the editor and
        // saving would rewrite every shared reference on this dashboard as a
        // literal -- silently, and with no way to tell afterwards. A shut
        // editor with a reason beats a save that quietly strips references.
        <Alert
          severity="error"
          action={
            <Button
              size="small"
              color="inherit"
              onClick={() => void refetchFilterPresets()}
            >
              Retry
            </Button>
          }
        >
          Could not load the shared filter presets, so widgets cannot be edited
          safely right now — saving one would turn its shared preset references
          into fixed copies. Everything else on this page still works.
        </Alert>
      )}

      <SectionCard title="Dashboard settings">
        <Box sx={{ display: "flex", flexWrap: "wrap", gap: 2 }}>
          <TextField
            label="Display name"
            required
            value={working.displayName}
            onChange={(e) => updateWorking({ displayName: e.target.value })}
            size="small"
            sx={{ flex: "1 1 260px" }}
            slotProps={{ htmlInput: { "aria-label": "Dashboard display name" } }}
          />
          <TextField
            select
            label="Type (optional)"
            value={working.type ?? ""}
            onChange={(e) => updateWorking({ type: (e.target.value || undefined) as DashboardDraft["type"] })}
            size="small"
            sx={{ minWidth: 160 }}
            slotProps={{
              inputLabel: {
                shrink: (working.type ?? "") !== "",
                sx: { top: "0px !important" },
              },
              select: { notched: (working.type ?? "") !== "" },
            }}
          >
            <MenuItem value="">None</MenuItem>
            {DASHBOARD_TYPES.map((t) => (
              <MenuItem key={t} value={t}>
                {t.toUpperCase()}
              </MenuItem>
            ))}
          </TextField>
          <TextField
            label="Target team (optional)"
            value={working.targetTeam ?? ""}
            onChange={(e) => updateWorking({ targetTeam: e.target.value || undefined })}
            size="small"
            sx={{ minWidth: 200 }}
          />
          <FormControlLabel
            control={
              <Checkbox
                checked={working.isDefault}
                onChange={(e) => updateWorking({ isDefault: e.target.checked })}
              />
            }
            label="Default dashboard"
          />
          <FormControlLabel
            control={
              <Checkbox
                checked={working.isTeamBased}
                onChange={(e) => updateWorking({ isTeamBased: e.target.checked })}
              />
            }
            label="Team-based"
          />
          {isTeamBased && (
            <FormControl size="small" sx={{ minWidth: 200 }}>
              <Select
                value={previewTeamId ?? ""}
                onChange={(e) => setPreviewTeamOverride(e.target.value || undefined)}
                displayEmpty
                aria-label="Preview team"
              >
                <MenuItem value={ALL_TEAMS_SENTINEL}>All ABTs</MenuItem>
                {(teams.data ?? []).map((t) => (
                  <MenuItem key={t.id} value={t.id}>
                    {t.name}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          )}
        </Box>
      </SectionCard>

      <SectionCard
        title="Shared sections"
        action={
          <Button
            size="small"
            variant="outlined"
            startIcon={<Plus size={14} />}
            disabled={(sharedSections ?? []).length === 0}
            onClick={() =>
              updateWorking({
                includeSections: [
                  ...(working.includeSections ?? []),
                  { section: (sharedSections ?? [])[0]?.name ?? "", position: "start" },
                ],
              })
            }
          >
            Include a section
          </Button>
        }
      >
        {/* Three distinct states, not two. Collapsing "still loading" and
            "the request failed" into the same empty-list message told the
            admin this deployment has no shared sections, which in both cases
            is a claim we have no basis for -- and the one case where it IS
            true looks identical, so there was no way to tell them apart.
            The failure notice sits ALONGSIDE the list rather than replacing
            it: whatever this dashboard already includes is still real and
            still removable, it just cannot be added to until the catalogue
            loads. */}
        {sharedSectionsFailed && (
          <Alert
            severity="error"
            sx={{ mb: 1.5 }}
            action={
              <Button
                size="small"
                color="inherit"
                onClick={() => void refetchSharedSections()}
              >
                Retry
              </Button>
            }
          >
            Could not load the shared sections, so there is nothing new to
            choose from right now. Anything this dashboard already includes is
            unaffected.
          </Alert>
        )}
        {sharedSectionsPending ? (
          <Typography variant="body2" color="text.secondary">
            Loading shared sections&hellip;
          </Typography>
        ) : !sharedSectionsFailed && (sharedSections ?? []).length === 0 ? (
          <Typography variant="body2" color="text.secondary">
            This deployment has no shared sections yet. Design one under
            &ldquo;Shared presets &amp; sections&rdquo; on the dashboards list.
          </Typography>
        ) : (working.includeSections ?? []).length === 0 ? (
          <Typography variant="body2" color="text.secondary">
            None. Including a shared section pulls its widgets in by reference,
            so editing the section changes every dashboard that includes it.
          </Typography>
        ) : (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
            {(working.includeSections ?? []).map((include, index) => {
              const patch = (next: Partial<SectionInclude>): void =>
                updateWorking({
                  includeSections: (working.includeSections ?? []).map((inc, i) =>
                    i === index ? { ...inc, ...next } : inc,
                  ),
                });
              return (
                <Box
                  key={index}
                  sx={{ display: "flex", flexWrap: "wrap", gap: 1, alignItems: "flex-start" }}
                >
                  <TextField
                    select
                    size="small"
                    label="Section"
                    value={include.section}
                    onChange={(e) => patch({ section: e.target.value })}
                    sx={{ minWidth: 200 }}
                    slotProps={{
                      inputLabel: { shrink: true, sx: { top: "0px !important" } },
                      select: { notched: true },
                    }}
                  >
                    {(sharedSections ?? []).map((sec) => (
                      <MenuItem key={sec.name} value={sec.name}>
                        {sec.name}
                      </MenuItem>
                    ))}
                  </TextField>
                  <TextField
                    select
                    size="small"
                    label="Position"
                    value={include.position ?? "start"}
                    onChange={(e) =>
                      patch({ position: e.target.value as "start" | "end" })
                    }
                    sx={{ minWidth: 140 }}
                    slotProps={{
                      inputLabel: { shrink: true, sx: { top: "0px !important" } },
                      select: { notched: true },
                    }}
                  >
                    <MenuItem value="start">Before own widgets</MenuItem>
                    <MenuItem value="end">After own widgets</MenuItem>
                  </TextField>
                  <TextField
                    size="small"
                    label="Widget id prefix"
                    value={include.idPrefix ?? ""}
                    onChange={(e) => patch({ idPrefix: e.target.value || undefined })}
                    helperText="Optional, e.g. ob_"
                    sx={{ minWidth: 150 }}
                  />
                  <TextField
                    size="small"
                    label="Heading override"
                    value={include.displayName ?? ""}
                    onChange={(e) => patch({ displayName: e.target.value || undefined })}
                    helperText="Optional"
                    sx={{ minWidth: 170 }}
                  />
                  <Tooltip title="Stop including this section">
                    <IconButton
                      size="small"
                      aria-label={`Remove included section ${include.section}`}
                      onClick={() =>
                        updateWorking({
                          includeSections: (working.includeSections ?? []).filter(
                            (_, i) => i !== index,
                          ),
                        })
                      }
                    >
                      <Trash2 size={16} />
                    </IconButton>
                  </Tooltip>
                </Box>
              );
            })}
            <Typography variant="caption" color="text.secondary">
              Included widgets are not shown in the preview below: they are
              resolved when the definition is loaded, from whatever the shared
              section says at that moment.
            </Typography>
          </Box>
        )}
      </SectionCard>

      <SectionCard
        title="Widgets"
        action={
          <Box sx={{ display: "flex", gap: 1 }}>
            <Button size="small" variant="outlined" startIcon={<Plus size={14} />} onClick={handleAddSection}>
              Add section
            </Button>
            <Button
              size="small"
              variant="contained"
              startIcon={<Plus size={14} />}
              onClick={() => setEditingWidget({ widget: undefined })}
            >
              Add widget
            </Button>
          </Box>
        }
      >
        {working.widgets.length === 0 && emptySectionsToRender.length === 0 ? (
          <Typography variant="body2" color="text.secondary">
            This dashboard has no widgets yet — use "Add widget" above to add the first one.
          </Typography>
        ) : (
          <DashboardWidgetGrid
            widgets={working.widgets}
            selectedTeamCreGroupId={selectedTeamCreGroupId}
            selectedTeamSreGroupId={selectedTeamSreGroupId}
            selectedTeamLabel={selectedTeamLabel}
            renderWidgetAction={(widget) => (
              <Box sx={{ display: "flex", gap: 0.5 }}>
                <Tooltip title={`Edit ${widget.displayName}`}>
                  <IconButton
                    size="small"
                    aria-label={`Edit widget ${widget.displayName}`}
                    onClick={() => setEditingWidget({ widget })}
                    sx={{ bgcolor: "background.paper", boxShadow: 1 }}
                  >
                    <Settings size={14} />
                  </IconButton>
                </Tooltip>
                <Tooltip title={`Remove ${widget.displayName}`}>
                  <IconButton
                    size="small"
                    aria-label={`Remove widget ${widget.displayName}`}
                    onClick={() =>
                      setPendingRemoval({
                        kind: "widget",
                        widgetId: widget.widgetId,
                        label: widget.displayName,
                      })
                    }
                    sx={{ bgcolor: "background.paper", boxShadow: 1 }}
                  >
                    <Trash2 size={14} />
                  </IconButton>
                </Tooltip>
              </Box>
            )}
            renderSectionActions={(section, sectionTitle, widgetIds) =>
              section ? (
                <>
                  <Button
                    size="small"
                    variant="text"
                    startIcon={<Plus size={14} />}
                    // The RAW section key, not the display-resolved title —
                    // a widget added here must land in the same section it
                    // was opened from (`widget.section === section`), which
                    // a placeholder-resolved title (e.g. "{{currentTeam}}
                    // Escalations" -> "All ABTs Escalations") would silently
                    // fork into a brand-new section instead.
                    onClick={() => setEditingWidget({ widget: undefined, defaultSection: section })}
                  >
                    Add widget
                  </Button>
                  <Tooltip
                    title={`Remove section "${sectionTitle ?? section}" and every widget in it (${widgetIds.size})`}
                  >
                    <IconButton
                      size="small"
                      aria-label={`Remove section ${sectionTitle ?? section}`}
                      // Same reasoning as above: `pendingRemoval.section` is
                      // matched against `widget.section` (raw) on confirm.
                      onClick={() => setPendingRemoval({ kind: "section", section })}
                    >
                      <Trash2 size={14} />
                    </IconButton>
                  </Tooltip>
                </>
              ) : undefined
            }
            trailingContent={
              emptySectionsToRender.length > 0 && (
                <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
                  {emptySectionsToRender.map((section) => (
                    <Box
                      key={section}
                      sx={{
                        border: 1,
                        borderColor: "divider",
                        borderStyle: "dashed",
                        borderRadius: 1,
                        p: 2,
                        display: "flex",
                        alignItems: "center",
                        justifyContent: "space-between",
                      }}
                    >
                      <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>
                        {section}
                      </Typography>
                      <Box sx={{ display: "flex", gap: 1 }}>
                        <Button
                          size="small"
                          startIcon={<Plus size={14} />}
                          onClick={() => setEditingWidget({ widget: undefined, defaultSection: section })}
                        >
                          Add widget
                        </Button>
                        <IconButton
                          size="small"
                          aria-label={`Remove section ${section}`}
                          onClick={() => setPendingRemoval({ kind: "section", section })}
                        >
                          <Trash2 size={14} />
                        </IconButton>
                      </Box>
                    </Box>
                  ))}
                </Box>
              )
            }
          />
        )}
      </SectionCard>

      {/* A NEW widget needs no catalogue: it has no server-expanded filters to
          misread, and with no catalogue the editor simply offers no preset rows,
          so nothing can be lost. Editing an EXISTING widget does need it —
          without it, that widget's expanded filters would be re-saved as
          literals and its shared references silently dropped. */}
      {editingWidget &&
        (editingWidget.widget === undefined || filterPresets !== undefined) && (
        <WidgetEditorDialog
          presets={filterPresets}
          widget={editingWidget.widget}
          defaultSection={editingWidget.defaultSection}
          sectionSuggestions={sectionNames}
          selectedTeamCreGroupId={selectedTeamCreGroupId}
          selectedTeamSreGroupId={selectedTeamSreGroupId}
          selectedTeamLabel={selectedTeamLabel}
          onClose={() => setEditingWidget(undefined)}
          onSave={handleSaveWidget}
          onDelete={
            editingWidget.widget
              ? () => {
                  setPendingRemoval({
                    kind: "widget",
                    widgetId: editingWidget.widget!.widgetId,
                    label: editingWidget.widget!.displayName,
                  });
                  setEditingWidget(undefined);
                }
              : undefined
          }
        />
      )}

      <Dialog open={!!pendingRemoval} onClose={() => setPendingRemoval(undefined)} maxWidth="xs" fullWidth>
        <DialogTitle>
          {pendingRemoval?.kind === "section" ? "Remove section?" : "Remove widget?"}
        </DialogTitle>
        <DialogContent>
          <Typography variant="body2" color="text.secondary">
            {pendingRemoval?.kind === "section"
              ? `This removes "${pendingRemoval.section}" and every widget in it from this draft. This only affects your local draft.`
              : pendingRemoval?.kind === "widget"
                ? `This removes "${pendingRemoval.label}" from this draft. This only affects your local draft.`
                : ""}
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setPendingRemoval(undefined)}>Cancel</Button>
          <Button color="error" variant="contained" onClick={handleConfirmRemoval}>
            Remove
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
