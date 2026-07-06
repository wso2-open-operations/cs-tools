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

import { Box, Typography } from "@wso2/oxygen-ui";
import { useMemo, type JSX } from "react";
import { useNavigate } from "react-router";
import { casesHref } from "@features/csm-cases/utils/casesFiltersUrl";
import {
  COMPOSITION_STATES,
  useCaseComposition,
} from "@features/csm-dashboard/api/useCaseComposition";
import { MATRIX_SEVERITIES } from "@features/csm-dashboard/api/useCaseCountsMatrix";
import CompositionDonut, {
  type CompositionSlice,
} from "@features/csm-dashboard/components/CompositionDonut";
import { paletteColor } from "@features/csm-dashboard/utils/paletteColor";
import {
  SEVERITY_LABEL,
  STATE_LABEL,
} from "@features/csm-dashboard/utils/abtDashboard";
import type {
  CaseState,
  Severity,
} from "@features/csm-dashboard/types/abtDashboard";

// Distinct hue per slice (the SEVERITY_COLOR/STATE_COLOR chip palettes reuse
// the same role colour for several keys, which would merge pie slices).
const SEVERITY_SLICE_COLOR: Record<Severity, string> = {
  S0: paletteColor("red", 600, "#dc2626"),
  S1: paletteColor("deepOrange", 500, "#f4511e"),
  S2: paletteColor("amber", 700, "#b45309"),
  S3: paletteColor("blue", 500, "#3b82f6"),
  S4: paletteColor("green", 500, "#22c55e"),
};

const STATE_SLICE_COLOR: Record<CaseState, string> = {
  open: paletteColor("blue", 500, "#3b82f6"),
  work_in_progress: paletteColor("indigo", 400, "#6366f1"),
  waiting_on_wso2: paletteColor("amber", 700, "#b45309"),
  awaiting_info: paletteColor("cyan", 500, "#06b6d4"),
  solution_proposed: paletteColor("teal", 500, "#14b8a6"),
  closed: paletteColor("grey", 500, "#6b7280"),
};

/**
 * Two donuts on the dashboard: severity composition of all cases, and state
 * composition of all cases (regardless of severity). Both read from a single
 * {@link useCaseComposition} query.
 */
export default function CaseCompositionCharts(): JSX.Element {
  const { data, isLoading, isError } = useCaseComposition();
  const navigate = useNavigate();

  const severitySlices = useMemo<CompositionSlice[]>(
    () =>
      MATRIX_SEVERITIES.map((sev) => ({
        id: sev,
        name: `${sev} · ${SEVERITY_LABEL[sev]}`,
        value: data?.bySeverity[sev] ?? 0,
        color: SEVERITY_SLICE_COLOR[sev],
      })),
    [data],
  );

  const stateSlices = useMemo<CompositionSlice[]>(
    () =>
      COMPOSITION_STATES.map((st) => ({
        id: st,
        name: STATE_LABEL[st],
        value: data?.byState[st] ?? 0,
        color: STATE_SLICE_COLOR[st],
      })),
    [data],
  );

  const closedTotal = data?.closedTotal ?? 0;

  return (
    <Box>
      <Box
        sx={{
          display: "grid",
          gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
          gap: 3,
        }}
      >
        <CompositionDonut
          title="Cases by severity"
          description="Share of active cases at each severity level (S0–S4), excluding closed."
          slices={severitySlices}
          total={data?.severityTotal ?? 0}
          isLoading={isLoading}
          isError={isError}
          onSliceClick={(id) =>
            navigate(casesHref({ severities: [id as Severity] }))
          }
        />
        <CompositionDonut
          title="Cases by state"
          description="Share of active cases in each lifecycle state, excluding closed."
          slices={stateSlices}
          total={data?.stateTotal ?? 0}
          isLoading={isLoading}
          isError={isError}
          onSliceClick={(id) =>
            navigate(casesHref({ states: [id as CaseState] }))
          }
        />
      </Box>
      {/* Reconciles the pies with the matrix above: both show active cases only,
          so the closed count is called out here rather than mixed into a slice. */}
      {!isLoading && !isError && closedTotal > 0 && (
        <Typography
          variant="caption"
          color="text.secondary"
          sx={{ display: "block", mt: 1.5 }}
        >
          These charts and the matrix above show active cases only. Excludes{" "}
          {closedTotal} closed {closedTotal === 1 ? "case" : "cases"}.
        </Typography>
      )}
    </Box>
  );
}
