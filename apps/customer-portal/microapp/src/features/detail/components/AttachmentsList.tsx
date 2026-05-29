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
import { Stack, Typography } from "@wso2/oxygen-ui";

import { AttachmentItem, AttachmentItemSkeleton } from "@features/detail/components";
import { useAttachments } from "@features/detail/hooks";

export function AttachmentsList() {
  const { data, isLoading } = useAttachments();

  if (isLoading) return <AttachmentsListSkeleton />;

  if (!data?.length) {
    return (
      <Typography variant="body2" color="text.secondary">
        No attachments for this case.
      </Typography>
    );
  }

  return (
    <Stack gap={1.5}>
      {data.map((attachment) => (
        <AttachmentItem key={attachment.id} attachment={attachment} />
      ))}
    </Stack>
  );
}

function AttachmentsListSkeleton() {
  return (
    <Stack gap={1.5}>
      {Array.from({ length: 3 }).map((_, index) => (
        <AttachmentItemSkeleton key={index} />
      ))}
    </Stack>
  );
}
