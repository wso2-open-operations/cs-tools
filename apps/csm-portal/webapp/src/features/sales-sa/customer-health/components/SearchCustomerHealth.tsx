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

// Ported verbatim — no backend call, pure debounced-input UI.
import { useEffect, useState, type ChangeEvent } from "react";
import { Box, InputAdornment, TextField } from "@wso2/oxygen-ui";
import { SearchIcon } from "@wso2/oxygen-ui-icons-react";
import { DEBOUNCE_DELAY } from "../api/customerHealthTypes";

interface SearchCustomerHealthProps {
  onSearch: (query: string) => void;
  initialValue?: string;
}

export default function SearchCustomerHealth({ onSearch, initialValue }: SearchCustomerHealthProps) {
  const [inputValue, setInputValue] = useState(initialValue || "");

  useEffect(() => {
    const delayDebounceFn = setTimeout(() => {
      onSearch(inputValue);
    }, DEBOUNCE_DELAY);

    return () => clearTimeout(delayDebounceFn);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [inputValue]);

  const handleInputChange = (event: ChangeEvent<HTMLInputElement>) => {
    setInputValue(event.target.value);
  };

  return (
    <TextField
      fullWidth
      size="medium"
      type="search"
      placeholder="Search by Account Name"
      value={inputValue}
      onChange={handleInputChange}
      sx={{ my: 2 }}
      slotProps={{
        input: {
          endAdornment: (
            <InputAdornment position="end">
              <Box component="span" sx={{ display: "inline-flex", color: "action.active" }}>
                <SearchIcon size={18} />
              </Box>
            </InputAdornment>
          ),
        },
      }}
    />
  );
}
