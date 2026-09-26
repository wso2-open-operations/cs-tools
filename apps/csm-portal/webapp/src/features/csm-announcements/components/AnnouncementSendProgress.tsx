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

import { Box, Card, Chip, LinearProgress, Typography } from "@wso2/oxygen-ui";
import { CheckCircle, Megaphone, XCircle } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";

export interface AnnouncementSendProgressState {
  total: number;
  completed: number;
  succeeded: number;
  failed: number;
  /**
   * The project id of every case-create call that has rejected so far, in
   * the order they failed. This is the part that actually matters to the
   * sender: a bare failed *count* doesn't tell them which customer projects
   * didn't get the announcement and need a manual retry — only the ids do.
   */
  failedProjectIds: string[];
}

interface AnnouncementSendProgressProps {
  progress: AnnouncementSendProgressState;
  /**
   * Resolves a project id to a human-readable label (its short key, e.g.
   * "CUPPTSUB") for display. `failedProjectIds` itself stays raw ids — that's
   * what the rest of the form's own retry/error-message logic keys off —
   * this only affects what the chip below shows. Defaults to the id itself
   * when omitted, or when a given id has no known key yet.
   */
  projectLabel?: (projectId: string) => string;
  /**
   * Hides the progress bar and the succeeded/failed tally row, showing only
   * the title and the failed-project chips. For a *live* send (a create
   * form, or a dialog's first Publish attempt) the full tally is useful —
   * it's the only feedback the sender has while the batch is in flight. Once
   * a request is being *resumed* later (reopening an approved request that
   * already has some succeeded deliveries from an earlier session), the
   * succeeded count is stale history, not something worth re-litigating each
   * time — only what's still outstanding matters. Defaults to false so
   * every existing caller keeps the full tally unless it opts into this.
   */
  hideSucceededTally?: boolean;
}

/**
 * Live progress for a batch announcement send — one real `POST /cases` per
 * target project, fanned out through settleWithConcurrencyLimit. Both create
 * forms render this the moment handleSubmit starts (see each form's own
 * `sendProgress` state, updated via settleWithConcurrencyLimit's `onSettle`
 * callback as each project's create call actually lands), so a sender
 * targeting a large audience sees "N of total" tick up in real time instead
 * of a single opaque "Creating…" button label for however long the whole
 * batch takes.
 *
 * Stays mounted after the batch finishes, showing the final tally ("N/N",
 * "Announcement sent") — for a full success both forms navigate away
 * immediately after, so this is only visible for a moment. On a case-create
 * failure (partial or total) the form deliberately stays on screen instead
 * of navigating away, so this card's own failed-project-id chips remain
 * visible for the sender to act on directly, alongside the existing error
 * banner's own summary. A fresh submit resets everything (including the
 * failed-id list) back to empty before the next batch starts.
 */
export default function AnnouncementSendProgress({
  progress,
  projectLabel = (projectId) => projectId,
  hideSucceededTally = false,
}: AnnouncementSendProgressProps): JSX.Element {
  const { total, completed, succeeded, failed, failedProjectIds } = progress;
  const percent = total > 0 ? Math.round((completed / total) * 100) : 0;

  return (
    <Card
      variant="outlined"
      sx={{
        mt: 2.5,
        p: 2.5,
        bgcolor: "action.hover",
        display: "flex",
        flexDirection: "column",
        gap: 1.25,
      }}
      role="status"
      aria-live="polite"
    >
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <Megaphone size={18} />
          <Typography variant="subtitle1" fontWeight={700}>
            {completed < total
              ? "Sending announcement…"
              : failed > 0
                ? "Announcement sent with failures"
                : "Announcement sent"}
          </Typography>
        </Box>
        {!hideSucceededTally && (
          <Typography variant="body2" color="text.secondary" fontWeight={600}>
            {completed}/{total}
          </Typography>
        )}
      </Box>

      {!hideSucceededTally && (
        <>
          <LinearProgress
            variant="determinate"
            value={percent}
            color={failed > 0 ? "warning" : "primary"}
            sx={{ height: 8, borderRadius: 1 }}
          />

          <Box sx={{ display: "flex", gap: 2.5 }}>
            <Box sx={{ display: "flex", alignItems: "center", gap: 0.5, color: "success.main" }}>
              <CheckCircle size={14} aria-hidden />
              <Typography variant="caption" color="success.main">
                {succeeded} succeeded
              </Typography>
            </Box>
            {failed > 0 && (
              <Box sx={{ display: "flex", alignItems: "center", gap: 0.5, color: "error.main" }}>
                <XCircle size={14} aria-hidden />
                <Typography variant="caption" color="error.main">
                  {failed} failed
                </Typography>
              </Box>
            )}
          </Box>
        </>
      )}

      {failedProjectIds.length > 0 && (
        <Box>
          <Typography variant="caption" color="error.main" fontWeight={600} sx={{ display: "block", mb: 0.5 }}>
            Didn&apos;t receive the announcement — retry these project{failedProjectIds.length === 1 ? "" : "s"}:
          </Typography>
          <Box sx={{ display: "flex", flexWrap: "wrap", gap: 0.5 }}>
            {failedProjectIds.map((projectId) => (
              <Chip
                key={projectId}
                label={projectLabel(projectId)}
                size="small"
                color="error"
                variant="outlined"
              />
            ))}
          </Box>
        </Box>
      )}
    </Card>
  );
}
