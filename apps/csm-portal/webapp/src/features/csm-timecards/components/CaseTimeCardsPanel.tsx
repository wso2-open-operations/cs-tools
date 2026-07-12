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

import { useMemo, useState, type JSX } from "react";
import {
  Box,
  Button,
  Card,
  Divider,
  Skeleton,
  Typography,
} from "@wso2/oxygen-ui";
import { Clock, Plus } from "@wso2/oxygen-ui-icons-react";
import RelativeTime from "@components/RelativeTime";
import { useCaseTimeCards, useDecideTimeCard } from "@features/csm-timecards/api/useTimeCards";
import { useCurrentEngineer } from "@features/csm-timecards/api/useTimeSheets";
import { useIsTeamLead } from "@features/csm-timecards/hooks/useIsTeamLead";
import { billableLabel } from "@features/csm-timecards/constants/timeCardConstants";
import { BackendApiError } from "@api/backend/client";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import TimeCardStatusChip from "@features/csm-timecards/components/TimeCardStatusChip";
import TimeCardReviewDialog from "@features/csm-timecards/components/TimeCardReviewDialog";
import TimeCardTruncatedNotice from "@features/csm-timecards/components/TimeCardTruncatedNotice";
import type { CsmTimeCard } from "@features/csm-timecards/types/timeCards";

interface CaseTimeCardsPanelProps {
  caseId: string;
  /** The case's project — required to scope `/time-cards/search` (see useCaseTimeCards). */
  projectId: string;
  /** Opens the log-time dialog (owned by the page so the action bar can trigger it). */
  onLogTime: () => void;
}

/**
 * The body of a case's "Time tracking" tab: the time cards logged on this
 * case, with a running total and per-entry status. A team lead can review
 * (accept or reject) any submitted entry inline. Available even after the
 * case is closed — time is often logged after the fact.
 */
export default function CaseTimeCardsPanel({
  caseId,
  projectId,
  onLogTime,
}: CaseTimeCardsPanelProps): JSX.Element {
  const { data, isLoading, isError } = useCaseTimeCards(caseId, projectId);
  const isTeamLead = useIsTeamLead();
  const me = useCurrentEngineer();
  const decide = useDecideTimeCard();
  const { showError } = useErrorBanner();
  const [reviewCard, setReviewCard] = useState<CsmTimeCard | null>(null);

  const cards = useMemo(() => data?.cards ?? [], [data]);
  const total = useMemo(
    () => cards.reduce((s, c) => s + c.totalMinutes, 0),
    [cards],
  );

  return (
    <Card variant="outlined" sx={{ p: 2 }}>
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 1,
          mb: 1.25,
        }}
      >
        <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
          <Clock size={16} />
          <Typography variant="subtitle2">Time tracked</Typography>
        </Box>
        <Button
          size="small"
          variant="text"
          startIcon={<Plus size={14} />}
          onClick={onLogTime}
        >
          Log time
        </Button>
      </Box>

      <Box
        sx={{
          display: "flex",
          alignItems: "baseline",
          justifyContent: "space-between",
          mb: 1,
        }}
      >
        <Typography variant="h6" sx={{ lineHeight: 1 }}>
          {total} min
        </Typography>
        <Typography variant="caption" color="text.secondary">
          Across {cards.length} {cards.length === 1 ? "entry" : "entries"}
        </Typography>
      </Box>

      <Divider sx={{ mb: 1 }} />

      {!isError && data?.truncated && (
        <TimeCardTruncatedNotice hint="Some entries on this case may not be shown." />
      )}

      {isLoading ? (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
          {[0, 1, 2].map((i) => (
            <Box
              key={i}
              sx={{ p: 1, borderRadius: 1, border: 1, borderColor: "divider" }}
            >
              <Box sx={{ display: "flex", justifyContent: "space-between", mb: 0.5 }}>
                <Skeleton variant="rounded" width="40%" height={20} />
                <Skeleton variant="rounded" width="20%" height={20} />
              </Box>
              <Skeleton variant="rounded" width="25%" height={16} />
            </Box>
          ))}
        </Box>
      ) : isError ? (
        <Typography variant="body2" color="error" sx={{ py: 2 }}>
          Could not load time cards.
        </Typography>
      ) : cards.length === 0 ? (
        <Typography variant="body2" color="text.secondary" sx={{ py: 2 }}>
          No time logged on this case yet.
        </Typography>
      ) : (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
          {cards.map((c) => (
            <Box
              key={c.id}
              sx={{
                display: "flex",
                flexDirection: "column",
                gap: 0.5,
                p: 1,
                borderRadius: 1,
                border: 1,
                borderColor: "divider",
              }}
            >
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "space-between",
                  gap: 1,
                }}
              >
                <Typography variant="body2">{c.userName}</Typography>
                <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                  <Typography variant="body2">{c.totalMinutes} min</Typography>
                  <TimeCardStatusChip state={c.state} />
                  {/* Never shown on your own card: the backend 403s a
                   self-decide regardless of approver status, so a card you
                   submitted yourself can never actually be reviewed by you. */}
                  {isTeamLead &&
                    c.state === "submitted" &&
                    !!me.id &&
                    c.userId !== me.id && (
                      <Button
                        size="small"
                        variant="outlined"
                        onClick={() => setReviewCard(c)}
                      >
                        Review
                      </Button>
                    )}
                </Box>
              </Box>
              <Typography variant="caption" color="text.secondary">
                {billableLabel(c.billable)} · <RelativeTime iso={c.createdOn} />
                {c.approvedByName && ` · Decided by ${c.approvedByName}`}
              </Typography>
            </Box>
          ))}
        </Box>
      )}

      {reviewCard && (
        <TimeCardReviewDialog
          card={reviewCard}
          isDeciding={decide.isPending}
          onClose={() => setReviewCard(null)}
          onDecide={(decision) =>
            decide.mutate(decision, {
              onSuccess: () => setReviewCard(null),
              onError: (err) => {
                // The backend 403s when the signed-in user isn't authorized
                // to decide this specific card — surface its own message
                // rather than failing silently (see CsmTimeCardsPage.tsx for
                // the same pattern and the confirmed-live evidence).
                const msg =
                  err instanceof BackendApiError && err.status < 500 && err.message
                    ? err.message
                    : "Could not submit your decision. Please try again.";
                showError(msg, err);
              },
            })
          }
        />
      )}
    </Card>
  );
}
