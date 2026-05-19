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
import { useCallback, useMemo } from "react";

import type { UseMutationResult } from "@tanstack/react-query";
import * as Yup from "yup";
import { useFormikContext, type FormikConfig } from "formik";

import { useAutoFill } from "@features/cases/hooks/useAutoFill";
import { useCreateCaseData } from "@features/cases/hooks/useCreateCaseData";
import type {
  CaseClassificationResponseDto,
  CreateCaseRequestDto,
  CreateCaseResponseDto,
} from "@features/cases/types/case.dto";
import type { Case } from "@features/cases/types/case.model";

export type CreateCaseFormValues = {
  project: string;
  product: string;
  deployment: string;
  type: string;
  severity: string;
  title: string;
  description: string;
};

export const createCaseValidationSchema = Yup.object({
  project: Yup.string().required("Project is required"),
  deployment: Yup.string().required("Deployment type is required"),
  product: Yup.string().when("deployment", {
    is: (deployment: string) => Boolean(deployment),
    then: (schema) => schema.required("Product & version is required"),
    otherwise: (schema) => schema.notRequired(),
  }),
  title: Yup.string()
    .trim()
    .min(5, "Title must be at least 5 characters")
    .max(200, "Title must be 200 characters or less")
    .required("Issue title is required"),
  description: Yup.string()
    .trim()
    .min(20, "Description must be at least 20 characters")
    .max(5000, "Description must be 5000 characters or less")
    .required("Case description is required"),
  type: Yup.number().typeError("Issue type is required").required("Issue type is required"),
  severity: Yup.number().typeError("Severity level is required").required("Severity level is required"),
});

export function createCaseFormInitialValues(projectId: string): CreateCaseFormValues {
  return {
    project: projectId,
    product: "",
    deployment: "",
    title: "",
    description: "",
    type: "",
    severity: "",
  };
}

export type CreateCaseMutation = UseMutationResult<CreateCaseResponseDto, Error, CreateCaseRequestDto>;

type UseCreateCaseFormConfigArgs = {
  projectId: string;
  relatedCase: Case | undefined;
  mutation: CreateCaseMutation;
};

export function useCreateCaseFormConfig({
  projectId,
  relatedCase,
  mutation,
}: UseCreateCaseFormConfigArgs): Pick<
  FormikConfig<CreateCaseFormValues>,
  "initialValues" | "validationSchema" | "onSubmit" | "validateOnBlur" | "validateOnChange"
> {
  return useMemo(
    () => ({
      initialValues: createCaseFormInitialValues(projectId),
      validationSchema: createCaseValidationSchema,
      validateOnBlur: true,
      validateOnChange: true,
      onSubmit: async (values: CreateCaseFormValues) => {
        await mutation.mutateAsync({
          type: "default_case",
          projectId: values.project,
          deploymentId: values.deployment,
          deployedProductId: values.product,
          title: values.title,
          description: values.description,
          issueTypeKey: Number(values.type),
          severityKey: Number(values.severity),
          relatedCaseId: relatedCase?.id,
        });
      },
    }),
    [projectId, mutation, relatedCase],
  );
}

type UseCreateCaseFormArgs = {
  classifications: CaseClassificationResponseDto | undefined;
  relatedCase: Case | undefined;
  mutation: CreateCaseMutation;
};

export function useCreateCaseForm({ classifications, relatedCase, mutation }: UseCreateCaseFormArgs) {
  const { values, setFieldValue, isSubmitting, handleSubmit } = useFormikContext<CreateCaseFormValues>();

  const {
    projectsOptions,
    deploymentOptions,
    productOptions,
    issueTypeOptions,
    severityLevelOptions,
    deploymentsFieldDisabled,
    isLoadingDeployments,
    isLoadingProducts,
  } = useCreateCaseData(values.project, values.deployment);

  const { classified, setClassified } = useAutoFill({
    classifications,
    relatedCase,
    deploymentOptions,
    productOptions,
    issueTypeOptions,
    severityLevelOptions,
    deploymentsFieldDisabled,
    setFieldValue,
  });

  const clearClassifiedKeys = useCallback((keys: string[]) => {
    setClassified((prev) => {
      const next = new Set(prev);
      keys.forEach((key) => next.delete(key));
      return next;
    });
  }, [setClassified]);

  const afterProjectChange = useCallback(() => {
    setFieldValue("deployment", "");
    setFieldValue("product", "");
    clearClassifiedKeys(["deployment", "product"]);
  }, [setFieldValue, clearClassifiedKeys]);

  const afterDeploymentChange = useCallback(() => {
    setFieldValue("product", "");
    clearClassifiedKeys(["deployment", "product"]);
  }, [setFieldValue, clearClassifiedKeys]);

  const afterProductChange = useCallback(() => {
    clearClassifiedKeys(["product"]);
  }, [clearClassifiedKeys]);

  const afterTitleChange = useCallback(() => {
    clearClassifiedKeys(["title"]);
  }, [clearClassifiedKeys]);

  const afterDescriptionChange = useCallback(() => {
    clearClassifiedKeys(["description"]);
  }, [clearClassifiedKeys]);

  const afterTypeChange = useCallback(() => {
    clearClassifiedKeys(["type"]);
  }, [clearClassifiedKeys]);

  const afterSeverityChange = useCallback(() => {
    clearClassifiedKeys(["severity"]);
  }, [clearClassifiedKeys]);

  const deploymentSelectDisabled =
    !!relatedCase || !values.project || isLoadingDeployments || deploymentsFieldDisabled;
  const productSelectDisabled = !!relatedCase || !values.deployment || isLoadingProducts;

  return {
    classified,
    isSubmitting,
    handleSubmit,
    isSavePending: isSubmitting || mutation.isPending,
    deploymentSelectDisabled,
    productSelectDisabled,
    projectsOptions,
    deploymentOptions,
    productOptions,
    issueTypeOptions,
    severityLevelOptions,
    afterProjectChange,
    afterDeploymentChange,
    afterProductChange,
    afterTitleChange,
    afterDescriptionChange,
    afterTypeChange,
    afterSeverityChange,
  };
}
