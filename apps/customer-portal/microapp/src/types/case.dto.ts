import type { Pagination } from "@src/types";

export interface CasesDTO extends Pagination {
  cases: CaseDTO[];
}

interface CaseDTO {
  id: string;
  internalId: string;
  number: string;
  createdOn: string;
  title: string;
  description?: string;
  assignedEngineer?: EntityReference;
  project: EntityReference;
  deployedProduct?: EntityReference;
  issueType?: EntityReference;
  deployment?: EntityReference;
  severity?: EntityReference;
  status?: EntityReference;
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
    caseTypeIds?: string[];
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
