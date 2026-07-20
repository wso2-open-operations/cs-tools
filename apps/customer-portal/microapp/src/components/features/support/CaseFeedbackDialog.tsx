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

import { useCallback, useEffect, useMemo, useState, type ChangeEvent, type ComponentProps } from "react";
import {
  Alert,
  Box,
  Button,
  ButtonBase,
  Card,
  Chip,
  CircularProgress,
  Dialog,
  IconButton,
  Stack,
  TextField,
  Typography,
  alpha,
  useTheme,
} from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { metadata } from "@src/services/metadata";
import { cases } from "@src/services/cases";
import type { CaseFeedbackInput } from "@src/types";

const COMMENT_MAX_LENGTH = 1000;
const EMOJI_SIZE = 40;

// Hoisted so it keeps a stable identity across renders — an inline arrow
// function here would be redefined on every keystroke in the comment field,
// making MUI treat it as a new Paper component and remount the dialog
// contents (dropping focus) each time.
function DialogPaper(props: ComponentProps<typeof Card>) {
  // elevation={0} disables MUI's dark-mode elevation overlay (a translucent
  // white gradient blended over the background for elevation > 0), which
  // otherwise makes the dialog read as slightly see-through in dark mode.
  return <Card component={Stack} elevation={0} {...props} />;
}

interface CaseFeedbackDialogProps {
  open: boolean;
  caseId: string;
  onClose: () => void;
  onSubmitted: () => void;
}

