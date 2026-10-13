// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.

import {
  Box,
  ColorSchemeToggle,
  Container,
  Divider,
  Grid,
  Header as HeaderUI,
  Link as OxygenLink,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";
import { useEffect, useState, type JSX } from "react";
import { Link, Outlet, useNavigate } from "react-router";

const BRAND_LOGO_HEIGHT = {
  xs: 18,
  sm: 20,
  md: 24,
  lg: 20,
  xl: 24,
} as const;

/**
 * Shell for every page of the public knowledge portal. The whole site is
 * public, so there is no login button, no user profile, and no banner --
 * just the WSO2 brand, the page content, and a footer.
 */
export default function KBLayout(): JSX.Element {
  const navigate = useNavigate();

  // Oxygen doesn't expose a themed logo asset yet, so the colour scheme is
  // read off the document attribute directly (mirrors Brand.tsx).
  const [isDarkMode, setIsDarkMode] = useState<boolean>(
    document.documentElement.getAttribute("data-color-scheme") === "dark",
  );

  useEffect(() => {
    const observer = new MutationObserver(() => {
      setIsDarkMode(
        document.documentElement.getAttribute("data-color-scheme") === "dark",
      );
    });
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["data-color-scheme"],
    });
    return () => observer.disconnect();
  }, []);

  const logoSrc = isDarkMode ? "/logo-white.svg" : "/logo-dark.svg";

  return (
    <Box sx={{ minHeight: "100vh", display: "flex", flexDirection: "column" }}>
      <HeaderUI sx={{ width: "100%", maxWidth: "100%" }}>
        <HeaderUI.Brand
          onClick={() => navigate("/")}
          sx={{ cursor: "pointer", flexShrink: 0, gap: { xs: 0.75, sm: 1, md: 1.5 } }}
        >
          <HeaderUI.BrandLogo
            sx={{ display: "flex", alignItems: "center", flexShrink: 0 }}
          >
            <Box
              component="img"
              key={logoSrc}
              src={logoSrc}
              alt="WSO2"
              sx={{ height: BRAND_LOGO_HEIGHT, width: "auto", display: "block" }}
            />
          </HeaderUI.BrandLogo>
          <HeaderUI.BrandTitle
            sx={{
              whiteSpace: "nowrap",
              lineHeight: 1.2,
              flexShrink: 0,
              fontSize: { xs: "0.8125rem", sm: "0.875rem" },
            }}
          >
            Knowledge Base
          </HeaderUI.BrandTitle>
        </HeaderUI.Brand>
        <HeaderUI.Spacer />
        <ColorSchemeToggle />
      </HeaderUI>

      <Box component="main" sx={{ flex: 1 }}>
        <Outlet />
      </Box>

      <Box
        component="footer"
        sx={{
          borderTop: 2,
          borderColor: "primary.main",
          mt: 8,
          py: 4,
        }}
      >
        <Container maxWidth="lg">
          <Stack
            direction="row"
            flexWrap="wrap"
            alignItems="center"
            rowGap={1}
            columnGap={{ xs: 2.5, md: 4 }}
          >
            <Typography variant="body2" color="text.secondary">
              ©{new Date().getFullYear()} WSO2 LLC
            </Typography>
            <OxygenLink href="https://wso2.com/legal/" target="_blank" rel="noopener" underline="hover" color="text.primary" variant="body2">
              WSO2 Legal
            </OxygenLink>
            <OxygenLink href="https://asgardeo.io/legal/" target="_blank" rel="noopener" underline="hover" color="text.primary" variant="body2">
              Asgardeo Legal
            </OxygenLink>
            <OxygenLink href="https://asgardeo.io/privacy-policy/" target="_blank" rel="noopener" underline="hover" color="text.primary" variant="body2">
              Asgardeo Privacy
            </OxygenLink>
            <OxygenLink href="https://wso2.com/privacy-policy/do-not-sell-my-personal-information/" target="_blank" rel="noopener" underline="hover" color="text.primary" variant="body2">
              Do Not Sell My Personal Information
            </OxygenLink>
            <OxygenLink href="https://wso2.com/modern-slavery-statement/" target="_blank" rel="noopener" underline="hover" color="text.primary" variant="body2">
              Modern Slavery Statement
            </OxygenLink>
            <OxygenLink href="https://wso2.com/contact/" target="_blank" rel="noopener" underline="hover" color="text.primary" variant="body2">
              Report a Problem With This Page
            </OxygenLink>
          </Stack>
        </Container>
      </Box>
    </Box>
  );
}
