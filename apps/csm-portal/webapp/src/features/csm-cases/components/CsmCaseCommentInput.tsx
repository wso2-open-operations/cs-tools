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

import {
  Box,
  Button,
  FormControlLabel,
  IconButton,
  Switch,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import {
  Code,
  Lock,
  Maximize2,
  Minimize2,
  Send,
} from "@wso2/oxygen-ui-icons-react";
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type JSX,
} from "react";
import Editor from "@components/rich-text-editor/Editor";
import { formatBytes } from "@utils/formatBytes";
import { MAX_ATTACHMENT_SIZE_BYTES } from "@features/csm-cases/api/useCsmCaseAttachments";
import CsmUploadAttachmentModal from "@features/csm-cases/components/CsmUploadAttachmentModal";

/** A file staged for upload, with the display name chosen in the modal. */
export interface CommentAttachmentDraft {
  file: File;
  name: string;
}

interface CsmCaseCommentInputProps {
  onSubmit: (
    html: string,
    internal: boolean,
    attachments: CommentAttachmentDraft[],
  ) => Promise<unknown> | void;
  disabled?: boolean;
  /**
   * When set, a **customer-visible** reply cannot be sent right now (e.g. the
   * case isn't in-progress/ongoing) and this string explains why. Only the
   * public-reply path is blocked: internal work notes and attachment-only sends
   * are always allowed. `null`/absent = public replies allowed.
   */
  publicCommentDisabledReason?: string | null;
  /** Focus the editor as soon as it mounts (e.g. when the composer opens). */
  autoFocus?: boolean;
}

/** Stable identity for an attached File, used to dedupe re-picked files. */
function fileSignature(f: File): string {
  return `${f.name}-${f.size}-${f.lastModified}`;
}

// Mirrors the BE request-body cap for POST /cases/{id}/comments
// (handler `maxCommentBodyBytes = 10 << 20`). Comments carry inline images as
// base64 data URIs, so the body can get large; the BE returns 413 past this.
const MAX_COMMENT_BODY_BYTES = 10 * 1024 * 1024;
// Reserve headroom for the JSON envelope ({ type, content }) + string escaping
// so the FE blocks before the BE rejects with 413.
const MAX_COMMENT_CONTENT_BYTES = MAX_COMMENT_BODY_BYTES - 1024;

/** Strip tags + collapse whitespace to decide if the editor is effectively empty. */
function isEmpty(html: string): boolean {
  const text = html.replace(/<[^>]*>/g, "").replace(/&nbsp;/g, " ").trim();
  return text.length === 0;
}

