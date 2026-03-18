import type { Pagination } from "@src/types";

export interface CasesDTO extends Pagination {
  cases: CaseSummaryDTO[];
}

export interface CaseSummaryDTO {
  id: string;
  internalId: string;
  number: string;
  createdOn: string;
  title: string;
  description?: string;
  assignedEngineer?: EntityReference;
  project: EntityReference;
  type?: EntityReference;
  deployedProduct?: EntityReference;
  parentCase?: EntityReference;
  chat?: EntityReference;
  issueType?: EntityReference;
  deployment?: EntityReference;
  severity?: EntityReference;
  status?: EntityReference;
}

export interface CaseDTO {
  updatedOn: string;
  slaResponseTime: string;
  deployedProduct: (EntityReference & { version: string | null }) | null;
  account: (EntityReference & { type: string | null; count: number | null }) | null;
  csManager: (EntityReference & { email: string | null }) | null;
  closedOn: string | null;
  closedBy: (EntityReference & { count: number | null }) | null;
  closeNotes: string | null;
  hasAutoClosed: boolean | null;
  id: string;
  internalId: string;
  number: string;
  createdOn: string;
  title: string;
  description: string;
  assignedEngineer: EntityReference | null;
  project: EntityReference | null;
  type: EntityReference | null;
  product: EntityReference | null;
  parentCase: EntityReference | null;
  conversation: EntityReference | null;
  issueType: EntityReference | null;
  deployment: (EntityReference & { type: string }) | null;
  severity: EntityReference | null;
  status: EntityReference | null;
}

export interface CasesFiltersDTO {
  caseStates: EntityReference[];
  severities: EntityReference[];
  issueTypes: EntityReference[];
  deploymentTypes: EntityReference[];
  callRequestStates: EntityReference[];
  changeRequestStates: EntityReference[];
  changeRequestImpacts: EntityReference[];
  caseTypes: EntityReference[];
  severityBasedAllocationTime: {
    "0": number;
    "10": number;
    "11": number;
    "12": number;
    "13": number;
    "14": number;
  };
}

export interface EntityReference {
  id: string;
  label: string;
}

export interface GetCasesRequestDTO {
  filters?: {
    caseTypes?: string[];
    createdByMe?: boolean;
    deploymentId?: string;
    issueId?: number;
    searchQuery?: string;
    severityId?: number;
    statusIds?: number[];
  };
  pagination?: {
    limit?: number;
    offset?: number;
  };
  sortBy?: {
    field?: string;
    order?: "asc" | "desc";
  };
}

export interface CreateCaseRequestDTO {
  projectId: string;
  deploymentId: string;
  productId: string;
  title: string;
  description: string;
  issueTypeKey: number;
  severityKey: number;
}

export interface CreateCaseResponseDTO {
  id: string;
  internalId: string;
  number: number;
  createdBy: string;
  createdOn: string;
  state: EntityReference;
  type: EntityReference;
}

export interface CaseClassificationRequestDTO {
  chatHistory: string;
  envProducts: Record<string, string[]>;
  region: string;
  tier: string;
}

export interface CaseClassificationResponseDTO {
  issueType: string;
  severityLevel: string;
  caseInfo: {
    description: string;
    shortDescription: string;
    productName: string;
    productVersion: string;
    environment: string;
    tier: string;
    region: string;
  };
}

export interface CasesStatsDTO {
  totalCases: number;
  averageResponseTime: number;
  resolvedCases: {
    total: number;
    currentMonth: number;
  };
  stateCount: { id: string; label: string; count: number }[];
  severityCount: { id: string; label: string; count: number }[];
  outstandingSeverityCount: { id: string; label: string; count: number }[];
  caseTypeCount: { id: string; label: string; count: number }[];
  casesTrend: { period: string; severities: { id: string; label: string; count: number }[] }[];
}

export interface CommentsDTO extends Pagination {
  comments: CommentDTO[];
}

export interface CommentDTO {
  id: string;
  content: string;
  type: string;
  createdOn: string;
  createdBy: string;
  isEscalated: string;
  hasInlineAttachments: boolean;
  inlineAttachments: {
    id: string;
    fileName: string;
    contentType: string;
    downloadUrl: string;
    createdOn: string;
    createdBy: string;
  }[];
}

export interface CreateCommentRequestDTO {
  content: string;
  type: "work_note" | "comments";
}
