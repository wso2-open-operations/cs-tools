// Copyright (c) 2025 WSO2 LLC. (https://www.wso2.com).
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

import type { Theme, Components } from "@mui/material";

export default function Buttons(theme: Theme): Components {
  return {
    MuiButtonBase: {
      styleOverrides: {
        root: {
          padding: 10,
          borderRadius: 8,
          fontSize: theme.typography.button.fontSize,
          display: "flex",
          gap: 7,
          variants: [
            {
              props: { variant: "outlined" },
              style: {
                background: theme.palette.background.default,
                color: theme.palette.text.primary,
                border: `1px solid ${theme.palette.semantic.border.subtle}`,
              },
            },
            {
              props: { variant: "contained" },
              style: {
                color: theme.palette.primary.contrastText,
                background: theme.palette.primary.main,
                borderColor: theme.palette.primary.main,
                borderWidth: 1,
              },
            },
          ],
        },
      },
    },
  };
}

declare module "@mui/material/ButtonBase" {
  interface ButtonBaseOwnProps {
    variant?: "outlined" | "contained";
  }
}
