import type { EntityReference } from "./case.dto";
import type { Pagination } from "./pagination.types";

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
