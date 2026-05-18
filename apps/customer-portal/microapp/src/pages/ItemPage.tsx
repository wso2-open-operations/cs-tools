import {
  CaseItemView,
  ChangeRequestItemView,
  ChatItemView,
  EngagementItemView,
  SecurityReportAnalysisItemView,
  ServiceRequestItemView,
} from "@features/detail/components";

import { CASE_TYPES } from "@shared/constants";
import type { CaseType } from "@shared/types";

export default function ItemPage({ type }: { type: CaseType }) {
  switch (type) {
    case CASE_TYPES.DEFAULT:
      return <CaseItemView />;
    case CASE_TYPES.CHAT:
      return <ChatItemView />;
    case CASE_TYPES.SERVICE_REQUEST:
      return <ServiceRequestItemView />;
    case CASE_TYPES.CHANGE_REQUEST:
      return <ChangeRequestItemView />;
    case CASE_TYPES.SECURITY_REPORT_ANALYSIS:
      return <SecurityReportAnalysisItemView />;
    case CASE_TYPES.ENGAGEMENT:
      return <EngagementItemView />;
    // case CASE_TYPES.ANNOUNCEMENT:
    //   return <AnnouncementItemsList key={type} />;
    default:
      return null;
  }
}
