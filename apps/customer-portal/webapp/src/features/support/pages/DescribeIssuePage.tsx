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

import {
  Box,
  Button,
  Card,
  CardContent,
  Stack,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowLeft, Send } from "@wso2/oxygen-ui-icons-react";
import { useState, useRef, useCallback, useMemo, type JSX } from "react";
import { useLocation, useNavigate, useParams } from "react-router";
import { usePostProjectDeploymentsSearchAll } from "@api/usePostProjectDeploymentsSearch";
import useGetProjectDetails from "@api/useGetProjectDetails";
import { useAllDeploymentProducts } from "@features/support/hooks/useAllDeploymentProducts";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { buildEnvProducts } from "@features/support/utils/caseCreation";
import { filterDeploymentsForCaseCreation } from "@utils/permission";
import type { ChatNavState } from "@features/support/types/conversations";

const ISSUE_PLACEHOLDER =
  "Example: I'm experiencing API Gateway timeout issues in our production environment. The errors started appearing yesterday around 3 PM, and we're seeing 504 errors intermittently...";

/**
 * DescribeIssuePage lets users describe their issue before navigating to chat.
 * On submit, calls POST /projects/:projectId/conversations and navigates to
 * chat with the API response.
 *
 * @returns {JSX.Element} The describe issue page.
 */
export default function DescribeIssuePage(): JSX.Element {
  const navigate = useNavigate();
  const location = useLocation();
  const { projectId } = useParams<{ projectId: string }>();
  const { showError } = useErrorBanner();
  const [value, setValue] = useState("");
  const [isLoadingAfterClick, setIsLoadingAfterClick] = useState(false);

  const { data: allProjectDeployments } = usePostProjectDeploymentsSearchAll(
    projectId || "",
  );
  const { data: projectDetails } = useGetProjectDetails(projectId || "");
  const projectDeployments = useMemo(
    () =>
      filterDeploymentsForCaseCreation(
        allProjectDeployments,
        projectDetails?.type?.label,
      ),
    [allProjectDeployments, projectDetails?.type?.label],
  );
  const { productsByDeploymentId, isLoading: isLoadingDeploymentProducts } =
    useAllDeploymentProducts(projectDeployments);
  const envProducts = useMemo(
    () => buildEnvProducts(productsByDeploymentId, projectDeployments),
    [productsByDeploymentId, projectDeployments],
  );

  const submittingRef = useRef(false);

  const handleBack = useCallback(() => {
    if (
      location.key === "default" ||
      (typeof window !== "undefined" && window.history.length <= 1)
    ) {
      navigate(projectId ? `/projects/${projectId}/dashboard` : "/");
    } else {
      navigate(-1);
    }
  }, [navigate, location.key, projectId]);

  const plainText = value;

  const handleSubmit = useCallback(async () => {
    if (!plainText.trim() || !projectId) return;
    if (submittingRef.current) return;
    submittingRef.current = true;
    setIsLoadingAfterClick(true);

    try {
      const accountId = projectDetails?.account?.id ?? projectId;
      const state: ChatNavState = {
        initialUserMessage: plainText.trim(),
        initialEnvProducts: envProducts,
        accountId,
      };
      navigate(`/projects/${projectId}/support/chat`, { state });
    } catch {
      showError(
        "Could not get help. Please try again or create a support case.",
      );
    } finally {
      submittingRef.current = false;
      setIsLoadingAfterClick(false);
    }
  }, [
    plainText,
    projectId,
    envProducts,
    projectDetails?.account?.id,
    navigate,
    showError,
  ]);

  const isSubmitDisabled =
    !projectId || !plainText.trim() || isLoadingDeploymentProducts;

  return (
    <Box
      sx={{
        flex: 1,
        display: "flex",
        flexDirection: "column",
        width: "100%",
        minHeight: 0,
        mt: -1.5,
        overflow: "hidden",
        maxHeight: "100%",
      }}
    >
      <Button
        startIcon={<ArrowLeft size={18} />}
        onClick={handleBack}
        sx={{
          mb: 1.5,
          textTransform: "none",
          alignSelf: "flex-start",
          flexShrink: 0,
        }}
        variant="text"
      >
        Back
      </Button>

      <Card
        variant="outlined"
        sx={{
          flex: 1,
          minHeight: 0,
          display: "flex",
          flexDirection: "column",
          overflow: "hidden",
        }}
      >
        <CardContent
          sx={{
            p: 3,
            flex: 1,
            minHeight: 0,
            display: "flex",
            flexDirection: "column",
            overflow: "hidden",
          }}
        >
          <Box
            sx={{
              flex: 1,
              minHeight: 0,
              display: "flex",
              flexDirection: "column",
            }}
          >
            <Stack
              spacing={3}
              sx={{
                pb: 0.5,
                flex: 1,
                minHeight: 0,
                display: "flex",
                flexDirection: "column",
              }}
            >
              <Box>
                <Typography variant="h5" sx={{ mb: 1 }} component="h1">
                  What can we help you with?
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  Describe your issue or question in as much detail as
                  you&apos;d like. We&apos;ll analyze it and guide you to a
                  solution.
                </Typography>
              </Box>

              <Box sx={{ minWidth: 0 }}>
                <TextField
                  id="describe-issue-editor"
                  value={value}
                  onChange={(e) => setValue(e.target.value)}
                  placeholder={ISSUE_PLACEHOLDER}
                  multiline
                  rows={15}
                  fullWidth
                  variant="outlined"
                  sx={{
                    flex: 1,
                    "& textarea": { overflowY: "auto", resize: "none" },
                  }}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" && !e.shiftKey && !isSubmitDisabled) {
                      if (e.nativeEvent.isComposing) return;
                      e.preventDefault();
                      void handleSubmit();
                    }
                  }}
                />
                <Typography
                  variant="caption"
                  color="text.secondary"
                  sx={{ display: "block", mt: 1 }}
                >
                  Tip: Include details like error messages, when the issue
                  started, affected systems, and what you&apos;ve already tried.
                </Typography>
              </Box>

              <Box
                sx={{
                  position: "sticky",
                  bottom: 0,
                  display: "flex",
                  justifyContent: "flex-end",
                  gap: 2,
                  pt: 1,
                  pb: 1,
                  pr: 2,
                  mt: 1,
                  zIndex: 1,
                }}
              >
                <Button
                  variant="contained"
                  color="warning"
                  startIcon={<Send size={18} />}
                  onClick={handleSubmit}
                  loading={isLoadingAfterClick}
                  loadingPosition="start"
                  disabled={isSubmitDisabled}
                  sx={{ py: 0.5 }}
                >
                  Submit & Get Help
                </Button>
              </Box>
            </Stack>
          </Box>
        </CardContent>
      </Card>
    </Box>
  );
}
