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
  Chip,
  Dialog,
  IconButton,
  Switch,
  Typography,
} from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";
import { featureState, listPreviewableFeatures } from "@config/featureFlags";
import { useFeaturePreviewOverrides } from "@config/featurePreview";
import { usePortalAccess, useSafeCurrentUserId } from "@context/current-user/usePortalAccess";

export interface FeaturePreviewDialogProps {
  open: boolean;
  onClose: () => void;
}

/**
 * Admin-only: lists every nav feature this deployment currently hides or
 * marks work-in-progress, each behind its own toggle. Turning one on makes
 * that feature visible to the signed-in admin ONLY, in this browser ONLY
 * (see `featurePreview.ts`) — it never touches the deployment-wide
 * `CSM_PORTAL_FEATURE_OVERRIDES` config (`public/config.js`), which stays
 * ops's own instant, affects-everyone kill switch.
 *
 * Reached from the "Feature Preview" item in the profile menu (`UserProfile`),
 * same open/close-by-props shape as that menu's "Preferences" item
 * (`PreferencesDialog`), whose chrome this is modeled on.
 */
export default function FeaturePreviewDialog({
  open,
  onClose,
}: FeaturePreviewDialogProps): JSX.Element {
  const access = usePortalAccess();
  const { overrides, setOverride } = useFeaturePreviewOverrides(useSafeCurrentUserId());
  const candidates = listPreviewableFeatures(access);

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="xs">
      <Box sx={{ p: 3, display: "flex", flexDirection: "column", gap: 2 }}>
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
          }}
        >
          <Typography variant="h6">Feature Preview</Typography>
          <IconButton onClick={onClose} size="small" aria-label="Close">
            <X size={18} />
          </IconButton>
        </Box>

        <Typography variant="body2" color="text.secondary">
          These are in-progress features this deployment has hidden from most
          users. Turning one on only changes what you see — nobody else's
          view changes.
        </Typography>

        {candidates.length === 0 ? (
          <Typography variant="body2" color="text.secondary" sx={{ py: 1 }}>
            Nothing to preview right now — every feature is already visible to
            you.
          </Typography>
        ) : (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
            {candidates.map((node) => {
              // Resolved WITHOUT any preview override, so the badge always
              // shows this deployment's real current state, not the
              // admin's own already-applied toggle — it never changes
              // meaning: it's still describing what everyone ELSE sees,
              // which stays hidden/WIP regardless of this one admin's own
              // toggle. But showing it right next to an already-on switch
              // reads as a contradiction ("Hidden" beside a lit-up toggle
              // the admin can clearly see), so it's dropped for exactly as
              // long as this admin's own override is active — restored the
              // moment they turn it back off.
              const state = featureState(node.id, access);
              const enabled = Boolean(overrides[node.id]);
              return (
                <Box
                  key={node.id}
                  sx={{
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "space-between",
                    gap: 1,
                  }}
                >
                  <Box sx={{ display: "flex", alignItems: "center", gap: 1, minWidth: 0 }}>
                    <Typography variant="body2" noWrap>
                      {node.label}
                    </Typography>
                    {!enabled && (
                      <Chip
                        size="small"
                        color={state === "preview" ? "warning" : "default"}
                        label={state === "preview" ? "Preview" : "Hidden"}
                      />
                    )}
                  </Box>
                  <Switch
                    size="small"
                    checked={enabled}
                    onChange={(e) => setOverride(node.id, e.target.checked)}
                    // MUI v7: `Switch`'s own internal `slotProps.input = { role:
                    // "switch" }` overwrites a plain `inputProps={{ "aria-label"
                    // }}` (last-key-wins), silently dropping it — passing both
                    // keys through `slotProps.input` directly keeps the label.
                    slotProps={{
                      input: { role: "switch", "aria-label": `Preview ${node.label}` },
                    }}
                  />
                </Box>
              );
            })}
          </Box>
        )}
      </Box>
    </Dialog>
  );
}