export function CaseFeedbackDialog({ open, caseId, onClose, onSubmitted }: CaseFeedbackDialogProps) {
  const theme = useTheme();
  const { data, isLoading: isMetadataLoading } = useQuery(metadata.get());
  const emojis = useMemo(() => data?.feedbackEmojies ?? [], [data]);

  const [selectedEmojiId, setSelectedEmojiId] = useState<string | null>(null);
  const [selectedChipIds, setSelectedChipIds] = useState<string[]>([]);
  const [comment, setComment] = useState("");
  const [inlineError, setInlineError] = useState<string | null>(null);

  const { mutate, isPending } = useMutation(cases.submitFeedback(caseId));

  const selectedEmoji = useMemo(() => emojis.find((e) => e.id === selectedEmojiId) ?? null, [emojis, selectedEmojiId]);

  // Nothing to collect — close silently so the case action flow is never blocked.
  useEffect(() => {
    if (open && !isMetadataLoading && emojis.length === 0) onClose();
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
    setSelectedChipIds((prev) => (prev.includes(chipId) ? prev.filter((c) => c !== chipId) : [...prev, chipId]));
  }, []);

  const handleCommentChange = useCallback((e: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    setComment(e.target.value.slice(0, COMMENT_MAX_LENGTH));
  }, []);

  const handleSubmit = useCallback(() => {
    if (!selectedEmojiId || isPending) return;
    setInlineError(null);
    const trimmedComment = comment.trim();

    const input: CaseFeedbackInput = {
      emojiId: selectedEmojiId,
      chipIds: selectedChipIds.length ? selectedChipIds : undefined,
      additionalComment: trimmedComment ? trimmedComment : undefined,
    };

    mutate(input, {
      onSuccess: () => {
        resetAndClose();
        onSubmitted();
      },
      onError: () => setInlineError("Failed to submit feedback. Please try again."),
    });
  }, [selectedEmojiId, isPending, comment, selectedChipIds, mutate, resetAndClose, onSubmitted]);

  const canSubmit = !!selectedEmojiId && !isPending;

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      fullWidth
      aria-labelledby="case-feedback-dialog-title"
      slots={{ paper: DialogPaper }}
      slotProps={{
        paper: {
          // MuiDialog.styleOverrides.paper sets opacity: 0.7 + a backdrop
          // blur theme-wide (an intentional "frosted glass" look for most
          // dialogs). background.paper is also itself a translucent
          // "acrylic" color (#000000b8) in this theme, not a solid one —
          // opt this text-heavy form out of both and use the solid
          // background.default instead so it reads as fully opaque.
          sx: { bgcolor: "background.default", opacity: 1, backdropFilter: "none", p: 2, gap: 2, m: 2 },
        },
      }}
    >
      <Stack direction="row" justifyContent="space-between" alignItems="flex-start">
        <Stack gap={0.25}>
          <Typography id="case-feedback-dialog-title" variant="h6" fontWeight={650}>
            How was your support experience?
          </Typography>
          <Typography variant="body2" sx={{ opacity: 0.8 }}>
            Your feedback helps us improve the support we provide.
          </Typography>
        </Stack>
        <IconButton aria-label="Close" size="small" onClick={handleClose} disabled={isPending}>
          <X size={18} />
        </IconButton>
      </Stack>

      {inlineError && (
        <Alert severity="error" onClose={() => setInlineError(null)}>
          {inlineError}
        </Alert>
      )}

      {isMetadataLoading ? (
        <Box sx={{ display: "flex", justifyContent: "center", py: 4 }}>
          <CircularProgress size={28} />
        </Box>
      ) : (
        <Stack gap={2}>
          <Stack gap={1}>
            <Typography variant="body2" fontWeight={600}>
              Rate your experience{" "}
              <Box component="span" sx={{ color: "error.main" }}>
                *
              </Box>
            </Typography>
            <Stack direction="row" role="radiogroup" aria-label="Rate your experience" justifyContent="space-between">
              {emojis.map((emoji) => {
                const selected = selectedEmojiId === emoji.id;
                return (
                  <Stack key={emoji.id} alignItems="center" gap={0.5} sx={{ flex: 1 }}>
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
                        bgcolor: selected ? alpha(theme.palette.primary.main, 0.1) : "transparent",
                      }}
                    >
                      <Box
                        component="img"
                        src={selected ? emoji.selectedImage : emoji.unselectedImage}
                        alt={emoji.name}
                        sx={{ width: EMOJI_SIZE, height: EMOJI_SIZE, display: "block", objectFit: "contain" }}
                      />
                    </ButtonBase>
                    <Typography
                      variant="caption"
                      color={selected ? "text.primary" : "text.secondary"}
                      fontWeight={selected ? 600 : 400}
                      textAlign="center"
                    >
                      {emoji.name}
                    </Typography>
                  </Stack>
                );
              })}
            </Stack>
          </Stack>

          {selectedEmoji && selectedEmoji.chips.length > 0 && (
            <Stack gap={1}>
              <Typography variant="body2" fontWeight={600}>
                What stood out?{" "}
                <Box component="span" sx={{ color: "text.secondary", fontWeight: 400 }}>
                  (optional)
                </Box>
              </Typography>
              <Box sx={{ display: "flex", flexWrap: "wrap", gap: 1 }}>
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
                      size="small"
                    />
                  );
                })}
              </Box>
            </Stack>
          )}

          <Stack gap={1}>
            <Typography variant="body2" fontWeight={600}>
              Additional comments{" "}
              <Box component="span" sx={{ color: "text.secondary", fontWeight: 400 }}>
                (optional)
              </Box>
            </Typography>
            <TextField
              placeholder="Tell us anything else about your experience..."
              value={comment}
              onChange={handleCommentChange}
              fullWidth
              multiline
              rows={3}
              disabled={isPending}
              slotProps={{
                htmlInput: { "aria-label": "Additional comments", maxLength: COMMENT_MAX_LENGTH },
                formHelperText: { sx: { textAlign: "right", m: 0, mt: 0.5 } },
              }}
              helperText={`${comment.length}/${COMMENT_MAX_LENGTH}`}
            />
          </Stack>
        </Stack>
      )}

      <Stack direction="row" justifyContent="end" gap={1}>
        <Button variant="outlined" onClick={handleClose} disabled={isPending}>
          Skip
        </Button>
        <Button
          variant="contained"
          onClick={handleSubmit}
          disabled={!canSubmit}
          startIcon={isPending ? <CircularProgress size={16} color="inherit" /> : undefined}
        >
          {isPending ? "Submitting..." : "Submit Feedback"}
        </Button>
      </Stack>
    </Dialog>
  );
}
