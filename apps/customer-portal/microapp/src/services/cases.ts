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

import apiClient from "@src/services/apiClient";
import { infiniteQueryOptions, mutationOptions, queryOptions } from "@tanstack/react-query";
import type {
  CaseSummary,
  CaseClassificationRequestDto,
  CaseClassificationResponseDto,
  CasesDto,
  CasesFiltersDto,
  CasesStatsDto,
  CreateCaseRequestDto,
  CreateCaseResponseDto,
  GetCasesRequestDto,
  Case,
  CaseDto,
  CommentsDto,
  CommentDto,
  Comment,
  CreateCommentRequestDto,
  PaginatedArray,
  GetCasesStatsRequestDto,
  EditCaseResponseDto,
  EditCaseRequestDto,
  Attachment,
  AttachmentsDto,
  AttachmentDto,
  Pagination,
} from "@src/types";

import {
  ATTACHMENT_DETAIL_ENDPOINT,
  CASE_ATTACHMENTS_ENDPOINT,
  CASE_CLASSIFICATION_ENDPOINT,
  CASE_COMMENTS_ENDPOINT,
  CASE_DETAILS_ENDPOINT,
  CASE_STATS_ENDPOINT,
  CREATE_CASE_ENDPOINT,
  PROJECT_CASES_ENDPOINT,
  PROJECT_CASES_FILTERS_ENDPOINT,
} from "@config/endpoints";

