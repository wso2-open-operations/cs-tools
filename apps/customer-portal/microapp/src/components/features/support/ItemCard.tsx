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

import { Card, Stack, Typography, pxToRem, useTheme } from "@wso2/oxygen-ui";
import { ChevronRight, Clock4 } from "@wso2/oxygen-ui-icons-react";
import { Circle } from "@mui/icons-material";
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
  const theme = useTheme();
  const { id, title, type, status, timestamp, to } = props;
  const { icon: Icon, color } = TYPE_CONFIG[type];

  return (
    <Card component={Link} to={to} sx={{ textDecoration: "none", p: 1 }}>
      <Stack gap={0.8}>
        <Stack direction="row" justifyContent="space-between" alignItems="center">
          <Stack direction="row" alignItems="center" gap={1}>
            <Icon size={pxToRem(18)} color={color} />
            <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
              {id}
            </Typography>
            {(type === "case" || type === "service") && <PriorityChip size="small" priority={props.priority} />}
            {type === "change" && <PriorityChip size="small" prefix="Impact" priority={props.impact} />}
          </Stack>
          <ChevronRight size={pxToRem(18)} color={theme.palette.text.secondary} />
        </Stack>

        <Typography variant="body2" color="text.primary">
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
          <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
            Scheduled: {props.scheduled}
          </Typography>
        )}

        <Stack gap={0.5} mt={1}>
          <Stack direction="row" alignItems="center" gap={1}>
            <Clock4 size={pxToRem(13)} color={theme.palette.text.secondary} />
            <Typography
              fontWeight="regular"
              color="text.tertiary"
              sx={(theme) => ({ fontSize: theme.typography.pxToRem(13) })}
            >
              {timestamp}
            </Typography>
          </Stack>
        </Stack>
      </Stack>
    </Card>
  );
}
