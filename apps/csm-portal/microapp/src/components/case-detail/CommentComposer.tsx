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
import { Button, FormControlLabel, Stack, Switch, TextField, Typography } from "@wso2/oxygen-ui";
import { Lock } from "@wso2/oxygen-ui-icons-react";
import type { CaseCommentType, CaseState, CaseWorkState } from "@src/types";
import { AttachmentsList, AttachmentsPickerButton } from "@components/support/AttachmentsField";
import { MAX_INLINE_ATTACHMENT_SIZE_BYTES, type PendingAttachment } from "@utils/attachments";
import {
  caseAcceptsPublicComments,
  caseAcceptsWorkNotes,
  publicCommentGateReason,
  workNoteGateReason,
} from "@utils/caseWorkState";

interface CommentComposerProps {
  caseState: CaseState | undefined;
  workState: CaseWorkState;
  isSubmitting: boolean;
  /** Resolves true on a successful post — only then does the composer clear its text/attachments,
   * so a failed submit (e.g. the 409 above) leaves what the user typed intact instead of silently
   * discarding it. */
  onSubmit: (fields: { type: CaseCommentType; content: string; attachments: PendingAttachment[] }) => Promise<boolean>;
}

/** `work_note` is restricted to internal users server-side — both options are offered here since
 * every microapp user is internal staff (see reference_csm_operations memory: no customer-facing
 * login path exists in this app). `activity` is system/backend-generated only, not user-postable.
 *
 * Two independent gates, transcribed from the backend's actual guard (CreateCaseComment in
 * backend/internal/handler/cases.go — not the openapi.yaml doc, which omits both rules):
 *   - Comment (public): only while `work_in_progress` + `ongoing` — 409s otherwise.
 *   - Work note: allowed in any state except `closed` — 409s only there.
 * The toggle disables whichever option is currently blocked and defaults to whichever one isn't,
 * so the 409 for either case is prevented at the UI level rather than just handled after the fact. */
export function CommentComposer({ caseState, workState, isSubmitting, onSubmit }: CommentComposerProps) {
  const publicCommentsBlocked = !caseAcceptsPublicComments(caseState, workState);
  const workNotesBlocked = !caseAcceptsWorkNotes(caseState);
  // Both blocked only means closed — show that reason over the narrower public-comment one, since
  // "customer replies disabled" undersells a fully read-only case. Neither reason applies once
  // public comments are allowed (work notes are never blocked while the case is work_in_progress).
  const gateReason = workNotesBlocked
    ? workNoteGateReason(caseState)
    : publicCommentsBlocked
      ? publicCommentGateReason(caseState, workState)
      : null;
  const [type, setType] = useState<CaseCommentType>(publicCommentsBlocked ? "work_note" : "comment");
  const [content, setContent] = useState("");
  const [attachments, setAttachments] = useState<PendingAttachment[]>([]);
  const [attachmentError, setAttachmentError] = useState<string | null>(null);
  const isWorkNote = type === "work_note";

  // Both comment types are blocked only once the case is closed (work notes' one and only gate).
  const allBlocked = publicCommentsBlocked && workNotesBlocked;
  // Attachment-only posts are allowed (no text required) — the case still gets the files.
  const canSubmit = !allBlocked && (content.trim().length > 0 || attachments.length > 0) && !isSubmitting;

  const handleSubmit = () => {
    if (!canSubmit) return;
    void onSubmit({ type, content: content.trim(), attachments }).then((succeeded) => {
      if (succeeded) {
        setContent("");
        setAttachments([]);
      }
    });
  };

  return (
    <Stack gap={1}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" gap={1} flexWrap="wrap">
        <Typography variant="caption" color="text.secondary">
          {isWorkNote ? "Internal work note — not visible to the customer." : "Reply to this case…"}
        </Typography>
        <FormControlLabel
          sx={{ mr: 0 }}
          control={
            <Switch
              size="small"
              color="primary"
              checked={isWorkNote}
              // Mirrors the webapp's CsmCaseCommentInput: public comments and work notes are
              // never both blocked except when closed (allBlocked), where the whole composer is
              // disabled anyway — so it's always safe to force+lock the toggle to work_note
              // whenever public comments are the blocked side.
              onChange={(e) => setType(e.target.checked ? "work_note" : "comment")}
              disabled={publicCommentsBlocked}
            />
          }
          label={
            <Stack direction="row" alignItems="center" gap={0.5}>
              <Lock size={14} />
              <Typography variant="caption">Internal note</Typography>
            </Stack>
          }
        />
      </Stack>

      {gateReason && (
        <Typography variant="caption" color="text.secondary">
          {gateReason}
        </Typography>
      )}

      <TextField
        placeholder={isWorkNote ? "Add an internal work note…" : "Add a comment…"}
        multiline
        minRows={2}
        fullWidth
        size="small"
        value={content}
        onChange={(e) => setContent(e.target.value)}
        disabled={allBlocked}
      />

      <Stack direction="row" alignItems="center" justifyContent="space-between" gap={1}>
        <AttachmentsPickerButton
          onChange={setAttachments}
          disabled={isSubmitting || allBlocked}
          maxSizeBytes={MAX_INLINE_ATTACHMENT_SIZE_BYTES}
          onError={setAttachmentError}
        />
        <Button variant="contained" size="small" disabled={!canSubmit} onClick={handleSubmit}>
          Post
        </Button>
      </Stack>

      {attachmentError && (
        <Typography variant="caption" color="error.main">
          {attachmentError}
        </Typography>
      )}

      <AttachmentsList attachments={attachments} onChange={setAttachments} disabled={isSubmitting || allBlocked} />
    </Stack>
  );
}
