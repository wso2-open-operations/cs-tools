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
        <AttachmentItem key={attachment.id} attachment={attachment} onPreview={() => {}} /> /* TODO: */
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
