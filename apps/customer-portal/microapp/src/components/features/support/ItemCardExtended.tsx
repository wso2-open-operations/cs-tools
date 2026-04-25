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
import { Box, Card, Chip, Divider, Skeleton, Stack, Typography, pxToRem, useTheme } from "@wso2/oxygen-ui";
import { Calendar, ChevronRight } from "@wso2/oxygen-ui-icons-react";
import { Link } from "react-router-dom";
import { PriorityChip, StatusChip } from "./Chip";
import type { CaseSummary, ChangeRequestSummary } from "@src/types";
import type { Chat } from "@root/src/types/chat.model";
import { stripHtmlTags } from "@root/src/utils/others";
import type { ServiceRequestSummary } from "@root/src/types/service.model";

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

interface ServiceItemCardExtendedProps extends BaseItemCardExtendedProps, ServiceRequestSummary {
  type: "service";
}

interface ChangeItemCardExtendedProps extends BaseItemCardExtendedProps, ChangeRequestSummary {
  type: "change";
}

interface SraItemCardExtendedProps extends BaseItemCardExtendedProps, CaseSummary {
  type: "sra";
}

interface EngagementItemCardExtendedProps extends BaseItemCardExtendedProps, CaseSummary {
  type: "engagement";
}

export type ItemCardExtendedProps =
  | CaseItemCardExtendedProps
  | ChatItemCardExtendedProps
  | ServiceItemCardExtendedProps
  | ChangeItemCardExtendedProps
  | SraItemCardExtendedProps
  | EngagementItemCardExtendedProps;

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
                {props.number}
              </Typography>
              {(type === "case" || type === "service" || type === "sra") && (
                <PriorityChip size="small" id={props.severityId} />
              )}
              {type === "service" && <Chip size="small" label={props.issueType ?? "N/A"} />}
              {type === "engagement" && <Chip size="small" label={props.engagementType ?? "Unspecified"} />}
              {type === "change" && (
                <>
                  <PriorityChip type="change" size="small" prefix="Impact" id={props.impactId} />
                  <Chip size="small" label={props.requestType ?? "N/A"} />
                </>
              )}
            </Stack>
            <Stack direction="row" gap={2}>
              <StatusChip type={type} size="small" id={props.statusId} />
              <Box color="text.secondary">
                <ChevronRight size={pxToRem(18)} />
              </Box>
            </Stack>
          </Stack>

          <Stack gap={0.2}>
            <Typography variant="body1" color="text.primary" noWrap>
              {(type === "case" ||
                type === "service" ||
                type === "change" ||
                type === "sra" ||
                type === "engagement") &&
                props.title}
              {type === "chat" && props.description}
            </Typography>
            {(type === "case" ||
              type === "service" ||
              type === "change" ||
              type === "sra" ||
              type === "engagement") && (
              <Typography
                variant="subtitle2"
                color="text.secondary"
                sx={{
                  display: "-webkit-box",
                  WebkitLineClamp: 3,
                  WebkitBoxOrient: "vertical",
                  overflow: "hidden",
                }}
              >
                {stripHtmlTags(props.description)}
              </Typography>
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
                    case "sra":
                    case "engagement":
                      return "Created By";
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
                    case "sra":
                    case "service":
                    case "engagement":
                      return props.createdBy;
                    case "change":
                      return props.assignedTeam ?? "N/A";
                  }
                })()}
              </Typography>
            </Stack>
            <Stack>
              <Typography variant="caption" color="text.secondary">
                {(() => {
                  switch (type) {
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
                    case "service":
                      return props.assignee ?? "N/A";
                    case "change":
                      return <PriorityChip size="small" id={props.impactId} />;
                  }
                })()}
              </Typography>
            </Stack>
          </Stack>
          <Typography variant="caption" color="text.secondary">
            {(() => {
              switch (type) {
                case "case":
                case "chat":
                case "service":
                case "sra":
                case "engagement":
                  return `Created ${dayjs(props.createdOn).fromNow()}`;
                case "change":
                  return `Updated ${dayjs(props.updatedOn).fromNow()}`;
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
