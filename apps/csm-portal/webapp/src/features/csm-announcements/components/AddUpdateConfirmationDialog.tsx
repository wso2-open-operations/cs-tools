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

import type { JSX } from "react";
import { Box, Button, Dialog, DialogActions, DialogContent, DialogTitle, Typography } from "@wso2/oxygen-ui";
import { sanitizeRichTextHtml } from "@utils/sanitizeHtml";

interface AddUpdateConfirmationDialogProps {
  open: boolean;
  content: string;
  caseCount: number;
  confirming: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}

/**
 * Confirmation gate before an update is actually appended as a real comment
 * on every case a published announcement created — same "here's exactly
 * what's about to happen" pattern as PublishConfirmationDialog, mirrors
 * ServiceNow's own "Announcement Update with Comments" flow (see that
 * flow's own doc reference), which has its own separate approval/confirm
 * step distinct from the original announcement's.
 */
export default function AddUpdateConfirmationDialog({
  open,
  content,
  caseCount,
  confirming,
  onCancel,
  onConfirm,
}: AddUpdateConfirmationDialogProps): JSX.Element {
  return (
    <Dialog open={open} onClose={confirming ? undefined : onCancel} maxWidth="sm" fullWidth>
      <DialogTitle>Confirm update</DialogTitle>
      <DialogContent sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="body2" color="text.secondary">
          This appends the text below as a real comment on every case this announcement created ({caseCount}{" "}
          case{caseCount === 1 ? "" : "s"}) — not a replacement for the original content.
        </Typography>
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
          dangerouslySetInnerHTML={{ __html: sanitizeRichTextHtml(content) }}
        />
      </DialogContent>
      <DialogActions>
        <Button onClick={onCancel} disabled={confirming}>
          Cancel
        </Button>
        <Button variant="contained" color="primary" onClick={onConfirm} disabled={confirming}>
          {confirming ? "Posting…" : `Post to ${caseCount} case${caseCount === 1 ? "" : "s"}`}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
