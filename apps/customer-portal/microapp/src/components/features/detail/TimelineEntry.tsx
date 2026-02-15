import { Box, Card, Stack, Typography, useTheme, pxToRem } from "@wso2/oxygen-ui";
import { Circle, CircleCheck, CircleDot, Clock4, Paperclip } from "@wso2/oxygen-ui-icons-react";
import { TimelineConnector, TimelineContent, TimelineItem, TimelineSeparator } from "@mui/lab";

interface TimelineEntryBaseProps {
  timestamp?: string;
  last?: boolean;
}
export interface ActivityTimelineEntryProps extends TimelineEntryBaseProps {
  variant: "activity";
  author?: string;
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
  const theme = useTheme();
  const activity = props.variant === "activity";
  const progress = props.variant === "progress";
  const step = props.variant === "step";

  const Indicator = () => {
    switch (props.variant) {
      case "progress":
        if (props.status === "completed")
          return (
            <Box color="primary.contrastText" sx={{ fill: "primary.main" }}>
              <CircleCheck size={pxToRem(24)} fill={theme.palette.primary.main} />
            </Box>
          );
        if (props.status === "active")
          return (
            <Box color="primary.contrastText" sx={{ fill: "primary.main" }}>
              <CircleDot size={pxToRem(24)} fill={theme.palette.primary.main} />
            </Box>
          );
        return (
          <Box color="text.disabled">
            <Circle size={pxToRem(20)} />
          </Box>
        );

      case "step":
        return (
          <Box
            sx={{
              display: "grid",
              placeItems: "center",
              width: pxToRem(20),
              height: pxToRem(20),
              position: "relative",
              color: "text.disabled",
            }}
          >
            <Circle size={pxToRem(20)} strokeWidth={2} />
            <Typography variant="subtitle2" color="text.primary" position="absolute">
              {props.index}
            </Typography>
          </Box>
        );

      default:
        return (
          <Box color="text.disabled">
            <Circle size={pxToRem(20)} />
          </Box>
        );
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
              <Typography variant="body2" fontWeight={step || progress || !props.author ? "medium" : undefined}>
                {activity && (
                  <Box component="span" fontWeight="bold" mr={0.5}>
                    {props.author}
                  </Box>
                )}
                {props.title}
                {props.description && (
                  <Typography variant="subtitle2" fontWeight="regular" color="text.secondary" mt={0.2}>
                    {props.description}
                  </Typography>
                )}
              </Typography>
              <Typography variant="body2" color="text.secondary"></Typography>
            </Stack>
            <Stack direction="row" alignItems="center" gap={1}>
              {progress && timestamp && (
                <Box color="text.secondary">
                  <Clock4 size={pxToRem(14)} />
                </Box>
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
    <Card sx={{ p: 1.5, bgcolor: "action.hover" }}>
      <Typography variant="body2">{children}</Typography>
      {attachment && (
        <Stack direction="row" alignItems="center" pt={1} gap={1}>
          <Box color="text.secondary">
            <Paperclip size={pxToRem(14)} />
          </Box>
          <Typography variant="body2" color="text.secondary">
            {attachment}
          </Typography>
        </Stack>
      )}
    </Card>
  );
}
