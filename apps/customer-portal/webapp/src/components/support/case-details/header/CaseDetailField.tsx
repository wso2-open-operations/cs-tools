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

import { Box, Typography } from "@wso2/oxygen-ui";
import { type JSX } from "react";
import ErrorIndicator from "@components/common/error-indicator/ErrorIndicator";
import { formatValue } from "@utils/support";

export interface CaseDetailFieldProps {
  label: string;
  value: string | null | undefined;
  isLoading: boolean;
  isError: boolean;
}

/**
 * Single label-value field row for the case details panel (e.g. Description, Created, Product).
 *
 * @param {CaseDetailFieldProps} props - Label, value, and loading/error state.
 * @returns {JSX.Element} The field row.
 */
export default function CaseDetailField({
  label,
  value,
  isLoading,
  isError,
}: CaseDetailFieldProps): JSX.Element {
  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: { xs: "column", sm: "row" },
        gap: 0.5,
        alignItems: { sm: "center" },
      }}
    >
      <Typography
        variant="body2"
        color="text.secondary"
        sx={{ minWidth: 140, fontWeight: 500 }}
      >
        {label}
      </Typography>
      {isError ? (
        <ErrorIndicator entityName="case details" size="small" />
      ) : isLoading ? (
        <Box
          sx={{
            width: 120,
            height: 20,
            bgcolor: "action.hover",
          }}
        />
      ) : (
        <Typography variant="body2" color="text.primary">
          {formatValue(value)}
        </Typography>
      )}
    </Box>
  );
}
