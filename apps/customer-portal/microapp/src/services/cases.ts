import apiClient from "@src/services/apiClient";
import { infiniteQueryOptions, mutationOptions, queryOptions } from "@tanstack/react-query";
import { stripHtmlTags } from "@utils/others";
import type {
  CaseSummary,
  CaseClassificationRequestDTO,
  CaseClassificationResponseDTO,
  CasesDTO,
  CasesFiltersDTO,
  CasesStatsDTO,
  CreateCaseRequestDTO,
  CreateCaseResponseDTO,
  GetCasesRequestDTO,
  Case,
  CaseDTO,
  CommentsDTO,
  CommentDTO,
  Comment,
  CreateCommentRequestDTO,
  PaginatedArray,
} from "@src/types";

import {
  CASE_CLASSIFICATION_ENDPOINT,
  CASE_COMMENTS_ENDPOINT,
  CASE_DETAILS_ENDPOINT,
  CASE_STATS_ENDPOINT,
  CREATE_CASE_ENDPOINT,
  PROJECT_CASES_ENDPOINT,
  PROJECT_CASES_FILTERS_ENDPOINT,
} from "@config/endpoints";

export const getAllCases = async (id: string, body: GetCasesRequestDTO = {}): Promise<PaginatedArray<CaseSummary>> => {
  const response = (
    await apiClient.post<CasesDTO>(PROJECT_CASES_ENDPOINT(id), {
      ...body,
      filters: {
        ...(body?.filters ?? {}),
        caseTypes: ["default_case"],
      },
    })
  ).data;
  const result = response.cases.map(toCaseSummary) as PaginatedArray<CaseSummary>;
  result.pagination = {
    totalRecords: response.totalRecords,
    offset: response.offset,
    limit: response.limit,
  };

  return result;
};

const getCase = async (id: string): Promise<Case> => {
  const response = (await apiClient.get<CaseDTO>(CASE_DETAILS_ENDPOINT(id))).data;
  return toCase(response);
};

const getFilters = async (id: string): Promise<CasesFiltersDTO> => {
  return (await apiClient.get<CasesFiltersDTO>(PROJECT_CASES_FILTERS_ENDPOINT(id))).data;
};

const createCase = async (body: CreateCaseRequestDTO): Promise<CreateCaseResponseDTO> => {
  return (await apiClient.post<CreateCaseResponseDTO>(CREATE_CASE_ENDPOINT, body)).data;
};

const classify = async (
  props: Omit<CaseClassificationRequestDTO, "region" | "tier">,
): Promise<CaseClassificationResponseDTO> => {
  return (
    await apiClient.post<CaseClassificationResponseDTO>(CASE_CLASSIFICATION_ENDPOINT, {
      ...props,
      region: "EU", // TODO: Remove hardcoded
      tier: "Tier 1", // TODO: Remove hardcoded
    })
  ).data;
};

const getCasesStats = async (id: string): Promise<CasesStatsDTO> => {
  return (await apiClient.get<CasesStatsDTO>(CASE_STATS_ENDPOINT(id))).data;
};

const getComments = async (id: string): Promise<Comment[]> => {
  const response = (await apiClient.get<CommentsDTO>(CASE_COMMENTS_ENDPOINT(id))).data;
  return response.comments.map(toComment);
};

const createComment = async (id: string, body: CreateCommentRequestDTO): Promise<Comment> => {
  const response = (await apiClient.post<CommentDTO>(CASE_COMMENTS_ENDPOINT(id), body)).data;
  return toComment(response);
};

/* Mappers */
export function toCaseSummary(dto: CasesDTO["cases"][number]): CaseSummary {
  return {
    id: dto.id,
    internalId: dto.internalId,
    number: dto.number,
    createdOn: new Date(dto.createdOn.replace(" ", "T")),
    title: dto.title,
    description: dto.description ?? "",
    assigned: dto.assignedEngineer?.label,
    statusId: dto.status?.id,
    severityId: dto.severity?.id,
    issueTypeId: dto.issueType?.id,
  };
}

function toCase(dto: CaseDTO): Case {
  return {
    id: dto.id,
    internalId: dto.internalId,
    number: dto.number,
    createdOn: new Date(dto.createdOn.replace(" ", "T")),
    updatedOn: new Date(dto.updatedOn.replace(" ", "T")),
    title: dto.title,
    description: stripHtmlTags(dto.description) ?? "",
    assigned: dto.assignedEngineer?.label,
    statusId: dto.status?.id,
    severityId: dto.severity?.id,
    issueTypeId: dto.issueType?.id,
    product: dto.deployedProduct?.label,
    productVersion: dto.deployedProduct?.version ?? undefined,
    deployment: dto.deployment?.label ?? undefined,
    reporter: dto.csManager?.label,
    account: dto.account?.label,
    parentCaseId: dto.parentCase?.id,
    conversationId: dto.parentCase?.id,
    slaResponseTime: dto.slaResponseTime,
  };
}

export function toComment(dto: CommentDTO): Comment {
  return {
    id: dto.id,
    content: stripHtmlTags(dto.content),
    createdOn: new Date(dto.createdOn.replace(" ", "T")),
    createdBy: dto.createdBy,
    attachments: dto.inlineAttachments.map((attachment) => ({
      id: attachment.id,
      fileName: attachment.fileName,
      downloadUrl: attachment.downloadUrl,
      createdOn: new Date(attachment.createdOn.replace(" ", "T")),
      createdBy: attachment.createdBy,
    })),
  };
}

/* Query Options */
export const cases = {
  get: (id: string) => queryOptions({ queryKey: ["case", id], queryFn: () => getCase(id) }),

  all: (id: string, body: GetCasesRequestDTO = {}) =>
    queryOptions({
      queryKey: ["cases", id, body],
      queryFn: () => getAllCases(id, body),
    }),

  paginated: (id: string, body: GetCasesRequestDTO = {}) =>
    infiniteQueryOptions({
      queryKey: ["cases", "paginated", id, body],
      queryFn: ({ pageParam }) => getAllCases(id, { ...body, pagination: { ...body.pagination, offset: pageParam } }),
      initialPageParam: 0,
      getNextPageParam: (lastPage) => {
        const { offset, limit, totalRecords } = lastPage.pagination;
        const maxOffset = Math.ceil(totalRecords / limit);
        return offset >= maxOffset ? undefined : offset + 1;
      },
    }),

  filters: (id: string) =>
    queryOptions({
      queryKey: ["filters", id],
      queryFn: () => getFilters(id),
    }),

  create: mutationOptions({
    mutationFn: (body: CreateCaseRequestDTO) => createCase(body),
  }),

  classify: mutationOptions({
    mutationFn: (body: Omit<CaseClassificationRequestDTO, "region" | "tier">) => classify(body),
  }),

  stats: (id: string) =>
    queryOptions({
      queryKey: ["cases-stats", id],
      queryFn: () => getCasesStats(id),
      staleTime: 0,
      gcTime: 0,
    }),

  comments: (id: string) => queryOptions({ queryKey: ["comments", id], queryFn: () => getComments(id) }),
  createComment: (id: string) =>
    mutationOptions({
      mutationFn: (body: CreateCommentRequestDTO) => createComment(id, body),
    }),
};
