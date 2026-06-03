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

import { Box, Button, Typography } from "@wso2/oxygen-ui";
import { Plus } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";
import EngagementsStatCards from "@features/csm-engagements/components/EngagementsStatCards";
import EngagementsFilterBar from "@features/csm-engagements/components/EngagementsFilterBar";
import EngagementsList from "@features/csm-engagements/components/EngagementsList";
import CreateEngagementDialog from "@features/csm-engagements/components/CreateEngagementDialog";
import { useGetCsmEngagements } from "@features/csm-engagements/api/useGetCsmEngagements";
import {
  isActiveState,
  stageIndex,
} from "@features/csm-engagements/utils/engagements";
import type {
  CsmEngagementListFilters,
  CsmEngagementRow,
} from "@features/csm-engagements/types/csmEngagements";

const DEFAULT_FILTERS: CsmEngagementListFilters = {
  search: "",
  types: [],
  states: [],
  stages: [],
  assignee: "all",
  health: "all",
};

type StatKey = "all" | "requested" | "in_progress" | "on_hold" | "completed" | "at_risk";

function applyFilters(
  rows: CsmEngagementRow[],
  f: CsmEngagementListFilters,
  stat: StatKey,
): CsmEngagementRow[] {
  const q = f.search.trim().toLowerCase();
  return rows.filter((e) => {
    if (f.types.length && !f.types.includes(e.type)) return false;
    if (f.states.length && !f.states.includes(e.state)) return false;
    if (f.stages.length && !f.stages.includes(e.stage)) return false;
    if (f.assignee === "me" && !e.assigneeIsMe) return false;
    if (f.assignee === "unassigned" && e.assigneeName !== "Unassigned") return false;
    if (f.health !== "all" && e.health !== f.health) return false;
    if (q) {
      const hay = `${e.reference} ${e.name} ${e.customer} ${e.projectName ?? ""} ${e.assigneeName}`.toLowerCase();
      if (!hay.includes(q)) return false;
    }
    // Stat card secondary filter
    if (stat === "requested" && e.state !== "requested") return false;
    if (stat === "in_progress" && e.state !== "in_progress") return false;
    if (stat === "on_hold" && e.state !== "on_hold") return false;
    if (stat === "completed" && e.state !== "completed") return false;
    if (stat === "at_risk") {
      if (!isActiveState(e.state)) return false;
      if (e.health === "green" || !e.health) return false;
    }
    return true;
  });
}

function sortByUrgency(a: CsmEngagementRow, b: CsmEngagementRow): number {
  // Cancelled/completed go last
  const aClosed = a.state === "completed" || a.state === "cancelled" ? 1 : 0;
  const bClosed = b.state === "completed" || b.state === "cancelled" ? 1 : 0;
  if (aClosed !== bClosed) return aClosed - bClosed;
  // Red health first, amber next
  const healthRank = (h: CsmEngagementRow["health"]): number =>
    h === "red" ? 0 : h === "amber" ? 1 : 2;
  const dh = healthRank(a.health) - healthRank(b.health);
  if (dh !== 0) return dh;
  // Earlier-stage means more work remaining
  const ds = stageIndex(a.stage) - stageIndex(b.stage);
  if (ds !== 0) return ds;
  // Newer updates last (more recent activity to the top)
  return new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime();
}

export default function CsmEngagementsPage(): JSX.Element {
  const { data, isLoading, isError } = useGetCsmEngagements();
  const { showError } = useErrorBanner();
  const { showSuccess } = useSuccessBanner();
  const [filters, setFilters] = useState<CsmEngagementListFilters>(DEFAULT_FILTERS);
  const [stat, setStat] = useState<StatKey>("all");
  const [createOpen, setCreateOpen] = useState(false);

  if (isError) {
    showError("Could not load engagements.");
  }

  const all = data?.engagements ?? [];
  const filtered = useMemo(() => {
    return applyFilters(all, filters, stat).slice().sort(sortByUrgency);
  }, [all, filters, stat]);

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
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
          <Typography variant="h5">Engagements</Typography>
          <Typography variant="body2" color="text.secondary">
            {isLoading
              ? "Loading…"
              : `${filtered.length} of ${all.length} shown`}
          </Typography>
        </Box>
        <Button
          variant="contained"
          color="primary"
          size="small"
          startIcon={<Plus size={16} />}
          onClick={() => setCreateOpen(true)}
        >
          Create engagement
        </Button>
      </Box>

      <EngagementsStatCards
        engagements={all}
        selected={stat}
        onSelect={(k) => setStat(k)}
      />

      <EngagementsFilterBar
        filters={filters}
        onChange={setFilters}
        onReset={() => {
          setFilters(DEFAULT_FILTERS);
          setStat("all");
        }}
      />

      <EngagementsList engagements={filtered} isLoading={isLoading} />

      <CreateEngagementDialog
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        onCreated={(detail) => {
          setCreateOpen(false);
          showSuccess(`Engagement ${detail.reference} created.`);
        }}
      />
    </Box>
  );
}
