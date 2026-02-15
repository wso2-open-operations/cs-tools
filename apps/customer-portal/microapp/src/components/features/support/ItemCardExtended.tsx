import dayjs from "dayjs";
import relativeTime from "dayjs/plugin/relativeTime";
import { Box, Card, Divider, Stack, Typography, pxToRem } from "@wso2/oxygen-ui";
import { ChevronRight } from "@wso2/oxygen-ui-icons-react";
import { Link } from "react-router-dom";
import type { Priority, ServiceCategory, Status } from "./ItemCard";
import { PriorityChip, StatusChip } from "./Chip";
import { TYPE_CONFIG } from "./config";
import type { Case } from "@src/types";

dayjs.extend(relativeTime);

interface BaseItemCardExtendedProps {
  to: string;
}

interface CaseItemCardExtendedProps extends BaseItemCardExtendedProps, Case {
  type: "case";
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

export function ItemCardExtended(props: CaseItemCardExtendedProps) {
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
                {props.number}
              </Typography>
              <PriorityChip size="small" id={props.severityId ?? "N/A"} />
            </Stack>
            <Stack direction="row" gap={2}>
              <StatusChip size="small" id={props.statusId ?? "N/A"} />
              <Box color="text.secondary">
                <ChevronRight size={pxToRem(18)} />
              </Box>
            </Stack>
          </Stack>

          <Stack gap={0.2}>
            <Typography variant="body1" color="text.primary">
              {props.title}
            </Typography>
            <Typography variant="subtitle2" color="text.secondary">
              {props.description}
            </Typography>
          </Stack>
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
                  }
                })()}
              </Typography>
              <Typography variant="caption">
                {(() => {
                  switch (type) {
                    case "case":
                      return props.assigned ?? "N/A";
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
                  }
                })()}
              </Typography>
              <Typography variant="caption">
                {(() => {
                  switch (type) {
                    case "case":
                      return dayjs(props.createdOn).fromNow();
                  }
                })()}
              </Typography>
            </Stack>
          </Stack>
          <Typography variant="subtitle2" color="text.secondary">
            Updated {dayjs(props.createdOn).fromNow()}
          </Typography>
        </Stack>
      </Stack>
    </Card>
  );
}
