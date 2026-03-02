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
  Card,
  Box,
  Typography,
  TextField,
  InputAdornment,
  FormControl,
  Select,
  MenuItem,
} from "@wso2/oxygen-ui";
import { Calendar } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";
import type { MetadataItem } from "@models/responses";

export interface TimeCardsDateFilterProps {
  startDate: string;
  endDate: string;
  onStartDateChange: (value: string) => void;
  onEndDateChange: (value: string) => void;
  state: string;
  onStateChange: (value: string) => void;
  timeCardStates?: MetadataItem[];
}

/**
 * TimeCardsDateFilter provides date range and state filters for time cards.
 *
 * @param {TimeCardsDateFilterProps} props - Date values, handlers, and counts.
 * @returns {JSX.Element} The rendered filter card.
 */
export default function TimeCardsDateFilter({
  startDate,
  endDate,
  onStartDateChange,
  onEndDateChange,
  state,
  onStateChange,
  timeCardStates = [],
}: TimeCardsDateFilterProps): JSX.Element {
  return (
    <Card
      sx={{
        p: 3,
        display: "flex",
        flexDirection: "column",
        gap: 3,
      }}
    >
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          gap: 4,
          flexWrap: "wrap",
        }}
      >
        {/* Date Range Filter */}
        <Box sx={{ display: "flex", alignItems: "center", gap: 2, flex: 1, minWidth: "300px" }}>
          <Typography
            variant="body2"
            component="label"
            sx={{ fontWeight: 500, color: "text.secondary", whiteSpace: "nowrap" }}
          >
            Filter by Date Range:
          </Typography>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
            <Typography
              component="label"
              htmlFor="time-cards-start-date"
              variant="body2"
              sx={{ fontWeight: 500, color: "text.secondary", whiteSpace: "nowrap" }}
            >
              From:
            </Typography>
            <TextField
              id="time-cards-start-date"
              type="date"
              size="small"
              value={startDate}
              onChange={(e) => onStartDateChange(e.target.value)}
              sx={{ minWidth: 200 }}
              InputProps={{
                startAdornment: (
                  <InputAdornment position="start">
                    <Calendar size={16} />
                  </InputAdornment>
                ),
              }}
            />
          </Box>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
            <Typography
              component="label"
              htmlFor="time-cards-end-date"
              variant="body2"
              sx={{ fontWeight: 500, color: "text.secondary", whiteSpace: "nowrap" }}
            >
              To:
            </Typography>
            <TextField
              id="time-cards-end-date"
              type="date"
              size="small"
              value={endDate}
              onChange={(e) => onEndDateChange(e.target.value)}
              sx={{ minWidth: 200 }}
              InputProps={{
                startAdornment: (
                  <InputAdornment position="start">
                    <Calendar size={16} />
                  </InputAdornment>
                ),
              }}
            />
          </Box>
        </Box>

        {/* State Filter */}
        <Box sx={{ display: "flex", alignItems: "center", gap: 2, flex: 1, minWidth: "300px" }}>
          <Typography
            variant="body2"
            component="label"
            sx={{ fontWeight: 500, color: "text.secondary", whiteSpace: "nowrap" }}
          >
            Filter by State:
          </Typography>
          <FormControl size="small" sx={{ minWidth: 200 }}>
            <Select
              value={state}
              onChange={(e) => onStateChange(e.target.value as string)}
              displayEmpty
            >
              <MenuItem value="">All States</MenuItem>
              {timeCardStates.map((stateOption) => (
                <MenuItem key={stateOption.id} value={stateOption.label}>
                  {stateOption.label}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
        </Box>
      </Box>
    </Card>
  );
}
