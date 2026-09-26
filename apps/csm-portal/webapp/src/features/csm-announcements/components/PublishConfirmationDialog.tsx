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

import { useEffect, type JSX } from "react";
import {
  Box,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Typography,
} from "@wso2/oxygen-ui";
import { sanitizeRichTextHtml } from "@utils/sanitizeHtml";
import ResolvedAudienceList from "@features/csm-announcements/components/ResolvedAudienceList";
import { useResolvedAudiencePreview } from "@features/csm-announcements/api/useResolvedAudiencePreview";
import type { AnnouncementRequest } from "@features/csm-announcements/types/announcementRequests";

interface PublishConfirmationDialogProps {
  open: boolean;
  request: AnnouncementRequest;
  /** True on a retry (only some projects failed last attempt) — changes the
   * confirm button's wording, not its behavior; the caller's own
   * handlePublish already knows to resend only the outstanding subset. */
  isRetry: boolean;
  confirming: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}

/**
 * The final "here's exactly what's about to go out, and to whom" gate
 * before Publish actually fires — added per Danidu's explicit request: an
 * approver signing off on a request isn't the same as someone having looked
 * at the literal content and recipient list right before it becomes real
 * customer-facing cases. Opens only after the creator clicks Publish (see
 * AnnouncementRequestDialog); resolving the audience by name is deferred
 * until then rather than eagerly, since most approved requests sit in that
 * state for a while before anyone actually publishes them.
 */
export default function PublishConfirmationDialog({
  open,
  request,
  isRetry,
  confirming,
  onCancel,
  onConfirm,
}: PublishConfirmationDialogProps): JSX.Element {
  const audience = useResolvedAudiencePreview();

  useEffect(() => {
    if (open) void audience.resolve(request.resolvedProjectIds ?? []);
    // Only re-resolve when the dialog opens for a (possibly different)
    // request — not on every audience object identity change, which would
    // otherwise re-fire on each of its own state updates mid-resolve.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, request.id]);

  return (
    <Dialog open={open} onClose={confirming ? undefined : onCancel} maxWidth="sm" fullWidth>
      <DialogTitle>{isRetry ? "Confirm retry" : "Confirm before sending"}</DialogTitle>
      <DialogContent sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="body2" color="text.secondary">
          This creates a real, customer-visible case for every project below. Review the content and
          recipients before continuing.
        </Typography>

        <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
            <Typography variant="body1" sx={{ fontWeight: 600 }}>
              {request.subject || "(no subject)"}
            </Typography>
            {request.isSecurityAnnouncement && <Chip size="small" color="warning" label="Security" />}
          </Box>
          <Box
            sx={{
              fontSize: "0.875rem",
              lineHeight: 1.5,
              wordBreak: "break-word",
              border: 1,
              borderColor: "divider",
              borderRadius: 1,
              p: 1.5,
              maxHeight: 200,
              overflowY: "auto",
            }}
            dangerouslySetInnerHTML={{ __html: sanitizeRichTextHtml(request.description) }}
          />
        </Box>

        <Box>
          <Typography variant="subtitle2" sx={{ mb: 0.5 }}>
            Recipients
          </Typography>
          <ResolvedAudienceList
            projects={audience.projects}
            total={audience.total}
            isLoading={audience.isLoading}
            isError={audience.isError}
          />
          {audience.truncated && !audience.isLoading && (
            <Typography variant="caption" color="text.secondary">
              Showing the first {audience.projects.length} of {audience.total} — every one of the {audience.total}{" "}
              still receives it, this list is just capped for display.
            </Typography>
          )}
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onCancel} disabled={confirming}>
          Cancel
        </Button>
        <Button variant="contained" color="primary" onClick={onConfirm} disabled={confirming}>
          {confirming
            ? "Sending…"
            : isRetry
              ? "Retry"
              : `Send to ${request.resolvedProjectCount ?? 0} project${
                  request.resolvedProjectCount === 1 ? "" : "s"
                }`}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
