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

import { Header as HeaderUI } from "@wso2/oxygen-ui";
import { WSO2 } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { useNavigate } from "react-router";

/**
 * Brand component for the header.
 *
 * @param {object} props - Component props.
 * @param {boolean} props.isNavigationDisabled - Whether navigation to the project hub is disabled.
 * @returns {JSX.Element} The Brand component.
 */
export default function Brand({
  isNavigationDisabled = false,
}: {
  isNavigationDisabled?: boolean;
}): JSX.Element {
  const navigate = useNavigate();

  return (
    <HeaderUI.Brand
      onClick={() => !isNavigationDisabled && navigate("/")}
      sx={{ cursor: isNavigationDisabled ? "default" : "pointer" }}
    >
      {/* brand logo */}
      <HeaderUI.BrandLogo>
        <WSO2 />
      </HeaderUI.BrandLogo>
      {/* brand title */}
      <HeaderUI.BrandTitle>Customer Portal</HeaderUI.BrandTitle>
    </HeaderUI.Brand>
  );
}
