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
import { useState, useEffect, useRef, type JSX } from "react";
import { useNavigate, useParams } from "react-router";
import { useGetCaseCreationDetails } from "@/api/useGetCaseCreationDetails";
import useGetProjectDetails from "@/api/useGetProjectDetails";
import { useLogger } from "@/hooks/useLogger";
import { useLoader } from "@/context/linearLoader/LoaderContext";
import { AIInfoCard } from "@/components/support/caseCreationLayout/AIInfoCard";
import { BasicInformationSection } from "@/components/support/caseCreationLayout/BasicInformationSection";
import { CaseCreationHeader } from "@/components/support/caseCreationLayout/CaseCreationHeader";
import { CaseDetailsSection } from "@/components/support/caseCreationLayout/CaseDetailsSection";
import { ConversationSummary } from "@/components/support/caseCreationLayout/ConversationSummary";
import {
  getGeneratedIssueTitle,
  getGeneratedIssueDescription,
} from "@/models/mockFunctions";

/**
 * CreateCasePage component to review and edit AI-generated case details.
 *
 * @returns {JSX.Element} The rendered CreateCasePage.
 */
export default function CreateCasePage(): JSX.Element {
  const navigate = useNavigate();
  const { projectId } = useParams<{ projectId: string }>();
  const logger = useLogger();
  const { showLoader, hideLoader } = useLoader();
  const { data: metadata, isLoading, isError } = useGetCaseCreationDetails();
  const { data: projectDetails, isLoading: isProjectLoading } =
    useGetProjectDetails(projectId || "");

  const [project, setProject] = useState("");
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [issueType, setIssueType] = useState("");
  const [product, setProduct] = useState("");
  const [deployment, setDeployment] = useState("");
  const [severity, setSeverity] = useState("");

  const hasInitializedRef = useRef(false);

  useEffect(() => {
    if (isLoading || isProjectLoading) {
      showLoader();
    } else {
      hideLoader();
    }
    return () => hideLoader();
  }, [isLoading, isProjectLoading, showLoader, hideLoader]);

  useEffect(() => {
    if (projectDetails?.name) {
      setProject(projectDetails.name);
    }
  }, [projectDetails]);

  useEffect(() => {
    if (metadata && !hasInitializedRef.current) {
      if (!project && !projectId && metadata.projects?.[0]) {
        setProject(metadata.projects[0]);
      }
      setProduct(metadata.products?.[0] || "");
      setDeployment(metadata.deploymentTypes?.[0] || "");
      setIssueType(metadata.issueTypes?.[0] || "");
      setSeverity(metadata.severityLevels?.[1]?.id || "");
      setTitle(getGeneratedIssueTitle());
      setDescription(getGeneratedIssueDescription());
      hasInitializedRef.current = true;
    }
  }, [metadata, project, projectId]);

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

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
  };

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
              isLoading={isLoading}
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
