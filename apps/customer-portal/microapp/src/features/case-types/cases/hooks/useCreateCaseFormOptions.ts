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
import { useFormikContext } from "formik";

import { useFilters } from "@context/filters";
import { useProject } from "@context/project";

import { DEPLOYMENT_DISABLED_PROJECT_TYPES } from "@config/constants";

import type { CreateCaseFormValues } from "@features/case-types/cases/hooks";
import { projects } from "@features/projects/api/projects.queries";

import { overrideOrDefault } from "@shared/utils/string.utils";

interface FieldOptions {
  options: { value: string | number; label: string }[];
  pending: boolean;
}

export interface CreateCaseFieldOptions {
  projects: FieldOptions;
  deployments: FieldOptions;
  products: FieldOptions;
  issueTypes: FieldOptions;
  severities: FieldOptions;
}

export function useCreateCaseFormOptions(): CreateCaseFieldOptions {
  const { type, features } = useProject();
  const { data: filters } = useFilters();
  const { values } = useFormikContext<CreateCaseFormValues>();

  const projectsOptions = useSuspenseQuery(projects.all()).data.map((p) => ({
    value: p.id,
    label: p.name,
  }));

  const { data: deployments, ...deploymentsQuery } = useQuery({
    ...projects.deployments(values.project),
    enabled: !!values.project,
  });

  const { data: products, ...productsQuery } = useQuery({
    ...projects.products(values.deployment, {
      filters: { productCategories: features?.defaultCaseProductCategories ?? undefined },
    }),
    enabled: !!values.deployment,
  });

  const deploymentOptions = useMemo(() => {
    const data = deployments ?? [];
    const filtered = (type ? DEPLOYMENT_DISABLED_PROJECT_TYPES.includes(type) : false)
      ? data.filter((deployment) => deployment.type === "Primary Production").slice(0, 1)
      : data;

    return filtered.map((deployment) => ({
      value: deployment.id,
      label: deployment.name,
    }));
  }, [deployments]);

  const productOptions = useMemo(
    () =>
      (products ?? []).map((product) => ({
        value: product.id,
        label: `${product.name} ${product.version}`,
      })) ?? [],
    [products],
  );

  const issueTypeOptions = useMemo(
    () => filters?.issueTypes.map((i) => ({ value: Number(i.id), label: i.label })) ?? [],
    [filters?.issueTypes],
  );

  const severityLevelOptions = useMemo(
    () => filters?.severities.map((s) => ({ value: Number(s.id), label: overrideOrDefault(s.label) })) ?? [],
    [filters?.severities],
  );

  return {
    projects: {
      options: projectsOptions,
      pending: false,
    },
    deployments: {
      options: deploymentOptions,
      pending: deploymentsQuery.isPending,
    },
    products: {
      options: productOptions,
      pending: productsQuery.isPending,
    },
    issueTypes: {
      options: issueTypeOptions,
      pending: false,
    },
    severities: {
      options: severityLevelOptions,
      pending: false,
    },
  };
}
