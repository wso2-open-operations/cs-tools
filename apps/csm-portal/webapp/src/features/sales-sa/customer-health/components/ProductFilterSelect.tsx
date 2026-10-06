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

import { FormControl, Select, MenuItem, ListItemText, CircularProgress, type SelectChangeEvent } from "@wso2/oxygen-ui";
import { useProducts } from "../api/useLookups";

interface ProductFilterSelectProps {
  selectedProduct: string | null;
  onProductChange: (product: string | null) => void;
}

export default function ProductFilterSelect({ selectedProduct, onProductChange }: ProductFilterSelectProps) {
  const { data: products, isLoading } = useProducts();

  const handleChange = (event: SelectChangeEvent<string>) => {
    onProductChange(event.target.value || null);
  };

  return (
    <FormControl sx={{ m: 1, width: 220 }} size="small">
      <Select
        value={selectedProduct || ""}
        onChange={handleChange}
        displayEmpty
        disabled={isLoading}
        renderValue={(selected) => (isLoading ? "Loading..." : selected || "All Products")}
      >
        <MenuItem value="">
          <ListItemText primary="All Products" />
        </MenuItem>
        {isLoading && (
          <MenuItem disabled>
            <CircularProgress size={14} sx={{ mr: 1 }} /> Loading...
          </MenuItem>
        )}
        {(products ?? []).map((p) => (
          <MenuItem key={p} value={p}>
            <ListItemText primary={p} />
          </MenuItem>
        ))}
      </Select>
    </FormControl>
  );
}
