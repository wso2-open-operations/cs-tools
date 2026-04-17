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

import type { CaseCommentInlineAttachment } from "@features/support/types/attachments";
import type {
  AuditMetadata,
  IdLabelRef,
  MetadataItem,
  SharedEnvContext,
  PaginationResponse,
  SearchRequestBase,
} from "@/types/common";
import type { TrendData } from "@features/dashboard/types/stats";

// Item type for severity level option for case creation form.
export type SeverityLevelOption = {
  id: string;
  label: string;
  description: string;
};

// Item type for conversation summary for case creation form.
export type CaseCreationConversationSummary = {
  messagesExchanged: number;
  troubleshootingAttempts: number;
  kbArticlesReviewed: number;
};

// Response type for case creation form metadata.
export type CaseCreationMetadata = {
  projects: string[];
  products: string[];
  deploymentTypes: string[];
  issueTypes: string[];
  severityLevels: SeverityLevelOption[];
  conversationSummary: CaseCreationConversationSummary;
};

// Item type for case severity.
export type CaseSeverity = {
  id: number;
  label: string;
  count: number;
};

// Item type for case state.
export type CaseState = {
  id: string;
  label: string;
  count: number;
};

// Item type for case type.
export type CaseType = {
  id: string;
  label: string;
  count: number;
};

// Item type for engagement type count.
export type EngagementTypeCount = {
  id: string;
  label: string;
  count: number;
};

// Item type for cases trend period.
export type CasesTrendPeriod = {
  period: string;
  severities: CaseSeverity[];
};

// Item type for resolved cases statistics.
export type ResolvedCasesStats = {
  total: number;
  currentMonth: number;
  pastThirtyDays?: number;
};

// Item type for cases change rate.
export type CasesChangeRate = {
  resolvedEngagements?: number;
  averageResponseTime?: number;
};

// Response type for project cases statistics.
export type ProjectCasesStats = {
  totalCases: number;
  totalCount?: number;
  activeCount?: number;
  outstandingCount?: number;
  averageResponseTime: number;
  resolvedCases: ResolvedCasesStats;
  changeRate?: CasesChangeRate;
  stateCount: CaseState[];
  severityCount: CaseSeverity[];
  outstandingSeverityCount: CaseSeverity[];
  caseTypeCount: CaseType[];
  casesTrend: CasesTrendPeriod[];
  engagementTypeCount?: EngagementTypeCount[];
  outstandingEngagementTypeCount?: EngagementTypeCount[];
};

// Model type for dashboard statistics.
export type DashboardStat = {
  value: number;
  trend: TrendData;
};

// Model type for dashboard response time statistics.
export type DashboardResponseTimeStat = {
  value: string;
  trend?: TrendData;
};

// Item type for cases trend chart rows.
export type CasesTrendChartRow = {
  name: string;
  TypeA: number;
  TypeB: number;
  TypeC: number;
  TypeD: number;
};

// Response type for dashboard mock statistics.
export type DashboardMockStats = {
  totalCases: DashboardStat;
  openCases: DashboardStat;
  resolvedCases: DashboardStat;
  avgResponseTime: DashboardResponseTimeStat;
  casesTrend: CasesTrendChartRow[];
};

// Item type for case list items.
export type CaseListItem = AuditMetadata & {
  id: string;
  internalId: string;
  number: string;
  title: string;
  description: string;
  assignedEngineer: string | IdLabelRef | null;
  project: IdLabelRef;
  issueType: IdLabelRef | null;
  deployedProduct: IdLabelRef | null;
  deployment: IdLabelRef | null;
  severity: IdLabelRef | null;
  status: IdLabelRef | null;
  type?: IdLabelRef | null;
  caseTypes?: IdLabelRef | null;
};

// Response type for case search results.
export type CaseSearchResponse = PaginationResponse & {
  cases: CaseListItem[];
  projects?: IdLabelRef[];
};

// Item type for account details within a case.
export type CaseDetailsAccount = {
  type: string | null;
  id: string;
  label: string;
};

// Item type for deployed product details within a case.
export type CaseDetailsDeployedProduct = IdLabelRef & {
  version?: string | null;
};

// Item type for user who closed a case.
export type CaseDetailsClosedBy = {
  id: string;
  label?: string | null;
  name?: string | null;
};

// Item type for assigned engineer details within a case.
export type CaseDetailsAssignedEngineer = {
  id: string;
  label?: string;
  name?: string;
};

// Item type for custom variables within a case.
export type CaseVariable = {
  name: string;
  value: string;
};

