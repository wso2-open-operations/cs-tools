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
import { useNavigate } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { dashboard } from "@src/services/dashboard";
import type { CaseSeverity, CaseState } from "@src/types";
import { ALL_SEVERITIES, STATE_LABELS } from "@components/support/config";
import { EMPTY_FILTERS, filtersToSearchParams } from "@components/support/filters";
import { COMPOSITION_STATES, SEVERITY_SHORT_LABELS, SEVERITY_SLICE_COLOR, STATE_SLICE_COLOR } from "./config";
import { CompositionDonut, type CompositionSlice } from "./CompositionDonut";

function supportHref(overrides: Partial<{ severities: CaseSeverity[]; states: CaseState[] }>): string {
  const params = filtersToSearchParams("", { ...EMPTY_FILTERS, ...overrides });
  const qs = params.toString();
  return qs ? `/support?${qs}` : "/support";
}

export function CaseCompositionSection() {
  const navigate = useNavigate();
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
            slices={severitySlices}
            total={data?.severityTotal ?? 0}
            isLoading={isLoading}
            isError={isError}
            onSliceClick={(id) => navigate(supportHref({ severities: [id as CaseSeverity] }))}
          />
        </Grid>
        <Grid size={6}>
          <CompositionDonut
            title="Cases by state"
            slices={stateSlices}
            total={data?.stateTotal ?? 0}
            onSliceClick={(id) => navigate(supportHref({ states: [id as CaseState] }))}
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
