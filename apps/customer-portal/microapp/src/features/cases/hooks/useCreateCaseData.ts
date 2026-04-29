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

import { useMemo } from "react";
import { useQuery, useSuspenseQuery } from "@tanstack/react-query";
import { useProject } from "@context/project";
import { useFilters } from "@context/filters";
import { projects } from "@features/projects/api/projects.queries";
import { overrideOrDefault } from "@shared/utils/string.utils";
import { DEPLOYMENT_DISABLED_PROJECT_TYPES } from "@config/constants";

export function useCreateCaseData(formProjectId: string, formDeploymentId: string) {
  const { type } = useProject();
  const { data: filters } = useFilters();
  const { data: features } = useQuery(projects.features(formProjectId));

  const deploymentQuery = useQuery({
    ...projects.deployments(formProjectId),
    enabled: !!formProjectId,
  });

  const productQuery = useQuery({
    ...projects.products(formDeploymentId, {
      filters: { productCategories: features?.defaultCaseProductCategories ?? undefined },
    }),
    enabled: !!formDeploymentId,
  });

  const projectsOptions = useSuspenseQuery(projects.all()).data.map((project) => ({
    value: project.id,
    label: project.name,
  }));

  const deploymentsFieldDisabled = type ? DEPLOYMENT_DISABLED_PROJECT_TYPES.includes(type) : false;

  const deploymentOptions = useMemo(() => {
    const data = deploymentQuery.data ?? [];
    const filteredData = deploymentsFieldDisabled
      ? data.filter((deployment) => deployment.type === "Primary Production").slice(0, 1)
      : data;
    return filteredData.map((deployment) => ({
      value: deployment.id,
      label: deployment.name,
    }));
  }, [deploymentQuery.data, deploymentsFieldDisabled]);

  const productOptions = useMemo(
    () =>
      productQuery.data?.map((product) => ({
        value: product.id,
        label: `${product.name} ${product.version}`,
      })) ?? [],
    [productQuery.data],
  );

  const issueTypeOptions = filters?.issueTypes.map((issueType) => ({
    value: Number(issueType.id),
    label: issueType.label,
  }));

  const severityLevelOptions = filters?.severities.map((severity) => ({
    value: Number(severity.id),
    label: overrideOrDefault(severity.label),
  }));

  return {
    projectsOptions,
    deploymentOptions,
    productOptions,
    issueTypeOptions,
    severityLevelOptions,
    deploymentsFieldDisabled,
    isLoadingDeployments: deploymentQuery.isLoading,
    isLoadingProducts: productQuery.isLoading,
  };
}