// Response type for detailed case information.
export type CaseDetails = AuditMetadata & {
  id: string;
  internalId: string;
  number: string | null;
  title: string | null;
  description: string | null;
  slaResponseTime: string | null;
  product: IdLabelRef | null;
  account: CaseDetailsAccount | null;
  csManager: IdLabelRef | string | null;
  assignedEngineer: string | CaseDetailsAssignedEngineer | null;
  project: IdLabelRef | null;
  type: IdLabelRef | null;
  deployedProduct: CaseDetailsDeployedProduct | null;
  parentCase: IdLabelRef | null;
  conversation: unknown;
  issueType: IdLabelRef | null;
  catalog?: IdLabelRef | null;
  catalogItem?: IdLabelRef | null;
  variables?: CaseVariable[];
  changeRequests?: IdLabelRef[];
  duration?: string | null;
  assignedTeam?: IdLabelRef | null;
  engagementStartDate?: string | null;
  engagementEndDate?: string | null;
  deployment: IdLabelRef | null;
  severity: IdLabelRef | null;
  status: IdLabelRef | null;
  closedOn: string | null;
  closedBy: CaseDetailsClosedBy | null;
  closeNotes: string | null;
  hasAutoClosed: boolean | null;
  engineerEmail: string | null;
  findingsResolved: number | null;
  findingsTotal: number | null;
};

// Item type for a single case comment.
export type CaseComment = AuditMetadata & {
  id: string;
  content: string;
  type: string;
  createdByFirstName?: string | null;
  createdByLastName?: string | null;
  isEscalated: boolean;
  hasInlineAttachments?: boolean;
  inlineAttachments?: CaseCommentInlineAttachment[];
};

// Response type for case comments list.
export type CaseCommentsResponse = PaginationResponse & {
  comments: CaseComment[];
};

// Response type for case metadata (fetching possible statuses, severities, types).
export type CaseMetadataResponse = {
  statuses?: MetadataItem[];
  caseStates?: MetadataItem[];
  severities?: MetadataItem[];
  issueTypes?: MetadataItem[];
  deploymentTypes?: MetadataItem[];
  callRequestStates?: MetadataItem[];
  changeRequestStates?: MetadataItem[];
  changeRequestImpacts?: MetadataItem[];
  caseTypes?: MetadataItem[];
  conversationStates?: MetadataItem[];
  timeCardStates?: MetadataItem[];
  severityBasedAllocationTime?: Record<string, number>;
};

// Model type for all cases filter values state.
export type AllCasesFilterValues = {
  [key: string]: string | undefined;
  statusId?: string;
  severityId?: string;
  issueTypes?: string;
  deploymentId?: string;
};

// Item type for a case attachment.
export type CaseAttachment = AuditMetadata & {
  id: string;
  name: string;
  type: string;
  description?: string | null;
  size?: number;
  sizeBytes?: string;
  content?: string | null;
  downloadUrl?: string | null;
};

// Response type for case attachments list.
export type CaseAttachmentsResponse = PaginationResponse & {
  attachments: CaseAttachment[];
};

// Model type for extracted case info from AI classification.
export type CaseClassificationInfo = {
  description: string;
  shortDescription: string;
  productName: string;
  productVersion: string;
  environment: string;
  tier: string;
  region: string;
};

// Response type for case classification.
export type CaseClassificationResponse = {
  issueType: string;
  severityLevel: string;
  caseInfo: CaseClassificationInfo;
};

// Response type for creating a support case.
export type CreateCaseResponse = AuditMetadata & {
  id: string;
  internalId?: string;
  number?: string;
  state?: IdLabelRef;
  type?: IdLabelRef;
};

// Filter type for searching cases.
export type CaseSearchFilters = {
  issueId?: number;
  deploymentId?: string;
  severityId?: number;
  statusId?: number;
  statusIds?: number[];
  searchQuery?: string;
  caseTypes?: string[];
  createdByMe?: boolean;
};

// Request type for searching cases.
export type CaseSearchRequest = SearchRequestBase & {
  filters?: CaseSearchFilters;
};

// Request type for case classification.
export type CaseClassificationRequest = SharedEnvContext & {
  chatHistory: string;
};

// Request type for patching a case.
export type PatchCaseRequest = {
  stateKey: number;
};

// Request type for creating a support case.
export type CreateCaseRequest = {
  attachments?: Array<{ file: string; name: string }>;
  type?: string;
  deploymentId: string;
  description: string;
  issueTypeKey?: number;
  deployedProductId: string;
  projectId: string;
  severityKey?: number;
  title: string;
  parentCaseId?: string;
  conversationId?: string;
};
