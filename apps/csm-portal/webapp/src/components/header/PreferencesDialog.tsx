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
  Dialog,
  FormControlLabel,
  IconButton,
  MenuItem,
  Select,
  Switch,
  Typography,
  type SelectChangeEvent,
} from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";
import { useThemePreference } from "@context/theme/ThemePreferenceContext";
import { isThemeKey } from "@config/themeConfig";
import {
  useCaseTabsBehavior,
  type CaseTabsCapMode,
} from "@context/case-tabs/CaseTabsBehaviorContext";

function isCapMode(value: string): value is CaseTabsCapMode {
  return value === "evict-oldest" || value === "evict-newest";
}

export interface PreferencesDialogProps {
  open: boolean;
  onClose: () => void;
}

/**
 * Single consolidated entry point for user-level display/behavior
 * preferences: the Oxygen UI theme (previously its own standalone dropdown,
 * `ThemeSelect`) and the two case-tabs preferences (`CaseTabsBehaviorContext`)
 * — an on/off toggle for the mechanism, and a cap-behavior mode that's only
 * relevant (and only shown as interactive) while it's on. All three are
 * localStorage-only, no-backend-sync preferences with the same persistence
 * shape, so one dialog holds them rather than growing the header with a new
 * standalone control per preference added.
 *
 * Reached from the "Preferences" item in the profile menu (`UserProfile`) —
 * same open/close-by-props shape as that same menu's "Profile" item
 * (`UserProfileModal`), rather than its own header icon button.
 */
export default function PreferencesDialog({
  open,
  onClose,
}: PreferencesDialogProps): JSX.Element {
  const { themeKey, setThemeKey, options: themeOptions } = useThemePreference();
  const {
    enabled,
    setEnabled,
    capMode,
    setCapMode,
    capModeOptions,
  } = useCaseTabsBehavior();

  const handleThemeChange = (e: SelectChangeEvent<string>): void => {
    const next = e.target.value;
    if (isThemeKey(next)) setThemeKey(next);
  };

  const handleCapModeChange = (e: SelectChangeEvent<string>): void => {
    const next = e.target.value;
    if (isCapMode(next)) setCapMode(next);
  };

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
          <Typography variant="h6">Preferences</Typography>
          <IconButton onClick={onClose} size="small" aria-label="Close">
            <X size={18} />
          </IconButton>
        </Box>

        <Box>
          <Typography
            variant="caption"
            color="text.secondary"
            sx={{ display: "block", fontWeight: 600, mb: 0.5 }}
          >
            Theme
          </Typography>
          <Select
            value={themeKey}
            onChange={handleThemeChange}
            size="small"
            fullWidth
            aria-label="Select theme"
          >
            {themeOptions.map((o) => (
              <MenuItem key={o.key} value={o.key}>
                {o.label}
              </MenuItem>
            ))}
          </Select>
        </Box>
        <Box>
          <Typography
            variant="caption"
            color="text.secondary"
            sx={{ display: "block", fontWeight: 600, mb: 0.5 }}
          >
            Case tabs
          </Typography>
          <FormControlLabel
            sx={{ ml: 0, justifyContent: "space-between", width: "100%" }}
            labelPlacement="start"
            control={
              <Switch
                size="small"
                checked={enabled}
                onChange={(e) => setEnabled(e.target.checked)}
                inputProps={{ "aria-label": "Open cases in tabs" }}
              />
            }
            label={
              <Typography variant="body2" color="text.secondary">
                Open cases in tabs
              </Typography>
            }
          />
          {/* Only meaningful while the toggle above is on — kept visible but
              disabled (rather than removed) so its current value stays
              legible and the dialog doesn't reflow when the toggle
              changes. */}
          <Select
            value={capMode}
            onChange={handleCapModeChange}
            size="small"
            fullWidth
            disabled={!enabled}
            aria-label="When the tab limit is reached"
            sx={{ mt: 1 }}
          >
            {capModeOptions.map((o) => (
              <MenuItem key={o.mode} value={o.mode}>
                {o.label}
              </MenuItem>
            ))}
          </Select>
        </Box>
      </Box>
    </Dialog>
  );
}
