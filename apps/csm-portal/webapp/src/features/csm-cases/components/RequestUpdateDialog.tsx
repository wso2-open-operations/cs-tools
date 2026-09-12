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
  Alert,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Skeleton,
  TextField,
  Typography,
  type SelectChangeEvent,
} from "@wso2/oxygen-ui";
import { useMemo, useState, type JSX } from "react";
import { escapeHtml, sanitizeRichTextHtml } from "@utils/sanitizeHtml";
import { useGetCaseUpdateRequestTemplates } from "@features/csm-cases/api/useRequestCaseUpdate";
import {
  REQUEST_UPDATE_STAGE_LABEL,
  type CaseUpdateRequestCategory,
} from "@features/csm-cases/utils/caseUpdateRequests";
import type { BeCaseUpdateRequestStage } from "@api/backend/types";

/** Payload this dialog produces on confirm — mirrors `RequestCaseUpdateInput` minus `caseId`. */
export interface RequestUpdateSavePayload {
  stage: BeCaseUpdateRequestStage;
  customContent?: string;
}

interface RequestUpdateDialogProps {
  /** Which fixed reminder-message set to preview — see `deriveCaseUpdateRequestCategory`. */
  category: CaseUpdateRequestCategory;
  /** True while the `POST /cases/{id}/request-update` call is in flight. */
  isSaving: boolean;
  onClose: () => void;
  onSave: (payload: RequestUpdateSavePayload) => void;
}

const STAGE_ORDER: BeCaseUpdateRequestStage[] = ["first", "second", "final", "custom"];

/**
 * "Request update" dialog: nudge the customer for a response using one of
 * three fixed reminder templates (previewed read-only, exactly as they'll be
 * posted) or a custom message. Adapted from ServiceNow's equivalent "Add
 * Comment Options" modal for this case's own dedicated backend endpoint —
 * see `POST /cases/{id}/request-update` / `GET /case-update-request-templates`.
 */
export default function RequestUpdateDialog({
  category,
  isSaving,
  onClose,
  onSave,
}: RequestUpdateDialogProps): JSX.Element {
  const { data: templates, isLoading, isError } =
    useGetCaseUpdateRequestTemplates();
  const [stage, setStage] = useState<BeCaseUpdateRequestStage>("first");
  const [customMessage, setCustomMessage] = useState("");

  const previewHtml = useMemo(() => {
    if (stage === "custom" || !templates) return "";
    return sanitizeRichTextHtml(templates[category][stage]);
  }, [templates, category, stage]);

  const trimmedCustomMessage = customMessage.trim();
  // A custom message needs no template — it's still sendable even if the
  // template fetch failed or is still loading; only the three fixed stages
  // depend on the templates query having actually resolved.
  const canSubmit =
    stage === "custom"
      ? trimmedCustomMessage.length > 0
      : !isLoading && !isError && !!templates?.[category]?.[stage];

  const handleSave = (): void => {
    if (!canSubmit) return;
    if (stage === "custom") {
      // The backend accepts any string as `customContent`, but every stored
      // comment is rendered as HTML on read (see CsmCaseCommentBubble), so
      // wrap the plain-text input in escaped paragraphs rather than sending
      // it unescaped — a literal "<" in the engineer's message must not be
      // interpreted as markup once it comes back through the feed.
      const html = trimmedCustomMessage
        .split(/\n+/)
        .filter((line) => line.trim().length > 0)
        .map((line) => `<p>${escapeHtml(line.trim())}</p>`)
        .join("");
      onSave({ stage, customContent: html });
      return;
    }
    onSave({ stage });
  };

  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Request update</DialogTitle>
      <DialogContent>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          <Typography variant="body2" color="text.secondary">
            Post a customer-visible comment nudging the customer for a
            response. Pick one of the fixed reminder messages below, or write
            your own.
          </Typography>

          <FormControl fullWidth size="small">
            <InputLabel id="request-update-stage-label">Message</InputLabel>
            <Select
              labelId="request-update-stage-label"
              label="Message"
              value={stage}
              disabled={isSaving}
              onChange={(e: SelectChangeEvent<string>) =>
                setStage(e.target.value as BeCaseUpdateRequestStage)
              }
            >
              {STAGE_ORDER.map((s) => (
                <MenuItem key={s} value={s}>
                  {s === "custom" ? "Custom message" : REQUEST_UPDATE_STAGE_LABEL[s]}
                </MenuItem>
              ))}
            </Select>
          </FormControl>

          {isError && (
            <Alert severity="error">
              Couldn't load the reminder message templates. You can still send
              a custom message, or close and try again.
            </Alert>
          )}

          {stage === "custom" ? (
            <TextField
              label="Custom message"
              placeholder="Write the message to post on this case…"
              multiline
              minRows={4}
              fullWidth
              size="small"
              value={customMessage}
              onChange={(e) => setCustomMessage(e.target.value)}
              disabled={isSaving}
            />
          ) : isLoading ? (
            <Skeleton variant="rounded" width="100%" height={100} />
          ) : (
            !isError && (
              <Box
                sx={{
                  border: 1,
                  borderColor: "divider",
                  borderRadius: 1,
                  p: 1.5,
                  bgcolor: "action.hover",
                  "& p": { m: 0 },
                  "& p + p": { mt: 0.75 },
                }}
                data-testid="request-update-preview"
                // Fixed templates are server-provided rich text, sanitized
                // above — same safe-render approach the comment feed itself
                // uses (`sanitizeRichTextHtml` + `dangerouslySetInnerHTML`).
                dangerouslySetInnerHTML={{ __html: previewHtml }}
              />
            )
          )}
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={isSaving}>
          Close
        </Button>
        <Button
          variant="contained"
          disabled={!canSubmit || isSaving}
          loading={isSaving}
          onClick={handleSave}
        >
          Send
        </Button>
      </DialogActions>
    </Dialog>
  );
}
