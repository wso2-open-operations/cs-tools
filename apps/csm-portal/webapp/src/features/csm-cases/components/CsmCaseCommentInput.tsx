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
  Link,
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
  Upload,
} from "@wso2/oxygen-ui-icons-react";
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type DragEvent,
  type JSX,
} from "react";
import Editor from "@components/rich-text-editor/Editor";
import {
  ALLOWED_IMAGE_TYPES_LABEL,
  MAX_IMAGE_SIZE_BYTES,
} from "@components/rich-text-editor/richTextConstants";
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
  /**
   * True when the *only* thing blocking a public reply is paused work — i.e.
   * the case is already `work_in_progress` and assigned, just not `ongoing`,
   * so resuming (a single click, no reassignment or state change needed)
   * would unlock it right away. Drives the inline "Resume work" quick-fix
   * shown next to the Internal note toggle; omit/false when the case isn't
   * even in progress yet, where "resume" doesn't apply. Ignored unless
   * `publicCommentDisabledReason` and `onResumeWork` are both set.
   */
  canResumeToUnlockPublicReply?: boolean;
  /** Resumes work on the case (`workState` paused → ongoing). Runs the same
   * single-active-case conflict check as the case header's own Resume
   * action — see CsmCaseDetailPage's `onAction("toggle_work_state")`. Once it
   * succeeds, `publicCommentDisabledReason` clears on its own (the case
   * detail query refetches) and the Internal note toggle unlocks — this
   * doesn't flip it or send anything by itself. */
  onResumeWork?: () => void;
  /** True while a resume (or any other case mutation sharing the same
   * pending flag) is in flight; disables the quick-fix link. */
  isResumingWork?: boolean;
  /** Focus the editor as soon as it mounts (e.g. when the composer opens). */
  autoFocus?: boolean;
  /**
   * Draft state, lifted to the parent so it survives this component
   * unmounting (e.g. the case-detail page hides the Activities tab body on
   * tab switch). All four are optional and independently controllable; any
   * omitted pair falls back to this component's own local state, so callers
   * that don't need cross-unmount persistence (e.g. the incident/change
   * request detail pages) are unaffected.
   */
  draftHtml?: string;
  onDraftHtmlChange?: (html: string) => void;
  draftAttachments?: CommentAttachmentDraft[];
  onDraftAttachmentsChange?: (attachments: CommentAttachmentDraft[]) => void;
  draftInternal?: boolean;
  onDraftInternalChange?: (internal: boolean) => void;
  draftSourceMode?: boolean;
  onDraftSourceModeChange?: (sourceMode: boolean) => void;
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

/**
 * A single piece of state that's either controlled by the parent (value +
 * onChange both supplied) or managed locally (either omitted) — same
 * "value"/"onChange" convention as a controlled `<input>`, but supporting a
 * `setState`-style functional updater since several call sites below update
 * off the previous value (e.g. toggling, appending an attachment).
 *
 * The returned setter has a **stable identity across renders**, exactly like
 * `useState`'s own setter — several callers below memoize with an empty
 * `useCallback` dep array, which would silently close over a stale value if
 * the setter itself weren't stable (current value/mode are read from refs at
 * call time instead of from the closure).
 *
 * `defaultValue` only seeds the local fallback's initial state; it isn't
 * re-read on every render (mirrors `useState`'s own initial-value semantics).
 */
function useDraftState<T>(
  controlledValue: T | undefined,
  onChange: ((value: T) => void) | undefined,
  defaultValue: T,
): [T, (updater: T | ((prev: T) => T)) => void] {
  const [localValue, setLocalValue] = useState<T>(defaultValue);
  const isControlled = controlledValue !== undefined && onChange !== undefined;
  const value = isControlled ? controlledValue : localValue;

  // Refs are synced via effect (not written during render) so this stays
  // compliant with the "no ref mutation during render" rule — the setter
  // below is only ever invoked from event handlers, which always run after
  // the effect for the render that produced them, so it never observes a
  // stale value.
  const valueRef = useRef(value);
  const isControlledRef = useRef(isControlled);
  const onChangeRef = useRef(onChange);
  useEffect(() => {
    valueRef.current = value;
    isControlledRef.current = isControlled;
    onChangeRef.current = onChange;
  });

  const setValue = useCallback((updater: T | ((prev: T) => T)) => {
    const next =
      typeof updater === "function"
        ? (updater as (prev: T) => T)(valueRef.current)
        : updater;
    if (isControlledRef.current) onChangeRef.current?.(next);
    else setLocalValue(next);
  }, []);

  return [value, setValue];
}

