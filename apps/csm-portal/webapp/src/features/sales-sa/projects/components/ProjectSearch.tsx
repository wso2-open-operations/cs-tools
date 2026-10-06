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

import { useState } from "react";
import {
  Box,
  List,
  ListItemButton,
  ListItemText,
  Paper,
  TextField,
  InputAdornment,
  Typography,
} from "@wso2/oxygen-ui";
import { SearchIcon } from "@wso2/oxygen-ui-icons-react";
import { useNavigate } from "react-router";
import { useSearchSplProjects } from "../api/useProjectsApi";

export default function ProjectSearch({ setShowTable }: { setShowTable: (value: boolean) => void }) {
  const [inputValue, setInputValue] = useState("");
  const navigate = useNavigate();
  const enabled = inputValue.length >= 4;
  const { data: results } = useSearchSplProjects({ phrase: inputValue, offset: 0, limit: 10, enabled });

  const handleChange = (value: string) => {
    setInputValue(value);
    if (value.length >= 4) setShowTable(false);
    else if (value.length === 0) setShowTable(true);
  };

  return (
    <Box sx={{ position: "relative", maxWidth: 480, mb: 2 }}>
      <TextField
        fullWidth
        placeholder="Search by Project Name"
        value={inputValue}
        onChange={(e) => handleChange(e.target.value)}
        slotProps={{
          input: {
            startAdornment: (
              <InputAdornment position="start">
                <SearchIcon size={18} />
              </InputAdornment>
            ),
          },
        }}
      />
      {inputValue.length >= 4 && (
        <Paper sx={{ position: "absolute", top: "100%", left: 0, right: 0, zIndex: 10, mt: 0.5 }}>
          {results === undefined ? (
            <Box sx={{ p: 2 }}>
              <Typography variant="body2" color="text.secondary">
                Searching…
              </Typography>
            </Box>
          ) : results.length === 0 ? (
            <Box sx={{ p: 2 }}>
              <Typography variant="body2" color="text.secondary">
                No results found.
              </Typography>
            </Box>
          ) : (
            <List dense>
              {results.map((item) => (
                <ListItemButton key={item.number} onClick={() => navigate(`/spl/projects/${item.sysId}`)}>
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
