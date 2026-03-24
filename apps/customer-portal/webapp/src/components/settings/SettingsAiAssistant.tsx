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
import useInfiniteProjects, { flattenProjectPages } from "@api/useGetProjects";
import { usePatchProject } from "@api/usePatchProject";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";
import { setNoveraChatEnabled } from "@utils/settingsStorage";

interface SettingsAiAssistantProps {
  projectId: string;
}

interface AiCapability {
  id: string;
  title: string;
  description: string;
  enabled: boolean;
  onToggle: (enabled: boolean) => void;
}

/**
 * AI Assistant settings tab: Novera chat, knowledge base suggestions.
 *
 * @param {SettingsAiAssistantProps} props - Component props.
 * @returns {JSX.Element} The component.
 */
export default function SettingsAiAssistant({
  projectId,
}: SettingsAiAssistantProps): JSX.Element {
  const theme = useTheme();
  const { showSuccess } = useSuccessBanner();
  const {
    data: projectsData,
    isSuccess: isProjectsLoaded,
    refetch: refetchProjects,
  } = useInfiniteProjects({ pageSize: 20, enabled: !!projectId });
  const patchProject = usePatchProject(projectId);
  const [noveraOverride, setNoveraOverride] = useState<boolean | null>(null);
  const [kbSuggestionsEnabled, setKbSuggestionsEnabled] = useState(true);

  const projectHasAgent = useMemo(() => {
    if (!isProjectsLoaded) return undefined;
    const project = flattenProjectPages(projectsData).find((p) => p.id === projectId);
    return project?.hasAgent;
  }, [isProjectsLoaded, projectsData, projectId]);

  const noveraEnabled = noveraOverride ?? projectHasAgent ?? true;

  useEffect(() => {
    if (projectHasAgent !== undefined) {
      setNoveraChatEnabled(projectHasAgent);
    }
  }, [projectHasAgent]);

  const handleNoveraToggle = useCallback(
    (checked: boolean) => {
      const previous = noveraEnabled;
      setNoveraOverride(checked);
      setNoveraChatEnabled(checked);
      patchProject.mutate(
        { hasAgent: checked },
        {
          onSuccess: async () => {
            await refetchProjects();
            setNoveraOverride(null);
            showSuccess("Novera settings updated successfully");
          },
          onError: () => {
            setNoveraOverride(previous);
            setNoveraChatEnabled(previous);
          },
        },
      );
    },
    [noveraEnabled, patchProject, refetchProjects, showSuccess],
  );

  const handleKbToggle = useCallback((checked: boolean) => {
    setKbSuggestionsEnabled(checked);
    // KB suggestions can be persisted similarly when backend supports it
  }, []);

  const capabilities: AiCapability[] = [
    {
      id: "novera",
      title: "AI Chat Assistant (Novera)",
      description:
        "Enable AI-powered chat assistant to help with troubleshooting before creating cases",
      enabled: noveraEnabled,
      onToggle: handleNoveraToggle,
    },
    {
      id: "kb",
      title: "Smart Knowledge Base Suggestions",
      description:
        "Get AI-powered article recommendations based on your issue description",
      enabled: kbSuggestionsEnabled,
      onToggle: handleKbToggle,
    },
  ];

  const enabledCount = capabilities.filter((c) => c.enabled).length;
  const disabledCount = capabilities.length - enabledCount;

  const indigoMain = colors.indigo?.[600] ?? colors.purple[600];

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      {/* AI support header card */}
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
              AI-Powered Support Assistant
            </Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
              Configure Novera, your intelligent support assistant. Enable or
              disable specific AI capabilities based on your team&apos;s needs.
              Changes take effect immediately.
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
                  {enabledCount} capabilities enabled
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
                  {disabledCount} capabilities disabled
                </Typography>
              </Box>
            </Box>
          </Box>
        </Box>
      </Paper>

      {/* Support capabilities list */}
      <Box>
        <Typography variant="h6" color="text.primary" sx={{ mb: 2 }}>
          Support Capabilities
        </Typography>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
          {capabilities.map((cap) => (
            <Paper key={cap.id} sx={{ p: 2.5 }}>
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
                    <Typography variant="body1">{cap.title}</Typography>
                    <Chip
                      label={cap.enabled ? "Active" : "Inactive"}
                      size="small"
                      color={cap.enabled ? "success" : "default"}
                      variant="outlined"
                      sx={{ height: 20, fontSize: "0.7rem" }}
                    />
                  </Box>
                  <Typography variant="body2" color="text.secondary">
                    {cap.description}
                  </Typography>
                </Box>
                <FormControlLabel
                  control={
                    <Switch
                      checked={cap.enabled}
                      onChange={(_, checked) => cap.onToggle(checked)}
                      color="warning"
                    />
                  }
                  label=""
                />
              </Box>
            </Paper>
          ))}
        </Box>
      </Box>

      {/* AI best practices card */}
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
              AI Best Practices
            </Typography>
            <Box component="ul" sx={{ m: 0, pl: 2.5, "& li": { mb: 0.5 } }}>
              <li>
                <Typography variant="body2" color="text.secondary">
                  Enable AI Chat Assistant to reduce case creation time by 60%
                </Typography>
              </li>
              <li>
                <Typography variant="body2" color="text.secondary">
                  Smart suggestions help users find relevant knowledge base
                  articles
                </Typography>
              </li>
              <li>
                <Typography variant="body2" color="text.secondary">
                  Automatic categorization improves case routing and faster
                  resolution
                </Typography>
              </li>
              <li>
                <Typography variant="body2" color="text.secondary">
                  AI insights help identify patterns and prevent recurring issues
                </Typography>
              </li>
            </Box>
          </Box>
        </Box>
      </Paper>
    </Box>
  );
}