export default function CsmCaseCommentInput({
  onSubmit,
  disabled = false,
  publicCommentDisabledReason = null,
  canResumeToUnlockPublicReply = false,
  onResumeWork,
  isResumingWork = false,
  autoFocus = false,
  draftHtml,
  onDraftHtmlChange,
  draftAttachments,
  onDraftAttachmentsChange,
  draftInternal,
  onDraftInternalChange,
  draftSourceMode,
  onDraftSourceModeChange,
}: CsmCaseCommentInputProps): JSX.Element {
  const [html, setHtml] = useDraftState(draftHtml, onDraftHtmlChange, "");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [sourceMode, setSourceMode] = useDraftState(
    draftSourceMode,
    onDraftSourceModeChange,
    false,
  );
  // When customer replies are blocked, the only allowed entry is an internal
  // work note, so start in work-note mode and (below) lock the toggle there.
  const [internal, setInternal] = useDraftState(
    draftInternal,
    onDraftInternalChange,
    !!publicCommentDisabledReason,
  );
  const [maximized, setMaximized] = useState(false);
  // Files attached to this comment; uploaded to the case on send.
  const [attachments, setAttachments] = useDraftState<CommentAttachmentDraft[]>(
    draftAttachments,
    onDraftAttachmentsChange,
    [],
  );
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
  }, [setAttachments]);
  const onAttachmentRemove = useCallback((index: number) => {
    setAttachments((prev) => prev.filter((_, i) => i !== index));
  }, [setAttachments]);
  const onPasteError = useCallback((reason: "size" | "type") => {
    setError(
      reason === "type"
        ? `That image format isn't supported. Allowed formats: ${ALLOWED_IMAGE_TYPES_LABEL}.`
        : `Pasted image exceeds the maximum allowed size of ${formatBytes(MAX_IMAGE_SIZE_BYTES)}.`,
    );
  }, []);

  // Dropping files directly onto the composer, as an alternative to the
  // paperclip button's naming-modal flow (see CsmUploadAttachmentModal) —
  // faster for the common case of "just attach these," at the cost of
  // skipping the custom-name step. dragCounter handles dragenter/dragleave
  // firing on every child element as the pointer crosses them; dragOver
  // should only go false once the pointer has actually left the composer,
  // not just moved between its children.
  const [dragOver, setDragOver] = useState(false);
  const dragCounter = useRef(0);

  const isFileDrag = useCallback(
    (e: DragEvent) => Array.from(e.dataTransfer.types).includes("Files"),
    [],
  );

  const onDragEnter = useCallback(
    (e: DragEvent) => {
      // preventDefault unconditionally for a real file drag, before the
      // disabled/submitting check below — otherwise a file dropped while
      // the composer is unavailable falls through to the browser's own
      // default handling (navigating the tab to open the file), discarding
      // whatever was on the page.
      if (!isFileDrag(e)) return;
      e.preventDefault();
      if (disabled || submitting) return;
      dragCounter.current += 1;
      setDragOver(true);
    },
    [disabled, submitting, isFileDrag],
  );
  const onDragOver = useCallback(
    (e: DragEvent) => {
      // Required on dragover too (not just dragenter) — without this the
      // browser refuses to fire a drop event at all.
      if (!isFileDrag(e)) return;
      e.preventDefault();
    },
    [isFileDrag],
  );
  const onDragLeave = useCallback((e: DragEvent) => {
    if (!isFileDrag(e)) return;
    dragCounter.current = Math.max(0, dragCounter.current - 1);
    if (dragCounter.current === 0) setDragOver(false);
  }, [isFileDrag]);

  const addDroppedFiles = useCallback((files: FileList) => {
    const incoming = Array.from(files);
    if (incoming.length === 0) return;

    // Same size limit the naming modal enforces. Report the first offender
    // (matching submit()'s own single-message pattern below) but still
    // attach whichever dropped files DO pass, rather than rejecting the
    // whole drop over one oversized file.
    const tooLarge = incoming.find((f) => f.size > MAX_ATTACHMENT_SIZE_BYTES);
    const accepted = incoming.filter((f) => f.size <= MAX_ATTACHMENT_SIZE_BYTES);

    if (accepted.length > 0) {
      setAttachments((prev) => {
        const next = [...prev];
        for (const file of accepted) {
          // Same dedupe rule as the modal-driven path.
          if (next.some((a) => fileSignature(a.file) === fileSignature(file))) continue;
          next.push({ file, name: file.name });
        }
        return next;
      });
    }
    setError(
      tooLarge
        ? `"${tooLarge.name}" is too large. The maximum attachment size is ${formatBytes(
            MAX_ATTACHMENT_SIZE_BYTES,
          )}.`
        : null,
    );
  }, [setAttachments]);

  const onDrop = useCallback(
    (e: DragEvent) => {
      // Same ordering as onDragEnter: prevent the browser's default file
      // handling before checking whether the composer can actually accept
      // the drop, and always reset the drag-visual state either way.
      if (!isFileDrag(e)) return;
      e.preventDefault();
      dragCounter.current = 0;
      setDragOver(false);
      if (disabled || submitting) return;
      addDroppedFiles(e.dataTransfer.files);
    },
    [disabled, submitting, isFileDrag, addDroppedFiles],
  );

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
    setHtml,
    attachments,
    setAttachments,
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
  }, [publicReplyLocked, internal, setInternal]);

  // One reason, shown once. When resuming would unlock public replies, the
  // quick-fix next to Internal note covers it (with the actionable link) —
  // the send-row status line below falls back to its normal hint instead of
  // repeating the same reason a second time. Otherwise (the case hasn't even
  // started) there's no quick fix to offer, so the send-row line is the only
  // place the reason shows.
  const showResumeQuickFix =
    publicReplyLocked && canResumeToUnlockPublicReply && !!onResumeWork;
  const showBottomLockReason = publicReplyLocked && !showResumeQuickFix;

  const toggleSourceMode = useCallback(
    (nextSource: boolean) => {
      setSourceMode(nextSource);
      // Coming back to rich mode: remount Editor so it picks up the (possibly
      // hand-edited) HTML as its initial value.
      if (!nextSource) setEditorMountKey((k) => k + 1);
    },
    [setSourceMode],
  );

  return (
    <Box
      data-testid="csm-comment-composer"
      onDragEnter={onDragEnter}
      onDragOver={onDragOver}
      onDragLeave={onDragLeave}
      onDrop={onDrop}
      sx={{
        position: "relative",
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
      {dragOver && (
        <Box
          aria-hidden
          sx={{
            position: "absolute",
            inset: 0,
            zIndex: 1,
            pointerEvents: "none",
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            justifyContent: "center",
            gap: 0.5,
            borderRadius: 1,
            border: "2px dashed",
            borderColor: "primary.main",
            bgcolor: "action.hover",
          }}
        >
          <Upload size={22} />
          <Typography variant="body2">Drop files to attach</Typography>
        </Box>
      )}
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

      {/* Quick-fix for the most common lock reason (paused work): resuming
          is a single click here, unlike the other lock reason (case not
          started yet), which needs the full assign/start flow and isn't
          offered inline. */}
      {showResumeQuickFix && (
        <Typography variant="caption" color="text.secondary">
          Only resumed work can send public replies to the customer.{" "}
          <Link
            component="button"
            type="button"
            variant="caption"
            onClick={onResumeWork}
            disabled={isResumingWork}
          >
            {isResumingWork ? "Resuming…" : "Resume work"}
          </Link>{" "}
          to publish this as a public comment.
        </Typography>
      )}

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
          onPasteError={onPasteError}
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
              : showBottomLockReason
                ? "warning.main"
                : "text.secondary"
          }
        >
          {sizeError ??
            error ??
            (showBottomLockReason
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
