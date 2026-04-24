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
import { Card, Skeleton, Stack, Typography, pxToRem, useTheme } from "@wso2/oxygen-ui";
import { Calendar, ChevronRight, Clock4 } from "@wso2/oxygen-ui-icons-react";
import { Circle } from "@mui/icons-material";
import { PriorityChip, StatusChip } from "@components/features/support";
import { Link } from "react-router-dom";

import { TYPE_CONFIG } from "./config";
import type { CaseSummary, ChangeRequestSummary } from "@src/types";
import type { Chat } from "@src/types/chat.model";
import type { ServiceRequestSummary } from "@root/src/types/service.model";

dayjs.extend(relativeTime);

export type ItemType = "case" | "chat" | "service" | "change" | "sra" | "engagement" | "announcement";

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

interface CaseItemCardProps extends BaseItemCardProps, CaseSummary {
  type: "case";
}

interface ChatItemCardProps extends BaseItemCardProps, Chat {
  type: "chat";
}

interface ChangeItemCardProps extends BaseItemCardProps, ChangeRequestSummary {
  type: "change";
}

interface ServiceItemCardProps extends BaseItemCardProps, ServiceRequestSummary {
  type: "service";
}

interface SraItemCardProps extends BaseItemCardProps, CaseSummary {
  type: "sra";
}

interface EngagementItemCardProps extends BaseItemCardProps, CaseSummary {
  type: "engagement";
}

interface AnnouncementItemCardProps extends BaseItemCardProps, CaseSummary {
  type: "announcement";
}

export type ItemCardProps =
  | CaseItemCardProps
  | ChatItemCardProps
  | ChangeItemCardProps
  | ServiceItemCardProps
  | SraItemCardProps
  | EngagementItemCardProps
  | AnnouncementItemCardProps;

export function ItemCard(props: ItemCardProps) {
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
            {(type === "case" || type === "sra" || type === "service") && (
              <PriorityChip size="small" id={props.severityId} />
            )}
            {type === "change" && <PriorityChip type="change" size="small" prefix="Impact" id={props.impactId} />}
          </Stack>
          <ChevronRight size={pxToRem(18)} color={theme.palette.text.secondary} />
        </Stack>

        <Typography variant="body1" color="text.primary" mr={1} noWrap>
          {(type === "case" ||
            type === "sra" ||
            type === "engagement" ||
            type === "service" ||
            type === "change" ||
            type === "announcement") &&
            props.title}
          {type === "chat" && props.description}
        </Typography>

        <Stack direction="row" alignItems="center" gap={1}>
          {type !== "announcement" && (
            <>
              <StatusChip type={type} size="small" id={props.statusId} />
              <Circle sx={(theme) => ({ color: "text.tertiary", fontSize: theme.typography.pxToRem(4) })} />
            </>
          )}
          <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
            {type === "case" && (props.assigned ?? "Not Assigned")}
            {type === "chat" && `${props.count} messages`}
            {type === "service" && (props.issueType ?? "Unspecified")}
            {type === "change" && (props.requestType ?? "Unspecified")}
            {type === "sra" && (props.deployment ?? "No Environment")}
            {type === "engagement" && (props.engagementType ?? "Unspecified")}
          </Typography>

          {type === "chat" && (
            <>
              <Circle sx={(theme) => ({ color: "text.tertiary", fontSize: theme.typography.pxToRem(4) })} />
              <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
                0 KB
              </Typography>
            </>
          )}
        </Stack>

        {type === "change" && (
          <Stack direction="row" alignItems="center" gap={1}>
            <Calendar size={pxToRem(16)} color={theme.palette.text.secondary} />
            <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
              Scheduled:{" "}
              {props.endDate?.toLocaleDateString("en-US", {
                month: "short",
                day: "numeric",
                year: "numeric",
              }) ?? "N/A"}
            </Typography>
          </Stack>
        )}

        <Stack gap={0.5} mt={1} direction="row" justifyContent="space-between">
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
          {type === "announcement" && <StatusChip type={type} size="small" id={props.statusId} />}
        </Stack>
      </Stack>
    </Card>
  );
}

export function ItemCardSkeleton() {
  return (
    <Card sx={{ p: 1 }}>
      <Stack gap={0.8}>
        <Stack direction="row" justifyContent="space-between" alignItems="center">
          <Stack direction="row" alignItems="center" gap={1}>
            <Skeleton variant="circular" width={pxToRem(18)} height={pxToRem(18)} />
            <Skeleton variant="text" width={60} height={20} />
            <Skeleton variant="rounded" width={50} height={24} sx={{ borderRadius: 1 }} />
          </Stack>
          <Skeleton variant="circular" width={pxToRem(18)} height={pxToRem(18)} />
        </Stack>

        <Skeleton variant="text" width="90%" height={28} />

        <Stack direction="row" alignItems="center" gap={1}>
          <Skeleton variant="rounded" width={70} height={24} sx={{ borderRadius: 1 }} />
          <Skeleton variant="circular" width={4} height={4} />
          <Skeleton variant="text" width={80} height={20} />
        </Stack>

        <Stack gap={0.5} mt={1}>
          <Stack direction="row" alignItems="center" gap={1}>
            <Skeleton variant="circular" width={pxToRem(16)} height={pxToRem(16)} />
            <Skeleton variant="text" width={100} height={18} />
          </Stack>
        </Stack>
      </Stack>
    </Card>
  );
}
