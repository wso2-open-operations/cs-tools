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

import { Button, Card, Dialog, Stack, Typography } from "@wso2/oxygen-ui";

interface ConfirmDialogProps {
  open: boolean;
  onClose: () => void;
  onConfirm: () => void;
  title: string;
  description: string;
  cancelLabel?: string;
  confirmLabel?: string;
  confirmColor?: "error" | "primary" | "secondary" | "warning" | "success" | "info";
}

export function ConfirmDialog({
  open,
  onClose,
  onConfirm,
  title,
  description,
  cancelLabel = "Cancel",
  confirmLabel = "Confirm",
  confirmColor = "primary",
}: ConfirmDialogProps) {
  return (
    <Dialog
      open={open}
      onClose={onClose}
      slots={{ paper: (props) => <Card component={Stack} {...props} /> }}
      slotProps={{
        paper: {
          sx: {
            bgcolor: "background.paper",
            p: 1.5,
            gap: 3,
            m: 2,
          },
        },
      }}
    >
      <Stack>
        <Typography variant="h6" fontWeight={650} mb={0.2}>
          {title}
        </Typography>
        <Typography variant="body2" sx={{ opacity: 0.8 }}>
          {description}
        </Typography>
      </Stack>

      <Stack direction="row" justifyContent="end" gap={1}>
        <Button variant="outlined" onClick={onClose}>
          {cancelLabel}
        </Button>
        <Button color={confirmColor} variant="contained" onClick={onConfirm}>
          {confirmLabel}
        </Button>
      </Stack>
    </Dialog>
  );
}
