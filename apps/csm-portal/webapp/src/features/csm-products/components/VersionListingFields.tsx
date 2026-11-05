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


import { Box, FormControlLabel, MenuItem, Switch, TextField } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import type { CatalogReleaseLine } from "@features/csm-products/types/productCatalog";
import { CARBON_KEYS } from "@features/csm-products/utils/catalogChange";
import { CARBON_FIELD_LABEL } from "@features/csm-products/utils/catalogFormat";
import type { ListingFormValues } from "@features/csm-products/utils/catalogProducts";

type ListingTextKey = "publicName" | "publicAbbreviation" | "description" | "tags" | "releaseLinks";

interface VersionListingFieldsProps {
  values: ListingFormValues;
  onChange: (patch: Partial<ListingFormValues>) => void;
  releaseLines: readonly CatalogReleaseLine[];
  errors: Record<string, string>;
  /** Errors show only once the user has tried to save. */
  showErrors: boolean;
  /** The Carbon platform fields; only Carbon-based products use them. */
  showCarbon: boolean;
}

/**
 * A version's publishing settings in a two-column grid: the two feed flags,
 * public name and abbreviation, release line, tags, release links,
 * description and the Carbon platform — shared by the Add product version and
 * Edit version dialogs.
 */
export default function VersionListingFields({
  values,
  onChange,
  releaseLines,
  errors,
  showErrors,
  showCarbon,
}: VersionListingFieldsProps): JSX.Element {
  const text = (key: ListingTextKey, label: string) => ({
    label,
    value: values[key],
    onChange: (event: { target: { value: string } }) => onChange({ [key]: event.target.value }),
    error: showErrors && Boolean(errors[key]),
    helperText: showErrors ? errors[key] : undefined,
    size: "small" as const,
    fullWidth: true,
  });

  return (
    <Box sx={{ display: "grid", gap: 2, gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr" } }}>
      <FormControlLabel
        control={
          <Switch
            checked={values.showInSupportMatrix}
            onChange={(event) => onChange({ showInSupportMatrix: event.target.checked })}
          />
        }
        label="On Support Matrix"
      />
      <FormControlLabel
        control={<Switch checked={values.isPublic} onChange={(event) => onChange({ isPublic: event.target.checked })} />}
        label="On Release Matrix"
      />
      <TextField {...text("publicName", "Public name")} />
      <TextField {...text("publicAbbreviation", "Public abbreviation")} />
      <TextField
        select
        size="small"
        fullWidth
        label="Release line"
        value={values.releaseLineId}
        onChange={(event) => onChange({ releaseLineId: event.target.value })}
      >
        <MenuItem value="">None</MenuItem>
        {releaseLines.map((line) => (
          <MenuItem key={line.id} value={line.id}>
            {line.name}
          </MenuItem>
        ))}
      </TextField>
      <TextField {...text("tags", "Tags")} helperText={(showErrors && errors.tags) || "Comma separated, e.g. OB"} />
      <TextField {...text("releaseLinks", "Release links")} sx={{ gridColumn: { sm: "1 / -1" } }} />
      <TextField {...text("description", "Description")} multiline minRows={2} sx={{ gridColumn: { sm: "1 / -1" } }} />
      {showCarbon &&
        CARBON_KEYS.map((key) => (
          <TextField
            key={key}
            label={`Carbon ${CARBON_FIELD_LABEL[key].toLowerCase()}`}
            size="small"
            fullWidth
            value={values.carbon[key]}
            onChange={(event) => onChange({ carbon: { ...values.carbon, [key]: event.target.value } })}
          />
        ))}
    </Box>
  );
}
