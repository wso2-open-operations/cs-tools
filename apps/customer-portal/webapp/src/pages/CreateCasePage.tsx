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

import { Box, Button, Grid } from "@wso2/oxygen-ui";
import { CircleCheck } from "@wso2/oxygen-ui-icons-react";
import { useState, useEffect, useRef, type FormEvent, type JSX } from "react";
import { useNavigate, useParams, useLocation } from "react-router";
import { useQueries } from "@tanstack/react-query";
import { useAsgardeo } from "@asgardeo/react";
import { ApiQueryKeys } from "@constants/apiConstants";
import useGetCasesFilters from "@api/useGetCasesFilters";
import useGetProjectDetails from "@api/useGetProjectDetails";
import { useGetProjectDeployments } from "@api/useGetProjectDeployments";
import { fetchDeploymentProducts } from "@api/useGetDeploymentsProducts";
import { usePostCase } from "@api/usePostCase";
import { useLoader } from "@context/linear-loader/LoaderContext";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";
import { useMockConfig } from "@providers/MockConfigProvider";
import type { CreateCaseRequest } from "@models/requests";
import { AIInfoCard } from "@components/support/case-creation-layout/header/AIInfoCard";
import { BasicInformationSection } from "@components/support/case-creation-layout/sections/basic-information-section/BasicInformationSection";
import { CaseCreationHeader } from "@components/support/case-creation-layout/header/CaseCreationHeader";
import { CaseDetailsSection } from "@components/support/case-creation-layout/sections/case-details-section/CaseDetailsSection";
import { ConversationSummary } from "@components/support/case-creation-layout/sections/conversation-summary-section/ConversationSummary";
import {
  getGeneratedIssueTitle,
  getGeneratedIssueDescription,
} from "@models/mockFunctions";
import type { CaseClassificationResponse } from "@models/responses";
import {
  buildClassificationProductLabel,
  getBaseDeploymentOptions,
  getBaseProductOptions,
  resolveDeploymentMatch,
  resolveIssueTypeKey,
  resolveProductId,
  shouldAddClassificationProductToOptions,
} from "@utils/caseCreation";

/**
 * CreateCasePage component to review and edit AI-generated case details.
 *
 * @returns {JSX.Element} The rendered CreateCasePage.
 */
