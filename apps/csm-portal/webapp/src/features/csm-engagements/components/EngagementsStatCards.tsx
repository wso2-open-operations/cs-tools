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

import { Box, Card, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import {
  AlertOctagon,
  Briefcase,
  CheckCircle,
  Pause,
  Play,
  Send,
} from "@wso2/oxygen-ui-icons-react";
import type { CsmEngagementRow } from "@features/csm-engagements/types/csmEngagements";

interface EngagementsStatCardsProps {
  engagements: CsmEngagementRow[];
  onSelect?: (key: "all" | "requested" | "in_progress" | "on_hold" | "completed" | "at_risk") => void;
  selected?: "all" | "requested" | "in_progress" | "on_hold" | "completed" | "at_risk";
}

interface StatCard {
  key: NonNullable<EngagementsStatCardsProps["selected"]>;
  label: string;
  value: number;
  icon: JSX.Element;
  color: "default" | "primary" | "warning" | "success" | "error" | "info";
}

export default function EngagementsStatCards({
  engagements,
  onSelect,
  selected = "all",
}: EngagementsStatCardsProps): JSX.Element {
  const counts = {
    all: engagements.length,
    requested: engagements.filter((e) => e.state === "requested").length,
    in_progress: engagements.filter((e) => e.state === "in_progress").length,
    on_hold: engagements.filter((e) => e.state === "on_hold").length,
    completed: engagements.filter((e) => e.state === "completed").length,
    at_risk: engagements.filter(
      (e) => (e.state === "in_progress" || e.state === "on_hold") && e.health !== "green",
    ).length,
  };

  const cards: StatCard[] = [
    { key: "all", label: "All engagements", value: counts.all, icon: <Briefcase size={18} />, color: "default" },
    { key: "requested", label: "Requested", value: counts.requested, icon: <Send size={18} />, color: "info" },
    { key: "in_progress", label: "In progress", value: counts.in_progress, icon: <Play size={18} />, color: "primary" },
    { key: "on_hold", label: "On hold", value: counts.on_hold, icon: <Pause size={18} />, color: "warning" },
    { key: "at_risk", label: "At risk", value: counts.at_risk, icon: <AlertOctagon size={18} />, color: "error" },
    { key: "completed", label: "Completed", value: counts.completed, icon: <CheckCircle size={18} />, color: "success" },
  ];

  return (
    <Box
      sx={{
        display: "grid",
        gap: 1.5,
        gridTemplateColumns: {
          xs: "repeat(2, minmax(0, 1fr))",
          sm: "repeat(3, minmax(0, 1fr))",
          md: "repeat(6, minmax(0, 1fr))",
        },
      }}
    >
      {cards.map((c) => {
        const isSelected = selected === c.key;
        return (
          <Card
            key={c.key}
            variant="outlined"
            onClick={() => onSelect?.(c.key)}
            sx={{
              p: 1.5,
              cursor: onSelect ? "pointer" : "default",
              borderColor: isSelected ? `${c.color}.main` : undefined,
              borderWidth: isSelected ? 2 : 1,
              transition: "border-color 120ms",
              "&:hover": onSelect
                ? { borderColor: `${c.color}.main`, opacity: 0.95 }
                : undefined,
            }}
          >
            <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  color: `${c.color}.main`,
                }}
              >
                {c.icon}
              </Box>
              <Box sx={{ display: "flex", flexDirection: "column", minWidth: 0 }}>
                <Typography
                  variant="caption"
                  color="text.secondary"
                  sx={{
                    whiteSpace: "nowrap",
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                  }}
                >
                  {c.label}
                </Typography>
                <Typography variant="h6" sx={{ lineHeight: 1.2 }}>
                  {c.value}
                </Typography>
              </Box>
            </Box>
          </Card>
        );
      })}
    </Box>
  );
}
