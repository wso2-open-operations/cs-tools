import {
  AnnouncementItemCard,
  CaseItemCard,
  ChangeRequestItemCard,
  ChatItemCard,
  EngagementItemCard,
  SecurityReportAnalysisItemCard,
  ServiceRequestItemCard,
} from "@features/support/components";
import {
  useOutstandingAnnouncements,
  useOutstandingCases,
  useOutstandingChangeRequests,
  useOutstandingChats,
  useOutstandingEngagements,
  useOutstandingSecurityReportAnalysis,
  useOutstandingServiceRequests,
} from "@features/support/hooks";

import { EmptyState } from "@shared/components/common";

import { CASE_TYPES } from "@shared/constants";
import type { CaseType } from "@shared/types";

export function TabContent({ tab }: { tab: CaseType }) {
  switch (tab) {
    case CASE_TYPES.DEFAULT:
      return <CaseItemListContent />;
    case CASE_TYPES.CHAT:
      return <ChatItemListContent />;
    case CASE_TYPES.SERVICE_REQUEST:
      return <ServiceRequestItemListContent />;
    case CASE_TYPES.CHANGE_REQUEST:
      return <ChangeRequestItemListContent />;
    case CASE_TYPES.SECURITY_REPORT_ANALYSIS:
      return <SecurityReportAnalysisItemListContent />;
    case CASE_TYPES.ENGAGEMENT:
      return <EngagementItemListContent />;
    case CASE_TYPES.ANNOUNCEMENT:
      return <AnnouncementItemListContent />;
  }
}

function CaseItemListContent() {
  const { data } = useOutstandingCases();

  if (data.length === 0) return <EmptyState />;
  return data.map((item) => <CaseItemCard key={item.id} {...item} />);
}

function ChatItemListContent() {
  const { data } = useOutstandingChats();

  if (data.length === 0) return <EmptyState />;
  return data.map((item) => <ChatItemCard key={item.id} {...item} />);
}

function ServiceRequestItemListContent() {
  const { data } = useOutstandingServiceRequests();

  if (data.length === 0) return <EmptyState />;
  return data.map((item) => <ServiceRequestItemCard key={item.id} {...item} />);
}

function ChangeRequestItemListContent() {
  const { data } = useOutstandingChangeRequests();

  if (data.length === 0) return <EmptyState />;
  return data.map((item) => <ChangeRequestItemCard key={item.id} {...item} />);
}

function SecurityReportAnalysisItemListContent() {
  const { data } = useOutstandingSecurityReportAnalysis();

  if (data.length === 0) return <EmptyState />;
  return data.map((item) => <SecurityReportAnalysisItemCard key={item.id} {...item} />);
}

function EngagementItemListContent() {
  const { data } = useOutstandingEngagements();

  if (data.length === 0) return <EmptyState />;
  return data.map((item) => <EngagementItemCard key={item.id} {...item} />);
}

function AnnouncementItemListContent() {
  const { data } = useOutstandingAnnouncements();

  if (data.length === 0) return <EmptyState />;
  return data.map((item) => <AnnouncementItemCard key={item.id} {...item} />);
}