export default function CsmCaseCommentInput({
  onSubmit,
  disabled = false,
  publicCommentDisabledReason = null,
  autoFocus = false,
}: CsmCaseCommentInputProps): JSX.Element {
  const [html, setHtml] = useState<string>("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [sourceMode, setSourceMode] = useState(false);
  // When customer replies are blocked, the only allowed entry is an internal
  // work note, so start in work-note mode and (below) lock the toggle there.
  const [internal, setInternal] = useState<boolean>(
    () => !!publicCommentDisabledReason,
  );
  const [maximized, setMaximized] = useState(false);
  // Files attached to this comment; uploaded to the case on send.
  const [attachments, setAttachments] = useState<CommentAttachmentDraft[]>([]);
  const [attachModalOpen, setAttachModalOpen] = useState(false);

  const onAttachmentClick = useCallback(() => setAttachModalOpen(true), []);
  const onSelectAttachment = useCallback((file: File, name: string) => {
    setAttachments((prev) => {
      // Dedupe re-picked files by identity (the chosen name may still differ).
      if (prev.some((a) => fileSignature(a.file) === fileSignature(file))) {
        return prev;
      }
      return [...prev, { file, name }];
    });
  }, []);
  const onAttachmentRemove = useCallback((index: number) => {
    setAttachments((prev) => prev.filter((_, i) => i !== index));
  }, []);

  // Incrementing this trigger clears the editor (see Editor's ResetPlugin).
  const resetTriggerRef = useRef(0);
  const [resetTrigger, setResetTrigger] = useState(0);

  // Bump this on every source→rich switch so the Editor remounts with the
  // edited HTML as its new initial value (InitialValuePlugin only injects once
  // per mount).
  const [editorMountKey, setEditorMountKey] = useState(0);

  // UTF-8 byte size of the comment body. The BE caps the whole request body, so
  // mirror it here to fail fast with a clear message instead of a 413.
  const bodyBytes = useMemo(
    () => new TextEncoder().encode(html).length,
    [html],
  );
  const overSizeLimit = bodyBytes > MAX_COMMENT_CONTENT_BYTES;
  const sizeError = overSizeLimit
    ? `Comment is too large (${formatBytes(bodyBytes)}). Maximum is ${formatBytes(
        MAX_COMMENT_BODY_BYTES,
      )} — remove or shrink inline images.`
    : null;

  const submit = useCallback(async () => {
    if (submitting || disabled) return;
    // Allow an attachment-only post (no text) — the case still gets the files.
    if (isEmpty(html) && attachments.length === 0) {
      setError("Add a comment or an attachment.");
      return;
    }
    // A customer-visible reply with text is gated on case state; work notes and
    // attachment-only sends are not. Block before posting so the BE guard isn't
    // hit with a doomed request.
    if (!internal && publicCommentDisabledReason && !isEmpty(html)) {
      setError(publicCommentDisabledReason);
      return;
    }
    if (overSizeLimit) {
      setError(sizeError);
      return;
    }
    // Pre-validate file sizes before posting anything: the send is multi-step
    // (comment then uploads), so catching an oversized file here avoids posting
    // the comment and then failing on the upload (which would double-post on retry).
    const tooLarge = attachments.find(
      (a) => a.file.size > MAX_ATTACHMENT_SIZE_BYTES,
    );
    if (tooLarge) {
      setError(
        `"${tooLarge.name}" is too large. The maximum attachment size is ${formatBytes(
          MAX_ATTACHMENT_SIZE_BYTES,
        )}.`,
      );
      return;
    }
    setError(null);
    setSubmitting(true);
    try {
      await onSubmit(html, internal, attachments);
      setHtml("");
      setAttachments([]);
      resetTriggerRef.current += 1;
      setResetTrigger(resetTriggerRef.current);
      setEditorMountKey((k) => k + 1);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to post comment.");
    } finally {
      setSubmitting(false);
    }
  }, [
    disabled,
    html,
    attachments,
    internal,
    publicCommentDisabledReason,
    onSubmit,
    overSizeLimit,
    sizeError,
    submitting,
  ]);

  // Customer replies are blocked for this case state. The composer is locked to
  // work-note mode (toggle disabled, forced on) so the engineer can only log an
  // internal note; work notes are allowed in any state.
  const publicReplyLocked = !!publicCommentDisabledReason;
  useEffect(() => {
    if (publicReplyLocked && !internal) setInternal(true);
  }, [publicReplyLocked, internal]);

  const toggleSourceMode = useCallback(
    (nextSource: boolean) => {
      setSourceMode(nextSource);
      // Coming back to rich mode: remount Editor so it picks up the (possibly
      // hand-edited) HTML as its initial value.
      if (!nextSource) setEditorMountKey((k) => k + 1);
    },
    [],
  );

  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        gap: 1,
        p: internal ? 1.25 : 0,
        borderRadius: internal ? 1 : 0,
        bgcolor: internal ? "action.hover" : undefined,
        border: internal ? 1 : 0,
        borderColor: internal ? "divider" : undefined,
        ...(internal && { borderLeftWidth: "3px", borderLeftColor: "primary.main" }),
      }}
    >
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 1,
          flexWrap: "wrap",
        }}
      >
        <Typography
          variant="caption"
          color="text.secondary"
        >
          {internal
            ? "Internal work note - not visible to the customer."
            : sourceMode
              ? "HTML source — edit raw markup directly."
              : "Reply to this case…"}
        </Typography>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
          <FormControlLabel
            control={
              <Switch
                size="small"
                color="primary"
                checked={internal}
                onChange={(e) => setInternal(e.target.checked)}
                disabled={disabled || submitting || publicReplyLocked}
              />
            }
            label={
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 0.5,
                  fontSize: "0.8rem",
                }}
              >
                <Lock size={14} />
                Internal note
              </Box>
            }
          />
          <FormControlLabel
            control={
              <Switch
                size="small"
                checked={sourceMode}
                onChange={(e) => toggleSourceMode(e.target.checked)}
                disabled={disabled || submitting}
              />
            }
            label={
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 0.5,
                  fontSize: "0.8rem",
                }}
              >
                <Code size={14} />
                HTML source
              </Box>
            }
          />
          <Tooltip
            title={maximized ? "Collapse editor" : "Maximize editor"}
          >
            <IconButton
              size="small"
              onClick={() => setMaximized((m) => !m)}
              aria-label={
                maximized ? "Collapse comment editor" : "Maximize comment editor"
              }
            >
              {maximized ? <Minimize2 size={16} /> : <Maximize2 size={16} />}
            </IconButton>
          </Tooltip>
        </Box>
      </Box>

      {sourceMode ? (
        <TextField
          value={html}
          onChange={(e) => setHtml(e.target.value)}
          placeholder="<p>Type HTML here…</p>"
          multiline
          minRows={maximized ? 18 : 6}
          maxRows={maximized ? 32 : 12}
          disabled={disabled || submitting}
          inputProps={{
            style: {
              fontFamily:
                "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace",
              fontSize: "0.825rem",
              lineHeight: 1.45,
            },
            spellCheck: false,
          }}
          fullWidth
          variant="outlined"
        />
      ) : (
        <Editor
          key={editorMountKey}
          value={html}
          onChange={setHtml}
          resetTrigger={resetTrigger}
          disabled={disabled || submitting}
          placeholder="Reply to this case…"
          minHeight={maximized ? 480 : 96}
          maxHeight={maximized ? 720 : 260}
          toolbarVariant="full"
          showKeyboardHint
          autoFocus={autoFocus}
          enterToSubmit={false}
          onAttachmentClick={onAttachmentClick}
          attachments={attachments.map((a) => a.file)}
          onAttachmentRemove={onAttachmentRemove}
          onSubmitKeyDown={() => {
            void submit();
          }}
        />
      )}

      {/* Naming picker driven by the editor toolbar's attach button. */}
      <CsmUploadAttachmentModal
        open={attachModalOpen}
        onClose={() => setAttachModalOpen(false)}
        onSelect={onSelectAttachment}
      />

      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 1,
        }}
      >
        <Typography
          variant="caption"
          color={
            sizeError || error
              ? "error"
              : publicReplyLocked
                ? "warning.main"
                : "text.secondary"
          }
        >
          {sizeError ??
            error ??
            (publicReplyLocked
              ? publicCommentDisabledReason
              : sourceMode
                ? "Output is sent as-is. Use this to fix paste-formatting or insert tables."
                : "Ctrl/Cmd + Enter to send.")}
        </Typography>
        <Button
          variant="contained"
          color="primary"
          size="small"
          startIcon={internal ? <Lock size={16} /> : <Send size={16} />}
          disabled={
            disabled ||
            submitting ||
            (isEmpty(html) && attachments.length === 0) ||
            overSizeLimit ||
            // Safety net: a public reply with text while replies are locked.
            // Normally unreachable — the toggle is forced to work-note mode.
            (!internal && publicReplyLocked && !isEmpty(html))
          }
          onClick={() => {
            void submit();
          }}
        >
          {submitting
            ? "Sending…"
            : internal
              ? "Save work note"
              : "Send to customer"}
        </Button>
      </Box>
    </Box>
  );
}
