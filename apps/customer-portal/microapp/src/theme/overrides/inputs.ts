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

export default function Inputs(theme: Theme): Components {
  return {
    MuiInputBase: {
      styleOverrides: {
        root: {
          padding: 2,
          paddingLeft: 8,
          paddingRight: 8,
          borderRadius: 8,
          fontSize: theme.typography.subtitle1.fontSize,
          outline: `1px solid ${theme.palette.semantic.border.subtle}`,
          transition: "outline-color 0.2s ease",

          "& .MuiInputBase-input": {
            marginLeft: 2,
          },

          "&.Mui-focused": {
            outline: `1.5px solid ${theme.palette.primary.main}`,
          },

          "& .MuiInputAdornment-root .MuiSvgIcon-root": {
            fontSize: theme.typography.pxToRem(19),
          },
        },
      },
    },
  };
}
