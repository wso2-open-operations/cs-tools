// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License. You may obtain a copy of the License at
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
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";

export interface MarkdownDialogProps {
  open: boolean;
  content: string;
  onChange: (value: string) => void;
  onApply: () => void;
  onClose: () => void;
}

/**
 * Markdown editor dialog for the rich text editor.
 *
 * @param {MarkdownDialogProps} props - Component props.
 * @returns {JSX.Element} The rendered markdown dialog.
 */
export function MarkdownEditorDialog({
  open,
  content,
  onChange,
  onApply,
  onClose,
}: MarkdownDialogProps): JSX.Element {
  const handleApply = () => {
    onApply();
    onClose();
  };

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="md">
      <DialogTitle sx={{ pb: 1, pr: 6, position: "relative" }}>
        <Typography variant="h6">Edit as Markdown</Typography>
        <IconButton
          aria-label="close"
          onClick={onClose}
          sx={{
            position: "absolute",
            right: 12,
            top: 12,
          }}
          size="small"
        >
          <X size={20} />
        </IconButton>
      </DialogTitle>
      <DialogContent
        sx={{ pt: 2, pb: 2, display: "flex", flexDirection: "column" }}
      >
        <TextField
          multiline
          fullWidth
          value={content}
          onChange={(e) => onChange(e.target.value)}
          placeholder="Write your content in Markdown..."
          sx={{
            flex: 1,
            "& .MuiInputBase-root": {
              height: "100%",
              alignItems: "flex-start",
            },
            "& .MuiInputBase-input": {
              minHeight: { xs: "48vh", sm: "53vh", md: "60vh" },
              fontFamily: "monospace",
              fontSize: "0.875rem",
            },
          }}
        />
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2, pt: 2 }}>
        <Button variant="outlined" onClick={onClose} size="small">
          Cancel
        </Button>
        <Button
          variant="contained"
          onClick={handleApply}
          color="primary"
          size="small"
        >
          Apply
        </Button>
      </DialogActions>
    </Dialog>
  );
}
