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

import { useEffect, useMemo, useRef, useState } from "react";
import { Badge, Button, Alert, Skeleton, Stack, Tab, Tabs, Typography } from "@wso2/oxygen-ui";
import { SlidersHorizontal } from "@wso2/oxygen-ui-icons-react";
import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
  type UseInfiniteQueryResult,
} from "@tanstack/react-query";
import { timecards, type TimeSheetsView } from "@src/services/timecards";
import { users } from "@src/services/users";
import type { CsmTimeCard, TimeCardDecisionInput } from "@src/types";
import {
  countActiveTimecardFilters,
  EMPTY_TIMECARD_FILTERS,
  isTimecardApprover,
  type TimeCardFilters,
} from "@utils/timecard";
import { EmptyState } from "@components/support/EmptyState";
import { ErrorState } from "@components/support/ErrorState";
import { TimeSheetCard } from "@components/timecards/TimeSheetCard";
import { TimeCardReviewDialog } from "@components/timecards/TimeCardReviewDialog";
import { TimeCardFiltersSheet } from "@components/timecards/TimeCardFiltersSheet";

type TimeCardTab = "mine" | "all" | "approvals";

interface ReviewTarget {
  card: CsmTimeCard;
  decision: "approved" | "rejected";
}

export default function TimeCardsPage() {
  const { data: me } = useQuery(users.me());
  const isApprover = useMemo(() => isTimecardApprover(me?.roles ?? []), [me?.roles]);

  const [tab, setTab] = useState<TimeCardTab>("mine");
  // If a non-approver somehow lands on the approvals tab (e.g. role loads late), fall back to mine.
  const activeTab: TimeCardTab = tab === "approvals" && !isApprover ? "mine" : tab;

  const [review, setReview] = useState<ReviewTarget | null>(null);
  const [filters, setFilters] = useState<TimeCardFilters>(EMPTY_TIMECARD_FILTERS);
  const [filtersOpen, setFiltersOpen] = useState(false);
  const activeFilterCount = countActiveTimecardFilters(filters);
  const queryClient = useQueryClient();

  const decide = useMutation({
    mutationFn: (input: TimeCardDecisionInput) => timecards.decide(input),
    onSuccess: () => {
      setReview(null);
      void queryClient.invalidateQueries({ queryKey: ["timecards"] });
    },
  });

  const handleDecide = (card: CsmTimeCard, decision: "approved" | "rejected") => {
    decide.reset();
    setReview({ card, decision });
  };

  // One query for the active tab, lifted here so the filter sheet can offer the
  // work-item / engineer options derived from the loaded cards.
  const meId = me?.id ?? null;
  const meName = me?.fullName ?? "Me";
  const queryOptions =
    activeTab === "mine"
      ? timecards.mySheets(meId, meName, filters)
      : activeTab === "all"
        ? timecards.all(meId, filters)
        : timecards.approvalQueue(meId, filters);
  const query = useInfiniteQuery(queryOptions);

  const emptyMessage =
    activeTab === "approvals"
      ? "No time cards waiting on your approval."
      : activeFilterCount > 0
        ? "No time cards match these filters."
        : activeTab === "mine"
          ? "You haven't logged any time yet."
          : "No time cards yet.";

  return (
    <Stack gap={2}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" gap={1}>
        <Typography variant="h5">Time cards</Typography>
        <Badge badgeContent={activeFilterCount} color="primary">
          <Button
            size="small"
            variant="outlined"
            startIcon={<SlidersHorizontal size={16} />}
            onClick={() => setFiltersOpen(true)}
          >
            Filters
          </Button>
        </Badge>
      </Stack>

      {decide.isError && (
        <Alert severity="error" onClose={() => decide.reset()}>
          Couldn't update the time card. Please try again.
        </Alert>
      )}

      <Tabs value={activeTab} onChange={(_, value: TimeCardTab) => setTab(value)}>
        <Tab label="My sheets" value="mine" disableRipple />
        <Tab label="All" value="all" disableRipple />
        {isApprover && <Tab label="Approvals" value="approvals" disableRipple />}
      </Tabs>

      <SheetListView
        query={query}
        emptyMessage={emptyMessage}
        showEngineer={activeTab !== "mine"}
        onDecide={activeTab === "approvals" ? handleDecide : undefined}
        decidingCardId={decide.isPending ? review?.card.id : null}
      />

      <TimeCardFiltersSheet
        open={filtersOpen}
        onClose={() => setFiltersOpen(false)}
        filters={filters}
        onApply={setFilters}
        showStateFilter={activeTab !== "approvals"}
        showEngineerFilter={activeTab !== "mine"}
        workItemOptions={query.data?.availableWorkItems ?? []}
        engineerOptions={query.data?.availableEngineers ?? []}
      />

      <TimeCardReviewDialog
        open={!!review}
        card={review?.card ?? null}
        decision={review?.decision ?? "approved"}
        submitting={decide.isPending}
        onClose={() => decide.isPending || setReview(null)}
        onConfirm={(leadComment) => {
          if (!review) return;
          decide.mutate({ cardId: review.card.id, state: review.decision, leadComment });
        }}
      />
    </Stack>
  );
}

// Renders a paginated list of weekly sheets: an IntersectionObserver sentinel at
// the bottom fetches the next page as it scrolls into view (the mobile equivalent
// of the webapp's page controls), with an "X of Y" progress line and an
// end-of-list marker.
function SheetListView({
  query,
  emptyMessage,
  showEngineer = false,
  onDecide,
  decidingCardId,
}: {
  query: UseInfiniteQueryResult<TimeSheetsView, Error>;
  emptyMessage: string;
  showEngineer?: boolean;
  onDecide?: (card: CsmTimeCard, decision: "approved" | "rejected") => void;
  decidingCardId?: string | null;
}) {
  const { data, isLoading, isError, refetch, hasNextPage, isFetchingNextPage, fetchNextPage } = query;

  const sentinelRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = sentinelRef.current;
    if (!el) return;

    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry?.isIntersecting && hasNextPage && !isFetchingNextPage) {
          void fetchNextPage();
        }
      },
      { threshold: 0.1 },
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [hasNextPage, isFetchingNextPage, fetchNextPage]);

  if (isLoading) return <SheetsSkeleton />;
  if (isError) return <ErrorState onRetry={() => void refetch()} />;

  const sheets = data?.sheets ?? [];
  if (sheets.length === 0) return <EmptyState message={emptyMessage} />;

  return (
    <Stack gap={1.5}>
      <Typography variant="caption" color="text.secondary">
        {data?.loaded ?? sheets.length} of {data?.total ?? sheets.length}
      </Typography>

      {sheets.map((sheet, index) => (
        <TimeSheetCard
          key={sheet.id}
          sheet={sheet}
          showEngineer={showEngineer}
          onDecide={onDecide}
          decidingCardId={decidingCardId}
          defaultExpanded={index === 0}
        />
      ))}

      {/* IntersectionObserver can miss a zero-height target, so give the sentinel 1px to observe. */}
      <div ref={sentinelRef} style={{ height: 1 }} />

      {isFetchingNextPage && <Skeleton variant="rounded" height={140} />}
      {!hasNextPage && (
        <Typography variant="body2" color="text.secondary" textAlign="center" py={1}>
          You're all caught up!
        </Typography>
      )}
    </Stack>
  );
}

function SheetsSkeleton() {
  return (
    <Stack gap={1.5}>
      {Array.from({ length: 3 }).map((_, i) => (
        <Skeleton key={i} variant="rounded" height={140} />
      ))}
    </Stack>
  );
}
