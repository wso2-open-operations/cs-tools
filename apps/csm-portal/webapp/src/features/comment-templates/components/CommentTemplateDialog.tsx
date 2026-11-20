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
  Checkbox,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  FormControlLabel,
  InputLabel,
  MenuItem,
  Select,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import Editor from "@components/rich-text-editor/Editor";
import type { BeCommentTemplateAppliesTo } from "@api/backend/types";
import { useSaveCommentTemplate } from "@features/comment-templates/api/useCommentTemplates";
import {
  APPLIES_TO_LABELS,
  bodyBytes,
  COMMENT_TEMPLATE_BODY_MAX_BYTES,
  COMMENT_TEMPLATE_NAME_MAX,
  isBlankBody,
} from "@features/comment-templates/utils/commentTemplates";

export interface CommentTemplateDraft {
  /** Set when editing an existing template. */
  id?: string;
  name: string;
  body: string;
  appliesTo: BeCommentTemplateAppliesTo;
  shared: boolean;
}

interface CommentTemplateDialogProps {
  open: boolean;
  /** What the form starts with: a draft to save, or the template being edited. */
  initial: CommentTemplateDraft;
  /** Admins may create shared templates; the backend refuses anyone else. */
  canShare: boolean;
  onClose: () => void;
}

/**
 * Creates or edits a saved comment template: its name, which entry it is for,
 * its text, and (new templates, admins only) whether everyone sees it.
 * Render it with a `key` per opening so the form starts from `initial`.
 */
export default function CommentTemplateDialog({
  open,
  initial,
  canShare,
  onClose,
}: CommentTemplateDialogProps): JSX.Element {
  const editing = !!initial.id;
  const [name, setName] = useState(initial.name);
  const [appliesTo, setAppliesTo] = useState<BeCommentTemplateAppliesTo>(initial.appliesTo);
  const [shared, setShared] = useState(initial.shared);
  const [body, setBody] = useState(initial.body);
  const save = useSaveCommentTemplate();

  const trimmedName = name.trim();
  const tooBig = bodyBytes(body) > COMMENT_TEMPLATE_BODY_MAX_BYTES;
  const problem = !trimmedName
    ? "Give the template a name."
    : trimmedName.length > COMMENT_TEMPLATE_NAME_MAX
      ? `Keep the name under ${COMMENT_TEMPLATE_NAME_MAX} characters.`
      : isBlankBody(body)
        ? "The template has no text."
        : tooBig
          ? "The template is too long. Remove some text or images."
          : null;

  const onSave = async () => {
    if (problem) return;
    try {
      await save.mutateAsync({
        id: initial.id,
        shared: editing ? initial.shared : shared,
        payload: { name: trimmedName, body, appliesTo },
      });
      onClose();
    } catch {
      // Shown from save.error below.
    }
  };

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="md">
      <DialogTitle>{editing ? "Edit template" : "Save as template"}</DialogTitle>
      <DialogContent>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 1 }}>
          <TextField
            label="Name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            inputProps={{ maxLength: COMMENT_TEMPLATE_NAME_MAX }}
            autoFocus
            fullWidth
            size="small"
          />
          <FormControl size="small" fullWidth>
            <InputLabel id="comment-template-applies-to">Use for</InputLabel>
            <Select
              labelId="comment-template-applies-to"
              label="Use for"
              value={appliesTo}
              onChange={(e) => setAppliesTo(e.target.value as BeCommentTemplateAppliesTo)}
            >
              {(Object.keys(APPLIES_TO_LABELS) as BeCommentTemplateAppliesTo[]).map((key) => (
                <MenuItem key={key} value={key}>
                  {APPLIES_TO_LABELS[key]}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
          {canShare && !editing && (
            <FormControlLabel
              control={<Checkbox checked={shared} onChange={(e) => setShared(e.target.checked)} />}
              label="Share with everyone (a shared template; only admins can change it)"
            />
          )}
          {editing && initial.shared && (
            <Typography variant="caption" color="text.secondary">
              A shared template: every engineer sees your changes.
            </Typography>
          )}
          <Editor
            value={initial.body}
            onChange={setBody}
            placeholder="Template text…"
            minHeight={160}
            maxHeight={360}
            toolbarVariant="full"
          />
          {(problem && (name !== initial.name || body !== initial.body)) || tooBig ? (
            <Typography variant="caption" color="error">
              {problem}
            </Typography>
          ) : null}
          {save.error && <Alert severity="error">{save.error.message}</Alert>}
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button
          variant="contained"
          onClick={() => {
            void onSave();
          }}
          disabled={!!problem || save.isPending}
        >
          {save.isPending ? "Saving…" : "Save"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
