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

import { Button, type SxProps, type Theme } from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";

export interface CaseDetailsBackButtonProps {
  onClick: () => void;
  sx?: SxProps<Theme>;
}

/**
 * Back navigation button for case details (e.g. "Back to Support Center").
 *
 * @param {CaseDetailsBackButtonProps} props - onClick and optional sx.
 * @returns {JSX.Element} The back button.
 */
export default function CaseDetailsBackButton({
  onClick,
  sx = { mb: 1.5, ml: -0.5, alignSelf: "flex-start" },
}: CaseDetailsBackButtonProps): JSX.Element {
  return (
    <Button
      startIcon={<ArrowLeft size={16} />}
      onClick={onClick}
      sx={sx}
      variant="text"
    >
      Back to Support Center
    </Button>
  );
}
