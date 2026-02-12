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

import { Box, Button, Grid, Typography } from "@wso2/oxygen-ui";
import { CircleCheck } from "@wso2/oxygen-ui-icons-react";
import { useState, useEffect, useRef, type FormEvent, type JSX } from "react";
import { useNavigate, useParams, useLocation } from "react-router";
import { useGetCaseCreationDetails } from "@api/useGetCaseCreationDetails";
import useGetCasesFilters from "@api/useGetCasesFilters";
import useGetProjectDetails from "@api/useGetProjectDetails";
import { useLogger } from "@hooks/useLogger";
import { useLoader } from "@context/linear-loader/LoaderContext";
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

/**
 * CreateCasePage component to review and edit AI-generated case details.
 *
 * @returns {JSX.Element} The rendered CreateCasePage.
 */
export default function CreateCasePage(): JSX.Element {
  const navigate = useNavigate();
  const location = useLocation();
  const { projectId } = useParams<{ projectId: string }>();
  const logger = useLogger();
  const { showLoader, hideLoader } = useLoader();
  const { data: metadata, isLoading, isError } = useGetCaseCreationDetails();
  const { data: projectDetails, isLoading: isProjectLoading } =
    useGetProjectDetails(projectId || "");
  const { data: filters, isLoading: isFiltersLoading } = useGetCasesFilters(
    projectId || "",
  );

  const [project, setProject] = useState("");
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
  const classificationProduct = classificationInfo?.productName
    ? classificationInfo.productVersion
      ? `${classificationInfo.productName} - ${classificationInfo.productVersion}`
      : classificationInfo.productName
    : "";

  useEffect(() => {
    if (isLoading || isProjectLoading || isFiltersLoading) {
      showLoader();
    } else {
      hideLoader();
    }
    return () => hideLoader();
  }, [isLoading, isProjectLoading, isFiltersLoading, showLoader, hideLoader]);

  useEffect(() => {
    if (projectDetails?.name) {
      setProject(projectDetails.name);
    }
  }, [projectDetails]);

  useEffect(() => {
    if (hasInitializedRef.current) {
      return;
    }

    if (isLoading || isFiltersLoading) {
      return;
    }

    if (!metadata) {
      return;
    }

    if (!project && !projectId && metadata?.projects?.[0]) {
      setProject(metadata.projects[0]);
    }

    const initialProduct =
      classificationProduct || metadata?.products?.[0] || "";
    const initialDeployment =
      classificationDeployment || metadata?.deploymentTypes?.[0] || "";

    // TODO: Remove this fallback logic once the mock interface removed.
    const fallbackIssueType =
      filters?.issueTypes?.[0]?.label || metadata?.issueTypes?.[0] || "";
    const initialIssueType = classificationIssueType || fallbackIssueType;

    const severityLevels =
      filters?.severities || metadata?.severityLevels || [];
    const severityMatch = severityLevels.find(
      (level: any) =>
        level.id === classificationSeverity ||
        level.label === classificationSeverity,
    );
    const fallbackSeverity = severityLevels?.[1]?.id || "";
    const initialSeverity =
      severityMatch?.id || classificationSeverity || fallbackSeverity;

    setProduct(initialProduct);
    setDeployment(initialDeployment);
    setIssueType(initialIssueType);
    setSeverity(initialSeverity);

    setTitle(classificationInfo?.shortDescription || getGeneratedIssueTitle());
    setDescription(
      classificationInfo?.description || getGeneratedIssueDescription(),
    );

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
    isLoading,
    metadata,
    project,
    projectId,
  ]);

  useEffect(() => {
    if (isError) {
      logger.error("Failed to load case creation details in CreateCasePage");
    }
  }, [isError, logger]);

  const handleBack = () => {
    if (projectId) {
      navigate(`/${projectId}/support/chat`);
    } else {
      navigate(-1);
    }
  };

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
  };

  const issueTypesList = filters?.issueTypes || metadata?.issueTypes || [];
  const hasIssueType = issueTypesList.some((type: any) => {
    const label = typeof type === "string" ? type : type.label;
    return label === classificationIssueType;
  });
  const extraIssueTypes =
    classificationIssueType && !hasIssueType ? [classificationIssueType] : [];

  const severityLevelsList = (filters?.severities ||
    metadata?.severityLevels ||
    []) as {
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

  const baseDeploymentOptions = metadata?.deploymentTypes ?? [];
  const baseProductOptions = metadata?.products ?? [];
  const extraDeploymentOptions =
    classificationDeployment &&
    !baseDeploymentOptions.includes(classificationDeployment)
      ? [classificationDeployment]
      : [];
  const extraProductOptions =
    classificationProduct && !baseProductOptions.includes(classificationProduct)
      ? [classificationProduct]
      : [];

  const renderContent = () => {
    if (isError) {
      return (
        <Box sx={{ py: 10, textAlign: "center" }}>
          <Typography variant="h6" color="error">
            Error loading case creation details. Please try again later.
          </Typography>
        </Box>
      );
    }

    return (
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
              project={project}
              product={product}
              setProduct={setProduct}
              deployment={deployment}
              setDeployment={setDeployment}
              metadata={metadata}
              isLoading={isLoading || isProjectLoading}
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
              metadata={metadata}
              filters={filters}
              isLoading={isLoading || isFiltersLoading}
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
                disabled={isLoading}
              >
                Create Support Case
              </Button>
            </Box>
          </Box>
        </Grid>

        {/* right column - sidebar */}
        <Grid size={{ xs: 12, md: 4 }}>
          <ConversationSummary metadata={metadata} isLoading={isLoading} />
        </Grid>
      </Grid>
    );
  };

  return (
    <Box sx={{ width: "100%", pt: 0, position: "relative" }}>
      {/* header section */}
      <CaseCreationHeader onBack={handleBack} />

      {/* main content grid container */}
      {renderContent()}
    </Box>
  );
}
