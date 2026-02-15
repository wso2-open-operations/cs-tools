import { Card, Stack, Typography } from "@wso2/oxygen-ui";
interface CommentProps {
  children: string;
  author: string;
  timestamp: string;
}

export function Comment({ children, author, timestamp }: CommentProps) {
  return (
    <Card component={Stack} p={1} gap={1.5} sx={{ bgcolor: "background.default" }}>
      <Stack direction="row" justifyContent="space-between">
        <Typography variant="body2" fontWeight="medium">
          {author}
        </Typography>
        <Typography variant="caption" color="text.secondary">
          {timestamp}
        </Typography>
      </Stack>
      <Typography variant="body2">{children}</Typography>
    </Card>
  );
}
