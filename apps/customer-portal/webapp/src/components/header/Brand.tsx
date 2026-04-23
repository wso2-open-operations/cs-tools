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
import { useEffect, useState, type JSX } from "react";
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

  // TODO : This need to remove once svg available on oxygen ui
  const [isDarkMode, setIsDarkMode] = useState<boolean>(
    document.documentElement.getAttribute("data-color-scheme") === "dark",
  );

  useEffect(() => {
    const observer = new MutationObserver(() => {
      const currentScheme =
        document.documentElement.getAttribute("data-color-scheme");
      setIsDarkMode(currentScheme === "dark");
    });

    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["data-color-scheme"],
    });

    return () => observer.disconnect();
  }, []);

  const logoSrc = isDarkMode ? "/logo-white.svg" : "/logo-dark.svg";

  return (
    <HeaderUI.Brand
      onClick={() =>
        !isNavigationDisabled &&
        navigate("/", { state: { fromHeader: true } })
      }
      sx={{ cursor: isNavigationDisabled ? "default" : "pointer" }}
    >
      {/* brand logo */}
      <HeaderUI.BrandLogo>
        <img
          key={logoSrc}
          src={logoSrc}
          alt="Company Logo"
          style={{ height: "24px", width: "auto" }}
        />
      </HeaderUI.BrandLogo>
      {/* brand title */}
      <HeaderUI.BrandTitle>Customer Portal</HeaderUI.BrandTitle>
    </HeaderUI.Brand>
  );
}
