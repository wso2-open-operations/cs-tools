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

import { Box, Header as HeaderUI, useTheme } from "@wso2/oxygen-ui";
import { useEffect, useState, type JSX } from "react";
import { useNavigate } from "react-router";

const BRAND_LOGO_HEIGHT = {
  xs: 18,
  sm: 20,
  md: 24,
  lg: 20,
  xl: 24,
} as const;

const BRAND_TITLE_BASE_SX = {
  whiteSpace: "nowrap",
  lineHeight: 1.2,
  flexShrink: 0,
} as const;

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
  const theme = useTheme();
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
        !isNavigationDisabled && navigate("/", { state: { fromHeader: true } })
      }
      sx={{
        cursor: isNavigationDisabled ? "default" : "pointer",
        flexShrink: 0,
        gap: { xs: 0.75, sm: 1, md: 1.5 },
        [theme.breakpoints.between("lg", "xl")]: {
          gap: 0.75,
        },
      }}
    >
      <HeaderUI.BrandLogo
        sx={{
          display: "flex",
          alignItems: "center",
          flexShrink: 0,
        }}
      >
        <Box
          component="img"
          key={logoSrc}
          src={logoSrc}
          alt="Company Logo"
          sx={{
            height: BRAND_LOGO_HEIGHT,
            width: "auto",
            display: "block",
          }}
        />
      </HeaderUI.BrandLogo>
      <HeaderUI.BrandTitle
        sx={{
          ...BRAND_TITLE_BASE_SX,
          fontSize: {
            xs: "0.8125rem",
            sm: "0.875rem",
            md: "1rem",
            lg: "0.875rem",
            xl: "1rem",
          },
          [theme.breakpoints.down("xl")]: {
            display: "inline-block",
          },
        }}
      >
        Customer Portal
      </HeaderUI.BrandTitle>
    </HeaderUI.Brand>
  );
}
