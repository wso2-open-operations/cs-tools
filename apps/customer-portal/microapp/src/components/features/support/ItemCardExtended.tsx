// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import { Box, Card, Chip, Divider, Stack, Typography, pxToRem } from "@wso2/oxygen-ui";
import { Calendar, ChevronRight } from "@wso2/oxygen-ui-icons-react";
import { Link } from "react-router-dom";
import type { Priority, ServiceCategory, Status } from "./ItemCard";
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
  const { icon: Icon, color } = TYPE_CONFIG[type];

  return (
    <Card component={Link} to={to} sx={{ textDecoration: "none" }}>
      <Stack bgcolor="background.paper" p={2} gap={2}>
        <Stack gap={0.8}>
          <Stack direction="row" justifyContent="space-between" gap={5}>
            <Stack direction="row" alignItems="center" flexWrap="wrap" gap={1}>
              <Icon size={pxToRem(19)} color={color} />
              <Typography variant="subtitle2" color="text.secondary">
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
              <Box color="text.secondary">
                <ChevronRight size={pxToRem(18)} />
              </Box>
            </Stack>
          </Stack>

          <Stack gap={0.2}>
            <Typography variant="body2" color="text.primary">
              {title}
            </Typography>
            <Typography sx={(theme) => ({ fontSize: theme.typography.pxToRem(13) })} color="text.secondary">
              {description}
            </Typography>
          </Stack>
          {type === "change" && (
            <Stack direction="row" alignItems="center" gap={1}>
              <Box color="text.secondary">
                <Calendar size={pxToRem(16)} />
              </Box>
              <Typography variant="subtitle2" color="text.secondary">
                Scheduled: {props.scheduled}
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
                    case "service":
                      return "Requested By";
                    case "change":
                      return "Owner";
                  }
                })()}
              </Typography>
              <Typography variant="caption">
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
              <Typography variant="caption" color="text.secondary">
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
              <Typography variant="caption">
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
          <Typography variant="caption" color="text.secondary">
            Updated {updated}
          </Typography>
        </Stack>
      </Stack>
    </Card>
  );
}
