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
import { useSearchParams } from "react-router-dom";
import { Stack, Tab, Tabs, Typography } from "@wso2/oxygen-ui";
import { useQueryErrorResetBoundary, useSuspenseQuery } from "@tanstack/react-query";
import { cases } from "@src/services/cases";
import type { CaseType } from "@src/types";
import { ErrorBoundary } from "@components/common/ErrorBoundary";
import { CaseCard, CaseCardSkeleton } from "@components/support/CaseCard";
import { EmptyState } from "@components/support/EmptyState";
import { ErrorState } from "@components/support/ErrorState";
import { TABS, TAB_CONFIG } from "@components/support/config";

export default function SupportPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  // Derived directly from the URL (not local state) so browser back/forward navigation that
  // changes ?tab= is reflected immediately, instead of showing a stale tab.
  const tab = TABS.find((t) => t === searchParams.get("tab")) ?? "case";

  const handleTabChange = (value: CaseType) => {
    setSearchParams(
      (prev) => {
        prev.set("tab", value);
        return prev;
      },
      { replace: true },
    );
  };

  return (
    <Stack gap={2}>
      <Typography variant="h5">Support</Typography>

      <Tabs variant="scrollable" value={tab} onChange={(_, value) => handleTabChange(value)}>
        {TABS.map((t) => (
          <Tab key={t} label={TAB_CONFIG[t].title} value={t} disableRipple />
        ))}
      </Tabs>

      <CaseListErrorBoundary>
        <Suspense fallback={<CaseListSkeleton />}>
          <CaseListContent type={tab} />
        </Suspense>
      </CaseListErrorBoundary>
    </Stack>
  );
}

function CaseListContent({ type }: { type: CaseType }) {
  const { data } = useSuspenseQuery(
    cases.all({
      filters: { types: [type] },
      sortBy: { field: "updatedOn", order: "desc" },
      pagination: { limit: 20 },
    }),
  );

  if (data.items.length === 0) return <EmptyState message={TAB_CONFIG[type].emptyMessage} />;

  return (
    <Stack gap={1.5}>
      {data.items.map((item) => (
        <CaseCard key={item.id} item={item} />
      ))}
    </Stack>
  );
}

function CaseListSkeleton() {
  return (
    <Stack gap={1.5}>
      {Array.from({ length: 4 }).map((_, index) => (
        <CaseCardSkeleton key={index} />
      ))}
    </Stack>
  );
}

function CaseListErrorBoundary({ children }: { children: ReactNode }) {
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
