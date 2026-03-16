import dayjs from "dayjs";
import relativeTime from "dayjs/plugin/relativeTime";
import { Box, Card, Chip, Divider, Skeleton, Stack, Typography, pxToRem, useTheme } from "@wso2/oxygen-ui";
import { Calendar, ChevronRight } from "@wso2/oxygen-ui-icons-react";
import { Link } from "react-router-dom";
import { PriorityChip, StatusChip } from "./Chip";
import type { CaseSummary, ChangeRequestSummary } from "@src/types";
import type { Chat } from "@root/src/types/chat.model";
import { stripHtmlTags } from "@root/src/utils/others";

import { TYPE_CONFIG } from "./config";

dayjs.extend(relativeTime);

interface BaseItemCardExtendedProps {
  to: string;
}

interface CaseItemCardExtendedProps extends BaseItemCardExtendedProps, CaseSummary {
  type: "case";
}

interface ChatItemCardExtendedProps extends BaseItemCardExtendedProps, Chat {
  type: "chat";
}

interface ServiceItemCardExtendedProps extends BaseItemCardExtendedProps {
  type: "service";
}

interface ChangeItemCardExtendedProps extends BaseItemCardExtendedProps, ChangeRequestSummary {
  type: "change";
}

export type ItemCardExtendedProps =
  | CaseItemCardExtendedProps
  | ChatItemCardExtendedProps
  | ServiceItemCardExtendedProps
  | ChangeItemCardExtendedProps;

export function ItemCardExtended(props: ItemCardExtendedProps) {
  const theme = useTheme();
  const { type, to } = props;
  const { icon: Icon, color } = TYPE_CONFIG[type];

  return (
    <Card component={Link} to={to} sx={{ textDecoration: "none" }}>
      <Stack bgcolor="background.paper" p={2} gap={2}>
        <Stack gap={0.8}>
          <Stack direction="row" justifyContent="space-between" gap={5}>
            <Stack direction="row" alignItems="center" flexWrap="wrap" gap={1}>
              <Icon size={pxToRem(19)} color={color} />
              <Typography variant="subtitle2" color="text.secondary">
                {(type === "case" || type === "chat" || type === "change") && props.number}
              </Typography>
              {type === "case" && <PriorityChip size="small" id={props.severityId ?? "N/A"} />}
              {type === "change" && (
                <>
                  <PriorityChip size="small" prefix="Impact" id={props.impactId ?? "N/A"} />
                  <Chip size="small" label={props.requestType ?? "N/A"} />
                </>
              )}
            </Stack>
            <Stack direction="row" gap={2}>
              {(type === "case" || type === "chat" || type === "change") && (
                <StatusChip size="small" id={props.statusId ?? "N/A"} />
              )}
              <Box color="text.secondary">
                <ChevronRight size={pxToRem(18)} />
              </Box>
            </Stack>
          </Stack>

          <Stack gap={0.2}>
            <Typography variant="body1" color="text.primary">
              {(type === "case" || type === "change") && props.title}
              {type === "chat" && props.description}
            </Typography>
            {(type === "case" || type === "change") && (
              <Typography variant="subtitle2" color="text.secondary">
                {stripHtmlTags(props.description)}
              </Typography>
            )}
          </Stack>

          {type === "change" && (
            <Stack direction="row" alignItems="center" gap={1}>
              <Calendar size={pxToRem(16)} color={theme.palette.text.secondary} />
              <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
                Scheduled:{" "}
                {props.scheduledOn?.toLocaleDateString("en-US", {
                  month: "short",
                  day: "numeric",
                  year: "numeric",
                }) ?? "N/A"}
              </Typography>
            </Stack>
          )}
        </Stack>

        <Divider />

        <Stack direction="row" justifyContent="space-between" alignItems="center" gap={5}>
          <Stack direction="row" gap={3}>
            <Stack>
              <Typography variant="caption" color="text.secondary">
                {(() => {
                  switch (type) {
                    case "case":
                      return "Assigned";
                    case "chat":
                      return "Messages";
                    case "change":
                      return "Owner";
                  }
                })()}
              </Typography>
              <Typography variant="caption">
                {(() => {
                  switch (type) {
                    case "case":
                      return props.assigned ?? "N/A";
                    case "chat":
                      return props.count;
                    case "change":
                      return props.owner ?? "N/A";
                  }
                })()}
              </Typography>
            </Stack>
            <Stack>
              <Typography variant="caption" color="text.secondary">
                {(() => {
                  switch (type) {
                    case "case":
                      return "Created";
                    case "chat":
                      return "Started";
                    case "change":
                      return "Priority";
                  }
                })()}
              </Typography>
              <Typography variant="caption">
                {(() => {
                  switch (type) {
                    case "case":
                      return dayjs(props.createdOn).fromNow();
                    case "chat":
                      return dayjs(props.createdOn).fromNow();
                    case "change":
                      return <PriorityChip size="small" id={props.impactId} />;
                  }
                })()}
              </Typography>
            </Stack>
          </Stack>
          <Typography variant="caption" color="text.secondary">
            Updated &nbsp;
            {(() => {
              switch (type) {
                case "case":
                case "chat":
                  return dayjs(props.createdOn).fromNow();
                case "change":
                  return dayjs(props.updatedOn).fromNow();
              }
            })()}
          </Typography>
        </Stack>
      </Stack>
    </Card>
  );
}

export function ItemCardExtendedSkeleton() {
  return (
    <Card sx={{ textDecoration: "none" }}>
      <Stack bgcolor="background.paper" p={2} gap={2}>
        <Stack gap={0.8}>
          <Stack direction="row" justifyContent="space-between" gap={5}>
            <Stack direction="row" alignItems="center" flexWrap="wrap" gap={1}>
              <Skeleton variant="circular" width={pxToRem(19)} height={pxToRem(19)} />
              <Skeleton variant="text" width={80} height={20} />
              <Skeleton variant="rounded" width={60} height={24} sx={{ borderRadius: 1 }} />
            </Stack>

            <Stack direction="row" gap={2} alignItems="center">
              <Skeleton variant="rounded" width={70} height={24} sx={{ borderRadius: 1 }} />
              <Skeleton variant="circular" width={pxToRem(18)} height={pxToRem(18)} />
            </Stack>
          </Stack>

          <Stack gap={0.2}>
            <Skeleton variant="text" width="60%" height={28} />
            <Skeleton variant="text" width="85%" height={20} />
            <Skeleton variant="text" width="40%" height={20} />
          </Stack>
        </Stack>

        <Divider />

        <Stack direction="row" justifyContent="space-between" alignItems="center" gap={5}>
          <Stack direction="row" gap={3}>
            <Stack>
              <Skeleton variant="text" width={50} height={16} />
              <Skeleton variant="text" width={90} height={22} />
            </Stack>
            <Stack>
              <Skeleton variant="text" width={50} height={16} />
              <Skeleton variant="text" width={90} height={22} />
            </Stack>
          </Stack>

          <Skeleton variant="text" width={120} height={20} />
        </Stack>
      </Stack>
    </Card>
  );
}
