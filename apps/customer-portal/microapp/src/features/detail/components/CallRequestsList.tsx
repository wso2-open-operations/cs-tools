import { Stack, Typography } from "@wso2/oxygen-ui";

import { CallRequestItem, CallRequestItemSkeleton } from "@features/detail/components";
import { useCallRequests } from "@features/detail/hooks";

export function CallRequestsList() {
  const { data, isLoading } = useCallRequests();

  if (isLoading) return <CallRequestsListSkeleton />;

  if (!data?.length) {
    return (
      <Typography variant="body2" color="text.secondary">
        No Call Requests.
      </Typography>
    );
  }

  return (
    <Stack gap={1.5}>
      {data.map((props) => (
        <CallRequestItem key={props.id} {...props} />
      ))}
    </Stack>
  );
}

function CallRequestsListSkeleton() {
  return (
    <Stack gap={1.5}>
      {Array.from({ length: 3 }).map((_, index) => (
        <CallRequestItemSkeleton key={index} />
      ))}
    </Stack>
  );
}
