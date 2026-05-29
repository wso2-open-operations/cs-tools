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
import { useEffect, useRef } from "react";

import { useFormikContext } from "formik";

import { useClassification } from "@features/case-types/cases/context";
import { type CreateCaseFormValues, useCreateCase, useCreateCaseFormOptions } from "@features/case-types/cases/hooks";

export function useAutoFill() {
  const { setFieldValue } = useFormikContext<CreateCaseFormValues>();
  const { deployments, products, issueTypes, severities } = useCreateCaseFormOptions();
  const { set } = useClassification();
  const { state } = useCreateCase();

  const classifications = state.classifications;
  const fieldsSet = useRef<Set<string>>(new Set());

  useEffect(() => {
    if (!classifications) return;

    const { caseInfo, issueType, severityLevel } = classifications;
    const { environment, productName, shortDescription, description } = caseInfo || {};
    const classifiedFields: string[] = [];

    if (!deployments.pending && !fieldsSet.current.has("deployment")) {
      const match = deployments.options.find((d) => d.label === environment);
      if (match) {
        setFieldValue("deployment", match.value);
        classifiedFields.push("deployment");
        fieldsSet.current.add("deployment");
      }
    }

    if (!products.pending && !fieldsSet.current.has("product")) {
      const match = products.options.find((p) => p.label === productName);
      if (match) {
        setFieldValue("product", match.value);
        classifiedFields.push("product");
        fieldsSet.current.add("product");
      }
    }

    if (!issueTypes.pending && !fieldsSet.current.has("type")) {
      const match = issueTypes.options.find((i) => i.label === issueType);
      if (match) {
        setFieldValue("type", match.value);
        classifiedFields.push("type");
        fieldsSet.current.add("type");
      }
    }

    if (!severities.pending && !fieldsSet.current.has("severity")) {
      const match = severities.options.find((s) => s.label === severityLevel);
      if (match) {
        setFieldValue("severity", match.value);
        classifiedFields.push("severity");
        fieldsSet.current.add("severity");
      }
    }

    if (shortDescription && !fieldsSet.current.has("title")) {
      setFieldValue("title", shortDescription);
      classifiedFields.push("title");
      fieldsSet.current.add("title");
    }

    if (description && !fieldsSet.current.has("description")) {
      setFieldValue("description", description);
      classifiedFields.push("description");
      fieldsSet.current.add("description");
    }

    if (classifiedFields.length > 0) set(classifiedFields);
  }, [classifications, deployments, products, issueTypes, severities, setFieldValue, set]);
}
