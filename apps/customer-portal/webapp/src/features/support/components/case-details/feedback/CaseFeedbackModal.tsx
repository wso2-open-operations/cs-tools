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

import type { CaseFeedbackModalProps } from "@features/support/types/supportComponents";
import {
  Alert,
  Box,
  Button,
  ButtonBase,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  Stack,
  TextField,
  Typography,
  alpha,
  useTheme,
} from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type ChangeEvent,
  type JSX,
} from "react";
import useGetMetadata from "@api/useGetMetadata";
import { usePostCaseFeedback } from "@features/support/api/usePostCaseFeedback";

const INLINE_ERROR_STATUSES = new Set([400, 403, 404, 409]);
const COMMENT_MAX_LENGTH = 1000;
const EMOJI_SIZE = 44;

/**
 * Modal for submitting feedback after a case is closed. The rating emojis and
 * their chips are served from GET /metadata. Submission is non-blocking — the
 * case has already closed, so the user can skip. HTTP 4xx errors are shown
 * inline; 5xx errors are forwarded via onError.
 *
 * @param {CaseFeedbackModalProps} props - Modal control props.
 * @returns {JSX.Element} The case feedback modal.
 */
export default function CaseFeedbackModal({
  open,
  caseId,
  onClose,
  onSubmitted,
  onError,
}: CaseFeedbackModalProps): JSX.Element {
  const theme = useTheme();
  const { data: metadata, isLoading: isMetadataLoading } = useGetMetadata();
  const emojis = useMemo(() => metadata?.feedbackEmojies ?? [], [metadata]);

  const [selectedEmojiId, setSelectedEmojiId] = useState<string | null>(null);
  const [selectedChipIds, setSelectedChipIds] = useState<string[]>([]);
  const [comment, setComment] = useState("");
  const [inlineError, setInlineError] = useState<string | null>(null);

  const { mutate, isPending } = usePostCaseFeedback(caseId);

  const selectedEmoji = useMemo(
    () => emojis.find((e) => e.id === selectedEmojiId) ?? null,
    [emojis, selectedEmojiId],
  );

  // Nothing to collect — close silently so the flow is never blocked.
  useEffect(() => {
    if (open && !isMetadataLoading && emojis.length === 0) {
      onClose();
    }
  }, [open, isMetadataLoading, emojis.length, onClose]);

  const resetAndClose = useCallback(() => {
    setSelectedEmojiId(null);
    setSelectedChipIds([]);
    setComment("");
    setInlineError(null);
    onClose();
  }, [onClose]);

  const handleClose = useCallback(() => {
    if (isPending) return;
    resetAndClose();
  }, [isPending, resetAndClose]);

  const handleSelectEmoji = useCallback((emojiId: string) => {
    setSelectedEmojiId(emojiId);
    setSelectedChipIds([]); // chips are emoji-specific
    setInlineError(null);
  }, []);

  const toggleChip = useCallback((chipId: string) => {
    setSelectedChipIds((prev) =>
      prev.includes(chipId) ? prev.filter((c) => c !== chipId) : [...prev, chipId],
    );
  }, []);

  const handleCommentChange = useCallback(
    (e: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
      setComment(e.target.value.slice(0, COMMENT_MAX_LENGTH));
    },
    [],
  );

  const handleSubmit = useCallback(() => {
    if (!selectedEmojiId || isPending) return;
    setInlineError(null);
    const trimmedComment = comment.trim();
    mutate(
      {
        emojiId: selectedEmojiId,
        chipIds: selectedChipIds.length ? selectedChipIds : undefined,
        additionalComment: trimmedComment ? trimmedComment : undefined,
      },
      {
        onSuccess: () => {
          resetAndClose();
          onSubmitted?.();
        },
        onError: (err) => {
          if (INLINE_ERROR_STATUSES.has(err.status)) {
            setInlineError(err.message);
          } else {
            onError?.(err.message ?? "Failed to submit feedback. Please try again.");
            resetAndClose();
          }
        },
      },
    );
  }, [
    selectedEmojiId,
    isPending,
    comment,
    selectedChipIds,
    mutate,
    resetAndClose,
    onSubmitted,
    onError,
  ]);

  const canSubmit = !!selectedEmojiId && !isPending;

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      maxWidth="sm"
      fullWidth
      aria-labelledby="case-feedback-modal-title"
    >
      <IconButton
        aria-label="Close"
        size="small"
        onClick={handleClose}
        disabled={isPending}
        sx={{ position: "absolute", right: 12, top: 12, zIndex: 1 }}
      >
        <X size={18} />
      </IconButton>

      <DialogTitle id="case-feedback-modal-title" sx={{ pr: 6, pb: 0.5 }}>
        <Typography variant="h6" component="span" fontWeight={700}>
          How was your support experience?
        </Typography>
        <Typography
          variant="body2"
          color="text.secondary"
          sx={{ mt: 0.5, fontWeight: "normal" }}
        >
          Your feedback helps us improve the support we provide.
        </Typography>
      </DialogTitle>

      <DialogContent sx={{ pt: 1.5 }}>
        {inlineError && (
          <Alert severity="error" onClose={() => setInlineError(null)} sx={{ mb: 2 }}>
            {inlineError}
          </Alert>
        )}

        {isMetadataLoading ? (
          <Box sx={{ display: "flex", justifyContent: "center", py: 4 }}>
            <CircularProgress size={28} />
          </Box>
        ) : (
          <>
            {/* Emoji rating */}
            <Typography variant="body2" fontWeight={600} sx={{ mb: 1 }}>
              Rate your experience{" "}
              <Box component="span" sx={{ color: "error.main" }}>
                *
              </Box>
            </Typography>
            <Stack
              direction="row"
              spacing={1}
              role="radiogroup"
              aria-label="Rate your experience"
              sx={{ justifyContent: "space-between", mb: 2.5 }}
            >
              {emojis.map((emoji) => {
                const selected = selectedEmojiId === emoji.id;
                return (
                  <Stack key={emoji.id} alignItems="center" spacing={0.5} sx={{ flex: 1 }}>
                    <ButtonBase
                      role="radio"
                      aria-label={emoji.name}
                      aria-checked={selected}
                      onClick={() => handleSelectEmoji(emoji.id)}
                      disabled={isPending}
                      sx={{
                        borderRadius: 2,
                        p: 0.75,
                        border: 1,
                        borderColor: selected ? "primary.main" : "divider",
                        bgcolor: selected
                          ? alpha(theme.palette.primary.main, 0.1)
                          : "transparent",
                        "&:hover": {
                          bgcolor: alpha(theme.palette.primary.main, 0.16),
                        },
                      }}
                    >
                      <Box
                        component="img"
                        src={selected ? emoji.selectedImage : emoji.unselectedImage}
                        alt={emoji.name}
                        sx={{
                          width: EMOJI_SIZE,
                          height: EMOJI_SIZE,
                          display: "block",
                          objectFit: "contain",
                        }}
                      />
                    </ButtonBase>
                    <Typography
                      variant="caption"
                      color={selected ? "text.primary" : "text.secondary"}
                      fontWeight={selected ? 600 : 400}
                      sx={{ textAlign: "center" }}
                    >
                      {emoji.name}
                    </Typography>
                  </Stack>
                );
              })}
            </Stack>

            {/* Chips for the selected emoji */}
            {selectedEmoji && selectedEmoji.chips.length > 0 && (
              <>
                <Typography variant="body2" fontWeight={600} sx={{ mb: 1 }}>
                  What stood out?{" "}
                  <Box component="span" sx={{ color: "text.secondary", fontWeight: 400 }}>
                    (optional)
                  </Box>
                </Typography>
                <Box sx={{ display: "flex", flexWrap: "wrap", gap: 1, mb: 2.5 }}>
                  {selectedEmoji.chips.map((chip) => {
                    const selected = selectedChipIds.includes(chip.id);
                    return (
                      <Chip
                        key={chip.id}
                        label={chip.name}
                        onClick={() => toggleChip(chip.id)}
                        aria-pressed={selected}
                        variant={selected ? "filled" : "outlined"}
                        color={selected ? "primary" : "default"}
                        disabled={isPending}
                        sx={{ fontSize: "0.75rem" }}
                      />
                    );
                  })}
                </Box>
              </>
            )}

            {/* Optional comment */}
            <Typography variant="body2" fontWeight={600} sx={{ mb: 1 }}>
              Additional comments{" "}
              <Box component="span" sx={{ color: "text.secondary", fontWeight: 400 }}>
                (optional)
              </Box>
            </Typography>
            <TextField
              id="case-feedback-comment"
              placeholder="Tell us anything else about your experience..."
              value={comment}
              onChange={handleCommentChange}
              fullWidth
              multiline
              rows={4}
              disabled={isPending}
              inputProps={{
                "aria-label": "Additional comments",
                maxLength: COMMENT_MAX_LENGTH,
              }}
              helperText={`${comment.length}/${COMMENT_MAX_LENGTH}`}
              FormHelperTextProps={{ sx: { textAlign: "right", m: 0, mt: 0.5 } }}
            />
          </>
        )}
      </DialogContent>

      <DialogActions sx={{ px: 3, pb: 2.5, gap: 1 }}>
        <Button variant="outlined" onClick={handleClose} disabled={isPending}>
          Skip
        </Button>
        <Button
          variant="contained"
          onClick={handleSubmit}
          disabled={!canSubmit}
          startIcon={
            isPending ? <CircularProgress size={16} color="inherit" /> : undefined
          }
        >
          {isPending ? "Submitting..." : "Submit Feedback"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
