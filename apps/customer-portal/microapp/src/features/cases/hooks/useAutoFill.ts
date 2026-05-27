import { useEffect, useRef } from "react";

import { useFormikContext } from "formik";

import { useClassification } from "@features/cases/context";
import { type CreateCaseFormValues, useCreateCase, useCreateCaseFormOptions } from "@features/cases/hooks";

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
