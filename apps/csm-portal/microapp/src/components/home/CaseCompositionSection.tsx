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

import { useMemo } from "react";
import { Grid, Stack, Typography } from "@wso2/oxygen-ui";
import { useQuery } from "@tanstack/react-query";
import { dashboard } from "@src/services/dashboard";
import { ALL_SEVERITIES, STATE_LABELS } from "@components/support/config";
import { COMPOSITION_STATES, SEVERITY_SHORT_LABELS, SEVERITY_SLICE_COLOR, STATE_SLICE_COLOR } from "./config";
import { CompositionDonut, type CompositionSlice } from "./CompositionDonut";

// Two donuts — severity composition and state composition of active cases — mirrors the webapp's
// CaseCompositionCharts (apps/csm-portal/webapp/src/features/csm-dashboard/components/CaseCompositionCharts.tsx),
// laid out side by side via the same 2-column Grid the customer-portal microapp's Home page uses
// for its own pie-chart widgets. Self-contained plain useQuery rather than Suspense, matching how
// TimeCardsPage.tsx already handles independent, non-blocking widgets in this app — one
// failed/slow widget shouldn't hold up the rest of the Home page.
export function CaseCompositionSection() {
  const { data, isLoading, isError } = useQuery(dashboard.composition());

  const severitySlices = useMemo<CompositionSlice[]>(
    () =>
      ALL_SEVERITIES.map((severity) => ({
        id: severity,
        name: SEVERITY_SHORT_LABELS[severity],
        value: data?.bySeverity[severity] ?? 0,
        color: SEVERITY_SLICE_COLOR[severity],
      })),
    [data],
  );

  const stateSlices = useMemo<CompositionSlice[]>(
    () =>
      COMPOSITION_STATES.map((state) => ({
        id: state,
        name: STATE_LABELS[state],
        value: data?.byState[state] ?? 0,
        color: STATE_SLICE_COLOR[state],
      })),
    [data],
  );

  const closedTotal = data?.closedTotal ?? 0;

  return (
    <Stack gap={1.5}>
      <Typography variant="subtitle1">Case composition</Typography>

      <Grid container spacing={1.5}>
        <Grid size={6}>
          <CompositionDonut
            title="Cases by severity"
            description="Share of active cases at each severity level, excluding closed."
            slices={severitySlices}
            total={data?.severityTotal ?? 0}
            isLoading={isLoading}
            isError={isError}
          />
        </Grid>
        <Grid size={6}>
          <CompositionDonut
            title="Cases by state"
            description="Share of active cases in each lifecycle state, excluding closed."
            slices={stateSlices}
            total={data?.stateTotal ?? 0}
            isLoading={isLoading}
            isError={isError}
          />
        </Grid>
      </Grid>

      {/* Reconciles with the donuts: both show active cases only, so the closed count is called
          out here rather than mixed into a slice — same as the webapp. */}
      {!isLoading && !isError && closedTotal > 0 && (
        <Typography variant="caption" color="text.secondary">
          Excludes {closedTotal} closed {closedTotal === 1 ? "case" : "cases"}.
        </Typography>
      )}
    </Stack>
  );
}
