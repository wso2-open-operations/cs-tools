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
  useTheme,
} from "@wso2/oxygen-ui";
import { Bot, CircleAlert, Sparkles } from "@wso2/oxygen-ui-icons-react";
import { useCallback, useState, useMemo, useEffect, type JSX } from "react";
import useGetProjectDetails from "@api/useGetProjectDetails";
import useInfiniteProjects from "@api/useGetProjects";
import { usePatchProject } from "@features/settings/api/usePatchProject";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { setNoveraChatEnabled } from "@features/settings/utils/settingsStorage";
import {
  SETTINGS_AI_ADMIN_ONLY_HINT,
  SETTINGS_AI_BEST_PRACTICES_ITEMS,
  SETTINGS_AI_BEST_PRACTICES_TITLE,
  SETTINGS_AI_CAPABILITIES_SECTION_TITLE,
  SETTINGS_AI_DISABLED_CAPABILITIES_LABEL,
  SETTINGS_AI_ENABLED_CAPABILITIES_LABEL,
  SETTINGS_AI_HEADER_BODY,
  SETTINGS_AI_HEADER_TITLE,
  SETTINGS_AI_KB_DESCRIPTION,
  SETTINGS_AI_KB_LABEL,
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
  const theme = useTheme();
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
  const [kbOverride, setKbOverride] = useState<boolean | null>(null);

  const { projectHasAgent, projectHasKbReferences } = useMemo(() => {
    const detailsAvailable = !!projectDetails;
    const agent = detailsAvailable
      ? (projectDetails.hasAgent ?? projectDetails.account?.hasAgent)
      : undefined;
    const kb = detailsAvailable
      ? (projectDetails.hasKbReferences ?? projectDetails.account?.hasKbReferences)
      : undefined;
    return {
      projectHasAgent: agent,
      projectHasKbReferences: kb,
    };
  }, [projectDetails]);

  const noveraEnabled = noveraOverride ?? projectHasAgent ?? false;
  const kbReferencesEnabled =
    noveraEnabled &&
    (kbOverride ?? projectHasKbReferences ?? false);

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
      setKbOverride(null);
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
      if (!checked) {
        setKbOverride(false);
      }
      patchProject.mutate(checked ? { hasAgent: true } : { hasAgent: false }, {
        onSuccess: () => {
          void notifyPatchSuccess(AiAssistantPatchSuccessKind.NOVERA);
        },
        onError: (err) => {
          handlePatchError(err);
          setNoveraOverride(null);
          setKbOverride(null);
          setNoveraChatEnabled(rollbackNovera);
        },
      });
    },
    [noveraEnabled, patchProject, notifyPatchSuccess, handlePatchError],
  );

  const handleKbToggle = useCallback(
    (checked: boolean) => {
      if (!noveraEnabled) {
        return;
      }
      setKbOverride(checked);
      patchProject.mutate(
        { hasKbReferences: checked },
        {
          onSuccess: () => {
            void notifyPatchSuccess(AiAssistantPatchSuccessKind.KB);
          },
          onError: (err) => {
            handlePatchError(err);
            setKbOverride(null);
          },
        },
      );
    },
    [noveraEnabled, patchProject, notifyPatchSuccess, handlePatchError],
  );

  const enabledCount =
    (noveraEnabled ? 1 : 0) + (noveraEnabled && kbReferencesEnabled ? 1 : 0);
  const disabledCount = 2 - enabledCount;

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
            <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
              {SETTINGS_AI_HEADER_BODY}
            </Typography>
            <Box sx={{ display: "flex", alignItems: "center", gap: 2 }}>
              <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                <Box
                  sx={{
                    width: 8,
                    height: 8,
                    borderRadius: "50%",
                    bgcolor: "success.main",
                  }}
                />
                <Typography variant="caption" color="text.secondary">
                  {SETTINGS_AI_ENABLED_CAPABILITIES_LABEL(enabledCount)}
                </Typography>
              </Box>
              <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                <Box
                  sx={{
                    width: 8,
                    height: 8,
                    borderRadius: "50%",
                    bgcolor: "action.disabled",
                  }}
                />
                <Typography variant="caption" color="text.secondary">
                  {SETTINGS_AI_DISABLED_CAPABILITIES_LABEL(disabledCount)}
                </Typography>
              </Box>
            </Box>
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

          <Box
            sx={{
              pl: 2.5,
              ml: 1.5,
              borderLeft: "2px solid",
              borderColor: "divider",
            }}
          >
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
                    <Typography variant="body1" id="kb-suggestions-label">
                      {SETTINGS_AI_KB_LABEL}
                    </Typography>
                    <Chip
                      label={kbReferencesEnabled ? "Active" : "Inactive"}
                      size="small"
                      color={kbReferencesEnabled ? "success" : "default"}
                      variant="outlined"
                      sx={{ height: 20, fontSize: "0.7rem" }}
                    />
                  </Box>
                  <Typography variant="body2" color="text.secondary">
                    {SETTINGS_AI_KB_DESCRIPTION}
                  </Typography>
                </Box>
                <FormControlLabel
                  control={
                    <Switch
                      checked={kbReferencesEnabled}
                      disabled={!noveraEnabled || disabledForSwitches}
                      onChange={(_, checked) => handleKbToggle(checked)}
                      color="warning"
                      inputProps={{
                        "aria-labelledby": "kb-suggestions-label",
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

      <Paper
        sx={{
          p: 2.5,
          bgcolor: alpha(theme.palette.warning.main, 0.08),
          border: "1px solid",
          borderColor: alpha(theme.palette.warning.main, 0.2),
        }}
      >
        <Box sx={{ display: "flex", alignItems: "flex-start", gap: 1.5 }}>
          <Paper
            sx={{
              width: 40,
              height: 40,
              bgcolor: alpha(theme.palette.warning.light, 0.15),
              color: theme.palette.warning.main,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              flexShrink: 0,
            }}
          >
            <CircleAlert size={20} color={colors.orange[600]} />
          </Paper>
          <Box>
            <Typography variant="h6" color="text.primary" sx={{ mb: 1 }}>
              {SETTINGS_AI_BEST_PRACTICES_TITLE}
            </Typography>
            <Box component="ul" sx={{ m: 0, pl: 2.5, "& li": { mb: 0.5 } }}>
              {SETTINGS_AI_BEST_PRACTICES_ITEMS.map((item) => (
                <li key={item}>
                  <Typography variant="body2" color="text.secondary">
                    {item}
                  </Typography>
                </li>
              ))}
            </Box>
          </Box>
        </Box>
      </Paper>
    </Box>
  );
}
