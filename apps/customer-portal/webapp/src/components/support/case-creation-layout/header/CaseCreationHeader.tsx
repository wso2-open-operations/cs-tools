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

import { Box, Button, Chip, Typography } from "@wso2/oxygen-ui";
import { ArrowLeft, Sparkles } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";

interface CaseCreationHeaderProps {
  onBack: () => void;
}

/**
 * Header section for the Case Creation page.
 *
 * @param {CaseCreationHeaderProps} props - Component props.
 * @returns {JSX.Element} The rendered header.
 */
export const CaseCreationHeader = ({
  onBack,
}: CaseCreationHeaderProps): JSX.Element => (
  <Box sx={{ mb: 3 }}>
    {/* navigation button container */}
    <Button
      startIcon={<ArrowLeft size={16} />}
      onClick={onBack}
      variant="text"
      sx={{ mb: 2 }}
    >
      Back to Chat
    </Button>
    {/* header content container */}
    <Box
      sx={{
        display: "flex",
        justifyContent: "space-between",
        alignItems: "flex-start",
        flexWrap: "wrap",
        gap: 2,
      }}
    >
      {/* title and description container */}
      <Box>
        <Typography variant="h5" sx={{ mb: 0.5 }}>
          Review Case Details
        </Typography>
        <Typography variant="body2" color="text.secondary">
          Please review and edit the auto-populated information before
          submitting
        </Typography>
      </Box>
      <Chip
        icon={<Sparkles size={10} />}
        label="AI Generated"
        color="warning"
        variant="outlined"
        sx={{ p: 0.5 }}
      />
    </Box>
  </Box>
);
