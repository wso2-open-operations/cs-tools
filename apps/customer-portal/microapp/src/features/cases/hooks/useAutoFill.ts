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

import { useEffect, useState } from "react";
import { useLayout } from "@context/layout";
import type { CaseClassificationResponseDto } from "@features/cases/types/case.dto";
import type { Case } from "@features/cases/types/case.model";

type SelectOption<T> = { value: T; label: string };

type UseAutoFillOptions = {
  classifications: CaseClassificationResponseDto | undefined;
  relatedCase: Case | undefined;
  deploymentOptions: SelectOption<string>[];
  productOptions: SelectOption<string>[];
  issueTypeOptions: SelectOption<number>[] | undefined;
  severityLevelOptions: SelectOption<number>[] | undefined;
  deploymentsFieldDisabled: boolean;
  setFieldValue: (field: string, value: unknown) => void;
};

export function useAutoFill({
  classifications,
  relatedCase,
  deploymentOptions,
  productOptions,
  issueTypeOptions,
  severityLevelOptions,
  deploymentsFieldDisabled,
  setFieldValue,
}: UseAutoFillOptions) {
  const layout = useLayout();
  const [classified, setClassified] = useState(new Set<string>());

  useEffect(() => {
    if (!classifications) return;

    const autoFilledFields = new Set<string>();

    const matchedDeployment = deploymentOptions.find((option) => option.label === classifications.caseInfo.environment);
    if (matchedDeployment && !deploymentsFieldDisabled) {
      setFieldValue("deployment", matchedDeployment.value);
      autoFilledFields.add("deployment");
    }

    const matchedProduct = productOptions.find((option) => option.label === classifications.caseInfo.productName);
    if (matchedProduct) {
      setFieldValue("product", matchedProduct.value);
      autoFilledFields.add("product");
    }

    const matchedType = issueTypeOptions?.find((option) => option.label === classifications.issueType);
    if (matchedType) {
      setFieldValue("type", matchedType.value);
      autoFilledFields.add("type");
    }

    const matchedSeverity = severityLevelOptions?.find((option) =>
      option.label.includes(classifications.severityLevel),
    );
    if (matchedSeverity) {
      setFieldValue("severity", matchedSeverity.value);
      autoFilledFields.add("severity");
    }

    if (classifications.caseInfo.shortDescription) {
      setFieldValue("title", classifications.caseInfo.shortDescription);
      autoFilledFields.add("title");
    }

    if (classifications.caseInfo.description) {
      setFieldValue("description", classifications.caseInfo.description);
      autoFilledFields.add("description");
    }

    setClassified(autoFilledFields);
  }, [classifications, deploymentOptions]);

  useEffect(() => {
    if (!relatedCase) return;

    layout.setLayoutOverrides({ title: "Create Related Case" });

    setFieldValue("title", relatedCase.title);

    const matchedDeployment = deploymentOptions.find((option) => option.label === relatedCase.deployment);
    if (matchedDeployment) {
      setFieldValue("deployment", matchedDeployment.value);
    }

    const matchedProduct = productOptions.find((option) => option.label === relatedCase.product);
    if (matchedProduct) {
      setFieldValue("product", matchedProduct.value);
    }

    return () => layout.setLayoutOverrides({ title: undefined });
  }, [relatedCase, deploymentOptions]);

  useEffect(() => {
    if (!deploymentsFieldDisabled) return;
    if (!deploymentOptions.length) return;

    setFieldValue("deployment", deploymentOptions[0].value);
  }, [deploymentsFieldDisabled]);

  return { classified, setClassified };
}
