import type { ReactNode } from "react";

import { Box, Card, Skeleton, Stack, Typography } from "@wso2/oxygen-ui";

interface CommentItemProps {
  children: string | ReactNode;
  author: string;
  timestamp: string;
}

export function CommentItem({ children, author, timestamp }: CommentItemProps) {
  return (
    <Card component={Stack} sx={{ bgcolor: "background.default" }}>
      <Stack p={1}>
        <Typography variant="body2" fontWeight="medium">
          {author}
        </Typography>

        <Typography variant="caption" color="text.secondary">
          {timestamp}
        </Typography>
      </Stack>

      <Box bgcolor="background.paper" p={1}>
        <Typography variant="body2">{children}</Typography>
      </Box>
    </Card>
  );
}

export function CommentItemSkeleton() {
  return (
    <Card component={Stack} p={1} gap={1.5} sx={{ bgcolor: "background.default" }}>
      <Stack direction="row" justifyContent="space-between" alignItems="center">
        <Typography variant="body2">
          <Skeleton width={100} />
        </Typography>

        <Typography variant="caption">
          <Skeleton width={60} />
        </Typography>
      </Stack>

      <Typography variant="body2">
        <Skeleton variant="text" />
        <Skeleton variant="text" width="80%" />
      </Typography>
    </Card>
  );
}
