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
  CircularProgress,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowLeft, Send, PlusCircle } from "@wso2/oxygen-ui-icons-react";
import { useState, useRef, useCallback, useMemo, type JSX } from "react";
import { useLocation, useNavigate, useParams } from "react-router";
import Editor from "@components/common/rich-text-editor/Editor";
import { useGetProjectDeployments } from "@api/useGetProjectDeployments";
import { usePostConversations } from "@api/usePostConversations";
import { useAllDeploymentProducts } from "@hooks/useAllDeploymentProducts";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import {
  DEFAULT_CONVERSATION_REGION,
  DEFAULT_CONVERSATION_TIER,
} from "@constants/conversationConstants";
import type { ChatNavState } from "@models/chatNavState";
import { buildEnvProducts } from "@utils/caseCreation";
import { htmlToPlainText } from "@utils/richTextEditor";

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
  const [hasApiFailed, setHasApiFailed] = useState(false);
  const [isLoadingAfterClick, setIsLoadingAfterClick] = useState(false);

  const { data: projectDeployments } = useGetProjectDeployments(
    projectId || "",
  );
  const { productsByDeploymentId } =
    useAllDeploymentProducts(projectDeployments);
  const envProducts = useMemo(
    () => buildEnvProducts(productsByDeploymentId, projectDeployments),
    [productsByDeploymentId, projectDeployments],
  );

  const { mutateAsync: postConversation, isPending: isSubmitting } =
    usePostConversations();
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

  const plainText = useMemo(() => htmlToPlainText(value), [value]);

  const handleSubmit = useCallback(async () => {
    if (!plainText.trim() || !projectId) return;
    if (submittingRef.current) return;
    submittingRef.current = true;
    setIsLoadingAfterClick(true);

    try {
      const conversationResponse = await postConversation({
        projectId,
        message: plainText.trim(),
        envProducts,
        region: DEFAULT_CONVERSATION_REGION,
        tier: DEFAULT_CONVERSATION_TIER,
      });

      const state: ChatNavState = {
        initialUserMessage: plainText.trim(),
        conversationResponse,
      };
      navigate(`/projects/${projectId}/support/chat`, { state });
    } catch {
      setHasApiFailed(true);
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
    postConversation,
    navigate,
    showError,
  ]);

  const handleCreateCase = useCallback(() => {
    if (!projectId) return;
    navigate(`/projects/${projectId}/support/chat/create-case`, {
      state: { skipChat: true },
    });
  }, [projectId, navigate]);

  const isSubmitDisabled =
    !projectId || !plainText.trim();

  return (
    <Box
      sx={{
        flex: 1,
        display: "flex",
        flexDirection: "column",
        width: "100%",
        minHeight: 0,
        mt: -1.5,
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
        sx={{ flex: 1, minHeight: 0, display: "flex", flexDirection: "column" }}
      >
        <CardContent
          sx={{
            p: 3,
            flex: 1,
            minHeight: 0,
            display: "flex",
            flexDirection: "column",
          }}
        >
          <Stack spacing={3} sx={{ flex: 1, minHeight: 0 }}>
            <Box>
              <Typography variant="h5" sx={{ mb: 1 }} component="h1">
                What can we help you with?
              </Typography>
              <Typography variant="body2" color="text.secondary">
                Describe your issue or question in as much detail as you&apos;d
                like. We&apos;ll analyze it and guide you to a solution.
              </Typography>
            </Box>

            <Box>
              <Typography
                variant="body2"
                fontWeight={500}
                color="text.primary"
                component="label"
                htmlFor="describe-issue-editor"
                sx={{ display: "block", mb: 1, flexShrink: 0 }}
              >
                Describe your issue
              </Typography>
              <Editor
                id="describe-issue-editor"
                value={value}
                onChange={setValue}
                placeholder={ISSUE_PLACEHOLDER}
                minHeight={300}
                showToolbar
                toolbarVariant="describeIssue"
                onSubmitKeyDown={handleSubmit}
                disabled={isSubmitting}
              />
              <Typography
                variant="caption"
                color="text.secondary"
                sx={{ display: "block", mt: 1, flexShrink: 0 }}
              >
                Tip: Include details like error messages, when the issue
                started, affected systems, and what you&apos;ve already tried.
              </Typography>
            </Box>

            <Box
              sx={{
                display: "flex",
                justifyContent: "flex-end",
                gap: 2,
                flexShrink: 0,
              }}
            >
              {hasApiFailed && (
                <Button
                  variant="contained"
                  color="primary"
                  startIcon={<PlusCircle size={18} />}
                  onClick={handleCreateCase}
                  disabled={!projectId}
                >
                  Create Case
                </Button>
              )}
              <Button
                variant="contained"
                color="warning"
                startIcon={
                  isLoadingAfterClick ? (
                    <CircularProgress color="inherit" size={18} />
                  ) : (
                    <Send size={18} />
                  )
                }
                onClick={handleSubmit}
                disabled={isSubmitDisabled}
              >
                {isLoadingAfterClick ? "Getting help…" : "Submit & Get Help"}
              </Button>
            </Box>
          </Stack>
        </CardContent>
      </Card>
    </Box>
  );
}
