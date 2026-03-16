import type { EntityReference, Pagination } from "@src/types";

export interface ChangeRequestsDTO extends Pagination {
  changeRequests: ChangeRequestDTO[];
}

interface ChangeRequestDTO {
  id: string;
  number: string;
  title: string;
  case: (EntityReference & { number: string }) | null;
  endDate: string | null;
  impact: EntityReference | null;
  state: EntityReference | null;
  type: EntityReference | null;
  assignedTeam: EntityReference | null;
  createdOn: string;
  updatedOn: string;
}

export interface GetChangeRequestsRquestDTO {
  filters?: {
    impactKey?: number;
    searchQuery?: string;
    stateKeys?: number[];
  };
  pagination?: {
    limit?: number;
    offset?: number;
  };
}
