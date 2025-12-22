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

import { ExpandMore } from "@mui/icons-material";
import { type Theme, type Components, InputBase } from "@mui/material";

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

          "&.MuiInputBase-multiline": {
            padding: 10,
          },

          "&.Mui-focused": {
            outline: `1.5px solid ${theme.palette.primary.main}`,
          },

          "& .MuiInputAdornment-root .MuiSvgIcon-root": {
            fontSize: theme.typography.pxToRem(20),
          },
        },
      },
    },
    MuiSelect: {
      defaultProps: {
        input: <InputBase />,
        IconComponent: ExpandMore,
        MenuProps: {
          slotProps: {
            paper: {
              sx: (theme) => ({
                outline: `1px solid ${theme.palette.divider}`,
                boxShadow: "rgba(0, 0, 0, 0.05) 0px 6px 24px 0px, rgba(0, 0, 0, 0.08) 0px 0px 0px 1px", // TODO: Replace this with proper palette tokens
              }),
            },
            list: {
              sx: (theme) => ({
                "& .MuiMenuItem-root": {
                  borderRadius: 0,
                  fontSize: theme.typography.body2,
                },
              }),
            },
          },
        },
      },
    },
    MuiMenuItem: {
      defaultProps: {
        dense: true,
      },
    },
  };
}
