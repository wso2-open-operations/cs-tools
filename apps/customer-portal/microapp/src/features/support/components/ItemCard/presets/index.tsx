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
import type { CaseSummary } from "@features/case-types/cases/types";
import type { ChangeRequestSummary } from "@features/case-types/change-requests/types/change.model";
import type { Chat } from "@features/case-types/conversations/types";
import type { ServiceRequestSummary } from "@features/case-types/service-requests/types";
import { ItemCard } from "@features/support/components";

import { PriorityChip, StatusChip } from "@shared/components/support";

import { CASE_TYPE_CONFIGS, CASE_TYPES, ROUTES } from "@shared/constants";

export function CaseItemCard({
  id,
  title,
  number,
  internalId,
  statusId,
  severityId,
  assigned,
  createdOn,
}: CaseSummary) {
  const type = CASE_TYPES.DEFAULT;
  const { icon, color } = CASE_TYPE_CONFIGS[type];

  return (
    <ItemCard.Root to={ROUTES[type].by(id)}>
      <ItemCard.Header icon={icon} iconColor={color} number={number} internalId={internalId} />
      <ItemCard.Title>{title}</ItemCard.Title>
      <ItemCard.Meta label={assigned ?? "Not Assigned"}>
        <PriorityChip size="small" id={severityId} />
      </ItemCard.Meta>
      <ItemCard.Footer timestamp={createdOn}>
        <StatusChip type={type} size="small" id={statusId} />
      </ItemCard.Footer>
    </ItemCard.Root>
  );
}

export function ChatItemCard({ id, description, number, statusId, count, createdOn }: Chat) {
  const type = CASE_TYPES.CHAT;
  const { icon, color } = CASE_TYPE_CONFIGS[type];

  return (
    <ItemCard.Root to={ROUTES[type].by(id)}>
      <ItemCard.Header icon={icon} iconColor={color} number={number} />
      <ItemCard.Title>{description}</ItemCard.Title>
      <ItemCard.Meta label={`${count} messages`} suffix="0 KB" />
      <ItemCard.Footer timestamp={createdOn}>
        <StatusChip type={type} size="small" id={statusId} />
      </ItemCard.Footer>
    </ItemCard.Root>
  );
}

export function ServiceRequestItemCard({
  id,
  title,
  number,
  internalId,
  statusId,
  issueType,
  createdOn,
}: ServiceRequestSummary) {
  const type = CASE_TYPES.SERVICE_REQUEST;
  const { icon, color } = CASE_TYPE_CONFIGS[type];

  return (
    <ItemCard.Root to={ROUTES[type].by(id)}>
      <ItemCard.Header icon={icon} iconColor={color} number={number} internalId={internalId} />
      <ItemCard.Title>{title}</ItemCard.Title>
      <ItemCard.Meta label={issueType ?? "Unspecified"} />
      <ItemCard.Footer timestamp={createdOn}>
        <StatusChip type={type} size="small" id={statusId} />
      </ItemCard.Footer>
    </ItemCard.Root>
  );
}

export function ChangeRequestItemCard({
  id,
  title,
  number,
  internalId,
  statusId,
  impactId,
  requestType,
  endDate,
  createdOn,
}: ChangeRequestSummary) {
  const type = CASE_TYPES.CHANGE_REQUEST;
  const { icon, color } = CASE_TYPE_CONFIGS[type];

  return (
    <ItemCard.Root to={ROUTES[type].by(id)}>
      <ItemCard.Header icon={icon} iconColor={color} number={number} internalId={internalId} />
      <ItemCard.Title>{title}</ItemCard.Title>
      <ItemCard.Meta label={requestType ?? "Unspecified"}>
        <PriorityChip type={type} size="small" prefix="Impact" id={impactId} />
      </ItemCard.Meta>
      <ItemCard.ScheduledDate date={endDate} />
      <ItemCard.Footer timestamp={createdOn}>
        <StatusChip type={type} size="small" id={statusId} />
      </ItemCard.Footer>
    </ItemCard.Root>
  );
}

export function SecurityReportAnalysisItemCard({
  id,
  title,
  number,
  internalId,
  statusId,
  deployment,
  createdOn,
}: CaseSummary) {
  const type = CASE_TYPES.SECURITY_REPORT_ANALYSIS;
  const { icon, color } = CASE_TYPE_CONFIGS[type];

  return (
    <ItemCard.Root to={ROUTES[type].by(id)}>
      <ItemCard.Header icon={icon} iconColor={color} number={number} internalId={internalId} />
      <ItemCard.Title>{title}</ItemCard.Title>
      <ItemCard.Meta label={deployment ?? "No Environment"} />
      <ItemCard.Footer timestamp={createdOn}>
        <StatusChip type={type} size="small" id={statusId} />
      </ItemCard.Footer>
    </ItemCard.Root>
  );
}

export function EngagementItemCard({
  id,
  title,
  number,
  internalId,
  statusId,
  engagementType,
  createdOn,
}: CaseSummary) {
  const type = CASE_TYPES.ENGAGEMENT;
  const { icon, color } = CASE_TYPE_CONFIGS[type];

  return (
    <ItemCard.Root to={ROUTES[type].by(id)}>
      <ItemCard.Header icon={icon} iconColor={color} number={number} internalId={internalId} />
      <ItemCard.Title>{title}</ItemCard.Title>
      <ItemCard.Meta label={engagementType ?? "Unspecified"} />
      <ItemCard.Footer timestamp={createdOn}>
        <StatusChip type={type} size="small" id={statusId} />
      </ItemCard.Footer>
    </ItemCard.Root>
  );
}

export function AnnouncementItemCard({ id, title, number, statusId, createdOn }: CaseSummary) {
  const type = CASE_TYPES.ANNOUNCEMENT;
  const { icon, color } = CASE_TYPE_CONFIGS[type];

  return (
    <ItemCard.Root to={ROUTES[type].by(id)}>
      <ItemCard.Header icon={icon} iconColor={color} number={number} />
      <ItemCard.Title>{title}</ItemCard.Title>
      <ItemCard.Meta label="" />
      <ItemCard.Footer timestamp={createdOn}>
        <StatusChip type={type} size="small" id={statusId} />
      </ItemCard.Footer>
    </ItemCard.Root>
  );
}
