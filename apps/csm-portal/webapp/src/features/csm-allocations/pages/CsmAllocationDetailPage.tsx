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

import { Box, Button, Paper, Skeleton, Typography } from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import type { JSX, ReactNode } from "react";
import { Link as RouterLink, useLocation, useParams } from "react-router";
import QueryErrorState from "@components/QueryErrorState";
import { useNavTransition } from "@hooks/useNavTransition";
import { formatAbsoluteForUser, formatDateOnlyForDisplay } from "@utils/dateTime";
import { useAllocation } from "../api/useAllocations";
import AllocationStateChip from "../components/AllocationStateChip";
import AllocationUpdatesSection from "../components/AllocationUpdatesSection";
import { formatUtilization, humanizeEnum } from "../utils/allocations";
import type { Allocation } from "../types";

function Field({ label, children }: { label: string; children: ReactNode }): JSX.Element {
  return (
    <Box>
      <Typography variant="caption" color="text.secondary" component="dt">
        {label}
      </Typography>
      <Typography variant="body2" component="dd" sx={{ m: 0, overflowWrap: "anywhere" }}>
        {children === undefined || children === null || children === "" ? "—" : children}
      </Typography>
    </Box>
  );
}

function FieldGrid({ children, label }: { children: ReactNode; label: string }): JSX.Element {
  return (
    <Box
      component="dl"
      aria-label={label}
      sx={{
        m: 0,
        display: "grid",
        gap: 2,
        gridTemplateColumns: { xs: "1fr", sm: "repeat(2, 1fr)", md: "repeat(3, 1fr)" },
      }}
    >
      {children}
    </Box>
  );
}

const date = (v?: string): string | undefined => formatDateOnlyForDisplay(v) ?? undefined;

function AllocationDetails({ a }: { a: Allocation }): JSX.Element {
  const e = a.engagement;
  const time = a.startTime || a.endTime ? `${a.startTime ?? "—"} - ${a.endTime ?? "—"}` : undefined;
  return (
    <>
      <Paper variant="outlined" component="section" aria-labelledby="alloc-section" sx={{ p: 2 }}>
        <Typography id="alloc-section" variant="subtitle1" component="h2" sx={{ mb: 2 }}>
          Allocation
        </Typography>
        <FieldGrid label="Allocation details">
          <Field label="Reference">{a.allocationId}</Field>
          <Field label="State">{a.state ? <AllocationStateChip state={a.state} /> : undefined}</Field>
          <Field label="Utilization">{formatUtilization(a.utilization)}</Field>
          <Field label="Start date">{date(a.startDate)}</Field>
          <Field label="End date">{date(a.endDate)}</Field>
          <Field label="Time of day">{time}</Field>
          <Field label="Timezone">{a.timezone}</Field>
          <Field label="Engineer">{a.engineer?.name}</Field>
          <Field label="Engineer email">{a.engineer?.email}</Field>
          <Field label="Customer">
            {a.account ? (
              <RouterLink to={`/customers/accounts/${a.account.id}`}>{a.account.name}</RouterLink>
            ) : undefined}
          </Field>
          <Field label="Latest update">{formatAbsoluteForUser(a.latestUpdateDate) ?? undefined}</Field>
        </FieldGrid>
      </Paper>

      <Paper variant="outlined" component="section" aria-labelledby="engagement-section" sx={{ p: 2 }}>
        <Typography id="engagement-section" variant="subtitle1" component="h2" sx={{ mb: 2 }}>
          Engagement
        </Typography>
        <FieldGrid label="Engagement details">
          <Field label="Name">{e.name}</Field>
          <Field label="Engagement ID">{e.displayId}</Field>
          <Field label="State">{humanizeEnum(e.state)}</Field>
          <Field label="Type">{humanizeEnum(e.type)}</Field>
          <Field label="Mode of delivery">{humanizeEnum(e.modeOfDelivery)}</Field>
          <Field label="Business unit">{e.businessUnit}</Field>
          <Field label="Paid">{e.isPaid === undefined ? undefined : e.isPaid ? "Yes" : "No"}</Field>
          <Field label="Project">{e.projectId}</Field>
          <Field label="Planned start">{date(e.plannedStartDate)}</Field>
          <Field label="Planned end">{date(e.plannedEndDate)}</Field>
          <Field label="Actual start">{date(e.actualStartDate)}</Field>
          <Field label="Actual end">{date(e.actualEndDate)}</Field>
        </FieldGrid>
      </Paper>
    </>
  );
}

export default function CsmAllocationDetailPage(): JSX.Element {
  const { allocationId = "" } = useParams<{ allocationId: string }>();
  const navigate = useNavTransition();
  const location = useLocation();
  const backTo = (location.state as { from?: string } | null)?.from ?? "/allocations";
  const q = useAllocation(allocationId);

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Button
        variant="text"
        size="small"
        startIcon={<ArrowLeft size={16} />}
        onClick={() => navigate(backTo)}
        sx={{ alignSelf: "flex-start" }}
      >
        Back to allocations
      </Button>

      {q.isLoading ? (
        <Box aria-busy="true" sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <Skeleton variant="rounded" height={32} width="40%" />
          <Skeleton variant="rounded" height={160} />
          <Skeleton variant="rounded" height={160} />
        </Box>
      ) : q.isError ? (
        <QueryErrorState
          message={q.error.message.trim() || "Failed to load the allocation."}
          error={q.error}
          onRetry={() => void q.refetch()}
        />
      ) : !q.data ? (
        <Typography variant="body1">Allocation not found.</Typography>
      ) : (
        <>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
            <Typography variant="h5" component="h1">
              {q.data.allocationId ?? "Allocation"}
              {q.data.engagement.name ? ` · ${q.data.engagement.name}` : ""}
            </Typography>
            {q.data.state && <AllocationStateChip state={q.data.state} />}
          </Box>
          <AllocationDetails a={q.data} />
          <AllocationUpdatesSection allocationId={q.data.id} />
        </>
      )}
    </Box>
  );
}
