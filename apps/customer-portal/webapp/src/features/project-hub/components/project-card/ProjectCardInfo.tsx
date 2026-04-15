// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
//
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

import { Box, Form } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { ClampedTextWithTooltip } from "@features/project-hub/components/project-card/ClampedTextWithTooltip";

interface ProjectCardInfoProps {
  title: string;
}

const TITLE_BLOCK_HEIGHT = "3.2rem";

/**
 * Renders the project card title (no description on hub cards).
 *
 * @param {ProjectCardInfoProps} props - The props for the component.
 * @returns {JSX.Element} The rendered info section.
 */
export default function ProjectCardInfo({
  title,
}: ProjectCardInfoProps): JSX.Element {
  const displayTitle = title || "--";

  return (
    <Form.CardHeader
      sx={{ pt: 1.5 }}
      title={
        <Box sx={{ height: TITLE_BLOCK_HEIGHT, overflow: "hidden" }}>
          <ClampedTextWithTooltip
            text={displayTitle}
            lineClamp={2}
            variant="h6"
            sx={{ mb: 1 }}
          />
        </Box>
      }
    />
  );
}
