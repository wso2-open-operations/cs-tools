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

import { Box, Card, Radio, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";

export type AnnouncementKind = "customer" | "eol";

interface AnnouncementKindOption {
  value: AnnouncementKind;
  label: string;
  description: string;
}

const OPTIONS: AnnouncementKindOption[] = [
  {
    value: "customer",
    label: "Create announcement for customers",
    description:
      "Target specific projects or all customer projects, with an optional security label.",
  },
  {
    value: "eol",
    label: "Product version / EOL announcement",
    description:
      "Sends only to customers running the selected end-of-life product version.",
  },
];

interface AnnouncementKindSelectorProps {
  kind: AnnouncementKind;
  onKindChange: (kind: AnnouncementKind) => void;
  disabled?: boolean;
}

/**
 * The first choice on the announcement create flow: which of the two flows
 * this send is. Each kind owns its own audience-resolution logic entirely —
 * "customer" resolves from project scope/exclusions (AudienceScopeControls),
 * "eol" resolves from a product+version's own affected deployments — so this
 * selector only ever decides which form renders below it, not any field
 * within either form.
 */
export default function AnnouncementKindSelector({
  kind,
  onKindChange,
  disabled,
}: AnnouncementKindSelectorProps): JSX.Element {
  return (
    <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }} role="radiogroup" aria-label="Announcement type">
      {OPTIONS.map((opt) => {
        const selected = kind === opt.value;
        return (
          <Card
            key={opt.value}
            variant="outlined"
            role="radio"
            aria-checked={selected}
            aria-label={opt.label}
            tabIndex={0}
            onClick={() => !disabled && onKindChange(opt.value)}
            onKeyDown={(e) => {
              if (disabled) return;
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                onKindChange(opt.value);
              }
            }}
            sx={{
              flex: "1 1 260px",
              minWidth: 240,
              p: 2,
              display: "flex",
              gap: 1.5,
              alignItems: "flex-start",
              cursor: disabled ? "default" : "pointer",
              borderColor: selected ? "primary.main" : "divider",
              borderWidth: selected ? 2 : 1,
              opacity: disabled ? 0.6 : 1,
            }}
          >
            {/* Purely presentational — the Card above is the actual role="radio"
                control (it owns tabIndex and onKeyDown). Without readOnly +
                inputProps, this native <input type="radio"> would be its own
                separately focusable, separately announced control nested
                inside another one. */}
            <Radio
              checked={selected}
              disabled={disabled}
              readOnly
              size="small"
              sx={{ p: 0, mt: 0.25 }}
              inputProps={{ tabIndex: -1, "aria-hidden": true }}
            />
            <Box sx={{ minWidth: 0 }}>
              <Typography variant="body2" fontWeight={600}>
                {opt.label}
              </Typography>
              <Typography variant="caption" color="text.secondary">
                {opt.description}
              </Typography>
            </Box>
          </Card>
        );
      })}
    </Box>
  );
}
