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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

import { type JSX } from "react";
import {
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  IconButton,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";
import { ShieldAlert, X } from "@wso2/oxygen-ui-icons-react";

export interface PiiWarningDialogProps {
  open: boolean;
  /** Distinct human-readable labels of the PII types detected. */
  detectedLabels: string[];
  /** Close the dialog and let the user edit their text (submission cancelled). */
  onEdit: () => void;
  /** Proceed with the original submission despite the warning. */
  onProceed: () => void;
}

const TITLE = "Sensitive Information Detected";
const DESCRIPTION =
  "Information that's usually private, like ID numbers or passwords, was " +
  "found in your message. It will be visible to anyone with access to this " +
  "ticket. If it isn't needed to resolve your issue, removing it is recommended.";

export default function PiiWarningDialog({
  open,
  detectedLabels,
  onEdit,
  onProceed,
}: PiiWarningDialogProps): JSX.Element {
  return (
    <Dialog open={open} onClose={onEdit} maxWidth="xs" fullWidth>
      <DialogTitle
        sx={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}
      >
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <ShieldAlert size={20} />
          {TITLE}
        </Box>
        <IconButton size="small" onClick={onEdit} aria-label="close">
          <X size={18} />
        </IconButton>
      </DialogTitle>
      <DialogContent>
        <DialogContentText sx={{ mb: 2 }}>{DESCRIPTION}</DialogContentText>
        {detectedLabels.length > 0 && (
          <>
            <Typography variant="subtitle2" sx={{ mb: 1 }}>
              Detected:
            </Typography>
            <Stack
              component="ol"
              spacing={0.5}
              sx={{ m: 0, pl: 0, listStyle: "none" }}
            >
              {detectedLabels.map((label, index) => (
                <Box
                  component="li"
                  key={label}
                  sx={{ display: "flex", alignItems: "baseline", gap: 1 }}
                >
                  <Typography
                    variant="body2"
                    color="warning.main"
                    sx={{ fontWeight: 600, minWidth: 16, textAlign: "right" }}
                  >
                    {index + 1}.
                  </Typography>
                  <Typography variant="body2">{label}</Typography>
                </Box>
              ))}
            </Stack>
          </>
        )}
      </DialogContent>
      <DialogActions>
        <Button variant="contained" onClick={onEdit} autoFocus>
          Edit
        </Button>
        <Button color="warning" onClick={onProceed}>
          Post anyway
        </Button>
      </DialogActions>
    </Dialog>
  );
}
