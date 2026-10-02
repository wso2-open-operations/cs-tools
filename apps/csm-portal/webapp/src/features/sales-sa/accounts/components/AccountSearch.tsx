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

import { useEffect, useState } from "react";
import { Box, CircularProgress, List, ListItemButton, ListItemText, Paper, TextField, Typography } from "@wso2/oxygen-ui";
import { SearchIcon } from "@wso2/oxygen-ui-icons-react";
import { useNavigate } from "react-router";
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import { useSearchSplAccounts } from "../api/useAccountsApi";

export default function AccountSearch({
  searchOption,
  setShowTable,
}: {
  searchOption: "account" | "myAccount";
  setShowTable?: (value: boolean) => void;
}) {
  const navigate = useNavigate();
  const email = useIdTokenClaims()?.email;
  const [inputValue, setInputValue] = useState("");

  const enabled = inputValue.length >= 4 && (searchOption !== "myAccount" || Boolean(email));
  const { data, isLoading } = useSearchSplAccounts({
    email: searchOption === "myAccount" ? email : undefined,
    phrase: inputValue,
    offset: 0,
    limit: 10,
    active: false,
    enabled,
  });

  useEffect(() => {
    setShowTable?.(inputValue.length === 0);
  }, [inputValue, setShowTable]);

  return (
    <Box sx={{ maxWidth: 640, mx: "auto", mb: 2 }}>
      <TextField
        fullWidth
        placeholder="Search by Account Name"
        value={inputValue}
        onChange={(e) => setInputValue(e.target.value)}
        slotProps={{ input: { startAdornment: <SearchIcon size={18} style={{ marginRight: 8, opacity: 0.6 }} /> } }}
      />
      {inputValue.length >= 4 && (
        <Paper variant="outlined" sx={{ mt: 1 }}>
          {isLoading ? (
            <Box sx={{ display: "flex", justifyContent: "center", p: 2 }}>
              <CircularProgress size={20} />
            </Box>
          ) : !data || data.length === 0 ? (
            <Typography variant="body2" color="text.secondary" sx={{ p: 2, textAlign: "center" }}>
              No results found.
            </Typography>
          ) : (
            <List disablePadding>
              {data.map((item) => (
                <ListItemButton key={item.number} onClick={() => navigate(`/spl/accounts/${item.id}`)}>
                  <ListItemText primary={item.name} secondary={item.number} />
                </ListItemButton>
              ))}
            </List>
          )}
        </Paper>
      )}
    </Box>
  );
}
