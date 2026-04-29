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

import apiClient from "@infrastructure/api/client";
import type { Pagination, PaginatedArray } from "@shared/types";
import type {
  AttachmentsDto,
  CaseClassificationRequestDto,
  CaseClassificationResponseDto,
  CaseDto,
  CasesDto,
  CasesFiltersDto,
  CasesStatsDto,
  CommentDto,
  CommentsDto,
  CreateCaseRequestDto,
  CreateCaseResponseDto,
  EditCaseRequestDto,
  EditCaseResponseDto,
  GetCasesRequestDto,
  GetCasesStatsRequestDto,
  CreateCommentRequestDto,
} from "@features/cases/types/case.dto";
import type { Attachment, Case, CaseSummary, Comment } from "@features/cases/types/case.model";
import { toAttachment, toCase, toCaseSummary, toComment } from "@features/cases/mappers/case.mapper";
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
      filters: { ...(body?.filters ?? {}), caseTypes: ["default_case"] },
    })
  ).data;
  const result = response.cases.map(toCaseSummary) as PaginatedArray<CaseSummary>;
  result.pagination = { totalRecords: response.totalRecords, offset: response.offset, limit: response.limit };
  return result;
};

export const getCase = async (id: string): Promise<Case> => {
  const response = (await apiClient.get<CaseDto>(CASE_DETAILS_ENDPOINT(id))).data;
  return toCase(response);
};

export const editCase = async (id: string, body: EditCaseRequestDto): Promise<EditCaseResponseDto> => {
  return (await apiClient.patch<EditCaseResponseDto>(CASE_DETAILS_ENDPOINT(id), body)).data;
};

export const getCasesFilters = async (id: string): Promise<CasesFiltersDto> => {
  return (await apiClient.get<CasesFiltersDto>(PROJECT_CASES_FILTERS_ENDPOINT(id))).data;
};

export const createCase = async (body: CreateCaseRequestDto): Promise<CreateCaseResponseDto> => {
  return (await apiClient.post<CreateCaseResponseDto>(CREATE_CASE_ENDPOINT, body)).data;
};

export const classifyCase = async (
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

export const getCasesStats = async (id: string, body: Partial<GetCasesStatsRequestDto>): Promise<CasesStatsDto> => {
  return (
    await apiClient.get<CasesStatsDto>(CASE_STATS_ENDPOINT(id), {
      params: { ...body, caseTypes: body.caseTypes?.join(",") },
    })
  ).data;
};

export const getComments = async (id: string): Promise<Comment[]> => {
  const response = (await apiClient.get<CommentsDto>(CASE_COMMENTS_ENDPOINT(id))).data;
  return response.comments.map(toComment);
};

export const createComment = async (id: string, body: CreateCommentRequestDto): Promise<Comment> => {
  const response = (await apiClient.post<CommentDto>(CASE_COMMENTS_ENDPOINT(id), body)).data;
  return toComment(response);
};

export const getAttachments = async (
  id: string,
  body: Partial<Omit<Pagination, "totalRecords">>,
): Promise<Attachment[]> => {
  const response = (await apiClient.get<AttachmentsDto>(CASE_ATTACHMENTS_ENDPOINT(id), { params: body })).data;
  return response.attachments.map(toAttachment);
};

export const getAttachment = async (id: string): Promise<{ content: string }> => {
  return (await apiClient.get<{ content: string }>(ATTACHMENT_DETAIL_ENDPOINT(id))).data;
};
