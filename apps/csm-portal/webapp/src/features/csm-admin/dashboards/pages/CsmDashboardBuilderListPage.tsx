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
  Card,
  CardActionArea,
  Chip,
  IconButton,
  Skeleton,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { LayoutGrid, Plus, Trash2 } from "@wso2/oxygen-ui-icons-react";
import { useMemo, type JSX } from "react";
import { useNavigate } from "react-router";
import { useDashboardList } from "@features/csm-dashboard/api/useDashboardList";
import { useDashboard } from "@features/csm-dashboard/api/useDashboard";
import { isDraftDrifted } from "@features/csm-admin/dashboards/utils/dashboardDrift";
import {
  deleteDashboardDraft,
  newDraftId,
  useDashboardDraft,
  useDashboardDrafts,
} from "@features/csm-admin/dashboards/utils/dashboardDraftsStorage";

/**
 * Per-row drift indicator for a deployed dashboard that also has a local
 * draft. `CsmDashboardBuilderListPage` only knows draft EXISTENCE
 * (`draftIds`, from the cheap `useDashboardDrafts()` list) — this component
 * fetches that one dashboard's own live detail (only for rows that actually
 * have a draft, gated by the caller) and reuses the same `isDraftDrifted`
 * comparison the editor page's own banner uses, so the badge only appears
 * when the draft ACTUALLY differs from what's deployed, not merely because a
 * draft record exists (e.g. one saved byte-identical to the live dashboard,
 * or re-saved after "open, look, do nothing").
 */
function LocalDraftDriftChip({ dashboardId }: { dashboardId: string }): JSX.Element | null {
  const draft = useDashboardDraft(dashboardId);
  const live = useDashboard(dashboardId);
  if (!draft || live.isLoading || live.isError) return null;
  if (!isDraftDrifted(draft, live.data ?? undefined)) return null;
  return (
    <Tooltip title="A local draft for this dashboard has unsaved-to-deployment changes">
      <Chip size="small" color="warning" label="Local draft" />
    </Tooltip>
  );
}

/**
 * Admin-only landing page for the dashboard builder: every deployed
 * dashboard (`GET /dashboards`), each openable for edit, plus any local
 * draft that hasn't (yet) been opened from — or matched to — a deployed
 * one. There is no dashboard CRUD API; "Edit" always opens the builder
 * against a `localStorage` draft, seeded from the live dashboard the first
 * time it's opened (see `CsmDashboardBuilderEditorPage`).
 */
