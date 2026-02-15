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

import dayjs from "dayjs";
import relativeTime from "dayjs/plugin/relativeTime";
import { Card, Stack, Typography, pxToRem, useTheme } from "@wso2/oxygen-ui";
import { ChevronRight, Clock4 } from "@wso2/oxygen-ui-icons-react";
import { Circle } from "@mui/icons-material";
import { PriorityChip, StatusChip } from "@components/features/support";
import { Link } from "react-router-dom";

import { TYPE_CONFIG } from "./config";
import type { Case } from "@src/types";

dayjs.extend(relativeTime);

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
  to: string;
}

interface CaseItemCardProps extends BaseItemCardProps, Case {
  type: "case";
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

export function ItemCard(props: CaseItemCardProps) {
  const theme = useTheme();
  const { type, to } = props;
  const { icon: Icon, color } = TYPE_CONFIG[type];

  return (
    <Card component={Link} to={to} sx={{ textDecoration: "none", p: 1 }}>
      <Stack gap={0.8}>
        <Stack direction="row" justifyContent="space-between" alignItems="center">
          <Stack direction="row" alignItems="center" gap={1}>
            <Icon size={pxToRem(18)} color={color} />
            <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
              {props.number}
            </Typography>
            <PriorityChip size="small" id={props.severityId ?? "N/A"} />
          </Stack>
          <ChevronRight size={pxToRem(18)} color={theme.palette.text.secondary} />
        </Stack>

        <Typography variant="body1" color="text.primary">
          {props.title}
        </Typography>

        <Stack direction="row" alignItems="center" gap={1}>
          <StatusChip size="small" id={props.statusId ?? "N/A"} />
          <Circle sx={(theme) => ({ color: "text.tertiary", fontSize: theme.typography.pxToRem(4) })} />
          <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
            {type === "case" && (props.assigned ?? "N/A")}
          </Typography>
        </Stack>

        <Stack gap={0.5} mt={1}>
          <Stack direction="row" alignItems="center" gap={1}>
            <Clock4 size={pxToRem(13)} color={theme.palette.text.secondary} />
            <Typography
              fontWeight="regular"
              color="text.tertiary"
              sx={(theme) => ({ fontSize: theme.typography.pxToRem(13) })}
            >
              {dayjs(props.createdOn).fromNow()}
            </Typography>
          </Stack>
        </Stack>
      </Stack>
    </Card>
  );
}
