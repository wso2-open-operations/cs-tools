import { Box } from "@mui/material";
import React from "react";
import type { Case } from "@/types/support.types";
import { CaseCard } from "./CaseCard";

interface AllCasesListProps {
  cases: Case[];
}

export const AllCasesList: React.FC<AllCasesListProps> = ({ cases }) => {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {cases.map((caseItem) => (
        <CaseCard key={caseItem.sysId} item={caseItem} />
      ))}
    </Box>
  );
};