export const getAllCases = async (id: string, body: GetCasesRequestDto = {}): Promise<PaginatedArray<CaseSummary>> => {
  const response = (
    await apiClient.post<CasesDto>(PROJECT_CASES_ENDPOINT(id), {
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
  const response = (await apiClient.get<CaseDto>(CASE_DETAILS_ENDPOINT(id))).data;
  return toCase(response);
};

const editCase = async (id: string, body: EditCaseRequestDto): Promise<EditCaseResponseDto> => {
  const response = await apiClient.patch<EditCaseResponseDto>(CASE_DETAILS_ENDPOINT(id), body);
  return response.data;
};

const getFilters = async (id: string): Promise<CasesFiltersDto> => {
  return (await apiClient.get<CasesFiltersDto>(PROJECT_CASES_FILTERS_ENDPOINT(id))).data;
};

const createCase = async (body: CreateCaseRequestDto): Promise<CreateCaseResponseDto> => {
  return (await apiClient.post<CreateCaseResponseDto>(CREATE_CASE_ENDPOINT, body)).data;
};

const classify = async (
  props: Omit<CaseClassificationRequestDto, "region" | "tier">,
): Promise<CaseClassificationResponseDto> => {
  return (
    await apiClient.post<CaseClassificationResponseDto>(CASE_CLASSIFICATION_ENDPOINT, {
      ...props,
      region: "EU", // TODO: Remove hardcoded
      tier: "Tier 1", // TODO: Remove hardcoded
    })
  ).data;
};

const getCasesStats = async (id: string, body: Partial<GetCasesStatsRequestDto>): Promise<CasesStatsDto> => {
  return (
    await apiClient.get<CasesStatsDto>(CASE_STATS_ENDPOINT(id), {
      params: { ...body, caseTypes: body.caseTypes?.join(",") },
    })
  ).data;
};

const getComments = async (id: string): Promise<Comment[]> => {
  const response = (await apiClient.get<CommentsDto>(CASE_COMMENTS_ENDPOINT(id))).data;
  return response.comments.map(toComment);
};

const createComment = async (id: string, body: CreateCommentRequestDto): Promise<Comment> => {
  const response = (await apiClient.post<CommentDto>(CASE_COMMENTS_ENDPOINT(id), body)).data;
  return toComment(response);
};

const getAttachments = async (id: string, body: Partial<Omit<Pagination, "totalRecords">>): Promise<Attachment[]> => {
  const response = (await apiClient.get<AttachmentsDto>(CASE_ATTACHMENTS_ENDPOINT(id), { params: body })).data;
  return response.attachments.map(toAttachment);
};

const getAttachment = async (id: string): Promise<{ content: string }> => {
  const response = (await apiClient.get<{ content: string }>(ATTACHMENT_DETAIL_ENDPOINT(id))).data;
  return response;
};

/* Mappers */
export function toCaseSummary(dto: CasesDto["cases"][number]): CaseSummary {
  return {
    id: dto.id,
    internalId: dto.internalId,
    number: dto.number,
    createdOn: new Date(dto.createdOn.replace(" ", "T")),
    createdBy: dto.createdBy,
    title: dto.title,
    description: dto.description ?? "",
    assigned: dto.assignedEngineer?.label,
    statusId: dto.status.id,
    severityId: dto.severity?.id,
    issueTypeId: dto.issueType?.id,
    deployment: dto.deployment?.label,
    engagementType: dto.engagementType?.label,
  };
}

export function toCase(dto: CaseDto): Case {
  return {
    id: dto.id,
    internalId: dto.internalId,
    number: dto.number,
    createdOn: new Date(dto.createdOn.replace(" ", "T")),
    createdBy: dto.createdBy,
    updatedOn: new Date(dto.updatedOn.replace(" ", "T")),
    title: dto.title,
    description: dto.description ?? "",
    assigned: dto.assignedEngineer?.label,
    statusId: dto.status.id,
    severityId: dto.severity?.id,
    issueTypeId: dto.issueType?.id,
    product: dto.deployedProduct ? `${dto.deployedProduct?.label} ${dto.deployedProduct?.version}` : undefined,
    deployment: dto.deployment?.label ?? undefined,
    reporter: dto.createdBy,
    account: dto.account?.label,
    parentCaseId: dto.parentCase?.id,
    conversationId: dto.conversation?.id,
    slaResponseTime: dto.slaResponseTime,
  };
}

export function toComment(dto: CommentDto): Comment {
  return {
    id: dto.id,
    content: dto.content,
    createdOn: new Date(dto.createdOn.replace(" ", "T")),
    createdBy: dto.createdBy,
    attachments: dto.inlineAttachments.map((attachment) => ({
      id: attachment.id,
      type: "others",
      fileName: attachment.fileName,
      downloadUrl: attachment.downloadUrl,
      createdOn: new Date(attachment.createdOn.replace(" ", "T")),
      createdBy: attachment.createdBy,
    })),
  };
}

export function toAttachment(dto: AttachmentDto): Attachment {
  return {
    id: dto.id,
    type: /^image\//.test(dto.type) ? "image" : "others",
    fileName: dto.name,
    downloadUrl: dto.downloadUrl,
    createdOn: new Date(dto.createdOn.replace(" ", "T")),
    createdBy: dto.createdBy,
  };
}

/* Query Options */
export const cases = {
  get: (id: string) => queryOptions({ queryKey: ["case", id], queryFn: () => getCase(id) }),

  edit: (id: string) =>
    mutationOptions({
      mutationFn: (body: EditCaseRequestDto) => editCase(id, body),
    }),

  all: (id: string, body: GetCasesRequestDto = {}) =>
    queryOptions({
      queryKey: ["cases", id, body],
      queryFn: () => getAllCases(id, body),
    }),

  paginated: (id: string, body: GetCasesRequestDto = {}) =>
    infiniteQueryOptions({
      queryKey: ["cases", "paginated", id, body],
      queryFn: ({ pageParam }) => getAllCases(id, { ...body, pagination: { ...body.pagination, offset: pageParam } }),
      initialPageParam: 0,
      getNextPageParam: (lastPage) => {
        const { offset, limit, totalRecords } = lastPage.pagination;
        const nextOffset = offset + 1;
        const totalPages = Math.ceil(totalRecords / limit);
        return nextOffset >= totalPages ? undefined : nextOffset;
      },
    }),

  filters: (id: string) =>
    queryOptions({
      queryKey: ["filters", id],
      queryFn: () => getFilters(id),
    }),

  create: mutationOptions({
    mutationFn: (body: CreateCaseRequestDto) => createCase(body),
  }),

  classify: mutationOptions({
    mutationFn: (body: Omit<CaseClassificationRequestDto, "region" | "tier">) => classify(body),
  }),

  stats: (id: string, body: Partial<GetCasesStatsRequestDto> = {}) =>
    queryOptions({
      queryKey: ["cases-stats", id, body],
      queryFn: () => getCasesStats(id, body),
    }),

  comments: (id: string) => queryOptions({ queryKey: ["comments", id], queryFn: () => getComments(id) }),
  createComment: (id: string) =>
    mutationOptions({
      mutationFn: (body: CreateCommentRequestDto) => createComment(id, body),
    }),

  attachments: (id: string, body: Partial<Omit<Pagination, "totalRecords">> = {}) =>
    queryOptions({ queryKey: ["cases", id, "attachments"], queryFn: () => getAttachments(id, body) }),

  attachment: (id: string) => queryOptions({ queryKey: ["attachment", id], queryFn: () => getAttachment(id) }),
};
