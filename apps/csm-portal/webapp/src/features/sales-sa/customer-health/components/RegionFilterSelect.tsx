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

// Ported verbatim — no backend call, static option list.
import { Select, MenuItem, ListItemText, FormControl, Checkbox, type SelectChangeEvent } from "@wso2/oxygen-ui";

const regionOptions = [
  "AFRICA", "ANZ", "ASIA", "Benelux + France",
  "LATAM", "ME", "NA - CENTRAL", "NA - EAST", "NA - SOUTH", "NA - WEST",
  "ROE", "RoA", "SAARC", "UK", "UK + Ireland",
];

interface RegionFilterSelectProps {
  selectedRegions: string[];
  onRegionChange: (newRegions: string[]) => void;
}

export default function RegionFilterSelect({ selectedRegions, onRegionChange }: RegionFilterSelectProps) {
  const handleChange = (event: SelectChangeEvent<string[]>) => {
    const value = event.target.value;
    onRegionChange(typeof value === "string" ? value.split(",") : value);
  };

  return (
    <FormControl sx={{ m: 1, width: 200 }} size="small">
      <Select
        multiple
        value={selectedRegions}
        onChange={handleChange}
        displayEmpty
        renderValue={(selected) => {
          if (!selected || selected.length === 0) return "All Regions";
          if (selected.length === 1) return selected[0];
          return `${selected.length} Regions`;
        }}
      >
        {regionOptions.map((region) => (
          <MenuItem key={region} value={region}>
            <Checkbox checked={selectedRegions.indexOf(region) > -1} />
            <ListItemText primary={region} />
          </MenuItem>
        ))}
      </Select>
    </FormControl>
  );
}
