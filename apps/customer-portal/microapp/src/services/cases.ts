import apiClient from "@src/services/apiClient";
import { mutationOptions, queryOptions } from "@tanstack/react-query";
import type {
  Case,
  CaseClassificationRequestDTO,
  CaseClassificationResponseDTO,
  CasesDTO,
  CasesFiltersDTO,
  CreateCaseRequestDTO,
  CreateCaseResponseDTO,
  GetCasesRequestDTO,
} from "@src/types";

import {
  CASE_CLASSIFICATION_ENDPOINT,
  CREATE_CASE_ENDPOINT,
  PROJECT_CASES_ENDPOINT,
  PROJECT_CASES_FILTERS_ENDPOINT,
} from "@config/endpoints";

const getCases = async (id: string, body: GetCasesRequestDTO = {}): Promise<Case[]> => {
  const cases = (await apiClient.post<CasesDTO>(PROJECT_CASES_ENDPOINT(id), body)).data.cases;
  return cases.map(toCase);
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
  // TODO: Remove any hardcoded values
  return (
    await apiClient.post<CaseClassificationResponseDTO>(CASE_CLASSIFICATION_ENDPOINT, {
      ...props,
      region: "EU",
      tier: "Tier 1",
    })
  ).data;
};

/* Mappers */
function toCase(dto: CasesDTO["cases"][number]): Case {
  return {
    id: dto.id,
    internalId: dto.internalId,
    number: dto.number,
    createdOn: new Date(dto.createdOn.replace(" ", "T")),
    title: dto.title,
    description: dto.description,
    assigned: dto.assignedEngineer?.label,
    statusId: dto.status?.id,
    severityId: dto.severity?.id,
    issueTypeId: dto.issueType?.id,
  };
}

/* Query Options */
export const cases = {
  all: (id: string, body: GetCasesRequestDTO = {}) =>
    queryOptions({
      queryKey: ["cases", id, body],
      queryFn: () => getCases(id, body),
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
};