export default function CsmDashboardBuilderListPage(): JSX.Element {
  const navigate = useNavigate();
  const { data: dashboards, isLoading, isError } = useDashboardList();
  const drafts = useDashboardDrafts();

  const liveIds = useMemo(() => new Set((dashboards ?? []).map((d) => d.id)), [dashboards]);
  const draftIds = useMemo(() => new Set(drafts.map((d) => d.id)), [drafts]);
  // Drafts that don't (yet) correspond to any deployed dashboard — either a
  // brand-new dashboard that's never been deployed, or a draft whose
  // deployed source has since been removed from the registry.
  const orphanDrafts = useMemo(() => drafts.filter((d) => !liveIds.has(d.id)), [drafts, liveIds]);

  const handleCreate = (): void => {
    navigate(`/admin/dashboards/${newDraftId()}`);
  };

  const handleDiscardDraft = (id: string): void => {
    deleteDashboardDraft(id);
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
        <Typography variant="body2" color="text.secondary">
          Build or adjust a dashboard's widgets, then hand the exported JSON to a maintainer to
          deploy — this builder never writes to the live registry itself.
        </Typography>
        <Box sx={{ display: "flex", gap: 1, flexWrap: "wrap" }}>
          <Button variant="contained" startIcon={<Plus size={16} />} onClick={handleCreate}>
            Create new dashboard
          </Button>
          {/* Presets and sections are shared ACROSS dashboards and deploy as
              their own files, so they get their own page rather than living
              inside one dashboard's editor. */}
          <Button
            variant="outlined"
            startIcon={<LayoutGrid size={16} />}
            onClick={() => navigate("/admin/dashboards/shared")}
          >
            Shared presets &amp; sections
          </Button>
        </Box>
      </Box>

      {isError ? (
        <Typography variant="body2" color="text.secondary">
          Could not load the dashboard list.
        </Typography>
      ) : isLoading ? (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
          {Array.from({ length: 3 }, (_, i) => (
            <Skeleton key={i} variant="rounded" height={64} />
          ))}
        </Box>
      ) : (dashboards ?? []).length === 0 ? (
        <Typography variant="body2" color="text.secondary">
          No dashboards are registered in this deployment yet.
        </Typography>
      ) : (
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: "repeat(auto-fill, minmax(320px, 1fr))",
            gap: 2,
          }}
        >
          {(dashboards ?? []).map((d) => (
            <Card key={d.id} variant="outlined">
              <CardActionArea
                onClick={() => navigate(`/admin/dashboards/${d.id}`)}
                sx={{
                  p: 3,
                  display: "flex",
                  flexDirection: "column",
                  alignItems: "flex-start",
                  gap: 1.5,
                  minHeight: 120,
                }}
              >
                <LayoutGrid size={24} />
                <Box sx={{ minWidth: 0, width: "100%" }}>
                  <Typography variant="subtitle1" sx={{ fontWeight: 600 }} noWrap>
                    {d.displayName}
                  </Typography>
                  <Typography variant="caption" color="text.secondary" noWrap>
                    {d.id}
                  </Typography>
                </Box>
                <Box sx={{ display: "flex", flexWrap: "wrap", gap: 0.5 }}>
                  {d.isDefault && <Chip size="small" label="Default" />}
                  {d.isTeamBased && <Chip size="small" label="Team-based" variant="outlined" />}
                  {d.type && <Chip size="small" label={d.type.toUpperCase()} variant="outlined" />}
                  {draftIds.has(d.id) && <LocalDraftDriftChip dashboardId={d.id} />}
                </Box>
              </CardActionArea>
            </Card>
          ))}
        </Box>
      )}

      {orphanDrafts.length > 0 && (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
          <Typography variant="subtitle2">
            Local drafts not yet deployed
          </Typography>
          <Box
            sx={{
              display: "grid",
              gridTemplateColumns: "repeat(auto-fill, minmax(320px, 1fr))",
              gap: 2,
            }}
          >
            {orphanDrafts.map((d) => (
              <Card key={d.id} variant="outlined" sx={{ position: "relative" }}>
                <Tooltip title="Discard this local draft">
                  <IconButton
                    size="small"
                    aria-label={`Discard draft ${d.displayName || d.id}`}
                    onClick={() => handleDiscardDraft(d.id)}
                    sx={{ position: "absolute", top: 4, right: 4, zIndex: 1 }}
                  >
                    <Trash2 size={16} />
                  </IconButton>
                </Tooltip>
                <CardActionArea
                  onClick={() => navigate(`/admin/dashboards/${d.id}`)}
                  sx={{
                    p: 3,
                    display: "flex",
                    flexDirection: "column",
                    alignItems: "flex-start",
                    gap: 1.5,
                    minHeight: 120,
                  }}
                >
                  <LayoutGrid size={24} />
                  <Box sx={{ minWidth: 0, width: "100%", pr: 3 }}>
                    <Typography variant="subtitle1" sx={{ fontWeight: 600 }} noWrap>
                      {d.displayName || "Untitled dashboard"}
                    </Typography>
                    <Typography variant="caption" color="text.secondary">
                      Saved locally {new Date(d.updatedAt).toLocaleString()}
                    </Typography>
                  </Box>
                </CardActionArea>
              </Card>
            ))}
          </Box>
        </Box>
      )}
    </Box>
  );
}
