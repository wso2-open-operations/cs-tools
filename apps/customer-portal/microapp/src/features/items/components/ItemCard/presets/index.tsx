import { ItemCard } from "@features/items/components";
import type { CaseSummary } from "@features/cases/types";
import type { ChangeRequestSummary } from "@features/changes/types";
import type { Chat } from "@features/chats/types";
import type { ServiceRequestSummary } from "@features/service-requests/types";
import { CASE_TYPE_CONFIGS, CASE_TYPES } from "@shared/constants";
import { PriorityChip, StatusChip } from "@shared/components/support";

export function CaseItemCard({ to, ...props }: CaseSummary & { to: string }) {
    const { icon, color } = CASE_TYPE_CONFIGS[CASE_TYPES.DEFAULT];
    return (
      <ItemCard.Root to={to}>
        <ItemCard.Header
          icon={icon}
          iconColor={color}
          number={props.number}
          internalId={props.internalId}
          chips={<PriorityChip size="small" id={props.severityId} />}
          status={<StatusChip type={CASE_TYPES.DEFAULT} size="small" id={props.statusId} />}
        />
        <ItemCard.Body title={props.title} description={props.description} />
        <ItemCard.Footer timestamp={props.createdOn} label={props.assigned ?? "Not Assigned"} />
      </ItemCard.Root>
    );
  }
   
  export function ChatItemCard({ to, ...props }: Chat & { to: string }) {
    const { icon, color } = CASE_TYPE_CONFIGS[CASE_TYPES.CHAT];
    return (
      <ItemCard.Root to={to}>
        <ItemCard.Header
          icon={icon}
          iconColor={color}
          number={props.number}
          status={<StatusChip type={CASE_TYPES.CHAT} size="small" id={props.statusId} />}
        />
        <ItemCard.Body title={props.description} />
        <ItemCard.Footer timestamp={props.createdOn} label={`${props.count} messages`} />
      </ItemCard.Root>
    );
  }
   
  export function ServiceRequestItemCard({ to, ...props }: ServiceRequestSummary & { to: string }) {
    const { icon, color } = CASE_TYPE_CONFIGS[CASE_TYPES.SERVICE_REQUEST];
    return (
      <ItemCard.Root to={to}>
        <ItemCard.Header
          icon={icon}
          iconColor={color}
          number={props.number}
          internalId={props.internalId}
          chips={<PriorityChip size="small" id={props.severityId} />}
          status={<StatusChip type={CASE_TYPES.SERVICE_REQUEST} size="small" id={props.statusId} />}
        />
        <ItemCard.Body title={props.title} description={props.description} />
        <ItemCard.Footer timestamp={props.createdOn} label={props.issueType ?? "Unspecified"} />
      </ItemCard.Root>
    );
  }
   
  export function ChangeRequestItemCard({ to, ...props }: ChangeRequestSummary & { to: string }) {
    const { icon, color } = CASE_TYPE_CONFIGS[CASE_TYPES.CHANGE_REQUEST];
    return (
      <ItemCard.Root to={to}>
        <ItemCard.Header
          icon={icon}
          iconColor={color}
          number={props.number}
          internalId={props.internalId}
          chips={<PriorityChip type={CASE_TYPES.CHANGE_REQUEST} size="small" id={props.impactId} />}
          status={<StatusChip type={CASE_TYPES.CHANGE_REQUEST} size="small" id={props.statusId} />}
        />
        <ItemCard.Body title={props.title} description={props.description} />
        <ItemCard.ScheduledDate date={props.endDate} />
        <ItemCard.Footer timestamp={props.createdOn} label={props.assignedTeam ?? "N/A"} />
      </ItemCard.Root>
    );
  }
   
  export function SecurityReportAnalysisItemCard({ to, ...props }: CaseSummary & { to: string }) {
    const { icon, color } = CASE_TYPE_CONFIGS[CASE_TYPES.SECURITY_REPORT_ANALYSIS];
    return (
      <ItemCard.Root to={to}>
        <ItemCard.Header
          icon={icon}
          iconColor={color}
          number={props.number}
          internalId={props.internalId}
          chips={<PriorityChip size="small" id={props.severityId} />}
          status={<StatusChip type={CASE_TYPES.SECURITY_REPORT_ANALYSIS} size="small" id={props.statusId} />}
        />
        <ItemCard.Body title={props.title} description={props.description} />
        <ItemCard.Footer timestamp={props.createdOn} label={props.createdBy} />
      </ItemCard.Root>
    );
  }
   
  export function EngagementItemCard({ to, ...props }: CaseSummary & { to: string }) {
    const { icon, color } = CASE_TYPE_CONFIGS[CASE_TYPES.ENGAGEMENT];
    return (
      <ItemCard.Root to={to}>
        <ItemCard.Header
          icon={icon}
          iconColor={color}
          number={props.number}
          internalId={props.internalId}
          status={<StatusChip type={CASE_TYPES.ENGAGEMENT} size="small" id={props.statusId} />}
        />
        <ItemCard.Body title={props.title} description={props.description} />
        <ItemCard.Footer timestamp={props.createdOn} label={props.engagementType ?? "Unspecified"} />
      </ItemCard.Root>
    );
  }
   
  export function AnnouncementItemCard({ to, ...props }: CaseSummary & { to: string }) {
    const { icon, color } = CASE_TYPE_CONFIGS[CASE_TYPES.ANNOUNCEMENT];
    return (
      <ItemCard.Root to={to}>
        <ItemCard.Header
          icon={icon}
          iconColor={color}
          number={props.number}
        />
        <ItemCard.Body title={props.title} />
        <ItemCard.Footer timestamp={props.createdOn} label="">
          <StatusChip type={CASE_TYPES.ANNOUNCEMENT} size="small" id={props.statusId} />
        </ItemCard.Footer>
      </ItemCard.Root>
    );
  }
   