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

import { Suspense, type ReactNode } from "react";
import { Chip, LinearProgress, Skeleton, Stack, Typography } from "@wso2/oxygen-ui";
import { useQueryErrorResetBoundary, useSuspenseQuery } from "@tanstack/react-query";
import { slas } from "@src/services/slas";
import type { CaseSla } from "@src/types";
import { ErrorBoundary } from "@components/common/ErrorBoundary";
import { ErrorState } from "@components/support/ErrorState";

const STAGE_LABEL: Record<string, string> = {
  in_progress: "In progress",
  paused: "Paused",
  completed: "Completed",
  cancelled: "Cancelled",
  breached: "Breached",
};

/** ServiceNow data source only — the search returns an empty list (not an error) for cases
 * sourced elsewhere, so an empty tab doesn't necessarily mean "no SLAs apply". */
export function SlaTab({ caseId }: { caseId: string }) {
  return (
    <SlaTabErrorBoundary>
      <Suspense fallback={<SlaTabSkeleton />}>
        <SlaTabContent caseId={caseId} />
      </Suspense>
    </SlaTabErrorBoundary>
  );
}

function SlaTabContent({ caseId }: { caseId: string }) {
  const { data: caseSlas } = useSuspenseQuery(slas.forCase(caseId));

  if (caseSlas.length === 0) {
    return (
      <Typography variant="body2" color="text.secondary">
        No SLAs found for this case.
      </Typography>
    );
  }

  return (
    <Stack gap={1.5}>
      {caseSlas.map((sla) => (
        <SlaRow key={sla.id} sla={sla} />
      ))}
    </Stack>
  );
}

function SlaRow({ sla }: { sla: CaseSla }) {
  const percent = Math.min(100, Math.max(0, sla.businessElapsedPercent ?? 0));
  const color = sla.hasBreached || percent >= 100 ? "error" : "primary";

  return (
    <Stack gap={0.75} sx={{ p: 1.5, border: "1px solid", borderColor: "divider", borderRadius: 1 }}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" gap={1}>
        <Typography variant="body2" fontWeight={500} noWrap sx={{ minWidth: 0 }}>
          {sla.definitionName}
        </Typography>
        {sla.stage && (
          <Chip
            size="small"
            color={sla.hasBreached ? "error" : "default"}
            variant={sla.hasBreached ? "filled" : "outlined"}
            label={STAGE_LABEL[sla.stage] ?? sla.stage}
          />
        )}
      </Stack>

      {sla.businessElapsedPercent != null && (
        <LinearProgress variant="determinate" value={percent} color={color} sx={{ borderRadius: 1, height: 6 }} />
      )}

      <Stack direction="row" justifyContent="space-between" gap={2}>
        {sla.businessTimeLeft && (
          <Typography variant="caption" color="text.secondary">
            {sla.businessTimeLeft} left
          </Typography>
        )}
        {sla.businessElapsedTime && (
          <Typography variant="caption" color="text.secondary">
            {sla.businessElapsedTime} elapsed
          </Typography>
        )}
      </Stack>
    </Stack>
  );
}

function SlaTabSkeleton() {
  return (
    <Stack gap={1.5}>
      <Skeleton variant="rounded" height={72} />
      <Skeleton variant="rounded" height={72} />
    </Stack>
  );
}

function SlaTabErrorBoundary({ children }: { children: ReactNode }) {
  const { reset } = useQueryErrorResetBoundary();

  return (
    <ErrorBoundary
      fallback={(_error, resetErrorBoundary) => (
        <ErrorState
          onRetry={() => {
            reset();
            resetErrorBoundary();
          }}
        />
      )}
    >
      {children}
    </ErrorBoundary>
  );
}
