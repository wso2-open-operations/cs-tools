import { useNavigate } from "react-router-dom";

import * as Yup from "yup";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useFormik } from "formik";

import { useProject } from "@context/project";

import { cases } from "@features/cases/api/cases.queries";
import type { AttachmentFile } from "@features/cases/components";
import { useCreateCase } from "@features/cases/hooks";

import { toBase64 } from "@shared/utils/attachments.utils";

export interface CreateCaseFormValues {
  project: string;
  product: string;
  deployment: string;
  type: string;
  severity: string;
  title: string;
  description: string;
  attachments: AttachmentFile[];
}

const validationSchema = Yup.object({
  project: Yup.string().required("Project is required"),
  deployment: Yup.string().required("Deployment type is required"),
  product: Yup.string().required("Product & version is required"),
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

export function useCreateCaseForm() {
  const { projectId } = useProject();
  const { state, create } = useCreateCase();
  const createAttachmentMutation = useMutation(cases.createAttachment);
  const queryClient = useQueryClient();
  const navigate = useNavigate();

  const formik = useFormik<CreateCaseFormValues>({
    initialValues: {
      project: projectId!,
      product: "",
      deployment: "",
      title: "",
      description: "",
      type: "",
      severity: "",
      attachments: [],
    },
    validationSchema,
    validateOnBlur: true,
    validateOnChange: true,
    onSubmit: async (values) => {
      const response = await create.mutateAsync({
        type: "default_case",
        projectId: values.project,
        deploymentId: values.deployment,
        deployedProductId: values.product,
        title: values.title,
        description: values.description,
        issueTypeKey: Number(values.type),
        severityKey: Number(values.severity),
        relatedCaseId: state?.case?.id,
      });

      await Promise.all(
        values.attachments.map(async (attachment) => {
          const content = await toBase64(attachment.raw);
          await createAttachmentMutation.mutateAsync({
            caseId: response.id,
            type: attachment.type,
            name: attachment.name,
            content,
          });
        }),
      );

      queryClient.invalidateQueries({ queryKey: ["cases"] });
      setTimeout(() => {
        navigate(`/cases/${response.id}`);
      }, 500);
    },
  });

  return formik;
}
