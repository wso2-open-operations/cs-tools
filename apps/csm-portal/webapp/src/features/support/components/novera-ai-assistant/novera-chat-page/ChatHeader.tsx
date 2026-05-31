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

import type { ChatHeaderProps } from "@features/support/types/supportComponents";
import { Box, Button, Typography } from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";

/**
 * Renders the header section for the Novera Chat page.
 *
 * Includes navigation controls such as the back action and Create Case button.
 *
 * @returns The ChatHeader JSX element.
 */
export default function ChatHeader({
  onBack,
  chatNumber,
}: ChatHeaderProps): JSX.Element {
  return (
    <Box
      sx={{
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        mx: 3,
        mb: 1.5,
      }}
    >
      <Button
        startIcon={<ArrowLeft size={18} />}
        onClick={onBack}
        sx={{
          textTransform: "none",
          alignSelf: "flex-start",
          flexShrink: 0,
        }}
        variant="text"
      >
        Back
      </Button>
      {chatNumber && (
        <Typography variant="body2" color="text.secondary">
          Chat {chatNumber}
        </Typography>
      )}
    </Box>
  );
}
