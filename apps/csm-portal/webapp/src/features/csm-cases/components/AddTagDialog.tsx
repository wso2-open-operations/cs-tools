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
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";

const MAX_TAG_LABEL_LEN = 100;

interface AddTagDialogProps {
  /** Tags already on the case, so a duplicate label can't be submitted twice. */
  existingLabels: string[];
  isSaving: boolean;
  onClose: () => void;
  onSave: (label: string) => void;
}

/**
 * Add a free-text tag to a case (`POST /cases/{id}/tags`). Tags are genuinely
 * free-text on the backing data source (SN's generic label mechanism, e.g.
 * `micro-gw`, `ws-policy`) — a plain text field, not a picker constrained to a
 * closed enum. ServiceNow-source only; the caller surfaces a rejection on
 * another source.
 */
export default function AddTagDialog({
  existingLabels,
  isSaving,
  onClose,
  onSave,
}: AddTagDialogProps): JSX.Element {
  const [label, setLabel] = useState("");
  const trimmed = label.trim();
  const isDuplicate = existingLabels.some(
    (l) => l.toLowerCase() === trimmed.toLowerCase(),
  );
  const canSubmit = trimmed.length > 0 && !isDuplicate;

  const handleSubmit = (): void => {
    if (!canSubmit) return;
    onSave(trimmed);
  };

  return (
    <Dialog open onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>Add tag</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
          <TextField
            label="Tag"
            placeholder="e.g. micro-gw, ws-policy"
            size="small"
            fullWidth
            autoFocus
            value={label}
            onChange={(e) => setLabel(e.target.value.slice(0, MAX_TAG_LABEL_LEN))}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                handleSubmit();
              }
            }}
            disabled={isSaving}
            error={isDuplicate}
            helperText={
              isDuplicate ? "This case already has that tag." : undefined
            }
          />
          <Typography variant="caption" color="text.secondary">
            Tags are free-text — type any label, there's no fixed list.
          </Typography>
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={isSaving}>
          Cancel
        </Button>
        <Button
          variant="contained"
          disabled={!canSubmit || isSaving}
          loading={isSaving}
          onClick={handleSubmit}
        >
          Add tag
        </Button>
      </DialogActions>
    </Dialog>
  );
}
