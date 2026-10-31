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

import { Box, Chip, Tooltip } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { levelDotColor } from "@features/csm-skills/utils/levels";
import type { SkillLevel } from "@features/csm-skills/types";

interface LevelChipProps {
  level: SkillLevel | null | undefined;
}

/**
 * A skill level as an outlined chip. The name is the readable label; the
 * backend's colour hint (when it is a plain hex colour) only tints a small dot,
 * so contrast never depends on data we do not control.
 */
export default function LevelChip({ level }: LevelChipProps): JSX.Element {
  if (!level) return <Chip size="small" variant="outlined" label="Not set" />;
  const dot = levelDotColor(level);
  const chip = (
    <Chip
      size="small"
      variant="outlined"
      label={
        <Box component="span" sx={{ display: "inline-flex", alignItems: "center", gap: 0.75 }}>
          {dot && (
            <Box
              component="span"
              aria-hidden
              sx={{ width: 8, height: 8, borderRadius: "50%", flexShrink: 0, bgcolor: dot }}
            />
          )}
          {level.name}
        </Box>
      }
    />
  );
  return level.description ? (
    <Tooltip title={level.description} placement="top" arrow>
      {chip}
    </Tooltip>
  ) : (
    chip
  );
}
