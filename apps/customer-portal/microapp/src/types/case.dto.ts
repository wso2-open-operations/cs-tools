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
  statuses: EntityReference[];
  severities: EntityReference[];
  issueTypes: EntityReference[];
}

interface EntityReference {
  id: string;
  label: string;
}

export interface GetCasesRequestDTO {
  filters?: {
    deploymentId?: string;
    issueId?: number;
    searchQuery?: string;
    severityId?: number;
    statusId?: number;
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
