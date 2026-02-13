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
  Avatar,
  Box,
  Button,
  Divider,
  Paper,
  Skeleton,
  Stack,
  Typography,
  alpha,
  useTheme,
  type Theme,
} from "@wso2/oxygen-ui";
import { CirclePlay } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";
import ErrorIndicator from "@components/common/error-indicator/ErrorIndicator";
import {
  CASE_STATUS_ACTIONS,
  type CaseStatusPaletteIntent,
} from "@constants/supportConstants";
import { formatValue } from "@utils/support";

const ACTION_BUTTON_ICON_SIZE = 12;

function getActionButtonSx(
  theme: Theme,
  intent: CaseStatusPaletteIntent,
): Record<string, unknown> {
  const light = theme.palette[intent].light;
  return {
    borderColor: light,
    bgcolor: alpha(light, 0.1),
    color: light,
    fontSize: "0.7rem",
    minHeight: 0,
    py: 0.5,
    px: 1,
    "&:hover": {
      borderColor: theme.palette[intent].main,
      bgcolor: alpha(light, 0.2),
    },
    textTransform: "none",
  };
}

export interface CaseDetailsActionRowProps {
  assignedEngineer: string | null | undefined;
  engineerInitials: string;
  isError: boolean;
  isLoading?: boolean;
}

/**
 * Support action row: avatar, name, "Support Engineer" label, and "Manage case status" actions.
 *
 * @param {CaseDetailsActionRowProps} props - Action display data and error state.
 * @returns {JSX.Element} The action row wrapped in Paper.
 */
export default function CaseDetailsActionRow({
  assignedEngineer,
  engineerInitials,
  isError,
  isLoading = false,
}: CaseDetailsActionRowProps): JSX.Element {
  const theme = useTheme();

  return (
    <Paper
      variant="outlined"
      sx={{
        mt: 2,
        mb: 1,
        py: 0.5,
        px: 2,
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        flexWrap: "wrap",
        gap: 1,
        bgcolor: "background.default",
        minHeight: 0,
      }}
    >
      <Stack direction="row" spacing={1.5} alignItems="center">
        {isLoading ? (
          <Skeleton variant="circular" width={18} height={18} />
        ) : (
          <Avatar
            sx={{
              width: 18,
              height: 18,
              bgcolor: "primary.light",
              color: "primary.contrastText",
              fontSize: "0.6rem",
            }}
          >
            {engineerInitials}
          </Avatar>
        )}
        <Box>
          {isError ? (
            <ErrorIndicator entityName="case details" size="small" />
          ) : isLoading ? (
            <Skeleton variant="text" width={90} height={14} sx={{ mb: 0.25 }} />
          ) : (
            <Typography
              variant="caption"
              color="text.primary"
              sx={{ lineHeight: 1.2 }}
            >
              {formatValue(assignedEngineer)}
            </Typography>
          )}
          <Typography
            variant="caption"
            color="text.secondary"
            sx={{ fontSize: "0.7rem", lineHeight: 1.2, display: "block" }}
          >
            Support Engineer
          </Typography>
        </Box>
        <Divider orientation="vertical" flexItem />
        <Stack direction="row" spacing={1.5} alignItems="center">
          <CirclePlay size={12} color={theme.palette.primary.main} />
          <Typography
            variant="caption"
            color="text.secondary"
            sx={{ fontSize: "0.7rem" }}
          >
            Manage case status
          </Typography>
        </Stack>
      </Stack>

      <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap">
        {CASE_STATUS_ACTIONS.map(({ label, Icon, paletteIntent }) => (
          <Button
            key={label}
            variant="outlined"
            size="small"
            startIcon={<Icon size={ACTION_BUTTON_ICON_SIZE} />}
            sx={
              getActionButtonSx(theme, paletteIntent) as Record<string, unknown>
            }
          >
            {label}
          </Button>
        ))}
      </Stack>
    </Paper>
  );
}
