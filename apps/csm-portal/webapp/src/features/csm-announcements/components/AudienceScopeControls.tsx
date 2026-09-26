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
  Checkbox,
  Chip,
  FormControlLabel,
  Radio,
  RadioGroup,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";
import type { JSX } from "react";
import AsyncProjectMultiSelect from "@features/csm-cases/components/AsyncProjectMultiSelect";

export type AnnouncementAudienceScope = "specific" | "all";

interface AudienceScopeControlsProps {
  scope: AnnouncementAudienceScope;
  onScopeChange: (scope: AnnouncementAudienceScope) => void;
  projectIds: string[];
  onProjectIdsChange: (ids: string[]) => void;
  excludeCloudTypes: boolean;
  onExcludeCloudTypesChange: (value: boolean) => void;
  excludeClosedStates: boolean;
  onExcludeClosedStatesChange: (value: boolean) => void;
  /**
   * The backend-configured project keys that are always excluded from "All
   * customer projects" — read-only, shown for transparency only. Empty
   * while still loading or when nothing is configured; both render as no
   * chips, not an error state, since an unconfigured deployment is normal.
   */
  excludedProjectKeys: string[];
  disabled?: boolean;
}

/**
 * The announcement create form's audience scope: either today's hand-picked
 * project list, or "all customer projects" resolved from the entity
 * service's project search under a pair of default exclusions mirroring the
 * two ServiceNow flow conditions this replaces (Project Type != Cloud
 * Support/Cloud Evaluation Support, Closure State != Restricted/Suspended).
 * Both exclusions are shown and editable rather than applied silently, per
 * the Phase 1 brief's "default exclusions shown explicitly on the form, not
 * implicit."
 *
 * A third exclusion — a backend-configured denylist of specific project
 * keys that must never receive an automated announcement, mirroring a
 * hardcoded Project Key condition in the same real ServiceNow flow — is
 * applied unconditionally server-side (see `useResolveAnnouncementAudience`)
 * and is deliberately NOT one of the editable checkboxes above: it's a
 * mandatory platform policy, not a per-send choice, the same way the
 * EOL/product-version flow's own mandatory exclusion works.
 *
 * "All customer projects" here means "every tracked project except the ones
 * excluded below" — there is no Account Life Cycle field in the platform
 * yet (see the announcement-enhancement brief's gap analysis), so this
 * cannot filter to only true paying-customer accounts the way the
 * ServiceNow flow's own condition does. The resolved list below this
 * control is the review step that exists specifically to catch that gap.
 */
export default function AudienceScopeControls({
  scope,
  onScopeChange,
  projectIds,
  onProjectIdsChange,
  excludeCloudTypes,
  onExcludeCloudTypesChange,
  excludeClosedStates,
  onExcludeClosedStatesChange,
  excludedProjectKeys,
  disabled,
}: AudienceScopeControlsProps): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
      <RadioGroup
        row
        value={scope}
        onChange={(e) => onScopeChange(e.target.value as AnnouncementAudienceScope)}
      >
        <FormControlLabel
          value="specific"
          control={<Radio size="small" disabled={disabled} />}
          label="Specific projects"
        />
        <FormControlLabel
          value="all"
          control={<Radio size="small" disabled={disabled} />}
          label="All customer projects"
        />
      </RadioGroup>

      {scope === "specific" && (
        <AsyncProjectMultiSelect
          id="announcement-create-projects"
          label="Projects"
          values={projectIds}
          onChange={onProjectIdsChange}
        />
      )}

      {scope === "all" && (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
          <Typography variant="caption" color="text.secondary">
            Account Life Cycle filtering isn't available yet, so this includes every
            tracked project except the ones excluded below. Review the resolved list
            before sending.
          </Typography>
          <FormControlLabel
            control={
              <Checkbox
                size="small"
                checked={excludeCloudTypes}
                disabled={disabled}
                onChange={(e) => onExcludeCloudTypesChange(e.target.checked)}
              />
            }
            label="Exclude Cloud Support / Cloud Evaluation Support projects"
          />
          <FormControlLabel
            control={
              <Checkbox
                size="small"
                checked={excludeClosedStates}
                disabled={disabled}
                onChange={(e) => onExcludeClosedStatesChange(e.target.checked)}
              />
            }
            label="Exclude Restricted / Suspended projects"
          />
          {excludedProjectKeys.length > 0 && (
            <Box>
              <FormControlLabel
                control={<Checkbox size="small" checked disabled />}
                label="Exclude these configured projects (mandatory, can't be turned off here)"
              />
              <Stack
                direction="row"
                spacing={0.5}
                sx={{ flexWrap: "wrap", gap: 0.5, ml: 4.5, mt: -0.5 }}
              >
                {excludedProjectKeys.map((key) => (
                  <Chip key={key} label={key} size="small" variant="outlined" />
                ))}
              </Stack>
              <Typography variant="caption" color="text.secondary" sx={{ display: "block", ml: 4.5 }}>
                Change via CSM_ANNOUNCEMENT_EXCLUDED_PROJECT_KEYS.
              </Typography>
            </Box>
          )}
        </Box>
      )}
    </Box>
  );
}
