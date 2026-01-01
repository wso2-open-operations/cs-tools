import { AttachmentOutlined, CircleOutlined } from "@mui/icons-material";
import { TimelineConnector, TimelineContent, TimelineItem, TimelineSeparator } from "@mui/lab";
import { Box, Card, Stack, Typography } from "@mui/material";

interface TimelineEntryProps {
  author: string;
  title: string;
  timestamp: string;
  comment?: string;
  attachment?: string;
  last?: boolean;
}

export function TimelineEntry({ author, title, timestamp, comment, attachment, last = false }: TimelineEntryProps) {
  return (
    <TimelineItem sx={{ minHeight: "auto" }}>
      <TimelineSeparator>
        <CircleOutlined sx={{ color: "text.tertiary", fontSize: 20 }} />
        {!last && <TimelineConnector />}
      </TimelineSeparator>
      <TimelineContent sx={{ p: 0, pb: last ? 0 : 3 }} ml={1.5}>
        <Stack gap={1.5}>
          <Stack direction="row" justifyContent="space-between" gap={1}>
            <Stack direction="row" gap={0.5}>
              <Typography variant="body2">
                <Box component="span" fontWeight="bold" mr={0.5}>
                  {author}
                </Box>
                {title}
              </Typography>
              <Typography variant="body2" color="text.secondary"></Typography>
            </Stack>
            <Typography variant="subtitle2" fontWeight="regular" color="text.disabled" flexShrink={0}>
              {timestamp}
            </Typography>
          </Stack>
          {comment && <Comment attachment={attachment}>{comment}</Comment>}
        </Stack>
      </TimelineContent>
    </TimelineItem>
  );
}

function Comment({ children, attachment }: { children: string; attachment?: string }) {
  return (
    <Card variant="outlined" sx={{ p: 1.5, bgcolor: "action.hover", border: "none" }}>
      <Typography variant="body2">{children}</Typography>
      {attachment && (
        <Stack direction="row" alignItems="center" pt={1} gap={1}>
          <AttachmentOutlined sx={(theme) => ({ fontSize: theme.typography.pxToRem(20), color: "text.secondary" })} />
          <Typography variant="subtitle1" color="text.secondary">
            {attachment}
          </Typography>
        </Stack>
      )}
    </Card>
  );
}
