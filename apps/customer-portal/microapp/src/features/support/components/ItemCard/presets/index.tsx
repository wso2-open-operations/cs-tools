import type { CaseSummary } from "@src/features/cases/types";
import type { Chat } from "@src/features/chats/types";
import type { ServiceRequestSummary } from "@src/features/service-requests/types";

import type { ChangeRequestSummary } from "@features/changes/types/change.model";
import { ItemCard } from "@features/support/components";

import { PriorityChip, StatusChip } from "@shared/components/support";

import { CASE_TYPES, CASE_TYPE_CONFIGS, ROUTES } from "@shared/constants";

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
        <StatusChip type={type} size="small" id={statusId} />
      </ItemCard.Meta>
      <ItemCard.Footer timestamp={createdOn}>
        <PriorityChip size="small" id={severityId} />
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
      <ItemCard.Meta label={`${count} messages`} suffix="0 KB">
        <StatusChip type={type} size="small" id={statusId} />
      </ItemCard.Meta>
      <ItemCard.Footer timestamp={createdOn} />
    </ItemCard.Root>
  );
}

export function ServiceRequestItemCard({
  id,
  title,
  number,
  internalId,
  statusId,
  severityId,
  issueType,
  createdOn,
}: ServiceRequestSummary) {
  const type = CASE_TYPES.SERVICE_REQUEST;
  const { icon, color } = CASE_TYPE_CONFIGS[type];

  return (
    <ItemCard.Root to={ROUTES[type].by(id)}>
      <ItemCard.Header icon={icon} iconColor={color} number={number} internalId={internalId} />
      <ItemCard.Title>{title}</ItemCard.Title>
      <ItemCard.Meta label={issueType ?? "Unspecified"}>
        <StatusChip type={type} size="small" id={statusId} />
      </ItemCard.Meta>
      <ItemCard.Footer timestamp={createdOn}>
        <PriorityChip size="small" id={severityId} />
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
        <StatusChip type={type} size="small" id={statusId} />
      </ItemCard.Meta>
      <ItemCard.ScheduledDate date={endDate} />
      <ItemCard.Footer timestamp={createdOn}>
        <PriorityChip type={type} size="small" prefix="Impact" id={impactId} />
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
  severityId,
  deployment,
  createdOn,
}: CaseSummary) {
  const type = CASE_TYPES.SECURITY_REPORT_ANALYSIS;
  const { icon, color } = CASE_TYPE_CONFIGS[type];

  return (
    <ItemCard.Root to={ROUTES[type].by(id)}>
      <ItemCard.Header icon={icon} iconColor={color} number={number} internalId={internalId} />
      <ItemCard.Title>{title}</ItemCard.Title>
      <ItemCard.Meta label={deployment ?? "No Environment"}>
        <StatusChip type={type} size="small" id={statusId} />
      </ItemCard.Meta>
      <ItemCard.Footer timestamp={createdOn}>
        <PriorityChip size="small" id={severityId} />
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
