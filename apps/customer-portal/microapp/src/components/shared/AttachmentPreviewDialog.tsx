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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

import type { Attachment } from "@root/src/types";
import { Box, Dialog, IconButton, Stack, Typography } from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";

export function AttachmentPreviewDialog({
  open,
  attachment,
  src,
  onClose,
}: {
  open: boolean;
  attachment: Attachment | null;
  src: string | null;
  onClose: () => void;
}) {
  return (
    <Dialog open={open} onClose={onClose} fullScreen fullWidth>
      <Stack direction="row" justifyContent="space-between" alignItems="center" sx={{ mt: "var(--safe-top)", p: 1 }}>
        <Typography variant="h6">{attachment?.fileName}</Typography>
        <IconButton size="small">
          <X />
        </IconButton>
      </Stack>
      <Box bgcolor="grey" sx={{ height: "100%", mb: "var(--safe-bottom)" }}>
        <Box
          component="img"
          src={src ?? "#"}
          alt="Attachment preview"
          sx={{
            maxWidth: "100%",
            maxHeight: "100%",
            objectFit: "contain",
            display: "block",
            margin: "auto",
          }}
        />
      </Box>
    </Dialog>
  );
}
