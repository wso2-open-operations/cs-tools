import apiClient from "@src/services/apiClient";
import type { PaginatedArray, GetChangeRequestsRquestDTO, ChangeRequestSummary, ChangeRequestsDTO } from "@src/types";
import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";

import { PROJECT_CHANGE_REQUESTS_ENDPOINT } from "@src/config/endpoints";

const getAllChangeRequests = async (
  id: string,
  body: GetChangeRequestsRquestDTO = {},
): Promise<PaginatedArray<ChangeRequestSummary>> => {
  const response = (await apiClient.post<ChangeRequestsDTO>(PROJECT_CHANGE_REQUESTS_ENDPOINT(id), body)).data;
  const result = response.changeRequests.map(toChangeRequestSummary) as PaginatedArray<ChangeRequestSummary>;
  result.pagination = {
    totalRecords: response.totalRecords,
    offset: response.offset,
    limit: response.limit,
  };

  return result;
};

/* Mappers */
function toChangeRequestSummary(dto: ChangeRequestsDTO["changeRequests"][number]): ChangeRequestSummary {
  return {
    id: dto.id,
    number: dto.number,
    title: dto.title,
    description: dto.case?.label ?? "",
    requestType: dto.type?.label,
    impactId: dto.impact?.id,
    statusId: dto.state?.id,
    owner: dto.assignedTeam?.label,
    scheduledOn: dto.endDate ? new Date(dto.endDate.replace(" ", "T")) : undefined,
    createdOn: new Date(dto.createdOn.replace(" ", "T")),
    updatedOn: new Date(dto.updatedOn.replace(" ", "T")),
  };
}

/* Query Options */
export const changeRequests = {
  all: (id: string, body: GetChangeRequestsRquestDTO = {}) =>
    queryOptions({
      queryKey: ["change-requests", id, body],
      queryFn: () => getAllChangeRequests(id, body),
    }),

  paginated: (id: string, body: GetChangeRequestsRquestDTO = {}) =>
    infiniteQueryOptions({
      queryKey: ["change-requests", "paginated", id, body],
      queryFn: ({ pageParam }) =>
        getAllChangeRequests(id, { ...body, pagination: { ...body.pagination, offset: pageParam } }),
      initialPageParam: 0,
      getNextPageParam: (lastPage) => {
        const { offset, limit, totalRecords } = lastPage.pagination;
        const maxOffset = Math.ceil(totalRecords / limit);
        return offset >= maxOffset ? undefined : offset + 1;
      },
    }),
};