export default function CreateCasePage(): JSX.Element {
  const navigate = useNavigate();
  const location = useLocation();
  const { projectId } = useParams<{ projectId: string }>();
  const { showLoader, hideLoader } = useLoader();
  const { data: projectDetails, isLoading: isProjectLoading } =
    useGetProjectDetails(projectId || "");
  const { data: filters, isLoading: isFiltersLoading } = useGetCasesFilters(
    projectId || "",
  );
  const { getIdToken } = useAsgardeo();
  const { isMockEnabled } = useMockConfig();
  const { data: projectDeployments } = useGetProjectDeployments(
    projectId || "",
  );
  const deploymentIds =
    projectDeployments?.map((d) => d.id).filter(Boolean) ?? [];
  const deploymentProductQueries = useQueries({
    queries: deploymentIds.map((deploymentId) => ({
      queryKey: [ApiQueryKeys.DEPLOYMENT_PRODUCTS, deploymentId] as const,
      queryFn: () =>
        fetchDeploymentProducts(deploymentId, {
          getIdToken,
          isMockEnabled,
        }),
    })),
  });
  const deploymentProductsLoading = deploymentProductQueries.some(
    (q) => q.isLoading,
  );
  const deploymentProductsError = deploymentProductQueries.some(
    (q) => q.isError,
  );
  const allDeploymentProducts =
    !deploymentProductsLoading && !deploymentProductsError
      ? deploymentProductQueries.flatMap((q) => q.data ?? [])
      : [];
  const { showError } = useErrorBanner();
  const { showSuccess } = useSuccessBanner();
  const { mutate: postCase, isPending: isCreatePending } = usePostCase();

  useEffect(() => {
    if (deploymentProductsError) {
      showError(
        "Could not load product options. Some options may be unavailable.",
      );
    }
  }, [deploymentProductsError, showError]);

  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [issueType, setIssueType] = useState("");
  const [product, setProduct] = useState("");
  const [deployment, setDeployment] = useState("");
  const [severity, setSeverity] = useState("");

  const hasInitializedRef = useRef(false);
  const classificationState = location.state as
    | { classification?: CaseClassificationResponse }
    | undefined;
  const classification = classificationState?.classification;
  const classificationInfo = classification?.case_info;
  const classificationIssueType = classification?.issueType?.trim() || "";
  const classificationSeverity = classification?.severityLevel?.trim() || "";
  const classificationDeployment =
    classificationInfo?.environment?.trim() || "";
  const classificationProduct =
    buildClassificationProductLabel(classificationInfo);

  useEffect(() => {
    if (isProjectLoading || isFiltersLoading) {
      showLoader();
    } else {
      hideLoader();
    }
    return () => hideLoader();
  }, [isProjectLoading, isFiltersLoading, showLoader, hideLoader]);

  const projectDisplay = projectDetails?.name ?? "";

  useEffect(() => {
    if (hasInitializedRef.current) {
      return;
    }

    if (isFiltersLoading) {
      return;
    }

    const initialProduct = classificationProduct || "";
    const initialDeployment = classificationDeployment || "";

    const fallbackIssueType = filters?.issueTypes?.[0]?.label || "";
    const initialIssueType = classificationIssueType || fallbackIssueType;

    const severityLevels = (filters?.severities || []) as {
      id: string;
      label: string;
    }[];
    const severityMatch = severityLevels.find(
      (level) =>
        level.id === classificationSeverity ||
        level.label === classificationSeverity,
    );
    const fallbackSeverity = severityLevels?.[1]?.id || "";
    const initialSeverity =
      severityMatch?.id || classificationSeverity || fallbackSeverity;

    // Defer state updates to avoid cascading renders (eslint: no setState in effect body).
    queueMicrotask(() => {
      setProduct(initialProduct);
      setDeployment(initialDeployment);
      setIssueType(initialIssueType);
      setSeverity(initialSeverity);
      setTitle(
        classificationInfo?.shortDescription || getGeneratedIssueTitle(),
      );
      setDescription(
        classificationInfo?.description || getGeneratedIssueDescription(),
      );
    });
    hasInitializedRef.current = true;
  }, [
    classification,
    classificationDeployment,
    classificationInfo,
    classificationIssueType,
    classificationProduct,
    classificationSeverity,
    filters,
    isFiltersLoading,
    projectId,
  ]);

  const handleBack = () => {
    if (projectId) {
      navigate(`/${projectId}/support/chat`);
    } else {
      navigate(-1);
    }
  };

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    if (!projectId) return;

    if (isMockEnabled) {
      showSuccess("Case created successfully");
      navigate(`/${projectId}/support/cases/mock-case-created`);
      return;
    }

    const deploymentMatch = resolveDeploymentMatch(
      deployment,
      projectDeployments,
      filters?.deployments,
    );
    if (!deploymentMatch) {
      showError("Please select a valid deployment.");
      return;
    }

    const productId = resolveProductId(product, allDeploymentProducts);
    if (!productId) {
      showError("Please select a valid product.");
      return;
    }

    const issueTypeKey = resolveIssueTypeKey(issueType, filters?.issueTypes);
    const severityKey = parseInt(severity, 10) || 0;

    const payload: CreateCaseRequest = {
      deploymentId: String(deploymentMatch.id),
      description,
      issueTypeKey,
      productId: String(productId),
      projectId,
      severityKey,
      title,
    };

    postCase(payload, {
      onSuccess: (data) => {
        showSuccess("Case created successfully");
        navigate(`/${projectId}/support/cases/${data.id}`);
      },
      onError: () => {
        showError("We couldn't create your case. Please try again.");
      },
    });
  };

  const issueTypesList = filters?.issueTypes || [];
  const hasIssueType = issueTypesList.some(
    (type: string | { id: string; label: string }) => {
      const label = typeof type === "string" ? type : type.label;
      return label === classificationIssueType;
    },
  );
  const extraIssueTypes =
    classificationIssueType && !hasIssueType ? [classificationIssueType] : [];

  const severityLevelsList = (filters?.severities || []) as {
    id: string;
    label: string;
    description?: string;
  }[];
  const hasSeverity = severityLevelsList.some(
    (level) =>
      level.id === classificationSeverity ||
      level.label === classificationSeverity,
  );
  const extraSeverityLevels =
    classificationSeverity && !hasSeverity
      ? [{ id: classificationSeverity, label: classificationSeverity }]
      : [];

  const baseDeploymentOptions = getBaseDeploymentOptions(projectDeployments);
  const baseProductOptions = getBaseProductOptions(allDeploymentProducts);
  const sectionMetadata = {
    deploymentTypes: baseDeploymentOptions,
    products: baseProductOptions,
  };
  const extraDeploymentOptions =
    classificationDeployment &&
    !baseDeploymentOptions.includes(classificationDeployment)
      ? [classificationDeployment]
      : [];
  const extraProductOptions =
    classificationProduct &&
    shouldAddClassificationProductToOptions(
      classificationProduct,
      baseProductOptions,
    )
      ? [classificationProduct]
      : [];

  const renderContent = () => (
    <Grid container spacing={3}>
      {/* left column - form content */}
      <Grid size={{ xs: 12, md: 8 }}>
        {/* case creation form */}
        <Box
          component="form"
          onSubmit={handleSubmit}
          sx={{ display: "flex", flexDirection: "column", gap: 3 }}
        >
          <AIInfoCard />

          <BasicInformationSection
            project={projectDisplay}
            product={product}
            setProduct={setProduct}
            deployment={deployment}
            setDeployment={setDeployment}
            metadata={sectionMetadata}
            isLoading={isProjectLoading}
            extraDeploymentOptions={extraDeploymentOptions}
            extraProductOptions={extraProductOptions}
          />

          <CaseDetailsSection
            title={title}
            setTitle={setTitle}
            description={description}
            setDescription={setDescription}
            issueType={issueType}
            setIssueType={setIssueType}
            severity={severity}
            setSeverity={setSeverity}
            metadata={undefined}
            filters={filters}
            isLoading={isFiltersLoading}
            storageKey={
              projectId ? `create-case-draft-${projectId}` : undefined
            }
            extraIssueTypes={extraIssueTypes}
            extraSeverityLevels={extraSeverityLevels}
          />

          {/* form actions container */}
          <Box sx={{ display: "flex", justifyContent: "right" }}>
            {/* submit button */}
            <Button
              type="submit"
              variant="contained"
              startIcon={<CircleCheck size={18} />}
              color="primary"
              disabled={
                isMockEnabled ||
                isProjectLoading ||
                isFiltersLoading ||
                isCreatePending ||
                !projectId ||
                deploymentProductsLoading ||
                deploymentProductsError
              }
            >
              {isCreatePending ? "Creating..." : "Create Support Case"}
            </Button>
          </Box>
        </Box>
      </Grid>

      {/* right column - sidebar */}
      <Grid size={{ xs: 12, md: 4 }}>
        <ConversationSummary metadata={undefined} isLoading={false} />
      </Grid>
    </Grid>
  );

  return (
    <Box sx={{ width: "100%", pt: 0, position: "relative" }}>
      {/* header section */}
      <CaseCreationHeader onBack={handleBack} />

      {/* main content grid container */}
      {renderContent()}
    </Box>
  );
}
