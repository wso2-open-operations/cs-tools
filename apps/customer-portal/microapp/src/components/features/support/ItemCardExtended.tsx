import { Card, Chip, Divider, Stack, SvgIcon, Typography } from "@mui/material";
import type { Priority, ServiceCategory, Status } from "./ItemCard";
import { Link } from "react-router-dom";
import { CalendarMonth, ChevronRight } from "@mui/icons-material";
import { PriorityChip, StatusChip } from "./Chip";
import { TYPE_CONFIG } from "./config";

interface BaseItemCardExtendedProps {
  id: string;
  title: string;
  description: string;
  updated: string;
  to: string;
}

interface CaseItemCardExtendedProps extends BaseItemCardExtendedProps {
  type: "case";
  priority: Priority;
  status: Status;
  assignee: string;
  created: string;
}

interface ChatItemCardExtendedProps extends BaseItemCardExtendedProps {
  type: "chat";
  category: string;
  status: Status;
  count: number;
  started: string;
}

interface ServiceItemCardExtendedProps extends BaseItemCardExtendedProps {
  type: "service";
  priority: Priority;
  status: Status;
  category: ServiceCategory;
  requestedBy: string;
  assignee: string;
}

interface ChangeItemCardExtendedProps extends BaseItemCardExtendedProps {
  type: "change";
  impact: Priority;
  priority: Priority;
  status: Status;
  category: ServiceCategory;
  scheduled: string;
  owner: string;
}

export type ItemCardExtendedProps =
  | CaseItemCardExtendedProps
  | ChatItemCardExtendedProps
  | ServiceItemCardExtendedProps
  | ChangeItemCardExtendedProps;

export function ItemCardExtended(props: ItemCardExtendedProps) {
  const { id, title, description, type, status, updated, to } = props;
  const { icon, color } = TYPE_CONFIG[type];

  return (
    <Card component={Link} to={to} elevation={0} sx={{ textDecoration: "none" }}>
      <Stack bgcolor="background.paper" borderRadius={1} p={2} gap={2}>
        <Stack gap={0.8}>
          <Stack direction="row" justifyContent="space-between" alignItems="" gap={5}>
            <Stack direction="row" alignItems="center" flexWrap="wrap" gap={1}>
              <SvgIcon component={icon} sx={(theme) => ({ color: color, fontSize: theme.typography.pxToRem(21) })} />
              <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
                {id}
              </Typography>
              {(type === "case" || type === "service") && <PriorityChip size="small" priority={props.priority} />}
              {type === "chat" && <Chip label={props.category} size="small" color="default" />}
              {type === "change" && <PriorityChip size="small" prefix="Impact" priority={props.impact} />}
              {(type === "service" || type === "change") && (
                <Chip label={props.category} size="small" color="default" />
              )}
            </Stack>
            <Stack direction="row" gap={2}>
              <StatusChip size="small" status={status} />
              <ChevronRight sx={{ color: "text.tertiary" }} />
            </Stack>
          </Stack>

          <Stack gap={0.2}>
            <Typography variant="body1" color="text.primary">
              {title}
            </Typography>
            <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
              {description}
            </Typography>
          </Stack>
          {type === "change" && (
            <Stack direction="row" alignItems="center" gap={1}>
              <CalendarMonth sx={(theme) => ({ fontSize: theme.typography.pxToRem(18), color: "text.secondary" })} />
              <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
                Scheduled: {props.scheduled}
              </Typography>
            </Stack>
          )}
        </Stack>

        <Divider />

        <Stack direction="row" justifyContent="space-between" alignItems="center" gap={5}>
          <Stack direction="row" gap={3}>
            <Stack>
              <Typography sx={(theme) => ({ fontSize: theme.typography.pxToRem(14) })} color="text.secondary">
                {(() => {
                  switch (type) {
                    case "case":
                      return "Assigned";
                    case "chat":
                      return "Messages";
                    case "service":
                      return "Requested By";
                    case "change":
                      return "Owner";
                  }
                })()}
              </Typography>
              <Typography sx={(theme) => ({ fontSize: theme.typography.pxToRem(14) })}>
                {(() => {
                  switch (type) {
                    case "case":
                      return props.assignee;
                    case "chat":
                      return props.count;
                    case "service":
                      return props.requestedBy;
                    case "change":
                      return props.owner;
                  }
                })()}
              </Typography>
            </Stack>
            <Stack>
              <Typography sx={(theme) => ({ fontSize: theme.typography.pxToRem(14) })} color="text.secondary">
                {(() => {
                  switch (type) {
                    case "case":
                      return "Created";
                    case "chat":
                      return "Started";
                    case "service":
                      return "Assignee";
                    case "change":
                      return "Priority";
                  }
                })()}
              </Typography>
              <Typography sx={(theme) => ({ fontSize: theme.typography.pxToRem(14) })}>
                {(() => {
                  switch (type) {
                    case "case":
                      return props.created;
                    case "chat":
                      return props.started;
                    case "service":
                      return props.assignee;
                    case "change":
                      return <PriorityChip size="small" priority={props.priority} />;
                  }
                })()}
              </Typography>
            </Stack>
          </Stack>
          <Typography
            fontWeight="regular"
            color="text.secondary"
            sx={(theme) => ({ fontSize: theme.typography.pxToRem(14) })}
          >
            Updated {updated}
          </Typography>
        </Stack>
      </Stack>
    </Card>
  );
}
