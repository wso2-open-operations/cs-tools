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
import { Box, Card, IconButton, Skeleton, Stack, Typography } from "@wso2/oxygen-ui";
import { ArrowUpRight, Image, Paperclip } from "@wso2/oxygen-ui-icons-react";

import { usePreview } from "@context/preview";

import type { Attachment } from "@features/cases/types/case.model";

import { useDateTime } from "@shared/hooks/useDateTime";

export function AttachmentItem({ attachment }: { attachment: Attachment }) {
  const { fromNow } = useDateTime();
  const { open } = usePreview();

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
          {attachment.type === "image" ? <Image size={18} /> : <Paperclip size={18} />}
        </Box>

        <Stack gap={0.25} minWidth={0} flex={1}>
          <Typography variant="subtitle2" fontWeight="medium" noWrap>
            {attachment.fileName}
          </Typography>

          <Typography variant="caption" color="text.secondary" noWrap>
            {attachment.createdBy} · {fromNow(attachment.createdOn)}
          </Typography>
        </Stack>

        <IconButton onClick={() => open(attachment)}>
          <ArrowUpRight size={18} />
        </IconButton>
      </Stack>
    </Card>
  );
}

export function AttachmentItemSkeleton() {
  return (
    <Card variant="outlined" sx={{ p: 1.5 }}>
      <Stack direction="row" gap={1} alignItems="center">
        <Skeleton variant="rounded" width={40} height={40} />
        <Stack gap={0.5} flex={1} minWidth={0}>
          <Skeleton variant="text" width="80%" />
          <Skeleton variant="text" width="50%" />
        </Stack>
        <Skeleton variant="circular" width={32} height={32} />
      </Stack>
    </Card>
  );
}
