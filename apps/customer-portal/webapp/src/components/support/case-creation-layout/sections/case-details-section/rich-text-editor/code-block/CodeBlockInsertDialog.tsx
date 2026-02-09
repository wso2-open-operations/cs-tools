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
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  TextField,
} from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";

export interface CodeBlockInsertDialogProps {
  open: boolean;
  onInsert: (code: string) => void;
  onClose: () => void;
}

/**
 * Dialog for inserting code blocks into the rich text editor.
 *
 * @param {CodeBlockInsertDialogProps} props - Component props.
 * @returns {JSX.Element} The rendered code block insert dialog.
 */
export function CodeBlockInsertDialog({
  open,
  onInsert,
  onClose,
}: CodeBlockInsertDialogProps): JSX.Element {
  const [code, setCode] = useState("");

  const handleInsert = () => {
    if (code.trim()) {
      onInsert(code);
      setCode("");
    }
  };

  const handleCancel = () => {
    setCode("");
    onClose();
  };

  return (
    <Dialog
      open={open}
      onClose={handleCancel}
      maxWidth="md"
      fullWidth
      aria-labelledby="codeblock-dialog-title"
    >
      <DialogTitle
        id="codeblock-dialog-title"
        sx={{ pr: 6, position: "relative" }}
      >
        Insert Code Block
        <IconButton
          aria-label="close"
          onClick={handleCancel}
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
      <DialogContent sx={{ pb: 2, pt: 2 }}>
        <Box sx={{ display: "flex", flexDirection: "column", height: "100%" }}>
          <TextField
            multiline
            fullWidth
            value={code}
            onChange={(e) => setCode(e.target.value)}
            placeholder="Enter your code here..."
            sx={{
              flex: 1,
              "& .MuiInputBase-root": {
                fontFamily: "'Consolas', 'Monaco', 'Courier New', monospace",
                fontSize: "0.875rem",
                height: "100%",
                alignItems: "flex-start",
              },
              "& .MuiInputBase-input": {
                minHeight: { xs: "48vh", sm: "53vh", md: "60vh" },
              },
            }}
          />
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={handleCancel} variant="outlined" size="small">
          Cancel
        </Button>
        <Button
          onClick={handleInsert}
          variant="contained"
          size="small"
          color="primary"
          disabled={!code.trim()}
        >
          Insert
        </Button>
      </DialogActions>
    </Dialog>
  );
}
