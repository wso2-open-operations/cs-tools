import { Card, Stack, Typography } from "@mui/material";

export function Comment({ children, author, timestamp }: { children: string; author: string; timestamp: string }) {
  return (
    <Card component={Stack} p={1} gap={1.5} elevation={0} sx={{ bgcolor: "background.card" }}>
      <Stack direction="row" justifyContent="space-between">
        <Typography variant="body2" fontWeight="medium">
          {author}
        </Typography>
        <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
          {timestamp}
        </Typography>
      </Stack>
      <Typography variant="body2">{children}</Typography>
    </Card>
  );
}
