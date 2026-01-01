import { AccessTime, AttachmentOutlined, CheckCircle, CircleOutlined, RadioButtonChecked } from "@mui/icons-material";
import { TimelineConnector, TimelineContent, TimelineItem, TimelineSeparator } from "@mui/lab";
import { Avatar, Box, Card, Stack, Typography } from "@mui/material";

interface TimelineEntryBaseProps {
  timestamp?: string;
  last?: boolean;
}
export interface ActivityTimelineEntryProps extends TimelineEntryBaseProps {
  variant: "activity";
  author: string;
  title?: string;
  description?: string;
  comment?: string;
  attachment?: string;
}

export interface ProgressTimelineEntryProps extends TimelineEntryBaseProps {
  variant: "progress";
  title: string;
  description: string;
  status?: "completed" | "active" | "pending";
}

export interface StepTimelineEntryProps extends TimelineEntryBaseProps {
  variant: "step";
  index: number;
  title: string;
  description: string;
}

export type TimelineEntryProps = ActivityTimelineEntryProps | ProgressTimelineEntryProps | StepTimelineEntryProps;

export function TimelineEntry({ timestamp, last = false, ...props }: TimelineEntryProps) {
  const activity = props.variant === "activity";
  const progress = props.variant === "progress";
  const step = props.variant === "step";

  const Indicator = () => {
    switch (props.variant) {
      case "progress":
        if (props.status === "completed") return <CheckCircle sx={{ color: "primary.main", fontSize: 22 }} />;
        if (props.status === "active")
          return <RadioButtonChecked sx={{ color: "primary.main", fontSize: 22, fontWeight: "bold" }} />;
        return <CircleOutlined sx={{ color: "text.disabled", fontSize: 22 }} />;

      case "step":
        return (
          <Avatar
            sx={(theme) => ({
              width: 20,
              height: 20,
              fontSize: theme.typography.pxToRem(12),
              fontWeight: "medium",
              bgcolor: "transparent",
              border: "2px solid",
              borderColor: "text.tertiary",
              color: "text.primary",
            })}
          >
            {props.index}
          </Avatar>
        );

      default:
        return <CircleOutlined sx={{ color: "text.tertiary", fontSize: 20 }} />;
    }
  };

  return (
    <TimelineItem sx={{ minHeight: "auto" }}>
      <TimelineSeparator>
        <Indicator />
        {!last && (
          <TimelineConnector
            sx={{
              bgcolor:
                progress && props.status !== "pending" && props.status !== undefined ? "primary.main" : undefined,
            }}
          />
        )}
      </TimelineSeparator>
      <TimelineContent sx={{ p: 0, pb: last ? 0 : 3 }} ml={1.5}>
        <Stack gap={1.5}>
          <Stack direction={progress ? "column" : "row"} justifyContent="space-between" gap={1}>
            <Stack direction="row" gap={0.5}>
              <Typography variant="body2" fontWeight={step || progress ? "medium" : undefined}>
                {activity && (
                  <Box component="span" fontWeight="bold" mr={0.5}>
                    {props.author}
                  </Box>
                )}
                {props.title}
                {(step || progress) && (
                  <Typography variant="subtitle2" fontWeight="regular" color="text.secondary" mt={0.2}>
                    {props.description}
                  </Typography>
                )}
              </Typography>
              <Typography variant="body2" color="text.secondary"></Typography>
            </Stack>
            <Stack direction="row" alignItems="center" gap={1}>
              {progress && timestamp && (
                <AccessTime sx={(theme) => ({ fontSize: theme.typography.pxToRem(18), color: "text.tertiary" })} />
              )}
              <Typography variant="subtitle2" fontWeight="regular" color="text.disabled" flexShrink={0}>
                {timestamp}
              </Typography>
            </Stack>
          </Stack>
          {activity && props.comment && <Comment attachment={props.attachment}>{props.comment}</Comment>}
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
