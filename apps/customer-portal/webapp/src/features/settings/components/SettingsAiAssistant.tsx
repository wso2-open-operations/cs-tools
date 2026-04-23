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
// software distributed under the License is distributed on
// an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import {
  Box,
  Chip,
  FormControlLabel,
  Paper,
  Switch,
  Typography,
  alpha,
  colors,
} from "@wso2/oxygen-ui";
import { Bot, Sparkles } from "@wso2/oxygen-ui-icons-react";
import { useCallback, useState, useMemo, useEffect, type JSX } from "react";
import useGetProjectDetails from "@api/useGetProjectDetails";
import useInfiniteProjects from "@api/useGetProjects";
import { usePatchProject } from "@features/settings/api/usePatchProject";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { setNoveraChatEnabled } from "@features/settings/utils/settingsStorage";
import {
  SETTINGS_AI_ADMIN_ONLY_HINT,
  SETTINGS_AI_CAPABILITIES_SECTION_TITLE,
  SETTINGS_AI_HEADER_BODY,
  SETTINGS_AI_HEADER_TITLE,
  SETTINGS_AI_NOVERA_DESCRIPTION,
  SETTINGS_AI_NOVERA_LABEL,
  SETTINGS_AI_PATCH_ERROR,
} from "@features/settings/constants/settingsConstants";
import {
  AiAssistantPatchSuccessKind,
  type SettingsAiAssistantProps,
} from "@features/settings/types/settings";
import { getAiAssistantPatchSuccessMessage } from "@features/settings/utils/settingsPage";

/**
 * AI Assistant settings: Novera chat and Smart Knowledge Base suggestions (hasKbReferences).
 *
 * @param {SettingsAiAssistantProps} props - Component props.
 * @returns {JSX.Element} The component.
 */
export default function SettingsAiAssistant({
  projectId,
  canEdit = true,
}: SettingsAiAssistantProps): JSX.Element {
  const { showSuccess } = useSuccessBanner();
  const { showError } = useErrorBanner();
  const { refetch: refetchProjects } = useInfiniteProjects({
    pageSize: 20,
    enabled: !!projectId,
  });
  const { data: projectDetails, isLoading: isProjectDetailsLoading } =
    useGetProjectDetails(projectId);
  const patchProject = usePatchProject(projectId);
  const [noveraOverride, setNoveraOverride] = useState<boolean | null>(null);

  const { projectHasAgent } = useMemo(() => {
    const detailsAvailable = !!projectDetails;
    const agent = detailsAvailable
      ? (projectDetails.hasAgent ?? projectDetails.account?.hasAgent)
      : undefined;
    return { projectHasAgent: agent };
  }, [projectDetails]);

  const noveraEnabled = noveraOverride ?? projectHasAgent ?? false;

  useEffect(() => {
    if (projectHasAgent !== undefined) {
      setNoveraChatEnabled(projectHasAgent);
    }
  }, [projectHasAgent]);

  const notifyPatchSuccess = useCallback(
    async (kind: AiAssistantPatchSuccessKind) => {
      const refreshed = await refetchProjects();
      const refreshedProject = refreshed.data?.pages
        ?.flatMap((page) => page.projects ?? [])
        ?.find((p) => p.id === projectId);
      setNoveraOverride(null);
      if (refreshedProject?.hasAgent !== undefined) {
        setNoveraChatEnabled(refreshedProject.hasAgent);
      } else if (projectDetails?.hasAgent !== undefined) {
        setNoveraChatEnabled(projectDetails.hasAgent);
      }
      showSuccess(getAiAssistantPatchSuccessMessage(kind));
    },
    [projectDetails, projectId, refetchProjects, showSuccess],
  );

  const handlePatchError = useCallback(
    (err: unknown) => {
      showError(
        err instanceof Error
          ? err.message
          : SETTINGS_AI_PATCH_ERROR,
      );
    },
    [showError],
  );

  const handleNoveraToggle = useCallback(
    (checked: boolean) => {
      const rollbackNovera = noveraEnabled;
      setNoveraOverride(checked);
      setNoveraChatEnabled(checked);
      patchProject.mutate(checked ? { hasAgent: true } : { hasAgent: false }, {
        onSuccess: () => {
          void notifyPatchSuccess(AiAssistantPatchSuccessKind.NOVERA);
        },
        onError: (err) => {
          handlePatchError(err);
          setNoveraOverride(null);
          setNoveraChatEnabled(rollbackNovera);
        },
      });
    },
    [noveraEnabled, patchProject, notifyPatchSuccess, handlePatchError],
  );

  const disabledForSwitches =
    !canEdit || isProjectDetailsLoading || patchProject.isPending;

  const indigoMain = colors.indigo?.[600] ?? colors.purple[600];

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Paper
        sx={{
          p: 2.5,
          bgcolor: alpha(indigoMain, 0.08),
          border: "1px solid",
          borderColor: alpha(indigoMain, 0.2),
        }}
      >
        <Box sx={{ display: "flex", alignItems: "flex-start", gap: 2 }}>
          <Paper
            sx={{
              width: 40,
              height: 40,
              bgcolor: alpha(indigoMain, 0.15),
              color: indigoMain,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              flexShrink: 0,
            }}
          >
            <Sparkles size={20} color={indigoMain} />
          </Paper>
          <Box sx={{ flex: 1 }}>
            <Typography variant="h6" color="text.primary" sx={{ mb: 0.5 }}>
              {SETTINGS_AI_HEADER_TITLE}
            </Typography>
            <Typography variant="body2" color="text.secondary">
              {SETTINGS_AI_HEADER_BODY}
            </Typography>
          </Box>
        </Box>
      </Paper>

      <Box>
        <Typography variant="h6" color="text.primary" sx={{ mb: 2 }}>
          {SETTINGS_AI_CAPABILITIES_SECTION_TITLE}
        </Typography>
        {!canEdit && (
          <Typography
            variant="body2"
            color="text.secondary"
            sx={{ mb: 1.5, fontStyle: "italic" }}
          >
            {SETTINGS_AI_ADMIN_ONLY_HINT}
          </Typography>
        )}
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
          <Paper sx={{ p: 2.5 }}>
            <Box
              sx={{
                display: "flex",
                alignItems: "flex-start",
                justifyContent: "space-between",
                gap: 2,
              }}
            >
              <Box sx={{ flex: 1 }}>
                <Box
                  sx={{
                    display: "flex",
                    alignItems: "center",
                    gap: 1.5,
                    mb: 1,
                  }}
                >
                  <Bot size={20} color={colors.orange[600]} />
                  <Typography variant="body1" id="ai-chat-assistant-label">
                    {SETTINGS_AI_NOVERA_LABEL}
                  </Typography>
                  <Chip
                    label={noveraEnabled ? "Active" : "Inactive"}
                    size="small"
                    color={noveraEnabled ? "success" : "default"}
                    variant="outlined"
                    sx={{ height: 20, fontSize: "0.7rem" }}
                  />
                </Box>
                <Typography variant="body2" color="text.secondary">
                  {SETTINGS_AI_NOVERA_DESCRIPTION}
                </Typography>
              </Box>
              <FormControlLabel
                control={
                  <Switch
                    checked={noveraEnabled}
                    disabled={disabledForSwitches}
                    onChange={(_, checked) => handleNoveraToggle(checked)}
                    color="warning"
                    inputProps={{
                      "aria-labelledby": "ai-chat-assistant-label",
                    }}
                  />
                }
                label=""
              />
            </Box>
          </Paper>
        </Box>
      </Box>

    </Box>
  );
}
