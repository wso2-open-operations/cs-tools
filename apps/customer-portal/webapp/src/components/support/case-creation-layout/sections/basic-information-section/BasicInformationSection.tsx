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

import {
  Box,
  Chip,
  FormControl,
  Grid,
  MenuItem,
  Paper,
  Select,
  Typography,
  TextField,
  IconButton,
} from "@wso2/oxygen-ui";
import { PencilLine, Sparkles } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";

interface BasicInformationSectionProps {
  project: string;
  product: string;
  setProduct: (value: string) => void;
  deployment: string;
  setDeployment: (value: string) => void;
  metadata: any;
  isLoading: boolean;
}

/**
 * Renders the Basic Information section used during case creation.
 *
 * This section allows users to select project, product, and deployment
 * details and displays related metadata.
 *
 * @returns The Basic Information section JSX element.
 */
export const BasicInformationSection = ({
  project,
  product,
  setProduct,
  deployment,
  setDeployment,
  metadata,
  isLoading,
}: BasicInformationSectionProps): JSX.Element => {
  const [isEditing, setIsEditing] = useState(false);

  return (
    <Paper sx={{ p: 3 }}>
      {/* section header container */}
      <Box
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          mb: 3,
        }}
      >
        <Typography variant="h6">Basic Information</Typography>
        <IconButton>
          <PencilLine size={18} onClick={() => setIsEditing(true)} />
        </IconButton>
      </Box>

      {/* project card grid layout */}
      <Grid container spacing={3}>
        {/* project selection field wrapper */}
        <Grid size={{ xs: 12 }}>
          {/* project field label container */}
          <Box sx={{ mb: 1, display: "flex", alignItems: "center", gap: 1 }}>
            <Typography variant="caption">Project</Typography>
            <Chip
              label="Auto detected"
              size="small"
              variant="outlined"
              icon={<Sparkles size={10} />}
              sx={{ height: 20, fontSize: "0.65rem", p: 0.5 }}
            />
          </Box>
          <TextField fullWidth size="small" disabled={true} value={project} />
        </Grid>

        {/* deployment selection field wrapper */}
        <Grid size={{ xs: 12 }}>
          {/* deployment field label container */}
          <Box sx={{ mb: 1, display: "flex", alignItems: "center", gap: 1 }}>
            <Typography variant="caption">
              Deployment Type{" "}
              <Box component="span" sx={{ color: "warning.main" }}>
                *
              </Box>
            </Typography>
            <Chip
              label="Auto detected"
              size="small"
              variant="outlined"
              icon={<Sparkles size={10} />}
              sx={{ height: 20, fontSize: "0.65rem", p: 0.5 }}
            />
          </Box>
          <FormControl
            fullWidth
            size="small"
            disabled={isLoading || !isEditing}
          >
            <Select
              value={deployment}
              onChange={(e) => setDeployment(e.target.value)}
              displayEmpty
              renderValue={(value) =>
                value === "" ? "Select Deployment Type..." : value
              }
            >
              <MenuItem value="" disabled>
                Select Deployment Type...
              </MenuItem>
              {metadata?.deploymentTypes?.map((d: string) => (
                <MenuItem key={d} value={d}>
                  {d}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
        </Grid>

        {/* product selection field wrapper */}
        <Grid size={{ xs: 12 }}>
          {/* product field label container */}
          <Box sx={{ mb: 1, display: "flex", alignItems: "center", gap: 1 }}>
            <Typography variant="caption">
              Product & Version{" "}
              <Box component="span" sx={{ color: "warning.main" }}>
                *
              </Box>
            </Typography>
            <Chip
              label="Auto detected"
              size="small"
              variant="outlined"
              icon={<Sparkles size={10} />}
              sx={{ height: 20, fontSize: "0.65rem", p: 0.5 }}
            />
          </Box>
          <FormControl
            fullWidth
            size="small"
            disabled={isLoading || !isEditing}
          >
            <Select
              value={product}
              onChange={(e) => setProduct(e.target.value)}
              displayEmpty
              renderValue={(value) =>
                value === "" ? "Select Product & Version..." : value
              }
            >
              <MenuItem value="" disabled>
                Select Product & Version...
              </MenuItem>
              {metadata?.products?.map((p: string) => (
                <MenuItem key={p} value={p}>
                  {p}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
        </Grid>
      </Grid>
    </Paper>
  );
};
