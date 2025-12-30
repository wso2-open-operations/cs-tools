import { Card, Stack, Typography, SvgIcon } from "@mui/material";
import { CalendarMonth, ChevronRight, Circle, Schedule } from "@mui/icons-material";
import { PriorityChip, StatusChip } from "@components/features/support";
import { Link } from "react-router-dom";

import { TYPE_CONFIG } from "./config";

export type ItemType = "case" | "chat" | "service" | "change";

export type Status =
  | "in progress"
  | "open"
  | "resolved"
  | "waiting"
  | "closed"
  | "active"
  | "scheduled"
  | "approved"
  | "draft"
  | "rejected"
  | "pending approval";

export type Priority = "low" | "medium" | "high";

export type ServiceCategory = "Security Update" | "Database Change" | "Infrastructure";

interface BaseItemCardProps {
  id: string;
  title: string;
  timestamp: string;
  to: string;
}

interface CaseItemCardProps extends BaseItemCardProps {
  type: "case";
  priority: Priority;
  status: Status;
  assignee: string;
}

interface ChatItemCardProps extends BaseItemCardProps {
  type: "chat";
  status: Status;
  count: number;
  kb: number;
}

interface ServiceItemCardProps extends BaseItemCardProps {
  type: "service";
  priority: Priority;
  status: Status;
  category: ServiceCategory;
}

interface ChangeItemCardProps extends BaseItemCardProps {
  type: "change";
  impact: Priority;
  status: Status;
  category: ServiceCategory;
  scheduled: string;
}

export type ItemCardProps = CaseItemCardProps | ChatItemCardProps | ServiceItemCardProps | ChangeItemCardProps;

export function ItemCard(props: ItemCardProps) {
  const { id, title, type, status, timestamp, to } = props;
  const { icon, color } = TYPE_CONFIG[type];

  return (
    <Card component={Link} to={to} elevation={0} sx={{ textDecoration: "none" }}>
      <Stack gap={0.8}>
        <Stack direction="row" justifyContent="space-between" alignItems="center">
          <Stack direction="row" alignItems="center" gap={1}>
            <SvgIcon component={icon} sx={(theme) => ({ color: color, fontSize: theme.typography.pxToRem(21) })} />
            <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
              {id}
            </Typography>
            {(type === "case" || type === "service") && <PriorityChip size="small" priority={props.priority} />}
            {type === "change" && <PriorityChip size="small" prefix="Impact" priority={props.impact} />}
          </Stack>
          <ChevronRight sx={{ color: "text.tertiary" }} />
        </Stack>

        <Typography variant="body1" color="text.primary">
          {title}
        </Typography>

        <Stack direction="row" alignItems="center" gap={1}>
          <StatusChip size="small" status={status} />
          <Circle sx={(theme) => ({ color: "text.tertiary", fontSize: theme.typography.pxToRem(4) })} />
          <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
            {type === "case" && props.assignee}
            {type === "chat" && `${props.count} messages`}
            {(type === "service" || type === "change") && props.category}
          </Typography>
          {type === "chat" && (
            <>
              <Circle sx={(theme) => ({ color: "text.tertiary", fontSize: theme.typography.pxToRem(4) })} />
              <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
                {props.kb} KB
              </Typography>
            </>
          )}
        </Stack>

        {type === "change" && (
          <Stack direction="row" alignItems="center" gap={1}>
            <CalendarMonth sx={(theme) => ({ fontSize: theme.typography.pxToRem(18), color: "text.secondary" })} />
            <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
              Scheduled: {props.scheduled}
            </Typography>
          </Stack>
        )}

        <Stack gap={0.5} mt={1}>
          <Stack direction="row" alignItems="center" gap={1}>
            <Schedule sx={(theme) => ({ color: "text.tertiary", fontSize: theme.typography.pxToRem(18) })} />
            <Typography
              fontWeight="regular"
              color="text.tertiary"
              sx={(theme) => ({ fontSize: theme.typography.pxToRem(14) })}
            >
              {timestamp}
            </Typography>
          </Stack>
        </Stack>
      </Stack>
    </Card>
  );
}
