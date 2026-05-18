import { Stack, Typography } from "@wso2/oxygen-ui";

import { Bubble, BubbleSkeleton } from "@features/chats/components";
import { useConversation } from "@features/detail/hooks";

import { MESSAGE_AUTHOR_TYPES } from "@shared/constants";
import { useDateTime } from "@shared/hooks";

export function BubbleList() {
  const { data, isLoading } = useConversation();
  const { fromNow } = useDateTime();

  if (isLoading) return <BubbleListSkeleton />;

  if (!data?.length) {
    return (
      <Typography variant="body2" color="text.secondary">
        No messages yet.
      </Typography>
    );
  }

  return (
    <Stack gap={2} mt={1}>
      {data
        .slice()
        .reverse()
        .map(({ id, content, createdBy, createdOn }) => {
          const author = createdBy === "Novera" ? MESSAGE_AUTHOR_TYPES.AGENT : MESSAGE_AUTHOR_TYPES.USER;
          const timestamp = fromNow(createdOn);

          return (
            <Bubble
              key={id}
              content={content}
              author={author}
              timestamp={timestamp}
              thinking={false}
              animated={false}
            />
          );
        })}
    </Stack>
  );
}

function BubbleListSkeleton() {
  return (
    <Stack gap={2} p={2} width="100%">
      {Array.from({ length: 6 }).map((_, index) => {
        const author = index % 2 === 0 ? "agent" : "user";
        return <BubbleSkeleton key={index} author={author} />;
      })}
    </Stack>
  );
}
