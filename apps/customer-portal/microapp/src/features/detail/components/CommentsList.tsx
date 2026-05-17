import { Stack, Typography } from "@wso2/oxygen-ui";
import DOMPurify from "dompurify";

import { CommentItem, CommentItemSkeleton } from "@features/detail/components";
import { useComments } from "@features/detail/hooks";

import { RichText } from "@shared/components/common";

import { useDateTime } from "@shared/hooks";

export function CommentsList() {
  const { format } = useDateTime();
  const { comments, isLoading } = useComments();

  if (isLoading) return <CommentsListSkeleton />;

  if (!comments?.length) {
    return (
      <Typography variant="body2" color="text.secondary">
        No comments yet.
      </Typography>
    );
  }

  return (
    <Stack gap={1.5}>
      {comments.map(({ id, content, createdOn, createdBy }) => (
        <CommentItem key={id} author={createdBy} timestamp={format(createdOn)}>
          <RichText dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(content) }} />
        </CommentItem>
      ))}
    </Stack>
  );
}

function CommentsListSkeleton() {
  return (
    <Stack gap={1.5}>
      {Array.from({ length: 5 }).map((_, index) => (
        <CommentItemSkeleton key={index} />
      ))}
    </Stack>
  );
}
