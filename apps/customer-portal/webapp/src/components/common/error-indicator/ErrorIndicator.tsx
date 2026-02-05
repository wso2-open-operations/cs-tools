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

import { type JSX } from "react";
import { Tooltip, IconButton } from "@wso2/oxygen-ui";
import { TriangleAlert } from "@wso2/oxygen-ui-icons-react";

interface ErrorIndicatorProps {
  entityName: string;
  size?: "small" | "medium" | "large";
}

/**
 * A tooltip component to display API fetch errors.
 *
 * @param props - Props for the component.
 * @returns {JSX.Element} The API error tooltip component.
 */
export default function ErrorIndicator({
  entityName,
  size = "small",
}: ErrorIndicatorProps): JSX.Element {
  const iconSize = size === "small" ? 16 : size === "medium" ? 24 : 32;

  return (
    <Tooltip title={`Failed to fetch ${entityName} data`}>
      <IconButton size={size} color="error">
        <TriangleAlert size={iconSize} />
      </IconButton>
    </Tooltip>
  );
}
