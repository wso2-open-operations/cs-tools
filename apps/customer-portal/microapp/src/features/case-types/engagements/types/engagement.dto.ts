import type { EntityReference, Pagination } from "@shared/types";

export interface CallRequestsDto extends Pagination {
  callRequests: CallRequestDto[];
}

export interface CallRequestDto {
  id: string;
  createdOn: string;
  updatedOn: string;
  durationMin: number;
  scheduleTime: string;
  state: EntityReference;
  reason: string;
  preferredTimes: string[];
}
