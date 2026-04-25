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

import type { CaseDetailsActionRowProps } from "@features/support/types/supportComponents";
import {
  Button,
  CircularProgress,
  Stack,
  alpha,
  useTheme,
  type Theme,
} from "@wso2/oxygen-ui";
import { type JSX, useState } from "react";
import {
  CASE_STATUS_ACTIONS,
  type CaseStatusPaletteIntent,
} from "@features/support/constants/supportConstants";
import useGetProjectFilters from "@api/useGetProjectFilters";
import { usePatchCase } from "@features/support/api/usePatchCase";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";
import {
  ACTION_TO_CASE_STATE_LABEL,
  getAvailableCaseActions,
  isWithinOpenRelatedCaseWindow,
  toPresentContinuousActionLabel,
  toPresentTenseActionLabel,
} from "@features/support/utils/support";

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

function getStateKeyForAction(
  actionLabel: string,
  caseStates?: { id: string; label: string }[],
): number | undefined {
  if (!caseStates?.length) return undefined;
  const stateLabel = ACTION_TO_CASE_STATE_LABEL[actionLabel];
  if (!stateLabel) return undefined;
  const entry = caseStates.find(
    (s) => s.label.toLowerCase() === stateLabel.toLowerCase(),
  );
  if (!entry?.id) return undefined;
  const num = Number(entry.id);
  return Number.isNaN(num) ? undefined : num;
}

export default function CaseDetailsActionRow({
  assignedEngineer,
  engineerInitials,
  statusLabel,
  closedOn,
  onOpenRelatedCase,
  projectId = "",
  caseId = "",
  isLoading = false,
  showOnlyEngineer = false,
}: CaseDetailsActionRowProps): JSX.Element {
  void assignedEngineer;
  void engineerInitials;
  void isLoading;
  const theme = useTheme();
  const { data: filterMetadata } = useGetProjectFilters(projectId);
  const caseStates = filterMetadata?.caseStates;

  const { showSuccess } = useSuccessBanner();
  const { showError } = useErrorBanner();

  const patchCase = usePatchCase(projectId, caseId);
  const [pendingActionLabel, setPendingActionLabel] = useState<string | null>(null);

  const availableActions = getAvailableCaseActions(statusLabel).filter(
    (label) => {
      if (label === "Open Related Case") {
        if (!onOpenRelatedCase) return false;
        if (!isWithinOpenRelatedCaseWindow(closedOn)) return false;
      }
      return true;
    },
  );

  if (showOnlyEngineer) {
    return <></>;
  }

  return (
    <Stack
      direction="row"
      spacing={1}
      alignItems="center"
      flexWrap="wrap"
      sx={{
        justifyContent: "flex-end",
      }}
    >
      {CASE_STATUS_ACTIONS.filter((action) =>
        availableActions.includes(action.label),
      ).map(({ label, Icon, paletteIntent }) => {
        const stateKey = getStateKeyForAction(label, caseStates);
        const isOpenRelatedCase = label === "Open Related Case";
        const canPatch = !isOpenRelatedCase && stateKey != null && !!caseId;
        const isThisPending =
          !isOpenRelatedCase && patchCase.isPending && pendingActionLabel === label;

        return (
          <Button
            key={label}
            variant="outlined"
            size="small"
            startIcon={
              isThisPending ? (
                <CircularProgress
                  size={ACTION_BUTTON_ICON_SIZE}
                  color="inherit"
                  sx={{ display: "block" }}
                />
              ) : (
                <Icon size={ACTION_BUTTON_ICON_SIZE} />
              )
            }
            disabled={!isOpenRelatedCase && (patchCase.isPending || !canPatch)}
            onClick={
              isOpenRelatedCase
                ? onOpenRelatedCase
                : canPatch
                  ? () => {
                      setPendingActionLabel(label);
                      patchCase.mutate(
                        { stateKey: stateKey! },
                        {
                          onSuccess: () => {
                            showSuccess("State updated successfully.");
                          },
                          onError: (err) => {
                            showError(
                              err?.message ??
                                "Failed to update case status. Please try again.",
                            );
                          },
                          onSettled: () => {
                            setPendingActionLabel(null);
                          },
                        },
                      );
                    }
                  : undefined
            }
            sx={getActionButtonSx(theme, paletteIntent) as Record<string, unknown>}
          >
            {isThisPending
              ? toPresentContinuousActionLabel(label)
              : toPresentTenseActionLabel(label)}
          </Button>
        );
      })}
    </Stack>
  );
}
