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
  Button,
} from "@wso2/oxygen-ui";
import { Calendar, X } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";

export interface TimeCardsDateFilterProps {
  startDate: string;
  endDate: string;
  onStartDateChange: (value: string) => void;
  onEndDateChange: (value: string) => void;
  onClear?: () => void;
  hasFilters?: boolean;
}

/**
 * TimeCardsDateFilter provides date range filters for time cards.
 *
 * @param {TimeCardsDateFilterProps} props - Date values, change handlers, and optional clear handler.
 * @returns {JSX.Element} The rendered filter card.
 */
export default function TimeCardsDateFilter({
  startDate,
  endDate,
  onStartDateChange,
  onEndDateChange,
  onClear,
  hasFilters,
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
          gap: 2,
          flexWrap: "wrap",
        }}
      >
        <Typography
          variant="body2"
          component="label"
          sx={{
            fontWeight: 500,
            color: "text.secondary",
            whiteSpace: "nowrap",
          }}
        >
          Filter by Date Range:
        </Typography>
        <Box sx={{ display: "flex", alignItems: "center", gap: 2, flexWrap: "wrap" }}>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
            <Typography
              component="label"
              htmlFor="time-cards-start-date"
              variant="body2"
              sx={{
                fontWeight: 500,
                color: "text.secondary",
                whiteSpace: "nowrap",
              }}
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
              sx={{
                fontWeight: 500,
                color: "text.secondary",
                whiteSpace: "nowrap",
              }}
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
          {hasFilters && onClear && (
            <Button
              variant="text"
              size="small"
              onClick={onClear}
              startIcon={<X size={16} />}
              sx={{ color: "text.secondary" }}
            >
              Clear
            </Button>
          )}
        </Box>
      </Box>
    </Card>
  );
}
