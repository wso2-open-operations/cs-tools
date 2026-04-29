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

import { Box, Card, IconButton, Stack, Typography, pxToRem } from "@wso2/oxygen-ui";
import { Download, Image, Paperclip } from "@wso2/oxygen-ui-icons-react";
import { useQueryClient } from "@tanstack/react-query";
import { cases } from "@features/cases/api/cases.queries";
import { useDateTime } from "@shared/hooks/useDateTime";
import type { Attachment } from "@features/cases/types/case.model";

export function AttachmentCard({
  attachment,
  onPreview,
}: {
  attachment: Attachment;
  onPreview: (attachment: Attachment, blob: Blob) => void;
}) {
  const queryClient = useQueryClient();
  const { fromNow } = useDateTime();

  const handlePreview = async () => {
    const data = await queryClient.fetchQuery(cases.attachment(attachment.id));

    const [prefix, base64] = data.content.split(",");
    const mimeType = prefix.split(":")[1].split(";")[0];

    const byteCharacters = atob(base64);
    const byteArray = new Uint8Array(byteCharacters.length);
    for (let i = 0; i < byteCharacters.length; i++) {
      byteArray[i] = byteCharacters.charCodeAt(i);
    }

    const blob = new Blob([byteArray], { type: mimeType });

    onPreview(attachment, blob);
  };

  return (
    <Card sx={{ p: 1.5 }}>
      <Stack direction="row" alignItems="flex-start" gap={1}>
        <Box
          sx={{
            flexShrink: 0,
            width: 40,
            height: 40,
            borderRadius: 0.5,
            overflow: "hidden",
            bgcolor: "action.hover",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            color: "text.secondary",
          }}
        >
          {attachment.type === "image" ? <Image size={pxToRem(18)} /> : <Paperclip size={pxToRem(18)} />}
        </Box>
        <Stack gap={0.25} minWidth={0} flex={1}>
          <Typography variant="subtitle2" fontWeight="medium" noWrap>
            {attachment.fileName}
          </Typography>
          <Typography variant="caption" color="text.secondary" noWrap>
            {attachment.createdBy} · {fromNow(attachment.createdOn)}
          </Typography>
        </Stack>
        <IconButton onClick={handlePreview}>
          <Download size={pxToRem(18)} />
        </IconButton>
      </Stack>
    </Card>
  );
}
