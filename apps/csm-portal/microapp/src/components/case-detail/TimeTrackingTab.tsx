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

import { useState } from "react";
import { Box, Button, Divider, Skeleton, Stack, Typography } from "@wso2/oxygen-ui";
import { Plus } from "@wso2/oxygen-ui-icons-react";
import { useNavigate } from "react-router-dom";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { timecards } from "@src/services/timecards";
import type { CreateTimeCardInput, CsmTimeCard } from "@src/types";
import { cardDateLabel, formatMinutes } from "@utils/timecard";
import { TimeCardStateChip } from "@components/timecards/TimeCardStateChip";
import { LogTimeCardDialog } from "./LogTimeCardDialog";

interface TimeTrackingTabProps {
  caseId: string;
  caseNumber: string;
  projectId: string;
  projectName: string;
}

function TimeCardRow({ card }: { card: CsmTimeCard }) {
  return (
    <Stack direction="row" alignItems="center" justifyContent="space-between" gap={1}>
      <Typography variant="body2" noWrap sx={{ minWidth: 0 }}>
        {card.userName}
      </Typography>
      <Stack alignItems="flex-end" gap={0.5} flexShrink={0}>
        <TimeCardStateChip state={card.state} />
        <Typography variant="caption" color="text.secondary">
          {cardDateLabel(card.createdOn)} · {formatMinutes(card.totalMinutes)}
          {card.billable ? "" : " · Non-billable"}
        </Typography>
      </Stack>
    </Stack>
  );
}

/**
 * `caseId` is confirmed non-functional live as a `/time-cards/search` filter (see
 * services/timecards.ts's fetchCaseTimeCards), so this list is scoped to the case's
 * project server-side and filtered to the case client-side, same workaround the
 * webapp's CaseTimeCardsPanel uses. Only the first page of the project's cards is
 * fetched — a single case logging more than a page's worth of time isn't expected —
 * so a truncated notice covers the rare case where it does.
 */
export function TimeTrackingTab({ caseId, caseNumber, projectId, projectName }: TimeTrackingTabProps) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [logTimeOpen, setLogTimeOpen] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const { data, isLoading, isError } = useQuery(timecards.forCase(caseId, projectId));
  const cards = data?.cards ?? [];
  const total = cards.reduce((sum, c) => sum + c.totalMinutes, 0);

  const handleSubmit = (input: CreateTimeCardInput) => {
    setIsSubmitting(true);
    setError(null);
    timecards
      .create(input)
      .then(() => {
        setLogTimeOpen(false);
        // Root key — one invalidate refreshes every time-cards view (My sheets,
        // All, Approvals, and this case's own list) after this write, per
        // services/timecards.ts's convention.
        void queryClient.invalidateQueries({ queryKey: ["timecards"] });
      })
      .catch(() => setError("Could not log time. Please try again."))
      .finally(() => setIsSubmitting(false));
  };

  return (
    <Stack gap={1.5}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" gap={1}>
        <Box>
          <Typography variant="h6" sx={{ lineHeight: 1 }}>
            {formatMinutes(total)}
          </Typography>
          <Typography variant="caption" color="text.secondary">
            Across {cards.length} {cards.length === 1 ? "entry" : "entries"}
          </Typography>
        </Box>
        <Button variant="contained" size="small" startIcon={<Plus size={14} />} onClick={() => setLogTimeOpen(true)}>
          Log time
        </Button>
      </Stack>

      <Divider />

      {!isError && data?.truncated && (
        <Typography variant="caption" color="text.secondary">
          Some entries on this case may not be shown.
        </Typography>
      )}

      {isLoading ? (
        <Stack gap={1}>
          {[0, 1].map((i) => (
            <Skeleton key={i} variant="rounded" height={48} />
          ))}
        </Stack>
      ) : isError ? (
        <Typography variant="body2" color="error">
          Could not load time cards.
        </Typography>
      ) : cards.length === 0 ? (
        <Typography variant="body2" color="text.secondary">
          No time logged on this case yet.
        </Typography>
      ) : (
        <Stack gap={1.5}>
          {cards.map((c) => (
            <TimeCardRow key={c.id} card={c} />
          ))}
        </Stack>
      )}

      <Button variant="outlined" size="small" onClick={() => navigate("/more/time-cards")} sx={{ alignSelf: "start" }}>
        Open Time Cards
      </Button>

      {logTimeOpen && (
        <LogTimeCardDialog
          caseId={caseId}
          caseNumber={caseNumber}
          projectId={projectId}
          projectName={projectName}
          isSubmitting={isSubmitting}
          error={error}
          onClose={() => {
            if (!isSubmitting) {
              setLogTimeOpen(false);
              setError(null);
            }
          }}
          onSubmit={handleSubmit}
        />
      )}
    </Stack>
  );
}
